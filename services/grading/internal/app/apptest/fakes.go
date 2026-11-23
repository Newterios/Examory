// Package apptest is an in-memory implementation of the grading ports for
// unit tests. Transactions are serialised and rolled back from a snapshot;
// the schema's uniqueness rules (risk_id, one review per risk, idempotency
// key per reviewer) are emulated.
package apptest

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/events"
)

// ErrConstraint emulates a constraint violation.
var ErrConstraint = errors.New("constraint violation")

// Event is an enqueued outbox row.
type Event struct {
	Topic, Key string
	Env        events.Envelope
	Raw        []byte
}

// State is the fake database content.
type State struct {
	Grades   []domain.Grade
	Attempts map[string]domain.AttemptSummary
	Risks    map[string]domain.Risk
	Reviews  map[string]domain.Review
	Audit    []app.AuditEntry
	Rejected []app.Rejected
	Outbox   []Event
}

// Store is the fake.
type Store struct {
	mu sync.Mutex
	s  State
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{s: State{Attempts: map[string]domain.AttemptSummary{}, Risks: map[string]domain.Risk{}, Reviews: map[string]domain.Review{}}}
}

func clone(s State) State {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	if out.Attempts == nil {
		out.Attempts = map[string]domain.AttemptSummary{}
	}
	if out.Risks == nil {
		out.Risks = map[string]domain.Risk{}
	}
	if out.Reviews == nil {
		out.Reviews = map[string]domain.Review{}
	}
	out.Grades = append([]domain.Grade(nil), s.Grades...)
	return out
}

// Snapshot returns a copy of the state.
func (st *Store) Snapshot() State {
	st.mu.Lock()
	defer st.mu.Unlock()
	return clone(st.s)
}

// InTx implements app.Store.
func (st *Store) InTx(_ context.Context, fn func(app.Tx) error) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	snap := clone(st.s)
	if err := fn(&tx{st}); err != nil {
		st.s = snap
		return err
	}
	return nil
}

// Tx runs fn as one transaction and returns its error (consumer emulation).
func (st *Store) Tx(fn func(app.Tx) error) error { return st.InTx(context.Background(), fn) }

// ListRisks implements app.Store.
func (st *Store) ListRisks(_ context.Context, f app.Filter, limit int, after string) ([]app.RiskRow, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []app.RiskRow
	for _, r := range st.s.Risks {
		a, hasA := st.s.Attempts[r.TargetID]
		hasA = hasA && r.TargetType == "attempt"
		if f.Category != "" && f.Category != r.Category {
			continue
		}
		if r.ID <= after || (f.Status != "" && r.Status != f.Status) || (f.CourseID != "" && r.CourseID != f.CourseID) ||
			(f.QuizID != "" && (!hasA || a.QuizID != f.QuizID)) {
			continue
		}
		row := app.RiskRow{Risk: r}
		if hasA {
			row.Attempt = &a
		}
		if rv, ok := st.s.Reviews[r.ID]; ok {
			row.Review = &rv
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Risk.ID < out[j].Risk.ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type tx struct{ st *Store }

func (t *tx) UpsertAttempt(_ context.Context, a domain.AttemptSummary) (bool, error) {
	if _, ok := t.st.s.Attempts[a.AttemptID]; ok {
		return false, nil
	}
	t.st.s.Attempts[a.AttemptID] = a
	return true, nil
}

func (t *tx) InsertRisk(_ context.Context, r *domain.Risk) (bool, error) {
	for _, o := range t.st.s.Risks {
		if o.RiskID == r.RiskID {
			return false, nil
		}
		if o.EventID == r.EventID {
			return false, ErrConstraint
		}
	}
	t.st.s.Risks[r.ID] = *r
	return true, nil
}

func (t *tx) GetRisk(_ context.Context, id string, _ bool) (*domain.Risk, error) {
	r, ok := t.st.s.Risks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &r, nil
}

func (t *tx) GetAttempt(_ context.Context, id string) (*domain.AttemptSummary, error) {
	a, ok := t.st.s.Attempts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (t *tx) GetReview(_ context.Context, pk string) (*domain.Review, error) {
	r, ok := t.st.s.Reviews[pk]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &r, nil
}

func (t *tx) ReviewByKey(_ context.Context, reviewer, key string) (*domain.Review, error) {
	for _, r := range t.st.s.Reviews {
		if r.ReviewerID == reviewer && r.IdempotencyKey == key {
			return &r, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (t *tx) InsertReview(_ context.Context, r domain.Review) error {
	if _, dup := t.st.s.Reviews[r.RiskPK]; dup {
		return ErrConstraint
	}
	t.st.s.Reviews[r.RiskPK] = r
	return nil
}

func (t *tx) UpdateRiskStatus(_ context.Context, r *domain.Risk) error {
	old := t.st.s.Risks[r.ID]
	rv, ok := t.st.s.Reviews[r.ID]
	if old.Status != domain.StatusOpen || !ok || rv.Decision != r.Status || r.Version != old.Version+1 {
		return ErrConstraint // emulates the risks trigger
	}
	t.st.s.Risks[r.ID] = *r
	return nil
}

func (t *tx) Audit(_ context.Context, e app.AuditEntry) error {
	t.st.s.Audit = append(t.st.s.Audit, e)
	return nil
}

func (t *tx) Reject(_ context.Context, r app.Rejected) error {
	t.st.s.Rejected = append(t.st.s.Rejected, r)
	return nil
}

func (t *tx) Enqueue(_ context.Context, topic, key string, env events.Envelope) error {
	if err := env.Validate(); err != nil {
		return err
	}
	raw, _ := json.Marshal(env)
	t.st.s.Outbox = append(t.st.s.Outbox, Event{Topic: topic, Key: key, Env: env, Raw: raw})
	return nil
}
