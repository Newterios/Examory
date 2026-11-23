package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/app/apptest"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
)

var t0 = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

type fx struct {
	t       *testing.T
	ctx     context.Context
	store   *apptest.Store
	svc     *app.Service
	val     *eventschema.Validator
	teacher authn.Principal
	other   authn.Principal
	admin   authn.Principal
	student authn.Principal
	course  string
	group   string
}

func newFx(t *testing.T) *fx {
	dir, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	val, err := eventschema.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := apptest.NewStore()
	return &fx{t: t, ctx: context.Background(), store: st, val: val,
		svc:     app.New(st, val, clock.NewFake(t0), logging.Discard()),
		teacher: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher},
		other:   authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher},
		admin:   authn.Principal{UserID: ids.NewV7(), Role: authn.RoleAdmin},
		student: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent, GroupID: ids.NewV7()},
		course:  "c_10", group: ids.NewV7()}
}

func (f *fx) ingest(topic string, env events.Envelope) error {
	return f.store.Tx(func(tx app.Tx) error { return f.svc.Ingest(f.ctx, tx, env, topic) })
}

func (f *fx) submitted(student, attemptID, quizID string, score int) events.Envelope {
	env, err := events.New(events.TypeQuizSubmitted, 1, "quizzes", events.Actor{UserID: student, Role: "student"},
		events.Context{CourseID: events.Ptr(f.course), GroupID: events.Ptr(f.group)},
		events.QuizSubmitted{AttemptID: attemptID, QuizID: quizID, VariantID: ids.NewV7(), Score: score, MaxScore: 20,
			DurationMs: 400000, AnswersChanged: 1, Reason: "submitted"}, t0)
	if err != nil {
		f.t.Fatal(err)
	}
	return env
}

func riskPayload(attemptID string) map[string]any {
	return map[string]any{
		"risk_id": "risk_" + strings.ReplaceAll(attemptID, "-", "")[:24], "target_type": "attempt", "target_id": attemptID,
		"score": 0.997, "level": "high", "model": map[string]any{"name": "isolation-forest-v1", "version": "0.1.0"},
		"explanation": "mean_answer_time_ms=512 (below normal-train 1–99% range). Relative anomaly ranking, not a probability or proof of misconduct.",
		"evidence":    map[string]any{"features": map[string]any{"mean_answer_time_ms": 512.0, "z_score": nil, "synchronous_with": []string{"att_91"}}},
	}
}

func (f *fx) risk(payload map[string]any) events.Envelope {
	env, err := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"},
		events.Context{CourseID: events.Ptr(f.course), GroupID: events.Ptr(f.group)}, payload, t0.Add(10*time.Second))
	if err != nil {
		f.t.Fatal(err)
	}
	return env
}

// seed ingests one attempt and its risk; it returns grading's risk id.
func (f *fx) seed() (riskPK, attemptID, quizID string) {
	attemptID, quizID = ids.NewV7(), ids.NewV7()
	if err := f.ingest(events.TopicQuizzes, f.submitted(ids.NewV7(), attemptID, quizID, 18)); err != nil {
		f.t.Fatal(err)
	}
	if err := f.ingest(events.TopicRisk, f.risk(riskPayload(attemptID))); err != nil {
		f.t.Fatal(err)
	}
	rows, _ := f.svc.ListRisks(f.ctx, f.teacher, app.Filter{QuizID: quizID}, 10, "")
	if len(rows) != 1 {
		f.t.Fatalf("seeded risks %d", len(rows))
	}
	return rows[0].Risk.ID, attemptID, quizID
}

func TestIngestStoresAndDeduplicates(t *testing.T) {
	f := newFx(t)
	pk, attemptID, _ := f.seed()
	// Re-emission of the same risk_id under a new event_id: no second signal.
	if err := f.ingest(events.TopicRisk, f.risk(riskPayload(attemptID))); err != nil {
		t.Fatal(err)
	}
	s := f.store.Snapshot()
	if len(s.Risks) != 1 {
		t.Fatalf("%d risks after re-emission", len(s.Risks))
	}
	r := s.Risks[pk]
	if r.Status != "open" || r.Level != "high" || r.Score != 0.997 || r.ModelName != "isolation-forest-v1" || r.CourseID != f.course || !strings.Contains(string(r.Evidence), "synchronous_with") {
		t.Fatalf("stored risk %+v", r)
	}
	row, err := f.svc.GetRisk(f.ctx, f.teacher, pk)
	if err != nil || row.Attempt == nil || row.Attempt.Score != 18 || row.Review != nil {
		t.Fatalf("detail %+v %v", row, err)
	}
}

