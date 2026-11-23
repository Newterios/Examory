// Package events implements the event envelope of
// docs/03_architecture/events.md §1 and the typed payloads produced by the
// platform core. It is the Go side of the contract; the JSON Schemas in
// docs/03_architecture/schemas/ are the language-neutral side.
package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/Newterios/Examory/services/platform/ids"
)

// Topics (events.md §2).
const (
	TopicUsers       = "lms.users"
	TopicCourses     = "lms.courses"
	TopicSubmissions = "lms.submissions"
	TopicQuizzes     = "lms.quizzes"
	TopicGrades      = "lms.grades"
	TopicExams       = "lms.exams"
	TopicRisk        = "ai.risk"
	TopicAnalytics   = "ai.analytics"
)

// TimeLayout is RFC 3339 UTC with millisecond precision.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// Time is a UTC instant serialised with millisecond precision.
type Time struct{ time.Time }

// NewTime truncates t to milliseconds in UTC.
func NewTime(t time.Time) Time { return Time{t.UTC().Truncate(time.Millisecond)} }

// MarshalJSON implements json.Marshaler.
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format(TimeLayout) + `"`), nil
}

// UnmarshalJSON accepts RFC 3339 with or without fractional seconds.
func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	*t = NewTime(p)
	return nil
}

// Actor is who performed the action.
type Actor struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// Context carries optional course/group/exam-session identifiers.
type Context struct {
	CourseID  *string `json:"course_id"`
	GroupID   *string `json:"group_id"`
	SessionID *string `json:"session_id"`
}

// Envelope is the common event wrapper.
type Envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Version    int             `json:"version"`
	OccurredAt Time            `json:"occurred_at"`
	Producer   string          `json:"producer"`
	Actor      Actor           `json:"actor"`
	Context    Context         `json:"context"`
	Payload    json.RawMessage `json:"payload"`
}

var typePattern = regexp.MustCompile(`^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$`)

var validRoles = map[string]bool{"student": true, "teacher": true, "admin": true, "system": true}

// Ptr returns a pointer to s, or nil when s is empty (for nullable context fields).
func Ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// New builds an envelope. occurred_at is `at` truncated to milliseconds and
// event_id is a UUIDv7 whose timestamp equals occurred_at.
func New(eventType string, version int, producer string, actor Actor, ectx Context, payload any, at time.Time) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}
	ts := NewTime(at)
	env := Envelope{
		EventID:    ids.NewV7At(ts.Time),
		EventType:  eventType,
		Version:    version,
		OccurredAt: ts,
		Producer:   producer,
		Actor:      actor,
		Context:    ectx,
		Payload:    raw,
	}
	return env, env.Validate()
}

// Validate checks envelope invariants (not the payload schema).
func (e Envelope) Validate() error {
	var errs []error
	ts, err := ids.V7Time(e.EventID)
	switch {
	case err != nil:
		errs = append(errs, errors.New("event_id must be a UUIDv7"))
	case !ts.Equal(e.OccurredAt.Time):
		errs = append(errs, errors.New("event_id timestamp must equal occurred_at"))
	}
	if !typePattern.MatchString(e.EventType) {
		errs = append(errs, errors.New("event_type must be <domain>.<action> in lower snake case"))
	}
	if e.Version < 1 {
		errs = append(errs, errors.New("version must be >= 1"))
	}
	if e.Producer == "" {
		errs = append(errs, errors.New("producer is required"))
	}
	if e.Actor.UserID == "" || !validRoles[e.Actor.Role] {
		errs = append(errs, errors.New("actor.user_id and a valid actor.role are required"))
	}
	if len(bytes.TrimSpace(e.Payload)) == 0 || bytes.TrimSpace(e.Payload)[0] != '{' {
		errs = append(errs, errors.New("payload must be a JSON object"))
	}
	return errors.Join(errs...)
}

// Parse decodes and validates an envelope.
func Parse(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return e, e.Validate()
}

// DecodePayload unmarshals the payload into dst, rejecting unknown fields
// only if strict is true (consumers should normally be lenient: events.md
// allows additive optional fields without a version bump).
func (e Envelope) DecodePayload(dst any, strict bool) error {
	dec := json.NewDecoder(bytes.NewReader(e.Payload))
	if strict {
		dec.DisallowUnknownFields()
	}
	return dec.Decode(dst)
}
