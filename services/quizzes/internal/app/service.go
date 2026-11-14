package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

// Producer is the envelope producer name.
const Producer = "quizzes"

// Actor is the effective caller of a use case: the principal (after any
// research act-as substitution) and the request clock.
type Actor struct {
	authn.Principal
	Clock domain.Clock
}

// Config tunes the service.
type Config struct {
	// Grace accepted after an attempt deadline for in-flight requests.
	Grace time.Duration
}

// Service implements the use cases.
type Service struct {
	store Store
	clock clock.Clock
	log   *slog.Logger
	cfg   Config
	newID domain.IDFunc
	seed  func() []byte
}

// New returns a Service.
func New(store Store, clk clock.Clock, logger *slog.Logger, cfg Config) *Service {
	return &Service{
		store: store, clock: clk, log: logger, cfg: cfg,
		newID: ids.NewV7At,
		seed: func() []byte {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			return b
		},
	}
}

// Now returns the real clock reading truncated to milliseconds (used to build
// real-time actors). Event times have millisecond precision; keeping every
// stored instant ms-aligned makes occurred_at, time_since_attempt_start_ms and
// duration_ms exactly consistent (answer.occurred_at == submitted.occurred_at
// - (duration_ms - time_since_attempt_start_ms)), which the anomaly feature
// pipeline checks to the millisecond.
func (s *Service) Now() time.Time { return s.clock.Now().Truncate(time.Millisecond) }

func canManage(p authn.Principal, q *domain.Quiz) bool {
	return p.Role == authn.RoleAdmin || (p.Role == authn.RoleTeacher && q.OwnerID == p.UserID)
}

func requireAuthor(p authn.Principal) error {
	if !p.Is(authn.RoleTeacher, authn.RoleAdmin) {
		return domain.ErrForbidden
	}
	return nil
}

func requireStudent(p authn.Principal) error {
	if p.Role != authn.RoleStudent {
		return domain.ErrForbidden
	}
	return nil
}

// ---- authoring ----

// CreateQuiz creates a draft owned by the caller.
func (s *Service) CreateQuiz(ctx context.Context, a Actor, spec domain.QuizSpec) (*domain.Quiz, error) {
	if err := requireAuthor(a.Principal); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	q, err := domain.BuildQuiz(spec, s.newID(now), a.UserID, now, s.newID)
	if err != nil {
		return nil, err
	}
	if err := s.store.InTx(ctx, func(tx Tx) error { return tx.InsertQuiz(ctx, q) }); err != nil {
		return nil, err
	}
	return q, nil
}

// manageable loads a quiz the caller may manage; others get ErrNotFound so
// that teachers cannot probe each other's quizzes (object-level authz).
func manageable(ctx context.Context, tx Tx, a Actor, id string, forUpdate bool) (*domain.Quiz, error) {
	if err := requireAuthor(a.Principal); err != nil {
		return nil, err
	}
	q, err := tx.GetQuiz(ctx, id, forUpdate)
	if err != nil {
		return nil, err
	}
	if !canManage(a.Principal, q) {
		return nil, domain.ErrNotFound
	}
	return q, nil
}

// UpdateQuiz replaces a draft.
func (s *Service) UpdateQuiz(ctx context.Context, a Actor, id string, spec domain.QuizSpec) (*domain.Quiz, error) {
	var out *domain.Quiz
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := manageable(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if q.Status != domain.StatusDraft {
			return domain.ErrNotDraft
		}
		now := s.clock.Now()
		nq, err := domain.BuildQuiz(spec, q.ID, q.OwnerID, now, s.newID)
		if err != nil {
			return err
		}
		nq.CreatedAt = q.CreatedAt
		if err := tx.ReplaceDraft(ctx, nq); err != nil {
			return err
		}
		out = nq
		return nil
	})
	return out, err
}

// PublishQuiz makes a draft immutable and attemptable.
func (s *Service) PublishQuiz(ctx context.Context, a Actor, id string) (*domain.Quiz, error) {
	var out *domain.Quiz
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := manageable(ctx, tx, a, id, true)
		if err != nil {
			return err
		}
		if err := q.Publish(s.clock.Now()); err != nil {
			return err
		}
		out = q
		return tx.SetPublished(ctx, q.ID, *q.PublishedAt)
	})
	return out, err
}

// GetQuizForAuthor returns the full quiz, including the marking key.
func (s *Service) GetQuizForAuthor(ctx context.Context, a Actor, id string) (*domain.Quiz, error) {
	var out *domain.Quiz
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := manageable(ctx, tx, a, id, false)
		out = q
		return err
	})
	return out, err
}

