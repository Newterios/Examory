package collector

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var errDenied = errors.New("owned open attempt not found")

type Attempt struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Course     string    `json:"course_id"`
	Session    *string   `json:"exam_session_id"`
	ServerTime time.Time `json:"server_time"`
	Deadline   time.Time `json:"deadline_at"`
}
type Authority struct {
	Base string
	HTTP *http.Client
}

// Quizzes verifies ownership using the original JWT. Only this header is forwarded.
func (a Authority) Lookup(ctx context.Context, token, id string) (Attempt, error) {
	var at Attempt
	req, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(a.Base, "/")+"/v1/attempts/"+id, nil)
	if e != nil {
		return at, e
	}
	req.Header.Set("Authorization", token)
	resp, e := a.HTTP.Do(req)
	if e != nil {
		return at, e
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 401, 403, 404, 410:
		return at, errDenied
	default:
		return at, errors.New("quizzes unavailable")
	}
	const max = 4 << 20
	b, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if e != nil || len(b) > max || json.Unmarshal(b, &at) != nil || at.ID != id || at.Deadline.IsZero() || at.ServerTime.IsZero() {
		return at, errors.New("invalid quizzes response")
	}
	if at.Status != "in_progress" || !at.Deadline.After(at.ServerTime) {
		return at, errDenied
	}
	return at, nil
}

type Input struct {
	Type     string  `json:"event_type"`
	ID       string  `json:"attempt_id"`
	Session  *string `json:"exam_session_id"`
	Duration *int64  `json:"duration_ms"`
	Chars    *int64  `json:"chars"`
}

func (i Input) payload() (map[string]any, bool) {
	if !ids.Valid(i.ID) || (i.Session != nil && !ids.Valid(*i.Session)) {
		return nil, false
	}
	p := map[string]any{"attempt_id": i.ID, "exam_session_id": i.Session}
	switch i.Type {
	case "client.paste":
		if i.Chars == nil || i.Duration != nil || *i.Chars < 0 || *i.Chars > 10000000 {
			return nil, false
		}
		p["chars"] = *i.Chars
	case "client.focus_lost", "client.tab_switched":
		if i.Duration == nil || i.Chars != nil || *i.Duration < 0 || *i.Duration > 86400000 {
			return nil, false
		}
		p["duration_ms"] = *i.Duration
	default:
		return nil, false
	}
	return p, true
}

type Service struct {
	Quizzes   Authority
	Producer  kafkax.Publisher
	Validator *eventschema.Validator
	Log       *slog.Logger
}

func (s Service) Register(e *gin.Engine, authenticate gin.HandlerFunc, limiter ratelimit.Limiter, limit int) {
	e.POST("/v1/events", authenticate, httpx.RequireRoles(authn.RoleStudent), httpx.RateLimit(limiter, s.Log, "event-collector", limit, time.Minute, httpx.BySubject), s.collect)
}
func (s Service) collect(c *gin.Context) {
	var i Input
	if !httpx.BindJSON(c, &i, 2048) {
		return
	}
	p, valid := i.payload()
	if !valid {
		httpx.Fail(c, 400, "invalid_event", "invalid client event")
		return
	}
	a, e := s.Quizzes.Lookup(c.Request.Context(), c.GetHeader("Authorization"), i.ID)
	if errors.Is(e, errDenied) {
		httpx.Fail(c, 404, "not_found", "owned open attempt not found")
		return
	}
	if e != nil {
		httpx.Fail(c, 503, "unavailable", "could not confirm attempt")
		return
	}
	if i.Session != nil && (a.Session == nil || *i.Session != *a.Session) {
		httpx.Fail(c, 400, "invalid_session", "session does not match the attempt")
		return
	}
	p["exam_session_id"] = a.Session
	student, _ := httpx.Principal(c)
	env, e := events.New(i.Type, 1, "event-collector", events.Actor{UserID: student.UserID, Role: "student"}, events.Context{CourseID: events.Ptr(a.Course), GroupID: events.Ptr(student.GroupID), SessionID: a.Session}, p, time.Now())
	if e != nil {
		httpx.Internal(c, s.Log, e)
		return
	}
	raw, e := json.Marshal(env)
	if e != nil || s.Validator.Validate(raw) != nil {
		httpx.Fail(c, 500, "contract_error", "event contract validation failed")
		return
	}
	errs := s.Producer.Publish(c.Request.Context(), []kafkax.Message{{Topic: events.TopicExams, Key: i.ID, Value: raw}})
	if len(errs) != 1 || errs[0] != nil {
		httpx.Fail(c, 503, "broker_unavailable", "event acknowledgement unavailable")
		return
	}
	c.JSON(202, gin.H{"event_id": env.EventID})
}
