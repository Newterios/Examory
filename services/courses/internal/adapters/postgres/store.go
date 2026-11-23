package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"time"

	"github.com/Newterios/Examory/services/courses/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrations() []migrate.Set {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic(err)
	}
	return []migrate.Set{outbox.Migrations(), {Name: "courses", FS: sub}}
}

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool} }

const fields = "c.id::text,c.title,c.status,c.revision,c.modules"
const visibility = `($1='admin' OR ($1='teacher' AND EXISTS(SELECT 1 FROM course_members m WHERE m.course_id=c.id AND m.kind='teacher' AND m.member_id=NULLIF($2,'')::uuid))
 OR ($1='student' AND c.status='published' AND EXISTS(SELECT 1 FROM course_members m WHERE m.course_id=c.id AND m.kind='group' AND m.member_id=NULLIF($3,'')::uuid)))`

func scan(row pgx.Row) (domain.Course, error) {
	var c domain.Course
	err := row.Scan(&c.ID, &c.Title, &c.Status, &c.Revision, &c.Modules)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	return c, err
}
func emit(ctx context.Context, tx pgx.Tx, p authn.Principal, cid, kind string, payload any) error {
	env, err := events.New(kind, 1, "courses", events.Actor{UserID: p.UserID, Role: string(p.Role)}, events.Context{CourseID: events.Ptr(cid), GroupID: events.Ptr(p.GroupID)}, payload, time.Now())
	if err != nil {
		return err
	}
	return outbox.Enqueue(ctx, tx, events.TopicCourses, cid, env)
}
func changed(ctx context.Context, tx pgx.Tx, p authn.Principal, c domain.Course, kind string) error {
	return emit(ctx, tx, p, c.ID, kind, map[string]any{"course_id": c.ID, "status": c.Status, "revision": c.Revision})
}
func (s *Store) Create(ctx context.Context, p authn.Principal, title string) (domain.Course, error) {
	c := domain.Course{ID: ids.NewV7(), Title: title, Revision: 1, Status: "draft", Modules: []domain.Module{}}
	if !p.Is(authn.RoleTeacher, authn.RoleAdmin) {
		return c, domain.ErrForbidden
	}
	if !domain.Title(title) {
		return c, domain.ErrInvalid
	}
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "INSERT INTO courses(id,title,status,revision) VALUES($1,$2,'draft',1)", c.ID, c.Title); e != nil {
			return e
		}
		if p.Role == authn.RoleTeacher {
			if _, e := tx.Exec(ctx, "INSERT INTO course_members VALUES($1,'teacher',$2)", c.ID, p.UserID); e != nil {
				return e
			}
		}
		return changed(ctx, tx, p, c, "course.created")
	})
	return c, err
}
func (s *Store) List(ctx context.Context, p authn.Principal, cursor string, n int) ([]domain.Course, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+fields+" FROM courses c WHERE "+visibility+" AND (NULLIF($4,'')::uuid IS NULL OR c.id>NULLIF($4,'')::uuid) ORDER BY c.id LIMIT $5", string(p.Role), p.UserID, p.GroupID, cursor, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []domain.Course{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		if p.Role == authn.RoleStudent {
			c = c.Published()
		}
		res = append(res, c)
	}
	return res, rows.Err()
}
func (s *Store) Get(ctx context.Context, p authn.Principal, id string) (domain.Course, error) {
	c, err := scan(s.pool.QueryRow(ctx, "SELECT "+fields+" FROM courses c WHERE "+visibility+" AND c.id=$4", string(p.Role), p.UserID, p.GroupID, id))
	if err == nil && p.Role == authn.RoleStudent {
		c = c.Published()
	}
	return c, err
}
func lock(ctx context.Context, tx pgx.Tx, id string) (domain.Course, error) {
	return scan(tx.QueryRow(ctx, "SELECT "+fields+" FROM courses c WHERE c.id=$1 FOR UPDATE", id))
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func member(ctx context.Context, q querier, cid, kind, id string) (bool, error) {
	var ok bool
	if id == "" {
		return false, nil
	}
	err := q.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM course_members WHERE course_id=$1 AND kind=$2 AND member_id=$3)", cid, kind, id).Scan(&ok)
	return ok, err
}
func (s *Store) Access(ctx context.Context, cid, kind, id string) (bool, error) {
	return member(ctx, s.pool, cid, kind, id)
}
func (s *Store) TeacherCourses(ctx context.Context, id string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT course_id::text FROM course_members WHERE kind='teacher' AND member_id=$1 ORDER BY course_id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Update(ctx context.Context, p authn.Principal, id string, in domain.Course) (domain.Course, error) {
	if !p.Is(authn.RoleTeacher, authn.RoleAdmin) {
		return in, domain.ErrForbidden
	}
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		old, e := lock(ctx, tx, id)
		if e != nil {
			return e
		}
		if p.Role == authn.RoleTeacher {
			ok, e := member(ctx, tx, id, "teacher", p.UserID)
			if e != nil {
				return e
			}
			if !ok {
				return domain.ErrNotFound
			}
		}
		if in.Revision != old.Revision {
			return domain.ErrConflict
		}
		if in.ID != "" && in.ID != id {
			return domain.ErrInvalid
		}
		if e = in.Prepare(old); e != nil {
			return e
		}
		in.ID = id
		in.Revision++
		blob, e := json.Marshal(in.Modules)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE courses SET status=$2,title=$3,modules=$4,revision=$5 WHERE id=$1", id, in.Status, in.Title, blob, in.Revision); e != nil {
			return e
		}
		return changed(ctx, tx, p, in, "course.updated")
	})
	return in, err
}
func (s *Store) Assign(ctx context.Context, p authn.Principal, cid, kind, id string, add bool) error {
	if p.Role != authn.RoleAdmin {
		return domain.ErrForbidden
	}
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		c, e := lock(ctx, tx, cid)
		if e != nil {
			return e
		}
		stmt := "DELETE FROM course_members WHERE course_id=$1 AND kind=$2 AND member_id=$3"
		if add {
			stmt = "INSERT INTO course_members VALUES($1,$2,$3) ON CONFLICT DO NOTHING"
		}
		r, e := tx.Exec(ctx, stmt, cid, kind, id)
		if e != nil || r.RowsAffected() == 0 {
			return e
		}
		c.Revision++
		if _, e = tx.Exec(ctx, "UPDATE courses SET revision=$2 WHERE id=$1", cid, c.Revision); e != nil {
			return e
		}
		return changed(ctx, tx, p, c, "course.updated")
	})
}
func (s *Store) View(ctx context.Context, p authn.Principal, cid, mid string, duration int64, complete bool) (bool, error) {
	if p.Role != authn.RoleStudent {
		return false, domain.ErrForbidden
	}
	if duration < 0 {
		return false, domain.ErrInvalid
	}
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		c, e := lock(ctx, tx, cid)
		if e != nil {
			return e
		}
		ok, e := member(ctx, tx, cid, "group", p.GroupID)
		if e != nil {
			return e
		}
		module, published := c.FindPublished(mid)
		if !ok || !published {
			return domain.ErrNotFound
		}
		if complete {
			_, e = tx.Exec(ctx, "INSERT INTO material_completions VALUES($1,$2,$3) ON CONFLICT DO NOTHING", cid, mid, p.UserID)
		} else {
			e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM material_completions WHERE course_id=$1 AND material_id=$2 AND student_id=$3)", cid, mid, p.UserID).Scan(&complete)
		}
		if e != nil {
			return e
		}
		return emit(ctx, tx, p, cid, "material.viewed", map[string]any{"material_id": mid, "module_id": module, "view_duration_ms": duration, "completed": complete})
	})
	return complete, err
}
