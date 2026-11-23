package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type recorder struct{ events []kafkax.Message }

func (r *recorder) Publish(_ context.Context, es []kafkax.Message) []error {
	r.events = append(r.events, es...)
	return make([]error, len(es))
}
func TestOwnershipSizeSchemaAndLimits(t *testing.T) {
	own, closed := ids.NewV7(), ids.NewV7()
	realToken := "Bearer student-token"
	q := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != realToken || r.Header.Get("X-Sim-Time") != "" {
			t.Error("incorrect forwarded headers")
			w.WriteHeader(401)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/attempts/")
		if id != own && id != closed {
			w.WriteHeader(404)
			return
		}
		status := "in_progress"
		if id == closed {
			status = "submitted"
		}
		_ = json.NewEncoder(w).Encode(Attempt{ID: id, Status: status, Course: "course", Deadline: time.Now().Add(time.Minute), ServerTime: time.Now()})
	}))
	defer q.Close()
	validator, e := eventschema.Load("../../cmd/event-collector/schemas")
	if e != nil {
		t.Fatal(e)
	}
	out := &recorder{}
	s := Service{Authority{q.URL, q.Client()}, out, validator, logging.Discard()}
	engine := func(limit int) *gin.Engine {
		g := gin.New()
		s.Register(g, func(c *gin.Context) {
			c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), authn.Principal{UserID: "student", Role: "student", GroupID: "group"}))
			c.Next()
		}, ratelimit.NewMemory(nil), limit)
		return g
	}
	post := func(g *gin.Engine, body string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/events", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", realToken)
		r.Header.Set("X-Sim-Time", "caller-data")
		g.ServeHTTP(w, r)
		return w.Code
	}
	body := func(id, field string) string {
		return `{"event_type":"client.paste","attempt_id":"` + id + `",` + field + `}`
	}
	g := engine(60)
	for _, tc := range []struct {
		body   string
		status int
	}{{body("not-uuid", `"chars":1`), 400}, {body(ids.NewV7(), `"chars":1`), 404}, {body(closed, `"chars":2`), 404}, {body(own, `"text":"clipboard content"`), 400}, {body(own, `"chars":0,"value":"`+strings.Repeat("x", 2100)+`"`), 413}, {body(own, `"chars":1,"exam_session_id":"`+ids.NewV7()+`"`), 400}, {body(own, `"duration_ms":9`), 400}} {
		if got := post(g, tc.body); got != tc.status {
			t.Fatal(got, tc.status)
		}
	}
	if len(out.events) != 0 {
		t.Fatal("rejected event published")
	}
	g = engine(1)
	if post(g, body(own, `"chars":0`)) != 202 || post(g, body(own, `"chars":2`)) != 429 {
		t.Fatal("acceptance/rate limit")
	}
	if len(out.events) != 1 || out.events[0].Key != own || out.events[0].Topic != events.TopicExams || validator.Validate(out.events[0].Value) != nil {
		t.Fatal("published record invalid")
	}
	for _, kind := range []string{"client.focus_lost", "client.tab_switched"} {
		b := `{"event_type":"` + kind + `","attempt_id":"` + own + `","duration_ms":4000}`
		if post(engine(5), b) != 202 {
			t.Fatal(kind)
		}
	}
}
