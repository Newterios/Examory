package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/app/apptest"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

type fixture struct {
	t                         *testing.T
	ctx                       context.Context
	clk                       *clock.Fake
	store                     *apptest.Store
	svc                       *app.Service
	teacherA, teacherB, admin authn.Principal
	studentA, studentB, other authn.Principal
	group1, group2            string
}

func newFixture(t *testing.T) *fixture {
	clk := clock.NewFake(t0)
	st := apptest.NewStore()
	g1, g2 := ids.NewV7(), ids.NewV7()
	return &fixture{
		t: t, ctx: context.Background(), clk: clk, store: st,
		svc:      app.New(st, clk, logging.Discard(), app.Config{Grace: 2 * time.Second}),
		teacherA: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher},
		teacherB: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleTeacher},
		admin:    authn.Principal{UserID: ids.NewV7(), Role: authn.RoleAdmin},
		studentA: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent, GroupID: g1},
		studentB: authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent, GroupID: g1},
		other:    authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent, GroupID: g2},
		group1:   g1, group2: g2,
	}
}

func (f *fixture) actor(p authn.Principal) app.Actor {
	return app.Actor{Principal: p, Clock: domain.Clock{Now: f.clk.Now()}}
}

func spec() domain.QuizSpec {
	return domain.QuizSpec{
		Title: "Quiz", CourseID: "c_10", TimeLimitSec: 600, MaxAttempts: 2, ShowScore: true,
		Questions: []domain.QuestionSpec{
			{Text: "2+2?", Points: 2, Options: []domain.OptionSpec{{Key: "a", Text: "3"}, {Key: "b", Text: "4"}}, CorrectKeys: []string{"b"}},
			{Text: "primes?", Points: 3, Options: []domain.OptionSpec{{Key: "a", Text: "2"}, {Key: "b", Text: "3"}, {Key: "c", Text: "4"}}, CorrectKeys: []string{"a", "b"}},
		},
	}
}

// publishedQuiz creates, publishes and assigns a quiz to group1.
func (f *fixture) publishedQuiz() *domain.Quiz {
	f.t.Helper()
	q, err := f.svc.CreateQuiz(f.ctx, f.actor(f.teacherA), spec())
	must(f.t, err)
	_, err = f.svc.PublishQuiz(f.ctx, f.actor(f.teacherA), q.ID)
	must(f.t, err)
	_, err = f.svc.Assign(f.ctx, f.actor(f.teacherA), q.ID, f.group1, t0.Add(-time.Hour), t0.Add(24*time.Hour))
	must(f.t, err)
	return q
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("got %v, want %v", err, target)
	}
}

func eventsOf(st *apptest.Store, typ string) []apptest.Event {
	var out []apptest.Event
	for _, e := range st.Events() {
		if e.Env.EventType == typ {
			out = append(out, e)
		}
	}
	return out
}

