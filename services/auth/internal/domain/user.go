// Package domain holds the auth service entities, validation rules and
// domain errors. It has no framework or infrastructure imports.
package domain

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Role of a user (events.md actor.role).
type Role string

// Roles.
const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
	RoleSystem  Role = "system"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleStudent, RoleTeacher, RoleAdmin, RoleSystem:
		return true
	}
	return false
}

// Status of an account. Deactivation never deletes academic history (FR-2).
type Status string

// Statuses.
const (
	StatusActive      Status = "active"
	StatusDeactivated Status = "deactivated"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool { return s == StatusActive || s == StatusDeactivated }

// User is an account. GroupID/GroupCourseID are "" when not in a group.
type User struct {
	ID            string
	Email         string
	DisplayName   string
	Role          Role
	Status        Status
	PasswordHash  string
	GroupID       string
	GroupCourseID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Active reports whether the user may authenticate.
func (u User) Active() bool { return u.Status == StatusActive }

// Group is a set of students (optionally bound to a course).
type Group struct {
	ID          string
	Name        string
	CourseID    string // "" when not bound
	CreatedBy   string
	CreatedAt   time.Time
	MemberCount int
}

// Domain errors. The HTTP adapter maps them to status codes.
var (
	ErrNotFound           = errors.New("not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrGroupNameTaken     = errors.New("group name already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrSelfModification   = errors.New("admins cannot demote or deactivate themselves")
	ErrLastAdmin          = errors.New("the last active admin cannot be demoted or deactivated")
	ErrNotStudent         = errors.New("only active students can be group members")
	ErrNotMember          = errors.New("user is not a member of this group")
	ErrForbidden          = errors.New("forbidden")
)

// LockedError means the account is temporarily locked after failed logins.
type LockedError struct{ Until time.Time }

func (e *LockedError) Error() string { return "account temporarily locked" }

// FieldError is one validation failure.
type FieldError struct{ Field, Message string }

// ValidationError aggregates field errors.
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Validator accumulates field errors.
type Validator struct{ errs []FieldError }

// Add records a failure.
func (v *Validator) Add(field, msg string) { v.errs = append(v.errs, FieldError{field, msg}) }

// Err returns a *ValidationError or nil.
func (v *Validator) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.errs}
}

// Limits.
const (
	MaxEmailLen       = 254
	MaxDisplayNameLen = 100
	MaxGroupNameLen   = 100
)

// NormalizeEmail trims and lower-cases an e-mail address.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidateEmail checks a normalized e-mail address.
func ValidateEmail(v *Validator, email string) {
	if email == "" || len(email) > MaxEmailLen {
		v.Add("email", fmt.Sprintf("must be 1..%d characters", MaxEmailLen))
		return
	}
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || a.Name != "" || strings.ContainsAny(email, " \t\r\n<>\"") {
		v.Add("email", "must be a plain e-mail address")
	}
}

// ValidateText checks a human-readable single-line text field.
func ValidateText(v *Validator, field, s string, maxRunes int) {
	n := utf8.RuneCountInString(s)
	if !utf8.ValidString(s) || n == 0 || n > maxRunes || strings.TrimSpace(s) != s {
		v.Add(field, fmt.Sprintf("must be 1..%d characters without leading or trailing spaces", maxRunes))
		return
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			v.Add(field, "must not contain control characters")
			return
		}
	}
}

var courseIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidateCourseID checks an opaque course identifier ("" is allowed).
func ValidateCourseID(v *Validator, s string) {
	if s != "" && !courseIDPattern.MatchString(s) {
		v.Add("course_id", "must match [A-Za-z0-9_-]{1,64}")
	}
}
