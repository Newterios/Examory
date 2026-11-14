package events

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Keys maps attempt_id -> question_id -> answer_hash of the correct option.
// When supplied to Validate, score is checked against final answers.
type Keys map[string]map[string]string

// Summary describes a validated event log.
type Summary struct {
	Events   int
	Attempts int
	ByType   map[string]int // attempts per anomaly_type
	Changes  int
}

var timeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type answer struct {
	QuestionID    string `json:"question_id"`
	QuestionIndex *int   `json:"question_index"`
	SinceShown    *int64 `json:"time_since_shown_ms"`
	IsChange      *bool  `json:"is_change"`
	Elapsed       *int64 `json:"time_since_attempt_start_ms"`
	AnswerHash    string `json:"answer_hash"`
}

type submit struct {
	QuizID    string `json:"quiz_id"`
	VariantID string `json:"variant_id"`
	Score     *int   `json:"score"`
	MaxScore  *int   `json:"max_score"`
	Duration  *int64 `json:"duration_ms"`
	Changes   *int   `json:"answers_changed"`
}

type vEvent struct {
	ms      int64
	ans     *answer
	actor   string
	context string
	label   string
	kind    string
}

type vAttempt struct {
	answers []vEvent
	submits []vEvent
	sub     *submit
}

var answerKeys = []string{"answer_hash", "attempt_id", "is_change", "question_id", "question_index", "time_since_attempt_start_ms", "time_since_shown_ms"}
var submitKeys = []string{"answers_changed", "attempt_id", "duration_ms", "max_score", "quiz_id", "score", "variant_id"}

func keySet(raw map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(raw))
	for k := range raw {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func canonicalObject(cell string) (string, error) {
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(cell))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil || m == nil {
		return "", errors.New("cell must be a JSON object")
	}
	b, err := json.Marshal(m) // sorted keys: canonical for dict equality
	return string(b), err
}

// Validate re-implements the invariants of ml/anomaly/features.py
// read_attempts plus the stricter generator invariants of the simulator.
func Validate(r io.Reader, keys Keys) (*Summary, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = false
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if !equalStrings(header, Columns) {
		return nil, fmt.Errorf("header %v differs from %v", header, Columns)
	}
	attempts := map[string]*vAttempt{}
	var order []string
	seen := map[string]bool{}
	var prevMs int64 = -1 << 62
	var prevID string
	sum := &Summary{ByType: map[string]int{}}
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		fail := func(f string, a ...any) error { return fmt.Errorf("line %d: %s", line, fmt.Sprintf(f, a...)) }
		id, typ, version, occurred, producer := rec[0], rec[1], rec[2], rec[3], rec[4]
		if typ != "quiz.answered" && typ != "quiz.submitted" {
			return nil, fail("unexpected event_type %q", typ)
		}
		if version != "1" || producer != Producer {
			return nil, fail("unsupported version/producer")
		}
		if seen[id] {
			return nil, fail("duplicate event_id %s", id)
		}
		seen[id] = true
		if !timeRe.MatchString(occurred) {
			return nil, fail("occurred_at %q is not RFC 3339 UTC with milliseconds", occurred)
		}
		t, err := time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, fail("occurred_at: %v", err)
		}
		ms := t.UnixMilli()
		if !uuidRe.MatchString(id) {
			return nil, fail("event_id %q is not a lowercase UUIDv7", id)
		}
		var tsHex = strings.ReplaceAll(id[:13], "-", "")
		var ts int64
		if _, err := fmt.Sscanf(tsHex, "%x", &ts); err != nil || ts != ms {
			return nil, fail("UUIDv7 timestamp differs from occurred_at")
		}
		if ms < prevMs || (ms == prevMs && id < prevID) {
			return nil, fail("rows not sorted by (occurred_at, event_id)")
		}
		prevMs, prevID = ms, id
		actor, err := canonicalObject(rec[5])
		if err != nil {
			return nil, fail("actor: %v", err)
		}
		context, err := canonicalObject(rec[6])
		if err != nil {
			return nil, fail("context: %v", err)
		}
		var ac struct {
			UserID string `json:"user_id"`
		}
		var cx struct {
			GroupID string `json:"group_id"`
		}
		_ = json.Unmarshal([]byte(rec[5]), &ac)
		_ = json.Unmarshal([]byte(rec[6]), &cx)
		if ac.UserID == "" || cx.GroupID == "" {
			return nil, fail("group_id and user_id are required")
		}
		label, kind := rec[8], rec[9]
		if label != "0" && label != "1" {
			return nil, fail("is_anomaly must be 0 or 1")
		}
		if (label == "0") != (kind == "none") {
			return nil, fail("inconsistent label/type")
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(rec[7]), &raw); err != nil || raw == nil {
			return nil, fail("payload must be a JSON object")
		}
		var attemptID string
		if err := json.Unmarshal(raw["attempt_id"], &attemptID); err != nil || attemptID == "" {
			return nil, fail("payload.attempt_id missing")
		}
		at := attempts[attemptID]
		if at == nil {
			at = &vAttempt{}
			attempts[attemptID] = at
			order = append(order, attemptID)
		}
		ev := vEvent{ms: ms, actor: actor, context: context, label: label, kind: kind}
		dec := func(dst any) error {
			d := json.NewDecoder(bytes.NewReader([]byte(rec[7])))
			d.DisallowUnknownFields()
			return d.Decode(dst)
		}
		if typ == "quiz.answered" {
			if !equalStrings(keySet(raw), answerKeys) {
				return nil, fail("quiz.answered payload keys %v", keySet(raw))
			}
			var a struct {
				answer
				AttemptID string `json:"attempt_id"`
			}
			if err := dec(&a); err != nil {
				return nil, fail("quiz.answered payload: %v", err)
			}
			if a.QuestionIndex == nil || a.SinceShown == nil || a.IsChange == nil || a.Elapsed == nil || *a.QuestionIndex < 0 {
				return nil, fail("invalid is_change/question_index/timing types")
			}
			if !hashRe.MatchString(a.AnswerHash) {
				return nil, fail("answer_hash must be lowercase SHA-256")
			}
			ans := a.answer
			ev.ans = &ans
			at.answers = append(at.answers, ev)
		} else {
			if !equalStrings(keySet(raw), submitKeys) {
				return nil, fail("quiz.submitted payload keys %v", keySet(raw))
			}
			var s struct {
				submit
				AttemptID string `json:"attempt_id"`
			}
			if err := dec(&s); err != nil {
				return nil, fail("quiz.submitted payload: %v", err)
			}
			if s.Score == nil || s.MaxScore == nil || s.Duration == nil || s.Changes == nil {
				return nil, fail("quiz.submitted payload has null fields")
			}
			sub := s.submit
			at.sub = &sub
			at.submits = append(at.submits, ev)
		}
		sum.Events++
	}
	for _, id := range order {
		if err := checkAttempt(id, attempts[id], keys, sum); err != nil {
			return nil, err
		}
	}
	if len(order) == 0 {
		return nil, errors.New("no complete quiz attempts")
	}
	sum.Attempts = len(order)
	return sum, nil
}

