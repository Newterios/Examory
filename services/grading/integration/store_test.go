//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Newterios/Examory/services/grading/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/grading/internal/adapters/schemas"
	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/testpg"
)

// TestStoreAgainstPostgres exercises the pgx adapter, the schema triggers and
// consumer-path ingestion (HandleOnce + Bind) against a real database.
func TestStoreAgainstPostgres(t *testing.T) {
	url := testpg.New(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{URL: url, ApplicationName: "grading-it", MaxConns: 30})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for i := 0; i < 2; i++ {
		if err := migrate.Up(ctx, pool, logging.Discard(), postgres.Migrations()...); err != nil {
			t.Fatalf("migrate #%d: %v", i, err)
		}
	}
	val, _ := schemas.Validator()
	svc := app.New(postgres.New(pool), val, clock.System{}, logging.Discard(), courseaccess.CheckFunc(func(context.Context, string, string, string) (bool, error) { return true, nil }))
	handle := func(topic string, env events.Envelope) bool {
		ran, err := consumer.HandleOnce(ctx, pool, "grading", env, func(ctx context.Context, tx pgx.Tx, e events.Envelope) error {
			return svc.Ingest(ctx, postgres.Bind(tx), e, topic)
		})
		if err != nil {
			t.Fatal(err)
		}
		return ran
	}
	course, group, attempt, quiz, student := "c_it", ids.NewV7(), ids.NewV7(), ids.NewV7(), ids.NewV7()
	sub, _ := events.New(events.TypeQuizSubmitted, 1, "quizzes", events.Actor{UserID: student, Role: "student"},
		events.Context{CourseID: &course, GroupID: &group},
		events.QuizSubmitted{AttemptID: attempt, QuizID: quiz, VariantID: ids.NewV7(), Score: 17, MaxScore: 20, DurationMs: 9000, AnswersChanged: 0, Reason: "submitted"}, time.Now())
	riskPayload := map[string]any{"risk_id": "risk_" + strings.ReplaceAll(attempt, "-", ""), "target_type": "attempt", "target_id": attempt,
		"score": 0.98, "level": "high", "model": map[string]any{"name": "isolation-forest-v1", "version": "0.1.0"},
		"explanation": "mean_answer_time_ms=700 (below normal-train 1–99% range)",
		"evidence":    map[string]any{"features": map[string]any{"mean_answer_time_ms": 700.0, "z_score": nil, "synchronous_with": []string{ids.NewV7()}}}}
	risk, _ := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"},
		events.Context{CourseID: &course, GroupID: &group}, riskPayload, time.Now())
	if !handle(events.TopicQuizzes, sub) || !handle(events.TopicRisk, risk) {
		t.Fatal("first delivery must run the handler")
	}
	if handle(events.TopicRisk, risk) || handle(events.TopicQuizzes, sub) {
		t.Fatal("re-delivery of the same event_id must be skipped")
	}
	again, _ := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"},
		events.Context{CourseID: &course, GroupID: &group}, riskPayload, time.Now())
	handle(events.TopicRisk, again) // same risk_id, new event_id: no second signal
	bad := map[string]any{}
	for k, v := range riskPayload {
		bad[k] = v
	}
	bad["risk_id"], bad["score"] = "risk_bad", 3.5
	badEnv, _ := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"}, events.Context{}, bad, time.Now())
	handle(events.TopicRisk, badEnv)
	svc.RecordRejected(ctx, app.Rejected{Topic: events.TopicRisk, Partition: 2, Offset: 41, Reason: "decode envelope", Excerpt: "\xff\xfe broken"})

	teacher := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher}
	rows, err := svc.ListRisks(ctx, teacher, app.Filter{QuizID: quiz, Status: "open", CourseID: course}, 10, "")
	if err != nil || len(rows) != 1 || rows[0].Attempt == nil || rows[0].Attempt.Score != 17 {
		t.Fatalf("list %+v %v", rows, err)
	}
	id := rows[0].Risk.ID
	row, err := svc.GetRisk(ctx, teacher, id)
	if err != nil || row.Risk.Version != 1 || !strings.Contains(string(row.Risk.Evidence), "synchronous_with") {
		t.Fatalf("get %+v %v", row, err)
	}
	// Concurrent reviews by several teachers: exactly one wins.
	var wg sync.WaitGroup
	results := make([]error, 12)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher}
			if i == 0 {
				p = teacher
			}
			_, results[i] = svc.Review(ctx, p, id, []string{"confirmed", "dismissed"}[i%2], fmt.Sprintf("reason %d", i), 1, fmt.Sprintf("it-key-%04d", i))
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrAlreadyReviewed), errors.Is(err, domain.ErrVersionMismatch):
		default:
			t.Fatalf("unexpected review error: %v", err)
		}
	}
	count := func(sql string, args ...any) int {
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if ok != 1 || count(`SELECT count(*) FROM reviews`) != 1 || count(`SELECT count(*) FROM outbox WHERE event_type='risk.reviewed'`) != 1 {
		t.Fatalf("ok=%d reviews=%d", ok, count(`SELECT count(*) FROM reviews`))
	}
	if count(`SELECT count(*) FROM risks`) != 1 || count(`SELECT count(*) FROM rejected_events`) != 2 {
		t.Fatalf("risks=%d rejected=%d", count(`SELECT count(*) FROM risks`), count(`SELECT count(*) FROM rejected_events`))
	}
	if count(`SELECT count(*) FROM audit_log WHERE action IN ('risk.received','risk.evidence_viewed','risk.reviewed')`) != 3 {
		t.Fatal("audit rows")
	}
	// FR-26 in the schema: the attempt summary is untouched and no grade revision was written by the review.
	if count(`SELECT score FROM attempts WHERE attempt_id=$1`, attempt) != 17 || count(`SELECT count(*) FROM grade_revisions`) != 0 {
		t.Fatal("FR-26")
	}
	mustFail := func(name, sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err == nil {
			t.Errorf("%s accepted by the schema", name)
		}
	}
	points := 12.5
	input := domain.GradeInput{Grade: &points, Feedback: "published"}
	var versions sync.WaitGroup
	for i := 0; i < 8; i++ {
		versions.Add(1)
		go func() {
			defer versions.Done()
			g, _, err := svc.SetGrade(ctx, teacher, attempt, "grade-same-001", input)
			if err != nil || g.Revision != 1 {
				t.Errorf("grade retry: %v %v", g, err)
			}
		}()
	}
	versions.Wait()
	points = 13.5
	if g, _, err := svc.SetGrade(ctx, teacher, attempt, "grade-correct-002", input); err != nil || g.Revision != 2 {
		t.Fatalf("grade correction: %v %v", g, err)
	}
	if count("SELECT count(*) FROM grade_revisions") != 2 || count("SELECT count(*) FROM outbox WHERE event_type='grade.assigned'") != 2 || count("SELECT count(*) FROM audit_log WHERE action='grade.assigned'") != 2 {
		t.Fatal("grade/history/outbox/audit counts")
	}
	mustFail("rewrite grade", "UPDATE grade_revisions SET grade=0")
	mustFail("delete grade", "DELETE FROM grade_revisions")
	mustFail("re-open risk", `UPDATE risks SET status='open', version=version+1 WHERE id=$1`, id)
	mustFail("edit evidence", `UPDATE risks SET explanation='edited' WHERE id=$1`, id)
	mustFail("delete risk", `DELETE FROM risks WHERE id=$1`, id)
	mustFail("edit review", `UPDATE reviews SET decision='dismissed'`)
	mustFail("delete audit", `DELETE FROM audit_log`)
	mustFail("second review", `INSERT INTO reviews (risk_pk, decision, comment, reviewer_id, reviewer_role, reviewed_at, idempotency_key, event_id)
		VALUES ($1,'dismissed','x',$2,'teacher',now(),'manual-key-1',$3)`, id, ids.NewV7(), ids.NewV7())
	if _, err := pool.Exec(ctx, `INSERT INTO risks (id, risk_id, event_id, category, target_type, target_id, score, level, model_name,
		model_version, explanation, evidence, producer, occurred_at, received_at, status, version)
		VALUES ($1,'r_x',$2,'risk.behavior_anomaly','attempt','a1',0.5,'medium','m','1','e','{}','p',now(),now(),'open',1)`, ids.NewV7(), ids.NewV7()); err != nil {
		t.Fatal(err)
	}
	mustFail("status without review", `UPDATE risks SET status='confirmed', version=2 WHERE risk_id='r_x'`)
	mustFail("version jump", `UPDATE risks SET version=5 WHERE risk_id='r_x'`)
}