func TestInvalidRiskIsDeadLetteredNotFatal(t *testing.T) {
	f := newFx(t)
	cases := map[string]func(map[string]any){
		"score above 1":    func(p map[string]any) { p["score"] = 1.7 },
		"unknown field":    func(p map[string]any) { p["student_name"] = "Alice" },
		"bad target type":  func(p map[string]any) { p["target_type"] = "submission" },
		"missing evidence": func(p map[string]any) { delete(p, "evidence") },
		"bad level":        func(p map[string]any) { p["level"] = "critical" },
		"bad features type": func(p map[string]any) {
			p["evidence"] = map[string]any{"features": map[string]any{"mean_answer_time_ms": "fast", "z_score": nil, "synchronous_with": []string{}}}
		},
		"evidence extra key": func(p map[string]any) {
			p["evidence"] = map[string]any{"features": map[string]any{"mean_answer_time_ms": 1.0, "z_score": nil, "synchronous_with": []string{}}, "raw": "x"}
		},
	}
	for name, mut := range cases {
		p := riskPayload(ids.NewV7())
		mut(p)
		if err := f.ingest(events.TopicRisk, f.risk(p)); err != nil {
			t.Fatalf("%s: ingest must not fail (would block the partition): %v", name, err)
		}
	}
	s := f.store.Snapshot()
	if len(s.Risks) != 0 || len(s.Rejected) != len(cases) {
		t.Fatalf("risks=%d rejected=%d", len(s.Risks), len(s.Rejected))
	}
	// Other risk categories are acknowledged and ignored.
	env, _ := events.New("risk.plagiarism", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"}, events.Context{}, map[string]any{"risk_id": "x"}, t0)
	if err := f.ingest(events.TopicRisk, env); err != nil || len(f.store.Snapshot().Risks) != 0 {
		t.Fatalf("plagiarism handling: %v", err)
	}
}

func TestAuthorization(t *testing.T) {
	f := newFx(t)
	pk, _, _ := f.seed()
	sys := authn.Principal{UserID: ids.NewV7(), Role: authn.RoleSystem}
	for _, p := range []authn.Principal{f.student, sys} {
		if _, err := f.svc.ListRisks(f.ctx, p, app.Filter{}, 10, ""); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s list: %v", p.Role, err)
		}
		if _, err := f.svc.GetRisk(f.ctx, p, pk); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s get: %v", p.Role, err)
		}
	}
	for _, p := range []authn.Principal{f.student, sys, f.admin} {
		if _, err := f.svc.Review(f.ctx, p, pk, "confirmed", "x", 1, "key-00000001"); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s review: %v", p.Role, err)
		}
	}
	if _, err := f.svc.ListRisks(f.ctx, f.admin, app.Filter{}, 10, ""); err != nil {
		t.Fatalf("admin read: %v", err)
	}
}