func checkAttempt(id string, at *vAttempt, keys Keys, sum *Summary) error {
	fail := func(f string, a ...any) error { return fmt.Errorf("attempt %s: %s", id, fmt.Sprintf(f, a...)) }
	if len(at.submits) != 1 || len(at.answers) == 0 {
		return fail("need one submission and answered questions")
	}
	sub, se := at.sub, at.submits[0]
	for _, e := range at.answers {
		if e.actor != se.actor || e.context != se.context || e.label != se.label || e.kind != se.kind {
			return fail("inconsistent identity/context/labels")
		}
		if e.ms > se.ms {
			return fail("answer after submission")
		}
	}
	if !(*sub.Score >= 0 && *sub.Score <= *sub.MaxScore && *sub.MaxScore > 0) {
		return fail("invalid score/max_score")
	}
	if *sub.Duration <= 0 {
		return fail("duration_ms must be positive")
	}
	answers := append([]vEvent(nil), at.answers...)
	sort.SliceStable(answers, func(i, j int) bool {
		if *answers[i].ans.Elapsed != *answers[j].ans.Elapsed {
			return *answers[i].ans.Elapsed < *answers[j].ans.Elapsed
		}
		return answers[i].ms < answers[j].ms
	})
	type qs struct {
		index  int
		times  []int64
		change []bool
		final  string
	}
	perQ := map[string]*qs{}
	var qorder []string
	changes := 0
	for _, e := range answers {
		a := e.ans
		el, sh := *a.Elapsed, *a.SinceShown
		if !(0 <= sh && sh <= el && el <= *sub.Duration) {
			return fail("invalid answer timing")
		}
		if e.ms != se.ms-(*sub.Duration-el) {
			return fail("answer timestamp disagrees with elapsed time")
		}
		q := perQ[a.QuestionID]
		if q == nil {
			q = &qs{index: *a.QuestionIndex}
			perQ[a.QuestionID] = q
			qorder = append(qorder, a.QuestionID)
		}
		if q.index != *a.QuestionIndex {
			return fail("question index changes within attempt")
		}
		q.times = append(q.times, sh)
		q.change = append(q.change, *a.IsChange)
		q.final = a.AnswerHash
		if *a.IsChange {
			changes++
		}
	}
	indices := make([]int, 0, len(perQ))
	for _, qid := range qorder {
		q := perQ[qid]
		if q.change[0] {
			return fail("question %s: first event is a change", qid)
		}
		for j := 1; j < len(q.change); j++ {
			if !q.change[j] {
				return fail("question %s: second initial answer", qid)
			}
			if q.times[j] < q.times[j-1] {
				return fail("question %s: cumulative question time decreases", qid)
			}
			// Stricter than read_attempts: equal times would make the
			// first-answer/change order depend on event_id tie-breaking.
			if q.times[j] == q.times[j-1] {
				return fail("question %s: two answers at the same time", qid)
			}
		}
		indices = append(indices, q.index)
	}
	sort.Ints(indices)
	for i, v := range indices {
		if v != i {
			return fail("question indices are not 0..n-1")
		}
	}
	if len(indices) != *sub.MaxScore {
		return fail("max_score %d differs from %d questions", *sub.MaxScore, len(indices))
	}
	if changes != *sub.Changes {
		return fail("answers_changed %d differs from %d change events", *sub.Changes, changes)
	}
	if keys != nil {
		k, ok := keys[id]
		if !ok {
			return fail("no answer keys")
		}
		score := 0
		for qid, q := range perQ {
			if k[qid] == q.final {
				score++
			}
		}
		if score != *sub.Score {
			return fail("score %d differs from %d correct final answers", *sub.Score, score)
		}
	}
	sum.ByType[se.kind]++
	sum.Changes += changes
	return nil
}
