//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
)

// TestE2EThroughGateway drives the M4 quiz scenario through the Nginx gateway
// against the real auth and quizzes containers (docker compose up), then
// reads lms.quizzes from Kafka. Opt-in:
//
//	E2E_GATEWAY_URL=http://127.0.0.1:8000 E2E_ADMIN_EMAIL=... E2E_ADMIN_PASSWORD=... IT_KAFKA_BROKERS=127.0.0.1:9092
func TestE2EThroughGateway(t *testing.T) {
	base := os.Getenv("E2E_GATEWAY_URL")
	if base == "" || os.Getenv("IT_KAFKA_BROKERS") == "" {
		t.Skip("E2E_GATEWAY_URL / IT_KAFKA_BROKERS not set")
	}
	cl := &http.Client{Timeout: 10 * time.Second}
	call := func(method, path, tok string, body any, hdr map[string]string) (*http.Response, map[string]any) {
		t.Helper()
		var rd io.Reader = http.NoBody
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, base+path, rd)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return resp, m
	}
	login := func(email, pw string) (string, map[string]any, *http.Cookie) {
		t.Helper()
		resp, m := call("POST", "/api/auth/v1/auth/login", "", map[string]any{"email": email, "password": pw}, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("login %s: %d %v", email, resp.StatusCode, m)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("login response cacheable: %q", resp.Header.Get("Cache-Control"))
		}
		var refresh *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "examory_refresh" {
				refresh = c
			}
		}
		if refresh == nil || !refresh.HttpOnly || !refresh.Secure || refresh.SameSite != http.SameSiteStrictMode || refresh.Path != "/api/auth/v1/auth" {
			t.Fatalf("refresh cookie attributes: %+v", refresh)
		}
		return m["access_token"].(string), m["user"].(map[string]any), refresh
	}
	pw := func() string { return "E2e-" + strings.ReplaceAll(ids.NewV7(), "-", "")[:20] }
	suffix := strings.ReplaceAll(ids.NewV7(), "-", "")[20:]

	admin, _, _ := login(os.Getenv("E2E_ADMIN_EMAIL"), os.Getenv("E2E_ADMIN_PASSWORD"))
	mkUser := func(role string) (email, password, id string) {
		email, password = fmt.Sprintf("%s.%s@examory.test", role, suffix), pw()
		resp, m := call("POST", "/api/auth/v1/users", admin, map[string]any{"email": email, "display_name": role + " " + suffix, "role": role, "password": password}, nil)
		if resp.StatusCode != 201 {
			t.Fatalf("create %s: %d %v", role, resp.StatusCode, m)
		}
		return email, password, m["id"].(string)
	}
	tEmail, tPw, _ := mkUser("teacher")
	sEmail, sPw, studentID := mkUser("student")
	resp, grp := call("POST", "/api/auth/v1/groups", admin, map[string]any{"name": "SE-" + suffix, "course_id": "c_e2e"}, nil)
	if resp.StatusCode != 201 {
		t.Fatalf("group %d %v", resp.StatusCode, grp)
	}
	groupID := grp["id"].(string)
	if resp, m := call("PUT", "/api/auth/v1/groups/"+groupID+"/members/"+studentID, admin, nil, nil); resp.StatusCode != 200 {
		t.Fatalf("member %d %v", resp.StatusCode, m)
	}
	teacher, _, _ := login(tEmail, tPw)
	student, me, refresh := login(sEmail, sPw)
	if me["group_id"] != groupID {
		t.Fatalf("student token group %v", me["group_id"])
	}

	// Refresh rotation through the gateway, then reuse detection.
	doRefresh := func(c *http.Cookie) int {
		req, _ := http.NewRequest("POST", base+"/api/auth/v1/auth/refresh", http.NoBody)
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		r, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if c := doRefresh(refresh); c != 200 {
		t.Fatalf("refresh %d", c)
	}
	if c := doRefresh(refresh); c != 401 {
		t.Fatalf("replayed refresh token accepted: %d", c)
	}

	// Teacher authors a quiz.
	resp, q := call("POST", "/api/quizzes/v1/quizzes", teacher, quizBody(600), nil)
	if resp.StatusCode != 201 {
		t.Fatalf("create quiz %d %v", resp.StatusCode, q)
	}
	qid := q["id"].(string)
	if resp, m := call("POST", "/api/quizzes/v1/quizzes/"+qid+"/publish", teacher, nil, nil); resp.StatusCode != 200 {
		t.Fatalf("publish %d %v", resp.StatusCode, m)
	}
	now := time.Now().UTC()
	if resp, m := call("PUT", "/api/quizzes/v1/quizzes/"+qid+"/assignments/"+groupID, teacher,
		map[string]any{"opens_at": now.Add(-time.Minute), "closes_at": now.Add(time.Hour)}, nil); resp.StatusCode != 200 {
		t.Fatalf("assign %d %v", resp.StatusCode, m)
	}
	// Student takes it. Research-replay headers sent by a client are dropped
	// by the gateway (otherwise quizzes would answer 400 simulation_disabled).
	resp, att := call("POST", "/api/quizzes/v1/quizzes/"+qid+"/attempts", student, nil, map[string]string{"X-Sim-Time": "2020-01-01T00:00:00.000Z"})
	if resp.StatusCode != 201 {
		t.Fatalf("start %d %v", resp.StatusCode, att)
	}
	aid := att["id"].(string)
	ab := "/api/quizzes/v1/attempts/" + aid
	q0, q1 := questionID(q, 0), questionID(q, 1)
	o0, o1 := optionIDs(q, 0), optionIDs(q, 1)
	steps := []struct {
		method, path string
		body         any
	}{
		{"POST", ab + "/questions/" + q0 + "/shown", nil},
		{"PUT", ab + "/answers/" + q0, map[string]any{"selected_option_ids": []string{o0[0]}}},
		{"PUT", ab + "/answers/" + q0, map[string]any{"selected_option_ids": []string{o0[1]}}},
		{"POST", ab + "/questions/" + q1 + "/shown", nil},
		{"PUT", ab + "/answers/" + q1, map[string]any{"selected_option_ids": []string{o1[0], o1[1]}}},
	}
	for _, s := range steps {
		time.Sleep(150 * time.Millisecond)
		if resp, m := call(s.method, s.path, student, s.body, nil); resp.StatusCode != 200 {
			t.Fatalf("%s %s: %d %v", s.method, s.path, resp.StatusCode, m)
		}
	}
	resp, res := call("POST", ab+"/submit", student, nil, map[string]string{"Idempotency-Key": "e2e-" + aid})
	if resp.StatusCode != 200 || res["score"] != float64(5) {
		t.Fatalf("submit %d %v", resp.StatusCode, res)
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "X-Request-ID"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("gateway response missing %s", h)
		}
	}
	// Internal endpoints are not routed.
	if resp, _ := call("GET", "/api/quizzes/readyz", "", nil, nil); resp.StatusCode != 404 {
		t.Fatalf("readyz exposed: %d", resp.StatusCode)
	}

	recs := consume(t, strings.Split(os.Getenv("IT_KAFKA_BROKERS"), ","), aid, 5)
	val, err := eventschema.Load(mustDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, r := range recs {
		types = append(types, r.env.EventType)
		if err := val.Validate(r.raw); err != nil {
			t.Fatalf("schema %s: %v", r.env.EventType, err)
		}
		if r.key != studentID || *r.env.Context.GroupID != groupID {
			t.Fatalf("key/group mismatch")
		}
	}
	if strings.Join(types, ",") != "quiz.started,quiz.answered,quiz.answered,quiz.answered,quiz.submitted" {
		t.Fatalf("event order %v", types)
	}
	var sub events.QuizSubmitted
	_ = recs[4].env.DecodePayload(&sub, true)
	if sub.AnswersChanged != 1 || sub.Score != 5 || sub.DurationMs <= 0 {
		t.Fatalf("submitted %+v", sub)
	}
	t.Logf("e2e attempt %s: %d events on %s, duration_ms=%d", aid, len(recs), events.TopicQuizzes, sub.DurationMs)
}
