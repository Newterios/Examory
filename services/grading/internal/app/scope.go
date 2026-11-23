package app

import (
	"context"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/courseaccess"
)

func (s *Service) allowed(ctx context.Context, p authn.Principal, course, group string) (bool, error) {
	if s.access == nil {
		return false, courseaccess.ErrUnavailable
	}
	if course == "" || group == "" {
		return false, nil
	}
	var teacher string
	switch p.Role {
	case authn.RoleTeacher:
		teacher = p.UserID
		if teacher == "" {
			return false, nil
		}
	case authn.RoleStudent:
		if p.GroupID != group {
			return false, nil
		}
	default:
		return false, nil
	}
	return s.access.Allowed(ctx, course, teacher, group)
}
func (s *Service) authorize(ctx context.Context, p authn.Principal, course, group string) error {
	ok, err := s.allowed(ctx, p, course, group)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}
