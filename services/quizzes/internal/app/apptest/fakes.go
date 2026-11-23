// Package apptest provides an in-memory implementation of the app ports for
// unit tests. Transactions are serialised by one mutex and rolled back by
// restoring a snapshot, and the uniqueness constraints of the real schema
// (one active attempt, attempt_no, one finalization per attempt, idempotency
// key per user) are emulated so that concurrency tests are meaningful.
package apptest

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

// ErrConstraint emulates a database constraint violation.
var ErrConstraint = errors.New("constraint violation")

// Event is an enqueued outbox row.
type Event struct {
	Topic string
	Key   string
	Env   events.Envelope
	Raw   []byte
}

type state struct {
	Quizzes       map[string]domain.Quiz
	Assignments   map[string]domain.Assignment // quizID|groupID
	Attempts      map[string]domain.Attempt
	Finalizations map[string]domain.Finalization
	AnswerLogs    []app.AnswerLog
	Outbox        []Event
}

// Store is the fake store.
type Store struct {
	mu sync.Mutex
	s  state
	// FailEnqueue makes Enqueue fail (to test transactional rollback).
	FailEnqueue bool
}

// NewStore returns an empty fake store.
func NewStore() *Store {
	return &Store{s: state{Quizzes: map[string]domain.Quiz{}, Assignments: map[string]domain.Assignment{},
		Attempts: map[string]domain.Attempt{}, Finalizations: map[string]domain.Finalization{}}}
}

func clone(s state) state {
	// JSON (not gob) so that pointers to zero values survive the copy.
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	var out state
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	if out.Quizzes == nil {
		out.Quizzes = map[string]domain.Quiz{}
	}
	if out.Assignments == nil {
		out.Assignments = map[string]domain.Assignment{}
	}
	if out.Attempts == nil {
		out.Attempts = map[string]domain.Attempt{}
	}
	if out.Finalizations == nil {
		out.Finalizations = map[string]domain.Finalization{}
	}
	return out
}

// InTx implements app.Store.
func (st *Store) InTx(ctx context.Context, fn func(app.Tx) error) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	snap := clone(st.s)
	if err := fn(&tx{st: st}); err != nil {
		st.s = snap
		return err
	}
	return nil
}

// Events returns a copy of the outbox in insertion order.
func (st *Store) Events() []Event {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]Event(nil), st.s.Outbox...)
}

// Attempt returns a stored attempt.
func (st *Store) Attempt(id string) (domain.Attempt, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	a, ok := st.s.Attempts[id]
	return a, ok
}

// SetAttemptDeadline adjusts an attempt (test helper).
func (st *Store) SetAttemptDeadline(id string, d time.Time) {
	st.mu.Lock()
	defer st.mu.Unlock()
	a := st.s.Attempts[id]
	a.DeadlineAt = d
	st.s.Attempts[id] = a
}

