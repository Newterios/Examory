package app_test

import (
	"context"
	"encoding/json"
	"errors"
	. "github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/app/apptest"
	"strings"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/auth/internal/password"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
)

// cheap parameters keep tests fast; production uses password.OWASP.
var testParams = password.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

const pw = "correct horse battery staple"

type fixture struct {
	svc    *Service
	store  *apptest.Store
	sess   *apptest.Sessions
	lock   *apptest.Lockout
	hasher *apptest.SpyHasher
	clk    *clock.Fake
	admin  domain.User
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	f := &fixture{
		store:  apptest.NewStore(),
		sess:   apptest.NewSessions(),
		lock:   apptest.NewLockout(clk),
		hasher: &apptest.SpyHasher{Hasher: password.New(testParams, 4)},
		clk:    clk,
	}
	svc, err := New(context.Background(), Deps{Store: f.store, Sessions: f.sess, Lockout: f.lock, Hasher: f.hasher,
		Tokens: apptest.Tokens{Clk: clk}, Clock: clk, Logger: logging.Discard(), Policy: domain.DefaultLockout})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	if _, err := svc.EnsureBootstrapAdmin(context.Background(), "admin@examory.local", pw+"-admin"); err != nil {
		t.Fatal(err)
	}
	a, _ := f.store.UserByEmail(context.Background(), "admin@examory.local")
	f.admin = *a
	return f
}

func (f *fixture) adminActor() Actor {
	return Actor{UserID: f.admin.ID, Role: domain.RoleAdmin, IP: "10.0.0.1"}
}

func (f *fixture) mkUser(t *testing.T, email string, role domain.Role) domain.User {
	t.Helper()
	u, err := f.svc.CreateUser(context.Background(), f.adminActor(), CreateUserInput{Email: email, DisplayName: "U " + email, Role: role, Password: pw})
	if err != nil {
		t.Fatal(err)
	}
	return *u
}

func TestLoginSuccessAndEvent(t *testing.T) {
	f := newFixture(t)
	u := f.mkUser(t, "Student@Example.kz", domain.RoleStudent)
	if u.Email != "student@example.kz" {
		t.Fatalf("email not normalized: %s", u.Email)
	}
	s, err := f.svc.Login(context.Background(), "  STUDENT@example.kz ", pw, "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if s.User.ID != u.ID || s.AccessToken == "" || !ValidRefreshTokenFormat(s.RefreshToken) {
		t.Fatalf("bad session %+v", s)
	}
	ev := f.store.EventsOfType(events.TypeUserLoggedIn)
	if len(ev) != 1 || ev[0].Key != u.ID || ev[0].Topic != events.TopicUsers || ev[0].Env.Actor.UserID != u.ID {
		t.Fatalf("logged_in event %+v", ev)
	}
	if f.store.AuditActions()["auth.login:success"] != 1 {
		t.Fatal("login not audited")
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	f := newFixture(t)
	u := f.mkUser(t, "s@x.kz", domain.RoleStudent)
	d := f.mkUser(t, "gone@x.kz", domain.RoleStudent)
	if _, err := f.svc.UpdateUser(context.Background(), f.adminActor(), d.ID, UpdateUserInput{Status: ptr(domain.StatusDeactivated)}); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, email, pass string }{
		{"wrong password", "s@x.kz", pw + "x"},
		{"unknown user", "nobody@x.kz", pw},
		{"deactivated", "gone@x.kz", pw},
		{"empty", "", ""},
		{"too long password", "s@x.kz", strings.Repeat("a", 10000)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := f.svc.Login(context.Background(), c.email, c.pass, "")
			if !errors.Is(err, domain.ErrInvalidCredentials) || err.Error() != domain.ErrInvalidCredentials.Error() {
				t.Fatalf("got %v", err)
			}
		})
	}
	// Unknown and deactivated accounts are verified against the dummy hash.
	_, _ = f.svc.Login(context.Background(), "nobody2@x.kz", pw, "")
	if f.hasher.LastVerified() != DummyHash(f.svc) {
		t.Fatal("dummy hash not used for unknown user")
	}
	_, _ = f.svc.Login(context.Background(), "gone@x.kz", pw, "")
	if f.hasher.LastVerified() != DummyHash(f.svc) {
		t.Fatal("dummy hash not used for deactivated user")
	}
	_, _ = f.svc.Login(context.Background(), "s@x.kz", "wrong-password-123", "")
	if f.hasher.LastVerified() != u.PasswordHash {
		t.Fatal("real hash not used for existing user")
	}
}

