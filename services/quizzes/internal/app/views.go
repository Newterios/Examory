package app

import (
	"time"

	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

// AttemptView is the student-facing attempt. It deliberately has no field
// that could carry the marking key: options expose only id and text.
type AttemptView struct {
	ID          string         `json:"id"`
	QuizID      string         `json:"quiz_id"`
	VariantID   string         `json:"variant_id"`
	AttemptNo   int            `json:"attempt_no"`
	Status      string         `json:"status"`
	StartedAt   time.Time      `json:"started_at"`
	DeadlineAt  time.Time      `json:"deadline_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	RemainingMs int64          `json:"remaining_ms"`
	ServerTime  time.Time      `json:"server_time"`
	Questions   []QuestionView `json:"questions"`
	Result      *Result        `json:"result"`
}

// QuestionView is a question inside an attempt.
type QuestionView struct {
	ID                string       `json:"id"`
	Index             int          `json:"index"`
	Text              string       `json:"text"`
	Points            int          `json:"points"`
	Selection         string       `json:"selection"` // "single" | "multiple"
	Shown             bool         `json:"shown"`
	Options           []OptionView `json:"options"`
	SelectedOptionIDs []string     `json:"selected_option_ids"`
}

// OptionView is an answer option without correctness.
type OptionView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func studentView(at *domain.Attempt, now time.Time) *AttemptView {
	v := &AttemptView{
		ID: at.ID, QuizID: at.QuizID, VariantID: at.VariantID, AttemptNo: at.AttemptNo, Status: at.Status,
		StartedAt: at.StartedAt, DeadlineAt: at.DeadlineAt, FinishedAt: at.FinishedAt, ServerTime: now,
		Questions: make([]QuestionView, 0, len(at.Questions)),
	}
	if at.Status == domain.AttemptInProgress {
		if rem := at.DeadlineAt.Sub(now).Milliseconds(); rem > 0 {
			v.RemainingMs = rem
		}
	} else {
		v.Result = resultOf(at)
	}
	for _, q := range at.Questions {
		qv := QuestionView{ID: q.QuestionID, Index: q.Index, Text: q.Text, Points: q.Points, Selection: "single",
			Shown: q.FirstShownAt != nil, SelectedOptionIDs: []string{}}
		correct := 0
		for _, o := range q.Options {
			if o.Correct {
				correct++
			}
		}
		if correct > 1 {
			qv.Selection = "multiple"
		}
		for _, o := range at.DisplayOrder(q) {
			qv.Options = append(qv.Options, OptionView{ID: o.ID, Text: o.Text})
		}
		if q.Answer != nil {
			qv.SelectedOptionIDs = q.Answer.Selected
		}
		v.Questions = append(v.Questions, qv)
	}
	return v
}
