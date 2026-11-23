package http

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/app/apptest"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

type env struct {
	t      *testing.T
	e      *gin.Engine
	signer *authn.Signer
	store  *apptest.Store
	clk    *clock.Fake
}

func newEnv(t *testing.T, sim bool) *env {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	clk := clock.NewFake(t0)
	signer, _ := authn.NewSigner("k1", priv, "examory-auth", "examory", 10*time.Minute, clk)
	ver, _ := authn.NewVerifier(authn.StaticKeys{"k1": pub}, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory", Clock: clk})
	st := apptest.NewStore()
	svc := app.New(st, clk, logging.Discard(), app.Config{Grace: 2 * time.Second})
	e, _ := httpx.NewEngine(httpx.EngineConfig{Logger: logging.Discard()})
	Register(e, Deps{Service: svc, Verifier: ver, Limiter: ratelimit.NewMemory(clk), Logger: logging.Discard(), SimClock: sim,
		Limits: RateLimits{PerIP: 10000, PerSubject: 10000, Window: time.Minute}})
	return &env{t: t, e: e, signer: signer, store: st, clk: clk}
}

func (v *env) token(uid string, role authn.Role, gid string) string {
	tok, _, err := v.signer.Issue(uid, role, gid)
	if err != nil {
		v.t.Fatal(err)
	}
	return tok
}

func (v *env) do(method, path, tok string, body any, hdr map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, val := range hdr {
		r.Header.Set(k, val)
	}
	w := httptest.NewRecorder()
	v.e.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

func quizBody() map[string]any {
	return map[string]any{
		"title": "Q", "course_id": "c_10", "time_limit_sec": 600, "max_attempts": 3,
		"questions": []any{
			map[string]any{"text": "2+2?", "points": 1, "options": []any{map[string]any{"key": "a", "text": "3"}, map[string]any{"key": "b", "text": "4"}}, "correct_keys": []string{"b"}},
		},
	}
}

// setup creates, publishes and assigns a quiz; it returns the quiz id.
func (v *env) setup(teacher, group string) string {
	tt := v.token(teacher, authn.RoleTeacher, "")
	w, m := v.do("POST", "/v1/quizzes", tt, quizBody(), nil)
	if w.Code != 201 {
		v.t.Fatalf("create %d %s", w.Code, w.Body)
	}
	qid := m["id"].(string)
	if w, _ := v.do("POST", "/v1/quizzes/"+qid+"/publish", tt, nil, nil); w.Code != 200 {
		v.t.Fatalf("publish %d %s", w.Code, w.Body)
	}
	if w, _ := v.do("PUT", "/v1/quizzes/"+qid+"/assignments/"+group, tt,
		map[string]any{"opens_at": t0.Add(-time.Hour), "closes_at": t0.Add(48 * time.Hour)}, nil); w.Code != 200 {
		v.t.Fatalf("assign %d %s", w.Code, w.Body)
	}
	return qid
}

func TestHTTPFlowAndAuthorization(t *testing.T) {
	v := newEnv(t, false)
	teacher, other, group := ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid := v.setup(teacher, group)
	alice, bob := ids.NewV7(), ids.NewV7()
	ta, tb := v.token(alice, authn.RoleStudent, group), v.token(bob, authn.RoleStudent, group)

	if w, _ := v.do("GET", "/v1/quizzes", "", nil, nil); w.Code != 401 {
		t.Fatalf("unauthenticated %d", w.Code)
	}
	if w, _ := v.do("POST", "/v1/quizzes", ta, quizBody(), nil); w.Code != 403 {
		t.Fatalf("student create %d", w.Code)
	}
	// Teacher B cannot see or edit teacher A's quiz.
	tob := v.token(other, authn.RoleTeacher, "")
	if w, _ := v.do("GET", "/v1/quizzes/"+qid, tob, nil, nil); w.Code != 404 {
		t.Fatalf("foreign teacher get %d", w.Code)
	}
	if w, _ := v.do("PUT", "/v1/quizzes/"+qid, tob, quizBody(), nil); w.Code != 404 {
		t.Fatalf("foreign teacher put %d", w.Code)
	}
	// Owner cannot edit after publish.
	if w, m := v.do("PUT", "/v1/quizzes/"+qid, v.token(teacher, authn.RoleTeacher, ""), quizBody(), nil); w.Code != 409 || m["code"] != "quiz_not_draft" {
		t.Fatalf("edit published %d %v", w.Code, m)
	}
	// Strict JSON: unknown fields are rejected.
	bad := quizBody()
	bad["owner_id"] = alice
	if w, m := v.do("POST", "/v1/quizzes", v.token(teacher, authn.RoleTeacher, ""), bad, nil); w.Code != 400 || m["code"] != "unknown_field" {
		t.Fatalf("unknown field %d %v", w.Code, m)
	}
	// Validation problems list fields.
	inv := quizBody()
	inv["time_limit_sec"] = 1
	if w, m := v.do("POST", "/v1/quizzes", v.token(teacher, authn.RoleTeacher, ""), inv, nil); w.Code != 422 || m["errors"] == nil {
		t.Fatalf("invalid %d %v", w.Code, m)
	}

	// Student flow.
	w, m := v.do("POST", "/v1/quizzes/"+qid+"/attempts", ta, nil, nil)
	if w.Code != 201 {
		t.Fatalf("start %d %s", w.Code, w.Body)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "correct") {
		t.Fatal("attempt view leaks the marking key")
	}
	aid := m["id"].(string)
	q0 := m["questions"].([]any)[0].(map[string]any)
	qID := q0["id"].(string)
	optID := q0["options"].([]any)[0].(map[string]any)["id"].(string)
	base := "/v1/attempts/" + aid

	// Bob (same group) cannot touch Alice's attempt.
	for _, rq := range []struct{ m, p string }{{"GET", base}, {"POST", base + "/questions/" + qID + "/shown"}, {"POST", base + "/submit"}} {
		if w, _ := v.do(rq.m, rq.p, tb, nil, map[string]string{"Idempotency-Key": "bob-key-0001"}); w.Code != 404 {
			t.Fatalf("IDOR %s %s -> %d", rq.m, rq.p, w.Code)
		}
	}
	if w, _ := v.do("PUT", base+"/answers/"+qID, tb, map[string]any{"selected_option_ids": []string{optID}}, nil); w.Code != 404 {
		t.Fatalf("IDOR answer %d", w.Code)
	}
	// Answer before shown -> 409.
	if w, m := v.do("PUT", base+"/answers/"+qID, ta, map[string]any{"selected_option_ids": []string{optID}}, nil); w.Code != 409 || m["code"] != "question_not_shown" {
		t.Fatalf("answer before shown %d %v", w.Code, m)
	}
	v.clk.Advance(time.Second)
	if w, _ := v.do("POST", base+"/questions/"+qID+"/shown", ta, nil, nil); w.Code != 200 {
		t.Fatalf("shown %d", w.Code)
	}
	v.clk.Advance(3 * time.Second)
	if w, m := v.do("PUT", base+"/answers/"+qID, ta, map[string]any{"selected_option_ids": []string{optID}}, nil); w.Code != 200 || m["is_change"] != false {
		t.Fatalf("answer %d %v", w.Code, m)
	}
	// Submit requires a valid Idempotency-Key.
	if w, m := v.do("POST", base+"/submit", ta, nil, nil); w.Code != 400 || m["code"] != "idempotency_key_required" {
		t.Fatalf("submit without key %d %v", w.Code, m)
	}
	w, m = v.do("POST", base+"/submit", ta, nil, map[string]string{"Idempotency-Key": "alice-key-01"})
	if w.Code != 200 || m["status"] != "submitted" || m["max_score"] != float64(1) {
		t.Fatalf("submit %d %v", w.Code, m)
	}
	w, _ = v.do("POST", base+"/submit", ta, nil, map[string]string{"Idempotency-Key": "alice-key-01"})
	if w.Code != 200 || w.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay %d", w.Code)
	}
	if w, _ := v.do("POST", base+"/submit", ta, nil, map[string]string{"Idempotency-Key": "alice-key-02"}); w.Code != 409 {
		t.Fatalf("different key after submit %d", w.Code)
	}
	// Malformed ids are 404, not 500.
	if w, _ := v.do("GET", "/v1/attempts/not-a-uuid", ta, nil, nil); w.Code != 404 {
		t.Fatalf("malformed id %d", w.Code)
	}
	// Author sees attempts of own quiz only.
	if w, m := v.do("GET", "/v1/quizzes/"+qid+"/attempts", v.token(teacher, authn.RoleTeacher, ""), nil, nil); w.Code != 200 || len(m["items"].([]any)) != 1 {
		t.Fatalf("list attempts %d %v", w.Code, m)
	}
	if w, _ := v.do("GET", "/v1/quizzes/"+qid+"/attempts", tob, nil, nil); w.Code != 404 {
		t.Fatalf("foreign list attempts %d", w.Code)
	}
}

func TestSimHeadersRejectedWhenDisabled(t *testing.T) {
	v := newEnv(t, false)
	sys := v.token(ids.NewV7(), authn.RoleSystem, "")
	for _, h := range []map[string]string{
		{HeaderSimTime: "2026-01-01T00:00:00.000Z"},
		{HeaderActAsUser: ids.NewV7()},
		{HeaderActAsGroup: ids.NewV7()},
		{HeaderSimTime: ""},
	} {
		if w, m := v.do("GET", "/v1/quizzes", sys, nil, h); w.Code != 400 || m["code"] != "simulation_disabled" {
			t.Fatalf("headers %v: %d %v", h, w.Code, m)
		}
	}
}

func TestSimClockEnabled(t *testing.T) {
	v := newEnv(t, true)
	teacher, group, student := ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid := v.setup(teacher, group)
	sys := v.token(ids.NewV7(), authn.RoleSystem, "")
	asStudent := func(at string) map[string]string {
		return map[string]string{HeaderSimTime: at, HeaderActAsUser: student, HeaderActAsGroup: group}
	}
	// Non-system callers may not use the headers even when enabled.
	if w, _ := v.do("POST", "/v1/quizzes/"+qid+"/attempts", v.token(student, authn.RoleStudent, group), nil, asStudent("2026-11-14T10:00:00.000Z")); w.Code != 403 {
		t.Fatalf("student with sim headers %d", w.Code)
	}
	// System without act-as is not a student.
	if w, _ := v.do("POST", "/v1/quizzes/"+qid+"/attempts", sys, nil, nil); w.Code != 403 {
		t.Fatalf("system as itself %d", w.Code)
	}
	if w, m := v.do("GET", "/v1/quizzes", sys, nil, map[string]string{HeaderSimTime: "yesterday"}); w.Code != 400 || m["code"] != "invalid_sim_time" {
		t.Fatalf("bad sim time %d %v", w.Code, m)
	}
	if w, _ := v.do("GET", "/v1/quizzes", sys, nil, map[string]string{HeaderActAsGroup: group}); w.Code != 400 {
		t.Fatalf("group without user %d", w.Code)
	}
	w, m := v.do("POST", "/v1/quizzes/"+qid+"/attempts", sys, nil, asStudent("2026-11-14T10:00:00.000Z"))
	if w.Code != 201 {
		t.Fatalf("sim start %d %s", w.Code, w.Body)
	}
	aid := m["id"].(string)
	q0 := m["questions"].([]any)[0].(map[string]any)
	qID := q0["id"].(string)
	opt := q0["options"].([]any)[1].(map[string]any)["id"].(string)
	base := "/v1/attempts/" + aid
	if w, _ := v.do("POST", base+"/questions/"+qID+"/shown", sys, nil, asStudent("2026-11-14T10:00:05.000Z")); w.Code != 200 {
		t.Fatalf("sim shown %d", w.Code)
	}
	if w, m := v.do("PUT", base+"/answers/"+qID, sys, map[string]any{"selected_option_ids": []string{opt}}, asStudent("2026-11-14T10:00:04.000Z")); w.Code != 409 || m["code"] != "non_monotonic_time" {
		t.Fatalf("backwards %d %v", w.Code, m)
	}
	if w, _ := v.do("PUT", base+"/answers/"+qID, sys, map[string]any{"selected_option_ids": []string{opt}}, asStudent("2026-11-14T10:00:12.345Z")); w.Code != 200 {
		t.Fatalf("sim answer %d", w.Code)
	}
	// The real student cannot mix real-time requests into the simulated attempt.
	if w, m := v.do("POST", base+"/submit", v.token(student, authn.RoleStudent, group), nil, map[string]string{"Idempotency-Key": "real-key-001"}); w.Code != 409 || m["code"] != "clock_mode_mismatch" {
		t.Fatalf("mixed clock %d %v", w.Code, m)
	}
	h := asStudent("2026-11-14T10:01:00.000Z")
	h["Idempotency-Key"] = "sim-key-0001"
	if w, _ := v.do("POST", base+"/submit", sys, nil, h); w.Code != 200 {
		t.Fatalf("sim submit %d", w.Code)
	}
	evs := v.store.Events()
	if len(evs) != 3 {
		t.Fatalf("%d events", len(evs))
	}
	if got := evs[1].Env.OccurredAt.Format("2006-01-02T15:04:05.000Z"); got != "2026-11-14T10:00:12.345Z" || evs[1].Env.Actor.UserID != student || evs[1].Key != student {
		t.Fatalf("simulated event %s actor %s", got, evs[1].Env.Actor.UserID)
	}
}

// TestEventTimesConsistentWithSubMillisecondClock: with a real clock that has
// sub-millisecond parts, answer timestamps still satisfy
// answered.occurred_at == submitted.occurred_at - (duration_ms - time_since_attempt_start_ms).
func TestEventTimesConsistentWithSubMillisecondClock(t *testing.T) {
	v := newEnv(t, false)
	teacher, group, student := ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid := v.setup(teacher, group)
	ts := v.token(student, authn.RoleStudent, group)
	v.clk.Advance(500 * time.Microsecond) // start at .5 ms
	_, m := v.do("POST", "/v1/quizzes/"+qid+"/attempts", ts, nil, nil)
	aid := m["id"].(string)
	q0 := m["questions"].([]any)[0].(map[string]any)
	qID := q0["id"].(string)
	opt := q0["options"].([]any)[0].(map[string]any)["id"].(string)
	v.clk.Advance(1777500 * time.Microsecond) // shown at .0 ms
	v.do("POST", "/v1/attempts/"+aid+"/questions/"+qID+"/shown", ts, nil, nil)
	v.clk.Advance(2345 * time.Millisecond) // answer at .0 ms
	v.do("PUT", "/v1/attempts/"+aid+"/answers/"+qID, ts, map[string]any{"selected_option_ids": []string{opt}}, nil)
	v.clk.Advance(987900 * time.Microsecond) // submit at .9 ms: 1 ms drift without truncation
	if w, _ := v.do("POST", "/v1/attempts/"+aid+"/submit", ts, nil, map[string]string{"Idempotency-Key": "ms-key-0001"}); w.Code != 200 {
		t.Fatalf("submit %d", w.Code)
	}
	evs := v.store.Events()
	var ans struct {
		TimeSinceAttemptStartMs int64 `json:"time_since_attempt_start_ms"`
	}
	var sub struct {
		DurationMs int64 `json:"duration_ms"`
	}
	_ = json.Unmarshal(evs[1].Env.Payload, &ans)
	_ = json.Unmarshal(evs[2].Env.Payload, &sub)
	want := evs[2].Env.OccurredAt.Add(-time.Duration(sub.DurationMs-ans.TimeSinceAttemptStartMs) * time.Millisecond)
	if !want.Equal(evs[1].Env.OccurredAt.Time) {
		t.Fatalf("answered at %v, derived %v", evs[1].Env.OccurredAt, want)
	}
}