func TestLockoutEscalationAndReset(t *testing.T) {
	f := newFixture(t)
	f.mkUser(t, "s@x.kz", domain.RoleStudent)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if _, err := f.svc.Login(ctx, "s@x.kz", "bad-password-123", ""); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	before := f.hasher.Count()
	_, err := f.svc.Login(ctx, "s@x.kz", pw, "") // correct password while locked
	var le *domain.LockedError
	if !errors.As(err, &le) || !le.Until.Equal(f.clk.Now().Add(30*time.Second)) {
		t.Fatalf("expected 30s lock, got %v", err)
	}
	if f.hasher.Count() != before {
		t.Fatal("password verified while locked")
	}
	if f.store.AuditActions()["auth.lockout:denied"] != 1 {
		t.Fatal("lockout not audited")
	}
	// Next failure after the lock doubles the lock.
	f.clk.Advance(31 * time.Second)
	_, _ = f.svc.Login(ctx, "s@x.kz", "bad-password-123", "")
	if _, err := f.svc.Login(ctx, "s@x.kz", pw, ""); !errors.As(err, &le) || !le.Until.Equal(f.clk.Now().Add(time.Minute)) {
		t.Fatalf("expected 60s lock, got %v", err)
	}
	f.clk.Advance(61 * time.Second)
	if _, err := f.svc.Login(ctx, "s@x.kz", pw, ""); err != nil {
		t.Fatalf("login after lock: %v", err)
	}
	if n := f.lock.Fails[AccountKey("s@x.kz")]; n != 0 {
		t.Fatalf("failures not reset: %d", n)
	}
	// Unknown e-mails lock too (no enumeration through lockout behaviour).
	for i := 0; i < 5; i++ {
		_, _ = f.svc.Login(ctx, "ghost@x.kz", pw, "")
	}
	if _, err := f.svc.Login(ctx, "ghost@x.kz", pw, ""); !errors.As(err, &le) {
		t.Fatalf("unknown account not locked: %v", err)
	}
}

func TestRehashOnLogin(t *testing.T) {
	f := newFixture(t)
	weak := password.New(password.Params{MemoryKiB: 32, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}, 1)
	u := f.mkUser(t, "s@x.kz", domain.RoleStudent)
	old, _ := weak.Hash(context.Background(), pw)
	_ = f.store.UpdatePasswordHash(context.Background(), u.ID, old, f.clk.Now())
	if _, err := f.svc.Login(context.Background(), "s@x.kz", pw, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := f.store.UserByID(context.Background(), u.ID)
	if got.PasswordHash == old || !strings.Contains(got.PasswordHash, "m=64,t=1,p=1") {
		t.Fatalf("not rehashed: %s", got.PasswordHash)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	f := newFixture(t)
	f.mkUser(t, "s@x.kz", domain.RoleStudent)
	ctx := context.Background()
	s1, err := f.svc.Login(ctx, "s@x.kz", pw, "")
	if err != nil {
		t.Fatal(err)
	}
	s2, err := f.svc.Refresh(ctx, s1.RefreshToken, "")
	if err != nil || s2.RefreshToken == s1.RefreshToken {
		t.Fatalf("rotation: %v", err)
	}
	s3, err := f.svc.Refresh(ctx, s2.RefreshToken, "")
	if err != nil {
		t.Fatal(err)
	}
	// Replay of an already-rotated token: reuse => whole family revoked.
	if _, err := f.svc.Refresh(ctx, s1.RefreshToken, ""); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatalf("replay accepted: %v", err)
	}
	if _, err := f.svc.Refresh(ctx, s3.RefreshToken, ""); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatal("family not revoked after reuse")
	}
	if f.store.AuditActions()["auth.token_reuse:denied"] != 1 {
		t.Fatal("reuse not audited")
	}
	ev := f.store.EventsOfType(events.TypeUserLoggedOut)
	var p events.UserLoggedOut
	if len(ev) != 1 || json.Unmarshal(ev[0].Env.Payload, &p) != nil || p.Reason != events.LogoutTokenReuse {
		t.Fatalf("logged_out token_reuse event: %+v", ev)
	}
	// A separate session (other device) is unaffected.
	other, _ := f.svc.Login(ctx, "s@x.kz", pw, "")
	if _, err := f.svc.Refresh(ctx, other.RefreshToken, ""); err != nil {
		t.Fatalf("independent family affected: %v", err)
	}
	for _, junk := range []string{"", "abc", strings.Repeat("A", 43), "../../etc/passwd"} {
		if _, err := f.svc.Refresh(ctx, junk, ""); !errors.Is(err, domain.ErrInvalidSession) {
			t.Fatalf("junk %q: %v", junk, err)
		}
	}
}

