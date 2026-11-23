package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/sim/internal/events"
)

func syntheticCSV(t *testing.T) []byte {
	t.Helper()
	base := int64(1700000000000)
	rows := []events.Row{}
	add := func(elapsed int64, kind string, p obj) {
		p["attempt_id"] = "a"
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		eid := events.EventID(42, base+elapsed, 0, 0, fmt.Sprint(elapsed))
		rows = append(rows, events.Row{Ms: base + elapsed, EventID: eid, Fields: []string{eid, kind, "1", events.FormatTime(base + elapsed), "quizzes", `{"user_id":"u","role":"student"}`, `{"group_id":"g","course_id":"c","session_id":"s"}`, string(b), "0", "none"}})
	}
	add(100, answer, obj{"question_id": "q", "question_index": 0, "time_since_shown_ms": 80, "time_since_attempt_start_ms": 100, "is_change": false, "answer_hash": events.AnswerHash(0)})
	add(170, answer, obj{"question_id": "q", "question_index": 0, "time_since_shown_ms": 150, "time_since_attempt_start_ms": 170, "is_change": true, "answer_hash": events.AnswerHash(1)})
	add(180, submit, obj{"quiz_id": "quiz", "variant_id": "v", "score": 1, "max_score": 1, "duration_ms": 180, "answers_changed": 1})
	var out bytes.Buffer
	if err := events.WriteCSV(&out, rows); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestWorldAndBudget(t *testing.T) {
	src := syntheticCSV(t)
	w, err := parseWorld(src, []byte(`{"q":1}`))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	var offsets []int64
	for _, a := range w.actions {
		kinds = append(kinds, a.kind)
		offsets = append(offsets, a.at-w.actions[0].at)
	}
	if !slices.Equal(kinds, []string{start, shown, answer, answer, submit}) || !slices.Equal(offsets, []int64{0, 20, 100, 170, 180}) {
		t.Fatalf("%v %v", kinds, offsets)
	}
	prefix, err := loadPrefix(w.actions, 3)
	if err != nil || len(prefix) != 4 || !prefix[3].change {
		t.Fatalf("%v %v", prefix, err)
	}
	if _, err = loadPrefix(w.actions, 5); err == nil {
		t.Fatal("accepted insufficient world")
	}
	for _, bad := range []string{`{"q":0}`, `{"q":null}`, `{"q":4}`, `{}`} {
		if _, err = parseWorld(src, []byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestLocalOrigins(t *testing.T) {
	for _, u := range []string{"http://localhost:8000", "http://127.0.0.1:8001", "http://[::1]:8001"} {
		if _, err := localBase(u); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"https://example.test:8000", "http://10.0.0.1:8000", "http://gateway:8000", "http://user@127.0.0.1:8000", "http://127.0.0.1:8000/else", "http://127.0.0.1:8000?target=x"} {
		if _, err := localBase(u); err == nil {
			t.Errorf("accepted %s", u)
		}
	}
}

func TestReplayAPIAndMeasurement(t *testing.T) {
	for _, mode := range []string{"load", "parity"} {
		t.Run(mode, func(t *testing.T) {
			w, err := parseWorld(syntheticCSV(t), []byte(`{"q":1}`))
			if err != nil {
				t.Fatal(err)
			}
			const aid = "00000000-0000-4000-8000-000000000001"
			const secret = "test-bearer"
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
				a := w.actions[calls]
				calls++
				if r.Header.Get("Authorization") != "Bearer "+secret || r.Header.Get("X-Act-As-User") != "user" || r.Header.Get("X-Act-As-Group") != "group" || r.Header.Get("X-Sim-Time") != events.FormatTime(a.at) {
					t.Error("invalid clock or authorized actor")
				}
				var b obj
				switch a.kind {
				case start:
					if r.URL.Path != "/v1/quizzes/quiz/attempts" {
						t.Error(r.URL.Path)
					}
					out.WriteHeader(201)
					b = obj{"id": aid, "variant_id": "variant", "started_at": events.FormatTime(a.at)}
				case shown:
					if r.URL.Path != "/v1/attempts/"+aid+"/questions/item/shown" {
						t.Error(r.URL.Path)
					}
					b = obj{"first_shown_at": events.FormatTime(a.at)}
				case answer:
					var input struct {
						IDs []string `json:"selected_option_ids"`
					}
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.IDs) != 1 || input.IDs[0] != fmt.Sprintf("option%d", a.option) || r.Method != "PUT" {
						t.Error("wrong selected option")
					}
					b = obj{"saved": true, "is_change": a.change, "saved_at": events.FormatTime(a.at)}
				case submit:
					if r.Header.Get("Idempotency-Key") != "e06-"+aid {
						t.Error("missing stable submit key")
					}
					b = obj{"status": "submitted", "score": 1, "max_score": 1, "finished_at": events.FormatTime(a.at)}
				}
				_ = json.NewEncoder(out).Encode(b)
			}))
			defer server.Close()
			m := manifest{Mode: mode, Rate: 20, Attempts: map[string]*attemptMapping{"a": {UserID: "user", GroupID: "group", QuizID: "quiz", VariantID: "variant", Status: "partial"}}, Questions: map[string]questionMapping{"q": {ID: "item", OptionIDs: [4]string{"option0", "option1", "option2", "option3"}}}}
			endpoint := api{client: server.Client(), quizzes: server.URL}
			if err := endpoint.execute(context.Background(), &session{access: secret, renew: time.Now().Add(time.Hour)}, w, w.actions, &m); err != nil {
				t.Fatal(err)
			}
			if calls != 5 || m.Attempts["a"].Status != "submitted" || m.Attempts["a"].SubmitAck == "" || m.Actions[1].Scheduled != m.Actions[2].Scheduled {
				t.Fatalf("unexpected observations: %+v", m.Actions)
			}
			first, _ := time.Parse(time.RFC3339Nano, m.Actions[0].Scheduled)
			last, _ := time.Parse(time.RFC3339Nano, m.Actions[4].Scheduled)
			if last.Sub(first) != 150*time.Millisecond {
				t.Fatal(last.Sub(first))
			}
			raw, _ := json.Marshal(m)
			if strings.Contains(string(raw), secret) {
				t.Error("credential in manifest")
			}
		})
	}
}

