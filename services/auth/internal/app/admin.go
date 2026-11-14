package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
)

// Actor is the authenticated caller of an admin use case.
type Actor struct {
	UserID string
	Role   domain.Role
	IP     string
}

func (a Actor) requireAdmin() error {
	if a.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return nil
}

// CreateUserInput is an admin request to provision an account.
type CreateUserInput struct {
	Email       string
	DisplayName string
	Role        domain.Role
	Password    string
}

func validateNewUser(in *CreateUserInput) error {
	in.Email = domain.NormalizeEmail(in.Email)
	var v domain.Validator
	domain.ValidateEmail(&v, in.Email)
	domain.ValidateText(&v, "display_name", in.DisplayName, domain.MaxDisplayNameLen)
	if !in.Role.Valid() {
		v.Add("role", "must be one of student, teacher, admin, system")
	}
	domain.ValidatePassword(&v, in.Password, in.Email)
	return v.Err()
}

// CreateUser provisions an account (FR-2) and publishes user.created.
func (s *Service) CreateUser(ctx context.Context, actor Actor, in CreateUserInput) (*domain.User, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	if err := validateNewUser(&in); err != nil {
		return nil, err
	}
	hash, err := s.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	u := domain.User{ID: ids.NewV7At(now), Email: in.Email, DisplayName: in.DisplayName, Role: in.Role,
		Status: domain.StatusActive, PasswordHash: hash, CreatedAt: now, UpdatedAt: now}
	err = s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.InsertUser(ctx, u); err != nil {
			return err
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: actor.UserID, Action: "admin.user_create", TargetType: "user", TargetID: u.ID,
			Outcome: OutcomeSuccess, IP: actor.IP, Detail: map[string]string{"role": string(u.Role)}}); err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TypeUserCreated, events.Actor{UserID: actor.UserID, Role: string(actor.Role)}, events.Context{},
			events.UserCreated{UserID: u.ID, Role: string(u.Role), CreatedBy: actor.UserID}, u.ID, now)
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// EnsureBootstrapAdmin creates the first admin from configuration when no
// active admin exists. It is safe to call from several replicas at once.
func (s *Service) EnsureBootstrapAdmin(ctx context.Context, email, password string) (created bool, err error) {
	in := CreateUserInput{Email: email, DisplayName: "Administrator", Role: domain.RoleAdmin, Password: password}
	if err := validateNewUser(&in); err != nil {
		return false, err
	}
	hash, err := s.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return false, err
	}
	now := s.Clock.Now()
	err = s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.LockBootstrap(ctx); err != nil {
			return err
		}
		n, err := tx.CountActiveAdmins(ctx)
		if err != nil || n > 0 {
			return err
		}
		u := domain.User{ID: ids.NewV7At(now), Email: in.Email, DisplayName: in.DisplayName, Role: domain.RoleAdmin,
			Status: domain.StatusActive, PasswordHash: hash, CreatedAt: now, UpdatedAt: now}
		if err := tx.InsertUser(ctx, u); err != nil {
			return err
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: u.ID, Action: "admin.bootstrap", TargetType: "user", TargetID: u.ID, Outcome: OutcomeSuccess}); err != nil {
			return err
		}
		created = true
		return s.publish(ctx, tx, events.TypeUserCreated, events.Actor{UserID: u.ID, Role: string(domain.RoleAdmin)}, events.Context{},
			events.UserCreated{UserID: u.ID, Role: string(domain.RoleAdmin), CreatedBy: u.ID}, u.ID, now)
	})
	return created, err
}

// UpdateUserInput changes role and/or status; nil fields are unchanged.
type UpdateUserInput struct {
	Role   *domain.Role
	Status *domain.Status
}

