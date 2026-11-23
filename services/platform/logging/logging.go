// Package logging builds the structured JSON logger used by every service.
//
// Rules enforced here (NFR-7):
//   - output is JSON (log/slog) with a "service" attribute;
//   - the request id stored in the context is attached to every record logged
//     with a *Context method (InfoContext, ErrorContext, ...);
//   - attributes whose key looks sensitive (password, token, secret, cookie,
//     authorization, answer, dsn, seed, private) are replaced by "[REDACTED]"
//     regardless of nesting, as a last line of defence. Code must still never
//     pass secrets to the logger.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type ctxKey struct{}

// WithRequestID stores a request id in ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID returns the request id stored in ctx, or "".
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

var sensitive = []string{
	"password", "passwd", "secret", "token", "authorization", "cookie",
	"answer", "dsn", "seed", "private", "credential", "selected",
}

// IsSensitiveKey reports whether a log key must be redacted.
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	if k == "answer_hash" { // a one-way digest defined by the event contract
		return false
	}
	for _, s := range sensitive {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// Redacted is the replacement value for sensitive attributes.
const Redacted = "[REDACTED]"

// New returns a JSON logger writing to w at the given level
// ("debug", "info", "warn", "error"; default info).
func New(w io.Writer, service, level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lv,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if IsSensitiveKey(a.Key) {
				return slog.String(a.Key, Redacted)
			}
			return a
		},
	})
	return slog.New(&contextHandler{Handler: h}).With(slog.String("service", service))
}

// Discard returns a logger that drops everything (tests).
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

type contextHandler struct{ slog.Handler }

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
