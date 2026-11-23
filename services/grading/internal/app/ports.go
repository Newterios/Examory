// Package app implements the grading use cases: ingesting risk signals and
// attempt summaries (idempotently, from Kafka) and the teacher review
// workflow. State changes, audit records and outbox events of one use case
// share one transaction.
package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/events"
)

// Filter narrows ListRisks. Empty fields do not filter.
type Filter struct {
	Status   string
	CourseID string
	QuizID   string
}

// RiskRow is a risk with its attempt summary (if known) and review (if any).
type RiskRow struct {
	Risk    domain.Risk
	Attempt *domain.AttemptSummary
	Review  *domain.Review
}

// AuditEntry is an append-only audit record (NFR-16). Detail holds ids and
// codes only, never evidence content or comments.
type AuditEntry struct {
	At         time.Time
	ActorID    string
	ActorRole  string
	Action     string
	TargetType string
	TargetID   string
	Outcome    string
	Detail     map[string]string
}

// Rejected is a dead-letter record of an event grading could not accept.
type Rejected struct {
	At        time.Time
	Topic     string
	Partition int32
	Offset    int64
	EventID   string
	EventType string
	Reason    string
	Excerpt   string
}

// Store is the relational store.
type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error
	ListRisks(ctx context.Context, f Filter, limit int, afterID string) ([]RiskRow, error)
}

// Tx is the transactional repository.
type Tx interface {
	LockGrade(ctx context.Context, attempt, teacher, key string) (*domain.AttemptSummary, error)
	LastGradeRevision(ctx context.Context, attempt string) (int, error)
	InsertGrade(ctx context.Context, g domain.Grade) error
	GradeByKey(ctx context.Context, teacher, key string) (*domain.Grade, error)
	Grades(ctx context.Context, attempt string, history bool) ([]domain.Grade, error)
	// UpsertAttempt stores an attempt summary once (first quiz.submitted wins).
	UpsertAttempt(ctx context.Context, a domain.AttemptSummary) (bool, error)
	// InsertRisk stores a risk unless its contract risk_id already exists.
	InsertRisk(ctx context.Context, r *domain.Risk) (bool, error)
	GetRisk(ctx context.Context, id string, forUpdate bool) (*domain.Risk, error)
	GetAttempt(ctx context.Context, attemptID string) (*domain.AttemptSummary, error)
	GetReview(ctx context.Context, riskPK string) (*domain.Review, error)
	ReviewByKey(ctx context.Context, reviewerID, key string) (*domain.Review, error)
	InsertReview(ctx context.Context, r domain.Review) error
	UpdateRiskStatus(ctx context.Context, r *domain.Risk) error
	Audit(ctx context.Context, e AuditEntry) error
	Reject(ctx context.Context, r Rejected) error
	Enqueue(ctx context.Context, topic, key string, env events.Envelope) error
}

// Validator validates a serialized event against its JSON Schema.
type Validator interface {
	Validate(raw []byte) error
}

// rawEnvelope re-serialises an envelope for schema validation.
func rawEnvelope(env events.Envelope) ([]byte, error) { return json.Marshal(env) }
