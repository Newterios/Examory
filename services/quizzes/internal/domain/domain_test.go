package domain

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

func seqID() IDFunc {
	n := 0
	return func(time.Time) string { n++; return fmt.Sprintf("00000000-0000-7000-8000-%012d", n) }
}

func validSpec() QuizSpec {
	return QuizSpec{
		Title: "Midterm", CourseID: "c_10", TimeLimitSec: 600, MaxAttempts: 2, ShowScore: true,
		Questions: []QuestionSpec{
			{Text: "2+2?", Points: 2, Options: []OptionSpec{{"a", "3"}, {"b", "4"}}, CorrectKeys: []string{"b"}},
			{Text: "primes?", Points: 3, TopicID: "t_1", Options: []OptionSpec{{"a", "2"}, {"b", "3"}, {"c", "4"}}, CorrectKeys: []string{"a", "b"}},
		},
	}
}

func TestQuizSpecValidation(t *testing.T) {
	mut := map[string]func(*QuizSpec){
		"empty title":      func(s *QuizSpec) { s.Title = "  " },
		"long title":       func(s *QuizSpec) { s.Title = strings.Repeat("я", MaxTitleLen+1) },
		"bad course":       func(s *QuizSpec) { s.CourseID = "c 10; drop" },
		"short limit":      func(s *QuizSpec) { s.TimeLimitSec = 5 },
		"attempts":         func(s *QuizSpec) { s.MaxAttempts = 0 },
		"no questions":     func(s *QuizSpec) { s.Questions = nil },
		"one option":       func(s *QuizSpec) { s.Questions[0].Options = s.Questions[0].Options[:1] },
		"nine options":     func(s *QuizSpec) { s.Questions[0].Options = make([]OptionSpec, 9) },
		"dup key":          func(s *QuizSpec) { s.Questions[0].Options[1].Key = "a" },
		"no correct":       func(s *QuizSpec) { s.Questions[0].CorrectKeys = nil },
		"unknown correct":  func(s *QuizSpec) { s.Questions[0].CorrectKeys = []string{"z"} },
		"repeated correct": func(s *QuizSpec) { s.Questions[1].CorrectKeys = []string{"a", "a"} },
		"zero points":      func(s *QuizSpec) { s.Questions[0].Points = 0 },
		"bad topic":        func(s *QuizSpec) { s.Questions[0].TopicID = "x/y" },
		"invalid utf8":     func(s *QuizSpec) { s.Title = "\xff" },
	}
	if err := validSpec().Validate(); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	for name, m := range mut {
		t.Run(name, func(t *testing.T) {
			s := validSpec()
			m(&s)
			var de *Error
			if err := s.Validate(); !errors.As(err, &de) || de.Kind != KindInvalid || len(de.Fields) == 0 {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestBuildAndPublish(t *testing.T) {
	q, err := BuildQuiz(validSpec(), "q1", "teacher", t0, seqID())
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Variants) != 1 || len(q.Variants[0].Questions) != 2 || q.Variants[0].MaxScore() != 5 {
		t.Fatalf("variant %+v", q.Variants)
	}
	if q.Questions[1].CorrectCount() != 2 || !q.Questions[0].Options[1].Correct || q.Questions[0].Options[0].Correct {
		t.Fatal("correct keys not mapped")
	}
	if err := q.Publish(t0); err != nil || q.Status != StatusPublished || q.PublishedAt == nil {
		t.Fatalf("publish: %v", err)
	}
	if err := q.Publish(t0); !errors.Is(err, ErrNotDraft) {
		t.Fatalf("second publish: %v", err)
	}
}

func TestAssignmentWindow(t *testing.T) {
	a := Assignment{OpensAt: t0, ClosesAt: t0.Add(time.Hour)}
	if a.Validate() != nil {
		t.Fatal("valid window rejected")
	}
	for _, bad := range []Assignment{{OpensAt: t0, ClosesAt: t0}, {OpensAt: t0, ClosesAt: t0.Add(-time.Second)}, {OpensAt: t0, ClosesAt: t0.Add(400 * 24 * time.Hour)}, {}} {
		if bad.Validate() == nil {
			t.Errorf("invalid window accepted: %+v", bad)
		}
	}
	cases := []struct {
		at   time.Time
		want error
	}{{t0.Add(-time.Second), ErrNotOpen}, {t0, nil}, {t0.Add(59 * time.Minute), nil}, {t0.Add(time.Hour), ErrClosed}}
	for _, c := range cases {
		if err := a.CheckOpen(c.at); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("CheckOpen(%v)=%v want %v", c.at, err, c.want)
		}
	}
}

func newAttempt(t *testing.T) (*Attempt, *Quiz) {
	t.Helper()
	q, _ := BuildQuiz(validSpec(), "q1", "teacher", t0, seqID())
	_ = q.Publish(t0)
	a := Assignment{QuizID: q.ID, GroupID: "g1", OpensAt: t0, ClosesAt: t0.Add(24 * time.Hour)}
	return NewAttempt("att1", q, q.Variants[0], a, "student", 1, []byte("0123456789abcdef"), t0, false), q
}

func TestAnswerTimingAndChanges(t *testing.T) {
	at, q := newAttempt(t)
	q0, q1 := q.Questions[0], q.Questions[1]
	right0 := []string{q0.Options[1].ID}
	wrong0 := []string{q0.Options[0].ID}

	// Answering before the question is shown is refused (no fabricated timing).
	if _, err := at.SaveAnswer(q0.ID, right0, t0.Add(time.Second), 0); !errors.Is(err, ErrNotShown) {
		t.Fatalf("expected ErrNotShown, got %v", err)
	}
	if ch, _ := at.MarkShown(q0.ID, t0.Add(2*time.Second), 0); !ch {
		t.Fatal("first shown not recorded")
	}
	if ch, _ := at.MarkShown(q0.ID, t0.Add(9*time.Second), 0); ch {
		t.Fatal("shown must be idempotent and keep the first time")
	}

	steps := []struct {
		at       time.Duration
		sel      []string
		noop     bool
		isChange bool
		shownMs  int64
		startMs  int64
	}{
		{10 * time.Second, wrong0, false, false, 8000, 10000},
		{12 * time.Second, wrong0, true, false, 0, 0}, // identical: no-op
		{15 * time.Second, right0, false, true, 13000, 15000},
		{20 * time.Second, wrong0, false, true, 18000, 20000}, // A->B->A counts again
		{21 * time.Second, []string{strings.ToUpper(q0.Options[0].ID)}, true, false, 0, 0},
	}
	for i, s := range steps {
		r, err := at.SaveAnswer(q0.ID, s.sel, t0.Add(s.at), 0)
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if r.Noop != s.noop {
			t.Fatalf("step %d: noop=%v", i, r.Noop)
		}
		if s.noop {
			continue
		}
		if r.IsChange != s.isChange || r.TimeSinceShownMs != s.shownMs || r.TimeSinceAttemptStartMs != s.startMs || r.QuestionIndex != 0 {
			t.Fatalf("step %d: %+v", i, r)
		}
	}
	if at.AnswersChanged != 2 {
		t.Fatalf("answers_changed=%d, want 2 (number of change events)", at.AnswersChanged)
	}
	// Second question: multiple choice, order of ids irrelevant for the hash.
	_, _ = at.MarkShown(q1.ID, t0.Add(30*time.Second), 0)
	r, err := at.SaveAnswer(q1.ID, []string{q1.Options[1].ID, q1.Options[0].ID}, t0.Add(31*time.Second), 0)
	if err != nil || r.QuestionIndex != 1 || r.IsChange {
		t.Fatalf("q1: %+v %v", r, err)
	}
	if r.Hash != AnswerHash([]string{q1.Options[0].ID, q1.Options[1].ID}) {
		t.Fatal("hash must not depend on selection order")
	}
	// Invalid selections.
	for name, sel := range map[string][]string{"empty": {}, "foreign": {q0.Options[0].ID}, "dup": {q1.Options[0].ID, q1.Options[0].ID}} {
		if _, err := at.SaveAnswer(q1.ID, sel, t0.Add(32*time.Second), 0); err == nil {
			t.Errorf("%s selection accepted", name)
		}
	}
	if _, err := at.SaveAnswer("nope", wrong0, t0.Add(33*time.Second), 0); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown question accepted")
	}

	res, err := at.Finalize(AttemptSubmitted, t0.Add(40*time.Second), 0)
	if err != nil {
		t.Fatal(err)
	}
	// q0 answered wrong (0 of 2), q1 right (3 of 3).
	want := FinalResult{Status: AttemptSubmitted, FinishedAt: t0.Add(40 * time.Second), Score: 3, MaxScore: 5, DurationMs: 40000, AnswersChanged: 2}
	if res != want {
		t.Fatalf("result %+v want %+v", res, want)
	}
	if _, err := at.Finalize(AttemptSubmitted, t0.Add(41*time.Second), 0); !errors.Is(err, ErrAttemptFinalized) {
		t.Fatal("double finalize allowed")
	}
	if _, err := at.SaveAnswer(q0.ID, right0, t0.Add(42*time.Second), 0); !errors.Is(err, ErrAttemptFinalized) {
		t.Fatal("answer after finalize allowed")
	}
}

func TestTimeoutSemantics(t *testing.T) {
	at, q := newAttempt(t)
	grace := 2 * time.Second
	deadline := at.DeadlineAt
	if !deadline.Equal(t0.Add(600 * time.Second)) {
		t.Fatalf("deadline %v", deadline)
	}
	_, _ = at.MarkShown(q.Questions[0].ID, t0, grace)
	// Within grace: accepted.
	if _, err := at.SaveAnswer(q.Questions[0].ID, []string{q.Questions[0].Options[1].ID}, deadline.Add(time.Second), grace); err != nil {
		t.Fatalf("within grace: %v", err)
	}
	if _, err := at.MarkShown(q.Questions[1].ID, deadline.Add(3*time.Second), grace); !errors.Is(err, ErrTimeOver) {
		t.Fatalf("after grace: %v", err)
	}
	// A late submit becomes a timeout. It finishes at the deadline, but never
	// before the last accepted activity (the answer saved within grace), so
	// quiz.submitted.occurred_at never precedes a quiz.answered.
	res, err := at.Finalize(AttemptSubmitted, deadline.Add(time.Minute), grace)
	if err != nil || res.Status != AttemptTimedOut || res.DurationMs != 601000 || res.Score != 2 {
		t.Fatalf("late submit: %+v %v", res, err)
	}
}

func TestDeadlineCappedByWindow(t *testing.T) {
	q, _ := BuildQuiz(validSpec(), "q1", "t", t0, seqID())
	a := Assignment{GroupID: "g", OpensAt: t0, ClosesAt: t0.Add(time.Minute)}
	at := NewAttempt("a", q, q.Variants[0], a, "s", 1, []byte("x"), t0, false)
	if !at.DeadlineAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("deadline not capped: %v", at.DeadlineAt)
	}
}

func TestEffectiveClock(t *testing.T) {
	at, _ := newAttempt(t)
	at.LastActivityAt = t0.Add(10 * time.Second)
	if _, err := at.Effective(Clock{Now: t0.Add(11 * time.Second), Sim: true}); !errors.Is(err, ErrClockMismatch) {
		t.Fatal("sim request on a real attempt accepted")
	}
	if got, err := at.Effective(Clock{Now: t0.Add(5 * time.Second)}); err != nil || !got.Equal(at.LastActivityAt) {
		t.Fatalf("real clock not clamped: %v %v", got, err)
	}
	at.Sim = true
	if _, err := at.Effective(Clock{Now: t0.Add(5 * time.Second), Sim: true}); !errors.Is(err, ErrNonMonotonicTime) {
		t.Fatal("backwards sim time accepted")
	}
	if got, err := at.Effective(Clock{Now: t0.Add(10 * time.Second), Sim: true}); err != nil || !got.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("equal sim time rejected: %v", err)
	}
}

