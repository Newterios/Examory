//go:build integration

// Package integration holds the M4 end-to-end scenario (project_management.md
// §3a) against the real containers:
//
//	quiz (quizzes, simulation clock) -> lms.quizzes -> anomaly-scorer ->
//	risk.behavior_anomaly on ai.risk -> grading -> teacher review ->
//	risk.reviewed on lms.grades
//
// Opt-in (make m4 && make e2e):
//
//	E2E_GATEWAY_URL=http://127.0.0.1:8000   public gateway
//	E2E_QUIZZES_URL=http://127.0.0.1:8001   quizzes directly (research overlay; the gateway strips sim headers)
//	E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD    bootstrap admin
//	IT_KAFKA_BROKERS=127.0.0.1:9092
//	IT_PG_ADMIN_URL                          superuser DSN (reads grading/anomaly DBs to verify dedup)
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c *client) do(method, path, tok string, body any, hdr map[string]string) (int, map[string]any, http.Header) {
	c.t.Helper()
	var rd io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m, resp.Header
}

func (c *client) must(want int, method, path, tok string, body any, hdr map[string]string) map[string]any {
	c.t.Helper()
	code, m, _ := c.do(method, path, tok, body, hdr)
	if code != want {
		c.t.Fatalf("%s %s: %d (want %d) %v", method, path, code, want, m)
	}
	return m
}

// rnd is a deterministic pseudo-random number in [0, n) for a label.
func rnd(label string, n int) int {
	h := sha256.Sum256([]byte(label))
	return int(binary.BigEndian.Uint32(h[:4]) % uint32(n))
}

func env(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Skipf("%s not set", k)
	}
	return v
}

type student struct {
	name, id string
	fast     bool
}

