//go:build integration

// Package integration runs the auth service in-process against real
// PostgreSQL, Redis and Kafka (docker compose infra). Configure with
// IT_PG_ADMIN_URL, IT_REDIS_ADDR, IT_REDIS_PASSWORD, IT_KAFKA_BROKERS.
package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"

	httpadapter "github.com/Newterios/Examory/services/auth/internal/adapters/http"
	"github.com/Newterios/Examory/services/auth/internal/adapters/postgres"
	redisadapter "github.com/Newterios/Examory/services/auth/internal/adapters/redis"
	"github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/auth/internal/password"
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
)

const pw = "correct horse battery staple"

type tokenIssuer struct{ s *authn.Signer }

func (t tokenIssuer) Issue(uid string, r domain.Role, gid string) (string, time.Time, error) {
	return t.s.Issue(uid, authn.Role(r), gid)
}

func env(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Skipf("%s not set; integration infra not configured", k)
	}
	return v
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// freshDB creates an isolated database and returns its URL.
func freshDB(t *testing.T, adminURL string) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	name := "auth_it_" + randHex(6)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = conn.Close(context.Background())
	})
	u, _ := url.Parse(adminURL)
	u.Path = "/" + name
	return u.String()
}

type client struct {
	t    *testing.T
	base string
}

func (c client) do(method, path, body, token, cookie string) (int, map[string]any, *http.Response) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", httpadapter.CookieName+"="+cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m, resp
}

func refreshCookie(r *http.Response) string {
	for _, c := range r.Cookies() {
		if c.Name == httpadapter.CookieName {
			return c.Value
		}
	}
	return ""
}