func TestAuthoringAuthorization(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.CreateQuiz(f.ctx, f.actor(f.studentA), spec())
	wantErr(t, err, domain.ErrForbidden)
	_, err = f.svc.CreateQuiz(f.ctx, f.actor(authn.Principal{UserID: ids.NewV7(), Role: authn.RoleSystem}), spec())
	wantErr(t, err, domain.ErrForbidden)

	q, err := f.svc.CreateQuiz(f.ctx, f.actor(f.teacherA), spec())
	must(t, err)
	// Teacher B cannot read, edit, publish, assign or list attempts of A's quiz.
	_, err = f.svc.GetQuizForAuthor(f.ctx, f.actor(f.teacherB), q.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.UpdateQuiz(f.ctx, f.actor(f.teacherB), q.ID, spec())
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.PublishQuiz(f.ctx, f.actor(f.teacherB), q.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.ListQuizAttempts(f.ctx, f.actor(f.teacherB), q.ID, 10, "")
	wantErr(t, err, domain.ErrNotFound)
	list, _ := f.svc.ListForAuthor(f.ctx, f.actor(f.teacherB), 10, "")
	if len(list) != 0 {
		t.Fatal("teacher B sees A's quiz in list")
	}
	// Assign before publish is refused; admin may manage any quiz.
	_, err = f.svc.Assign(f.ctx, f.actor(f.teacherA), q.ID, f.group1, t0, t0.Add(time.Hour))
	wantErr(t, err, domain.ErrNotPublished)
	s2 := spec()
	s2.Title = "Edited"
	upd, err := f.svc.UpdateQuiz(f.ctx, f.actor(f.admin), q.ID, s2)
	must(t, err)
	if upd.Title != "Edited" || upd.OwnerID != f.teacherA.UserID {
		t.Fatalf("admin update: %+v", upd)
	}
	_, err = f.svc.PublishQuiz(f.ctx, f.actor(f.teacherA), q.ID)
	must(t, err)
	// Published quizzes are immutable.
	_, err = f.svc.UpdateQuiz(f.ctx, f.actor(f.teacherA), q.ID, spec())
	wantErr(t, err, domain.ErrNotDraft)
	_, err = f.svc.Assign(f.ctx, f.actor(f.teacherB), q.ID, f.group1, t0, t0.Add(time.Hour))
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.Assign(f.ctx, f.actor(f.teacherA), q.ID, f.group1, t0.Add(time.Hour), t0)
	var de *domain.Error
	if !errors.As(err, &de) || de.Kind != domain.KindInvalid {
		t.Fatalf("inverted window: %v", err)
	}
}

func TestStartAttemptRules(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	// Not assigned to the other group; no group at all; teacher is not a student.
	_, err := f.svc.StartAttempt(f.ctx, f.actor(f.other), q.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.StartAttempt(f.ctx, f.actor(authn.Principal{UserID: ids.NewV7(), Role: authn.RoleStudent}), q.ID)
	wantErr(t, err, domain.ErrNoGroup)
	_, err = f.svc.StartAttempt(f.ctx, f.actor(f.teacherA), q.ID)
	wantErr(t, err, domain.ErrForbidden)
	// Student does not see unassigned quiz details either.
	_, err = f.svc.GetQuizForStudent(f.ctx, f.actor(f.other), q.ID)
	wantErr(t, err, domain.ErrNotFound)
	// Window.
	_, err = f.svc.Assign(f.ctx, f.actor(f.teacherA), q.ID, f.group2, t0.Add(time.Hour), t0.Add(2*time.Hour))
	must(t, err)
	_, err = f.svc.StartAttempt(f.ctx, f.actor(f.other), q.ID)
	wantErr(t, err, domain.ErrNotOpen)

	v, err := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	must(t, err)
	if v.Status != domain.AttemptInProgress || len(v.Questions) != 2 || v.RemainingMs != 600000 {
		t.Fatalf("view %+v", v)
	}
	// No parallel attempt.
	_, err = f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	wantErr(t, err, domain.ErrAttemptActive)
	// After the first attempt expires, starting again times it out and opens #2.
	f.clk.Advance(11 * time.Minute)
	v2, err := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	must(t, err)
	if v2.AttemptNo != 2 {
		t.Fatalf("attempt no %d", v2.AttemptNo)
	}
	if a, _ := f.store.Attempt(v.ID); a.Status != domain.AttemptTimedOut {
		t.Fatalf("first attempt status %s", a.Status)
	}
	_, err = f.svc.Submit(f.ctx, f.actor(f.studentA), v2.ID, "key-00000001")
	must(t, err)
	// max_attempts = 2.
	_, err = f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	wantErr(t, err, domain.ErrMaxAttempts)
	// Window closed.
	f.clk.Set(t0.Add(25 * time.Hour))
	_, err = f.svc.StartAttempt(f.ctx, f.actor(f.studentB), q.ID)
	wantErr(t, err, domain.ErrClosed)
}

func TestNoIDOR(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	v, err := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	must(t, err)
	qid := v.Questions[0].ID
	opt := v.Questions[0].Options[0].ID
	b := f.actor(f.studentB) // same group, different student
	_, err = f.svc.GetAttempt(f.ctx, b, v.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.MarkShown(f.ctx, b, v.ID, qid)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.SaveAnswer(f.ctx, b, v.ID, qid, []string{opt})
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.Submit(f.ctx, b, v.ID, "key-00000001")
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.GetAttempt(f.ctx, f.actor(f.teacherA), v.ID)
	wantErr(t, err, domain.ErrForbidden)
	if a, _ := f.store.Attempt(v.ID); a.Status != domain.AttemptInProgress || a.Questions[0].FirstShownAt != nil {
		t.Fatal("foreign request changed the attempt")
	}
}

func TestFullFlowEvents(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	a := f.actor(f.studentA)
	v, err := f.svc.StartAttempt(f.ctx, a, q.ID)
	must(t, err)
	// The student view never carries the marking key.
	raw, _ := json.Marshal(v)
	if strings.Contains(strings.ToLower(string(raw)), "correct") {
		t.Fatalf("view leaks correctness: %s", raw)
	}
	again, _ := f.svc.GetAttempt(f.ctx, a, v.ID)
	if fmt.Sprint(again.Questions[1].Options) != fmt.Sprint(v.Questions[1].Options) {
		t.Fatal("option order not stable within the attempt")
	}
	q0, q1 := q.Questions[0], q.Questions[1]

	step := func(d time.Duration) app.Actor { f.clk.Advance(d); return f.actor(f.studentA) }
	_, err = f.svc.SaveAnswer(f.ctx, step(time.Second), v.ID, q0.ID, []string{q0.Options[1].ID})
	wantErr(t, err, domain.ErrNotShown)
	shown, err := f.svc.MarkShown(f.ctx, step(time.Second), v.ID, q0.ID) // t=2s
	must(t, err)
	shown2, err := f.svc.MarkShown(f.ctx, step(5*time.Second), v.ID, q0.ID) // idempotent
	must(t, err)
	if !shown.Equal(shown2) {
		t.Fatal("shown time changed")
	}
	ack, err := f.svc.SaveAnswer(f.ctx, step(3*time.Second), v.ID, q0.ID, []string{q0.Options[0].ID}) // t=10s wrong
	must(t, err)
	if ack.IsChange || ack.Unchanged {
		t.Fatalf("first answer ack %+v", ack)
	}
	ack, _ = f.svc.SaveAnswer(f.ctx, step(time.Second), v.ID, q0.ID, []string{q0.Options[0].ID}) // identical
	if !ack.Unchanged {
		t.Fatal("identical answer not a no-op")
	}
	_, err = f.svc.SaveAnswer(f.ctx, step(4*time.Second), v.ID, q0.ID, []string{q0.Options[1].ID}) // t=15s change
	must(t, err)
	_, _ = f.svc.MarkShown(f.ctx, step(time.Second), v.ID, q1.ID)                                  // t=16s
	_, err = f.svc.SaveAnswer(f.ctx, step(4*time.Second), v.ID, q1.ID, []string{q1.Options[0].ID}) // t=20s
	must(t, err)
	_, err = f.svc.SaveAnswer(f.ctx, step(time.Second), v.ID, q1.ID, []string{q1.Options[1].ID, q1.Options[0].ID}) // t=21s change
	must(t, err)
	res, err := f.svc.Submit(f.ctx, step(9*time.Second), v.ID, "submit-key-1") // t=30s
	must(t, err)
	if res.Status != domain.AttemptSubmitted || *res.Score != 5 || *res.MaxScore != 5 {
		t.Fatalf("result %+v", res)
	}

	evs := f.store.Events()
	var types []string
	for _, e := range evs {
		types = append(types, e.Env.EventType)
		if e.Topic != events.TopicQuizzes || e.Key != f.studentA.UserID {
			t.Fatalf("topic/key %s/%s", e.Topic, e.Key)
		}
		if e.Env.Actor.UserID != f.studentA.UserID || e.Env.Actor.Role != "student" ||
			*e.Env.Context.CourseID != "c_10" || *e.Env.Context.GroupID != f.group1 || e.Env.Context.SessionID != nil {
			t.Fatalf("envelope %+v", e.Env)
		}
	}
	if got := strings.Join(types, ","); got != "quiz.started,quiz.answered,quiz.answered,quiz.answered,quiz.answered,quiz.submitted" {
		t.Fatalf("event sequence %s", got)
	}
	type ans = events.QuizAnswered
	var answered []ans
	for _, e := range eventsOf(f.store, events.TypeQuizAnswered) {
		var p ans
		must(t, e.Env.DecodePayload(&p, true))
		answered = append(answered, p)
	}
	want := []struct {
		idx    int
		change bool
		shown  int64
		start  int64
	}{{0, false, 8000, 10000}, {0, true, 13000, 15000}, {1, false, 4000, 20000}, {1, true, 5000, 21000}}
	for i, w := range want {
		p := answered[i]
		if p.QuestionIndex != w.idx || p.IsChange != w.change || p.TimeSinceShownMs != w.shown || p.TimeSinceAttemptStartMs != w.start || p.AttemptID != v.ID {
			t.Fatalf("answered[%d] = %+v, want %+v", i, p, w)
		}
	}
	if answered[1].AnswerHash != domain.AnswerHash([]string{q0.Options[1].ID}) {
		t.Fatal("answer_hash mismatch")
	}
	var sub events.QuizSubmitted
	must(t, eventsOf(f.store, events.TypeQuizSubmitted)[0].Env.DecodePayload(&sub, true))
	if sub.AnswersChanged != 2 || sub.Score != 5 || sub.MaxScore != 5 || sub.DurationMs != 30000 || sub.Reason != "submitted" || sub.VariantID != v.VariantID {
		t.Fatalf("submitted %+v", sub)
	}
	var started events.QuizStarted
	must(t, evs[0].Env.DecodePayload(&started, true))
	if started.QuestionCount != 2 || started.TimeLimitSec != 600 || started.ExamSessionID != nil {
		t.Fatalf("started %+v", started)
	}
	// After submit nothing can change.
	_, err = f.svc.SaveAnswer(f.ctx, step(time.Second), v.ID, q0.ID, []string{q0.Options[0].ID})
	wantErr(t, err, domain.ErrAttemptFinalized)

	// Contract: every produced event validates against the JSON Schemas.
	dir, err := eventschema.FindDir()
	must(t, err)
	val, err := eventschema.Load(dir)
	must(t, err)
	for _, e := range evs {
		if err := val.Validate(e.Raw); err != nil {
			t.Fatalf("%s violates schema: %v", e.Env.EventType, err)
		}
	}
}

func TestSubmitIdempotency(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	a := f.actor(f.studentA)
	v, _ := f.svc.StartAttempt(f.ctx, a, q.ID)
	_, err := f.svc.Submit(f.ctx, a, v.ID, "")
	wantErr(t, err, domain.ErrIdempotencyNeeded)
	_, err = f.svc.Submit(f.ctx, a, v.ID, "bad key with spaces")
	wantErr(t, err, domain.ErrIdempotencyNeeded)
	r1, err := f.svc.Submit(f.ctx, a, v.ID, "key-aaaaaaaa")
	must(t, err)
	r2, err := f.svc.Submit(f.ctx, a, v.ID, "key-aaaaaaaa")
	must(t, err)
	if !r2.Replayed || r1.Replayed || *r1.Score != *r2.Score {
		t.Fatalf("replay %+v %+v", r1, r2)
	}
	_, err = f.svc.Submit(f.ctx, a, v.ID, "key-bbbbbbbb")
	wantErr(t, err, domain.ErrAttemptFinalized)
	// Reusing the key for another attempt is rejected.
	v2, err := f.svc.StartAttempt(f.ctx, a, q.ID)
	must(t, err)
	_, err = f.svc.Submit(f.ctx, a, v2.ID, "key-aaaaaaaa")
	wantErr(t, err, domain.ErrIdempotencyReuse)
	if n := len(eventsOf(f.store, events.TypeQuizSubmitted)); n != 1 {
		t.Fatalf("%d quiz.submitted", n)
	}
}

func TestExactlyOnceSubmitUnderConcurrency(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	a := f.actor(f.studentA)
	v, _ := f.svc.StartAttempt(f.ctx, a, q.ID)
	var wg sync.WaitGroup
	var mu sync.Mutex
	outcomes := map[string]int{}
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%08d", i%4) // some share keys, some differ
			var err error
			if i%5 == 0 {
				_, err = f.svc.SweepExpired(f.ctx, 10) // sweeper racing (not expired: no-op)
			} else {
				_, err = f.svc.Submit(f.ctx, a, v.ID, key)
			}
			mu.Lock()
			outcomes[fmt.Sprint(err)]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if n := len(eventsOf(f.store, events.TypeQuizSubmitted)); n != 1 {
		t.Fatalf("expected exactly one quiz.submitted, got %d (outcomes %v)", n, outcomes)
	}
	for k := range outcomes {
		if k != "<nil>" && !strings.Contains(k, "attempt_finalized") {
			t.Fatalf("unexpected outcome %s", k)
		}
	}
}

func TestSweeperAndLazyTimeout(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	a := f.actor(f.studentA)
	v, _ := f.svc.StartAttempt(f.ctx, a, q.ID)
	vb, _ := f.svc.StartAttempt(f.ctx, f.actor(f.studentB), q.ID)
	_, _ = f.svc.MarkShown(f.ctx, a, v.ID, q.Questions[0].ID)

	f.clk.Advance(10*time.Minute + time.Second) // past deadline, within grace
	if n, _ := f.svc.SweepExpired(f.ctx, 10); n != 0 {
		t.Fatal("swept within grace")
	}
	f.clk.Advance(5 * time.Second)
	n, err := f.svc.SweepExpired(f.ctx, 10)
	must(t, err)
	if n != 2 {
		t.Fatalf("swept %d", n)
	}
	if n, _ := f.svc.SweepExpired(f.ctx, 10); n != 0 {
		t.Fatal("double sweep")
	}
	subs := eventsOf(f.store, events.TypeQuizSubmitted)
	if len(subs) != 2 {
		t.Fatalf("%d submitted events", len(subs))
	}
	var p events.QuizSubmitted
	must(t, subs[0].Env.DecodePayload(&p, true))
	if p.Reason != "timed_out" || p.DurationMs != 600000 || !subs[0].Env.OccurredAt.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("timeout payload %+v at %v", p, subs[0].Env.OccurredAt)
	}
	// A late submit returns the timed-out result without a second event.
	r, err := f.svc.Submit(f.ctx, f.actor(f.studentA), v.ID, "key-late-0001")
	must(t, err)
	if r.Status != domain.AttemptTimedOut || !r.Replayed {
		t.Fatalf("late submit %+v", r)
	}
	_, err = f.svc.SaveAnswer(f.ctx, f.actor(f.studentB), vb.ID, q.Questions[0].ID, []string{q.Questions[0].Options[0].ID})
	wantErr(t, err, domain.ErrAttemptFinalized)
	if len(eventsOf(f.store, events.TypeQuizSubmitted)) != 2 {
		t.Fatal("extra quiz.submitted")
	}
}

func TestLazyTimeoutOnAnswer(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	v, _ := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	_, _ = f.svc.MarkShown(f.ctx, f.actor(f.studentA), v.ID, q.Questions[0].ID)
	f.clk.Advance(time.Hour)
	_, err := f.svc.SaveAnswer(f.ctx, f.actor(f.studentA), v.ID, q.Questions[0].ID, []string{q.Questions[0].Options[0].ID})
	wantErr(t, err, domain.ErrTimeOver)
	a, _ := f.store.Attempt(v.ID)
	if a.Status != domain.AttemptTimedOut || len(eventsOf(f.store, events.TypeQuizSubmitted)) != 1 {
		t.Fatalf("lazy timeout not committed: %s", a.Status)
	}
}

func TestSimulatedClock(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	simAt := func(d time.Duration) app.Actor {
		return app.Actor{Principal: f.studentA, Clock: domain.Clock{Now: t0.Add(d), Sim: true}}
	}
	v, err := f.svc.StartAttempt(f.ctx, simAt(0), q.ID)
	must(t, err)
	_, err = f.svc.MarkShown(f.ctx, simAt(5*time.Second), v.ID, q.Questions[0].ID)
	must(t, err)
	// Backwards simulated time is rejected.
	_, err = f.svc.SaveAnswer(f.ctx, simAt(4*time.Second), v.ID, q.Questions[0].ID, []string{q.Questions[0].Options[1].ID})
	wantErr(t, err, domain.ErrNonMonotonicTime)
	// Mixing real-time requests into a simulated attempt is rejected.
	_, err = f.svc.SaveAnswer(f.ctx, f.actor(f.studentA), v.ID, q.Questions[0].ID, []string{q.Questions[0].Options[1].ID})
	wantErr(t, err, domain.ErrClockMismatch)
	_, err = f.svc.SaveAnswer(f.ctx, simAt(9*time.Second), v.ID, q.Questions[0].ID, []string{q.Questions[0].Options[1].ID})
	must(t, err)
	// The real-time sweeper never times out simulated attempts.
	f.clk.Advance(48 * time.Hour)
	if n, _ := f.svc.SweepExpired(f.ctx, 10); n != 0 {
		t.Fatal("sweeper touched a simulated attempt")
	}
	_, err = f.svc.Submit(f.ctx, simAt(60*time.Second), v.ID, "sim-key-0001")
	must(t, err)
	ans := eventsOf(f.store, events.TypeQuizAnswered)
	var p events.QuizAnswered
	must(t, ans[0].Env.DecodePayload(&p, true))
	if p.TimeSinceShownMs != 4000 || !ans[0].Env.OccurredAt.Equal(t0.Add(9*time.Second)) {
		t.Fatalf("sim timing %+v at %v", p, ans[0].Env.OccurredAt)
	}
	ts, _ := ids.V7Time(ans[0].Env.EventID)
	if !ts.Equal(t0.Add(9 * time.Second)) {
		t.Fatal("event_id timestamp must equal simulated occurred_at")
	}
}

func TestTransactionRollbackOnEnqueueFailure(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	f.store.FailEnqueue = true
	_, err := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID)
	if err == nil {
		t.Fatal("expected failure")
	}
	f.store.FailEnqueue = false
	if _, err := f.svc.StartAttempt(f.ctx, f.actor(f.studentA), q.ID); err != nil {
		t.Fatalf("state leaked from the failed transaction: %v", err)
	}
	if n := len(eventsOf(f.store, events.TypeQuizStarted)); n != 1 {
		t.Fatalf("%d started events", n)
	}
}

func TestStudentListsOnlyAssigned(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	_, _ = f.svc.CreateQuiz(f.ctx, f.actor(f.teacherA), spec()) // draft, not listed
	got, err := f.svc.ListForStudent(f.ctx, f.actor(f.studentA), 10, "")
	must(t, err)
	if len(got) != 1 || got[0].Quiz.ID != q.ID {
		t.Fatalf("assigned list %+v", got)
	}
	if got, _ := f.svc.ListForStudent(f.ctx, f.actor(f.other), 10, ""); len(got) != 0 {
		t.Fatal("other group sees quiz")
	}
	sq, err := f.svc.GetQuizForStudent(f.ctx, f.actor(f.studentA), q.ID)
	must(t, err)
	if sq.QuestionCount != 2 || sq.ActiveAttemptID != nil {
		t.Fatalf("student quiz %+v", sq)
	}
}
