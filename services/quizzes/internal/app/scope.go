package app

import (
	"context"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

func (s *Service) allowed(ctx context.Context, a Actor, course, group string) (bool, error) {
	if s.cfg.Access == nil {
		return false, courseaccess.ErrUnavailable
	}
	teacher := ""
	switch a.Role {
	case authn.RoleTeacher:
		teacher = a.UserID
	case authn.RoleAdmin:
		if group == "" {
			return true, nil
		}
	case authn.RoleStudent:
		if group == "" || a.GroupID != group {
			return false, nil
		}
	default:
		return false, nil
	}
	return s.cfg.Access.Allowed(ctx, course, teacher, group)
}
func (s *Service) authorize(ctx context.Context, a Actor, course, group string) error {
	ok, err := s.allowed(ctx, a, course, group)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}
