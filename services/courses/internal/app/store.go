package app

import (
	"context"
	"github.com/Newterios/Examory/services/courses/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
)

type Store interface {
	Create(context.Context, authn.Principal, string) (domain.Course, error)
	List(context.Context, authn.Principal, string, int) ([]domain.Course, error)
	Get(context.Context, authn.Principal, string) (domain.Course, error)
	Update(context.Context, authn.Principal, string, domain.Course) (domain.Course, error)
	Assign(context.Context, authn.Principal, string, string, string, bool) error
	Access(context.Context, string, string, string) (bool, error)
	TeacherCourses(context.Context, string) ([]string, error)
	View(context.Context, authn.Principal, string, string, int64, bool) (bool, error)
}
