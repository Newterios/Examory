package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Quiz statuses.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Limits enforced at the boundary and mirrored by database CHECKs.
const (
	MaxTitleLen      = 200
	MaxQuestionLen   = 2000
	MaxOptionLen     = 500
	MinOptions       = 2
	MaxOptions       = 8
	MaxQuestions     = 100
	MinPoints        = 1
	MaxPoints        = 100
	MinTimeLimitSec  = 30
	MaxTimeLimitSec  = 4 * 3600
	MaxAttemptsLimit = 20
	MaxWindow        = 366 * 24 * time.Hour
)

var (
	opaqueID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	keyRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`)
)

// Option is an answer option. Correct is part of the marking key and must
// never be exposed to students.
type Option struct {
	ID      string
	Text    string
	Correct bool
}

// Question is a bank question of a quiz.
type Question struct {
	ID      string
	Text    string
	Points  int
	TopicID string // "" when unmapped
	Options []Option
}

// CorrectCount is the number of correct options.
func (q Question) CorrectCount() int {
	n := 0
	for _, o := range q.Options {
		if o.Correct {
			n++
		}
	}
	return n
}

// Variant is an ordered selection of bank questions.
type Variant struct {
	ID        string
	Ordinal   int
	Questions []Question
}

// MaxScore is the sum of question points.
func (v Variant) MaxScore() int {
	s := 0
	for _, q := range v.Questions {
		s += q.Points
	}
	return s
}

// Quiz is the aggregate root. For M4 a quiz has exactly one variant holding
// all bank questions; the schema allows several.
type Quiz struct {
	ID           string
	OwnerID      string
	CourseID     string
	Title        string
	TimeLimitSec int
	MaxAttempts  int
	ShowScore    bool
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	PublishedAt  *time.Time
	Questions    []Question
	Variants     []Variant
}

// OptionSpec is an option in a create/update request. Key is a client-chosen
// label used only to reference correct options in the request.
type OptionSpec struct {
	Key  string
	Text string
}

// QuestionSpec is a question in a create/update request.
type QuestionSpec struct {
	Text        string
	Points      int
	TopicID     string
	Options     []OptionSpec
	CorrectKeys []string
}

// QuizSpec is the validated input for creating or replacing a draft.
type QuizSpec struct {
	Title        string
	CourseID     string
	TimeLimitSec int
	MaxAttempts  int
	ShowScore    bool
	Questions    []QuestionSpec
}

func runeLen(s string, minLen, maxLen int) bool {
	if !utf8.ValidString(s) {
		return false
	}
	n := utf8.RuneCountInString(s)
	return n >= minLen && n <= maxLen
}

// Validate checks every field and returns all problems at once.
func (s QuizSpec) Validate() error {
	var fe []FieldError
	add := func(f, m string) { fe = append(fe, FieldError{f, m}) }
	if !runeLen(strings.TrimSpace(s.Title), 1, MaxTitleLen) {
		add("title", fmt.Sprintf("must be 1..%d characters", MaxTitleLen))
	}
	if !opaqueID.MatchString(s.CourseID) {
		add("course_id", "must match [A-Za-z0-9_-]{1,64}")
	}
	if s.TimeLimitSec < MinTimeLimitSec || s.TimeLimitSec > MaxTimeLimitSec {
		add("time_limit_sec", fmt.Sprintf("must be within [%d, %d]", MinTimeLimitSec, MaxTimeLimitSec))
	}
	if s.MaxAttempts < 1 || s.MaxAttempts > MaxAttemptsLimit {
		add("max_attempts", fmt.Sprintf("must be within [1, %d]", MaxAttemptsLimit))
	}
	if len(s.Questions) < 1 || len(s.Questions) > MaxQuestions {
		add("questions", fmt.Sprintf("must contain 1..%d questions", MaxQuestions))
	}
	for i, q := range s.Questions {
		if i >= MaxQuestions {
			break
		}
		p := fmt.Sprintf("questions[%d]", i)
		if !runeLen(strings.TrimSpace(q.Text), 1, MaxQuestionLen) {
			add(p+".text", fmt.Sprintf("must be 1..%d characters", MaxQuestionLen))
		}
		if q.Points < MinPoints || q.Points > MaxPoints {
			add(p+".points", fmt.Sprintf("must be within [%d, %d]", MinPoints, MaxPoints))
		}
		if q.TopicID != "" && !opaqueID.MatchString(q.TopicID) {
			add(p+".topic_id", "must match [A-Za-z0-9_-]{1,64}")
		}
		if len(q.Options) < MinOptions || len(q.Options) > MaxOptions {
			add(p+".options", fmt.Sprintf("must contain %d..%d options", MinOptions, MaxOptions))
			continue
		}
		keys := map[string]bool{}
		for j, o := range q.Options {
			op := fmt.Sprintf("%s.options[%d]", p, j)
			if !keyRe.MatchString(o.Key) {
				add(op+".key", "must match [A-Za-z0-9_-]{1,16}")
			} else if keys[o.Key] {
				add(op+".key", "must be unique within the question")
			}
			keys[o.Key] = true
			if !runeLen(strings.TrimSpace(o.Text), 1, MaxOptionLen) {
				add(op+".text", fmt.Sprintf("must be 1..%d characters", MaxOptionLen))
			}
		}
		if len(q.CorrectKeys) < 1 || len(q.CorrectKeys) > len(q.Options) {
			add(p+".correct_keys", "must name at least one option")
		}
		seen := map[string]bool{}
		for _, k := range q.CorrectKeys {
			if !keys[k] {
				add(p+".correct_keys", "must reference option keys of this question")
				break
			}
			if seen[k] {
				add(p+".correct_keys", "must not repeat keys")
				break
			}
			seen[k] = true
		}
	}
	if len(fe) > 0 {
		return Invalid(fe)
	}
	return nil
}

// IDFunc generates a new id (UUIDv7) at a time.
type IDFunc func(time.Time) string

// BuildQuiz validates spec and materialises a draft quiz with ids and a single
// variant containing all questions in order.
func BuildQuiz(spec QuizSpec, id, ownerID string, now time.Time, newID IDFunc) (*Quiz, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	q := &Quiz{
		ID: id, OwnerID: ownerID, CourseID: spec.CourseID, Title: strings.TrimSpace(spec.Title),
		TimeLimitSec: spec.TimeLimitSec, MaxAttempts: spec.MaxAttempts, ShowScore: spec.ShowScore,
		Status: StatusDraft, CreatedAt: now, UpdatedAt: now,
	}
	for _, qs := range spec.Questions {
		correct := map[string]bool{}
		for _, k := range qs.CorrectKeys {
			correct[k] = true
		}
		question := Question{ID: newID(now), Text: strings.TrimSpace(qs.Text), Points: qs.Points, TopicID: qs.TopicID}
		for _, o := range qs.Options {
			question.Options = append(question.Options, Option{ID: newID(now), Text: strings.TrimSpace(o.Text), Correct: correct[o.Key]})
		}
		q.Questions = append(q.Questions, question)
	}
	q.Variants = []Variant{{ID: newID(now), Ordinal: 0, Questions: q.Questions}}
	return q, nil
}

// Publish makes the quiz immutable.
func (q *Quiz) Publish(now time.Time) error {
	if q.Status != StatusDraft {
		return ErrNotDraft
	}
	if len(q.Variants) == 0 {
		return InvalidField("variants", "the quiz has no variant")
	}
	for _, v := range q.Variants {
		if len(v.Questions) == 0 || v.MaxScore() <= 0 {
			return InvalidField("variants", "every variant needs at least one scored question")
		}
		for _, qs := range v.Questions {
			if qs.CorrectCount() == 0 {
				return InvalidField("questions", "every question needs a correct option")
			}
		}
	}
	q.Status = StatusPublished
	q.PublishedAt = &now
	q.UpdatedAt = now
	return nil
}

// VariantFor selects the variant for the n-th attempt (1-based), rotating
// through variants by ordinal.
func (q *Quiz) VariantFor(attemptNo int) Variant {
	return q.Variants[(attemptNo-1)%len(q.Variants)]
}

// Assignment opens a published quiz to a group within a time window.
type Assignment struct {
	QuizID     string
	GroupID    string
	OpensAt    time.Time
	ClosesAt   time.Time
	AssignedBy string
	AssignedAt time.Time
}

// Validate checks the window.
func (a Assignment) Validate() error {
	switch {
	case a.OpensAt.IsZero() || a.ClosesAt.IsZero():
		return InvalidField("opens_at", "opens_at and closes_at are required")
	case !a.ClosesAt.After(a.OpensAt):
		return InvalidField("closes_at", "must be after opens_at")
	case a.ClosesAt.Sub(a.OpensAt) > MaxWindow:
		return InvalidField("closes_at", "the window must not exceed 366 days")
	}
	return nil
}

// CheckOpen reports whether now lies within [OpensAt, ClosesAt).
func (a Assignment) CheckOpen(now time.Time) error {
	if now.Before(a.OpensAt) {
		return ErrNotOpen
	}
	if !now.Before(a.ClosesAt) {
		return ErrClosed
	}
	return nil
}
