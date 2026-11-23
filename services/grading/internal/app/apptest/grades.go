package apptest

import (
	"context"
	"github.com/Newterios/Examory/services/grading/internal/domain"
)

func (t *tx) LockGrade(ctx context.Context, id, teacher, key string) (*domain.AttemptSummary, error) {
	return t.GetAttempt(ctx, id)
}
func (t *tx) GradeByKey(_ context.Context, teacher, key string) (*domain.Grade, error) {
	for _, g := range t.st.s.Grades {
		if g.GradedBy == teacher && g.Key == key {
			return &g, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (t *tx) LastGradeRevision(_ context.Context, id string) (int, error) {
	n := 0
	for _, g := range t.st.s.Grades {
		if g.AttemptID == id {
			n++
		}
	}
	return n, nil
}
func (t *tx) InsertGrade(_ context.Context, g domain.Grade) error {
	t.st.s.Grades = append(t.st.s.Grades, g)
	return nil
}
func (t *tx) Grades(_ context.Context, id string, history bool) ([]domain.Grade, error) {
	var out []domain.Grade
	for i := len(t.st.s.Grades) - 1; i >= 0; i-- {
		g := t.st.s.Grades[i]
		if g.AttemptID == id {
			out = append(out, g)
			if !history {
				break
			}
		}
	}
	return out, nil
}