// UpdateUser changes a user's role or status (FR-2). Deactivation revokes
// all sessions; leaving the student role removes group membership.
func (s *Service) UpdateUser(ctx context.Context, actor Actor, id string, in UpdateUserInput) (*domain.User, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	var v domain.Validator
	if in.Role != nil && !in.Role.Valid() {
		v.Add("role", "must be one of student, teacher, admin, system")
	}
	if in.Status != nil && !in.Status.Valid() {
		v.Add("status", "must be active or deactivated")
	}
	if in.Role == nil && in.Status == nil {
		v.Add("body", "role or status is required")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	var out domain.User
	deactivated := false
	err := s.Store.InTx(ctx, func(tx Tx) error {
		u, err := tx.UserByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		newRole, newStatus := u.Role, u.Status
		if in.Role != nil {
			newRole = *in.Role
		}
		if in.Status != nil {
			newStatus = *in.Status
		}
		if newRole == u.Role && newStatus == u.Status {
			out = *u
			return nil
		}
		losesAdmin := u.Role == domain.RoleAdmin && u.Status == domain.StatusActive &&
			(newRole != domain.RoleAdmin || newStatus != domain.StatusActive)
		if losesAdmin && u.ID == actor.UserID {
			return domain.ErrSelfModification
		}
		if losesAdmin {
			n, err := tx.CountActiveAdmins(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return domain.ErrLastAdmin
			}
		}
		adminActor := events.Actor{UserID: actor.UserID, Role: string(actor.Role)}
		if u.GroupID != "" && (newRole != domain.RoleStudent) {
			g, err := tx.GroupByIDForUpdate(ctx, u.GroupID)
			if err != nil {
				return err
			}
			if err := tx.RemoveMembership(ctx, u.ID); err != nil {
				return err
			}
			if err := s.groupEvent(ctx, tx, adminActor, *g, events.GroupMemberRemoved, u.ID, now); err != nil {
				return err
			}
			u.GroupID, u.GroupCourseID = "", ""
		}
		if err := tx.UpdateUser(ctx, u.ID, newRole, newStatus, now); err != nil {
			return err
		}
		detail := map[string]string{"old_role": string(u.Role), "new_role": string(newRole), "old_status": string(u.Status), "new_status": string(newStatus)}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: actor.UserID, Action: "admin.user_update", TargetType: "user", TargetID: u.ID,
			Outcome: OutcomeSuccess, IP: actor.IP, Detail: detail}); err != nil {
			return err
		}
		if u.Status == domain.StatusActive && newStatus == domain.StatusDeactivated {
			deactivated = true
			if err := s.publish(ctx, tx, events.TypeUserLoggedOut, adminActor, userContext(*u),
				events.UserLoggedOut{UserID: u.ID, Reason: events.LogoutDeactivated}, u.ID, now); err != nil {
				return err
			}
		}
		u.Role, u.Status, u.UpdatedAt = newRole, newStatus, now
		out = *u
		return nil
	})
	if err != nil {
		return nil, err
	}
	if deactivated {
		// Refresh also re-checks the status in Postgres, so this is defence in depth.
		if err := s.Sessions.RevokeAllForUser(ctx, out.ID); err != nil {
			s.Logger.WarnContext(ctx, "revoking sessions of deactivated user failed", slog.String("user_id", out.ID), slog.String("error", err.Error()))
		}
	}
	return &out, nil
}

// GetUser returns any user (admin).
func (s *Service) GetUser(ctx context.Context, actor Actor, id string) (*domain.User, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	return s.Store.UserByID(ctx, id)
}

// ListUsers pages through users by id (admin).
func (s *Service) ListUsers(ctx context.Context, actor Actor, limit int, after string) ([]domain.User, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	return s.Store.ListUsers(ctx, limit, after)
}

// CreateGroupInput creates a group.
type CreateGroupInput struct {
	Name     string
	CourseID string
}

// CreateGroup creates a group and publishes group.updated (created).
func (s *Service) CreateGroup(ctx context.Context, actor Actor, in CreateGroupInput) (*domain.Group, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	var v domain.Validator
	domain.ValidateText(&v, "name", in.Name, domain.MaxGroupNameLen)
	domain.ValidateCourseID(&v, in.CourseID)
	if err := v.Err(); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	g := domain.Group{ID: ids.NewV7At(now), Name: in.Name, CourseID: in.CourseID, CreatedBy: actor.UserID, CreatedAt: now}
	err := s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.InsertGroup(ctx, g); err != nil {
			return err
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: actor.UserID, Action: "admin.group_create", TargetType: "group", TargetID: g.ID, Outcome: OutcomeSuccess, IP: actor.IP}); err != nil {
			return err
		}
		return s.groupEvent(ctx, tx, events.Actor{UserID: actor.UserID, Role: string(actor.Role)}, g, events.GroupCreated, "", now)
	})
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// GroupWithMembers is a group and its members.
type GroupWithMembers struct {
	Group   domain.Group
	Members []domain.User
}

// GetGroup returns a group with its members (admin).
func (s *Service) GetGroup(ctx context.Context, actor Actor, id string) (*GroupWithMembers, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	g, err := s.Store.GroupByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := s.Store.GroupMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	return &GroupWithMembers{Group: *g, Members: m}, nil
}

// ListGroups pages through groups (admin).
func (s *Service) ListGroups(ctx context.Context, actor Actor, limit int, after string) ([]domain.Group, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	return s.Store.ListGroups(ctx, limit, after)
}

