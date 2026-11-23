// Package http exposes the auth use cases over HTTP (Gin).
package http

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ratelimit"
)

// CookieName is the refresh-token cookie.
const CookieName = "examory_refresh"

// CookiePath restricts the refresh cookie to the auth endpoints.
const CookiePath = "/v1/auth"

// Config configures the router.
type Config struct {
	Engine        httpx.EngineConfig
	CookieSecure  bool
	RefreshTTL    time.Duration
	AllowOrigins  []string // also allowed for the CSRF Origin check
	JWKS          authn.JWKSet
	Limiter       ratelimit.Limiter
	LoginPerIP    int           // login attempts per IP per LoginWindow (20)
	LoginWindow   time.Duration // (5m)
	APIPerIP      int           // requests per IP per minute (300)
	APIPerSubject int           // requests per user per minute (600)
	Checks        map[string]httpx.CheckFunc
}

// Handler holds the HTTP dependencies.
type Handler struct {
	svc *app.Service
	cfg Config
	log *slog.Logger
}

// NewRouter builds the Gin engine with all routes.
func NewRouter(svc *app.Service, verifier *authn.Verifier, cfg Config) (*gin.Engine, error) {
	if cfg.LoginPerIP <= 0 {
		cfg.LoginPerIP = 20
	}
	if cfg.LoginWindow <= 0 {
		cfg.LoginWindow = 5 * time.Minute
	}
	if cfg.APIPerIP <= 0 {
		cfg.APIPerIP = 300
	}
	if cfg.APIPerSubject <= 0 {
		cfg.APIPerSubject = 600
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	cfg.Engine.CORSOrigins = cfg.AllowOrigins
	cfg.Engine.CORSCredentials = true
	e, err := httpx.NewEngine(cfg.Engine)
	if err != nil {
		return nil, err
	}
	h := &Handler{svc: svc, cfg: cfg, log: cfg.Engine.Logger}
	lg := cfg.Engine.Logger
	httpx.RegisterHealth(e, lg, cfg.Checks)

	e.GET("/.well-known/jwks.json", h.jwks)

	perIP := httpx.RateLimit(cfg.Limiter, lg, "api_ip", cfg.APIPerIP, time.Minute, httpx.ByIP)
	perSub := httpx.RateLimit(cfg.Limiter, lg, "api_sub", cfg.APIPerSubject, time.Minute, httpx.BySubject)

	a := e.Group("/v1/auth", perIP, h.sameOrigin)
	a.POST("/login", httpx.RateLimit(cfg.Limiter, lg, "login_ip", cfg.LoginPerIP, cfg.LoginWindow, httpx.ByIP), h.login)
	a.POST("/refresh", h.refresh)
	a.POST("/logout", h.logout)

	v1 := e.Group("/v1", perIP, httpx.Authenticate(verifier), perSub)
	v1.GET("/me", h.me)
	admin := v1.Group("", httpx.RequireRoles(authn.RoleAdmin))
	admin.POST("/users", h.createUser)
	admin.GET("/users", h.listUsers)
	admin.GET("/users/:id", h.getUser)
	admin.PATCH("/users/:id", h.updateUser)
	admin.POST("/groups", h.createGroup)
	admin.GET("/groups", h.listGroups)
	admin.GET("/groups/:id", h.getGroup)
	admin.PUT("/groups/:id/members/:userId", h.addMember)
	admin.DELETE("/groups/:id/members/:userId", h.removeMember)
	return e, nil
}

// sameOrigin is CSRF defence in depth for cookie endpoints: a browser
// request carrying an Origin header must come from an allow-listed origin
// or from the same host as the request. The SameSite=Strict cookie is the
// primary defence.
func (h *Handler) sameOrigin(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin == "" || slices.Contains(h.cfg.AllowOrigins, origin) {
		c.Next()
		return
	}
	u, err := url.Parse(origin)
	reqHost := c.Request.Host
	if hh, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = hh
	}
	if err == nil && u.Hostname() != "" && strings.EqualFold(u.Hostname(), reqHost) {
		c.Next()
		return
	}
	httpx.Fail(c, http.StatusForbidden, "origin_forbidden", "cross-origin request rejected")
}

func clientIP(c *gin.Context) string {
	if ip := net.ParseIP(c.ClientIP()); ip != nil {
		return ip.String()
	}
	return ""
}

