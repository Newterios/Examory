// Package courseaccess checks current membership against the private courses service.
package courseaccess

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("course membership service unavailable")

type Checker interface {
	Allowed(ctx context.Context, course, teacher, group string) (bool, error)
}

type CheckFunc func(context.Context, string, string, string) (bool, error)

func (f CheckFunc) Allowed(ctx context.Context, course, teacher, group string) (bool, error) {
	return f(ctx, course, teacher, group)
}

type Client struct {
	base, token string
	http        *http.Client
}

func FromEnv() *Client {
	return New(os.Getenv("COURSES_INTERNAL_URL"), os.Getenv("COURSES_INTERNAL_TOKEN"))
}
func New(base, token string) *Client {
	return &Client{strings.TrimRight(base, "/"), token, &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Allowed requires every supplied relationship. Failures never become permission grants.
func (c *Client) Allowed(ctx context.Context, course, teacher, group string) (bool, error) {
	if course == "" || (teacher == "" && group == "") {
		return false, nil
	}
	if c == nil || c.base == "" || c.token == "" {
		return false, ErrUnavailable
	}
	for _, selector := range []struct{ name, id string }{{"teacher_id", teacher}, {"group_id", group}} {
		if selector.id == "" {
			continue
		}
		q := url.Values{"course_id": {course}, selector.name: {selector.id}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/internal/v1/access?"+q.Encode(), nil)
		if err != nil {
			return false, ErrUnavailable
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.http.Do(req)
		if err != nil {
			return false, ErrUnavailable
		}
		var answer struct {
			Allowed *bool `json:"allowed"`
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1025))
		resp.Body.Close()
		if resp.StatusCode != 200 || readErr != nil || len(raw) > 1024 || json.Unmarshal(raw, &answer) != nil || answer.Allowed == nil {
			return false, ErrUnavailable
		}
		if !*answer.Allowed {
			return false, nil
		}
	}
	return true, nil
}
