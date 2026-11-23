// Package domain holds the quizzes entities, value objects, validation rules
// and the attempt state machine. It has no framework or I/O dependencies.
package domain

import (
	"errors"
	"fmt"
)

// Kind classifies domain errors for the transport layer.
type Kind int

// Error kinds.
const (
	KindInvalid    Kind = iota + 1 // 422
	KindNotFound                   // 404 (also used to hide objects the caller may not see)
	KindForbidden                  // 403 (role-level denial)
	KindConflict                   // 409 (state does not allow the action)
	KindBadRequest                 // 400
)

// FieldError is one invalid input field.
type FieldError struct {
	Field   string
	Message string
}

// Error is a domain error with a stable machine code.
type Error struct {
	Kind   Kind
	Code   string
	Msg    string
	Fields []FieldError
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Msg) }

// Is matches errors by kind and code.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.Kind == e.Kind && t.Code == e.Code
}

func newErr(k Kind, code, msg string) *Error { return &Error{Kind: k, Code: code, Msg: msg} }

// Sentinel errors.
var (
	ErrNotFound          = newErr(KindNotFound, "not_found", "resource not found")
	ErrForbidden         = newErr(KindForbidden, "forbidden", "this action is not allowed for your role")
	ErrNotDraft          = newErr(KindConflict, "quiz_not_draft", "a published quiz is immutable")
	ErrNotPublished      = newErr(KindConflict, "quiz_not_published", "the quiz must be published first")
	ErrNoGroup           = newErr(KindForbidden, "no_group", "you are not a member of any group")
	ErrNotOpen           = newErr(KindConflict, "quiz_not_open", "the quiz is not open yet")
	ErrClosed            = newErr(KindConflict, "quiz_closed", "the quiz is closed")
	ErrMaxAttempts       = newErr(KindConflict, "max_attempts_reached", "no attempts left")
	ErrAttemptActive     = newErr(KindConflict, "attempt_in_progress", "another attempt of this quiz is in progress")
	ErrAttemptFinalized  = newErr(KindConflict, "attempt_finalized", "the attempt is already finalized")
	ErrTimeOver          = newErr(KindConflict, "time_over", "the attempt time is over")
	ErrNotShown          = newErr(KindConflict, "question_not_shown", "the question must be marked as shown before answering")
	ErrNonMonotonicTime  = newErr(KindConflict, "non_monotonic_time", "simulated time must not go backwards within an attempt")
	ErrClockMismatch     = newErr(KindConflict, "clock_mode_mismatch", "simulated and real-time requests cannot be mixed in one attempt")
	ErrInvariant         = newErr(KindConflict, "state_conflict", "the request conflicts with the current state")
	ErrIdempotencyReuse  = newErr(KindInvalid, "idempotency_key_reused", "the idempotency key was already used for another attempt")
	ErrIdempotencyNeeded = newErr(KindBadRequest, "idempotency_key_required", "a valid Idempotency-Key header is required")
)

// Invalid builds a validation error.
func Invalid(fields []FieldError) *Error {
	return &Error{Kind: KindInvalid, Code: "validation_failed", Msg: "the request is invalid", Fields: fields}
}

// InvalidField builds a single-field validation error.
func InvalidField(field, msg string) *Error { return Invalid([]FieldError{{field, msg}}) }
