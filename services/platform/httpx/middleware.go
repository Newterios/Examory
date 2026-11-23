package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

// HeaderRequestID is the request-id header propagated by the gateway.
const HeaderRequestID = "X-Request-ID"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID accepts a well-formed incoming X-Request-ID (set by the gateway)
// or generates a UUIDv7, stores it in the request context and echoes it.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if !requestIDPattern.MatchString(id) {
			id = ids.NewV7()
		}
		c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), id))
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

// Recovery converts panics into a generic 500 problem. The panic value and
// stack are logged server-side only.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(r)
				}
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("panic", r), slog.String("stack", string(debug.Stack())))
				if !c.Writer.Written() {
					Fail(c, http.StatusInternalServerError, "internal", "an internal error occurred")
				} else {
					c.Abort()
				}
			}
		}()
		c.Next()
	}
}

// SecurityHeaders sets conservative headers for a JSON API. Responses default
// to Cache-Control: no-store; a handler may override it (e.g. JWKS).
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cache-Control", "no-store")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit caps the request body size. Requests that declare a larger
// Content-Length are rejected immediately with 413.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > n {
			Fail(c, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		}
		c.Next()
	}
}

// CORS implements an exact-match origin allow-list. Disallowed origins get no
// CORS headers (and 403 on preflight). Credentials are only allowed when
// allowCredentials is true.
func CORS(origins []string, allowCredentials bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		c.Writer.Header().Add("Vary", "Origin")
		allowed := slices.Contains(origins, origin)
		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			if !allowed {
				Fail(c, http.StatusForbidden, "cors_forbidden", "origin not allowed")
				return
			}
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			h.Set("Access-Control-Max-Age", "600")
			if allowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
			if allowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}
		c.Next()
	}
}

// AccessLog logs one line per request. The query string is never logged
// (it may carry identifiers); the route template is logged instead of the
// raw path when available.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
		}
		if p, ok := authn.FromContext(c.Request.Context()); ok {
			attrs = append(attrs, slog.String("user_id", p.UserID), slog.String("role", string(p.Role)))
		}
		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		}
		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// KeyFunc derives a rate-limit key from a request; "" skips limiting.
type KeyFunc func(c *gin.Context) string

// ByIP keys by client IP (as resolved through the trusted proxies).
func ByIP(c *gin.Context) string { return "ip:" + c.ClientIP() }

// BySubject keys by the authenticated user id; unauthenticated requests are
// not limited by this key (use ByIP for them).
func BySubject(c *gin.Context) string {
	if p, ok := authn.FromContext(c.Request.Context()); ok {
		return "sub:" + p.UserID
	}
	return ""
}

// RateLimit enforces limit requests per window for the key returned by key.
// If the limiter returns an error the request is allowed and the error
// logged; use ratelimit.Fallback to keep limiting during a Redis outage.
func RateLimit(l ratelimit.Limiter, logger *slog.Logger, name string, limit int, window time.Duration, key KeyFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		k := key(c)
		if k == "" {
			c.Next()
			return
		}
		d, err := l.Allow(c.Request.Context(), name+":"+k, limit, window)
		if err != nil {
			logger.WarnContext(c.Request.Context(), "rate limiter error", slog.String("limiter", name), slog.String("error", err.Error()))
			c.Next()
			return
		}
		if !d.Allowed {
			c.Header("Retry-After", strconv.Itoa(int(d.RetryAfter.Round(time.Second)/time.Second)+1))
			Fail(c, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		c.Next()
	}
}

// Authenticate requires a valid bearer access token and stores the principal
// in the request context. Failures return 401 without detail.
func Authenticate(v *authn.Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		scheme, tok, ok := strings.Cut(h, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(tok) == "" {
			c.Header("WWW-Authenticate", `Bearer`)
			Fail(c, http.StatusUnauthorized, "unauthenticated", "a valid bearer token is required")
			return
		}
		p, err := v.Verify(c.Request.Context(), strings.TrimSpace(tok))
		if err != nil {
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			Fail(c, http.StatusUnauthorized, "unauthenticated", "a valid bearer token is required")
			return
		}
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), p))
		c.Next()
	}
}

// RequireRoles allows only principals with one of roles (403 otherwise).
// It must run after Authenticate.
func RequireRoles(roles ...authn.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := authn.FromContext(c.Request.Context())
		if !ok {
			Fail(c, http.StatusUnauthorized, "unauthenticated", "a valid bearer token is required")
			return
		}
		if !p.Is(roles...) {
			Fail(c, http.StatusForbidden, "forbidden", "this action is not allowed for your role")
			return
		}
		c.Next()
	}
}

// Principal returns the authenticated principal of the request.
func Principal(c *gin.Context) (authn.Principal, bool) {
	return authn.FromContext(c.Request.Context())
}
