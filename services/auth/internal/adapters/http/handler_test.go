package http_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	httpadapter "github.com/Newterios/Examory/services/auth/internal/adapters/http"
	"github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/app/apptest"
	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/auth/internal/password"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

const pw = "correct horse battery staple"

type tokenIssuer struct{ s *authn.Signer }

func (t tokenIssuer) Issue(uid string, r domain.Role, gid string) (string, time.Time, error) {
	return t.s.Issue(uid, authn.Role(r), gid)
}

type env struct {
	h     *gin.Engine
	svc   *app.Service
	store *apptest.Store
	admin string // admin access token
}

func newEnv(t *testing.T) *env {
	t.Helper()
	clk := clock.System{}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := authn.NewSigner("k1", priv, "examory-auth", "examory", 10*time.Minute, clk)
	ver, _ := authn.NewVerifier(authn.StaticKeys{"k1": pub}, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})
	store := apptest.NewStore()
	svc, err := app.New(context.Background(), app.Deps{
		Store: store, Sessions: apptest.NewSessions(), Lockout: apptest.NewLockout(clk),
		Hasher: password.New(password.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}, 4),
		Tokens: tokenIssuer{signer}, Clock: clk, Logger: logging.Discard(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnsureBootstrapAdmin(context.Background(), "admin@examory.local", pw); err != nil {
		t.Fatal(err)
	}
	h, err := httpadapter.NewRouter(svc, ver, httpadapter.Config{
		Engine:       httpx.EngineConfig{Logger: logging.Discard()},
		CookieSecure: true,
		AllowOrigins: []string{"https://app.examory.kz"},
		JWKS:         authn.JWKSet{Keys: []authn.JWK{authn.PublicJWK("k1", pub)}},
		Limiter:      ratelimit.NewMemory(clk),
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{h: h, svc: svc, store: store}
	w := e.do("POST", "/v1/auth/login", `{"email":"admin@examory.local","password":"`+pw+`"}`, "", nil)
	if w.Code != 200 {
		t.Fatalf("admin login %d %s", w.Code, w.Body)
	}
	e.admin = decode[map[string]any](t, w)["access_token"].(string)
	return e
}

func (e *env) do(method, path, body, token string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", w.Body, err)
	}
	return v
}

func (e *env) createUser(t *testing.T, email, role string) string {
	t.Helper()
	w := e.do("POST", "/v1/users", `{"email":"`+email+`","display_name":"X","role":"`+role+`","password":"`+pw+`"}`, e.admin, nil)
	if w.Code != 201 {
		t.Fatalf("create user %d %s", w.Code, w.Body)
	}
	return decode[map[string]any](t, w)["id"].(string)
}

func (e *env) login(t *testing.T, email string) (string, *http.Cookie) {
	t.Helper()
	w := e.do("POST", "/v1/auth/login", `{"email":"`+email+`","password":"`+pw+`"}`, "", nil)
	if w.Code != 200 {
		t.Fatalf("login %d %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == httpadapter.CookieName {
			return decode[map[string]any](t, w)["access_token"].(string), c
		}
	}
	t.Fatal("no refresh cookie")
	return "", nil
}

func TestLoginResponseAndCookie(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "s@x.kz", "student")
	w := e.do("POST", "/v1/auth/login", `{"email":"s@x.kz","password":"`+pw+`"}`, "", nil)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
	}
	body := decode[map[string]any](t, w)
	if body["token_type"] != "Bearer" || body["expires_in"].(float64) < 590 || body["user"].(map[string]any)["group_id"] != nil {
		t.Fatalf("body %v", body)
	}
	if strings.Contains(w.Body.String(), "argon2") || strings.Contains(w.Body.String(), "password") {
		t.Fatal("hash leaked")
	}
	raw := w.Header().Get("Set-Cookie")
	for _, want := range []string{"examory_refresh=", "Path=/v1/auth", "HttpOnly", "Secure", "SameSite=Strict", "Max-Age=604800"} {
		if !strings.Contains(raw, want) {
			t.Errorf("cookie missing %s: %s", want, raw)
		}
	}
	// Identical problem for wrong password and unknown user.
	a := e.do("POST", "/v1/auth/login", `{"email":"s@x.kz","password":"wrong-password-1"}`, "", nil)
	b := e.do("POST", "/v1/auth/login", `{"email":"nobody@x.kz","password":"wrong-password-1"}`, "", nil)
	pa, pb := decode[httpx.Problem](t, a), decode[httpx.Problem](t, b)
	if a.Code != 401 || b.Code != 401 || pa.Code != pb.Code || pa.Detail != pb.Detail {
		t.Fatalf("distinguishable: %d %+v / %d %+v", a.Code, pa, b.Code, pb)
	}
	// Strict JSON.
	if w := e.do("POST", "/v1/auth/login", `{"email":"s@x.kz","password":"x","admin":true}`, "", nil); w.Code != 400 {
		t.Fatalf("unknown field accepted: %d", w.Code)
	}
}

