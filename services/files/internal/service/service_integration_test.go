//go:build integration

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/testpg"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUploadsInPostgresAndMinIO(t *testing.T) {
	if os.Getenv("MINIO_ENDPOINT") == "" {
		t.Skip("set private test MINIO_ENDPOINT, MINIO_PUBLIC_ENDPOINT, MINIO_ACCESS_KEY and MINIO_SECRET_KEY")
	}
	db, err := pgxpool.New(context.Background(), testpg.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err = migrate.Up(context.Background(), db, logging.Discard(), Migrations()...); err != nil {
		t.Fatal(err)
	}
	const token = "private-test-only"
	svc, err := New(db, token)
	if err != nil {
		t.Fatal(err)
	}
	owner := authn.Principal{UserID: ids.NewV7(), GroupID: ids.NewV7(), Role: authn.RoleStudent}
	engine := gin.New()
	svc.Register(engine, func(c *gin.Context) {
		p := owner
		p.UserID = c.GetHeader("Test-User")
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), p))
		c.Next()
	})
	call := func(method, path, who string, body []byte) (int, []byte) {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Test-User", who)
		if who == token {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res.Code, res.Body.Bytes()
	}
	data := []byte("student work\n")
	code, b := call("POST", "/v1/uploads", owner.UserID, []byte(fmt.Sprintf(`{"name":"essay.txt","mime":"text/plain","size":%d}`, len(data))))
	if code != 201 {
		t.Fatal(code, string(b))
	}
	var grant struct {
		ID      string            `json:"file_id"`
		URL     string            `json:"url"`
		Fields  map[string]string `json:"fields"`
		Expires time.Time         `json:"expires_at"`
	}
	if err = json.Unmarshal(b, &grant); err != nil || grant.ID == "" || time.Until(grant.Expires) > 5*time.Minute {
		t.Fatal("invalid grant", err)
	}
	post := func(payload []byte) int {
		var form bytes.Buffer
		m := multipart.NewWriter(&form)
		for name, value := range grant.Fields {
			if err := m.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
		part, err := m.CreateFormFile("file", "essay.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err = m.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := http.Post(grant.URL, m.FormDataContentType(), &form)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		io.Copy(io.Discard, r.Body)
		return r.StatusCode
	}
	if status := post(append(append([]byte{}, data...), 0)); status < 400 {
		t.Fatalf("oversize accepted %d", status)
	}
	if status := post(data); status >= 300 {
		t.Fatalf("upload rejected %d", status)
	}
	complete := "/v1/uploads/" + grant.ID + "/complete"
	if status, _ := call("POST", complete, ids.NewV7(), nil); status != 404 {
		t.Fatal("wrong owner", status)
	}
	for i := 0; i < 2; i++ {
		if status, body := call("POST", complete, owner.UserID, nil); status != 200 {
			t.Fatalf("complete %d %s", status, body)
		}
	}
	var count int
	if err = db.QueryRow(context.Background(), "SELECT count(*) FROM outbox WHERE event_type='file.uploaded'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if status := post(bytes.Repeat([]byte("x"), len(data))); status >= 300 {
		t.Fatal(status)
	}
	if status, body := call("GET", "/v1/files/"+grant.ID, owner.UserID, nil); status != 200 || !bytes.Equal(body, data) {
		t.Fatal("staging replay changed published bytes", status)
	}
	internal := "/internal/v1/submissions/" + ids.NewV7() + "/text"
	if status, _ := call("PUT", internal, "unauthorized", data); status != 403 {
		t.Fatal(status)
	}
	for _, test := range []struct {
		content []byte
		status  int
	}{{data, 204}, {data, 204}, {[]byte("other"), 409}} {
		if got, _ := call("PUT", internal, token, test.content); got != test.status {
			t.Fatalf("immutable write %d want %d", got, test.status)
		}
	}
}
