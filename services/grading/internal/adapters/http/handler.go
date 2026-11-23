// Package http is the Gin transport of grading. Every route requires a bearer
// token; risk signals are visible to teachers and admins only, and only
// teachers review (TS §2). Students never see risk signals (403).
package http

import (
	"encoding/json"
	"errors"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

// Notice accompanies every risk response (FR-26, human-in-the-loop).
const Notice = "A risk signal is a relative ranking for teacher review, not a finding of misconduct. " +
	"It does not change any grade. Absence of a signal is not evidence of integrity."

// Deps are the handler dependencies.
type Deps struct {
	Service    *app.Service
	Verifier   *authn.Verifier
	Limiter    ratelimit.Limiter
	Logger     *slog.Logger
	PerIP      int
	PerSubject int
}

// Register mounts the API.
func Register(e *gin.Engine, d Deps) {
	h := &handler{d: d}
	v1 := e.Group("/v1",
		httpx.RateLimit(d.Limiter, d.Logger, "ip", d.PerIP, time.Minute, httpx.ByIP),
		httpx.Authenticate(d.Verifier),
		httpx.RateLimit(d.Limiter, d.Logger, "sub", d.PerSubject, time.Minute, httpx.BySubject),
		httpx.RequireRoles(authn.RoleTeacher, authn.RoleAdmin),
	)
	v1.GET("/risks", h.list)
	v1.GET("/risks/:riskId", h.get)
	v1.POST("/risks/:riskId/review", h.review)
}

type handler struct{ d Deps }

func (h *handler) fail(c *gin.Context, err error) {
	if errors.Is(err, courseaccess.ErrUnavailable) {
		httpx.Fail(c, 503, "courses_unavailable", "course membership is unavailable")
		return
	}
	var de *domain.Error
	if !errors.As(err, &de) {
		httpx.Internal(c, h.d.Logger, err)
		return
	}
	status := map[domain.Kind]int{
		domain.KindInvalid: http.StatusUnprocessableEntity, domain.KindNotFound: http.StatusNotFound,
		domain.KindForbidden: http.StatusForbidden, domain.KindConflict: http.StatusConflict,
		domain.KindPrecondition: http.StatusPreconditionFailed, domain.KindPreconditionRequired: http.StatusPreconditionRequired,
		domain.KindBadRequest: http.StatusBadRequest,
	}[de.Kind]
	if status == 0 {
		httpx.Internal(c, h.d.Logger, err)
		return
	}
	httpx.Fail(c, status, de.Code, de.Msg)
}

type attemptOut struct {
	AttemptID      string    `json:"attempt_id"`
	StudentID      string    `json:"student_id"`
	QuizID         string    `json:"quiz_id"`
	VariantID      string    `json:"variant_id"`
	CourseID       string    `json:"course_id"`
	GroupID        string    `json:"group_id"`
	Score          int       `json:"score"`
	MaxScore       int       `json:"max_score"`
	DurationMs     int64     `json:"duration_ms"`
	AnswersChanged int       `json:"answers_changed"`
	Reason         string    `json:"reason"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type reviewOut struct {
	Decision   string    `json:"decision"`
	Comment    string    `json:"comment"`
	ReviewerID string    `json:"reviewer_id"`
	ReviewedAt time.Time `json:"reviewed_at"`
}

type riskOut struct {
	ID           string          `json:"id"`
	RiskID       string          `json:"risk_id"`
	Category     string          `json:"category"`
	TargetType   string          `json:"target_type"`
	TargetID     string          `json:"target_id"`
	Score        float64         `json:"score"`
	Level        string          `json:"level"`
	Model        gin.H           `json:"model"`
	Explanation  string          `json:"explanation"`
	Evidence     json.RawMessage `json:"evidence,omitempty"`
	CourseID     *string         `json:"course_id"`
	GroupID      *string         `json:"group_id"`
	Status       string          `json:"status"`
	Version      int             `json:"version"`
	OccurredAt   time.Time       `json:"occurred_at"`
	ReceivedAt   time.Time       `json:"received_at"`
	ProcessingMs int64           `json:"processing_ms"`
	Attempt      *attemptOut     `json:"attempt"`
	Review       *reviewOut      `json:"review"`
	Notice       string          `json:"notice"`
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func view(row *app.RiskRow, withEvidence bool) riskOut {
	r := row.Risk
	o := riskOut{ID: r.ID, RiskID: r.RiskID, Category: r.Category, TargetType: r.TargetType, TargetID: r.TargetID,
		Score: r.Score, Level: r.Level, Model: gin.H{"name": r.ModelName, "version": r.ModelVersion},
		Explanation: r.Explanation, CourseID: ptr(r.CourseID), GroupID: ptr(r.GroupID), Status: r.Status,
		Version: r.Version, OccurredAt: r.OccurredAt, ReceivedAt: r.ReceivedAt, Notice: Notice}
	if a := row.Attempt; a != nil {
		o.Attempt = &attemptOut{a.AttemptID, a.StudentID, a.QuizID, a.VariantID, a.CourseID, a.GroupID, a.Score,
			a.MaxScore, a.DurationMs, a.AnswersChanged, a.Reason, a.SubmittedAt}
		// FR-24 processing time: from submission to receipt of the signal.
		if d := r.ReceivedAt.Sub(a.SubmittedAt); d > 0 {
			o.ProcessingMs = d.Milliseconds()
		}
	}
	if rv := row.Review; rv != nil {
		o.Review = &reviewOut{rv.Decision, rv.Comment, rv.ReviewerID, rv.ReviewedAt}
	}
	if withEvidence {
		o.Evidence = r.Evidence
	}
	return o
}

var opaque = func(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	return strings.Trim(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") == ""
}

func (h *handler) list(c *gin.Context) {
	limit, cur, ok := httpx.Page(c, 20)
	if !ok {
		return
	}
	f := app.Filter{Status: c.Query("status"), CourseID: c.Query("course_id"), QuizID: c.Query("quiz_id")}
	if f.Status != "" && f.Status != domain.StatusOpen && f.Status != domain.StatusConfirmed && f.Status != domain.StatusDismissed {
		httpx.Fail(c, http.StatusBadRequest, "invalid_parameter", "status must be open, confirmed or dismissed")
		return
	}
	for _, v := range []string{f.CourseID, f.QuizID} {
		if v != "" && !opaque(v) {
			httpx.Fail(c, http.StatusBadRequest, "invalid_parameter", "course_id and quiz_id must be ids")
			return
		}
	}
	p, _ := httpx.Principal(c)
	rows, err := h.d.Service.ListRisks(c.Request.Context(), p, f, limit, cur)
	if err != nil {
		h.fail(c, err)
		return
	}
	items := make([]riskOut, 0, len(rows))
	for i := range rows {
		items = append(items, view(&rows[i], false))
	}
	var next *string
	if len(rows) == limit {
		next = &rows[len(rows)-1].Risk.ID
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": next, "notice": Notice})
}

func (h *handler) get(c *gin.Context) {
	id, ok := httpx.PathID(c, "riskId")
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	row, err := h.d.Service.GetRisk(c.Request.Context(), p, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("ETag", etag(row.Risk.Version))
	c.JSON(http.StatusOK, view(row, true))
}

func etag(v int) string { return `"v` + strconv.Itoa(v) + `"` }

// parseIfMatch accepts `"v<n>"` (optionally weak-prefixed); 0 when absent,
// -1 when malformed.
func parseIfMatch(h string) int {
	h = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "W/"))
	if h == "" {
		return 0
	}
	if len(h) < 4 || !strings.HasPrefix(h, `"v`) || !strings.HasSuffix(h, `"`) {
		return -1
	}
	n, err := strconv.Atoi(h[2 : len(h)-1])
	if err != nil || n < 1 || n > 1_000_000 {
		return -1
	}
	return n
}

type reviewIn struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

func (h *handler) review(c *gin.Context) {
	id, ok := httpx.PathID(c, "riskId")
	if !ok {
		return
	}
	var in reviewIn
	if !httpx.BindJSON(c, &in, 16<<10) {
		return
	}
	ifMatch := parseIfMatch(c.GetHeader("If-Match"))
	if ifMatch < 0 {
		httpx.Fail(c, http.StatusBadRequest, "invalid_if_match", `If-Match must be an ETag such as "v1"`)
		return
	}
	p, _ := httpx.Principal(c)
	if !ids.Valid(p.UserID) {
		httpx.Fail(c, http.StatusForbidden, "forbidden", "invalid principal")
		return
	}
	res, err := h.d.Service.Review(c.Request.Context(), p, id, in.Decision, in.Comment, ifMatch, c.GetHeader("Idempotency-Key"))
	if err != nil {
		h.fail(c, err)
		return
	}
	if res.Replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.Header("ETag", etag(res.Row.Risk.Version))
	c.JSON(http.StatusOK, view(res.Row, false))
}
