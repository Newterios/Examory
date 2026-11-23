// Package postgres implements app.Store with pgx. All SQL is static and
// parameterized.
package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the auth schema migration set.
func Migrations() migrate.Set {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return migrate.Set{Name: "auth", FS: sub}
}

// Store implements app.Store.
type Store struct{ pool *pgxpool.Pool }

// New returns a Store.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

const userCols = `u.id, u.email, u.display_name, u.role, u.status, u.password_hash, COALESCE(m.group_id::text, ''), COALESCE(g.course_id, ''), u.created_at, u.updated_at`
const userFrom = ` FROM users u LEFT JOIN group_members m ON m.user_id = u.id LEFT JOIN groups g ON g.id = m.group_id `

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var role, status string
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &role, &status, &u.PasswordHash, &u.GroupID, &u.GroupCourseID, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Role, u.Status = domain.Role(role), domain.Status(status)
	u.CreatedAt, u.UpdatedAt = u.CreatedAt.UTC(), u.UpdatedAt.UTC()
	return &u, nil
}

func userByID(ctx context.Context, q querier, id string, forUpdate bool) (*domain.User, error) {
	if !ids.Valid(id) {
		return nil, domain.ErrNotFound
	}
	sql := `SELECT ` + userCols + userFrom + `WHERE u.id = $1`
	if forUpdate {
		sql += ` FOR UPDATE OF u`
	}
	return scanUser(q.QueryRow(ctx, sql, id))
}

func collectUsers(rows pgx.Rows, err error) ([]domain.User, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UserByEmail implements app.Store.
func (s *Store) UserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+userFrom+`WHERE u.email = $1`, email))
}

// UserByID implements app.Store.
func (s *Store) UserByID(ctx context.Context, id string) (*domain.User, error) {
	return userByID(ctx, s.pool, id, false)
}

// ListUsers implements app.Store (keyset pagination over UUIDv7 ids).
func (s *Store) ListUsers(ctx context.Context, limit int, after string) ([]domain.User, error) {
	return collectUsers(s.pool.Query(ctx,
		`SELECT `+userCols+userFrom+`WHERE ($2::uuid IS NULL OR u.id > $2::uuid) ORDER BY u.id LIMIT $1`, limit, nullable(after)))
}

const groupSelect = `SELECT g.id, g.name, COALESCE(g.course_id, ''), g.created_by, g.created_at,
	(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) FROM groups g `

func scanGroup(row pgx.Row) (*domain.Group, error) {
	var g domain.Group
	err := row.Scan(&g.ID, &g.Name, &g.CourseID, &g.CreatedBy, &g.CreatedAt, &g.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	g.CreatedAt = g.CreatedAt.UTC()
	return &g, err
}

// GroupByID implements app.Store.
func (s *Store) GroupByID(ctx context.Context, id string) (*domain.Group, error) {
	if !ids.Valid(id) {
		return nil, domain.ErrNotFound
	}
	return scanGroup(s.pool.QueryRow(ctx, groupSelect+`WHERE g.id = $1`, id))
}

// ListGroups implements app.Store.
func (s *Store) ListGroups(ctx context.Context, limit int, after string) ([]domain.Group, error) {
	rows, err := s.pool.Query(ctx, groupSelect+`WHERE ($2::uuid IS NULL OR g.id > $2::uuid) ORDER BY g.id LIMIT $1`, limit, nullable(after))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Group{}
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

// GroupMembers implements app.Store (bounded: at most 1000 members listed).
func (s *Store) GroupMembers(ctx context.Context, groupID string) ([]domain.User, error) {
	return collectUsers(s.pool.Query(ctx, `SELECT `+userCols+userFrom+`WHERE m.group_id = $1 ORDER BY u.id LIMIT 1000`, groupID))
}

// InTx implements app.Store.
func (s *Store) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error { return fn(&txAdapter{tx: tx}) })
}

type txAdapter struct{ tx pgx.Tx }