func TestLockoutReturns429(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "s@x.kz", "student")
	for i := 0; i < 5; i++ {
		e.do("POST", "/v1/auth/login", `{"email":"s@x.kz","password":"wrong-password-1"}`, "", nil)
	}
	w := e.do("POST", "/v1/auth/login", `{"email":"s@x.kz","password":"`+pw+`"}`, "", nil)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" || decode[httpx.Problem](t, w).Code != "account_locked" {
		t.Fatalf("expected lock: %d %s", w.Code, w.Body)
	}
}

func TestPerIPLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	got429 := false
	for i := 0; i < 25; i++ {
		email := "user" + string(rune('a'+i)) + "@x.kz"
		if w := e.do("POST", "/v1/auth/login", `{"email":"`+email+`","password":"wrong-password-1"}`, "", nil); w.Code == 429 {
			got429 = true
			if decode[httpx.Problem](t, w).Code != "rate_limited" {
				t.Fatal("wrong limiter")
			}
		}
	}
	if !got429 {
		t.Fatal("per-IP login limit not enforced")
	}
}

func TestRefreshRotationOverHTTP(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "s@x.kz", "student")
	_, c1 := e.login(t, "s@x.kz")
	cookie := func(c *http.Cookie) map[string]string { return map[string]string{"Cookie": c.Name + "=" + c.Value} }
	w := e.do("POST", "/v1/auth/refresh", "", "", cookie(c1))
	if w.Code != 200 {
		t.Fatalf("refresh %d %s", w.Code, w.Body)
	}
	var c2 *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == httpadapter.CookieName {
			c2 = c
		}
	}
	if c2 == nil || c2.Value == c1.Value {
		t.Fatal("cookie not rotated")
	}
	// Replay the first cookie: 401 + cookie cleared, and c2 is dead too.
	w = e.do("POST", "/v1/auth/refresh", "", "", cookie(c1))
	if w.Code != 401 || !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("replay %d %v", w.Code, w.Header())
	}
	if w := e.do("POST", "/v1/auth/refresh", "", "", cookie(c2)); w.Code != 401 {
		t.Fatal("family not revoked")
	}
	// A refresh token in the body is never accepted.
	if w := e.do("POST", "/v1/auth/refresh", `{"refresh_token":"`+c2.Value+`"}`, "", nil); w.Code != 401 {
		t.Fatalf("body token accepted: %d", w.Code)
	}
}

func TestCSRFOriginCheck(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "s@x.kz", "student")
	_, c := e.login(t, "s@x.kz")
	h := map[string]string{"Cookie": c.Name + "=" + c.Value, "Origin": "https://evil.example"}
	if w := e.do("POST", "/v1/auth/refresh", "", "", h); w.Code != 403 {
		t.Fatalf("cross-origin refresh: %d", w.Code)
	}
	h["Origin"] = "https://app.examory.kz"
	if w := e.do("POST", "/v1/auth/refresh", "", "", h); w.Code != 200 {
		t.Fatalf("allowed origin: %d %s", w.Code, w.Body)
	}
}

func TestLogoutOverHTTP(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "s@x.kz", "student")
	_, c := e.login(t, "s@x.kz")
	h := map[string]string{"Cookie": c.Name + "=" + c.Value}
	if w := e.do("POST", "/v1/auth/logout", "", "", h); w.Code != 204 || !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout %d", w.Code)
	}
	if w := e.do("POST", "/v1/auth/refresh", "", "", h); w.Code != 401 {
		t.Fatal("refresh after logout")
	}
	if w := e.do("POST", "/v1/auth/logout", "", "", nil); w.Code != 204 {
		t.Fatal("logout without cookie must be idempotent")
	}
}

