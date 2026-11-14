// Package app implements the quizzes use cases on top of ports that the
// adapters implement. Every state change and its outbox event are written in
// one transaction (Store.InTx).
package app

import (
	"context"
	"time"

	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

// Store is the relational store.
type Store interface {
	// InTx runs fn in one database transaction.
	InTx(ctx context.Context, fn func(Tx) error) error

	// ListQuizzes lists quizzes, ownerID "" meaning all, keyset by id.
	ListQuizzes(ctx context.Context, ownerID string, limit int, afterID string) ([]domain.Quiz, error)
	// ListAssigned lists published quizzes assigned to groupID.
	ListAssigned(ctx context.Context, groupID string, limit int, afterID string) ([]AssignedQuiz, error)
	// ListAttempts lists attempts of a quiz.
	ListAttempts(ctx context.Context, quizID string, limit int, afterID string) ([]domain.Attempt, error)
}

// AssignedQuiz is a quiz header with the caller group's assignment.
type AssignedQuiz struct {
	Quiz       domain.Quiz // header only (no questions)
	Assignment domain.Assignment
}

// AnswerLog is one accepted answer save (the audit trail of quiz.answered).
type AnswerLog struct {
	AttemptID  string
	Result     domain.AnswerResult
	QuestionID string
	EventID    string
}

// Tx is the transactional repository.
type Tx interface {
	InsertQuiz(ctx context.Context, q *domain.Quiz) error
	// ReplaceDraft replaces title/settings and the whole question bank of a draft.
	ReplaceDraft(ctx context.Context, q *domain.Quiz) error
	// GetQuiz loads the full quiz; forUpdate locks the quiz row.
	GetQuiz(ctx context.Context, id string, forUpdate bool) (*domain.Quiz, error)
	SetPublished(ctx context.Context, id string, at time.Time) error

	UpsertAssignment(ctx context.Context, a domain.Assignment) error
	GetAssignment(ctx context.Context, quizID, groupID string) (*domain.Assignment, error)
	ListAssignments(ctx context.Context, quizID string) ([]domain.Assignment, error)

	// LockStudentQuiz serialises attempt creation for (quiz, user).
	LockStudentQuiz(ctx context.Context, quizID, userID string) error
	CountAttempts(ctx context.Context, quizID, userID string) (int, error)
	// ActiveAttemptID returns the in-progress attempt id or "".
	ActiveAttemptID(ctx context.Context, quizID, userID string) (string, error)
	InsertAttempt(ctx context.Context, a *domain.Attempt) error
	// GetAttempt loads the aggregate, scoped to userID unless userID is "".
	// forUpdate locks the attempt row: all mutations of one attempt are
	// serialised, which also orders its outbox rows.
	GetAttempt(ctx context.Context, attemptID, userID string, forUpdate bool) (*domain.Attempt, error)
	SaveShown(ctx context.Context, attemptID, questionID string, at time.Time) error
	SaveAnswer(ctx context.Context, a *domain.Attempt, log AnswerLog) error
	TouchAttempt(ctx context.Context, a *domain.Attempt) error
	// Finalize persists the terminal state and the finalization row (whose
	// primary key on attempt_id guarantees a single quiz.submitted).
	Finalize(ctx context.Context, a *domain.Attempt, f domain.Finalization) error
	GetFinalization(ctx context.Context, attemptID string) (*domain.Finalization, error)
	FinalizationByKey(ctx context.Context, userID, key string) (*domain.Finalization, error)
	// ExpiredAttempts returns ids of real-time in-progress attempts whose
	// deadline is before t (simulated attempts are excluded).
	ExpiredAttempts(ctx context.Context, before time.Time, limit int) ([]string, error)

	Enqueue(ctx context.Context, topic, key string, env events.Envelope) error
}
