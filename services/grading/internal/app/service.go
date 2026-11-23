package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"log/slog"
	"strings"
	"time"

	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
)

// Producer is the envelope producer name.
const Producer = "grading"

// Service implements the use cases.
type Service struct {
	access    courseaccess.Checker
	store     Store
	validator Validator
	clock     clock.Clock
	log       *slog.Logger
	newID     func(time.Time) string
}

// New returns a Service.
func New(store Store, v Validator, clk clock.Clock, logger *slog.Logger, access courseaccess.Checker) *Service {
	return &Service{access: access, store: store, validator: v, clock: clk, log: logger, newID: ids.NewV7At}
}

// Store returns the underlying store (for the consumer wiring).
func (s *Service) Store() Store { return s.store }

func canRead(p authn.Principal) bool   { return p.Is(authn.RoleTeacher, authn.RoleAdmin) }
func canReview(p authn.Principal) bool { return p.Role == authn.RoleTeacher }

// ---- ingestion (consumer) ----

// Ingest applies one event inside tx. It is called at most once per
// (consumer group, event_id) by the platform consumer. Invalid risk payloads
// are dead-letter-logged in tx and acknowledged; they never stop the consumer.
func (s *Service) Ingest(ctx context.Context, tx Tx, env events.Envelope, topic string) error {
	switch {
	case env.EventType == events.TypeQuizSubmitted && topic == events.TopicQuizzes:
		return s.ingestSubmitted(ctx, tx, env, topic)
	case (env.EventType == domain.CategoryBehaviorAnomaly || env.EventType == "risk.plagiarism") && topic == events.TopicRisk:
		return s.ingestRisk(ctx, tx, env, topic)
	case strings.HasPrefix(env.EventType, "risk.") && topic == events.TopicRisk:
		// Other risk categories are acknowledged but not stored yet.
		s.log.InfoContext(ctx, "risk category not handled yet; acknowledged", slog.String("event_type", env.EventType), slog.String("event_id", env.EventID))
		return nil
	default:
		return nil // other lms.quizzes events are not needed by grading
	}
}

func (s *Service) reject(ctx context.Context, tx Tx, env events.Envelope, topic string, reason error) error {
	raw, _ := rawEnvelope(env)
	excerpt := string(raw)
	if len(excerpt) > 4096 {
		excerpt = excerpt[:4096]
	}
	msg := reason.Error()
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	s.log.WarnContext(ctx, "event rejected (dead-letter logged)", slog.String("event_id", env.EventID),
		slog.String("event_type", env.EventType), slog.String("reason", msg))
	return tx.Reject(ctx, Rejected{At: s.clock.Now(), Topic: topic, EventID: env.EventID, EventType: env.EventType, Reason: msg, Excerpt: excerpt})
}

func (s *Service) ingestSubmitted(ctx context.Context, tx Tx, env events.Envelope, topic string) error {
	var p events.QuizSubmitted
	if err := env.DecodePayload(&p, false); err != nil || p.AttemptID == "" || p.MaxScore <= 0 {
		return s.reject(ctx, tx, env, topic, fmt.Errorf("invalid quiz.submitted payload"))
	}
	if env.Context.CourseID == nil || env.Context.GroupID == nil {
		return s.reject(ctx, tx, env, topic, fmt.Errorf("quiz.submitted without course_id/group_id"))
	}
	_, err := tx.UpsertAttempt(ctx, domain.AttemptSummary{
		AttemptID: p.AttemptID, StudentID: env.Actor.UserID, QuizID: p.QuizID, VariantID: p.VariantID,
		CourseID: *env.Context.CourseID, GroupID: *env.Context.GroupID, Score: p.Score, MaxScore: p.MaxScore,
		DurationMs: p.DurationMs, AnswersChanged: p.AnswersChanged, Reason: p.Reason,
		SubmittedAt: env.OccurredAt.Time, EventID: env.EventID,
	})
	return err
}

