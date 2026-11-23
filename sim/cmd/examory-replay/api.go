package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Newterios/Examory/sim/internal/events"
)

type obj = map[string]any

func localBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" || u.Port() == "" {
		return "", errors.New("expected a loopback Examory HTTP origin with an explicit port")
	}
	host := u.Hostname()
	if host == "localhost" {
		host = "127.0.0.1"
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return "", errors.New("only loopback Examory ports are supported")
	}
	n, err := strconv.Atoi(u.Port())
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("invalid port")
	}
	return "http://" + net.JoinHostPort(host, u.Port()), nil
}

type api struct {
	client           *http.Client
	gateway, quizzes string
}

type limited struct{ retryAfter string }

func (limited) Error() string { return "HTTP 429: rate limited" }

// Examory auth and the gateway reject rate-limited provisioning requests before
// the write handler runs. Nothing else is retried, including a lost HTTP response.
func provision(ctx context.Context, call func() error) error {
	deadline := time.Now().Add(2 * time.Minute)
	for retries := 0; ; retries++ {
		err := call()
		var rejection limited
		if !errors.As(err, &rejection) || retries >= 5 {
			return err
		}
		delay := retryDelay(rejection.retryAfter, time.Now(), retries)
		due := time.Now().Add(delay)
		if due.After(deadline) {
			return err
		}
		if err := sleepUntil(ctx, due); err != nil {
			return err
		}
	}
}

func retryDelay(value string, now time.Time, retries int) time.Duration {
	value = strings.TrimSpace(value)
	if value != "" && strings.Trim(value, "0123456789") == "" {
		seconds, err := strconv.ParseUint(value, 10, 32)
		if err != nil || seconds > 120 {
			return 121 * time.Second
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(time.Duration(0), at.Sub(now))
	}
	return time.Second << min(retries, 4)
}

// No external redirects, ambient proxy, raw response bodies or credential-bearing errors.
func (api *api) request(ctx context.Context, base, method, path, token string, body any, headers map[string]string, expect int, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, errors.New("request encoding")
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return 0, errors.New("invalid request")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return 0, errors.New("transport or timeout")
	}
	defer resp.Body.Close()
	if resp.StatusCode != expect {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusTooManyRequests {
			return resp.StatusCode, limited{resp.Header.Get("Retry-After")}
		}
		return resp.StatusCode, fmt.Errorf("HTTP %d (expected %d)", resp.StatusCode, expect)
	}
	if out == nil {
		_, err = io.Copy(io.Discard, resp.Body)
	} else {
		err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
	}
	if err != nil {
		return resp.StatusCode, errors.New("invalid response body")
	}
	return resp.StatusCode, nil
}

type session struct {
	mu                      sync.Mutex
	email, password, access string
	renew                   time.Time
}

func (s *session) token(ctx context.Context, api *api) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.access != "" && time.Now().Before(s.renew) {
		return s.access, nil
	}
	var login struct {
		Access  string `json:"access_token"`
		Expires int    `json:"expires_in"`
	}
	_, err := api.request(ctx, api.gateway, "POST", "/api/auth/v1/auth/login", "", obj{"email": s.email, "password": s.password}, nil, 200, &login)
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	if login.Access == "" || login.Expires <= 0 {
		return "", errors.New("empty or expired access token")
	}
	s.access = login.Access
	s.renew = time.Now().Add(time.Duration(login.Expires) * time.Second * 4 / 5)
	return s.access, nil
}

func nonce() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

var validID = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

type identified struct {
	ID string `json:"id"`
}

