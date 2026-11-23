package app_test

import (
	"context"
	"errors"
	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/logging"
	"testing"
)

func TestTeacherScopeAppliesToListViewReviewAndReplay(t *testing.T) {
	f := newFx(t)
	id, _, _ := f.seed()
	permitted := true
	unavailable := false
	check := courseaccess.CheckFunc(func(_ context.Context, course, teacher, group string) (bool, error) {
		if unavailable {
			return false, courseaccess.ErrUnavailable
		}
		return permitted && course == f.course && group == f.group && teacher == f.teacher.UserID, nil
	})
	f.svc = app.New(f.store, f.val, clock.NewFake(t0), logging.Discard(), check)
	if list, err := f.svc.ListRisks(f.ctx, f.other, app.Filter{}, 10, ""); err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
	if _, err := f.svc.GetRisk(f.ctx, f.other, id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Review(f.ctx, f.other, id, "confirmed", "unauthorized", 1, "review-no-access"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Review(f.ctx, f.teacher, id, "confirmed", "valid", 1, "repeat-after-revoke"); err != nil {
		t.Fatal(err)
	}
	permitted = false
	if _, err := f.svc.Review(f.ctx, f.teacher, id, "confirmed", "valid", 1, "repeat-after-revoke"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	unavailable = true
	if _, err := f.svc.GetRisk(f.ctx, f.teacher, id); !errors.Is(err, courseaccess.ErrUnavailable) {
		t.Fatal(err)
	}
}