func TestRefreshUsesCurrentRoleAndRejectsDeactivated(t *testing.T) {
	f := newFixture(t)
	u := f.mkUser(t, "s@x.kz", domain.RoleStudent)
	ctx := context.Background()
	g, _ := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "SE-2424"})
	s, _ := f.svc.Login(ctx, "s@x.kz", pw, "")
	if _, err := f.svc.AddMember(ctx, f.adminActor(), g.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := f.svc.Refresh(ctx, s.RefreshToken, "")
	if err != nil || s2.User.GroupID != g.ID || !strings.Contains(s2.AccessToken, g.ID) {
		t.Fatalf("refresh did not pick up group: %v %+v", err, s2)
	}
	if _, err := f.svc.UpdateUser(ctx, f.adminActor(), u.ID, UpdateUserInput{Status: ptr(domain.StatusDeactivated)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(ctx, s2.RefreshToken, ""); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatal("deactivated user refreshed")
	}
}

func TestLogout(t *testing.T) {
	f := newFixture(t)
	f.mkUser(t, "s@x.kz", domain.RoleStudent)
	ctx := context.Background()
	s, _ := f.svc.Login(ctx, "s@x.kz", pw, "")
	if err := f.svc.Logout(ctx, s.RefreshToken, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Refresh(ctx, s.RefreshToken, ""); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatal("refresh after logout")
	}
	if err := f.svc.Logout(ctx, s.RefreshToken, ""); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := f.svc.Logout(ctx, "garbage", ""); err != nil {
		t.Fatal(err)
	}
	if n := len(f.store.EventsOfType(events.TypeUserLoggedOut)); n != 1 {
		t.Fatalf("logged_out events: %d, want exactly 1", n)
	}
}

func TestAdminAuthorization(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	teacher := f.mkUser(t, "t@x.kz", domain.RoleTeacher)
	for _, role := range []domain.Role{domain.RoleStudent, domain.RoleTeacher, domain.RoleSystem} {
		a := Actor{UserID: teacher.ID, Role: role}
		if _, err := f.svc.CreateUser(ctx, a, CreateUserInput{Email: "z@x.kz", DisplayName: "Z", Role: domain.RoleAdmin, Password: pw}); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s created a user: %v", role, err)
		}
		if _, err := f.svc.UpdateUser(ctx, a, teacher.ID, UpdateUserInput{Role: ptr(domain.RoleAdmin)}); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s escalated: %v", role, err)
		}
		if _, err := f.svc.ListUsers(ctx, a, 10, ""); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s listed users", role)
		}
		if _, err := f.svc.CreateGroup(ctx, a, CreateGroupInput{Name: "g"}); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s created group", role)
		}
	}
}

func TestCreateUserValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.mkUser(t, "dup@x.kz", domain.RoleStudent)
	if _, err := f.svc.CreateUser(ctx, f.adminActor(), CreateUserInput{Email: "DUP@x.kz", DisplayName: "D", Role: domain.RoleStudent, Password: pw}); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("duplicate e-mail: %v", err)
	}
	var ve *domain.ValidationError
	bad := []CreateUserInput{
		{Email: "a@x.kz", DisplayName: "A", Role: "root", Password: pw},
		{Email: "a@x.kz", DisplayName: "A", Role: domain.RoleStudent, Password: "short"},
		{Email: "a@x.kz", DisplayName: "A", Role: domain.RoleStudent, Password: "A@x.kz"},
		{Email: "not-an-email", DisplayName: "A", Role: domain.RoleStudent, Password: pw},
		{Email: "a@x.kz", DisplayName: "", Role: domain.RoleStudent, Password: pw},
	}
	for i, in := range bad {
		if _, err := f.svc.CreateUser(ctx, f.adminActor(), in); !errors.As(err, &ve) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	if n := len(f.store.EventsOfType(events.TypeUserCreated)); n != 2 { // bootstrap admin + dup@x.kz
		t.Fatalf("user.created events: %d", n)
	}
}

func TestAdminSelfProtection(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.UpdateUser(ctx, f.adminActor(), f.admin.ID, UpdateUserInput{Role: ptr(domain.RoleTeacher)}); !errors.Is(err, domain.ErrSelfModification) {
		t.Fatalf("self demote: %v", err)
	}
	if _, err := f.svc.UpdateUser(ctx, f.adminActor(), f.admin.ID, UpdateUserInput{Status: ptr(domain.StatusDeactivated)}); !errors.Is(err, domain.ErrSelfModification) {
		t.Fatalf("self deactivate: %v", err)
	}
	other := f.mkUser(t, "a2@x.kz", domain.RoleAdmin)
	a2 := Actor{UserID: other.ID, Role: domain.RoleAdmin}
	if _, err := f.svc.UpdateUser(ctx, a2, f.admin.ID, UpdateUserInput{Role: ptr(domain.RoleTeacher)}); err != nil {
		t.Fatalf("other admin demotes: %v", err)
	}
	// other is now the last active admin; nobody else can remove it, and it cannot remove itself.
	if _, err := f.svc.UpdateUser(ctx, a2, other.ID, UpdateUserInput{Status: ptr(domain.StatusDeactivated)}); !errors.Is(err, domain.ErrSelfModification) {
		t.Fatalf("last admin self: %v", err)
	}
}

func TestGroupMembershipRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	st := f.mkUser(t, "s@x.kz", domain.RoleStudent)
	te := f.mkUser(t, "t@x.kz", domain.RoleTeacher)
	g1, err := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "SE-2424", CourseID: "c_10"})
	if err != nil {
		t.Fatal(err)
	}
	g2, _ := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "SE-2425"})
	if _, err := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "SE-2424"}); !errors.Is(err, domain.ErrGroupNameTaken) {
		t.Fatalf("dup group: %v", err)
	}
	if _, err := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "x", CourseID: "bad id!"}); err == nil {
		t.Fatal("bad course id accepted")
	}
	if _, err := f.svc.AddMember(ctx, f.adminActor(), g1.ID, te.ID); !errors.Is(err, domain.ErrNotStudent) {
		t.Fatalf("teacher added: %v", err)
	}
	if _, err := f.svc.AddMember(ctx, f.adminActor(), ids.NewV7(), st.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing group: %v", err)
	}
	g, err := f.svc.AddMember(ctx, f.adminActor(), g1.ID, st.ID)
	if err != nil || g.MemberCount != 1 {
		t.Fatalf("add: %v %+v", err, g)
	}
	if g, _ := f.svc.AddMember(ctx, f.adminActor(), g1.ID, st.ID); g.MemberCount != 1 { // no-op
		t.Fatal("re-add changed count")
	}
	if _, err := f.svc.AddMember(ctx, f.adminActor(), g2.ID, st.ID); err != nil { // move
		t.Fatal(err)
	}
	var changes []string
	for _, p := range f.store.EventsOfType(events.TypeGroupUpdated) {
		var gu events.GroupUpdated
		_ = json.Unmarshal(p.Env.Payload, &gu)
		if p.Key != gu.GroupID {
			t.Fatal("group.updated must be keyed by group id")
		}
		changes = append(changes, gu.Change+":"+gu.Name)
	}
	want := "created:SE-2424 created:SE-2425 member_added:SE-2424 member_removed:SE-2424 member_added:SE-2425"
	if strings.Join(changes, " ") != want {
		t.Fatalf("group events:\n got %v\nwant %s", changes, want)
	}
	u, _ := f.store.UserByID(ctx, st.ID)
	if u.GroupID != g2.ID {
		t.Fatal("not moved")
	}
	if err := f.svc.RemoveMember(ctx, f.adminActor(), g1.ID, st.ID); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("remove from wrong group: %v", err)
	}
	// Role change away from student removes membership.
	if _, err := f.svc.UpdateUser(ctx, f.adminActor(), st.ID, UpdateUserInput{Role: ptr(domain.RoleTeacher)}); err != nil {
		t.Fatal(err)
	}
	u, _ = f.store.UserByID(ctx, st.ID)
	if u.GroupID != "" || u.Role != domain.RoleTeacher {
		t.Fatalf("membership kept: %+v", u)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.EnsureBootstrapAdmin(context.Background(), "admin2@examory.local", pw)
	if err != nil || created {
		t.Fatalf("second bootstrap: %v %v", created, err)
	}
	if _, err := f.svc.EnsureBootstrapAdmin(context.Background(), "x@y.kz", "short"); err == nil {
		t.Fatal("weak bootstrap password accepted")
	}
}

// TestEventsMatchSchemas is the contract test: every event the use cases
// produce validates against docs/03_architecture/schemas.
func TestEventsMatchSchemas(t *testing.T) {
	dir, err := eventschema.FindDir()
	if err != nil {
		t.Fatal(err)
	}
	v, err := eventschema.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t)
	ctx := context.Background()
	st := f.mkUser(t, "s@x.kz", domain.RoleStudent)
	g, _ := f.svc.CreateGroup(ctx, f.adminActor(), CreateGroupInput{Name: "SE-2424", CourseID: "c_10"})
	_, _ = f.svc.AddMember(ctx, f.adminActor(), g.ID, st.ID)
	s, _ := f.svc.Login(ctx, "s@x.kz", pw, "")
	s2, _ := f.svc.Refresh(ctx, s.RefreshToken, "")
	_, _ = f.svc.Refresh(ctx, s.RefreshToken, "") // reuse
	_ = f.svc.Logout(ctx, s2.RefreshToken, "")
	_ = f.svc.RemoveMember(ctx, f.adminActor(), g.ID, st.ID)
	_, _ = f.svc.UpdateUser(ctx, f.adminActor(), st.ID, UpdateUserInput{Status: ptr(domain.StatusDeactivated)})
	seen := map[string]bool{}
	for _, p := range f.store.Out {
		b, _ := json.Marshal(p.Env)
		if err := v.Validate(b); err != nil {
			t.Errorf("%s does not match schema: %v\n%s", p.Env.EventType, err, b)
		}
		if p.Topic != events.TopicUsers {
			t.Errorf("topic %s", p.Topic)
		}
		seen[p.Env.EventType] = true
	}
	for _, typ := range []string{events.TypeUserCreated, events.TypeUserLoggedIn, events.TypeUserLoggedOut, events.TypeGroupUpdated} {
		if !seen[typ] {
			t.Errorf("no %s produced", typ)
		}
	}
	// Payloads never carry e-mails or names of users (NFR-9).
	for _, p := range f.store.Out {
		if strings.Contains(string(p.Env.Payload), "@") {
			t.Errorf("%s payload contains an e-mail: %s", p.Env.EventType, p.Env.Payload)
		}
	}
}

func ptr[T any](v T) *T { return &v }
