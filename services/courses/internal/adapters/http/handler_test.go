package httpadapter

import (
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestMembershipCredentialsAndRoles(t *testing.T) {
	g := gin.New()
	Handler{Logger: logging.Discard()}.Register(g, func(c *gin.Context) {
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), authn.Principal{UserID: "student", Role: "student"}))
		c.Next()
	}, "internal-secret")
	for _, tc := range []struct {
		method, path, token string
		want                int
	}{{"GET", "/internal/v1/access", "", 401}, {"GET", "/internal/v1/teacher-courses", "Bearer wrong", 401}, {"GET", "/internal/v1/access", "Bearer internal-secret", 400}, {"POST", "/v1/courses", "", 403}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", tc.token)
		g.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(w.Code, tc.path, w.Body.String())
		}
	}
}
