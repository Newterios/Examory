//go:build integration

// Integration tests against the compose PostgreSQL, Redis and Kafka.
//
//	IT_PG_ADMIN_URL   superuser DSN (a throwaway database is created per test)
//	IT_KAFKA_BROKERS  e.g. 127.0.0.1:9092
//	IT_REDIS_ADDR / IT_REDIS_PASSWORD (optional; in-memory limiter otherwise)
package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/platform/redisx"
	"github.com/Newterios/Examory/services/platform/testpg"
	httpadapter "github.com/Newterios/Examory/services/quizzes/internal/adapters/http"
	"github.com/Newterios/Examory/services/quizzes/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
)

type stack struct {
	t      *testing.T
	ctx    context.Context
	pool   *pgxpool.Pool
	srv    *httptest.Server
	signer *authn.Signer
	svc    *app.Service
	clk    *clock.Fake
	brk    []string
}

func newStack(t *testing.T) *stack {
	brk := os.Getenv("IT_KAFKA_BROKERS")
	if brk == "" {
		t.Skip("IT_KAFKA_BROKERS not set")
	}
	url := testpg.New(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, db.Config{URL: url, ApplicationName: "quizzes-it", MaxConns: 30})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for i := 0; i < 2; i++ { // idempotent re-application
		if err := migrate.Up(ctx, pool, logging.Discard(), postgres.Migrations()...); err != nil {
			t.Fatalf("migrate #%d: %v", i, err)
		}
	}

	// Issuer side: a JWKS endpoint exercising the real JWKS client.
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kid := authn.Thumbprint(pub)
	signer, _ := authn.NewSigner(kid, priv, "examory-auth", "examory", 10*time.Minute, clock.System{})
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(authn.JWKSet{Keys: []authn.JWK{authn.PublicJWK(kid, pub)}})
	}))
	t.Cleanup(jwksSrv.Close)
	verifier, _ := authn.NewVerifier(authn.NewJWKSClient(jwksSrv.URL, authn.JWKSOptions{}), authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})

	var limiter ratelimit.Limiter = ratelimit.NewMemory(nil)
	if addr := os.Getenv("IT_REDIS_ADDR"); addr != "" {
		rdb := redisx.New(redisx.Config{Addr: addr, Password: os.Getenv("IT_REDIS_PASSWORD")})
		t.Cleanup(func() { _ = rdb.Close() })
		limiter = &ratelimit.Fallback{Primary: ratelimit.NewRedis(rdb, "quizzes-it:"+ids.NewV7()+":"), Secondary: ratelimit.NewMemory(nil)}
	}
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Millisecond))
	svc := app.New(postgres.New(pool), clk, logging.Discard(), app.Config{Grace: 2 * time.Second})
	engine, _ := httpx.NewEngine(httpx.EngineConfig{Logger: logging.Discard(), MaxBodyBytes: 512 << 10})
	httpadapter.Register(engine, httpadapter.Deps{Service: svc, Verifier: verifier, Limiter: limiter, Logger: logging.Discard(),
		Limits: httpadapter.RateLimits{PerIP: 100000, PerSubject: 100000, Window: time.Minute}})
	srv := httptest.NewServer(engine)
	t.Cleanup(srv.Close)

	prod, err := kafkax.NewProducer(kafkax.Config{Brokers: strings.Split(brk, ","), ClientID: "quizzes-it"})
	if err != nil {
		t.Fatal(err)
	}
	rctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	relay := outbox.NewRelay(pool, prod, logging.Discard(), outbox.RelayConfig{PollInterval: 50 * time.Millisecond})
	go func() { _ = relay.Run(rctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done; prod.Close() })
	return &stack{t: t, ctx: ctx, pool: pool, srv: srv, signer: signer, svc: svc, clk: clk, brk: strings.Split(brk, ",")}
}

func (s *stack) tok(uid string, role authn.Role, gid string) string {
	tk, _, err := s.signer.Issue(uid, role, gid)
	if err != nil {
		s.t.Fatal(err)
	}
	return tk
}

