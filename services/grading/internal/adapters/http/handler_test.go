package http

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/grading/internal/adapters/schemas"
	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/app/apptest"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

func setup(t *testing.T) (http.Handler, *authn.Signer, string) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	clk := clock.NewFake(time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC))
	signer, _ := authn.NewSigner("k", priv, "examory-auth", "examory", 10*time.Minute, clk)
	ver, _ := authn.NewVerifier(authn.StaticKeys{"k": pub}, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory", Clock: clk})
	val, err := schemas.Validator()
	if err != nil {
		t.Fatal(err)
	}
	st := apptest.NewStore()
	svc := app.New(st, val, clk, logging.Discard())
	attempt := ids.NewV7()
	env, _ := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"},
		events.Context{CourseID: events.Ptr("c_10")}, map[string]any{
			"risk_id": "risk_1", "target_type": "attempt", "target_id": attempt, "score": 0.99, "level": "high",
			"model": map[string]any{"name": "m", "version": "1"}, "explanation": "fast answers",
			"evidence": map[string]any{"features": map[string]any{"mean_answer_time_ms": 400.0, "z_score": nil, "synchronous_with": []string{}}},
		}, clk.Now())
	if err := st.Tx(func(tx app.Tx) error { return svc.Ingest(context.Background(), tx, env, events.TopicRisk) }); err != nil {
		t.Fatal(err)
	}
	e, _ := httpx.NewEngine(httpx.EngineConfig{Logger: logging.Discard()})
	Register(e, Deps{Service: svc, Verifier: ver, Limiter: ratelimit.NewMemory(clk), Logger: logging.Discard(), PerIP: 1000, PerSubject: 1000})
	return e, signer, attempt
}

func call(h http.Handler, method, path, tok string, body any, hdr map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

func TestRiskAPI(t *testing.T) {
	h, s, attempt := setup(t)
	tok := func(role authn.Role) string { x, _, _ := s.Issue(ids.NewV7(), role, ""); return x }
	student, teacher, admin, sys := tok(authn.RoleStudent), tok(authn.RoleTeacher), tok(authn.RoleAdmin), tok(authn.RoleSystem)

	if w, _ := call(h, "GET", "/v1/risks", "", nil, nil); w.Code != 401 {
		t.Fatalf("anonymous %d", w.Code)
	}
	for _, tk := range []string{student, sys} {
		if w, _ := call(h, "GET", "/v1/risks", tk, nil, nil); w.Code != 403 {
			t.Fatalf("non-teacher list %d", w.Code)
		}
	}
	w, m := call(h, "GET", "/v1/risks?status=open", teacher, nil, nil)
	if w.Code != 200 || len(m["items"].([]any)) != 1 || !strings.Contains(m["notice"].(string), "not a finding of misconduct") {
		t.Fatalf("list %d %v", w.Code, m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	if _, has := item["evidence"]; has {
		t.Fatal("list must not carry evidence (only the audited detail view does)")
	}
	id := item["id"].(string)
	for _, q := range []string{"?status=bogus", "?limit=1000", "?course_id=a%20b"} {
		if w, _ := call(h, "GET", "/v1/risks"+q, teacher, nil, nil); w.Code != 400 {
			t.Fatalf("%s accepted: %d", q, w.Code)
		}
	}
	w, m = call(h, "GET", "/v1/risks/"+id, admin, nil, nil)
	if w.Code != 200 || w.Header().Get("ETag") != `"v1"` || m["evidence"] == nil || m["target_id"] != attempt {
		t.Fatalf("detail %d %v", w.Code, m)
	}
	if w, _ := call(h, "GET", "/v1/risks/"+id, student, nil, nil); w.Code != 403 {
		t.Fatalf("student detail %d", w.Code)
	}
	body := map[string]any{"decision": "confirmed", "comment": "every answer under a second"}
	key := map[string]string{"Idempotency-Key": "review-key-1", "If-Match": `"v1"`}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", admin, body, key); w.Code != 403 {
		t.Fatalf("admin review %d (TS §2: admins have no academic decision authority)", w.Code)
	}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", teacher, body, map[string]string{"Idempotency-Key": "review-key-1"}); w.Code != 428 {
		t.Fatalf("missing If-Match %d", w.Code)
	}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", teacher, body, map[string]string{"Idempotency-Key": "review-key-1", "If-Match": `"v9"`}); w.Code != 412 {
		t.Fatalf("stale If-Match %d", w.Code)
	}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", teacher, map[string]any{"decision": "confirmed", "comment": "x", "grade": 0}, key); w.Code != 400 {
		t.Fatalf("unknown field %d", w.Code)
	}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", teacher, map[string]any{"decision": "guilty", "comment": "x"}, key); w.Code != 422 {
		t.Fatalf("bad decision %d", w.Code)
	}
	w, m = call(h, "POST", "/v1/risks/"+id+"/review", teacher, body, key)
	if w.Code != 200 || m["status"] != "confirmed" || w.Header().Get("ETag") != `"v2"` {
		t.Fatalf("review %d %v", w.Code, m)
	}
	if w, _ := call(h, "POST", "/v1/risks/"+id+"/review", teacher, body, key); w.Code != 200 || w.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay %d", w.Code)
	}
	if w, m := call(h, "POST", "/v1/risks/"+id+"/review", teacher, map[string]any{"decision": "dismissed", "comment": "y"},
		map[string]string{"Idempotency-Key": "review-key-2", "If-Match": `"v2"`}); w.Code != 409 || m["code"] != "already_reviewed" {
		t.Fatalf("second review %d %v", w.Code, m)
	}
	if w, _ := call(h, "GET", "/v1/risks/not-a-uuid", teacher, nil, nil); w.Code != 404 {
		t.Fatalf("malformed id %d", w.Code)
	}
}

func TestParseIfMatch(t *testing.T) {
	for in, want := range map[string]int{"": 0, `"v1"`: 1, `W/"v12"`: 12, `v1`: -1, `"v0"`: -1, `"vx"`: -1, `*`: -1} {
		if got := parseIfMatch(in); got != want {
			t.Errorf("%q -> %d want %d", in, got, want)
		}
	}
}