func TestFailedRequestPreservesReceiptAndSkipsDependents(t *testing.T) {
	w, err := parseWorld(syntheticCSV(t), []byte(`{"q":1}`))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) { http.Error(out, "fixture-sensitive-detail", 409) }))
	defer server.Close()
	m := manifest{Mode: "load", Rate: 20, Attempts: map[string]*attemptMapping{"a": {Status: "partial"}}}
	endpoint := api{client: server.Client(), quizzes: server.URL}
	if err := endpoint.execute(context.Background(), &session{access: "test-token", renew: time.Now().Add(time.Minute)}, w, w.actions, &m); err == nil {
		t.Fatal("expected failure")
	}
	if m.Attempts["a"].Status != "failed" || m.Actions[0].HTTP != 409 || m.Actions[0].Ack == "" || m.Actions[1].Sent != "" {
		t.Fatalf("%+v", m.Actions)
	}
	b, _ := json.Marshal(m)
	if strings.Contains(string(b), "fixture-sensitive-detail") {
		t.Fatal("response body in manifest")
	}
}

func TestSessionRenewsThroughLogin(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/auth/v1/auth/login" {
			t.Error(r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(obj{"access_token": "renewed", "expires_in": 600})
	}))
	defer server.Close()
	s := session{email: "test", password: "fixture", access: "expired", renew: time.Now().Add(-time.Minute)}
	endpoint := api{gateway: server.URL, client: server.Client()}
	for i := 0; i < 2; i++ {
		token, err := s.token(context.Background(), &endpoint)
		if err != nil || token != "renewed" {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("login count %d", calls)
	}
}

func TestCourseProvisioningAcrossGatewayVersions(t *testing.T) {
	const courseID = "55555555-1234-4876-8123-222222222222"
	for _, status := range []int{201, 404, 403, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/courses/v1/courses" {
					t.Errorf("unexpected course route: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
				if status == 201 {
					_ = json.NewEncoder(w).Encode(obj{"id": courseID})
				}
			}))
			defer srv.Close()
			endpoint := api{gateway: srv.URL, client: srv.Client()}
			user := session{access: "fixture", renew: time.Now().Add(time.Minute)}
			id, managed, err := endpoint.replayCourse(context.Background(), &user, "fixture")
			switch status {
			case 201:
				if id != courseID || !managed || err != nil {
					t.Fatal(id, managed, err)
				}
			case 404:
				if id != "replay_fixture" || managed || err != nil {
					t.Fatal(id, managed, err)
				}
			default:
				if id != "" || managed || err == nil {
					t.Fatal("expected a course provisioning failure")
				}
			}
		})
	}
}

func TestProvisionRetriesOnlyExplicitRejection(t *testing.T) {
	for _, tc := range []struct {
		response   int
		retryAfter string
		wantCalls  int
		success    bool
	}{
		{429, "0", 2, true}, {429, "1200", 1, false}, {503, "0", 1, false}, {409, "0", 1, false},
	} {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				w.Header().Set("Retry-After", tc.retryAfter)
				w.WriteHeader(tc.response)
				return
			}
			w.WriteHeader(201)
		}))
		api := api{client: srv.Client(), gateway: srv.URL}
		call := func() error {
			_, err := api.request(context.Background(), api.gateway, "POST", "/api/auth/v1/users", "", obj{"role": "student"}, nil, 201, nil)
			return err
		}
		err := provision(context.Background(), call)
		srv.Close()
		if (err == nil) != tc.success || calls != tc.wantCalls {
			t.Fatalf("status %d: calls %d, err %v", tc.response, calls, err)
		}
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if retryDelay(now.Add(7*time.Second).Format(http.TimeFormat), now, 1) != 7*time.Second || retryDelay("", now, 3) != 8*time.Second || retryDelay("9999999999999999999", now, 0) <= 2*time.Minute {
		t.Fatal("incorrect Retry-After handling")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := provision(ctx, func() error { calls++; return limited{"1"} })
	if err != context.Canceled || calls != 1 {
		t.Fatal("ignored cancellation")
	}
}
