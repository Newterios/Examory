package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Newterios/Examory/sim/internal/events"
)

type questionMapping struct {
	ID        string    `json:"id"`
	OptionIDs [4]string `json:"option_ids"`
	Hashes    [4]string `json:"answer_hashes"`
}
type attemptMapping struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	GroupID    string `json:"group_id"`
	QuizID     string `json:"quiz_id"`
	VariantID  string `json:"variant_id"`
	Status     string `json:"status"`
	SubmitSent string `json:"submit_sent_at,omitempty"`
	SubmitAck  string `json:"submit_ack_at,omitempty"`
}
type observation struct {
	Kind      string `json:"kind"`
	Attempt   string `json:"world_attempt_id"`
	Scheduled string `json:"scheduled_at"`
	Sent      string `json:"sent_at,omitempty"`
	Ack       string `json:"ack_at,omitempty"`
	HTTP      int    `json:"http_status"`
	Error     string `json:"error,omitempty"`
}
type manifest struct {
	RunID       string                     `json:"run_id"`
	Mode        string                     `json:"mode"`
	Rate        int                        `json:"rate"`
	Duration    int64                      `json:"duration_ms"`
	Shift       int64                      `json:"shift_ms"`
	WorldDigest string                     `json:"world_sha256"`
	KeyDigest   string                     `json:"keys_sha256"`
	Courses     map[string]string          `json:"courses"`
	Users       map[string]string          `json:"users"`
	Groups      map[string]string          `json:"groups"`
	Quizzes     map[string]string          `json:"quizzes"`
	Variants    map[string]string          `json:"variants"`
	Questions   map[string]questionMapping `json:"questions"`
	Attempts    map[string]*attemptMapping `json:"attempts"`
	Actions     []observation              `json:"actions"`
	WindowStart string                     `json:"window_start"`
	WindowEnd   string                     `json:"window_end"`
	Error       string                     `json:"error,omitempty"`
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func (api *api) perform(ctx context.Context, system *session, a action, w *world, m *manifest, r *observation) {
	live := m.Attempts[a.attempt]
	if live.Status == "failed" {
		r.Error = "skipped after attempt failure"
		return
	}
	token, err := system.token(ctx, api)
	if err != nil {
		r.Error = "system session unavailable"
		live.Status = "failed"
		return
	}
	h := map[string]string{"X-Sim-Time": events.FormatTime(a.at), "X-Act-As-User": live.UserID, "X-Act-As-Group": live.GroupID}
	var detail struct {
		ID         string
		Variant    string    `json:"variant_id"`
		Start      time.Time `json:"started_at"`
		Saved      bool
		Unchanged  bool
		FirstShown time.Time `json:"first_shown_at"`
		SavedAt    time.Time `json:"saved_at"`
		Change     bool      `json:"is_change"`
		Status     string
		Score      *int
		Max        *int      `json:"max_score"`
		Finished   time.Time `json:"finished_at"`
	}
	route := "/v1/attempts/" + live.ID
	method, expect := "POST", 200
	var body any
	switch a.kind {
	case start:
		route = "/v1/quizzes/" + live.QuizID + "/attempts"
		expect = 201
	case shown:
		route += "/questions/" + m.Questions[a.question].ID + "/shown"
	case answer:
		route += "/answers/" + m.Questions[a.question].ID
		method = "PUT"
		body = obj{"selected_option_ids": []string{m.Questions[a.question].OptionIDs[a.option]}}
	case submit:
		route += "/submit"
		h["Idempotency-Key"] = "e06-" + live.ID
	}
	r.Sent = stamp(time.Now())
	if a.kind == submit {
		live.SubmitSent = r.Sent
	}
	r.HTTP, err = api.request(ctx, api.quizzes, method, route, token, body, h, expect, &detail)
	r.Ack = stamp(time.Now())
	if err == nil {
		switch a.kind {
		case start:
			if !validID.MatchString(detail.ID) || detail.Variant != live.VariantID || detail.Start.UnixMilli() != a.at {
				err = errors.New("unexpected attempt, variant or start time")
			} else {
				live.ID = detail.ID
			}
		case shown:
			if detail.FirstShown.UnixMilli() != a.at {
				err = errors.New("display time differs from source")
			}
		case answer:
			if !detail.Saved || detail.Unchanged || detail.Change != a.change || detail.SavedAt.UnixMilli() != a.at {
				err = errors.New("answer was not saved with the source revision flag")
			}
		case submit:
			src := w.attempts[a.attempt]
			if detail.Status != "submitted" || detail.Score == nil || *detail.Score != src.score || detail.Max == nil || *detail.Max != src.max || detail.Finished.UnixMilli() != a.at {
				err = errors.New("submit result differs from source")
			}
			if err == nil {
				live.Status = "submitted"
				live.SubmitAck = r.Ack
			}
		}
	}
	if err != nil {
		r.Error = err.Error()
		live.Status = "failed"
	}
}

// Absolute event slots are independent of completion time. Shown shares the next
// emitter's slot. Load serializes each student's stream and allows eight in-flight
// requests across students. Parity preserves the world's global event ordering.
func (api *api) execute(ctx context.Context, system *session, w *world, as []action, m *manifest) error {
	begin := time.Now()
	m.WindowStart = stamp(begin)
	m.Actions = make([]observation, len(as))
	var wg sync.WaitGroup
	workers := map[string]chan int{}
	inFlight := make(chan struct{}, 8)
	counts := map[string]int{}
	if m.Mode == "load" {
		for _, a := range as {
			counts[w.attempts[a.attempt].user]++
		}
		for user, count := range counts {
			jobs := make(chan int, count)
			workers[user] = jobs
			wg.Add(1)
			go func(jobs <-chan int) {
				defer wg.Done()
				for i := range jobs {
					select {
					case inFlight <- struct{}{}:
						api.perform(ctx, system, as[i], w, m, &m.Actions[i])
						<-inFlight
					case <-ctx.Done():
						m.Actions[i].Error = "cancelled before dispatch"
					}
				}
			}(jobs)
		}
	}
	emitters := 0
	for i, a := range as {
		due := begin.Add(time.Duration(emitters) * time.Second / time.Duration(m.Rate))
		row := &m.Actions[i]
		row.Kind = a.kind
		row.Attempt = a.attempt
		row.Scheduled = stamp(due)
		if a.kind != shown {
			emitters++
		}
		if err := sleepUntil(ctx, due); err != nil {
			row.Error = "cancelled before dispatch"
			continue
		}
		if m.Mode == "load" {
			workers[w.attempts[a.attempt].user] <- i
		} else {
			api.perform(ctx, system, a, w, m, row)
		}
	}
	for _, jobs := range workers {
		close(jobs)
	}
	wg.Wait()
	if m.Mode == "load" {
		_ = sleepUntil(ctx, begin.Add(time.Duration(m.Duration)*time.Millisecond))
	}
	m.WindowEnd = stamp(time.Now())
	failures := 0
	for _, row := range m.Actions {
		if row.Error != "" {
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d replay actions failed or skipped", failures)
	}
	return ctx.Err()
}
