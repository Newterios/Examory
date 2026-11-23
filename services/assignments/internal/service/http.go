package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/submissionfile"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Spec struct {
	Course       string    `json:"course_id"`
	Group        string    `json:"group_id"`
	Title        string    `json:"title"`
	Instructions string    `json:"instructions"`
	Deadline     time.Time `json:"deadline_at"`
	MaxAttempts  int       `json:"max_attempts"`
	MaxBytes     int       `json:"max_bytes"`
	Accepted     []string  `json:"accepted_types"`
	Kind         string    `json:"kind"`
	Language     string    `json:"language"`
}
type Assignment struct {
	ID string `json:"id"`
	Spec
}
type Receipt struct {
	ID         string               `json:"submission_id"`
	Assignment string               `json:"assignment_id"`
	Student    string               `json:"student_id"`
	Attempt    int                  `json:"attempt"`
	At         time.Time            `json:"submitted_at"`
	Late       bool                 `json:"late"`
	Files      []submissionfile.Ref `json:"files"`
}
type Service struct {
	db          *pgxpool.Pool
	access      courseaccess.Checker
	base, token string
	http        *http.Client
}

func New(db *pgxpool.Pool, token string) (*Service, error) {
	base := os.Getenv("FILES_INTERNAL_URL")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || token == "" {
		return nil, errors.New("files internal endpoint/token required")
	}
	return &Service{db, courseaccess.FromEnv(), strings.TrimRight(base, "/"), token, &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (s *Service) Register(e *gin.Engine, auth gin.HandlerFunc) {
	v := e.Group("/v1", auth)
	v.POST("/assignments", httpx.RequireRoles(authn.RoleTeacher), s.create)
	v.GET("/assignments", httpx.RequireRoles(authn.RoleTeacher, authn.RoleStudent), s.list)
	v.GET("/assignments/:id", httpx.RequireRoles(authn.RoleTeacher, authn.RoleStudent), s.get)
	v.POST("/assignments/:id/submissions", httpx.RequireRoles(authn.RoleStudent), s.submit)
	v.GET("/assignments/:id/submissions", httpx.RequireRoles(authn.RoleTeacher, authn.RoleStudent), s.receipts)
}

var language = regexp.MustCompile(`^[a-zA-Z0-9_+#.-]{1,32}$`)
var idem = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)

func (spec Spec) valid() bool {
	if !ids.Valid(spec.Course) || !ids.Valid(spec.Group) || strings.TrimSpace(spec.Title) == "" || utf8.RuneCountInString(spec.Title) > 200 || strings.TrimSpace(spec.Instructions) == "" || utf8.RuneCountInString(spec.Instructions) > 8000 || spec.Deadline.IsZero() || spec.MaxAttempts < 1 || spec.MaxAttempts > 100 || spec.MaxBytes < 1 || spec.MaxBytes > submissionfile.MaxBytes || len(spec.Accepted) < 1 || len(spec.Accepted) > 8 || !slices.Contains([]string{"text", "file", "code"}, spec.Kind) || !language.MatchString(spec.Language) {
		return false
	}
	seen := map[string]bool{}
	for _, v := range spec.Accepted {
		if seen[v] || !submissionfile.Supported(v) {
			return false
		}
		seen[v] = true
	}
	return true
}
func (s *Service) allowed(ctx context.Context, p authn.Principal, course, group string) (bool, error) {
	teacher := ""
	if p.Role == authn.RoleTeacher {
		teacher = p.UserID
	} else if p.Role != authn.RoleStudent || p.GroupID != group {
		return false, nil
	}
	return s.access.Allowed(ctx, course, teacher, group)
}
func (s *Service) authorize(c *gin.Context, spec Spec) bool {
	p, _ := httpx.Principal(c)
	allowed, err := s.allowed(c.Request.Context(), p, spec.Course, spec.Group)
	if err != nil {
		c.Status(503)
		return false
	}
	if !allowed {
		c.Status(404)
		return false
	}
	return true
}
func (s *Service) create(c *gin.Context) {
	var in Spec
	if !httpx.BindJSON(c, &in, 32<<10) {
		return
	}
	if !in.valid() {
		httpx.Fail(c, 422, "invalid_assignment", "invalid course, group, instructions, deadline, attempts or file limits")
		return
	}
	if !s.authorize(c, in) {
		return
	}
	p, _ := httpx.Principal(c)
	ctx := c.Request.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		c.Status(503)
		return
	}
	defer tx.Rollback(ctx)
	at := time.Now().UTC().Truncate(time.Millisecond)
	in.Deadline = in.Deadline.UTC().Truncate(time.Millisecond)
	a := Assignment{ID: ids.NewV7(), Spec: in}
	_, err = tx.Exec(ctx, "INSERT INTO assignments(id,teacher_id,course_id,group_id,title,instructions,deadline,max_attempts,max_bytes,accepted_types,kind,language,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)", a.ID, p.UserID, in.Course, in.Group, in.Title, in.Instructions, in.Deadline, in.MaxAttempts, in.MaxBytes, in.Accepted, in.Kind, in.Language, at)
	if err == nil {
		var env events.Envelope
		env, err = events.New("assignment.published", 1, "assignments", events.Actor{UserID: p.UserID, Role: "teacher"}, events.Context{CourseID: &in.Course, GroupID: &in.Group}, gin.H{"assignment_id": a.ID, "due_at": in.Deadline}, at)
		if err == nil {
			err = outbox.Enqueue(ctx, tx, "lms.courses", in.Course, env)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		c.Status(503)
		return
	}
	c.JSON(201, a)
}

const assignmentCols = "id::text,course_id::text,group_id::text,title,instructions,deadline,max_attempts,max_bytes,accepted_types,kind,language"

func scanAssignment(row pgx.Row) (Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.Course, &a.Group, &a.Title, &a.Instructions, &a.Deadline, &a.MaxAttempts, &a.MaxBytes, &a.Accepted, &a.Kind, &a.Language)
	return a, err
}
func (s *Service) load(c *gin.Context) (Assignment, bool) {
	id := c.Param("id")
	if !ids.Valid(id) {
		c.Status(400)
		return Assignment{}, false
	}
	a, err := scanAssignment(s.db.QueryRow(c.Request.Context(), "SELECT "+assignmentCols+" FROM assignments WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return a, false
	}
	if err != nil {
		c.Status(503)
		return a, false
	}
	return a, s.authorize(c, a.Spec)
}
func (s *Service) get(c *gin.Context) {
	if a, ok := s.load(c); ok {
		c.JSON(200, a)
	}
}
func (s *Service) list(c *gin.Context) {
	n, after, ok := httpx.Page(c, 20)
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	ctx := c.Request.Context()
	list, err := courseaccess.Visible(ctx, n, after, func(cur string, limit int) ([]Assignment, error) {
		rows, err := s.db.Query(ctx, "SELECT "+assignmentCols+" FROM assignments WHERE id>COALESCE(NULLIF($1,'')::uuid,'00000000-0000-0000-0000-000000000000'::uuid) AND ($2::boolean OR group_id=NULLIF($3,'')::uuid) ORDER BY id LIMIT $4", cur, p.Role == authn.RoleTeacher, p.GroupID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var batch []Assignment
		for rows.Next() {
			r, err := scanAssignment(rows)
			if err != nil {
				return nil, err
			}
			batch = append(batch, r)
		}
		return batch, rows.Err()
	}, func(a Assignment) (string, string, string) { return a.ID, a.Course, a.Group }, func(course, group string) (bool, error) { return s.allowed(ctx, p, course, group) })
	if err != nil {
		c.Status(503)
		return
	}
	if list == nil {
		list = []Assignment{}
	}
	var next *string
	if len(list) == n {
		next = &list[n-1].ID
	}
	c.JSON(200, gin.H{"items": list, "next_cursor": next})
}
func (s *Service) fileRequest(ctx context.Context, method, path string, b []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	if b != nil {
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	return s.http.Do(req)
}

var errFile = errors.New("invalid file references or submission text")

func (s *Service) texts(ctx context.Context, a Assignment, student string, files []string) ([]submissionfile.Ref, []byte, error) {
	var extracted bytes.Buffer
	refs := make([]submissionfile.Ref, 0, len(files))
	var total int64
	for i, id := range files {
		resp, err := s.fileRequest(ctx, "GET", "/internal/v1/files/"+id+"?owner_id="+url.QueryEscape(student), nil)
		if err != nil {
			return nil, nil, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 6*submissionfile.MaxBytes+4096))
		resp.Body.Close()
		if resp.StatusCode == 404 {
			return nil, nil, errFile
		}
		if readErr != nil || resp.StatusCode != 200 {
			return nil, nil, errors.New("files unavailable")
		}
		var f submissionfile.File
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, nil, err
		}
		content := []byte(f.Text)
		sum := sha256.Sum256(content)
		total += f.Size
		if f.ID != id || f.Size != int64(len(content)) || hex.EncodeToString(sum[:]) != f.SHA256 || !slices.Contains(a.Accepted, f.MIME) || !submissionfile.ValidText(content) || !submissionfile.ValidName(f.Name, f.MIME) || f.Size > int64(a.MaxBytes) || total+int64(i) > int64(a.MaxBytes) {
			return nil, nil, errFile
		}
		if i > 0 {
			extracted.WriteByte('\n')
		}
		extracted.Write(content)
		refs = append(refs, f.Ref)
	}
	return refs, extracted.Bytes(), nil
}

const receiptCols = "id::text,assignment_id::text,user_id::text,attempt,submitted_at,late,files"

func scanReceipt(row pgx.Row) (Receipt, error) {
	var r Receipt
	var raw []byte
	err := row.Scan(&r.ID, &r.Assignment, &r.Student, &r.Attempt, &r.At, &r.Late, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &r.Files)
	}
	return r, err
}
func stableID(group, user, key string) string {
	h := sha256.Sum256([]byte("examory-submission-v1\x00" + group + "\x00" + user + "\x00" + key))
	h[6] = (h[6] & 15) | 0x80
	h[8] = (h[8] & 63) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
func (s *Service) submit(c *gin.Context) {
	a, ok := s.load(c)
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	key := c.GetHeader("Idempotency-Key")
	var in struct {
		Files []string `json:"file_ids"`
	}
	if !httpx.BindJSON(c, &in, 4096) {
		return
	}
	if !idem.MatchString(key) || len(in.Files) < 1 || len(in.Files) > submissionfile.MaxFiles {
		httpx.Fail(c, 422, "invalid_submission", "valid Idempotency-Key and one to four files required")
		return
	}
	seen := map[string]bool{}
	for _, id := range in.Files {
		if !ids.Valid(id) || seen[id] {
			c.Status(422)
			return
		}
		seen[id] = true
	}
	ctx := c.Request.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		c.Status(503)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,31))", a.ID+":"+p.UserID); err != nil {
		c.Status(503)
		return
	}
	requestHash := sha256.Sum256([]byte(strings.Join(in.Files, "\n")))
	fingerprint := hex.EncodeToString(requestHash[:])
	id := stableID(a.ID, p.UserID, key)
	var old string
	err = tx.QueryRow(ctx, "SELECT request_hash FROM submissions WHERE id=$1", id).Scan(&old)
	if err == nil {
		if old != fingerprint {
			c.Status(409)
			return
		}
		receipt, err := scanReceipt(tx.QueryRow(ctx, "SELECT "+receiptCols+" FROM submissions WHERE id=$1", id))
		if err != nil {
			c.Status(503)
			return
		}
		c.Header("Idempotent-Replayed", "true")
		c.JSON(200, receipt)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		c.Status(503)
		return
	}
	var number int
	if err = tx.QueryRow(ctx, "SELECT count(*)+1 FROM submissions WHERE assignment_id=$1 AND user_id=$2", a.ID, p.UserID).Scan(&number); err != nil {
		c.Status(503)
		return
	}
	if number > a.MaxAttempts {
		httpx.Fail(c, 409, "attempt_limit", "attempt limit reached")
		return
	}
	refs, b, err := s.texts(ctx, a, p.UserID, in.Files)
	if err != nil {
		if errors.Is(err, errFile) {
			httpx.Fail(c, 422, "invalid_files", "file ownership, type, size or extracted text is invalid")
		} else {
			c.Status(503)
		}
		return
	}
	resp, err := s.fileRequest(ctx, "PUT", "/internal/v1/submissions/"+id+"/text", b)
	if err != nil {
		c.Status(503)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		if resp.StatusCode == 409 {
			c.Status(409)
		} else {
			c.Status(503)
		}
		return
	}
	at := time.Now().UTC().Truncate(time.Millisecond)
	receipt := Receipt{id, a.ID, p.UserID, number, at, at.After(a.Deadline), refs}
	ref := "minio://submissions/" + id + "/extracted.txt"
	raw, _ := json.Marshal(refs)
	_, err = tx.Exec(ctx, "INSERT INTO submissions(id,assignment_id,user_id,attempt,submitted_at,late,request_key,request_hash,files,text_ref) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", id, a.ID, p.UserID, number, at, receipt.Late, key, fingerprint, raw, ref)
	if err == nil {
		var env events.Envelope
		env, err = events.New("submission.created", 1, "assignments", events.Actor{UserID: p.UserID, Role: "student"}, events.Context{CourseID: &a.Course, GroupID: &a.Group}, gin.H{"submission_id": id, "assignment_id": a.ID, "attempt": number, "kind": a.Kind, "language": a.Language, "text_ref": ref, "files": refs, "submitted_at": at, "deadline_at": a.Deadline}, at)
		if err == nil {
			err = outbox.Enqueue(ctx, tx, "lms.submissions", p.UserID, env)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		c.Status(503)
		return
	}
	c.JSON(201, receipt)
}
func (s *Service) receipts(c *gin.Context) {
	a, ok := s.load(c)
	if !ok {
		return
	}
	n, after, ok := httpx.Page(c, 30)
	if !ok {
		return
	}
	p, _ := httpx.Principal(c)
	rows, err := s.db.Query(c.Request.Context(), "SELECT "+receiptCols+" FROM submissions WHERE assignment_id=$1 AND ($2 OR user_id=$3) AND id>COALESCE(NULLIF($4,'')::uuid,'00000000-0000-0000-0000-000000000000'::uuid) ORDER BY id LIMIT $5", a.ID, p.Role == authn.RoleTeacher, p.UserID, after, n)
	if err != nil {
		c.Status(503)
		return
	}
	defer rows.Close()
	out := []Receipt{}
	for rows.Next() {
		r, err := scanReceipt(rows)
		if err != nil {
			c.Status(503)
			return
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		c.Status(503)
		return
	}
	var next *string
	if len(out) == n {
		next = &out[n-1].ID
	}
	c.JSON(200, gin.H{"items": out, "next_cursor": next})
}
