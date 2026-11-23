// Package apptest provides in-memory fakes of the app ports for tests.
package apptest

import (
	"context"
	"encoding/json"
	"github.com/Newterios/Examory/services/auth/internal/app"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/auth/internal/password"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
)

// Published is an event written to the fake outbox.
type Published struct {
	Topic, Key string
	Env        events.Envelope
}

// Store implements Store and app.Tx in memory with rollback on error.
type Store struct {
	txMu   sync.Mutex
	mu     sync.Mutex
	users  map[string]domain.User
	groups map[string]domain.Group
	member map[string]string // user -> group
	audit  []app.AuditEntry
	Out    []Published
}

func NewStore() *Store {
	return &Store{users: map[string]domain.User{}, groups: map[string]domain.Group{}, member: map[string]string{}}
}

type snapshot struct {
	users  map[string]domain.User
	groups map[string]domain.Group
	member map[string]string
	audit  []app.AuditEntry
	Out    []Published
}

func (s *Store) InTx(ctx context.Context, fn func(app.Tx) error) error {
	s.txMu.Lock()
	defer s.txMu.Unlock()
	s.mu.Lock()
	snap := snapshot{maps.Clone(s.users), maps.Clone(s.groups), maps.Clone(s.member), append([]app.AuditEntry(nil), s.audit...), append([]Published(nil), s.Out...)}
	s.mu.Unlock()
	if err := fn(s); err != nil {
		s.mu.Lock()
		s.users, s.groups, s.member, s.audit, s.Out = snap.users, snap.groups, snap.member, snap.audit, snap.Out
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Store) withGroup(u domain.User) domain.User {
	u.GroupID = s.member[u.ID]
	u.GroupCourseID = s.groups[u.GroupID].CourseID
	return u
}

func (s *Store) UserByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Email == email {
			x := s.withGroup(u)
			return &x, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *Store) UserByID(_ context.Context, id string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	x := s.withGroup(u)
	return &x, nil
}

func (s *Store) UserByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return s.UserByID(ctx, id)
}

