package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

func TestRevokedEnrollmentRejectsAttemptReadsAndReplays(t *testing.T) {
	f := newFixture(t)
	q := f.publishedQuiz()
	a := f.actor(f.studentA)
	at, err := f.svc.StartAttempt(f.ctx, a, q.ID)
	must(t, err)
	allowed := true
	authority := courseaccess.CheckFunc(func(_ context.Context, course, teacher, group string) (bool, error) {
		return allowed && course == q.CourseID && group == f.group1, nil
	})
	f.svc = app.New(f.store, f.clk, logging.Discard(), app.Config{Access: authority})
	allowed = false
	_, err = f.svc.GetAttempt(f.ctx, a, at.ID)
	wantErr(t, err, domain.ErrNotFound)
	_, err = f.svc.Submit(f.ctx, a, at.ID, "submit-scoped-001")
	wantErr(t, err, domain.ErrNotFound)
	allowed = true
	_, err = f.svc.Submit(f.ctx, a, at.ID, "submit-scoped-001")
	must(t, err)
	allowed = false
	_, err = f.svc.Submit(f.ctx, a, at.ID, "submit-scoped-001")
	wantErr(t, err, domain.ErrNotFound)
	f.svc = app.New(f.store, f.clk, logging.Discard(), app.Config{})
	_, err = f.svc.GetAttempt(f.ctx, a, at.ID)
	if !errors.Is(err, courseaccess.ErrUnavailable) {
		t.Fatal(err)
	}
}
