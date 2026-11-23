package httpx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

func engine(t *testing.T) *gin.Engine {
	t.Helper()
	e, err := NewEngine(EngineConfig{Logger: logging.Discard(), MaxBodyBytes: 1024, CORSOrigins: []string{"https://app.example"}})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func do(e http.Handler, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

type payload struct {
	Name string `json:"name"`
}

func TestDecodeJSON(t *testing.T) {
	e := engine(t)
	e.POST("/x", func(c *gin.Context) {
		var p payload
		if !BindJSON(c, &p, 512) {
			return
		}
		c.JSON(200, p)
	})
	ct := map[string]string{"Content-Type": "application/json"}
	cases := []struct {
		name, body string
		hdr        map[string]string
		status     int
		code       string
	}{
		{"ok", `{"name":"a"}`, ct, 200, ""},
		{"ok charset", `{"name":"a"}`, map[string]string{"Content-Type": "application/json; charset=utf-8"}, 200, ""},
		{"no content type", `{"name":"a"}`, nil, 415, "unsupported_media_type"},
		{"form", `name=a`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 415, "unsupported_media_type"},
		{"unknown field", `{"name":"a","admin":true}`, ct, 400, "unknown_field"},
		{"two objects", `{"name":"a"}{"name":"b"}`, ct, 400, "malformed_json"},
		{"array", `[{"name":"a"}]`, ct, 400, "malformed_json"},
		{"null", `null`, ct, 400, "malformed_json"},
		{"wrong type", `{"name":5}`, ct, 400, "malformed_json"},
		{"bad utf8", "{\"name\":\"\xff\"}", ct, 400, "malformed_json"},
		{"too large for handler", `{"name":"` + strings.Repeat("a", 600) + `"}`, ct, 413, "body_too_large"},
		{"too large for engine", `{"name":"` + strings.Repeat("a", 2000) + `"}`, ct, 413, "body_too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(e, "POST", "/x", tc.body, tc.hdr)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body)
			}
			if tc.code != "" {
				var p Problem
				_ = json.Unmarshal(w.Body.Bytes(), &p)
				if p.Code != tc.code || w.Header().Get("Content-Type") != ProblemContentType || p.RequestID == "" {
					t.Fatalf("problem %+v ct=%s", p, w.Header().Get("Content-Type"))
				}
			}
		})
	}
}

func TestRecoveryAndHeaders(t *testing.T) {
	e := engine(t)
	e.GET("/panic", func(*gin.Context) { panic(errors.New("secret internal detail")) })
	w := do(e, "GET", "/panic", "", nil)
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret internal detail") {
		t.Fatalf("panic leaked: %d %s", w.Code, w.Body)
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Cache-Control", HeaderRequestID} {
		if w.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if w := do(e, "GET", "/nope", "", nil); w.Code != 404 || w.Header().Get("Content-Type") != ProblemContentType {
		t.Fatalf("404 not a problem: %d", w.Code)
	}
	// Request id: valid incoming id is kept, junk is replaced.
	w = do(e, "GET", "/nope", "", map[string]string{HeaderRequestID: "abcdefgh-123"})
	if w.Header().Get(HeaderRequestID) != "abcdefgh-123" {
		t.Fatal("request id not propagated")
	}
	w = do(e, "GET", "/nope", "", map[string]string{HeaderRequestID: "<script>"})
	if !ids.Valid(w.Header().Get(HeaderRequestID)) {
		t.Fatal("junk request id not replaced")
	}
}

func TestCORS(t *testing.T) {
	e := engine(t)
	e.GET("/x", func(c *gin.Context) { c.Status(204) })
	pre := map[string]string{"Origin": "https://app.example", "Access-Control-Request-Method": "POST"}
	if w := do(e, "OPTIONS", "/x", "", pre); w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("preflight %d %v", w.Code, w.Header())
	}
	pre["Origin"] = "https://evil.example"
	if w := do(e, "OPTIONS", "/x", "", pre); w.Code != 403 {
		t.Fatalf("evil preflight %d", w.Code)
	}
	if w := do(e, "GET", "/x", "", map[string]string{"Origin": "https://evil.example"}); w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("evil origin reflected")
	}
}

func TestAuthenticateAndRoles(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	clk := clock.NewFake(time.Now())
	s, _ := authn.NewSigner("k", priv, "iss", "aud", time.Minute, clk)
	v, _ := authn.NewVerifier(authn.StaticKeys{"k": pub}, authn.VerifierConfig{Issuer: "iss", Audience: "aud", Clock: clk})
	e := engine(t)
	e.GET("/t", Authenticate(v), RequireRoles(authn.RoleTeacher), func(c *gin.Context) {
		p, _ := Principal(c)
		c.String(200, p.UserID)
	})
	student, _, _ := s.Issue(ids.NewV7(), authn.RoleStudent, "")
	teacherID := ids.NewV7()
	teacher, _, _ := s.Issue(teacherID, authn.RoleTeacher, "")
	if w := do(e, "GET", "/t", "", nil); w.Code != 401 {
		t.Fatalf("no token %d", w.Code)
	}
	if w := do(e, "GET", "/t", "", map[string]string{"Authorization": "Bearer junk"}); w.Code != 401 {
		t.Fatalf("junk token %d", w.Code)
	}
	if w := do(e, "GET", "/t", "", map[string]string{"Authorization": "Bearer " + student}); w.Code != 403 {
		t.Fatalf("student %d", w.Code)
	}
	if w := do(e, "GET", "/t", "", map[string]string{"Authorization": "bearer " + teacher}); w.Code != 200 || w.Body.String() != teacherID {
		t.Fatalf("teacher %d %s", w.Code, w.Body)
	}
}

func TestRateLimit(t *testing.T) {
	e := engine(t)
	l := ratelimit.NewMemory(clock.NewFake(time.Now()))
	e.GET("/r", RateLimit(l, logging.Discard(), "t", 2, time.Minute, ByIP), func(c *gin.Context) { c.Status(204) })
	for i := 0; i < 2; i++ {
		if w := do(e, "GET", "/r", "", nil); w.Code != 204 {
			t.Fatalf("hit %d: %d", i, w.Code)
		}
	}
	w := do(e, "GET", "/r", "", nil)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d", w.Code)
	}
}

func TestPage(t *testing.T) {
	e := engine(t)
	e.GET("/p", func(c *gin.Context) {
		l, cur, ok := Page(c, 20)
		if ok {
			c.JSON(200, gin.H{"l": l, "c": cur})
		}
	})
	for q, want := range map[string]int{"": 200, "?limit=100": 200, "?limit=101": 400, "?limit=0": 400, "?cursor=x": 400, "?limit=abc": 400} {
		if w := do(e, "GET", "/p"+q, "", nil); w.Code != want {
			t.Errorf("%q: %d want %d", q, w.Code, want)
		}
	}
}

func TestServeGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, ServerConfig{Addr: "127.0.0.1:0"}, http.NotFoundHandler(), logging.Discard())
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
