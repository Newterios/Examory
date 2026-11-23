package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// Attempt statuses. The only transitions are
// in_progress -> submitted and in_progress -> timed_out.
const (
	AttemptInProgress = "in_progress"
	AttemptSubmitted  = "submitted"
	AttemptTimedOut   = "timed_out"
)

// Answer is the current saved answer to a question.
type Answer struct {
	Selected []string // canonical: lowercase option ids, sorted, unique
	Hash     string
	SavedAt  time.Time
	Changes  int // number of changes after the first save
}

// AttemptQuestion is a question as seen inside one attempt.
type AttemptQuestion struct {
	QuestionID   string
	Index        int // zero-based position in the variant (events.md question_index)
	Text         string
	Points       int
	Options      []Option // bank order; use DisplayOrder for presentation
	FirstShownAt *time.Time
	Answer       *Answer
}

// Attempt is the attempt aggregate.
type Attempt struct {
	ID             string
	QuizID         string
	VariantID      string
	UserID         string
	GroupID        string
	CourseID       string
	AttemptNo      int
	Status         string
	Sim            bool // started under the simulation clock
	ShuffleSeed    []byte
	TimeLimitSec   int
	ShowScore      bool
	StartedAt      time.Time
	DeadlineAt     time.Time
	LastActivityAt time.Time
	FinishedAt     *time.Time
	Score          *int
	MaxScore       *int
	AnswersChanged int
	Questions      []AttemptQuestion
}

// NewAttempt creates an in-progress attempt of variant v. The deadline is
// now+limit, capped at the assignment's close time.
func NewAttempt(id string, q *Quiz, v Variant, a Assignment, userID string, attemptNo int, seed []byte, now time.Time, sim bool) *Attempt {
	deadline := now.Add(time.Duration(q.TimeLimitSec) * time.Second)
	if a.ClosesAt.Before(deadline) {
		deadline = a.ClosesAt
	}
	at := &Attempt{
		ID: id, QuizID: q.ID, VariantID: v.ID, UserID: userID, GroupID: a.GroupID, CourseID: q.CourseID,
		AttemptNo: attemptNo, Status: AttemptInProgress, Sim: sim, ShuffleSeed: seed,
		TimeLimitSec: q.TimeLimitSec, ShowScore: q.ShowScore,
		StartedAt: now, DeadlineAt: deadline, LastActivityAt: now,
	}
	for i, qs := range v.Questions {
		at.Questions = append(at.Questions, AttemptQuestion{QuestionID: qs.ID, Index: i, Text: qs.Text, Points: qs.Points, Options: qs.Options})
	}
	return at
}

// Clock describes the time source of a request.
type Clock struct {
	Now time.Time
	Sim bool
}

// Effective validates the request time against the attempt and returns the
// instant to use. Simulated time must be monotone per attempt; real time is
// clamped to the last activity (protects against small clock differences
// between replicas so that timings never go negative).
func (a *Attempt) Effective(c Clock) (time.Time, error) {
	if c.Sim != a.Sim {
		return time.Time{}, ErrClockMismatch
	}
	if c.Now.Before(a.LastActivityAt) {
		if c.Sim {
			return time.Time{}, ErrNonMonotonicTime
		}
		return a.LastActivityAt, nil
	}
	return c.Now, nil
}

// Expired reports whether the attempt is past its deadline plus grace.
func (a *Attempt) Expired(now time.Time, grace time.Duration) bool {
	return a.Status == AttemptInProgress && now.After(a.DeadlineAt.Add(grace))
}

func (a *Attempt) question(id string) (*AttemptQuestion, bool) {
	for i := range a.Questions {
		if a.Questions[i].QuestionID == id {
			return &a.Questions[i], true
		}
	}
	return nil, false
}

func (a *Attempt) checkActive(now time.Time, grace time.Duration) error {
	if a.Status != AttemptInProgress {
		return ErrAttemptFinalized
	}
	if a.Expired(now, grace) {
		return ErrTimeOver
	}
	return nil
}

// MarkShown records the first time a question was displayed. It is
// idempotent: later calls keep the first time and report changed=false.
func (a *Attempt) MarkShown(questionID string, now time.Time, grace time.Duration) (changed bool, err error) {
	if err := a.checkActive(now, grace); err != nil {
		return false, err
	}
	q, ok := a.question(questionID)
	if !ok {
		return false, ErrNotFound
	}
	if q.FirstShownAt != nil {
		return false, nil
	}
	t := now
	q.FirstShownAt = &t
	a.LastActivityAt = now
	return true, nil
}

// AnswerResult describes the effect of SaveAnswer.
type AnswerResult struct {
	Noop                    bool // identical to the current answer: nothing saved, no event
	QuestionIndex           int
	Hash                    string
	Selected                []string
	IsChange                bool
	TimeSinceShownMs        int64
	TimeSinceAttemptStartMs int64
	SavedAt                 time.Time
}

