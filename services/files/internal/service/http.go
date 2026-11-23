package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/submissionfile"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const bucket = "submissions"

type Service struct {
	db         *pgxpool.Pool
	s3, public *minio.Client
	token      string
}

func client(endpoint, key, secret string) (*minio.Client, error) {
	if !strings.Contains(endpoint, "://") {
		scheme := "http"
		if os.Getenv("MINIO_SECURE") == "true" {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || strings.Trim(u.Path, "/") != "" {
		return nil, errors.New("invalid MinIO endpoint")
	}
	return minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(key, secret, ""), Region: "us-east-1", Secure: u.Scheme == "https", BucketLookup: minio.BucketLookupPath})
}
func New(db *pgxpool.Pool, token string) (*Service, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	pub := os.Getenv("MINIO_PUBLIC_ENDPOINT")
	key, secret := os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY")
	if token == "" || pub == "" || key == "" || secret == "" {
		return nil, errors.New("files requires internal token, S3 credentials and public endpoint")
	}
	a, err := client(endpoint, key, secret)
	if err != nil {
		return nil, err
	}
	b, err := client(pub, key, secret)
	if err != nil {
		return nil, err
	}
	return &Service{db, a, b, token}, nil
}
func (s *Service) Register(e *gin.Engine, auth gin.HandlerFunc) {
	v := e.Group("/v1", auth, httpx.RequireRoles(authn.RoleStudent))
	v.POST("/uploads", s.begin)
	v.POST("/uploads/:id/complete", s.complete)
	v.GET("/files/:id", s.download)
	internal := e.Group("/internal/v1", func(c *gin.Context) {
		if !submissionfile.Internal(c.Request, s.token) {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	})
	internal.GET("/files/:id", s.describe)
	internal.PUT("/submissions/:id/text", s.extract)
}

type request struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Size int64  `json:"size"`
}

func (s *Service) begin(c *gin.Context) {
	p, _ := httpx.Principal(c)
	var in request
	if !httpx.BindJSON(c, &in, 4096) {
		return
	}
	if !submissionfile.ValidName(in.Name, in.MIME) || in.Size < 1 || in.Size > submissionfile.MaxBytes {
		httpx.Fail(c, 422, "upload_rejected", "invalid name, type or size")
		return
	}
	id := ids.NewV7()
	expiry := time.Now().UTC().Add(5 * time.Minute)
	policy := minio.NewPostPolicy()
	for _, err := range []error{policy.SetBucket(bucket), policy.SetKey("uploads/" + id), policy.SetContentType(in.MIME), policy.SetContentLengthRange(in.Size, in.Size), policy.SetExpires(expiry)} {
		if err != nil {
			c.Status(500)
			return
		}
	}
	ctx := c.Request.Context()
	link, fields, err := s.public.PresignedPostPolicy(ctx, policy)
	if err != nil {
		c.Status(503)
		return
	}
	_, err = s.db.Exec(ctx, "INSERT INTO uploads(id,owner_id,group_id,name,mime,size,expires_at) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7)", id, p.UserID, p.GroupID, in.Name, in.MIME, in.Size, expiry)
	if err != nil {
		c.Status(503)
		return
	}
	c.JSON(201, gin.H{"file_id": id, "method": "POST", "url": link.String(), "fields": fields, "expires_at": expiry})
}

type row struct {
	submissionfile.Ref
	MIME, Group string
	Expiry      time.Time
}

func fetch(r pgx.Row) (row, error) {
	var i row
	err := r.Scan(&i.ID, &i.Name, &i.Size, &i.SHA256, &i.MIME, &i.Group, &i.Expiry)
	return i, err
}

const lookup = "SELECT id::text,name,size,COALESCE(sha256,''),mime,COALESCE(group_id::text,''),expires_at FROM uploads WHERE id=$1 AND owner_id=$2"