// StudentQuiz is the student view of an assigned quiz (no questions).
type StudentQuiz struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	CourseID        string         `json:"course_id"`
	TimeLimitSec    int            `json:"time_limit_sec"`
	MaxAttempts     int            `json:"max_attempts"`
	QuestionCount   int            `json:"question_count"`
	Assignment      AssignmentView `json:"assignment"`
	AttemptsUsed    int            `json:"attempts_used"`
	ActiveAttemptID *string        `json:"active_attempt_id"`
}

// AssignmentView is the JSON form of an assignment.
type AssignmentView struct {
	GroupID  string    `json:"group_id"`
	OpensAt  time.Time `json:"opens_at"`
	ClosesAt time.Time `json:"closes_at"`
}

// GetQuizForStudent returns an assigned, published quiz.
func (s *Service) GetQuizForStudent(ctx context.Context, a Actor, id string) (*StudentQuiz, error) {
	if err := requireStudent(a.Principal); err != nil {
		return nil, err
	}
	if a.GroupID == "" {
		return nil, domain.ErrNotFound
	}
	var out *StudentQuiz
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := tx.GetQuiz(ctx, id, false)
		if err != nil {
			return err
		}
		if q.Status != domain.StatusPublished {
			return domain.ErrNotFound
		}
		asg, err := tx.GetAssignment(ctx, q.ID, a.GroupID)
		if err != nil {
			return err
		}
		n, err := tx.CountAttempts(ctx, q.ID, a.UserID)
		if err != nil {
			return err
		}
		active, err := tx.ActiveAttemptID(ctx, q.ID, a.UserID)
		if err != nil {
			return err
		}
		out = &StudentQuiz{ID: q.ID, Title: q.Title, CourseID: q.CourseID, TimeLimitSec: q.TimeLimitSec,
			MaxAttempts: q.MaxAttempts, QuestionCount: len(q.Variants[0].Questions),
			Assignment:   AssignmentView{asg.GroupID, asg.OpensAt, asg.ClosesAt},
			AttemptsUsed: n, ActiveAttemptID: events.Ptr(active)}
		return nil
	})
	return out, err
}

// ListForAuthor lists the caller's quizzes (admins: all).
func (s *Service) ListForAuthor(ctx context.Context, a Actor, limit int, after string) ([]domain.Quiz, error) {
	if err := requireAuthor(a.Principal); err != nil {
		return nil, err
	}
	owner := a.UserID
	if a.Role == authn.RoleAdmin {
		owner = ""
	}
	return s.store.ListQuizzes(ctx, owner, limit, after)
}

// ListForStudent lists published quizzes assigned to the caller's group.
func (s *Service) ListForStudent(ctx context.Context, a Actor, limit int, after string) ([]AssignedQuiz, error) {
	if err := requireStudent(a.Principal); err != nil {
		return nil, err
	}
	if a.GroupID == "" {
		return nil, nil
	}
	return s.store.ListAssigned(ctx, a.GroupID, limit, after)
}

// Assign opens a published quiz to a group (upsert of the window).
func (s *Service) Assign(ctx context.Context, a Actor, quizID, groupID string, opens, closes time.Time) (*domain.Assignment, error) {
	asg := domain.Assignment{QuizID: quizID, GroupID: groupID, OpensAt: opens.UTC(), ClosesAt: closes.UTC(), AssignedBy: a.UserID, AssignedAt: s.clock.Now()}
	if err := asg.Validate(); err != nil {
		return nil, err
	}
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := manageable(ctx, tx, a, quizID, true)
		if err != nil {
			return err
		}
		if q.Status != domain.StatusPublished {
			return domain.ErrNotPublished
		}
		return tx.UpsertAssignment(ctx, asg)
	})
	if err != nil {
		return nil, err
	}
	return &asg, nil
}

// ListAssignments lists the groups a quiz is assigned to.
func (s *Service) ListAssignments(ctx context.Context, a Actor, quizID string) ([]domain.Assignment, error) {
	var out []domain.Assignment
	err := s.store.InTx(ctx, func(tx Tx) error {
		if _, err := manageable(ctx, tx, a, quizID, false); err != nil {
			return err
		}
		var err error
		out, err = tx.ListAssignments(ctx, quizID)
		return err
	})
	return out, err
}

