package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/Newterios/Examory/sim/internal/events"
)

const (
	start  = "quiz.started"
	shown  = "question.shown"
	answer = "quiz.answered"
	submit = "quiz.submitted"
)

type action struct {
	kind, attempt, question string
	at                      int64
	option                  int
	change                  bool
}

type sourceAttempt struct {
	user, group string
	score, max  int
}

type world struct {
	attempts              map[string]*sourceAttempt
	quiz, variant, course string
	questionIDs           []string
	keys                  map[string]int
	actions               []action
	digest, keyDigest     string
}

type payload struct {
	Attempt    string `json:"attempt_id"`
	Question   string `json:"question_id"`
	Index      int    `json:"question_index"`
	SinceShown int64  `json:"time_since_shown_ms"`
	Hash       string `json:"answer_hash"`
	Change     bool   `json:"is_change"`
	Quiz       string `json:"quiz_id"`
	Variant    string `json:"variant_id"`
	Score      int    `json:"score"`
	Max        int    `json:"max_score"`
	Duration   int64  `json:"duration_ms"`
}

func loadWorld(path, keyPath string) (*world, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return parseWorld(raw, keyBytes)
}

func parseWorld(raw, keyBytes []byte) (*world, error) {
	w := &world{attempts: map[string]*sourceAttempt{}, digest: fmt.Sprintf("%x", sha256.Sum256(raw)), keyDigest: fmt.Sprintf("%x", sha256.Sum256(keyBytes))}
	var indices map[string]*int
	if err := json.Unmarshal(keyBytes, &indices); err != nil {
		return nil, fmt.Errorf("read item keys: %w", err)
	}
	w.keys = make(map[string]int, len(indices))
	for q, k := range indices {
		if k == nil {
			return nil, errors.New("null marking key")
		}
		w.keys[q] = *k
	}
	if len(w.keys) < 1 || len(w.keys) > 100 {
		return nil, errors.New("a fixed quiz needs 1..100 keys")
	}
	w.questionIDs = make([]string, len(w.keys))
	keyHashes := map[string]string{}
	options := map[string]int{}
	for i := 0; i < 4; i++ {
		options[events.AnswerHash(i)] = i
	}
	for q, k := range w.keys {
		if k < 0 || k >= 4 || q == "" {
			return nil, errors.New("invalid question ID or option key")
		}
		keyHashes[q] = events.AnswerHash(k)
	}
	keysByAttempt := events.Keys{}
	userGroup := map[string]string{}
	attemptsPerUser := map[string]int{}
	type itemState struct {
		shownAt int64
		choice  int
	}
	lastAnswer := map[[2]string]itemState{}
	reader := csv.NewReader(bytes.NewReader(raw))
	hdr, err := reader.Read()
	if err != nil || !slices.Equal(hdr, events.Columns) {
		return nil, errors.New("expected simulator CSV event header")
	}
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var p payload
		var actor struct {
			User string `json:"user_id"`
		}
		var cx struct {
			Group  string `json:"group_id"`
			Course string `json:"course_id"`
		}
		if json.Unmarshal([]byte(rec[7]), &p) != nil || json.Unmarshal([]byte(rec[5]), &actor) != nil || json.Unmarshal([]byte(rec[6]), &cx) != nil {
			return nil, errors.New("invalid simulator event payload, actor or context")
		}
		at, err := time.Parse(time.RFC3339Nano, rec[3])
		if err != nil {
			return nil, err
		}
		ms := at.UnixMilli()
		if w.course != "" && w.course != cx.Course {
			return nil, errors.New("world must use one course")
		}
		if cx.Course == "" {
			return nil, errors.New("missing course")
		}
		w.course = cx.Course
		a := w.attempts[p.Attempt]
		if a == nil {
			a = &sourceAttempt{user: actor.User, group: cx.Group}
			w.attempts[p.Attempt] = a
			if group, known := userGroup[actor.User]; known && group != cx.Group {
				return nil, errors.New("student belongs to multiple groups")
			}
			userGroup[actor.User] = cx.Group
			attemptsPerUser[actor.User]++
			if attemptsPerUser[actor.User] > 20 {
				return nil, errors.New("world exceeds quiz limit of 20 attempts per student")
			}
			keysByAttempt[p.Attempt] = keyHashes
		}
		switch rec[1] {
		case submit:
			if w.quiz != "" && (p.Quiz != w.quiz || p.Variant != w.variant) {
				return nil, errors.New("world must contain one quiz and variant")
			}
			if p.Quiz == "" || p.Variant == "" || p.Max != len(w.keys) || p.Duration > 4*time.Hour.Milliseconds() {
				return nil, errors.New("expected a fixed roster without truncation, a variant ID and duration at most 4h")
			}
			w.quiz, w.variant = p.Quiz, p.Variant
			a.score, a.max = p.Score, p.Max
			w.actions = append(w.actions, action{kind: start, attempt: p.Attempt, at: ms - p.Duration}, action{kind: submit, attempt: p.Attempt, at: ms})
		case answer:
			_, keyOK := w.keys[p.Question]
			option, hashOK := options[p.Hash]
			if !keyOK || !hashOK || p.Index < 0 || p.Index >= len(w.keys) {
				return nil, errors.New("unknown item, option hash or question index")
			}
			if prior := w.questionIDs[p.Index]; prior != "" && prior != p.Question {
				return nil, errors.New("question roster or order differs within world")
			}
			w.questionIDs[p.Index] = p.Question
			item := [2]string{p.Attempt, p.Question}
			shownAt := ms - p.SinceShown
			previous, answered := lastAnswer[item]
			if p.Change != answered || (answered && (previous.shownAt != shownAt || previous.choice == option)) {
				return nil, errors.New("inconsistent question display time or repeated unchanged answer")
			}
			lastAnswer[item] = itemState{shownAt, option}
			if !p.Change {
				w.actions = append(w.actions, action{kind: shown, attempt: p.Attempt, question: p.Question, at: ms - p.SinceShown})
			}
			w.actions = append(w.actions, action{kind: answer, attempt: p.Attempt, question: p.Question, at: ms, option: option, change: p.Change})
		default:
			return nil, errors.New("unrecognized world event")
		}
	}
	// Reuse the simulator's complete invariants, including keys versus final answers.
	if _, err = events.Validate(bytes.NewReader(raw), keysByAttempt); err != nil {
		return nil, err
	}
	priority := map[string]int{start: 0, shown: 1, answer: 2, submit: 3}
	sort.SliceStable(w.actions, func(i, j int) bool {
		a, b := w.actions[i], w.actions[j]
		if a.at != b.at {
			return a.at < b.at
		}
		if priority[a.kind] != priority[b.kind] {
			return priority[a.kind] < priority[b.kind]
		}
		return a.attempt < b.attempt
	})
	return w, nil
}

// loadPrefix ends exactly on the event budget. A shown request never emits an event.
func loadPrefix(as []action, budget int) ([]action, error) {
	for i, a := range as {
		if a.kind != shown {
			budget--
			if budget == 0 {
				return as[:i+1], nil
			}
		}
	}
	return nil, errors.New("world has fewer quiz events than the requested load budget")
}
