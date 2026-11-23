package httpadapter

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"github.com/Newterios/Examory/services/courses/internal/app"
	"github.com/Newterios/Examory/services/courses/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
)

type Handler struct {
	Store  app.Store
	Logger *slog.Logger
}

func (h Handler) Register(e *gin.Engine, authenticate gin.HandlerFunc, internalToken string) {
	v := e.Group("/v1/courses", authenticate)
	v.GET("", h.list)
	v.GET("/:id", h.get)
	staff := v.Group("", httpx.RequireRoles(authn.RoleTeacher, authn.RoleAdmin))
	staff.POST("", h.create)
	staff.PUT("/:id", h.update)
	admin := v.Group("", httpx.RequireRoles(authn.RoleAdmin))
	for path, kind := range map[string]string{"teachers": "teacher", "groups": "group"} {
		admin.PUT("/:id/"+path+"/:member", h.assign(kind, true))
		admin.DELETE("/:id/"+path+"/:member", h.assign(kind, false))
	}
	v.POST("/:id/materials/:material/views", httpx.RequireRoles(authn.RoleStudent), h.view)
	internal := e.Group("/internal/v1", func(c *gin.Context) {
		got := sha256.Sum256([]byte(c.GetHeader("Authorization")))
		want := sha256.Sum256([]byte("Bearer " + internalToken))
		if internalToken == "" || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			httpx.Fail(c, 401, "unauthorized", "valid service credentials required")
			return
		}
		c.Next()
	})
	internal.GET("/access", h.access)
	internal.GET("/teacher-courses", h.teacherCourses)
}
func (h Handler) fail(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, domain.ErrInvalid):
		httpx.Fail(c, 400, "invalid_request", "invalid course data")
	case errors.Is(err, domain.ErrForbidden):
		httpx.Fail(c, 403, "forbidden", "role cannot perform this action")
	case errors.Is(err, domain.ErrNotFound):
		httpx.Fail(c, 404, "not_found", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		httpx.Fail(c, 409, "stale_revision", "fetch the current course before updating")
	default:
		httpx.Internal(c, h.Logger, err)
	}
	return true
}
func (h Handler) create(c *gin.Context) {
	var in struct {
		Title string `json:"title"`
	}
	if !httpx.BindJSON(c, &in, 4096) {
		return
	}
	p, _ := httpx.Principal(c)
	course, err := h.Store.Create(c.Request.Context(), p, in.Title)
	if !h.fail(c, err) {
		c.JSON(201, course)
	}
}
func (h Handler) list(c *gin.Context) {
	n, cur, ok := httpx.Page(c, 20)
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	rows, err := h.Store.List(c.Request.Context(), p, cur, n+1)
	if h.fail(c, err) {
		return
	}
	next := ""
	if len(rows) > n {
		next = rows[n-1].ID
		rows = rows[:n]
	}
	c.JSON(200, gin.H{"items": rows, "next_cursor": next})
}
func (h Handler) get(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	course, err := h.Store.Get(c.Request.Context(), p, id)
	if !h.fail(c, err) {
		c.JSON(200, course)
	}
}
func (h Handler) update(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var in domain.Course
	if !httpx.BindJSON(c, &in, 512<<10) {
		return
	}
	p, _ := httpx.Principal(c)
	course, err := h.Store.Update(c.Request.Context(), p, id, in)
	if !h.fail(c, err) {
		c.JSON(200, course)
	}
}
func (h Handler) assign(kind string, add bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, ok := httpx.PathID(c, "id")
		if !ok {
			return
		}
		id, ok := httpx.PathID(c, "member")
		if !ok {
			return
		}
		p, _ := httpx.Principal(c)
		if !h.fail(c, h.Store.Assign(c.Request.Context(), p, cid, kind, id, add)) {
			c.Status(http.StatusNoContent)
		}
	}
}
func (h Handler) view(c *gin.Context) {
	cid, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	mid, ok := httpx.PathID(c, "material")
	if !ok {
		return
	}
	var in struct {
		Duration  *int64 `json:"view_duration_ms"`
		Completed *bool  `json:"completed"`
	}
	if !httpx.BindJSON(c, &in, 1024) {
		return
	}
	if in.Duration == nil || in.Completed == nil {
		h.fail(c, domain.ErrInvalid)
		return
	}
	p, _ := httpx.Principal(c)
	completed, err := h.Store.View(c.Request.Context(), p, cid, mid, *in.Duration, *in.Completed)
	if !h.fail(c, err) {
		c.JSON(200, gin.H{"completed": completed})
	}
}
func (h Handler) access(c *gin.Context) {
	cid, t, g := c.Query("course_id"), c.Query("teacher_id"), c.Query("group_id")
	if !ids.Valid(cid) || (t == "") == (g == "") {
		h.fail(c, domain.ErrInvalid)
		return
	}
	kind, id := "teacher", t
	if g != "" {
		kind, id = "group", g
	}
	if !ids.Valid(id) {
		h.fail(c, domain.ErrInvalid)
		return
	}
	ok, err := h.Store.Access(c.Request.Context(), cid, kind, id)
	if !h.fail(c, err) {
		c.JSON(200, gin.H{"allowed": ok})
	}
}
func (h Handler) teacherCourses(c *gin.Context) {
	t := c.Query("teacher_id")
	if !ids.Valid(t) {
		h.fail(c, domain.ErrInvalid)
		return
	}
	courses, err := h.Store.TeacherCourses(c.Request.Context(), t)
	if !h.fail(c, err) {
		c.JSON(200, gin.H{"course_ids": courses})
	}
}
