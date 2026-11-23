// Package domain holds the grading entities: risk signals received from the
// ML modules, the teacher's review of a signal (human-in-the-loop), and the
// attempt summaries grading learns from lms.quizzes. No framework imports.
//
// Invariant (FR-26): nothing in this package changes a grade, imposes a
// penalty or declares misconduct. A review only records a decision about a
// signal; any academic action is a separate, explicit teacher operation that
// is recorded independently of the review.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Risk statuses. open -> confirmed | dismissed, exactly once.
const (
	StatusOpen      = "open"
	StatusConfirmed = "confirmed"
	StatusDismissed = "dismissed"
)

// Review decisions.
const (
	DecisionConfirmed = "confirmed"
	DecisionDismissed = "dismissed"
)

// Supported risk categories (event types). Others are acknowledged and ignored.
const CategoryBehaviorAnomaly = "risk.behavior_anomaly"

// MaxCommentLen bounds a review comment (runes).
const MaxCommentLen = 2000

// Kind classifies domain errors for the transport.
type Kind int

// Error kinds.
const (
	KindInvalid Kind = iota + 1
	KindNotFound
	KindForbidden
	KindConflict
	KindPrecondition
	KindPreconditionRequired
	KindBadRequest
)

// Error is a domain error with a stable code.
type Error struct {
	Kind Kind
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

// Is matches by kind and code.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Kind == e.Kind && t.Code == e.Code
}

// Errors.
var (
	ErrNotFound          = &Error{KindNotFound, "not_found", "resource not found"}
	ErrForbidden         = &Error{KindForbidden, "forbidden", "this action is not allowed for your role"}
	ErrAlreadyReviewed   = &Error{KindConflict, "already_reviewed", "the risk signal has already been reviewed"}
	ErrVersionMismatch   = &Error{KindPrecondition, "version_mismatch", "the risk signal changed; reload it and retry"}
	ErrIfMatchRequired   = &Error{KindPreconditionRequired, "if_match_required", "an If-Match header with the risk version is required"}
	ErrIdempotencyNeeded = &Error{KindBadRequest, "idempotency_key_required", "a valid Idempotency-Key header is required"}
	ErrIdempotencyReuse  = &Error{KindInvalid, "idempotency_key_reused", "the idempotency key was already used for another request"}
)

// Invalid builds a validation error.
func Invalid(code, msg string) *Error { return &Error{KindInvalid, code, msg} }

var opaqueID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Risk is a stored risk signal.
type Risk struct {
	ID           string // grading's own id (UUIDv7, keyset cursor)
	RiskID       string // contract risk_id (unique; dedup key across re-emissions)
	EventID      string
	Category     string // event_type
	TargetType   string
	TargetID     string
	Score        float64
	Level        string
	ModelName    string
	ModelVersion string
	Explanation  string
	Evidence     json.RawMessage
	CourseID     string
	GroupID      string
	Producer     string
	OccurredAt   time.Time
	ReceivedAt   time.Time
	Status       string
	Version      int
}

// RiskPayload is the shared risk.* payload (events.md §4).
type RiskPayload struct {
	RiskID     string  `json:"risk_id"`
	TargetType string  `json:"target_type"`
	TargetID   string  `json:"target_id"`
	Score      float64 `json:"score"`
	Level      string  `json:"level"`
	Model      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"model"`
	Explanation string          `json:"explanation"`
	Evidence    json.RawMessage `json:"evidence"`
}

// CheckPayload re-checks the invariants grading relies on (schema validation
// happens before; this guards the domain independently of the schema).
func (p RiskPayload) CheckPayload() error {
	switch {
	case !opaqueID.MatchString(p.RiskID), !opaqueID.MatchString(p.TargetID):
		return Invalid("invalid_risk", "risk_id and target_id must be opaque ids")
	case p.Score < 0 || p.Score > 1:
		return Invalid("invalid_risk", "score must be within [0, 1]")
	case p.Level != "low" && p.Level != "medium" && p.Level != "high":
		return Invalid("invalid_risk", "level must be low, medium or high")
	case p.Explanation == "" || !utf8.ValidString(p.Explanation):
		return Invalid("invalid_risk", "explanation is required")
	case len(p.Evidence) == 0 || !json.Valid(p.Evidence):
		return Invalid("invalid_risk", "evidence must be a JSON object")
	}
	return nil
}

// Review is the single teacher decision on a risk.
type Review struct {
	RiskPK         string // Risk.ID
	Decision       string
	Comment        string
	ReviewerID     string
	ReviewerRole   string
	ReviewedAt     time.Time
	IdempotencyKey string
	EventID        string
}

// ValidateDecision checks decision and comment ("with a reason", FR-25).
func ValidateDecision(decision, comment string) error {
	if decision != DecisionConfirmed && decision != DecisionDismissed {
		return Invalid("invalid_decision", "decision must be confirmed or dismissed")
	}
	if !utf8.ValidString(comment) {
		return Invalid("invalid_comment", "comment must be valid UTF-8")
	}
	n := utf8.RuneCountInString(strings.TrimSpace(comment))
	if n < 1 || utf8.RuneCountInString(comment) > MaxCommentLen {
		return Invalid("invalid_comment", fmt.Sprintf("a reason of 1..%d characters is required", MaxCommentLen))
	}
	return nil
}

// ApplyReview transitions the risk. expectedVersion implements optimistic
// concurrency (If-Match); a stale version is rejected even if still open.
func (r *Risk) ApplyReview(decision string, expectedVersion int) error {
	if r.Status != StatusOpen {
		return ErrAlreadyReviewed
	}
	if expectedVersion != r.Version {
		return ErrVersionMismatch
	}
	r.Status = decision // decisions and terminal statuses share names
	r.Version++
	return nil
}

// AttemptSummary is what grading knows about a quiz attempt (quiz.submitted).
type AttemptSummary struct {
	AttemptID      string
	StudentID      string
	QuizID         string
	VariantID      string
	CourseID       string
	GroupID        string
	Score          int
	MaxScore       int
	DurationMs     int64
	AnswersChanged int
	Reason         string
	SubmittedAt    time.Time
	EventID        string
}