func (s *stack) call(method, path, tok string, body any, hdr map[string]string) (int, map[string]any) {
	s.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, s.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

func quizBody(limitSec int) map[string]any {
	return map[string]any{
		"title": "Integration quiz", "course_id": "c_it", "time_limit_sec": limitSec, "max_attempts": 3,
		"questions": []any{
			map[string]any{"text": "2+2?", "points": 2, "options": []any{map[string]any{"key": "a", "text": "3"}, map[string]any{"key": "b", "text": "4"}}, "correct_keys": []string{"b"}},
			map[string]any{"text": "primes?", "points": 3, "topic_id": "t_primes", "options": []any{map[string]any{"key": "a", "text": "2"}, map[string]any{"key": "b", "text": "3"}, map[string]any{"key": "c", "text": "4"}}, "correct_keys": []string{"a", "b"}},
		},
	}
}

// publishAssign creates, publishes and assigns a quiz; returns quiz id and the
// author view (with the marking key).
func (s *stack) publishAssign(teacher, group string, limitSec int) (string, map[string]any) {
	tt := s.tok(teacher, authn.RoleTeacher, "")
	code, q := s.call("POST", "/v1/quizzes", tt, quizBody(limitSec), nil)
	if code != 201 {
		s.t.Fatalf("create %d %v", code, q)
	}
	id := q["id"].(string)
	if code, m := s.call("POST", "/v1/quizzes/"+id+"/publish", tt, nil, nil); code != 200 {
		s.t.Fatalf("publish %d %v", code, m)
	}
	now := s.clk.Now()
	if code, m := s.call("PUT", "/v1/quizzes/"+id+"/assignments/"+group, tt, map[string]any{"opens_at": now.Add(-time.Minute), "closes_at": now.Add(24 * time.Hour)}, nil); code != 200 {
		s.t.Fatalf("assign %d %v", code, m)
	}
	return id, q
}

func optionIDs(q map[string]any, qi int) []string {
	var out []string
	for _, o := range q["questions"].([]any)[qi].(map[string]any)["options"].([]any) {
		out = append(out, o.(map[string]any)["id"].(string))
	}
	return out
}

func questionID(q map[string]any, qi int) string {
	return q["questions"].([]any)[qi].(map[string]any)["id"].(string)
}

type rec struct {
	partition int32
	offset    int64
	key       string
	raw       []byte
	env       events.Envelope
}

// consume reads lms.quizzes from the beginning and returns the records of
// attemptID in (partition, offset) order.
func consume(t *testing.T, brk []string, attemptID string, want int) []rec {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brk...), kgo.ConsumeTopics(events.TopicQuizzes), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	var out []rec
	deadline := time.Now().Add(45 * time.Second)
	for len(out) < want && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		fs := cl.PollFetches(ctx)
		cancel()
		fs.EachRecord(func(r *kgo.Record) {
			if !bytes.Contains(r.Value, []byte(attemptID)) {
				return
			}
			env, err := events.Parse(r.Value)
			if err != nil {
				t.Errorf("invalid envelope: %v", err)
				return
			}
			out = append(out, rec{r.Partition, r.Offset, string(r.Key), append([]byte(nil), r.Value...), env})
		})
	}
	return out
}