func TestReviewFlowEventAndAudit(t *testing.T) {
	f := newFx(t)
	pk, attemptID, _ := f.seed()
	before := f.store.Snapshot()

	_, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "consistent with the evidence", 0, "key-00000001")
	if !errors.Is(err, domain.ErrIfMatchRequired) {
		t.Fatalf("missing If-Match: %v", err)
	}
	if _, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "x", 7, "key-00000001"); !errors.Is(err, domain.ErrVersionMismatch) {
		t.Fatalf("stale If-Match: %v", err)
	}
	if _, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "x", 1, ""); !errors.Is(err, domain.ErrIdempotencyNeeded) {
		t.Fatalf("missing key: %v", err)
	}
	res, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "consistent with the evidence", 1, "key-00000001")
	if err != nil || res.Replayed || res.Row.Risk.Status != "confirmed" || res.Row.Risk.Version != 2 || res.Row.Review.ReviewerID != f.teacher.UserID {
		t.Fatalf("review: %+v %v", res, err)
	}
	// Replay with the same key and body.
	rep, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "consistent with the evidence", 1, "key-00000001")
	if err != nil || !rep.Replayed {
		t.Fatalf("replay %+v %v", rep, err)
	}
	// Same key, different body; another teacher; second decision.
	if _, err := f.svc.Review(f.ctx, f.teacher, pk, "dismissed", "changed my mind", 2, "key-00000001"); !errors.Is(err, domain.ErrIdempotencyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if _, err := f.svc.Review(f.ctx, f.other, pk, "dismissed", "disagree", 2, "key-00000002"); !errors.Is(err, domain.ErrAlreadyReviewed) {
		t.Fatalf("second review: %v", err)
	}

	after := f.store.Snapshot()
	// Exactly one risk.reviewed, valid against the schema, actor = reviewer.
	if len(after.Outbox) != 1 {
		t.Fatalf("%d outbox events", len(after.Outbox))
	}
	ev := after.Outbox[0]
	if ev.Topic != events.TopicGrades || ev.Key != f.course || ev.Env.EventType != "risk.reviewed" || ev.Env.Actor.UserID != f.teacher.UserID || ev.Env.Actor.Role != "teacher" {
		t.Fatalf("event %+v", ev)
	}
	if err := f.val.Validate(ev.Raw); err != nil {
		t.Fatalf("risk.reviewed schema: %v", err)
	}
	var p map[string]any
	_ = json.Unmarshal(ev.Env.Payload, &p)
	if p["decision"] != "confirmed" || p["target_id"] != attemptID || p["reviewed_by"] != f.teacher.UserID || p["target_type"] != "attempt" {
		t.Fatalf("payload %v", p)
	}
	// FR-26: neither receipt nor confirmation changed the attempt (score),
	// and no grade event was produced.
	if !reflect.DeepEqual(before.Attempts, after.Attempts) {
		t.Fatal("review changed attempt data")
	}
	for _, e := range after.Outbox {
		if strings.HasPrefix(e.Env.EventType, "grade.") {
			t.Fatal("a grade event was produced by a review")
		}
	}
	// NFR-16: audit has received, review and rejected attempts with actor and outcome.
	var actions []string
	for _, a := range after.Audit {
		actions = append(actions, a.Action+":"+a.Outcome)
		if a.ActorID == "" || a.Action == "" || a.At.IsZero() {
			t.Fatalf("incomplete audit %+v", a)
		}
	}
	got := strings.Join(actions, ",")
	for _, want := range []string{"risk.received:stored", "risk.reviewed:confirmed", "risk.review_rejected:if_match_required", "risk.review_rejected:already_reviewed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("audit %s missing %s", got, want)
		}
	}
	for _, a := range after.Audit {
		for _, v := range a.Detail {
			if strings.Contains(v, "consistent with") {
				t.Fatal("audit stores the comment")
			}
		}
	}
}

func TestExactlyOneReviewUnderConcurrency(t *testing.T) {
	f := newFx(t)
	pk, _, _ := f.seed()
	var wg sync.WaitGroup
	var mu sync.Mutex
	outcomes := map[string]int{}
	teachers := []authn.Principal{f.teacher, f.other}
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d := []string{"confirmed", "dismissed"}[i%2]
			_, err := f.svc.Review(f.ctx, teachers[i%2], pk, d, "reason "+d, 1, fmt.Sprintf("key-%08d", i))
			mu.Lock()
			outcomes[fmt.Sprint(err)]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	s := f.store.Snapshot()
	if len(s.Reviews) != 1 || len(s.Outbox) != 1 || outcomes["<nil>"] != 1 {
		t.Fatalf("reviews=%d events=%d outcomes=%v", len(s.Reviews), len(s.Outbox), outcomes)
	}
}

func TestFilters(t *testing.T) {
	f := newFx(t)
	pk1, _, quiz1 := f.seed()
	_, _, _ = f.seed()
	if _, err := f.svc.Review(f.ctx, f.teacher, pk1, "dismissed", "legitimate fast solver", 1, "key-00000009"); err != nil {
		t.Fatal(err)
	}
	count := func(fl app.Filter) int {
		rows, err := f.svc.ListRisks(f.ctx, f.teacher, fl, 100, "")
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	if count(app.Filter{}) != 2 || count(app.Filter{Status: "open"}) != 1 || count(app.Filter{Status: "dismissed"}) != 1 ||
		count(app.Filter{QuizID: quiz1}) != 1 || count(app.Filter{CourseID: "c_other"}) != 0 {
		t.Fatal("filters")
	}
	// Keyset pagination.
	first, _ := f.svc.ListRisks(f.ctx, f.teacher, app.Filter{}, 1, "")
	second, _ := f.svc.ListRisks(f.ctx, f.teacher, app.Filter{}, 1, first[0].Risk.ID)
	if len(first) != 1 || len(second) != 1 || first[0].Risk.ID == second[0].Risk.ID {
		t.Fatal("pagination")
	}
}