func TestAuthEndToEnd(t *testing.T) {
	adminURL := env(t, "IT_PG_ADMIN_URL")
	redisAddr := env(t, "IT_REDIS_ADDR")
	// Kafka is optional: without IT_KAFKA_BROKERS the relay is not started and
	// the events are verified in the outbox table instead.
	var brokers []string
	if b := os.Getenv("IT_KAFKA_BROKERS"); b != "" {
		brokers = strings.Split(b, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	log := logging.Discard()
	clk := clock.System{}

	// --- infrastructure ---
	pool, err := db.Open(ctx, db.Config{URL: freshDB(t, adminURL), ApplicationName: "auth-it"})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for i := 0; i < 2; i++ { // idempotent
		if err := migrate.Up(ctx, pool, log, postgres.Migrations(), outbox.Migrations()); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr, Password: os.Getenv("IT_REDIS_PASSWORD")})
	defer rdb.Close()
	prefix := "auth_it_" + randHex(4) + ":"
	t.Cleanup(func() {
		keys, _ := rdb.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			rdb.Del(context.Background(), keys...)
		}
	})
	if len(brokers) > 0 {
		producer, err := kafkax.NewProducer(kafkax.Config{Brokers: brokers, ClientID: "auth-it", AllowAutoTopicCreation: true})
		if err != nil {
			t.Fatal(err)
		}
		defer producer.Close()
		relayCtx, stopRelay := context.WithCancel(ctx)
		relayDone := make(chan struct{})
		go func() {
			_ = outbox.NewRelay(pool, producer, log, outbox.RelayConfig{PollInterval: 50 * time.Millisecond}).Run(relayCtx)
			close(relayDone)
		}()
		defer func() { stopRelay(); <-relayDone }()
	} else {
		t.Log("IT_KAFKA_BROKERS not set: verifying events in the outbox table instead of Kafka")
	}

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kid := authn.Thumbprint(pub)
	signer, _ := authn.NewSigner(kid, priv, "examory-auth", "examory", 10*time.Minute, clk)
	verifier, _ := authn.NewVerifier(authn.StaticKeys{kid: pub}, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})
	svc, err := app.New(ctx, app.Deps{
		Store:    postgres.New(pool),
		Sessions: redisadapter.NewRefreshStore(rdb, prefix, 7*24*time.Hour, 30*24*time.Hour),
		Lockout:  redisadapter.NewLockout(rdb, prefix),
		Hasher:   password.New(password.OWASP, 4),
		Tokens:   tokenIssuer{signer},
		Clock:    clk,
		Logger:   log,
	})
	if err != nil {
		t.Fatal(err)
	}
	adminEmail := "admin-" + randHex(3) + "@examory.local"
	if created, err := svc.EnsureBootstrapAdmin(ctx, adminEmail, pw); err != nil || !created {
		t.Fatalf("bootstrap: %v %v", created, err)
	}
	if created, _ := svc.EnsureBootstrapAdmin(ctx, "other@examory.local", pw); created {
		t.Fatal("bootstrap ran twice")
	}
	router, err := httpadapter.NewRouter(svc, verifier, httpadapter.Config{
		Engine: httpx.EngineConfig{Logger: log}, CookieSecure: true,
		JWKS:    authn.JWKSet{Keys: []authn.JWK{authn.PublicJWK(kid, pub)}},
		Limiter: &ratelimit.Fallback{Primary: ratelimit.NewRedis(rdb, prefix+"rl:"), Secondary: ratelimit.NewMemory(clk)},
		Checks:  map[string]httpx.CheckFunc{"postgres": pool.Ping},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	defer srv.Close()
	c := client{t: t, base: srv.URL}

	// --- HTTP flow ---
	st, body, _ := c.do("POST", "/v1/auth/login", `{"email":"`+adminEmail+`","password":"`+pw+`"}`, "", "")
	if st != 200 {
		t.Fatalf("admin login %d %v", st, body)
	}
	admin := body["access_token"].(string)
	mk := func(email, role string) string {
		st, b, _ := c.do("POST", "/v1/users", `{"email":"`+email+`","display_name":"IT `+role+`","role":"`+role+`","password":"`+pw+`"}`, admin, "")
		if st != 201 {
			t.Fatalf("create %s: %d %v", role, st, b)
		}
		return b["id"].(string)
	}
	sfx := randHex(3)
	teacherID := mk("teacher-"+sfx+"@x.kz", "teacher")
	studentID := mk("student-"+sfx+"@x.kz", "student")
	systemID := mk("sim-"+sfx+"@x.kz", "system")
	st, g, _ := c.do("POST", "/v1/groups", `{"name":"IT-`+sfx+`","course_id":"c_10"}`, admin, "")
	if st != 201 {
		t.Fatalf("group %d %v", st, g)
	}
	groupID := g["id"].(string)
	if st, b, _ := c.do("PUT", "/v1/groups/"+groupID+"/members/"+studentID, "", admin, ""); st != 200 || b["member_count"].(float64) != 1 {
		t.Fatalf("add member %d %v", st, b)
	}
	if st, _, _ := c.do("PUT", "/v1/groups/"+groupID+"/members/"+teacherID, "", admin, ""); st != 409 {
		t.Fatalf("teacher added to group: %d", st)
	}

	st, sess, resp := c.do("POST", "/v1/auth/login", `{"email":"STUDENT-`+sfx+`@x.kz","password":"`+pw+`"}`, "", "")
	if st != 200 {
		t.Fatalf("student login %d %v", st, sess)
	}
	studentTok := sess["access_token"].(string)
	p, err := verifier.Verify(ctx, studentTok)
	if err != nil || p.UserID != studentID || p.Role != authn.RoleStudent || p.GroupID != groupID {
		t.Fatalf("student token claims %+v %v", p, err)
	}
	if st, me, _ := c.do("GET", "/v1/me", "", studentTok, ""); st != 200 || me["group_id"] != groupID || me["id"] != studentID {
		t.Fatalf("me %d %v", st, me)
	}
	rt1 := refreshCookie(resp)
	st, _, resp = c.do("POST", "/v1/auth/refresh", "", "", rt1)
	rt2 := refreshCookie(resp)
	if st != 200 || rt2 == "" || rt2 == rt1 {
		t.Fatalf("refresh %d", st)
	}
	if st, _, _ := c.do("POST", "/v1/auth/refresh", "", "", rt1); st != 401 {
		t.Fatalf("replayed refresh accepted: %d", st)
	}
	if st, _, _ := c.do("POST", "/v1/auth/refresh", "", "", rt2); st != 401 {
		t.Fatalf("family not revoked after reuse: %d", st)
	}
	// New session, then logout.
	_, _, resp = c.do("POST", "/v1/auth/login", `{"email":"student-`+sfx+`@x.kz","password":"`+pw+`"}`, "", "")
	rt3 := refreshCookie(resp)
	if st, _, _ := c.do("POST", "/v1/auth/logout", "", "", rt3); st != 204 {
		t.Fatalf("logout %d", st)
	}
	if st, _, _ := c.do("POST", "/v1/auth/refresh", "", "", rt3); st != 401 {
		t.Fatal("refresh after logout")
	}
	// system role user (research simulator) can log in.
	if st, b, _ := c.do("POST", "/v1/auth/login", `{"email":"sim-`+sfx+`@x.kz","password":"`+pw+`"}`, "", ""); st != 200 || b["user"].(map[string]any)["role"] != "system" {
		t.Fatalf("system login %d %v", st, b)
	}
	if st, _, _ := c.do("GET", "/readyz", "", "", ""); st != 200 {
		t.Fatalf("readyz %d", st)
	}

	// --- schema invariants enforced by the database ---
	if _, err := pool.Exec(ctx, `INSERT INTO group_members (user_id, group_id, added_by, added_at) VALUES ($1,$2,$1,now())`, teacherID, groupID); err == nil {
		t.Fatal("DB accepted a teacher as group member")
	}
	if _, err := pool.Exec(ctx, `UPDATE audit_log SET outcome='success'`); err == nil {
		t.Fatal("audit_log is not append-only")
	}
	_, err = pool.Exec(ctx, `INSERT INTO users (id,email,display_name,role,status,password_hash,created_at,updated_at) VALUES ($1,'UPPER@X.KZ','x','student','active','$argon2id$x',now(),now())`, ids.NewV7())
	if code, constraint, _ := db.ErrorCode(err); code != db.CodeCheckViolation || constraint != "users_email_check" {
		t.Fatalf("DB accepted a non-lower-case e-mail: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO group_members (user_id, group_id, added_by, added_at) VALUES ($1,$2,$1,now())`, systemID, groupID)
	if code, _, _ := db.ErrorCode(err); code != db.CodeCheckViolation {
		t.Fatalf("DB accepted a system user as group member: %v", err)
	}
	var reuse int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action='auth.token_reuse' AND target_id=$1`, studentID).Scan(&reuse); err != nil || reuse != 1 {
		t.Fatalf("token reuse audit rows: %d %v", reuse, err)
	}
	var leaked int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE detail::text ILIKE '%password%' OR detail::text ILIKE '%argon2%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("secrets in audit log")
	}

	// --- events on Kafka ---
	dir, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	validator, err := eventschema.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ours := map[string]bool{teacherID: true, studentID: true, systemID: true, groupID: true}
	want := map[string]int{
		"user.created/teacher": 1, "user.created/student": 1, "user.created/system": 1,
		"group.updated/created": 1, "group.updated/member_added": 1,
		"user.logged_in/" + studentID: 2, "user.logged_out/token_reuse": 1, "user.logged_out/logout": 1,
	}
	got := map[string]int{}
	seenIDs := map[string]bool{}
	check := func(key string, value []byte) {
		var head struct {
			EventType string         `json:"event_type"`
			EventID   string         `json:"event_id"`
			Payload   map[string]any `json:"payload"`
		}
		if json.Unmarshal(value, &head) != nil {
			return
		}
		subject, _ := head.Payload["user_id"].(string)
		if head.EventType == events.TypeGroupUpdated {
			subject, _ = head.Payload["group_id"].(string)
		}
		if !ours[subject] || seenIDs[head.EventID] { // other runs; at-least-once duplicates
			return
		}
		seenIDs[head.EventID] = true
		if err := validator.Validate(value); err != nil {
			t.Errorf("%s fails schema: %v", head.EventType, err)
		}
		if key != subject {
			t.Errorf("%s keyed by %q, want %q", head.EventType, key, subject)
		}
		switch head.EventType {
		case events.TypeUserCreated:
			got["user.created/"+head.Payload["role"].(string)]++
		case events.TypeGroupUpdated:
			got["group.updated/"+head.Payload["change"].(string)]++
		case events.TypeUserLoggedIn:
			got["user.logged_in/"+subject]++
		case events.TypeUserLoggedOut:
			got["user.logged_out/"+head.Payload["reason"].(string)]++
		}
	}
	done := func() bool {
		for k, n := range want {
			if got[k] < n {
				return false
			}
		}
		return true
	}
	if len(brokers) > 0 {
		cons, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(events.TopicUsers), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		if err != nil {
			t.Fatal(err)
		}
		defer cons.Close()
		deadline := time.Now().Add(45 * time.Second)
		for !done() && time.Now().Before(deadline) {
			pctx, pcancel := context.WithTimeout(ctx, 3*time.Second)
			fs := cons.PollFetches(pctx)
			pcancel()
			fs.EachRecord(func(r *kgo.Record) { check(string(r.Key), r.Value) })
		}
	} else {
		rows, err := pool.Query(ctx, `SELECT topic, partition_key, envelope FROM outbox ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var topic, key string
			var body []byte
			if err := rows.Scan(&topic, &key, &body); err != nil {
				t.Fatal(err)
			}
			if topic != events.TopicUsers {
				t.Errorf("topic %s", topic)
			}
			check(key, body)
		}
		rows.Close()
	}
	for k, n := range want {
		if got[k] < n {
			t.Errorf("event %s: got %d, want %d", k, got[k], n)
		}
	}
	t.Logf("verified %d distinct lms.users events for this run", len(seenIDs))
	if len(brokers) == 0 {
		return
	}
	// The relay marks rows after the broker ack; give it time to catch up.
	var pending int
	for wait := time.Now().Add(15 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&pending)
		if pending == 0 || time.Now().After(wait) {
			break
		}
	}
	if pending != 0 {
		t.Errorf("%d outbox rows still pending", pending)
	}
}