func (t *txAdapter) InsertUser(ctx context.Context, u domain.User) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO users (id, email, display_name, role, status, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, u.ID, u.Email, u.DisplayName, string(u.Role), string(u.Status), u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	if db.IsUniqueViolation(err, "users_email_key") {
		return domain.ErrEmailTaken
	}
	return err
}

func (t *txAdapter) UserByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return userByID(ctx, t.tx, id, true)
}

func (t *txAdapter) UpdateUser(ctx context.Context, id string, role domain.Role, status domain.Status, at time.Time) error {
	_, err := t.tx.Exec(ctx, `UPDATE users SET role = $2, status = $3, updated_at = $4,
		deactivated_at = CASE WHEN $3 = 'deactivated' THEN COALESCE(deactivated_at, $4) ELSE NULL END WHERE id = $1`,
		id, string(role), string(status), at)
	return err
}

func (t *txAdapter) UpdatePasswordHash(ctx context.Context, id, hash string, at time.Time) error {
	_, err := t.tx.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`, id, hash, at)
	return err
}

func (t *txAdapter) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := t.tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND status = 'active'`).Scan(&n)
	return n, err
}

// bootstrapLockKey is the transaction-scoped advisory lock for bootstrap.
const bootstrapLockKey int64 = 0x41555448424f4f54 // "AUTHBOOT"

func (t *txAdapter) LockBootstrap(ctx context.Context) error {
	_, err := t.tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapLockKey)
	return err
}

func (t *txAdapter) InsertGroup(ctx context.Context, g domain.Group) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO groups (id, name, course_id, created_by, created_at) VALUES ($1, $2, $3, $4, $5)`,
		g.ID, g.Name, nullable(g.CourseID), g.CreatedBy, g.CreatedAt)
	if db.IsUniqueViolation(err, "groups_name_key") {
		return domain.ErrGroupNameTaken
	}
	return err
}

func (t *txAdapter) GroupByIDForUpdate(ctx context.Context, id string) (*domain.Group, error) {
	if !ids.Valid(id) {
		return nil, domain.ErrNotFound
	}
	var g domain.Group
	err := t.tx.QueryRow(ctx, `SELECT id, name, COALESCE(course_id, ''), created_by, created_at FROM groups WHERE id = $1 FOR UPDATE`, id).
		Scan(&g.ID, &g.Name, &g.CourseID, &g.CreatedBy, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return &g, err
}

func (t *txAdapter) SetMembership(ctx context.Context, userID, groupID, addedBy string, at time.Time) error {
	_, err := t.tx.Exec(ctx, `INSERT INTO group_members (user_id, group_id, added_by, added_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET group_id = EXCLUDED.group_id, added_by = EXCLUDED.added_by, added_at = EXCLUDED.added_at`,
		userID, groupID, addedBy, at)
	if code, _, ok := db.ErrorCode(err); ok && code == db.CodeCheckViolation {
		return domain.ErrNotStudent
	}
	return err
}

func (t *txAdapter) RemoveMembership(ctx context.Context, userID string) error {
	_, err := t.tx.Exec(ctx, `DELETE FROM group_members WHERE user_id = $1`, userID)
	return err
}

func (t *txAdapter) CountMembers(ctx context.Context, groupID string) (int, error) {
	var n int
	err := t.tx.QueryRow(ctx, `SELECT count(*) FROM group_members WHERE group_id = $1`, groupID).Scan(&n)
	return n, err
}

func (t *txAdapter) Audit(ctx context.Context, e app.AuditEntry) error {
	detail := e.Detail
	if detail == nil {
		detail = map[string]string{}
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(ctx, `INSERT INTO audit_log (id, at, actor_id, action, target_type, target_id, outcome, ip, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CAST(NULLIF($8::text, '') AS inet), $9::jsonb)`,
		ids.NewV7At(e.At), e.At, nullable(e.ActorID), e.Action, nullable(e.TargetType), nullable(e.TargetID), e.Outcome, e.IP, string(b))
	return err
}

func (t *txAdapter) Publish(ctx context.Context, topic, key string, env events.Envelope) error {
	return outbox.Enqueue(ctx, t.tx, topic, key, env)
}
