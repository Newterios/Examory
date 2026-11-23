package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/ids"
)

func TestNewEnvelope(t *testing.T) {
	at := time.Date(2026, 11, 3, 10, 15, 42, 317_654_321, time.FixedZone("ALM", 5*3600))
	env, err := New(TypeQuizAnswered, 1, "quizzes", Actor{UserID: ids.NewV7(), Role: "student"},
		Context{CourseID: Ptr("c_10")}, QuizAnswered{AttemptID: "a"}, at)
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := ids.V7Time(env.EventID)
	if !ts.Equal(env.OccurredAt.Time) || env.OccurredAt.Nanosecond() != 317_000_000 {
		t.Fatalf("timestamp mismatch %v %v", ts, env.OccurredAt)
	}
	b, _ := json.Marshal(env)
	s := string(b)
	for _, want := range []string{`"occurred_at":"2026-11-03T05:15:42.317Z"`, `"group_id":null`, `"session_id":null`, `"course_id":"c_10"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	back, err := Parse(b)
	if err != nil || back.EventID != env.EventID || !back.OccurredAt.Equal(env.OccurredAt.Time) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestValidate(t *testing.T) {
	good, _ := New("quiz.started", 1, "quizzes", Actor{UserID: "u", Role: "student"}, Context{}, map[string]int{}, time.Now())
	bad := []func(e *Envelope){
		func(e *Envelope) { e.EventID = ids.NewV7At(time.Now().Add(time.Hour)) },
		func(e *Envelope) { e.EventType = "Quiz.Started" },
		func(e *Envelope) { e.Version = 0 },
		func(e *Envelope) { e.Actor.Role = "root" },
		func(e *Envelope) { e.Payload = json.RawMessage(`[]`) },
		func(e *Envelope) { e.Producer = "" },
	}
	for i, mut := range bad {
		e := good
		mut(&e)
		if e.Validate() == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}