// ListQuizAttempts lists attempts of a quiz for its author.
func (s *Service) ListQuizAttempts(ctx context.Context, a Actor, quizID string, limit int, after string) ([]domain.Attempt, error) {
	err := s.store.InTx(ctx, func(tx Tx) error {
		_, err := manageable(ctx, tx, a, quizID, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.store.ListAttempts(ctx, quizID, limit, after)
}

// ---- events ----

func (s *Service) enqueue(ctx context.Context, tx Tx, typ string, at *domain.Attempt, payload any, when time.Time) (string, error) {
	env, err := events.New(typ, 1, Producer,
		events.Actor{UserID: at.UserID, Role: string(authn.RoleStudent)},
		events.Context{CourseID: events.Ptr(at.CourseID), GroupID: events.Ptr(at.GroupID)},
		payload, when)
	if err != nil {
		return "", fmt.Errorf("build %s: %w", typ, err)
	}
	// Partition key actor.user_id keeps one student's events ordered.
	if err := tx.Enqueue(ctx, events.TopicQuizzes, at.UserID, env); err != nil {
		return "", err
	}
	return env.EventID, nil
}

// ---- attempts ----

// StartAttempt creates an attempt for an assigned, open quiz.
func (s *Service) StartAttempt(ctx context.Context, a Actor, quizID string) (*AttemptView, error) {
	if err := requireStudent(a.Principal); err != nil {
		return nil, err
	}
	if a.GroupID == "" {
		return nil, domain.ErrNoGroup
	}
	now := a.Clock.Now
	var view *AttemptView
	err := s.store.InTx(ctx, func(tx Tx) error {
		q, err := tx.GetQuiz(ctx, quizID, false)
		if err != nil {
			return err
		}
		if q.Status != domain.StatusPublished {
			return domain.ErrNotFound
		}
		// The group comes from the token, never from the request body.
		asg, err := tx.GetAssignment(ctx, q.ID, a.GroupID)
		if err != nil {
			return err
		}
		if err := asg.CheckOpen(now); err != nil {
			return err
		}
		if err := tx.LockStudentQuiz(ctx, q.ID, a.UserID); err != nil {
			return err
		}
		if activeID, err := tx.ActiveAttemptID(ctx, q.ID, a.UserID); err != nil {
			return err
		} else if activeID != "" {
			active, err := tx.GetAttempt(ctx, activeID, a.UserID, true)
			if err != nil {
				return err
			}
			if active.Sim != a.Clock.Sim || !active.Expired(now, s.cfg.Grace) {
				return domain.ErrAttemptActive
			}
			if err := s.finalize(ctx, tx, active, domain.AttemptTimedOut, "", now); err != nil {
				return err
			}
		}
		n, err := tx.CountAttempts(ctx, q.ID, a.UserID)
		if err != nil {
			return err
		}
		if n >= q.MaxAttempts {
			return domain.ErrMaxAttempts
		}
		v := q.VariantFor(n + 1)
		at := domain.NewAttempt(s.newID(now), q, v, *asg, a.UserID, n+1, s.seed(), now, a.Clock.Sim)
		if err := tx.InsertAttempt(ctx, at); err != nil {
			return err
		}
		if _, err := s.enqueue(ctx, tx, events.TypeQuizStarted, at, events.QuizStarted{
			AttemptID: at.ID, QuizID: q.ID, VariantID: v.ID, QuestionCount: len(v.Questions),
			TimeLimitSec: q.TimeLimitSec, ExamSessionID: nil,
		}, now); err != nil {
			return err
		}
		view = studentView(at, now)
		return nil
	})
	return view, err
}

// loadOwn locks and loads the caller's attempt and resolves request time.
func (s *Service) loadOwn(ctx context.Context, tx Tx, a Actor, attemptID string) (*domain.Attempt, time.Time, error) {
	if err := requireStudent(a.Principal); err != nil {
		return nil, time.Time{}, err
	}
	// Scoped by the token's user id: another student's attempt is not found.
	at, err := tx.GetAttempt(ctx, attemptID, a.UserID, true)
	if err != nil {
		return nil, time.Time{}, err
	}
	now, err := at.Effective(a.Clock)
	if err != nil {
		return nil, time.Time{}, err
	}
	return at, now, nil
}

// expireIfNeeded finalizes an attempt past its deadline (lazy timeout, needed
// for simulated attempts which the sweeper never touches). It returns true
// when the attempt was finalized.
func (s *Service) expireIfNeeded(ctx context.Context, tx Tx, at *domain.Attempt, now time.Time) (bool, error) {
	if !at.Expired(now, s.cfg.Grace) {
		return false, nil
	}
	return true, s.finalize(ctx, tx, at, domain.AttemptTimedOut, "", now)
}

// GetAttempt returns the student view of the caller's attempt.
func (s *Service) GetAttempt(ctx context.Context, a Actor, attemptID string) (*AttemptView, error) {
	var view *AttemptView
	err := s.store.InTx(ctx, func(tx Tx) error {
		if err := requireStudent(a.Principal); err != nil {
			return err
		}
		at, err := tx.GetAttempt(ctx, attemptID, a.UserID, false)
		if err != nil {
			return err
		}
		now := a.Clock.Now
		if now.Before(at.LastActivityAt) {
			now = at.LastActivityAt
		}
		view = studentView(at, now)
		return nil
	})
	return view, err
}

// errCommitThen lets a transaction commit a side effect (lazy timeout) and
// still return an error to the caller.
type errCommitThen struct{ err error }

func (e errCommitThen) Error() string { return e.err.Error() }

func (s *Service) inTxCommitOn(ctx context.Context, fn func(Tx) error) error {
	var deferred error
	err := s.store.InTx(ctx, func(tx Tx) error {
		err := fn(tx)
		var ct errCommitThen
		if errors.As(err, &ct) {
			deferred = ct.err
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}
	return deferred
}

// MarkShown records the first display time of a question (idempotent).
func (s *Service) MarkShown(ctx context.Context, a Actor, attemptID, questionID string) (time.Time, error) {
	var shownAt time.Time
	err := s.inTxCommitOn(ctx, func(tx Tx) error {
		at, now, err := s.loadOwn(ctx, tx, a, attemptID)
		if err != nil {
			return err
		}
		if done, err := s.expireIfNeeded(ctx, tx, at, now); err != nil {
			return err
		} else if done {
			return errCommitThen{domain.ErrTimeOver}
		}
		changed, err := at.MarkShown(questionID, now, s.cfg.Grace)
		if err != nil {
			return err
		}
		for _, q := range at.Questions {
			if q.QuestionID == questionID {
				shownAt = *q.FirstShownAt
			}
		}
		if !changed {
			return nil
		}
		if err := tx.SaveShown(ctx, at.ID, questionID, now); err != nil {
			return err
		}
		return tx.TouchAttempt(ctx, at)
	})
	return shownAt, err
}

// AnswerAck acknowledges a persisted answer (FR-13).
type AnswerAck struct {
	QuestionID     string    `json:"question_id"`
	Saved          bool      `json:"saved"`
	Unchanged      bool      `json:"unchanged"`
	IsChange       bool      `json:"is_change"`
	SavedAt        time.Time `json:"saved_at"`
	AnswersChanged int       `json:"answers_changed"`
}

// SaveAnswer stores an answer and publishes quiz.answered (unless identical).
func (s *Service) SaveAnswer(ctx context.Context, a Actor, attemptID, questionID string, selected []string) (*AnswerAck, error) {
	var ack *AnswerAck
	err := s.inTxCommitOn(ctx, func(tx Tx) error {
		at, now, err := s.loadOwn(ctx, tx, a, attemptID)
		if err != nil {
			return err
		}
		if done, err := s.expireIfNeeded(ctx, tx, at, now); err != nil {
			return err
		} else if done {
			return errCommitThen{domain.ErrTimeOver}
		}
		res, err := at.SaveAnswer(questionID, selected, now, s.cfg.Grace)
		if err != nil {
			return err
		}
		ack = &AnswerAck{QuestionID: questionID, Saved: true, Unchanged: res.Noop, IsChange: res.IsChange, SavedAt: res.SavedAt, AnswersChanged: at.AnswersChanged}
		if res.Noop {
			return nil
		}
		eventID, err := s.enqueue(ctx, tx, events.TypeQuizAnswered, at, events.QuizAnswered{
			AttemptID: at.ID, QuestionID: questionID, QuestionIndex: res.QuestionIndex, AnswerHash: res.Hash,
			IsChange: res.IsChange, TimeSinceShownMs: res.TimeSinceShownMs, TimeSinceAttemptStartMs: res.TimeSinceAttemptStartMs,
		}, now)
		if err != nil {
			return err
		}
		return tx.SaveAnswer(ctx, at, AnswerLog{AttemptID: at.ID, QuestionID: questionID, Result: res, EventID: eventID})
	})
	return ack, err
}

// Result is the outcome of a finalized attempt.
type Result struct {
	AttemptID  string    `json:"attempt_id"`
	Status     string    `json:"status"`
	FinishedAt time.Time `json:"finished_at"`
	Score      *int      `json:"score,omitempty"`
	MaxScore   *int      `json:"max_score,omitempty"`
	Replayed   bool      `json:"-"`
}

func resultOf(at *domain.Attempt) *Result {
	r := &Result{AttemptID: at.ID, Status: at.Status, FinishedAt: *at.FinishedAt}
	if at.ShowScore {
		r.Score, r.MaxScore = at.Score, at.MaxScore
	}
	return r
}

// Submit finalizes the caller's attempt exactly once. The same idempotency
// key replays the stored result; an attempt that timed out returns its
// timed-out result; a different key on a submitted attempt is a conflict.
func (s *Service) Submit(ctx context.Context, a Actor, attemptID, key string) (*Result, error) {
	if !ValidIdempotencyKey(key) {
		return nil, domain.ErrIdempotencyNeeded
	}
	var out *Result
	err := s.store.InTx(ctx, func(tx Tx) error {
		if err := requireStudent(a.Principal); err != nil {
			return err
		}
		if prev, err := tx.FinalizationByKey(ctx, a.UserID, key); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		} else if prev != nil && prev.AttemptID != attemptID {
			return domain.ErrIdempotencyReuse
		}
		at, err := tx.GetAttempt(ctx, attemptID, a.UserID, true)
		if err != nil {
			return err
		}
		if at.Status != domain.AttemptInProgress {
			f, err := tx.GetFinalization(ctx, at.ID)
			if err != nil {
				return err
			}
			if f.Reason == domain.AttemptSubmitted && f.IdempotencyKey != key {
				return domain.ErrAttemptFinalized
			}
			out = resultOf(at)
			out.Replayed = true
			return nil
		}
		now, err := at.Effective(a.Clock)
		if err != nil {
			return err
		}
		if err := s.finalize(ctx, tx, at, domain.AttemptSubmitted, key, now); err != nil {
			return err
		}
		out = resultOf(at)
		return nil
	})
	return out, err
}

// finalize transitions, scores, records the finalization and enqueues the
// single quiz.submitted, all inside tx.
func (s *Service) finalize(ctx context.Context, tx Tx, at *domain.Attempt, reason, key string, now time.Time) error {
	res, err := at.Finalize(reason, now, s.cfg.Grace)
	if err != nil {
		return err
	}
	if res.Status == domain.AttemptTimedOut {
		key = "" // timeouts carry no client key
	}
	payloadReason := events.ReasonSubmitted
	if res.Status == domain.AttemptTimedOut {
		payloadReason = events.ReasonTimedOut
	}
	eventID, err := s.enqueue(ctx, tx, events.TypeQuizSubmitted, at, events.QuizSubmitted{
		AttemptID: at.ID, QuizID: at.QuizID, VariantID: at.VariantID, Score: res.Score, MaxScore: res.MaxScore,
		DurationMs: res.DurationMs, AnswersChanged: res.AnswersChanged, Reason: payloadReason,
	}, res.FinishedAt)
	if err != nil {
		return err
	}
	return tx.Finalize(ctx, at, domain.Finalization{
		AttemptID: at.ID, UserID: at.UserID, Reason: res.Status, IdempotencyKey: key,
		FinalizedAt: res.FinishedAt, EventID: eventID,
	})
}

// SweepExpired times out real-time attempts past deadline+grace. It is safe
// to run on several replicas: each attempt is re-checked under its row lock.
func (s *Service) SweepExpired(ctx context.Context, batch int) (int, error) {
	now := s.clock.Now()
	var idsList []string
	err := s.store.InTx(ctx, func(tx Tx) error {
		var err error
		idsList, err = tx.ExpiredAttempts(ctx, now.Add(-s.cfg.Grace), batch)
		return err
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range idsList {
		err := s.store.InTx(ctx, func(tx Tx) error {
			at, err := tx.GetAttempt(ctx, id, "", true)
			if err != nil {
				return err
			}
			if at.Sim || !at.Expired(now, s.cfg.Grace) {
				return nil // finalized concurrently, or not ours to sweep
			}
			n++
			return s.finalize(ctx, tx, at, domain.AttemptTimedOut, "", now)
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// RunSweeper calls SweepExpired every interval until ctx ends.
func (s *Service) RunSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := s.SweepExpired(ctx, 100)
			if err != nil && ctx.Err() == nil {
				s.log.Warn("timeout sweeper failed", slog.String("error", err.Error()))
			} else if n > 0 {
				s.log.Info("attempts timed out", slog.Int("count", n))
			}
		}
	}
}

// ValidIdempotencyKey checks the Idempotency-Key format (8..128 of
// [A-Za-z0-9_.:-]); it mirrors the database CHECK.
func ValidIdempotencyKey(k string) bool {
	if len(k) < 8 || len(k) > 128 {
		return false
	}
	for _, c := range k {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '.', c == ':', c == '-':
		default:
			return false
		}
	}
	return true
}
