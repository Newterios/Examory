package http

import (
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *handler) grade(c *gin.Context) {
	id := c.Param("attemptId")
	if !opaque(id) {
		httpx.Fail(c, 400, "invalid_attempt", "invalid attempt id")
		return
	}
	var in domain.GradeInput
	if !httpx.BindJSON(c, &in, 16<<10) {
		return
	}
	p, _ := httpx.Principal(c)
	g, replayed, err := h.d.Service.SetGrade(c.Request.Context(), p, id, c.GetHeader("Idempotency-Key"), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	if replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(http.StatusOK, g)
}
func (h *handler) readGrade(c *gin.Context) {
	id := c.Param("attemptId")
	if !opaque(id) {
		httpx.Fail(c, 400, "invalid_attempt", "invalid attempt id")
		return
	}
	p, _ := httpx.Principal(c)
	history := c.FullPath() == "/v1/attempts/:attemptId/grade/history"
	grades, err := h.d.Service.GetGrades(c.Request.Context(), p, id, history)
	if err != nil {
		h.fail(c, err)
		return
	}
	if history {
		c.JSON(200, gin.H{"items": grades})
		return
	}
	c.JSON(200, grades[0])
}
