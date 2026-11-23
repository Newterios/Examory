// Package postgres implements the grading ports with pgx (parameterised
// statements only).
package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns all migration sets of the service.
func Migrations() []migrate.Set {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return []migrate.Set{outbox.Migrations(), consumer.Migrations(), {Name: "grading", FS: sub}}
}

// Store implements app.Store.
type Store struct{ pool *pgxpool.Pool }

// New returns a store.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Bind exposes an existing transaction (the consumer's) as app.Tx.
func Bind(t pgx.Tx) app.Tx { return &tx{t: t} }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	code, constraint, ok := db.ErrorCode(err)
	switch {
	case !ok:
		return err
	case code == db.CodeUniqueViolation && constraint == "reviews_pkey":
		return domain.ErrAlreadyReviewed
	case code == db.CodeUniqueViolation && constraint == "reviews_reviewer_id_idempotency_key_key":
		return domain.ErrIdempotencyReuse
	case code == db.CodeRaiseException:
		return fmt.Errorf("%w (%v)", domain.ErrAlreadyReviewed, err)
	}
	return err
}

// InTx implements app.Store.
func (s *Store) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return db.InTx(ctx, s.pool, func(t pgx.Tx) error { return fn(&tx{t: t}) })
}

const riskCols = `r.id, r.risk_id, r.event_id, r.category, r.target_type, r.target_id, r.score, r.level, r.model_name,
	r.model_version, r.explanation, r.evidence, coalesce(r.course_id, ''), coalesce(r.group_id, ''), r.producer,
	r.occurred_at, r.received_at, r.status, r.version`

func scanRisk(row pgx.Row, extra ...any) (domain.Risk, error) {
	var r domain.Risk
	var ev []byte
	dest := append([]any{&r.ID, &r.RiskID, &r.EventID, &r.Category, &r.TargetType, &r.TargetID, &r.Score, &r.Level,
		&r.ModelName, &r.ModelVersion, &r.Explanation, &ev, &r.CourseID, &r.GroupID, &r.Producer, &r.OccurredAt,
		&r.ReceivedAt, &r.Status, &r.Version}, extra...)
	err := row.Scan(dest...)
	r.Evidence = json.RawMessage(ev)
	return r, err
}

const zeroUUID = "00000000-0000-0000-0000-000000000000"