func (st *Store) ListQuizzes(_ context.Context, owner string, limit int, after string) ([]domain.Quiz, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []domain.Quiz
	for _, q := range st.s.Quizzes {
		if (owner == "" || q.OwnerID == owner) && q.ID > after {
			out = append(out, q)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (st *Store) ListAssigned(_ context.Context, group string, limit int, after string) ([]app.AssignedQuiz, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []app.AssignedQuiz
	for _, a := range st.s.Assignments {
		q := st.s.Quizzes[a.QuizID]
		if a.GroupID == group && q.Status == domain.StatusPublished && q.ID > after {
			out = append(out, app.AssignedQuiz{Quiz: q, Assignment: a})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Quiz.ID < out[j].Quiz.ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (st *Store) ListAttempts(_ context.Context, quizID string, limit int, after string) ([]domain.Attempt, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []domain.Attempt
	for _, a := range st.s.Attempts {
		if a.QuizID == quizID && a.ID > after {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type tx struct{ st *Store }

func (t *tx) InsertQuiz(_ context.Context, q *domain.Quiz) error {
	if _, ok := t.st.s.Quizzes[q.ID]; ok {
		return ErrConstraint
	}
	t.st.s.Quizzes[q.ID] = *q
	return nil
}

func (t *tx) ReplaceDraft(_ context.Context, q *domain.Quiz) error {
	old, ok := t.st.s.Quizzes[q.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if old.Status != domain.StatusDraft { // emulates the immutability trigger
		return ErrConstraint
	}
	t.st.s.Quizzes[q.ID] = *q
	return nil
}

func (t *tx) GetQuiz(_ context.Context, id string, _ bool) (*domain.Quiz, error) {
	q, ok := t.st.s.Quizzes[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := clone(state{Quizzes: map[string]domain.Quiz{id: q}}).Quizzes[id]
	return &c, nil
}

func (t *tx) SetPublished(_ context.Context, id string, at time.Time) error {
	q, ok := t.st.s.Quizzes[id]
	if !ok || q.Status != domain.StatusDraft {
		return ErrConstraint
	}
	q.Status, q.PublishedAt, q.UpdatedAt = domain.StatusPublished, &at, at
	t.st.s.Quizzes[id] = q
	return nil
}

func (t *tx) UpsertAssignment(_ context.Context, a domain.Assignment) error {
	t.st.s.Assignments[a.QuizID+"|"+a.GroupID] = a
	return nil
}

func (t *tx) GetAssignment(_ context.Context, quizID, groupID string) (*domain.Assignment, error) {
	a, ok := t.st.s.Assignments[quizID+"|"+groupID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (t *tx) ListAssignments(_ context.Context, quizID string) ([]domain.Assignment, error) {
	var out []domain.Assignment
	for _, a := range t.st.s.Assignments {
		if a.QuizID == quizID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (t *tx) LockStudentQuiz(context.Context, string, string) error { return nil }

func (t *tx) CountAttempts(_ context.Context, quizID, userID string) (int, error) {
	n := 0
	for _, a := range t.st.s.Attempts {
		if a.QuizID == quizID && a.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (t *tx) ActiveAttemptID(_ context.Context, quizID, userID string) (string, error) {
	for _, a := range t.st.s.Attempts {
		if a.QuizID == quizID && a.UserID == userID && a.Status == domain.AttemptInProgress {
			return a.ID, nil
		}
	}
	return "", nil
}

func (t *tx) InsertAttempt(_ context.Context, a *domain.Attempt) error {
	for _, o := range t.st.s.Attempts {
		if o.QuizID == a.QuizID && o.UserID == a.UserID && (o.AttemptNo == a.AttemptNo || o.Status == domain.AttemptInProgress) {
			return ErrConstraint
		}
	}
	t.st.s.Attempts[a.ID] = cloneAttempt(*a)
	return nil
}

func cloneAttempt(a domain.Attempt) domain.Attempt {
	return clone(state{Attempts: map[string]domain.Attempt{"x": a}}).Attempts["x"]
}

func (t *tx) GetAttempt(_ context.Context, id, userID string, _ bool) (*domain.Attempt, error) {
	a, ok := t.st.s.Attempts[id]
	if !ok || (userID != "" && a.UserID != userID) {
		return nil, domain.ErrNotFound
	}
	c := cloneAttempt(a)
	return &c, nil
}

func (t *tx) requireActive(id string) (domain.Attempt, error) {
	a, ok := t.st.s.Attempts[id]
	if !ok {
		return a, domain.ErrNotFound
	}
	if a.Status != domain.AttemptInProgress { // emulates the finalized-attempt trigger
		return a, ErrConstraint
	}
	return a, nil
}

func (t *tx) SaveShown(_ context.Context, id, qid string, at time.Time) error {
	a, err := t.requireActive(id)
	if err != nil {
		return err
	}
	for i := range a.Questions {
		if a.Questions[i].QuestionID == qid {
			if a.Questions[i].FirstShownAt != nil {
				return ErrConstraint
			}
			tt := at
			a.Questions[i].FirstShownAt = &tt
		}
	}
	t.st.s.Attempts[id] = a
	return nil
}

func (t *tx) SaveAnswer(_ context.Context, a *domain.Attempt, log app.AnswerLog) error {
	if _, err := t.requireActive(a.ID); err != nil {
		return err
	}
	t.st.s.Attempts[a.ID] = cloneAttempt(*a)
	t.st.s.AnswerLogs = append(t.st.s.AnswerLogs, log)
	return nil
}

func (t *tx) TouchAttempt(_ context.Context, a *domain.Attempt) error {
	if _, err := t.requireActive(a.ID); err != nil {
		return err
	}
	t.st.s.Attempts[a.ID] = cloneAttempt(*a)
	return nil
}

func (t *tx) Finalize(_ context.Context, a *domain.Attempt, f domain.Finalization) error {
	if _, err := t.requireActive(a.ID); err != nil {
		return err
	}
	if _, dup := t.st.s.Finalizations[a.ID]; dup {
		return ErrConstraint
	}
	for _, o := range t.st.s.Finalizations {
		if f.IdempotencyKey != "" && o.UserID == f.UserID && o.IdempotencyKey == f.IdempotencyKey {
			return ErrConstraint
		}
	}
	t.st.s.Finalizations[a.ID] = f
	t.st.s.Attempts[a.ID] = cloneAttempt(*a)
	return nil
}

func (t *tx) GetFinalization(_ context.Context, id string) (*domain.Finalization, error) {
	f, ok := t.st.s.Finalizations[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &f, nil
}

func (t *tx) FinalizationByKey(_ context.Context, userID, key string) (*domain.Finalization, error) {
	for _, f := range t.st.s.Finalizations {
		if f.UserID == userID && f.IdempotencyKey == key {
			return &f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (t *tx) ExpiredAttempts(_ context.Context, before time.Time, limit int) ([]string, error) {
	var out []string
	for _, a := range t.st.s.Attempts {
		if a.Status == domain.AttemptInProgress && !a.Sim && a.DeadlineAt.Before(before) && len(out) < limit {
			out = append(out, a.ID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (t *tx) Enqueue(_ context.Context, topic, key string, env events.Envelope) error {
	if t.st.FailEnqueue {
		return errors.New("enqueue failed")
	}
	if err := env.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	t.st.s.Outbox = append(t.st.s.Outbox, Event{Topic: topic, Key: key, Env: env, Raw: raw})
	return nil
}