func TestM4Scenario(t *testing.T) {
	gwURL, qzURL := env(t, "E2E_GATEWAY_URL"), env(t, "E2E_QUIZZES_URL")
	adminEmail, adminPW := env(t, "E2E_ADMIN_EMAIL"), env(t, "E2E_ADMIN_PASSWORD")
	brokers := strings.Split(env(t, "IT_KAFKA_BROKERS"), ",")
	pgAdmin := env(t, "IT_PG_ADMIN_URL")
	hc := &http.Client{Timeout: 15 * time.Second}
	gw := &client{t, gwURL, hc}
	qz := &client{t, qzURL, hc}
	run := strings.ReplaceAll(ids.NewV7(), "-", "")[20:]

	login := func(email, pw string) string {
		m := gw.must(200, "POST", "/api/auth/v1/auth/login", "", map[string]any{"email": email, "password": pw}, nil)
		return m["access_token"].(string)
	}
	admin := login(adminEmail, adminPW)
	pw := "M4-" + run + "-Passw0rd!"
	mkUser := func(role, name string) string {
		m := gw.must(201, "POST", "/api/auth/v1/users", admin, map[string]any{
			"email": fmt.Sprintf("%s.%s@examory.test", name, run), "display_name": name, "role": role, "password": pw}, nil)
		return m["id"].(string)
	}
	mkUser("teacher", "teacher")
	mkUser("system", "replay")
	group := gw.must(201, "POST", "/api/auth/v1/groups", admin, map[string]any{"name": "M4-" + run, "course_id": "c_m4"}, nil)["id"].(string)
	var students []student
	for i := 0; i < 8; i++ {
		students = append(students, student{name: fmt.Sprintf("normal%d", i)})
	}
	students = append(students, student{name: "fastA", fast: true}, student{name: "fastB", fast: true})
	for i := range students {
		students[i].id = mkUser("student", students[i].name)
		gw.must(200, "PUT", "/api/auth/v1/groups/"+group+"/members/"+students[i].id, admin, nil, nil)
	}
	teacher := login("teacher."+run+"@examory.test", pw)
	system := login("replay."+run+"@examory.test", pw)
	fastA := login("fastA."+run+"@examory.test", pw)

	// Teacher authors a 20-question quiz, 1 point each (the frozen pipeline v1
	// requires max_score == number of questions).
	var questions []any
	for i := 0; i < 20; i++ {
		questions = append(questions, map[string]any{"text": fmt.Sprintf("Question %d", i+1), "points": 1,
			"options": []any{map[string]any{"key": "a", "text": "A"}, map[string]any{"key": "b", "text": "B"},
				map[string]any{"key": "c", "text": "C"}, map[string]any{"key": "d", "text": "D"}},
			"correct_keys": []string{[]string{"a", "b", "c", "d"}[i%4]}})
	}
	quiz := gw.must(201, "POST", "/api/quizzes/v1/quizzes", teacher, map[string]any{"title": "M4 quiz " + run, "course_id": "c_m4",
		"time_limit_sec": 1800, "max_attempts": 1, "questions": questions}, nil)
	quizID := quiz["id"].(string)
	gw.must(200, "POST", "/api/quizzes/v1/quizzes/"+quizID+"/publish", teacher, nil, nil)
	now := time.Now().UTC().Truncate(time.Second)
	gw.must(200, "PUT", "/api/quizzes/v1/quizzes/"+quizID+"/assignments/"+group, teacher,
		map[string]any{"opens_at": now.Add(-time.Hour), "closes_at": now.Add(24 * time.Hour)}, nil)
	type q struct {
		id      string
		correct string
		opts    []string
	}
	var qs []q
	for _, raw := range quiz["questions"].([]any) {
		m := raw.(map[string]any)
		item := q{id: m["id"].(string)}
		for _, o := range m["options"].([]any) {
			om := o.(map[string]any)
			item.opts = append(item.opts, om["id"].(string))
			if om["correct"].(bool) {
				item.correct = om["id"].(string)
			}
		}
		qs = append(qs, item)
	}

	// Replay students through the real quizzes API with the simulation clock.
	attempts := map[string]string{} // student name -> attempt id
	for i, s := range students {
		start := now.Add(time.Duration(i) * 90 * time.Second)
		if s.fast {
			start = now.Add(30 * time.Minute) // both colluders start and submit together
		}
		t := start
		hdr := func() map[string]string {
			return map[string]string{"X-Sim-Time": t.Format("2006-01-02T15:04:05.000Z07:00"), "X-Act-As-User": s.id, "X-Act-As-Group": group}
		}
		att := qz.must(201, "POST", "/v1/quizzes/"+quizID+"/attempts", system, nil, hdr())
		aid := att["id"].(string)
		attempts[s.name] = aid
		for _, item := range qs {
			t = t.Add(300 * time.Millisecond)
			qz.must(200, "POST", "/v1/attempts/"+aid+"/questions/"+item.id+"/shown", system, nil, hdr())
			choice := item.correct
			if s.fast {
				t = t.Add(time.Duration(600+rnd(s.name+item.id, 300)) * time.Millisecond)
			} else {
				t = t.Add(time.Duration(14000+rnd(s.name+item.id, 22000)) * time.Millisecond)
				if rnd("wrong"+s.name+item.id, 100) < 25 {
					choice = item.opts[rnd("opt"+s.name+item.id, 4)]
				}
				if rnd("change"+s.name+item.id, 100) < 12 { // an occasional revision
					wrong := item.opts[(rnd("w"+s.name+item.id, 3)+1+indexOf(item.opts, choice))%4]
					qz.must(200, "PUT", "/v1/attempts/"+aid+"/answers/"+item.id, system, map[string]any{"selected_option_ids": []string{wrong}}, hdr())
					t = t.Add(time.Duration(3000+rnd("c2"+s.name+item.id, 6000)) * time.Millisecond)
				}
			}
			qz.must(200, "PUT", "/v1/attempts/"+aid+"/answers/"+item.id, system, map[string]any{"selected_option_ids": []string{choice}}, hdr())
		}
		t = t.Add(time.Second)
		if s.name == "fastB" {
			t = t.Add(400 * time.Millisecond) // synchronous with fastA (within 5 s)
		}
		h := hdr()
		h["Idempotency-Key"] = "m4-" + aid
		qz.must(200, "POST", "/v1/attempts/"+aid+"/submit", system, nil, h)
	}
	t.Logf("replayed %d attempts for quiz %s", len(attempts), quizID)

	// Wait for grading to list risk signals of this quiz.
	byTarget := map[string]map[string]any{}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		m := gw.must(200, "GET", "/api/grading/v1/risks?quiz_id="+quizID+"&limit=100", teacher, nil, nil)
		for _, it := range m["items"].([]any) {
			r := it.(map[string]any)
			byTarget[r["target_id"].(string)] = r
		}
		if byTarget[attempts["fastA"]] != nil && byTarget[attempts["fastB"]] != nil {
			break
		}
		time.Sleep(time.Second)
	}
	flagged := []string{}
	for name, aid := range attempts {
		if r := byTarget[aid]; r != nil {
			flagged = append(flagged, fmt.Sprintf("%s(%s %.3f)", name, r["level"], r["score"]))
		}
	}
	t.Logf("risk signals: %v", flagged)
	ra, rb := byTarget[attempts["fastA"]], byTarget[attempts["fastB"]]
	if ra == nil || rb == nil {
		t.Fatalf("the demo model did not flag both crafted attempts within 90 s (flagged: %v). "+
			"Do not lower thresholds silently; see services/grading/README.md", flagged)
	}
	for _, r := range []map[string]any{ra, rb} {
		if r["status"] != "open" || r["category"] != "risk.behavior_anomaly" || r["attempt"] == nil {
			t.Fatalf("risk %v", r)
		}
	}

	// Students never see risk signals.
	for _, p := range []string{"/api/grading/v1/risks", "/api/grading/v1/risks/" + ra["id"].(string)} {
		if code, _, _ := gw.do("GET", p, fastA, nil, nil); code != 403 {
			t.Fatalf("student GET %s -> %d", p, code)
		}
	}
	scoreBefore := gw.must(200, "GET", "/api/quizzes/v1/attempts/"+attempts["fastA"], fastA, nil, nil)["result"]

	// Teacher inspects evidence and reviews: confirm A, dismiss B.
	review := func(r map[string]any, decision, comment string) {
		code, detail, h := gw.do("GET", "/api/grading/v1/risks/"+r["id"].(string), teacher, nil, nil)
		if code != 200 || detail["evidence"] == nil || !strings.Contains(detail["explanation"].(string), "not a probability or proof of misconduct") {
			t.Fatalf("detail %d %v", code, detail)
		}
		gw.must(200, "POST", "/api/grading/v1/risks/"+r["id"].(string)+"/review", teacher,
			map[string]any{"decision": decision, "comment": comment},
			map[string]string{"If-Match": h.Get("ETag"), "Idempotency-Key": "m4-review-" + r["id"].(string)})
	}
	review(ra, "confirmed", "Every answer under one second and identical to a synchronous peer; discussed with the student.")
	review(rb, "dismissed", "Student showed prior work; fast pace explained.")
	if code, m, _ := gw.do("POST", "/api/grading/v1/risks/"+ra["id"].(string)+"/review", teacher,
		map[string]any{"decision": "dismissed", "comment": "again"}, map[string]string{"If-Match": `"v2"`, "Idempotency-Key": "m4-second-" + run}); code != 409 {
		t.Fatalf("second review %d %v", code, m)
	}
	// FR-26: the review changed nothing about the student's result.
	scoreAfter := gw.must(200, "GET", "/api/quizzes/v1/attempts/"+attempts["fastA"], fastA, nil, nil)["result"]
	if fmt.Sprint(scoreBefore) != fmt.Sprint(scoreAfter) {
		t.Fatalf("attempt result changed by a review: %v -> %v", scoreBefore, scoreAfter)
	}

	// Kafka: risk.behavior_anomaly on ai.risk and risk.reviewed on lms.grades, schema-valid.
	dir, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	val, err := eventschema.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	riskIDs := map[string]string{ra["risk_id"].(string): "confirmed", rb["risk_id"].(string): "dismissed"}
	riskRecs := collect(t, brokers, events.TopicRisk, func(e events.Envelope) bool {
		var p struct {
			RiskID string `json:"risk_id"`
		}
		_ = json.Unmarshal(e.Payload, &p)
		_, ok := riskIDs[p.RiskID]
		return e.EventType == "risk.behavior_anomaly" && ok
	}, 2)
	reviewRecs := collect(t, brokers, events.TopicGrades, func(e events.Envelope) bool {
		var p struct {
			RiskID string `json:"risk_id"`
		}
		_ = json.Unmarshal(e.Payload, &p)
		_, ok := riskIDs[p.RiskID]
		return e.EventType == "risk.reviewed" && ok
	}, 2)
	for _, r := range append(append([]*kgo.Record{}, riskRecs...), reviewRecs...) {
		if err := val.Validate(r.Value); err != nil {
			t.Fatalf("schema: %v\n%s", err, r.Value)
		}
	}
	for _, r := range reviewRecs {
		env, _ := events.Parse(r.Value)
		var p map[string]any
		_ = json.Unmarshal(env.Payload, &p)
		if riskIDs[p["risk_id"].(string)] != p["decision"] || env.Actor.Role != "teacher" || p["reviewed_by"] != env.Actor.UserID {
			t.Fatalf("risk.reviewed %s", r.Value)
		}
	}
	if len(riskRecs) != 2 || len(reviewRecs) != 2 {
		t.Fatalf("records: %d risk, %d reviewed", len(riskRecs), len(reviewRecs))
	}

	// Re-delivery: publish the same risk and quiz records again, followed by
	// sentinels on the same keys; once the sentinels are processed, the
	// duplicates (earlier on the same partitions) were too.
	prod, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer prod.Close()
	ctx := context.Background()
	quizRecs := collect(t, brokers, events.TopicQuizzes, func(e events.Envelope) bool { return e.Actor.UserID == students[8].id }, 22)
	var dup []*kgo.Record
	for _, r := range append(append([]*kgo.Record{}, riskRecs...), quizRecs...) {
		dup = append(dup, &kgo.Record{Topic: r.Topic, Key: r.Key, Value: r.Value})
	}
	sentinelAttempt := ids.NewV7()
	var sentinels []string
	for i, r := range riskRecs { // invalid risk (score 7) -> rejected_events, same key/partition as the duplicate
		bad, _ := events.New("risk.behavior_anomaly", 1, "integrity", events.Actor{UserID: "integrity", Role: "system"}, events.Context{},
			map[string]any{"risk_id": fmt.Sprintf("sentinel_%s_%d", run, i), "target_type": "attempt", "target_id": string(r.Key), "score": 7}, time.Now())
		raw, _ := json.Marshal(bad)
		dup = append(dup, &kgo.Record{Topic: events.TopicRisk, Key: r.Key, Value: raw})
		sentinels = append(sentinels, bad.EventID)
	}
	started, _ := events.New(events.TypeQuizStarted, 1, "quizzes", events.Actor{UserID: students[8].id, Role: "student"},
		events.Context{CourseID: events.Ptr("c_m4"), GroupID: events.Ptr(group)},
		events.QuizStarted{AttemptID: sentinelAttempt, QuizID: quizID, VariantID: ids.NewV7(), QuestionCount: 20, TimeLimitSec: 1800}, time.Now())
	raw, _ := json.Marshal(started)
	dup = append(dup, &kgo.Record{Topic: events.TopicQuizzes, Key: []byte(students[8].id), Value: raw})
	if err := prod.ProduceSync(ctx, dup...).FirstErr(); err != nil {
		t.Fatal(err)
	}
	gradingDB, anomalyDB := dbConn(t, pgAdmin, "grading"), dbConn(t, pgAdmin, "anomaly")
	waitFor(t, 60*time.Second, "grading processed the sentinels", func() bool {
		var n int
		_ = gradingDB.QueryRow(ctx, `SELECT count(*) FROM rejected_events WHERE event_id = ANY($1)`, sentinels).Scan(&n)
		return n == len(sentinels)
	})
	waitFor(t, 60*time.Second, "scorer processed the sentinel", func() bool {
		var n int
		_ = anomalyDB.QueryRow(ctx, `SELECT count(*) FROM quiz_events WHERE attempt_id = $1`, sentinelAttempt).Scan(&n)
		return n == 1
	})
	var risks, reviews, outbox, quizEvents int
	ridList := []string{ra["risk_id"].(string), rb["risk_id"].(string)}
	_ = gradingDB.QueryRow(ctx, `SELECT count(*) FROM risks WHERE risk_id = ANY($1)`, ridList).Scan(&risks)
	_ = gradingDB.QueryRow(ctx, `SELECT count(*) FROM reviews v JOIN risks r ON r.id = v.risk_pk WHERE r.risk_id = ANY($1)`, ridList).Scan(&reviews)
	_ = anomalyDB.QueryRow(ctx, `SELECT count(*) FROM risk_outbox WHERE risk_id = ANY($1)`, ridList).Scan(&outbox)
	_ = anomalyDB.QueryRow(ctx, `SELECT count(*) FROM quiz_events WHERE attempt_id = $1`, attempts["fastA"]).Scan(&quizEvents)
	if risks != 2 || reviews != 2 || outbox != 2 || quizEvents != 22 {
		t.Fatalf("after re-delivery: risks=%d reviews=%d outbox=%d quiz_events(fastA)=%d", risks, reviews, outbox, quizEvents)
	}
	listed := gw.must(200, "GET", "/api/grading/v1/risks?quiz_id="+quizID+"&limit=100", teacher, nil, nil)["items"].([]any)
	if len(listed) != len(flagged) {
		t.Fatalf("risk list changed after re-delivery: %d -> %d", len(flagged), len(listed))
	}
	reviewRecs = collect(t, brokers, events.TopicGrades, func(e events.Envelope) bool {
		var p struct {
			RiskID string `json:"risk_id"`
		}
		_ = json.Unmarshal(e.Payload, &p)
		_, ok := riskIDs[p.RiskID]
		return e.EventType == "risk.reviewed" && ok
	}, 3)
	if len(reviewRecs) != 2 {
		t.Fatalf("re-delivery produced extra risk.reviewed: %d", len(reviewRecs))
	}
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return 0
}

func waitFor(t *testing.T, d time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", what)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func dbConn(t *testing.T, admin, name string) *pgx.Conn {
	u, err := url.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	c, err := pgx.Connect(context.Background(), u.String())
	if err != nil {
		t.Fatalf("connect %s: %v", name, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// collect reads a topic from the start and returns matching records; it stops
// after `want` matches or when the topic is drained (reading up to 20 s).
func collect(t *testing.T, brokers []string, topic string, match func(events.Envelope) bool, want int) []*kgo.Record {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	var out []*kgo.Record
	deadline, idle := time.Now().Add(20*time.Second), 0
	for len(out) < want && time.Now().Before(deadline) && idle < 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		fs := cl.PollFetches(ctx)
		cancel()
		n := 0
		fs.EachRecord(func(r *kgo.Record) {
			n++
			env, err := events.Parse(r.Value)
			if err == nil && match(env) {
				out = append(out, r)
			}
		})
		if n == 0 {
			idle++
		} else {
			idle = 0
		}
	}
	return out
}
