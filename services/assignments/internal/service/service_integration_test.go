//go:build integration

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/submissionfile"
	"github.com/Newterios/Examory/services/platform/testpg"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReceiptAndOutboxInPostgres(t *testing.T) {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, testpg.New(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err = migrate.Up(ctx, db, logging.Discard(), Migrations()...); err != nil {
		t.Fatal(err)
	}
	teacher, student, group, course, fileID := ids.NewV7(), ids.NewV7(), ids.NewV7(), ids.NewV7(), ids.NewV7()
	other := ids.NewV7()
	const token = "in-process-test-token"
	const work = "қайырлы күн\n"
	hash := sha256.Sum256([]byte(work))
	files := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !submissionfile.Internal(r, token) {
			w.WriteHeader(403)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/internal/v1/files/"+fileID && r.URL.Query().Get("owner_id") == student {
			json.NewEncoder(w).Encode(submissionfile.File{Ref: submissionfile.Ref{ID: fileID, Name: "essay.txt", Size: int64(len(work)), SHA256: hex.EncodeToString(hash[:])}, MIME: "text/plain", Text: work})
			return
		}
		if r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/text") {
			b, _ := io.ReadAll(r.Body)
			if string(b) != work {
				t.Error("extraction changed bytes")
			}
			w.WriteHeader(204)
			return
		}
		w.WriteHeader(404)
	}))
	defer files.Close()
	check := courseaccess.CheckFunc(func(_ context.Context, c, tch, g string) (bool, error) {
		return c == course && g == group && (tch == teacher || tch == ""), nil
	})
	svc := &Service{db, check, files.URL, token, files.Client()}
	engine := gin.New()
	svc.Register(engine, func(c *gin.Context) {
		who := c.GetHeader("Test-User")
		p := authn.Principal{UserID: who, GroupID: group, Role: authn.RoleStudent}
		if who == teacher {
			p.Role = authn.RoleTeacher
		}
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), p))
		c.Next()
	})
	call := func(method, path, who, key string, body []byte) (int, []byte) {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Test-User", who)
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w.Code, w.Body.Bytes()
	}
	spec := Spec{Course: course, Group: group, Title: "Review", Instructions: "Upload a text", Deadline: time.Now().Add(-time.Hour), MaxAttempts: 1, MaxBytes: 200, Accepted: []string{"text/plain"}, Kind: "text", Language: "kk"}
	b, _ := json.Marshal(spec)
	status, body := call("POST", "/v1/assignments", teacher, "", b)
	if status != 201 {
		t.Fatal(status, string(body))
	}
	var assignment Assignment
	if err = json.Unmarshal(body, &assignment); err != nil {
		t.Fatal(err)
	}
	path := "/v1/assignments/" + assignment.ID + "/submissions"
	b = []byte(fmt.Sprintf(`{"file_ids":["%s"]}`, fileID))
	if status, _ := call("POST", path, other, "another-owned", b); status != 422 {
		t.Fatal("unowned file", status)
	}
	var wg sync.WaitGroup
	statuses := make([]int, 4)
	receipts := make([]Receipt, len(statuses))
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, data := call("POST", path, student, "retry-key-001", b)
			statuses[i] = code
			if err := json.Unmarshal(data, &receipts[i]); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	var created int
	for i, s := range statuses {
		if s == 201 {
			created++
		} else if s != 200 {
			t.Fatal(statuses)
		}
		if receipts[i].ID != receipts[0].ID || !receipts[i].Late || receipts[i].Attempt != 1 {
			t.Fatal(receipts)
		}
	}
	if created != 1 {
		t.Fatal(statuses)
	}
	if s, _ := call("POST", path, student, "new-attempt-02", b); s != 409 {
		t.Fatal("attempt limit", s)
	}
	if s, _ := call("POST", path, student, "retry-key-001", []byte(fmt.Sprintf(`{"file_ids":["%s"]}`, ids.NewV7()))); s != 409 {
		t.Fatal("changed retry", s)
	}
	if status, body := call("GET", path, other, "", nil); status != 200 || strings.Contains(string(body), fileID) {
		t.Fatal(status, string(body))
	}
	validator, err := eventschema.Load("../../../../docs/03_architecture/schemas")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	var raw []byte
	if err = db.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE event_type='submission.created'").Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = db.QueryRow(ctx, "SELECT envelope FROM outbox WHERE event_type='submission.created'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = validator.Validate(raw); err != nil || bytes.Contains(raw, []byte(work)) || !bytes.Contains(raw, []byte("minio://submissions/"+receipts[0].ID+"/extracted.txt")) {
		t.Fatal("invalid submission event", err)
	}
}