func (s *Service) ingestRisk(ctx context.Context, tx Tx, env events.Envelope, topic string) error {
	raw, err := rawEnvelope(env)
	if err != nil {
		return err
	}
	if err := s.validator.Validate(raw); err != nil {
		return s.reject(ctx, tx, env, topic, fmt.Errorf("schema: %w", err))
	}
	var p domain.RiskPayload
	if err := env.DecodePayload(&p, false); err != nil {
		return s.reject(ctx, tx, env, topic, fmt.Errorf("decode: %w", err))
	}
	if (env.EventType == "risk.plagiarism" && p.TargetType != "submission") || (env.EventType == domain.CategoryBehaviorAnomaly && p.TargetType != "attempt") {
		return s.reject(ctx, tx, env, topic, fmt.Errorf("wrong risk target"))
	}
	if err := p.CheckPayload(); err != nil {
		return s.reject(ctx, tx, env, topic, err)
	}
	now := s.clock.Now()
	r := &domain.Risk{
		ID: s.newID(now), RiskID: p.RiskID, EventID: env.EventID, Category: env.EventType,
		TargetType: p.TargetType, TargetID: p.TargetID, Score: p.Score, Level: p.Level,
		ModelName: p.Model.Name, ModelVersion: p.Model.Version, Explanation: p.Explanation, Evidence: p.Evidence,
		Producer: env.Producer, OccurredAt: env.OccurredAt.Time, ReceivedAt: now, Status: domain.StatusOpen, Version: 1,
	}
	if env.Context.CourseID != nil {
		r.CourseID = *env.Context.CourseID
	}
	if env.Context.GroupID != nil {
		r.GroupID = *env.Context.GroupID
	}
	inserted, err := tx.InsertRisk(ctx, r)
	if err != nil {
		return err
	}
	if !inserted {
		// Same risk_id re-emitted under a new event_id: keep the first one.
		s.log.InfoContext(ctx, "duplicate risk_id ignored", slog.String("risk_id", p.RiskID), slog.String("event_id", env.EventID))
		return nil
	}
	// Receipt changes nothing else: no grade, no penalty, no block (FR-26).
	return tx.Audit(ctx, AuditEntry{At: now, ActorID: env.Actor.UserID, ActorRole: env.Actor.Role, Action: "risk.received",
		TargetType: "risk", TargetID: r.ID, Outcome: "stored",
		Detail: map[string]string{"risk_id": r.RiskID, "event_id": env.EventID, "level": r.Level}})
}

// RecordRejected dead-letter-logs a record the consumer skipped before the
// handler ran (invalid envelope, permanent failure).
func (s *Service) RecordRejected(ctx context.Context, r Rejected) {
	r.At = s.clock.Now()
	if len(r.Excerpt) > 4096 {
		r.Excerpt = r.Excerpt[:4096]
	}
	if len(r.Reason) > 1000 {
		r.Reason = r.Reason[:1000]
	}
	if err := s.store.InTx(ctx, func(tx Tx) error { return tx.Reject(ctx, r) }); err != nil {
		s.log.ErrorContext(ctx, "could not record rejected event", slog.String("error", err.Error()))
	}
}

// ---- teacher workflow ----

// ListRisks lists risk signals (teacher/admin).
func (s *Service) ListRisks(ctx context.Context, p authn.Principal, f Filter, limit int, after string) ([]RiskRow, error) {
	if !canRead(p) {
		return nil, domain.ErrForbidden
	}
	if p.Role == authn.RoleAdmin {
		return s.store.ListRisks(ctx, f, limit, after)
	}
	return courseaccess.Visible(ctx, limit, after, func(cursor string, n int) ([]RiskRow, error) { return s.store.ListRisks(ctx, f, n, cursor) }, func(row RiskRow) (string, string, string) { return row.Risk.ID, row.Risk.CourseID, row.Risk.GroupID }, func(course, group string) (bool, error) { return s.allowed(ctx, p, course, group) })
}

// GetRisk returns one signal with evidence, attempt summary and review.
// Every read of evidence is audited (NFR-16).
func (s *Service) GetRisk(ctx context.Context, p authn.Principal, id string) (*RiskRow, error) {
	if p.Role != authn.RoleTeacher {
		return nil, domain.ErrForbidden
	}
	var row *RiskRow
	err := s.store.InTx(ctx, func(tx Tx) error {
		r, err := tx.GetRisk(ctx, id, false)
		if err != nil {
			return err
		}
		if err = s.authorize(ctx, p, r.CourseID, r.GroupID); err != nil {
			return err
		}
		row, err = s.loadRow(ctx, tx, r)
		if err != nil {
			return err
		}
		return tx.Audit(ctx, AuditEntry{At: s.clock.Now(), ActorID: p.UserID, ActorRole: string(p.Role),
			Action: "risk.evidence_viewed", TargetType: "risk", TargetID: r.ID, Outcome: "success"})
	})
	return row, err
}