func actor(c *gin.Context) app.Actor {
	p, _ := httpx.Principal(c)
	return app.Actor{UserID: p.UserID, Role: domain.Role(p.Role), IP: clientIP(c)}
}

// fail maps domain errors to problem responses; anything else is a 500.
func (h *Handler) fail(c *gin.Context, err error) {
	var ve *domain.ValidationError
	var le *domain.LockedError
	switch {
	case errors.As(err, &ve):
		fe := make([]httpx.FieldError, len(ve.Fields))
		for i, f := range ve.Fields {
			fe[i] = httpx.FieldError{Field: f.Field, Message: f.Message}
		}
		httpx.Invalid(c, "request validation failed", fe)
	case errors.As(err, &le):
		secs := int(time.Until(le.Until).Seconds()) + 1
		if secs < 1 {
			secs = 1
		}
		c.Header("Retry-After", strconv.Itoa(secs))
		httpx.Fail(c, http.StatusTooManyRequests, "account_locked", "too many failed attempts; try again later")
	case errors.Is(err, domain.ErrInvalidCredentials):
		httpx.Fail(c, http.StatusUnauthorized, "invalid_credentials", "invalid e-mail or password")
	case errors.Is(err, domain.ErrInvalidSession):
		h.clearCookie(c)
		httpx.Fail(c, http.StatusUnauthorized, "invalid_session", "session is invalid or expired")
	case errors.Is(err, domain.ErrNotFound):
		httpx.Fail(c, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrEmailTaken):
		httpx.Fail(c, http.StatusConflict, "email_taken", "e-mail is already registered")
	case errors.Is(err, domain.ErrGroupNameTaken):
		httpx.Fail(c, http.StatusConflict, "group_name_taken", "a group with this name exists")
	case errors.Is(err, domain.ErrSelfModification):
		httpx.Fail(c, http.StatusConflict, "self_modification", err.Error())
	case errors.Is(err, domain.ErrLastAdmin):
		httpx.Fail(c, http.StatusConflict, "last_admin", err.Error())
	case errors.Is(err, domain.ErrNotStudent):
		httpx.Fail(c, http.StatusConflict, "not_student", err.Error())
	case errors.Is(err, domain.ErrNotMember):
		httpx.Fail(c, http.StatusNotFound, "not_member", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		httpx.Fail(c, http.StatusForbidden, "forbidden", "this action is not allowed for your role")
	default:
		httpx.Internal(c, h.log, err)
	}
}

func (h *Handler) setCookie(c *gin.Context, token string) {
	// #nosec G124 -- HttpOnly and SameSite=Strict are fixed; Secure comes from
	// AUTH_COOKIE_SECURE (default true; false only for plain-HTTP local dev).
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieName, Value: token, Path: CookiePath, MaxAge: int(h.cfg.RefreshTTL.Seconds()),
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearCookie(c *gin.Context) {
	// #nosec G124 -- same attributes as setCookie; see there.
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieName, Value: "", Path: CookiePath, MaxAge: -1,
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func refreshCookie(c *gin.Context) string {
	v, err := c.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return v
}

// --- DTOs ---

type userDTO struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	GroupID     *string   `json:"group_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUser(u domain.User) userDTO {
	var gid *string
	if u.GroupID != "" {
		g := u.GroupID
		gid = &g
	}
	return userDTO{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: string(u.Role), Status: string(u.Status), GroupID: gid, CreatedAt: u.CreatedAt.UTC()}
}

type groupDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CourseID    *string   `json:"course_id"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	Members     []userDTO `json:"members,omitempty"`
}

func toGroup(g domain.Group) groupDTO {
	var cid *string
	if g.CourseID != "" {
		x := g.CourseID
		cid = &x
	}
	return groupDTO{ID: g.ID, Name: g.Name, CourseID: cid, MemberCount: g.MemberCount, CreatedAt: g.CreatedAt.UTC()}
}

type sessionDTO struct {
	AccessToken string  `json:"access_token"`
	TokenType   string  `json:"token_type"`
	ExpiresIn   int     `json:"expires_in"`
	User        userDTO `json:"user"`
}

func (h *Handler) writeSession(c *gin.Context, s *app.Session) {
	h.setCookie(c, s.RefreshToken)
	exp := int(time.Until(s.ExpiresAt).Seconds())
	if exp < 0 {
		exp = 0
	}
	c.JSON(http.StatusOK, sessionDTO{AccessToken: s.AccessToken, TokenType: "Bearer", ExpiresIn: exp, User: toUser(s.User)})
}

const maxAuthBody = 4 << 10

// --- handlers ---

func (h *Handler) jwks(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, h.cfg.JWKS)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginReq
	if !httpx.BindJSON(c, &req, maxAuthBody) {
		return
	}
	s, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, clientIP(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	h.writeSession(c, s)
}

func (h *Handler) refresh(c *gin.Context) {
	s, err := h.svc.Refresh(c.Request.Context(), refreshCookie(c), clientIP(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	h.writeSession(c, s)
}

func (h *Handler) logout(c *gin.Context) {
	if err := h.svc.Logout(c.Request.Context(), refreshCookie(c), clientIP(c)); err != nil {
		h.fail(c, err)
		return
	}
	h.clearCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	p, _ := httpx.Principal(c)
	u, err := h.svc.Me(c.Request.Context(), p.UserID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUser(*u))
}

type createUserReq struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

func (h *Handler) createUser(c *gin.Context) {
	var req createUserReq
	if !httpx.BindJSON(c, &req, maxAuthBody) {
		return
	}
	u, err := h.svc.CreateUser(c.Request.Context(), actor(c), app.CreateUserInput{
		Email: req.Email, DisplayName: req.DisplayName, Role: domain.Role(req.Role), Password: req.Password})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toUser(*u))
}

type page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func (h *Handler) listUsers(c *gin.Context) {
	limit, cursor, ok := httpx.Page(c, 50)
	if !ok {
		return
	}
	us, err := h.svc.ListUsers(c.Request.Context(), actor(c), limit, cursor)
	if err != nil {
		h.fail(c, err)
		return
	}
	out := page[userDTO]{Items: make([]userDTO, len(us))}
	for i, u := range us {
		out.Items[i] = toUser(u)
	}
	if len(us) == limit {
		last := us[len(us)-1].ID
		out.NextCursor = &last
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) getUser(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	u, err := h.svc.GetUser(c.Request.Context(), actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUser(*u))
}

type updateUserReq struct {
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

func (h *Handler) updateUser(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req updateUserReq
	if !httpx.BindJSON(c, &req, maxAuthBody) {
		return
	}
	var in app.UpdateUserInput
	if req.Role != nil {
		r := domain.Role(*req.Role)
		in.Role = &r
	}
	if req.Status != nil {
		s := domain.Status(*req.Status)
		in.Status = &s
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), actor(c), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toUser(*u))
}

type createGroupReq struct {
	Name     string  `json:"name"`
	CourseID *string `json:"course_id"`
}

func (h *Handler) createGroup(c *gin.Context) {
	var req createGroupReq
	if !httpx.BindJSON(c, &req, maxAuthBody) {
		return
	}
	in := app.CreateGroupInput{Name: req.Name}
	if req.CourseID != nil {
		in.CourseID = *req.CourseID
		if in.CourseID == "" {
			httpx.Invalid(c, "request validation failed", []httpx.FieldError{{Field: "course_id", Message: "must not be empty; omit or use null"}})
			return
		}
	}
	g, err := h.svc.CreateGroup(c.Request.Context(), actor(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toGroup(*g))
}

func (h *Handler) listGroups(c *gin.Context) {
	limit, cursor, ok := httpx.Page(c, 50)
	if !ok {
		return
	}
	gs, err := h.svc.ListGroups(c.Request.Context(), actor(c), limit, cursor)
	if err != nil {
		h.fail(c, err)
		return
	}
	out := page[groupDTO]{Items: make([]groupDTO, len(gs))}
	for i, g := range gs {
		out.Items[i] = toGroup(g)
	}
	if len(gs) == limit {
		last := gs[len(gs)-1].ID
		out.NextCursor = &last
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) getGroup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	g, err := h.svc.GetGroup(c.Request.Context(), actor(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	dto := toGroup(g.Group)
	dto.Members = make([]userDTO, len(g.Members))
	for i, m := range g.Members {
		dto.Members[i] = toUser(m)
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handler) addMember(c *gin.Context) {
	gid, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	uid, ok := httpx.PathID(c, "userId")
	if !ok {
		return
	}
	g, err := h.svc.AddMember(c.Request.Context(), actor(c), gid, uid)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toGroup(*g))
}

func (h *Handler) removeMember(c *gin.Context) {
	gid, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	uid, ok := httpx.PathID(c, "userId")
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(c.Request.Context(), actor(c), gid, uid); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