func TestRBACOnAdminRoutes(t *testing.T) {
	e := newEnv(t)
	sid := e.createUser(t, "s@x.kz", "student")
	e.createUser(t, "t@x.kz", "teacher")
	e.createUser(t, "sys@x.kz", "system")
	gid := ids.NewV7()
	routes := []struct{ method, path, body string }{
		{"GET", "/v1/users", ""},
		{"POST", "/v1/users", `{"email":"n@x.kz","display_name":"N","role":"admin","password":"` + pw + `"}`},
		{"GET", "/v1/users/" + sid, ""},
		{"PATCH", "/v1/users/" + sid, `{"role":"admin"}`},
		{"POST", "/v1/groups", `{"name":"G"}`},
		{"GET", "/v1/groups", ""},
		{"GET", "/v1/groups/" + gid, ""},
		{"PUT", "/v1/groups/" + gid + "/members/" + sid, ""},
		{"DELETE", "/v1/groups/" + gid + "/members/" + sid, ""},
	}
	for _, who := range []string{"s@x.kz", "t@x.kz", "sys@x.kz"} {
		tok, _ := e.login(t, who)
		for _, r := range routes {
			if w := e.do(r.method, r.path, r.body, tok, nil); w.Code != 403 {
				t.Errorf("%s %s %s: %d", who, r.method, r.path, w.Code)
			}
		}
	}
	for _, r := range routes {
		if w := e.do(r.method, r.path, r.body, "", nil); w.Code != 401 {
			t.Errorf("anonymous %s %s: %d", r.method, r.path, w.Code)
		}
	}
	// /v1/me works for everyone and returns only the caller.
	tok, _ := e.login(t, "s@x.kz")
	w := e.do("GET", "/v1/me", "", tok, nil)
	if w.Code != 200 || decode[map[string]any](t, w)["id"] != sid {
		t.Fatalf("me %d %s", w.Code, w.Body)
	}
}

func TestAdminFlowAndSelfDemotion(t *testing.T) {
	e := newEnv(t)
	sid := e.createUser(t, "s@x.kz", "student")
	tid := e.createUser(t, "t@x.kz", "teacher")
	w := e.do("POST", "/v1/groups", `{"name":"SE-2424","course_id":"c_10"}`, e.admin, nil)
	if w.Code != 201 {
		t.Fatalf("group %d %s", w.Code, w.Body)
	}
	gid := decode[map[string]any](t, w)["id"].(string)
	if w := e.do("PUT", "/v1/groups/"+gid+"/members/"+tid, "", e.admin, nil); w.Code != 409 {
		t.Fatalf("teacher membership: %d", w.Code)
	}
	if w := e.do("PUT", "/v1/groups/"+gid+"/members/"+sid, "", e.admin, nil); w.Code != 200 || decode[map[string]any](t, w)["member_count"].(float64) != 1 {
		t.Fatalf("add member %d %s", w.Code, w.Body)
	}
	w = e.do("GET", "/v1/groups/"+gid, "", e.admin, nil)
	g := decode[map[string]any](t, w)
	if len(g["members"].([]any)) != 1 || g["course_id"] != "c_10" {
		t.Fatalf("group %v", g)
	}
	tok, _ := e.login(t, "s@x.kz")
	if me := decode[map[string]any](t, e.do("GET", "/v1/me", "", tok, nil)); me["group_id"] != gid {
		t.Fatalf("group_id not in /me: %v", me)
	}
	// Admin cannot demote self.
	me := decode[map[string]any](t, e.do("GET", "/v1/me", "", e.admin, nil))
	if w := e.do("PATCH", "/v1/users/"+me["id"].(string), `{"role":"teacher"}`, e.admin, nil); w.Code != 409 {
		t.Fatalf("self demotion: %d", w.Code)
	}
	if w := e.do("PATCH", "/v1/users/"+sid, `{"status":"frozen"}`, e.admin, nil); w.Code != 422 {
		t.Fatalf("bad status: %d", w.Code)
	}
	if w := e.do("GET", "/v1/users/not-a-uuid", "", e.admin, nil); w.Code != 404 {
		t.Fatalf("bad id: %d", w.Code)
	}
	w = e.do("GET", "/v1/users?limit=2", "", e.admin, nil)
	p := decode[map[string]any](t, w)
	if len(p["items"].([]any)) != 2 || p["next_cursor"] == nil {
		t.Fatalf("pagination %v", p)
	}
	if w := e.do("GET", "/v1/users?limit=1000", "", e.admin, nil); w.Code != 400 {
		t.Fatal("page size cap not enforced")
	}
}

func TestJWKS(t *testing.T) {
	e := newEnv(t)
	w := e.do("GET", "/.well-known/jwks.json", "", "", nil)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "max-age=300") {
		t.Fatalf("jwks %d %v", w.Code, w.Header())
	}
	if _, err := authn.ParseJWKSet(w.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.Body.String(), `"d"`) {
		t.Fatal("private key material in JWKS")
	}
}
