//go:build integration

package inbox

import (
	"context"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/testpg"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"
)

func TestLateOwnershipPrivacyAndDedup(t *testing.T) {
	ctx := context.Background()
	pool, e := db.Open(ctx, db.Config{URL: testpg.New(t)})
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = migrate.Up(ctx, pool, logging.Discard(), Migrations()...); e != nil {
		t.Fatal(e)
	}
	a := authn.Principal{UserID: ids.NewV7(), Role: "student", GroupID: ids.NewV7()}
	b := a
	b.UserID = ids.NewV7()
	course, att := ids.NewV7(), ids.NewV7()
	allowed := true
	store := Store{pool, courseaccess.CheckFunc(func(context.Context, string, string, string) (bool, error) { return allowed, nil })}
	event := func(kind, producer, role, actor string, p any) events.Envelope {
		v, err := events.New(kind, 1, producer, events.Actor{UserID: actor, Role: role}, events.Context{CourseID: &course, GroupID: &a.GroupID}, p, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	apply := func(topic string, env events.Envelope) bool {
		ran, e := consumer.HandleOnce(ctx, pool, "notifications-v1", env, func(ctx context.Context, tx pgx.Tx, v events.Envelope) error { return Apply(ctx, tx, topic, v) })
		if e != nil {
			t.Fatal(e)
		}
		return ran
	}
	count := func(p authn.Principal, want int) {
		page, _, e := store.List(ctx, p, "", 10)
		if e != nil || len(page) != want {
			t.Fatal(page, e, want)
		}
	}
	grade := event("grade.assigned", "grading", "teacher", ids.NewV7(), map[string]any{"target_type": "attempt", "target_id": att, "grade": 42})
	if !apply(events.TopicGrades, grade) || apply(events.TopicGrades, grade) {
		t.Fatal("dedup")
	}
	count(a, 0)
	count(b, 0)
	apply(events.TopicQuizzes, event("quiz.started", "quizzes", "student", a.UserID, map[string]string{"attempt_id": att}))
	count(a, 1)
	count(b, 0)
	apply(events.TopicCourses, event("assignment.published", "assignments", "teacher", ids.NewV7(), map[string]any{"assignment_id": ids.NewV7(), "due_at": time.Now().Add(time.Hour)}))
	count(a, 2)
	count(b, 1)
	allowed = false
	count(a, 1)
	count(b, 0)
}