// ListRisks implements app.Store (keyset on grading's UUIDv7 id).
func (s *Store) ListRisks(ctx context.Context, f app.Filter, limit int, after string) ([]app.RiskRow, error) {
	if after == "" {
		after = zeroUUID
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+riskCols+`,
		       a.attempt_id, a.student_id, a.quiz_id, a.variant_id, a.course_id, a.group_id, a.score, a.max_score,
		       a.duration_ms, a.answers_changed, a.reason, a.submitted_at,
		       v.decision, v.comment, v.reviewer_id::text, v.reviewed_at
		  FROM risks r
		  LEFT JOIN attempts a ON r.target_type = 'attempt' AND a.attempt_id = r.target_id
		  LEFT JOIN reviews v ON v.risk_pk = r.id
		 WHERE r.id > $1
		   AND ($2::text = '' OR r.status = $2::text)
		   AND ($3::text = '' OR r.course_id = $3::text)
		   AND ($4::text = '' OR a.quiz_id = $4::text)
		 ORDER BY r.id LIMIT $5`, after, f.Status, f.CourseID, f.QuizID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.RiskRow
	for rows.Next() {
		var a struct {
			id, student, quiz, variant, course, group, reason *string
			score, maxScore, changes                          *int
			duration                                          *int64
			at                                                *time.Time
		}
		var dec, com, rid *string
		var rat *time.Time
		r, err := scanRisk(rows, &a.id, &a.student, &a.quiz, &a.variant, &a.course, &a.group, &a.score, &a.maxScore,
			&a.duration, &a.changes, &a.reason, &a.at, &dec, &com, &rid, &rat)
		if err != nil {
			return nil, err
		}
		row := app.RiskRow{Risk: r}
		if a.id != nil {
			row.Attempt = &domain.AttemptSummary{AttemptID: *a.id, StudentID: *a.student, QuizID: *a.quiz, VariantID: *a.variant,
				CourseID: *a.course, GroupID: *a.group, Score: *a.score, MaxScore: *a.maxScore, DurationMs: *a.duration,
				AnswersChanged: *a.changes, Reason: *a.reason, SubmittedAt: *a.at}
		}
		if dec != nil {
			row.Review = &domain.Review{RiskPK: r.ID, Decision: *dec, Comment: *com, ReviewerID: *rid, ReviewedAt: *rat}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type tx struct{ t pgx.Tx }

func (x *tx) UpsertAttempt(ctx context.Context, a domain.AttemptSummary) (bool, error) {
	tag, err := x.t.Exec(ctx, `
		INSERT INTO attempts (attempt_id, student_id, quiz_id, variant_id, course_id, group_id, score, max_score,
		                      duration_ms, answers_changed, reason, submitted_at, event_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (attempt_id) DO NOTHING`,
		a.AttemptID, a.StudentID, a.QuizID, a.VariantID, a.CourseID, a.GroupID, a.Score, a.MaxScore,
		a.DurationMs, a.AnswersChanged, a.Reason, a.SubmittedAt, a.EventID)
	if err != nil {
		return false, mapErr(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (x *tx) InsertRisk(ctx context.Context, r *domain.Risk) (bool, error) {
	var course, group *string
	if r.CourseID != "" {
		course = &r.CourseID
	}
	if r.GroupID != "" {
		group = &r.GroupID
	}
	tag, err := x.t.Exec(ctx, `
		INSERT INTO risks (id, risk_id, event_id, category, target_type, target_id, score, level, model_name,
		                   model_version, explanation, evidence, course_id, group_id, producer, occurred_at,
		                   received_at, status, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		ON CONFLICT (risk_id) DO NOTHING`,
		r.ID, r.RiskID, r.EventID, r.Category, r.TargetType, r.TargetID, r.Score, r.Level, r.ModelName,
		r.ModelVersion, r.Explanation, []byte(r.Evidence), course, group, r.Producer, r.OccurredAt,
		r.ReceivedAt, r.Status, r.Version)
	if err != nil {
		return false, mapErr(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (x *tx) GetRisk(ctx context.Context, id string, forUpdate bool) (*domain.Risk, error) {
	sql := `SELECT ` + riskCols + ` FROM risks r WHERE r.id = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	r, err := scanRisk(x.t.QueryRow(ctx, sql, id))
	if err != nil {
		return nil, mapErr(err)
	}
	return &r, nil
}

func (x *tx) GetAttempt(ctx context.Context, id string) (*domain.AttemptSummary, error) {
	var a domain.AttemptSummary
	err := x.t.QueryRow(ctx, `SELECT attempt_id, student_id, quiz_id, variant_id, course_id, group_id, score, max_score,
		duration_ms, answers_changed, reason, submitted_at, event_id FROM attempts WHERE attempt_id = $1`, id).
		Scan(&a.AttemptID, &a.StudentID, &a.QuizID, &a.VariantID, &a.CourseID, &a.GroupID, &a.Score, &a.MaxScore,
			&a.DurationMs, &a.AnswersChanged, &a.Reason, &a.SubmittedAt, &a.EventID)
	if err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

const reviewCols = `risk_pk, decision, comment, reviewer_id::text, reviewer_role, reviewed_at, idempotency_key, event_id`

func scanReview(row pgx.Row) (*domain.Review, error) {
	var r domain.Review
	if err := row.Scan(&r.RiskPK, &r.Decision, &r.Comment, &r.ReviewerID, &r.ReviewerRole, &r.ReviewedAt, &r.IdempotencyKey, &r.EventID); err != nil {
		return nil, mapErr(err)
	}
	return &r, nil
}

func (x *tx) GetReview(ctx context.Context, pk string) (*domain.Review, error) {
	return scanReview(x.t.QueryRow(ctx, `SELECT `+reviewCols+` FROM reviews WHERE risk_pk = $1`, pk))
}

func (x *tx) ReviewByKey(ctx context.Context, reviewer, key string) (*domain.Review, error) {
	return scanReview(x.t.QueryRow(ctx, `SELECT `+reviewCols+` FROM reviews WHERE reviewer_id = $1 AND idempotency_key = $2`, reviewer, key))
}

func (x *tx) InsertReview(ctx context.Context, r domain.Review) error {
	_, err := x.t.Exec(ctx, `INSERT INTO reviews (risk_pk, decision, comment, reviewer_id, reviewer_role, reviewed_at, idempotency_key, event_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		r.RiskPK, r.Decision, r.Comment, r.ReviewerID, r.ReviewerRole, r.ReviewedAt, r.IdempotencyKey, r.EventID)
	return mapErr(err)
}

func (x *tx) UpdateRiskStatus(ctx context.Context, r *domain.Risk) error {
	tag, err := x.t.Exec(ctx, `UPDATE risks SET status = $2, version = $3 WHERE id = $1 AND status = 'open' AND version = $3 - 1`,
		r.ID, r.Status, r.Version)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrVersionMismatch
	}
	return nil
}

func (x *tx) Audit(ctx context.Context, e app.AuditEntry) error {
	detail := e.Detail
	if detail == nil {
		detail = map[string]string{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = x.t.Exec(ctx, `INSERT INTO audit_log (at, actor_id, actor_role, action, target_type, target_id, outcome, detail)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, e.At, e.ActorID, e.ActorRole, e.Action, e.TargetType, e.TargetID, e.Outcome, raw)
	return mapErr(err)
}

func (x *tx) Reject(ctx context.Context, r app.Rejected) error {
	var part *int32
	var off *int64
	if r.Topic != "" && (r.Partition != 0 || r.Offset != 0) {
		part, off = &r.Partition, &r.Offset
	}
	reason := safeText(r.Reason, 1000)
	if reason == "" {
		reason = "unspecified"
	}
	_, err := x.t.Exec(ctx, `INSERT INTO rejected_events (received_at, topic, kafka_partition, kafka_offset, event_id, event_type, reason, excerpt)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8)`, r.At, r.Topic, part, off, truncate(r.EventID, 64), truncate(r.EventType, 64), reason, safeText(r.Excerpt, 4096))
	return mapErr(err)
}

func truncate(s string, n int) string { return safeText(s, n) }

// safeText returns valid UTF-8 of at most n runes (Postgres rejects invalid
// UTF-8 in text columns; dead-letter excerpts may be arbitrary bytes).
func safeText(s string, n int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func (x *tx) Enqueue(ctx context.Context, topic, key string, env events.Envelope) error {
	return outbox.Enqueue(ctx, x.t, topic, key, env)
}
