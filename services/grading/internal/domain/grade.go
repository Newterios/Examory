package domain

import (
	"math"
	"time"
	"unicode/utf8"
)

type CriterionScore struct {
	ID    string  `json:"criterion_id"`
	Score float64 `json:"score"`
}
type GradeInput struct {
	Grade    *float64         `json:"grade"`
	Feedback string           `json:"feedback"`
	Criteria []CriterionScore `json:"rubric_scores"`
}
type Grade struct {
	AttemptID string           `json:"attempt_id"`
	Revision  int              `json:"revision"`
	Grade     float64          `json:"grade"`
	MaxGrade  int              `json:"max_grade"`
	Feedback  string           `json:"feedback"`
	Criteria  []CriterionScore `json:"rubric_scores"`
	GradedBy  string           `json:"graded_by"`
	At        time.Time        `json:"graded_at"`
	Key       string           `json:"-"`
	EventID   string           `json:"-"`
}

// The attempt-total rubric has one optional criterion bounded by the authoritative quiz maximum.
func (in GradeInput) Validate(maximum int) error {
	if maximum <= 0 || in.Grade == nil || math.IsNaN(*in.Grade) || math.IsInf(*in.Grade, 0) || *in.Grade < 0 || *in.Grade > float64(maximum) {
		return Invalid("grade_bounds", "grade must be within the attempt's score bounds")
	}
	if len(in.Criteria) > 1 || (len(in.Criteria) == 1 && (in.Criteria[0].ID != "total" || in.Criteria[0].Score != *in.Grade)) {
		return Invalid("criterion_bounds", "the optional total criterion must equal the grade")
	}
	if !utf8.ValidString(in.Feedback) || utf8.RuneCountInString(in.Feedback) > 2000 {
		return Invalid("invalid_feedback", "feedback exceeds 2000 characters")
	}
	return nil
}