func (s *Store) ListUsers(_ context.Context, limit int, after string) ([]domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.User
	for _, u := range s.users {
		if u.ID > after {
			out = append(out, s.withGroup(u))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) countLocked(gid string) int {
	n := 0
	for _, g := range s.member {
		if g == gid {
			n++
		}
	}
	return n
}

func (s *Store) GroupByID(_ context.Context, id string) (*domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	g.MemberCount = s.countLocked(id)
	return &g, nil
}

func (s *Store) GroupByIDForUpdate(ctx context.Context, id string) (*domain.Group, error) {
	return s.GroupByID(ctx, id)
}

func (s *Store) ListGroups(context.Context, int, string) ([]domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Group
	for _, g := range s.groups {
		out = append(out, g)
	}
	return out, nil
}

func (s *Store) GroupMembers(_ context.Context, gid string) ([]domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.User
	for uid, g := range s.member {
		if g == gid {
			out = append(out, s.withGroup(s.users[uid]))
		}
	}
	return out, nil
}

func (s *Store) InsertUser(_ context.Context, u domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.users {
		if x.Email == u.Email {
			return domain.ErrEmailTaken
		}
	}
	s.users[u.ID] = u
	return nil
}

func (s *Store) UpdateUser(_ context.Context, id string, role domain.Role, st domain.Status, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	if role != domain.RoleStudent && s.member[id] != "" {
		return domain.ErrNotStudent // mirrors the DB trigger
	}
	u.Role, u.Status, u.UpdatedAt = role, st, at
	s.users[id] = u
	return nil
}

func (s *Store) UpdatePasswordHash(_ context.Context, id, hash string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[id]
	u.PasswordHash, u.UpdatedAt = hash, at
	s.users[id] = u
	return nil
}

func (s *Store) CountActiveAdmins(context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, u := range s.users {
		if u.Role == domain.RoleAdmin && u.Active() {
			n++
		}
	}
	return n, nil
}

func (s *Store) LockBootstrap(context.Context) error { return nil }

func (s *Store) InsertGroup(_ context.Context, g domain.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.groups {
		if x.Name == g.Name {
			return domain.ErrGroupNameTaken
		}
	}
	s.groups[g.ID] = g
	return nil
}

func (s *Store) SetMembership(_ context.Context, uid, gid, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users[uid].Role != domain.RoleStudent {
		return domain.ErrNotStudent
	}
	s.member[uid] = gid
	return nil
}

func (s *Store) RemoveMembership(_ context.Context, uid string) error {
	s.mu.Lock()
	delete(s.member, uid)
	s.mu.Unlock()
	return nil
}

func (s *Store) CountMembers(_ context.Context, gid string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countLocked(gid), nil
}

func (s *Store) Audit(_ context.Context, e app.AuditEntry) error {
	s.mu.Lock()
	s.audit = append(s.audit, e)
	s.mu.Unlock()
	return nil
}

func (s *Store) Publish(_ context.Context, topic, key string, env events.Envelope) error {
	s.mu.Lock()
	s.Out = append(s.Out, Published{topic, key, env})
	s.mu.Unlock()
	return nil
}

func (s *Store) EventsOfType(t string) []Published {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Published
	for _, p := range s.Out {
		if p.Env.EventType == t {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) AuditActions() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[string]int{}
	for _, a := range s.audit {
		m[a.Action+":"+a.Outcome]++
	}
	return m
}

// Sessions mirrors the Redis refresh-store semantics.
type Sessions struct {
	mu     sync.Mutex
	tokens map[string]*ftok
	fams   map[string]*ffam
}

type ftok struct {
	fam, uid string
	used     bool
}
type ffam struct {
	uid     string
	revoked bool
}

func NewSessions() *Sessions {
	return &Sessions{tokens: map[string]*ftok{}, fams: map[string]*ffam{}}
}

func (f *Sessions) Create(_ context.Context, uid, fam, h string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fams[fam] = &ffam{uid: uid}
	f.tokens[h] = &ftok{fam: fam, uid: uid}
	return nil
}

func (f *Sessions) Rotate(_ context.Context, old, nw string, _ time.Time) (app.RefreshResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[old]
	if !ok {
		return app.RefreshResult{Outcome: app.RefreshInvalid}, nil
	}
	fam := f.fams[t.fam]
	r := app.RefreshResult{UserID: t.uid, FamilyID: t.fam}
	switch {
	case fam.revoked:
		r.Outcome = app.RefreshRevoked
	case t.used:
		fam.revoked = true
		r.Outcome = app.RefreshReused
	default:
		t.used = true
		f.tokens[nw] = &ftok{fam: t.fam, uid: t.uid}
		r.Outcome = app.RefreshOK
	}
	return r, nil
}

func (f *Sessions) Revoke(_ context.Context, h string) (string, string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[h]
	if !ok || f.fams[t.fam].revoked {
		return "", "", false, nil
	}
	f.fams[t.fam].revoked = true
	return t.uid, t.fam, true, nil
}

func (f *Sessions) RevokeFamily(_ context.Context, fam string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if x, ok := f.fams[fam]; ok {
		x.revoked = true
	}
	return nil
}

func (f *Sessions) RevokeAllForUser(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.fams {
		if x.uid == uid {
			x.revoked = true
		}
	}
	return nil
}

// Lockout uses the fake clock.
type Lockout struct {
	mu    sync.Mutex
	clk   clock.Clock
	Fails map[string]int
	until map[string]time.Time
}

// NewLockout returns a lockout fake driven by clk.
func NewLockout(clk clock.Clock) *Lockout {
	return &Lockout{clk: clk, Fails: map[string]int{}, until: map[string]time.Time{}}
}

func (l *Lockout) LockedUntil(_ context.Context, k string) (time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if u := l.until[k]; u.After(l.clk.Now()) {
		return u, nil
	}
	return time.Time{}, nil
}
func (l *Lockout) Failure(_ context.Context, k string, _ time.Duration) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Fails[k]++
	return l.Fails[k], nil
}
func (l *Lockout) Lock(_ context.Context, k string, d time.Duration) error {
	l.mu.Lock()
	l.until[k] = l.clk.Now().Add(d)
	l.mu.Unlock()
	return nil
}
func (l *Lockout) Reset(_ context.Context, k string) error {
	l.mu.Lock()
	delete(l.Fails, k)
	delete(l.until, k)
	l.mu.Unlock()
	return nil
}

// SpyHasher wraps a real (cheap) Argon2id hasher and records verified hashes.
type SpyHasher struct {
	*password.Hasher
	mu       sync.Mutex
	verified []string
}

func (h *SpyHasher) Verify(ctx context.Context, pw, phc string) (bool, bool, error) {
	h.mu.Lock()
	h.verified = append(h.verified, phc)
	h.mu.Unlock()
	return h.Hasher.Verify(ctx, pw, phc)
}

func (h *SpyHasher) LastVerified() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.verified) == 0 {
		return ""
	}
	return h.verified[len(h.verified)-1]
}

func (h *SpyHasher) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.verified)
}

type Tokens struct{ Clk clock.Clock }

func (t Tokens) Issue(uid string, role domain.Role, gid string) (string, time.Time, error) {
	b, _ := json.Marshal(map[string]string{"sub": uid, "role": string(role), "group_id": gid})
	return string(b), t.Clk.Now().Add(10 * time.Minute), nil
}