// SaveAnswer validates and stores an answer, computing the timing fields of
// quiz.answered from server time (events.md §3 counting conventions).
func (a *Attempt) SaveAnswer(questionID string, selected []string, now time.Time, grace time.Duration) (AnswerResult, error) {
	if err := a.checkActive(now, grace); err != nil {
		return AnswerResult{}, err
	}
	q, ok := a.question(questionID)
	if !ok {
		return AnswerResult{}, ErrNotFound
	}
	canon, err := canonicalSelection(q.Options, selected)
	if err != nil {
		return AnswerResult{}, err
	}
	if q.FirstShownAt == nil {
		// Timing is never fabricated: a question must be shown first.
		return AnswerResult{}, ErrNotShown
	}
	hash := AnswerHash(canon)
	if q.Answer != nil && q.Answer.Hash == hash {
		return AnswerResult{Noop: true, QuestionIndex: q.Index, Hash: hash, Selected: canon, SavedAt: q.Answer.SavedAt}, nil
	}
	res := AnswerResult{
		QuestionIndex:           q.Index,
		Hash:                    hash,
		Selected:                canon,
		IsChange:                q.Answer != nil,
		TimeSinceShownMs:        nonNegMs(now.Sub(*q.FirstShownAt)),
		TimeSinceAttemptStartMs: nonNegMs(now.Sub(a.StartedAt)),
		SavedAt:                 now,
	}
	changes := 0
	if res.IsChange {
		changes = q.Answer.Changes + 1
		a.AnswersChanged++
	}
	q.Answer = &Answer{Selected: canon, Hash: hash, SavedAt: now, Changes: changes}
	a.LastActivityAt = now
	return res, nil
}

func nonNegMs(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}

// canonicalSelection validates option ids against the question and returns
// them lowercased, sorted and unique.
func canonicalSelection(opts []Option, selected []string) ([]string, error) {
	if len(selected) < 1 || len(selected) > len(opts) {
		return nil, InvalidField("selected_option_ids", "must select between 1 and the number of options")
	}
	valid := map[string]bool{}
	for _, o := range opts {
		valid[strings.ToLower(o.ID)] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(selected))
	for _, s := range selected {
		id := strings.ToLower(strings.TrimSpace(s))
		if !valid[id] {
			return nil, InvalidField("selected_option_ids", "contains an option that does not belong to the question")
		}
		if seen[id] {
			return nil, InvalidField("selected_option_ids", "must not contain duplicates")
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// AnswerHash is the events.md answer_hash: SHA-256 (lowercase hex) of the
// canonical normalized answer. For choice questions the canonical form is
// the selected option ids, each trimmed and case-folded, sorted
// lexicographically, de-duplicated and joined with "\n". Option ids are
// stable per question and identical for every student (they are not the
// per-attempt shuffled labels), so equal answers to the same question have
// equal hashes across students, and the hash contains no student, attempt
// or question identifier.
func AnswerHash(optionIDs []string) string {
	norm := make([]string, 0, len(optionIDs))
	seen := map[string]bool{}
	for _, id := range optionIDs {
		n := strings.ToLower(strings.TrimSpace(id))
		if !seen[n] {
			seen[n] = true
			norm = append(norm, n)
		}
	}
	sort.Strings(norm)
	sum := sha256.Sum256([]byte(strings.Join(norm, "\n")))
	return hex.EncodeToString(sum[:])
}

// Finalization records how and when an attempt ended.
type Finalization struct {
	AttemptID      string
	UserID         string
	Reason         string // AttemptSubmitted or AttemptTimedOut
	IdempotencyKey string // "" for timeouts
	FinalizedAt    time.Time
	EventID        string
}

// FinalResult is the outcome of Finalize.
type FinalResult struct {
	Status         string
	FinishedAt     time.Time
	Score          int
	MaxScore       int
	DurationMs     int64
	AnswersChanged int
}

// Finalize moves the attempt out of in_progress and scores it. A submit
// arriving after deadline+grace becomes a timeout. Timeouts finish at the
// deadline (the logical end of the attempt) so duration_ms never exceeds the
// time actually available.
func (a *Attempt) Finalize(reason string, now time.Time, grace time.Duration) (FinalResult, error) {
	if a.Status != AttemptInProgress {
		return FinalResult{}, ErrAttemptFinalized
	}
	finished := now
	status := reason
	if reason == AttemptSubmitted && a.Expired(now, grace) {
		status = AttemptTimedOut
	}
	if status == AttemptTimedOut && finished.After(a.DeadlineAt) {
		finished = a.DeadlineAt
	}
	if finished.Before(a.LastActivityAt) {
		finished = a.LastActivityAt
	}
	score, maxScore := a.score()
	a.Status = status
	a.FinishedAt = &finished
	a.Score, a.MaxScore = &score, &maxScore
	a.LastActivityAt = finished
	return FinalResult{
		Status: status, FinishedAt: finished, Score: score, MaxScore: maxScore,
		DurationMs: nonNegMs(finished.Sub(a.StartedAt)), AnswersChanged: a.AnswersChanged,
	}, nil
}

// score is all-or-nothing per question: full points iff the selected set
// equals the set of correct options.
func (a *Attempt) score() (score, maxScore int) {
	for _, q := range a.Questions {
		maxScore += q.Points
		if q.Answer == nil {
			continue
		}
		var correct []string
		for _, o := range q.Options {
			if o.Correct {
				correct = append(correct, strings.ToLower(o.ID))
			}
		}
		sort.Strings(correct)
		if strings.Join(correct, "\n") == strings.Join(q.Answer.Selected, "\n") {
			score += q.Points
		}
	}
	return score, maxScore
}

// DisplayOrder returns the question's options in the per-attempt
// deterministic shuffled order: options sorted by SHA-256(seed || question id
// || option id). The order is stable for the attempt and differs between
// attempts; it does not depend on PRNG implementation details.
func (a *Attempt) DisplayOrder(q AttemptQuestion) []Option {
	type keyed struct {
		k []byte
		o Option
	}
	ks := make([]keyed, len(q.Options))
	for i, o := range q.Options {
		h := sha256.New()
		h.Write(a.ShuffleSeed)
		h.Write([]byte(q.QuestionID))
		h.Write([]byte(o.ID))
		ks[i] = keyed{h.Sum(nil), o}
	}
	sort.Slice(ks, func(i, j int) bool { return bytes.Compare(ks[i].k, ks[j].k) < 0 })
	out := make([]Option, len(ks))
	for i, k := range ks {
		out[i] = k.o
	}
	return out
}
