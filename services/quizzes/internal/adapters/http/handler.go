// Package http is the Gin transport of the quizzes service.
package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

// Research replay headers (simulation clock, disabled by default).
const (
	HeaderSimTime     = "X-Sim-Time"
	HeaderActAsUser   = "X-Act-As-User"
	HeaderActAsGroup  = "X-Act-As-Group"
	HeaderIdempotency = "Idempotency-Key"
	maxQuizBody       = 512 << 10
	maxSmallBody      = 8 << 10
)

// RateLimits configures per-IP and per-subject limits.
type RateLimits struct {
	PerIP      int
	PerSubject int
	Window     time.Duration
}

// Deps are the handler dependencies.
type Deps struct {
	Service  *app.Service
	Verifier *authn.Verifier
	Limiter  ratelimit.Limiter
	Logger   *slog.Logger
	SimClock bool
	Limits   RateLimits
}

// Register mounts the API on e.
func Register(e *gin.Engine, d Deps) {
	h := &handler{d: d}
	v1 := e.Group("/v1",
		httpx.RateLimit(d.Limiter, d.Logger, "ip", d.Limits.PerIP, d.Limits.Window, httpx.ByIP),
		httpx.Authenticate(d.Verifier),
		h.resolveActor,
		httpx.RateLimit(d.Limiter, d.Logger, "sub", d.Limits.PerSubject, d.Limits.Window, actorKey),
	)
	v1.POST("/quizzes", h.createQuiz)
	v1.GET("/quizzes", h.listQuizzes)
	v1.GET("/quizzes/:quizId", h.getQuiz)
	v1.PUT("/quizzes/:quizId", h.updateQuiz)
	v1.POST("/quizzes/:quizId/publish", h.publish)
	v1.PUT("/quizzes/:quizId/assignments/:groupId", h.assign)
	v1.GET("/quizzes/:quizId/assignments", h.listAssignments)
	v1.GET("/quizzes/:quizId/attempts", h.listAttempts)
	v1.POST("/quizzes/:quizId/attempts", h.startAttempt)
	v1.GET("/attempts/:attemptId", h.getAttempt)
	v1.POST("/attempts/:attemptId/questions/:questionId/shown", h.markShown)
	v1.PUT("/attempts/:attemptId/answers/:questionId", h.saveAnswer)
	v1.POST("/attempts/:attemptId/submit", h.submit)
}

type handler struct{ d Deps }

const actorKeyName = "examory.actor"

func actorOf(c *gin.Context) app.Actor { return c.MustGet(actorKeyName).(app.Actor) }

func actorKey(c *gin.Context) string {
	if a, ok := c.Get(actorKeyName); ok {
		return "sub:" + a.(app.Actor).UserID
	}
	return ""
}

// resolveActor builds the effective actor. Simulation headers are rejected
// with 400 unless the simulation clock is enabled, and then only a `system`
// caller may use them (403 otherwise).
func (h *handler) resolveActor(c *gin.Context) {
	p, _ := httpx.Principal(c)
	simTime, actUser, actGroup := c.GetHeader(HeaderSimTime), c.GetHeader(HeaderActAsUser), c.GetHeader(HeaderActAsGroup)
	_, hasSim := c.Request.Header[HeaderSimTime]
	_, hasUser := c.Request.Header[HeaderActAsUser]
	_, hasGroup := c.Request.Header[HeaderActAsGroup]
	actor := app.Actor{Principal: p, Clock: domain.Clock{Now: h.d.Service.Now()}}
	if !hasSim && !hasUser && !hasGroup {
		c.Set(actorKeyName, actor)
		c.Next()
		return
	}
	if !h.d.SimClock {
		httpx.Fail(c, http.StatusBadRequest, "simulation_disabled", "simulation headers are not accepted by this deployment")
		return
	}
	if p.Role != authn.RoleSystem {
		httpx.Fail(c, http.StatusForbidden, "forbidden", "simulation headers require the system role")
		return
	}
	if hasSim {
		t, err := time.Parse(time.RFC3339Nano, simTime)
		if err != nil || len(simTime) > 40 {
			httpx.Fail(c, http.StatusBadRequest, "invalid_sim_time", "X-Sim-Time must be RFC 3339")
			return
		}
		actor.Clock = domain.Clock{Now: t.UTC().Truncate(time.Millisecond), Sim: true}
	}
	if hasUser {
		uid, err := ids.Canonical(actUser)
		if err != nil {
			httpx.Fail(c, http.StatusBadRequest, "invalid_act_as", "X-Act-As-User must be a UUID")
			return
		}
		gid := ""
		if hasGroup {
			if gid, err = ids.Canonical(actGroup); err != nil {
				httpx.Fail(c, http.StatusBadRequest, "invalid_act_as", "X-Act-As-Group must be a UUID")
				return
			}
		}
		actor.Principal = authn.Principal{UserID: uid, Role: authn.RoleStudent, GroupID: gid, TokenID: p.TokenID}
		h.d.Logger.DebugContext(c.Request.Context(), "simulated student request", slog.String("system_user", p.UserID), slog.String("act_as", uid))
	} else if hasGroup {
		httpx.Fail(c, http.StatusBadRequest, "invalid_act_as", "X-Act-As-Group requires X-Act-As-User")
		return
	}
	c.Set(actorKeyName, actor)
	c.Next()
}