func TestQuizFlowThroughKafka(t *testing.T) {
	s := newStack(t)
	teacher, group, student := ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid, authorView := s.publishAssign(teacher, group, 600)
	ts := s.tok(student, authn.RoleStudent, group)

	code, att := s.call("POST", "/v1/quizzes/"+qid+"/attempts", ts, nil, nil)
	if code != 201 {
		t.Fatalf("start %d %v", code, att)
	}
	aid := att["id"].(string)
	base := "/v1/attempts/" + aid
	q0, q1 := questionID(authorView, 0), questionID(authorView, 1)
	o0, o1 := optionIDs(authorView, 0), optionIDs(authorView, 1)

	adv := func(d time.Duration) { s.clk.Advance(d) }
	adv(2 * time.Second)
	if c, m := s.call("POST", base+"/questions/"+q0+"/shown", ts, nil, nil); c != 200 {
		t.Fatalf("shown %d %v", c, m)
	}
	adv(3 * time.Second)
	if c, m := s.call("PUT", base+"/answers/"+q0, ts, map[string]any{"selected_option_ids": []string{o0[0]}}, nil); c != 200 {
		t.Fatalf("answer %d %v", c, m)
	}
	adv(time.Second) // identical -> no event
	if c, m := s.call("PUT", base+"/answers/"+q0, ts, map[string]any{"selected_option_ids": []string{o0[0]}}, nil); c != 200 || m["unchanged"] != true {
		t.Fatalf("identical %d %v", c, m)
	}
	adv(4 * time.Second) // change to the right answer
	if c, m := s.call("PUT", base+"/answers/"+q0, ts, map[string]any{"selected_option_ids": []string{o0[1]}}, nil); c != 200 || m["is_change"] != true {
		t.Fatalf("change %d %v", c, m)
	}
	adv(time.Second)
	s.call("POST", base+"/questions/"+q1+"/shown", ts, nil, nil)
	adv(5 * time.Second)
	if c, m := s.call("PUT", base+"/answers/"+q1, ts, map[string]any{"selected_option_ids": []string{o1[1], o1[0]}}, nil); c != 200 {
		t.Fatalf("answer q1 %d %v", c, m)
	}
	adv(2 * time.Second)
	c, res := s.call("POST", base+"/submit", ts, nil, map[string]string{"Idempotency-Key": "it-" + aid})
	if c != 200 || res["score"] != float64(5) || res["max_score"] != float64(5) {
		t.Fatalf("submit %d %v", c, res)
	}
	if c, _ := s.call("POST", base+"/submit", ts, nil, map[string]string{"Idempotency-Key": "it-" + aid}); c != 200 {
		t.Fatalf("replay %d", c)
	}

	recs := consume(t, s.brk, aid, 5)
	if len(recs) != 5 {
		t.Fatalf("got %d records for the attempt, want 5", len(recs))
	}
	val, err := eventschema.Load(mustDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for i, r := range recs {
		types = append(types, r.env.EventType)
		if i > 0 && (r.partition != recs[0].partition || r.offset <= recs[i-1].offset) {
			t.Fatalf("records not in one partition in offset order")
		}
		if r.key != student || r.env.Actor.UserID != student || r.env.Actor.Role != "student" || r.env.Producer != "quizzes" {
			t.Fatalf("key/actor %s %+v", r.key, r.env.Actor)
		}
		if *r.env.Context.CourseID != "c_it" || *r.env.Context.GroupID != group || r.env.Context.SessionID != nil {
			t.Fatalf("context %+v", r.env.Context)
		}
		ts, _ := ids.V7Time(r.env.EventID)
		if !ts.Equal(r.env.OccurredAt.Time) {
			t.Fatalf("event_id time %v != occurred_at %v", ts, r.env.OccurredAt)
		}
		if err := val.Validate(r.raw); err != nil {
			t.Fatalf("%s schema: %v", r.env.EventType, err)
		}
	}
	if got := strings.Join(types, ","); got != "quiz.started,quiz.answered,quiz.answered,quiz.answered,quiz.submitted" {
		t.Fatalf("order %s", got)
	}
	var a1, a2, a3 events.QuizAnswered
	_ = recs[1].env.DecodePayload(&a1, true)
	_ = recs[2].env.DecodePayload(&a2, true)
	_ = recs[3].env.DecodePayload(&a3, true)
	check := func(name string, p events.QuizAnswered, idx int, change bool, shown, start int64) {
		if p.QuestionIndex != idx || p.IsChange != change || p.TimeSinceShownMs != shown || p.TimeSinceAttemptStartMs != start || p.QuestionID == "" {
			t.Fatalf("%s = %+v", name, p)
		}
	}
	check("first answer", a1, 0, false, 3000, 5000)
	check("change", a2, 0, true, 8000, 10000)
	check("q1", a3, 1, false, 5000, 16000)
	var sub events.QuizSubmitted
	_ = recs[4].env.DecodePayload(&sub, true)
	if sub.Score != 5 || sub.MaxScore != 5 || sub.AnswersChanged != 1 || sub.DurationMs != 18000 || sub.Reason != "submitted" || sub.QuizID != qid {
		t.Fatalf("submitted %+v", sub)
	}
	var started events.QuizStarted
	_ = recs[0].env.DecodePayload(&started, true)
	if started.QuestionCount != 2 || started.TimeLimitSec != 600 || started.VariantID != sub.VariantID {
		t.Fatalf("started %+v", started)
	}
	// The answer log is the persisted record of every quiz.answered.
	var logged int
	_ = s.pool.QueryRow(s.ctx, `SELECT count(*) FROM answer_log WHERE attempt_id=$1`, aid).Scan(&logged)
	if logged != 3 {
		t.Fatalf("answer_log rows %d", logged)
	}
}

func mustDir(t *testing.T) string {
	d, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExactlyOnceSubmitAndInvariantsInPostgres(t *testing.T) {
	s := newStack(t)
	teacher, group, student, other := ids.NewV7(), ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid, _ := s.publishAssign(teacher, group, 600)
	ts := s.tok(student, authn.RoleStudent, group)
	_, att := s.call("POST", "/v1/quizzes/"+qid+"/attempts", ts, nil, nil)
	aid := att["id"].(string)

	// IDOR against the real store.
	if c, _ := s.call("GET", "/v1/attempts/"+aid, s.tok(other, authn.RoleStudent, group), nil, nil); c != 404 {
		t.Fatalf("IDOR %d", c)
	}
	// Concurrent parallel start is refused.
	if c, m := s.call("POST", "/v1/quizzes/"+qid+"/attempts", ts, nil, nil); c != 409 || m["code"] != "attempt_in_progress" {
		t.Fatalf("parallel start %d %v", c, m)
	}

	var wg sync.WaitGroup
	codes := make([]int, 24)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = s.call("POST", "/v1/attempts/"+aid+"/submit", ts, nil, map[string]string{"Idempotency-Key": fmt.Sprintf("race-key-%02d", i%3)})
		}(i)
	}
	wg.Wait()
	var fin, outboxRows int
	_ = s.pool.QueryRow(s.ctx, `SELECT count(*) FROM attempt_finalizations WHERE attempt_id=$1`, aid).Scan(&fin)
	_ = s.pool.QueryRow(s.ctx, `SELECT count(*) FROM outbox WHERE event_type='quiz.submitted' AND convert_from(envelope,'UTF8') LIKE '%'||$1||'%'`, aid).Scan(&outboxRows)
	if fin != 1 || outboxRows != 1 {
		t.Fatalf("finalizations=%d quiz.submitted=%d codes=%v; want exactly one", fin, outboxRows, codes)
	}
	for _, c := range codes {
		if c != 200 && c != 409 {
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}

	// Schema-level invariants hold even for direct SQL.
	mustFail := func(name, sql string, args ...any) {
		if _, err := s.pool.Exec(s.ctx, sql, args...); err == nil {
			t.Errorf("%s: statement succeeded but must be rejected by the schema", name)
		}
	}
	mustFail("edit published quiz", `UPDATE quizzes SET title='hacked' WHERE id=$1`, qid)
	mustFail("edit published options", `UPDATE question_options SET is_correct = NOT is_correct WHERE question_id IN (SELECT id FROM questions WHERE quiz_id=$1)`, qid)
	mustFail("reopen finalized attempt", `UPDATE attempts SET status='in_progress', finished_at=NULL, score=NULL, max_score=NULL WHERE id=$1`, aid)
	mustFail("second finalization", `INSERT INTO attempt_finalizations (attempt_id, user_id, reason, finalized_at, event_id) VALUES ($1,$2,'timed_out',now(),$3)`, aid, student, ids.NewV7())
	mustFail("delete attempt", `DELETE FROM attempts WHERE id=$1`, aid)
	mustFail("bad status", `UPDATE attempts SET status='graded' WHERE id=$1`, aid)
}

func TestSweeperTimesOutOnce(t *testing.T) {
	s := newStack(t)
	teacher, group, student := ids.NewV7(), ids.NewV7(), ids.NewV7()
	qid, _ := s.publishAssign(teacher, group, 60)
	ts := s.tok(student, authn.RoleStudent, group)
	_, att := s.call("POST", "/v1/quizzes/"+qid+"/attempts", ts, nil, nil)
	aid := att["id"].(string)
	s.clk.Advance(61 * time.Second) // within grace
	if n, err := s.svc.SweepExpired(s.ctx, 50); err != nil || n != 0 {
		t.Fatalf("within grace swept %d %v", n, err)
	}
	s.clk.Advance(5 * time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ { // several replicas sweeping concurrently
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = s.svc.SweepExpired(s.ctx, 50) }()
	}
	wg.Wait()
	var status, reason string
	_ = s.pool.QueryRow(s.ctx, `SELECT a.status, f.reason FROM attempts a JOIN attempt_finalizations f ON f.attempt_id=a.id WHERE a.id=$1`, aid).Scan(&status, &reason)
	if status != "timed_out" || reason != "timed_out" {
		t.Fatalf("status %q reason %q", status, reason)
	}
	recs := consume(t, s.brk, aid, 2)
	if len(recs) != 2 || recs[1].env.EventType != events.TypeQuizSubmitted {
		t.Fatalf("records %d", len(recs))
	}
	var sub events.QuizSubmitted
	_ = recs[1].env.DecodePayload(&sub, true)
	if sub.Reason != "timed_out" || sub.DurationMs != 60000 || sub.Score != 0 {
		t.Fatalf("timeout payload %+v", sub)
	}
	if c, m := s.call("POST", "/v1/attempts/"+aid+"/submit", ts, nil, map[string]string{"Idempotency-Key": "late-key-01"}); c != 200 || m["status"] != "timed_out" {
		t.Fatalf("late submit %d %v", c, m)
	}
}