// AddMember puts an active student into a group, moving them out of any
// previous group (FR-2/FR-3 minimal). Re-adding is a no-op.
func (s *Service) AddMember(ctx context.Context, actor Actor, groupID, userID string) (*domain.Group, error) {
	if err := actor.requireAdmin(); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	var out domain.Group
	err := s.Store.InTx(ctx, func(tx Tx) error {
		u, err := tx.UserByIDForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if u.Role != domain.RoleStudent || !u.Active() {
			return domain.ErrNotStudent
		}
		// Lock both groups in id order to avoid deadlocks between concurrent moves.
		lockIDs := []string{groupID}
		if u.GroupID != "" && u.GroupID != groupID {
			if u.GroupID < groupID {
				lockIDs = []string{u.GroupID, groupID}
			} else {
				lockIDs = []string{groupID, u.GroupID}
			}
		}
		locked := map[string]*domain.Group{}
		for _, id := range lockIDs {
			g, err := tx.GroupByIDForUpdate(ctx, id)
			if err != nil {
				return err
			}
			locked[id] = g
		}
		target := locked[groupID]
		if u.GroupID == groupID {
			out = *target
			out.MemberCount, err = tx.CountMembers(ctx, groupID)
			return err
		}
		a := events.Actor{UserID: actor.UserID, Role: string(actor.Role)}
		if err := tx.SetMembership(ctx, u.ID, groupID, actor.UserID, now); err != nil {
			return err
		}
		if old, ok := locked[u.GroupID]; ok && u.GroupID != "" {
			if err := s.groupEvent(ctx, tx, a, *old, events.GroupMemberRemoved, u.ID, now); err != nil {
				return err
			}
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: actor.UserID, Action: "admin.group_member_add", TargetType: "user", TargetID: u.ID,
			Outcome: OutcomeSuccess, IP: actor.IP, Detail: map[string]string{"group_id": groupID, "previous_group_id": u.GroupID}}); err != nil {
			return err
		}
		if err := s.groupEvent(ctx, tx, a, *target, events.GroupMemberAdded, u.ID, now); err != nil {
			return err
		}
		n, err := tx.CountMembers(ctx, groupID)
		if err != nil {
			return err
		}
		out = *target
		out.MemberCount = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveMember removes a user from a group.
func (s *Service) RemoveMember(ctx context.Context, actor Actor, groupID, userID string) error {
	if err := actor.requireAdmin(); err != nil {
		return err
	}
	now := s.Clock.Now()
	return s.Store.InTx(ctx, func(tx Tx) error {
		u, err := tx.UserByIDForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if u.GroupID != groupID {
			return domain.ErrNotMember
		}
		g, err := tx.GroupByIDForUpdate(ctx, groupID)
		if err != nil {
			return err
		}
		if err := tx.RemoveMembership(ctx, u.ID); err != nil {
			return err
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: actor.UserID, Action: "admin.group_member_remove", TargetType: "user", TargetID: u.ID,
			Outcome: OutcomeSuccess, IP: actor.IP, Detail: map[string]string{"group_id": groupID}}); err != nil {
			return err
		}
		return s.groupEvent(ctx, tx, events.Actor{UserID: actor.UserID, Role: string(actor.Role)}, *g, events.GroupMemberRemoved, u.ID, now)
	})
}

// groupEvent publishes group.updated keyed by group id; member_count is
// read inside the transaction after the change.
func (s *Service) groupEvent(ctx context.Context, tx Tx, actor events.Actor, g domain.Group, change, memberID string, at time.Time) error {
	n, err := tx.CountMembers(ctx, g.ID)
	if err != nil {
		return err
	}
	p := events.GroupUpdated{GroupID: g.ID, Name: g.Name, CourseID: events.Ptr(g.CourseID), Change: change,
		MemberUserID: events.Ptr(memberID), MemberCount: n}
	return s.publish(ctx, tx, events.TypeGroupUpdated, actor,
		events.Context{CourseID: events.Ptr(g.CourseID), GroupID: events.Ptr(g.ID)}, p, g.ID, at)
}

// IsClientError reports whether err is an expected domain error.
func IsClientError(err error) bool {
	var ve *domain.ValidationError
	var le *domain.LockedError
	return errors.As(err, &ve) || errors.As(err, &le) || errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrEmailTaken) || errors.Is(err, domain.ErrGroupNameTaken) ||
		errors.Is(err, domain.ErrInvalidCredentials) || errors.Is(err, domain.ErrInvalidSession) ||
		errors.Is(err, domain.ErrSelfModification) || errors.Is(err, domain.ErrLastAdmin) ||
		errors.Is(err, domain.ErrNotStudent) || errors.Is(err, domain.ErrNotMember) || errors.Is(err, domain.ErrForbidden)
}