// fail maps domain errors to problem responses; unknown errors are 500.
func (h *handler) fail(c *gin.Context, err error) {
	var de *domain.Error
	if !errors.As(err, &de) {
		httpx.Internal(c, h.d.Logger, err)
		return
	}
	switch de.Kind {
	case domain.KindInvalid:
		fe := make([]httpx.FieldError, 0, len(de.Fields))
		for _, f := range de.Fields {
			fe = append(fe, httpx.FieldError{Field: f.Field, Message: f.Message})
		}
		httpx.WriteProblem(c, httpx.Problem{Status: http.StatusUnprocessableEntity, Code: de.Code, Detail: de.Msg, Errors: fe})
	case domain.KindNotFound:
		httpx.Fail(c, http.StatusNotFound, de.Code, de.Msg)
	case domain.KindForbidden:
		httpx.Fail(c, http.StatusForbidden, de.Code, de.Msg)
	case domain.KindConflict:
		httpx.Fail(c, http.StatusConflict, de.Code, de.Msg)
	case domain.KindBadRequest:
		httpx.Fail(c, http.StatusBadRequest, de.Code, de.Msg)
	default:
		httpx.Internal(c, h.d.Logger, err)
	}
}

// ---- DTOs ----

type optionIn struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type questionIn struct {
	Text        string     `json:"text"`
	Points      int        `json:"points"`
	TopicID     string     `json:"topic_id"`
	Options     []optionIn `json:"options"`
	CorrectKeys []string   `json:"correct_keys"`
}

type quizIn struct {
	Title        string       `json:"title"`
	CourseID     string       `json:"course_id"`
	TimeLimitSec int          `json:"time_limit_sec"`
	MaxAttempts  int          `json:"max_attempts"`
	ShowScore    *bool        `json:"show_score"`
	Questions    []questionIn `json:"questions"`
}

func (in quizIn) spec() domain.QuizSpec {
	s := domain.QuizSpec{Title: in.Title, CourseID: in.CourseID, TimeLimitSec: in.TimeLimitSec, MaxAttempts: in.MaxAttempts, ShowScore: true}
	if in.ShowScore != nil {
		s.ShowScore = *in.ShowScore
	}
	// Bound collection sizes before copying (validation reports the error).
	qs := in.Questions
	if len(qs) > domain.MaxQuestions+1 {
		qs = qs[:domain.MaxQuestions+1]
	}
	for _, q := range qs {
		qsp := domain.QuestionSpec{Text: q.Text, Points: q.Points, TopicID: q.TopicID, CorrectKeys: q.CorrectKeys}
		opts := q.Options
		if len(opts) > domain.MaxOptions+1 {
			opts = opts[:domain.MaxOptions+1]
		}
		if len(qsp.CorrectKeys) > domain.MaxOptions+1 {
			qsp.CorrectKeys = qsp.CorrectKeys[:domain.MaxOptions+1]
		}
		for _, o := range opts {
			qsp.Options = append(qsp.Options, domain.OptionSpec{Key: o.Key, Text: o.Text})
		}
		s.Questions = append(s.Questions, qsp)
	}
	return s
}

type optionOut struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}

type questionOut struct {
	ID      string      `json:"id"`
	Text    string      `json:"text"`
	Points  int         `json:"points"`
	TopicID *string     `json:"topic_id"`
	Options []optionOut `json:"options"`
}

type variantOut struct {
	ID          string   `json:"id"`
	Ordinal     int      `json:"ordinal"`
	QuestionIDs []string `json:"question_ids"`
	MaxScore    int      `json:"max_score"`
}

