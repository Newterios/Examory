package eventschema

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
)

func load(t *testing.T) *Validator {
	t.Helper()
	dir, err := FindDir()
	if err != nil {
		t.Fatal(err)
	}
	v, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSchemasAcceptValidAndRejectInvalid(t *testing.T) {
	v := load(t)
	env, err := events.New(events.TypeQuizAnswered, 1, "quizzes",
		events.Actor{UserID: ids.NewV7(), Role: "student"},
		events.Context{CourseID: events.Ptr("c_10"), GroupID: events.Ptr(ids.NewV7())},
		events.QuizAnswered{AttemptID: ids.NewV7(), QuestionID: ids.NewV7(), QuestionIndex: 0,
			AnswerHash: strings.Repeat("a", 64), TimeSinceShownMs: 10, TimeSinceAttemptStartMs: 20}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	good, _ := json.Marshal(env)
	if err := v.Validate(good); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	// Mutations that must be rejected.
	mutate := func(f func(m map[string]any)) []byte {
		var m map[string]any
		_ = json.Unmarshal(good, &m)
		f(m)
		b, _ := json.Marshal(m)
		return b
	}
	bad := map[string][]byte{
		"extra envelope field": mutate(func(m map[string]any) { m["x"] = 1 }),
		"extra payload field":  mutate(func(m map[string]any) { m["payload"].(map[string]any)["student_email"] = "a@b" }),
		"negative time":        mutate(func(m map[string]any) { m["payload"].(map[string]any)["time_since_shown_ms"] = -1 }),
		"bad hash":             mutate(func(m map[string]any) { m["payload"].(map[string]any)["answer_hash"] = "xyz" }),
		"seconds precision":    mutate(func(m map[string]any) { m["occurred_at"] = "2026-11-03T10:15:42Z" }),
		"missing context key":  mutate(func(m map[string]any) { delete(m["context"].(map[string]any), "session_id") }),
		"bad role":             mutate(func(m map[string]any) { m["actor"].(map[string]any)["role"] = "root" }),
		"wrong producer":       mutate(func(m map[string]any) { m["producer"] = "auth" }),
	}
	for name, b := range bad {
		if err := v.Validate(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
