package inbox

import (
	"encoding/json"
	"github.com/Newterios/Examory/services/platform/events"
	"strings"
	"testing"
	"time"
)

func TestSafeReviewProjection(t *testing.T) {
	env, e := events.New("risk.reviewed", 1, "grading", events.Actor{UserID: "teacher", Role: "teacher"}, events.Context{CourseID: events.Ptr("course")}, map[string]any{"target_type": "attempt", "target_id": "att", "decision": "confirmed", "comment": "private comment", "evidence": "private detail"}, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	a, ok, e := project(events.TopicGrades, env)
	if !ok || e != nil || a.notice.Decision == nil {
		t.Fatal(a, e)
	}
	b, _ := json.Marshal(a.notice)
	if strings.Contains(string(b), "private") {
		t.Fatal("private details retained")
	}
	if _, _, err := project(events.TopicCourses, env); err == nil {
		t.Fatal("wrong topic trusted")
	}
	env.Producer = "courses"
	if _, _, err := project(events.TopicGrades, env); err == nil {
		t.Fatal("wrong producer")
	}
}