// quizOut is the author view (includes the marking key).
type quizOut struct {
	ID           string        `json:"id"`
	OwnerID      string        `json:"owner_id"`
	CourseID     string        `json:"course_id"`
	Title        string        `json:"title"`
	TimeLimitSec int           `json:"time_limit_sec"`
	MaxAttempts  int           `json:"max_attempts"`
	ShowScore    bool          `json:"show_score"`
	Status       string        `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	PublishedAt  *time.Time    `json:"published_at"`
	Questions    []questionOut `json:"questions,omitempty"`
	Variants     []variantOut  `json:"variants,omitempty"`
}

func quizView(q *domain.Quiz, full bool) quizOut {
	o := quizOut{ID: q.ID, OwnerID: q.OwnerID, CourseID: q.CourseID, Title: q.Title, TimeLimitSec: q.TimeLimitSec,
		MaxAttempts: q.MaxAttempts, ShowScore: q.ShowScore, Status: q.Status, CreatedAt: q.CreatedAt,
		UpdatedAt: q.UpdatedAt, PublishedAt: q.PublishedAt}
	if !full {
		return o
	}
	for _, qs := range q.Questions {
		qo := questionOut{ID: qs.ID, Text: qs.Text, Points: qs.Points, Options: []optionOut{}}
		if qs.TopicID != "" {
			t := qs.TopicID
			qo.TopicID = &t
		}
		for _, op := range qs.Options {
			qo.Options = append(qo.Options, optionOut{ID: op.ID, Text: op.Text, Correct: op.Correct})
		}
		o.Questions = append(o.Questions, qo)
	}
	for _, v := range q.Variants {
		vo := variantOut{ID: v.ID, Ordinal: v.Ordinal, MaxScore: v.MaxScore(), QuestionIDs: []string{}}
		for _, qs := range v.Questions {
			vo.QuestionIDs = append(vo.QuestionIDs, qs.ID)
		}
		o.Variants = append(o.Variants, vo)
	}
	return o
}

func page(items any, next string) gin.H {
	var n *string
	if next != "" {
		n = &next
	}
	return gin.H{"items": items, "next_cursor": n}
}

// ---- authoring ----

func (h *handler) createQuiz(c *gin.Context) {
	var in quizIn
	if !httpx.BindJSON(c, &in, maxQuizBody) {
		return
	}
	q, err := h.d.Service.CreateQuiz(c.Request.Context(), actorOf(c), in.spec())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, quizView(q, true))
}

func (h *handler) updateQuiz(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	var in quizIn
	if !httpx.BindJSON(c, &in, maxQuizBody) {
		return
	}
	q, err := h.d.Service.UpdateQuiz(c.Request.Context(), actorOf(c), id, in.spec())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, quizView(q, true))
}

func (h *handler) publish(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	q, err := h.d.Service.PublishQuiz(c.Request.Context(), actorOf(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, quizView(q, false))
}

func (h *handler) listQuizzes(c *gin.Context) {
	limit, cur, ok := httpx.Page(c, 20)
	if !ok {
		return
	}
	a := actorOf(c)
	if a.Role == authn.RoleStudent {
		items, err := h.d.Service.ListForStudent(c.Request.Context(), a, limit, cur)
		if err != nil {
			h.fail(c, err)
			return
		}
		out := make([]gin.H, 0, len(items))
		next := ""
		for _, it := range items {
			out = append(out, gin.H{"id": it.Quiz.ID, "title": it.Quiz.Title, "course_id": it.Quiz.CourseID,
				"time_limit_sec": it.Quiz.TimeLimitSec, "max_attempts": it.Quiz.MaxAttempts,
				"assignment": app.AssignmentView{GroupID: it.Assignment.GroupID, OpensAt: it.Assignment.OpensAt, ClosesAt: it.Assignment.ClosesAt}})
			next = it.Quiz.ID
		}
		if len(items) < limit {
			next = ""
		}
		c.JSON(http.StatusOK, page(out, next))
		return
	}
	items, err := h.d.Service.ListForAuthor(c.Request.Context(), a, limit, cur)
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]quizOut, 0, len(items))
	next := ""
	for i := range items {
		out = append(out, quizView(&items[i], false))
		next = items[i].ID
	}
	if len(items) < limit {
		next = ""
	}
	c.JSON(http.StatusOK, page(out, next))
}

func (h *handler) getQuiz(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	a := actorOf(c)
	if a.Role == authn.RoleStudent {
		sq, err := h.d.Service.GetQuizForStudent(c.Request.Context(), a, id)
		if err != nil {
			h.fail(c, err)
			return
		}
		c.JSON(http.StatusOK, sq)
		return
	}
	q, err := h.d.Service.GetQuizForAuthor(c.Request.Context(), a, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, quizView(q, true))
}

type assignIn struct {
	OpensAt  time.Time `json:"opens_at"`
	ClosesAt time.Time `json:"closes_at"`
}

func (h *handler) assign(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	gid, ok := httpx.PathID(c, "groupId")
	if !ok {
		return
	}
	var in assignIn
	if !httpx.BindJSON(c, &in, maxSmallBody) {
		return
	}
	asg, err := h.d.Service.Assign(c.Request.Context(), actorOf(c), id, gid, in.OpensAt, in.ClosesAt)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"quiz_id": asg.QuizID, "group_id": asg.GroupID, "opens_at": asg.OpensAt, "closes_at": asg.ClosesAt})
}

func (h *handler) listAssignments(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	items, err := h.d.Service.ListAssignments(c.Request.Context(), actorOf(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]app.AssignmentView, 0, len(items))
	for _, a := range items {
		out = append(out, app.AssignmentView{GroupID: a.GroupID, OpensAt: a.OpensAt, ClosesAt: a.ClosesAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *handler) listAttempts(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	limit, cur, ok := httpx.Page(c, 50)
	if !ok {
		return
	}
	items, err := h.d.Service.ListQuizAttempts(c.Request.Context(), actorOf(c), id, limit, cur)
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	next := ""
	for _, a := range items {
		out = append(out, gin.H{"id": a.ID, "user_id": a.UserID, "group_id": a.GroupID, "attempt_no": a.AttemptNo,
			"status": a.Status, "started_at": a.StartedAt, "finished_at": a.FinishedAt, "score": a.Score,
			"max_score": a.MaxScore, "answers_changed": a.AnswersChanged, "simulated": a.Sim})
		next = a.ID
	}
	if len(items) < limit {
		next = ""
	}
	c.JSON(http.StatusOK, page(out, next))
}

// ---- attempts ----

func (h *handler) startAttempt(c *gin.Context) {
	id, ok := httpx.PathID(c, "quizId")
	if !ok {
		return
	}
	v, err := h.d.Service.StartAttempt(c.Request.Context(), actorOf(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", "/v1/attempts/"+v.ID)
	c.JSON(http.StatusCreated, v)
}

func (h *handler) getAttempt(c *gin.Context) {
	id, ok := httpx.PathID(c, "attemptId")
	if !ok {
		return
	}
	v, err := h.d.Service.GetAttempt(c.Request.Context(), actorOf(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *handler) markShown(c *gin.Context) {
	id, ok := httpx.PathID(c, "attemptId")
	if !ok {
		return
	}
	qid, ok := httpx.PathID(c, "questionId")
	if !ok {
		return
	}
	at, err := h.d.Service.MarkShown(c.Request.Context(), actorOf(c), id, qid)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"question_id": qid, "first_shown_at": at})
}

type answerIn struct {
	SelectedOptionIDs []string `json:"selected_option_ids"`
}

func (h *handler) saveAnswer(c *gin.Context) {
	id, ok := httpx.PathID(c, "attemptId")
	if !ok {
		return
	}
	qid, ok := httpx.PathID(c, "questionId")
	if !ok {
		return
	}
	var in answerIn
	if !httpx.BindJSON(c, &in, maxSmallBody) {
		return
	}
	if len(in.SelectedOptionIDs) > domain.MaxOptions {
		httpx.Invalid(c, "too many options selected", []httpx.FieldError{{Field: "selected_option_ids", Message: "at most 8 ids"}})
		return
	}
	for _, s := range in.SelectedOptionIDs {
		if len(s) > 64 || strings.ContainsAny(s, "\x00") {
			httpx.Invalid(c, "invalid option id", []httpx.FieldError{{Field: "selected_option_ids", Message: "must be option ids"}})
			return
		}
	}
	ack, err := h.d.Service.SaveAnswer(c.Request.Context(), actorOf(c), id, qid, in.SelectedOptionIDs)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ack)
}

func (h *handler) submit(c *gin.Context) {
	id, ok := httpx.PathID(c, "attemptId")
	if !ok {
		return
	}
	r, err := h.d.Service.Submit(c.Request.Context(), actorOf(c), id, c.GetHeader(HeaderIdempotency))
	if err != nil {
		h.fail(c, err)
		return
	}
	if r.Replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(http.StatusOK, r)
}
