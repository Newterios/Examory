package inbox

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrations() []migrate.Set {
	f, e := fs.Sub(migrationFiles, "migrations")
	if e != nil {
		panic(e)
	}
	return []migrate.Set{consumer.Migrations(), {Name: "notifications", FS: f}}
}

type payload struct {
	Attempt    string     `json:"attempt_id"`
	Submission string     `json:"submission_id"`
	Assignment string     `json:"assignment_id"`
	TargetType string     `json:"target_type"`
	Target     string     `json:"target_id"`
	Decision   string     `json:"decision"`
	Due        *time.Time `json:"due_at"`
}
type Notice struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Course     string     `json:"course_id"`
	TargetType string     `json:"target_type"`
	Target     string     `json:"target_id"`
	Occurred   time.Time  `json:"occurred_at"`
	Due        *time.Time `json:"due_at,omitempty"`
	Decision   *string    `json:"decision,omitempty"`
}
type action struct {
	notice Notice
	owner  string
	group  *string
}

func project(topic string, env events.Envelope) (action, bool, error) {
	var a action
	if env.Version != 1 {
		return a, false, nil
	}
	routes := map[string][2]string{"quiz.started": {events.TopicQuizzes, "quizzes"}, "submission.created": {events.TopicSubmissions, "assignments"}, "assignment.published": {events.TopicCourses, "assignments"}, "grade.assigned": {events.TopicGrades, "grading"}, "risk.reviewed": {events.TopicGrades, "grading"}}
	route, ok := routes[env.EventType]
	if !ok {
		return a, false, nil
	}
	if route[0] != topic || route[1] != env.Producer {
		return a, false, consumer.ErrPermanent
	}
	var p payload
	if env.Context.CourseID == nil || *env.Context.CourseID == "" || json.Unmarshal(env.Payload, &p) != nil {
		return a, false, consumer.ErrPermanent
	}
	n := Notice{ID: env.EventID, Course: *env.Context.CourseID, Occurred: env.OccurredAt.Time, TargetType: p.TargetType, Target: p.Target}
	switch env.EventType {
	case "quiz.started", "submission.created":
		if env.Actor.Role != "student" {
			return a, false, consumer.ErrPermanent
		}
		a.owner = env.Actor.UserID
		if env.EventType == "quiz.started" {
			n.TargetType = "attempt"
			n.Target = p.Attempt
		} else {
			n.TargetType = "submission"
			n.Target = p.Submission
		}
	case "assignment.published":
		if p.Due == nil {
			return a, false, nil
		}
		if env.Context.GroupID == nil || *env.Context.GroupID == "" {
			return a, false, consumer.ErrPermanent
		}
		n.Kind = "deadline"
		n.TargetType = "assignment"
		n.Target = p.Assignment
		n.Due = p.Due
		a.group = env.Context.GroupID
	case "grade.assigned", "risk.reviewed":
		if n.TargetType != "submission" && n.TargetType != "attempt" {
			return a, false, consumer.ErrPermanent
		}
		if env.EventType == "grade.assigned" {
			n.Kind = "grade"
		} else {
			if p.Decision != "confirmed" && p.Decision != "dismissed" {
				return a, false, consumer.ErrPermanent
			}
			n.Kind = "review"
			n.Decision = &p.Decision
		}
	}
	if n.Target == "" {
		return a, false, consumer.ErrPermanent
	}
	a.notice = n
	return a, true, nil
}
func Handle(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	return Apply(ctx, tx, consumer.TopicFromContext(ctx), env)
}
func Apply(ctx context.Context, tx pgx.Tx, topic string, env events.Envelope) error {
	a, ok, err := project(topic, env)
	if !ok || err != nil {
		return err
	}
	n := a.notice
	if a.owner != "" {
		// A later record cannot replace the original owner.
		tag, e := tx.Exec(ctx, `INSERT INTO notification_owners VALUES($1,$2,$3,$4)
    ON CONFLICT(target_type,target_id) DO UPDATE SET student_id=EXCLUDED.student_id
    WHERE notification_owners.student_id=EXCLUDED.student_id AND notification_owners.course_id=EXCLUDED.course_id`, n.TargetType, n.Target, n.Course, a.owner)
		if e == nil && tag.RowsAffected() == 0 {
			return consumer.ErrPermanent
		}
		return e
	}
	_, err = tx.Exec(ctx, `INSERT INTO notifications(event_id,kind,course_id,group_id,target_type,target_id,occurred_at,due_at,decision)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(event_id) DO NOTHING`, n.ID, n.Kind, n.Course, a.group, n.TargetType, n.Target, n.Occurred, n.Due, n.Decision)
	return err
}

type Store struct {
	Pool    *pgxpool.Pool
	Courses courseaccess.Checker
}

// List uses the verified subject and group. Origin records can arrive after grade events.
func (s Store) List(ctx context.Context, p authn.Principal, before string, limit int) ([]Notice, string, error) {
	if p.Role != authn.RoleStudent {
		return nil, "", errors.New("student required")
	}
	rows, err := s.Pool.Query(ctx, `SELECT n.event_id::text,n.kind,n.course_id,n.target_type,n.target_id,n.occurred_at,n.due_at,n.decision
 FROM notifications n WHERE (NULLIF($3,'')::uuid IS NULL OR n.event_id<NULLIF($3,'')::uuid)
 AND ((n.kind='deadline' AND n.group_id=$2) OR (n.kind IN ('grade','review') AND EXISTS
 (SELECT 1 FROM notification_owners o WHERE o.target_type=n.target_type AND o.target_id=n.target_id AND o.course_id=n.course_id AND o.student_id=$1)))
 ORDER BY n.event_id DESC LIMIT $4`, p.UserID, p.GroupID, before, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	page := []Notice{}
	for rows.Next() {
		var n Notice
		if e := rows.Scan(&n.ID, &n.Kind, &n.Course, &n.TargetType, &n.Target, &n.Occurred, &n.Due, &n.Decision); e != nil {
			return nil, "", e
		}
		page = append(page, n)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(page) > limit {
		next = page[limit-1].ID
		page = page[:limit]
	}
	result := []Notice{}
	permissions := map[string]bool{}
	for _, n := range page {
		if n.Kind == "deadline" {
			ok, cached := permissions[n.Course]
			if !cached {
				ok, err = s.Courses.Allowed(ctx, n.Course, "", p.GroupID)
				if err != nil {
					return nil, "", err
				}
				permissions[n.Course] = ok
			}
			if !ok {
				continue
			}
		}
		result = append(result, n)
	}
	return result, next, nil
}