func failRead(c *gin.Context, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else {
		c.Status(503)
	}
}
func (s *Service) read(ctx context.Context, key string, limit int64) ([]byte, error) {
	obj, err := s.s3.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	b, err := io.ReadAll(io.LimitReader(obj, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errContent
	}
	return b, nil
}

var errContent = errors.New("invalid object content")

func digest(data []byte) string { d := sha256.Sum256(data); return hex.EncodeToString(d[:]) }
func (s *Service) putOnce(ctx context.Context, key string, data []byte, mime string) error {
	opts := minio.PutObjectOptions{ContentType: mime, DisableMultipart: true}
	opts.SetMatchETagExcept("*")
	_, err := s.s3.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), opts)
	if err == nil {
		return nil
	}
	if e := minio.ToErrorResponse(err); e.Code != "PreconditionFailed" && e.StatusCode != 412 {
		return err
	}
	prior, err := s.read(ctx, key, int64(len(data)))
	if err != nil {
		return err
	}
	if !bytes.Equal(prior, data) {
		return errContent
	}
	return nil
}
func (s *Service) complete(c *gin.Context) {
	id := c.Param("id")
	if !ids.Valid(id) {
		c.Status(400)
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
	i, err := fetch(tx.QueryRow(ctx, lookup+" FOR UPDATE", id, p.UserID))
	if err != nil {
		failRead(c, err)
		return
	}
	if i.SHA256 != "" {
		c.JSON(200, i.Ref)
		return
	}
	if !time.Now().Before(i.Expiry) {
		httpx.Fail(c, 410, "upload_expired", "request a new upload")
		return
	}
	b, err := s.read(ctx, "uploads/"+id, i.Size)
	if err != nil {
		if e := minio.ToErrorResponse(err); e.Code == "NoSuchKey" || errors.Is(err, errContent) {
			httpx.Fail(c, 422, "upload_incomplete", "no file matching the grant was uploaded")
		} else {
			c.Status(503)
		}
		return
	}
	if int64(len(b)) != i.Size || !submissionfile.ValidText(b) {
		httpx.Fail(c, 422, "upload_rejected", "expected UTF-8 text of the declared size")
		return
	}
	i.SHA256 = digest(b)
	if err = s.putOnce(ctx, "blobs/"+id+"/"+i.SHA256, b, i.MIME); err != nil {
		c.Status(503)
		return
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, "UPDATE uploads SET sha256=$2,completed_at=$3 WHERE id=$1", id, i.SHA256, now)
	if err == nil {
		var env events.Envelope
		env, err = events.New("file.uploaded", 1, "files", events.Actor{UserID: p.UserID, Role: "student"}, events.Context{GroupID: events.Ptr(i.Group)}, i.Ref, now)
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
	c.JSON(200, i.Ref)
}
func (s *Service) get(c *gin.Context, owner string, internal bool) {
	id := c.Param("id")
	if !ids.Valid(id) || !ids.Valid(owner) {
		c.Status(400)
		return
	}
	i, err := fetch(s.db.QueryRow(c.Request.Context(), lookup+" AND sha256 IS NOT NULL", id, owner))
	if err != nil {
		failRead(c, err)
		return
	}
	b, err := s.read(c.Request.Context(), "blobs/"+id+"/"+i.SHA256, i.Size)
	if err != nil || digest(b) != i.SHA256 {
		c.Status(503)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if internal {
		c.JSON(200, submissionfile.File{Ref: i.Ref, MIME: i.MIME, Text: string(b)})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(i.Name)))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, i.MIME, b)
}
func (s *Service) download(c *gin.Context) { p, _ := httpx.Principal(c); s.get(c, p.UserID, false) }
func (s *Service) describe(c *gin.Context) { s.get(c, c.Query("owner_id"), true) }
func (s *Service) extract(c *gin.Context) {
	id := c.Param("id")
	if !ids.Valid(id) {
		c.Status(400)
		return
	}
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, submissionfile.MaxBytes+1))
	if err != nil || len(b) > submissionfile.MaxBytes || !submissionfile.ValidText(b) {
		c.Status(422)
		return
	}
	if err = s.putOnce(c.Request.Context(), id+"/extracted.txt", b, "text/plain; charset=utf-8"); err != nil {
		if errors.Is(err, errContent) {
			c.Status(http.StatusConflict)
		} else {
			c.Status(503)
		}
		return
	}
	c.Status(http.StatusNoContent)
}