func TestDisplayOrderDeterministic(t *testing.T) {
	at, q := newAttempt(t)
	o1 := at.DisplayOrder(at.Questions[1])
	o2 := at.DisplayOrder(at.Questions[1])
	if !reflect.DeepEqual(o1, o2) || len(o1) != len(q.Questions[1].Options) {
		t.Fatal("order not deterministic")
	}
	// Across many seeds the first option is not always the same one.
	first := map[string]bool{}
	for i := 0; i < 50; i++ {
		at.ShuffleSeed = []byte(fmt.Sprintf("seed-%d", i))
		first[at.DisplayOrder(at.Questions[1])[0].ID] = true
	}
	if len(first) < 2 {
		t.Fatal("options never shuffled")
	}
}

func TestAnswerHash(t *testing.T) {
	a := AnswerHash([]string{"B", " a "})
	b := AnswerHash([]string{"a", "b", "a"})
	if a != b || len(a) != 64 {
		t.Fatalf("normalisation: %s %s", a, b)
	}
	// Pinned: sha256("a\nb"), so other implementations can reproduce it.
	if a != "7e18f737311b2dc3b2f269dd78396b0351f14fb66efa879f768cb23181883c78" {
		t.Fatalf("canonical form changed: %s", a)
	}
	if AnswerHash([]string{"a"}) == AnswerHash([]string{"b"}) {
		t.Fatal("different answers share a hash")
	}
}
