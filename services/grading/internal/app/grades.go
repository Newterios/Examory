package app

import (
	"context"
	"errors"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/events"
	"slices"
	"strconv"
)

func (s *Service) SetGrade(ctx context.Context, p authn.Principal, id, key string, in domain.GradeInput) (*domain.Grade, bool, error) {
	if p.Role != authn.RoleTeacher {
		return nil, false, domain.ErrForbidden
	}
	if !validKey(key) {
		return nil, false, domain.ErrIdempotencyNeeded
	}
	if in.Criteria == nil {
		in.Criteria = []domain.CriterionScore{}
	}
	var out *domain.Grade
	var replay bool
	err := s.store.InTx(ctx, func(tx Tx) error {
		a, err := tx.LockGrade(ctx, id, p.UserID, key)
		if err != nil {
			return err
		}
		if err = s.authorize(ctx, p, a.CourseID, a.GroupID); err != nil {
			return err
		}
		if err = in.Validate(a.MaxScore); err != nil {
			return err
		}
		prior, err := tx.GradeByKey(ctx, p.UserID, key)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		if prior != nil {
			if prior.AttemptID != id || prior.Grade != *in.Grade || prior.Feedback != in.Feedback || !slices.Equal(prior.Criteria, in.Criteria) {
				return domain.ErrIdempotencyReuse
			}
			out, replay = prior, true
			return nil
		}
		version, err := tx.LastGradeRevision(ctx, id)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		payload := struct {
			TargetType string                  `json:"target_type"`
			TargetID   string                  `json:"target_id"`
			Grade      float64                 `json:"grade"`
			MaxGrade   int                     `json:"max_grade"`
			Scores     []domain.CriterionScore `json:"rubric_scores"`
			GradedBy   string                  `json:"graded_by"`
		}{"attempt", id, *in.Grade, a.MaxScore, in.Criteria, p.UserID}
		env, err := events.New("grade.assigned", 1, Producer, events.Actor{UserID: p.UserID, Role: string(p.Role)}, events.Context{CourseID: events.Ptr(a.CourseID), GroupID: events.Ptr(a.GroupID)}, payload, now)
		if err != nil {
			return err
		}
		raw, err := rawEnvelope(env)
		if err != nil {
			return err
		}
		if err = s.validator.Validate(raw); err != nil {
			return err
		}
		g := domain.Grade{AttemptID: id, Revision: version + 1, Grade: *in.Grade, MaxGrade: a.MaxScore, Feedback: in.Feedback, Criteria: in.Criteria, GradedBy: p.UserID, At: env.OccurredAt.Time, Key: key, EventID: env.EventID}
		if err = tx.InsertGrade(ctx, g); err != nil {
			return err
		}
		if err = tx.Audit(ctx, AuditEntry{At: now, ActorID: p.UserID, ActorRole: string(p.Role), Action: "grade.assigned", TargetType: "attempt", TargetID: id, Outcome: "published", Detail: map[string]string{"event_id": env.EventID, "revision": strconv.Itoa(g.Revision)}}); err != nil {
			return err
		}
		if err = tx.Enqueue(ctx, events.TopicGrades, a.CourseID, env); err != nil {
			return err
		}
		out = &g
		return nil
	})
	return out, replay, err
}

func (s *Service) GetGrades(ctx context.Context, p authn.Principal, id string, history bool) ([]domain.Grade, error) {
	if p.Role != authn.RoleTeacher && (p.Role != authn.RoleStudent || history) {
		return nil, domain.ErrForbidden
	}
	var grades []domain.Grade
	err := s.store.InTx(ctx, func(tx Tx) error {
		a, err := tx.GetAttempt(ctx, id)
		if err != nil {
			return err
		}
		if p.Role == authn.RoleStudent && p.UserID != a.StudentID {
			return domain.ErrNotFound
		}
		if err = s.authorize(ctx, p, a.CourseID, a.GroupID); err != nil {
			return err
		}
		grades, err = tx.Grades(ctx, id, history)
		if err != nil {
			return err
		}
		if len(grades) == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
	return grades, err
}