func sleepUntil(ctx context.Context, due time.Time) error {
	wait := time.Until(due)
	if wait <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (api *api) bootstrap(ctx context.Context, w *world, selected []action, m *manifest, email, password string) (*session, error) {
	admin := &session{email: email, password: password}
	if _, err := admin.token(ctx, api); err != nil {
		return nil, err
	}
	pass, err := nonce()
	if err != nil {
		return nil, err
	}
	pass = "R3!" + pass
	var nextAuth time.Time
	gateway := func(role *session, method, path string, b any, expect int, out any) error {
		return provision(ctx, func() error {
			token, err := role.token(ctx, api)
			if err != nil {
				return fmt.Errorf("provisioning session: %v", err)
			}
			if strings.HasPrefix(path, "/api/auth/") {
				if err = sleepUntil(ctx, nextAuth); err != nil {
					return err
				}
				// The auth service allows 300/IP/minute; leave room for login and other clients.
				nextAuth = time.Now().Add(250 * time.Millisecond)
			}
			_, err = api.request(ctx, api.gateway, method, path, token, b, nil, expect, out)
			return err
		})
	}
	createUser := func(role, name string) (string, string, error) {
		mail := name + "." + m.RunID + "@examory.test"
		var out identified
		err := gateway(admin, "POST", "/api/auth/v1/users", obj{"email": mail, "password": pass, "role": role, "display_name": name}, 201, &out)
		if err == nil && !validID.MatchString(out.ID) {
			err = errors.New("invalid created user id")
		}
		return mail, out.ID, err
	}
	tm, _, err := createUser("teacher", "teacher")
	if err != nil {
		return nil, fmt.Errorf("create teacher: %w", err)
	}
	sm, _, err := createUser("system", "replay")
	if err != nil {
		return nil, fmt.Errorf("create replay user: %w", err)
	}
	teacher := &session{email: tm, password: pass}
	system := &session{email: sm, password: pass}
	enrolled := map[string]string{}
	var users []string
	for _, v := range selected {
		if v.kind != start {
			continue
		}
		a := w.attempts[v.attempt]
		if _, ok := enrolled[a.user]; !ok {
			users = append(users, a.user)
			enrolled[a.user] = a.group
		}
		m.Attempts[v.attempt] = &attemptMapping{Status: "partial"}
	}
	slices.Sort(users)
	// Unique course prevents an earlier replay on the same cluster becoming a peer.
	course := "replay_" + m.RunID
	for i, u := range users {
		g := enrolled[u]
		gid, ok := m.Groups[g]
		if !ok {
			var created identified
			if err = gateway(admin, "POST", "/api/auth/v1/groups", obj{"name": fmt.Sprintf("Replay %s group %d", m.RunID, len(m.Groups)+1), "course_id": course}, 201, &created); err != nil {
				return nil, fmt.Errorf("create group: %w", err)
			}
			if !validID.MatchString(created.ID) {
				return nil, errors.New("invalid created group id")
			}
			gid = created.ID
			m.Groups[g] = gid
		}
		_, id, err := createUser("student", fmt.Sprintf("student%d", i+1))
		if err != nil {
			return nil, fmt.Errorf("create student: %w", err)
		}
		m.Users[u] = id
		if err = gateway(admin, "PUT", "/api/auth/v1/groups/"+gid+"/members/"+id, nil, 200, nil); err != nil {
			return nil, fmt.Errorf("enroll student: %w", err)
		}
	}
	questions := []obj{}
	for i, q := range w.questionIDs {
		opts := make([]obj, 4)
		for n := range opts {
			opts[n] = obj{"text": strconv.Itoa(n), "key": strconv.Itoa(n)}
		}
		questions = append(questions, obj{"text": fmt.Sprintf("Item %d", i+1), "points": 1, "options": opts, "correct_keys": []string{strconv.Itoa(w.keys[q])}})
	}
	var quiz struct {
		ID        string
		Variants  []identified
		Questions []struct {
			ID      string
			Options []struct {
				ID, Text string
				Correct  bool
			}
		}
	}
	err = gateway(teacher, "POST", "/api/quizzes/v1/quizzes", obj{"course_id": course, "title": "E06 " + m.RunID, "time_limit_sec": 14400, "max_attempts": 20, "show_score": true, "questions": questions}, 201, &quiz)
	if err != nil {
		return nil, fmt.Errorf("create quiz: %w", err)
	}
	if !validID.MatchString(quiz.ID) || len(quiz.Variants) != 1 || !validID.MatchString(quiz.Variants[0].ID) || len(quiz.Questions) != len(questions) {
		return nil, errors.New("invalid authored quiz mapping")
	}
	m.Quizzes[w.quiz] = quiz.ID
	m.Variants[w.variant] = quiz.Variants[0].ID
	m.Courses[w.course] = course
	for i, q := range quiz.Questions {
		if !validID.MatchString(q.ID) || len(q.Options) != 4 {
			return nil, errors.New("invalid authored question")
		}
		entry := questionMapping{ID: q.ID}
		for _, option := range q.Options {
			n, err := strconv.Atoi(option.Text)
			if err != nil || n < 0 || n >= 4 || !validID.MatchString(option.ID) || entry.OptionIDs[n] != "" || option.Correct != (n == w.keys[w.questionIDs[i]]) {
				return nil, errors.New("invalid authored option or marking key")
			}
			entry.OptionIDs[n] = option.ID
			entry.Hashes[n] = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(option.ID)))))
		}
		m.Questions[w.questionIDs[i]] = entry
	}
	if err = gateway(teacher, "POST", "/api/quizzes/v1/quizzes/"+quiz.ID+"/publish", nil, 200, nil); err != nil {
		return nil, fmt.Errorf("publish quiz: %w", err)
	}
	window := obj{"opens_at": events.FormatTime(selected[0].at - 1000), "closes_at": events.FormatTime(selected[len(selected)-1].at + 1000)}
	for _, gid := range m.Groups {
		if err = gateway(teacher, "PUT", "/api/quizzes/v1/quizzes/"+quiz.ID+"/assignments/"+gid, window, 200, nil); err != nil {
			return nil, fmt.Errorf("assign quiz: %w", err)
		}
	}
	for id, entry := range m.Attempts {
		a := w.attempts[id]
		entry.UserID = m.Users[a.user]
		entry.GroupID = m.Groups[a.group]
		entry.QuizID = quiz.ID
		entry.VariantID = quiz.Variants[0].ID
	}
	_, err = system.token(ctx, api)
	return system, err
}