func (s *Service) loadRow(ctx context.Context, tx Tx, r *domain.Risk) (*RiskRow, error) {
	row := &RiskRow{Risk: *r}
	if r.TargetType == "attempt" {
		a, err := tx.GetAttempt(ctx, r.TargetID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		row.Attempt = a
	}
	rv, err := tx.GetReview(ctx, r.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	row.Review = rv
	return row, nil
}

// ReviewResult is the outcome of Review.
type ReviewResult struct {
	Row      *RiskRow
	Replayed bool
}

// Review records the single teacher decision on a risk and publishes
// risk.reviewed in the same transaction. ifMatch is the expected risk
// version (0 = header missing).
func (s *Service) Review(ctx context.Context, p authn.Principal, id, decision, comment string, ifMatch int, key string) (*ReviewResult, error) {
	if !canReview(p) {
		return nil, domain.ErrForbidden
	}
	if !validKey(key) {
		return nil, domain.ErrIdempotencyNeeded
	}
	if err := domain.ValidateDecision(decision, comment); err != nil {
		return nil, err
	}
	var out *ReviewResult
	err := s.store.InTx(ctx, func(tx Tx) error {
		prev, err := tx.ReviewByKey(ctx, p.UserID, key)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		r, err := tx.GetRisk(ctx, id, true)
		if err != nil {
			return err
		}
		if err = s.authorize(ctx, p, r.CourseID, r.GroupID); err != nil {
			return err
		}
		if prev != nil {
			if prev.RiskPK != r.ID || prev.Decision != decision || prev.Comment != comment {
				return domain.ErrIdempotencyReuse
			}
			row, err := s.loadRow(ctx, tx, r)
			if err != nil {
				return err
			}
			out = &ReviewResult{Row: row, Replayed: true}
			return nil
		}
		if r.Status != domain.StatusOpen {
			return domain.ErrAlreadyReviewed
		}
		if ifMatch == 0 {
			return domain.ErrIfMatchRequired
		}
		if err := r.ApplyReview(decision, ifMatch); err != nil {
			return err
		}
		now := s.clock.Now()
		env, err := events.New("risk.reviewed", 1, Producer,
			events.Actor{UserID: p.UserID, Role: string(p.Role)},
			events.Context{CourseID: events.Ptr(r.CourseID), GroupID: events.Ptr(r.GroupID)},
			reviewedPayload{RiskID: r.RiskID, TargetType: r.TargetType, TargetID: r.TargetID, Decision: decision,
				ReviewedBy: p.UserID, Comment: comment}, now)
		if err != nil {
			return err
		}
		rv := domain.Review{RiskPK: r.ID, Decision: decision, Comment: comment, ReviewerID: p.UserID,
			ReviewerRole: string(p.Role), ReviewedAt: env.OccurredAt.Time, IdempotencyKey: key, EventID: env.EventID}
		// Review row first: the schema requires a matching review for a
		// non-open status.
		if err := tx.InsertReview(ctx, rv); err != nil {
			return err
		}
		if err := tx.UpdateRiskStatus(ctx, r); err != nil {
			return err
		}
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: p.UserID, ActorRole: string(p.Role), Action: "risk.reviewed",
			TargetType: "risk", TargetID: r.ID, Outcome: decision,
			Detail: map[string]string{"risk_id": r.RiskID, "event_id": env.EventID}}); err != nil {
			return err
		}
		key := r.CourseID // course-level event (events.md §1)
		if key == "" {
			key = r.RiskID
		}
		if err := tx.Enqueue(ctx, events.TopicGrades, key, env); err != nil {
			return err
		}
		row, err := s.loadRow(ctx, tx, r)
		if err != nil {
			return err
		}
		out = &ReviewResult{Row: row}
		return nil
	})
	var de *domain.Error
	if errors.As(err, &de) && (de.Kind == domain.KindConflict || de.Kind == domain.KindPrecondition || de.Kind == domain.KindPreconditionRequired || de.Kind == domain.KindInvalid) {
		// Failed review attempts are audited too, in their own transaction.
		_ = s.store.InTx(ctx, func(tx Tx) error {
			return tx.Audit(ctx, AuditEntry{At: s.clock.Now(), ActorID: p.UserID, ActorRole: string(p.Role), Action: "risk.review_rejected",
				TargetType: "risk", TargetID: id, Outcome: de.Code})
		})
	}
	return out, err
}

type reviewedPayload struct {
	RiskID     string `json:"risk_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Decision   string `json:"decision"`
	ReviewedBy string `json:"reviewed_by"`
	Comment    string `json:"comment"`
}

func validKey(k string) bool {
	if len(k) < 8 || len(k) > 128 {
		return false
	}
	for _, c := range k {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}
