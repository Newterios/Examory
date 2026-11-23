package app_test

import (
	"errors"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
	"testing"
)

func TestManualGradeIsIndependentOfReview(t *testing.T) {
	f := newFx(t)
	pk, attempt, _ := f.seed()
	owner := f.student
	owner.UserID = f.store.Snapshot().Attempts[attempt].StudentID
	owner.GroupID = f.group
	n := 0.
	in := domain.GradeInput{Grade: &n, Criteria: []domain.CriterionScore{{ID: "total", Score: 0}}}
	for _, p := range []authn.Principal{owner, f.admin} {
		if _, _, err := f.svc.SetGrade(f.ctx, p, attempt, "grade-key-01", in); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("role accepted %v", err)
		}
	}
	for _, bad := range []float64{-1, 21} {
		n = bad
		if _, _, err := f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-bound", in); err == nil {
			t.Fatal("bounds")
		}
	}
	n = 0.
	g, _, err := f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-key-01", in)
	if err != nil || g.Revision != 1 {
		t.Fatal(g, err)
	}
	if _, err := f.svc.Review(f.ctx, f.teacher, pk, "confirmed", "Separate decision", 1, "review-grade-key"); err != nil {
		t.Fatal(err)
	}
	if replay, ok, err := f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-key-01", in); err != nil || !ok || replay.Revision != 1 {
		t.Fatal(replay, ok, err)
	}
	n = 12.5
	if _, _, err = f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-badcriteria", in); err == nil {
		t.Fatal("inconsistent criterion")
	}
	in.Criteria = nil
	if _, _, err = f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-key-01", in); !errors.Is(err, domain.ErrIdempotencyReuse) {
		t.Fatal(err)
	}
	if _, _, err = f.svc.SetGrade(f.ctx, f.teacher, attempt, "grade-key-02", in); err != nil {
		t.Fatal(err)
	}
	history, err := f.svc.GetGrades(f.ctx, f.teacher, attempt, true)
	if err != nil || len(history) != 2 || history[1].Grade != 0 {
		t.Fatal(history, err)
	}
	row, err := f.svc.GetGrades(f.ctx, owner, attempt, false)
	if err != nil || len(row) != 1 || row[0].Grade != 12.5 {
		t.Fatal(row, err)
	}
	owner.UserID = ids.NewV7()
	if _, err = f.svc.GetGrades(f.ctx, owner, attempt, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	snap := f.store.Snapshot()
	var grades, audits int
	for _, ev := range snap.Outbox {
		if ev.Env.EventType == "grade.assigned" {
			grades++
			if ev.Key != f.course || ev.Topic != events.TopicGrades {
				t.Fatal(ev)
			}
		}
	}
	for _, a := range snap.Audit {
		if a.Action == "grade.assigned" {
			audits++
		}
	}
	if grades != 2 || audits != 2 || snap.Attempts[attempt].Score != 18 {
		t.Fatalf("grade count=%d audit=%d", grades, audits)
	}
}
