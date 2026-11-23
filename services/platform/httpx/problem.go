// Package httpx contains the HTTP building blocks shared by all services:
// a hardened Gin engine, server bootstrap with safe timeouts and graceful
// shutdown, middleware, strict JSON decoding, RFC 9457 problem responses and
// health endpoints.
package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/logging"
)

// ProblemContentType is the RFC 9457 media type.
const ProblemContentType = "application/problem+json"

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem is an RFC 9457 problem details object. Code is a stable machine
// readable identifier; Detail is a short human-readable message that never
// contains internal error text.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// WriteProblem writes p and aborts the Gin chain.
func WriteProblem(c *gin.Context, p Problem) {
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}
	if p.Type == "" {
		p.Type = "urn:examory:problem:" + p.Code
	}
	p.RequestID = logging.RequestID(c.Request.Context())
	c.Header("Content-Type", ProblemContentType)
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(p.Status, p)
}

// Fail writes a problem with the given status, code and detail.
func Fail(c *gin.Context, status int, code, detail string) {
	WriteProblem(c, Problem{Status: status, Code: code, Detail: detail})
}

// Invalid writes a 422 problem listing field errors.
func Invalid(c *gin.Context, detail string, errs []FieldError) {
	WriteProblem(c, Problem{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Detail: detail, Errors: errs})
}

// Internal logs err with the request id and writes a generic 500. The error
// text is never sent to the client.
func Internal(c *gin.Context, logger *slog.Logger, err error) {
	logger.ErrorContext(c.Request.Context(), "request failed",
		slog.String("method", c.Request.Method), slog.String("route", c.FullPath()), slog.String("error", err.Error()))
	Fail(c, http.StatusInternalServerError, "internal", "an internal error occurred")
}
