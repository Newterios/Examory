// Package postgres implements the app ports with pgx. Every statement is
// parameterised; no SQL text is ever built from input.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
	"github.com/Newterios/Examory/services/quizzes/internal/domain"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations returns the service migration sets (outbox first).
func Migrations() []migrate.Set {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return []migrate.Set{outbox.Migrations(), {Name: "quizzes", FS: sub}}
}

// Store implements app.Store.
type Store struct{ pool *pgxpool.Pool }

// New returns a store on pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// mapErr converts database errors into domain errors where the schema
// enforces a domain invariant; everything else stays internal.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	code, constraint, ok := db.ErrorCode(err)
	if !ok {
		return err
	}
	switch {
	case code == db.CodeUniqueViolation && constraint == "attempts_one_active_idx":
		return domain.ErrAttemptActive
	case code == db.CodeUniqueViolation && constraint == "attempts_quiz_id_user_id_attempt_no_key":
		return domain.ErrAttemptActive
	case code == db.CodeUniqueViolation && constraint == "attempt_finalizations_pkey":
		return domain.ErrAttemptFinalized
	case code == db.CodeUniqueViolation && constraint == "attempt_finalizations_user_id_idempotency_key_key":
		return domain.ErrIdempotencyReuse
	case code == db.CodeRaiseException:
		// A trigger-enforced invariant (immutable published quiz, frozen
		// finalized attempt) caught a request the use case did not.
		return fmt.Errorf("%w (%v)", domain.ErrInvariant, err)
	}
	return err
}

// InTx implements app.Store.
func (s *Store) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return db.InTx(ctx, s.pool, func(t pgx.Tx) error { return fn(&tx{t: t}) })
}

const quizCols = `id, owner_id, course_id, title, time_limit_sec, max_attempts, show_score, status, created_at, updated_at, published_at`

func scanQuiz(row pgx.Row) (domain.Quiz, error) {
	var q domain.Quiz
	err := row.Scan(&q.ID, &q.OwnerID, &q.CourseID, &q.Title, &q.TimeLimitSec, &q.MaxAttempts, &q.ShowScore,
		&q.Status, &q.CreatedAt, &q.UpdatedAt, &q.PublishedAt)
	return q, err
}

// ListQuizzes implements app.Store.
func (s *Store) ListQuizzes(ctx context.Context, owner string, limit int, after string) ([]domain.Quiz, error) {
	var rows pgx.Rows
	var err error
	if owner == "" {
		rows, err = s.pool.Query(ctx, `SELECT `+quizCols+` FROM quizzes WHERE id > $1 ORDER BY id LIMIT $2`, cursor(after), limit)
	} else {
		rows, err = s.pool.Query(ctx, `SELECT `+quizCols+` FROM quizzes WHERE owner_id = $1 AND id > $2 ORDER BY id LIMIT $3`, owner, cursor(after), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Quiz
	for rows.Next() {
		q, err := scanQuiz(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ListAssigned implements app.Store.
func (s *Store) ListAssigned(ctx context.Context, group string, limit int, after string) ([]app.AssignedQuiz, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT q.id, q.owner_id, q.course_id, q.title, q.time_limit_sec, q.max_attempts, q.show_score, q.status,
		       q.created_at, q.updated_at, q.published_at, a.group_id, a.opens_at, a.closes_at, a.assigned_by, a.assigned_at
		  FROM quiz_assignments a JOIN quizzes q ON q.id = a.quiz_id
		 WHERE a.group_id = $1 AND q.status = 'published' AND q.id > $2
		 ORDER BY q.id LIMIT $3`, group, cursor(after), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.AssignedQuiz
	for rows.Next() {
		var x app.AssignedQuiz
		q := &x.Quiz
		if err := rows.Scan(&q.ID, &q.OwnerID, &q.CourseID, &q.Title, &q.TimeLimitSec, &q.MaxAttempts, &q.ShowScore, &q.Status,
			&q.CreatedAt, &q.UpdatedAt, &q.PublishedAt, &x.Assignment.GroupID, &x.Assignment.OpensAt, &x.Assignment.ClosesAt,
			&x.Assignment.AssignedBy, &x.Assignment.AssignedAt); err != nil {
			return nil, err
		}
		x.Assignment.QuizID = q.ID
		out = append(out, x)
	}
	return out, rows.Err()
}

const attemptCols = `id, quiz_id, variant_id, user_id, group_id, course_id, attempt_no, status, sim, shuffle_seed,
	time_limit_sec, show_score, started_at, deadline_at, last_activity_at, finished_at, score, max_score, answers_changed`

func scanAttempt(row pgx.Row) (domain.Attempt, error) {
	var a domain.Attempt
	err := row.Scan(&a.ID, &a.QuizID, &a.VariantID, &a.UserID, &a.GroupID, &a.CourseID, &a.AttemptNo, &a.Status, &a.Sim,
		&a.ShuffleSeed, &a.TimeLimitSec, &a.ShowScore, &a.StartedAt, &a.DeadlineAt, &a.LastActivityAt, &a.FinishedAt,
		&a.Score, &a.MaxScore, &a.AnswersChanged)
	return a, err
}

// ListAttempts implements app.Store.
func (s *Store) ListAttempts(ctx context.Context, quizID string, limit int, after string) ([]domain.Attempt, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+attemptCols+` FROM attempts WHERE quiz_id = $1 AND id > $2 ORDER BY id LIMIT $3`, quizID, cursor(after), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Attempt
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const zeroUUID = "00000000-0000-0000-0000-000000000000"

// cursor maps an empty keyset cursor to the smallest UUID.
func cursor(after string) string {
	if after == "" {
		return zeroUUID
	}
	return after
}

type tx struct{ t pgx.Tx }

func (x *tx) exec(ctx context.Context, sql string, args ...any) error {
	_, err := x.t.Exec(ctx, sql, args...)
	return mapErr(err)
}

func (x *tx) InsertQuiz(ctx context.Context, q *domain.Quiz) error {
	if err := x.exec(ctx, `INSERT INTO quizzes (`+quizCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		q.ID, q.OwnerID, q.CourseID, q.Title, q.TimeLimitSec, q.MaxAttempts, q.ShowScore, q.Status, q.CreatedAt, q.UpdatedAt, q.PublishedAt); err != nil {
		return err
	}
	return x.insertContent(ctx, q)
}

func (x *tx) insertContent(ctx context.Context, q *domain.Quiz) error {
	for i, qs := range q.Questions {
		var topic *string
		if qs.TopicID != "" {
			topic = &qs.TopicID
		}
		if err := x.exec(ctx, `INSERT INTO questions (id, quiz_id, position, text, points, topic_id) VALUES ($1,$2,$3,$4,$5,$6)`,
			qs.ID, q.ID, i, qs.Text, qs.Points, topic); err != nil {
			return err
		}
		for j, o := range qs.Options {
			if err := x.exec(ctx, `INSERT INTO question_options (id, question_id, position, text, is_correct) VALUES ($1,$2,$3,$4,$5)`,
				o.ID, qs.ID, j, o.Text, o.Correct); err != nil {
				return err
			}
		}
	}
	for _, v := range q.Variants {
		if err := x.exec(ctx, `INSERT INTO variants (id, quiz_id, ordinal) VALUES ($1,$2,$3)`, v.ID, q.ID, v.Ordinal); err != nil {
			return err
		}
		for i, qs := range v.Questions {
			if err := x.exec(ctx, `INSERT INTO variant_questions (variant_id, quiz_id, question_id, position) VALUES ($1,$2,$3,$4)`,
				v.ID, q.ID, qs.ID, i); err != nil {
				return err
			}
		}
	}
	return nil
}

func (x *tx) ReplaceDraft(ctx context.Context, q *domain.Quiz) error {
	for _, sql := range []string{
		`DELETE FROM variant_questions WHERE quiz_id = $1`,
		`DELETE FROM variants WHERE quiz_id = $1`,
		`DELETE FROM questions WHERE quiz_id = $1`,
	} {
		if err := x.exec(ctx, sql, q.ID); err != nil {
			return err
		}
	}
	if err := x.exec(ctx, `UPDATE quizzes SET course_id=$2, title=$3, time_limit_sec=$4, max_attempts=$5, show_score=$6, updated_at=$7
		WHERE id = $1 AND status = 'draft'`, q.ID, q.CourseID, q.Title, q.TimeLimitSec, q.MaxAttempts, q.ShowScore, q.UpdatedAt); err != nil {
		return err
	}
	return x.insertContent(ctx, q)
}

func (x *tx) GetQuiz(ctx context.Context, id string, forUpdate bool) (*domain.Quiz, error) {
	sql := `SELECT ` + quizCols + ` FROM quizzes WHERE id = $1`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	q, err := scanQuiz(x.t.QueryRow(ctx, sql, id))
	if err != nil {
		return nil, mapErr(err)
	}
	// Questions with options.
	rows, err := x.t.Query(ctx, `
		SELECT qs.id, qs.text, qs.points, coalesce(qs.topic_id, ''), o.id, o.text, o.is_correct
		  FROM questions qs JOIN question_options o ON o.question_id = qs.id
		 WHERE qs.quiz_id = $1 ORDER BY qs.position, o.position`, id)
	if err != nil {
		return nil, err
	}
	byID := map[string]int{}
	for rows.Next() {
		var qs domain.Question
		var o domain.Option
		if err := rows.Scan(&qs.ID, &qs.Text, &qs.Points, &qs.TopicID, &o.ID, &o.Text, &o.Correct); err != nil {
			rows.Close()
			return nil, err
		}
		i, ok := byID[qs.ID]
		if !ok {
			i = len(q.Questions)
			byID[qs.ID] = i
			q.Questions = append(q.Questions, qs)
		}
		q.Questions[i].Options = append(q.Questions[i].Options, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Variants.
	vrows, err := x.t.Query(ctx, `
		SELECT v.id, v.ordinal, vq.question_id
		  FROM variants v JOIN variant_questions vq ON vq.variant_id = v.id
		 WHERE v.quiz_id = $1 ORDER BY v.ordinal, vq.position`, id)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	vIdx := map[string]int{}
	for vrows.Next() {
		var vid, qid string
		var ord int
		if err := vrows.Scan(&vid, &ord, &qid); err != nil {
			return nil, err
		}
		i, ok := vIdx[vid]
		if !ok {
			i = len(q.Variants)
			vIdx[vid] = i
			q.Variants = append(q.Variants, domain.Variant{ID: vid, Ordinal: ord})
		}
		qi, ok := byID[qid]
		if !ok {
			return nil, fmt.Errorf("variant references unknown question")
		}
		q.Variants[i].Questions = append(q.Variants[i].Questions, q.Questions[qi])
	}
	if err := vrows.Err(); err != nil {
		return nil, err
	}
	if len(q.Variants) == 0 {
		return nil, fmt.Errorf("quiz %s has no variants", id)
	}
	return &q, nil
}

func (x *tx) SetPublished(ctx context.Context, id string, at time.Time) error {
	tag, err := x.t.Exec(ctx, `UPDATE quizzes SET status='published', published_at=$2, updated_at=$2 WHERE id=$1 AND status='draft'`, id, at)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotDraft
	}
	return nil
}

func (x *tx) UpsertAssignment(ctx context.Context, a domain.Assignment) error {
	return x.exec(ctx, `
		INSERT INTO quiz_assignments (quiz_id, group_id, opens_at, closes_at, assigned_by, assigned_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (quiz_id, group_id) DO UPDATE
		   SET opens_at = EXCLUDED.opens_at, closes_at = EXCLUDED.closes_at,
		       assigned_by = EXCLUDED.assigned_by, assigned_at = EXCLUDED.assigned_at`,
		a.QuizID, a.GroupID, a.OpensAt, a.ClosesAt, a.AssignedBy, a.AssignedAt)
}

func (x *tx) GetAssignment(ctx context.Context, quizID, groupID string) (*domain.Assignment, error) {
	var a domain.Assignment
	err := x.t.QueryRow(ctx, `SELECT quiz_id, group_id, opens_at, closes_at, assigned_by, assigned_at
		FROM quiz_assignments WHERE quiz_id=$1 AND group_id=$2`, quizID, groupID).
		Scan(&a.QuizID, &a.GroupID, &a.OpensAt, &a.ClosesAt, &a.AssignedBy, &a.AssignedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (x *tx) ListAssignments(ctx context.Context, quizID string) ([]domain.Assignment, error) {
	rows, err := x.t.Query(ctx, `SELECT quiz_id, group_id, opens_at, closes_at, assigned_by, assigned_at
		FROM quiz_assignments WHERE quiz_id=$1 ORDER BY group_id`, quizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Assignment
	for rows.Next() {
		var a domain.Assignment
		if err := rows.Scan(&a.QuizID, &a.GroupID, &a.OpensAt, &a.ClosesAt, &a.AssignedBy, &a.AssignedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (x *tx) LockStudentQuiz(ctx context.Context, quizID, userID string) error {
	return x.exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || '/' || $2, 0))`, quizID, userID)
}

func (x *tx) CountAttempts(ctx context.Context, quizID, userID string) (int, error) {
	var n int
	err := x.t.QueryRow(ctx, `SELECT count(*) FROM attempts WHERE quiz_id=$1 AND user_id=$2`, quizID, userID).Scan(&n)
	return n, err
}

func (x *tx) ActiveAttemptID(ctx context.Context, quizID, userID string) (string, error) {
	var id string
	err := x.t.QueryRow(ctx, `SELECT id FROM attempts WHERE quiz_id=$1 AND user_id=$2 AND status='in_progress'`, quizID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (x *tx) InsertAttempt(ctx context.Context, a *domain.Attempt) error {
	if err := x.exec(ctx, `INSERT INTO attempts (`+attemptCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		a.ID, a.QuizID, a.VariantID, a.UserID, a.GroupID, a.CourseID, a.AttemptNo, a.Status, a.Sim, a.ShuffleSeed,
		a.TimeLimitSec, a.ShowScore, a.StartedAt, a.DeadlineAt, a.LastActivityAt, a.FinishedAt, a.Score, a.MaxScore, a.AnswersChanged); err != nil {
		return err
	}
	for _, q := range a.Questions {
		if err := x.exec(ctx, `INSERT INTO attempt_questions (attempt_id, question_id, position) VALUES ($1,$2,$3)`, a.ID, q.QuestionID, q.Index); err != nil {
			return err
		}
	}
	return nil
}

func (x *tx) GetAttempt(ctx context.Context, id, userID string, forUpdate bool) (*domain.Attempt, error) {
	sql := `SELECT ` + attemptCols + ` FROM attempts WHERE id = $1`
	args := []any{id}
	if userID != "" { // scope to the caller (no IDOR); "" only for the sweeper
		sql += ` AND user_id = $2`
		args = append(args, userID)
	}
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	a, err := scanAttempt(x.t.QueryRow(ctx, sql, args...))
	if err != nil {
		return nil, mapErr(err)
	}
	rows, err := x.t.Query(ctx, `
		SELECT aq.question_id, aq.position, aq.first_shown_at, qs.text, qs.points,
		       an.selected::text[], an.answer_hash, an.saved_at, an.change_count
		  FROM attempt_questions aq
		  JOIN questions qs ON qs.id = aq.question_id
		  LEFT JOIN answers an ON an.attempt_id = aq.attempt_id AND an.question_id = aq.question_id
		 WHERE aq.attempt_id = $1 ORDER BY aq.position`, id)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for rows.Next() {
		var q domain.AttemptQuestion
		var sel []string
		var hash *string
		var saved *time.Time
		var changes *int
		if err := rows.Scan(&q.QuestionID, &q.Index, &q.FirstShownAt, &q.Text, &q.Points, &sel, &hash, &saved, &changes); err != nil {
			rows.Close()
			return nil, err
		}
		if hash != nil {
			q.Answer = &domain.Answer{Selected: sel, Hash: *hash, SavedAt: *saved, Changes: *changes}
		}
		idx[q.QuestionID] = len(a.Questions)
		a.Questions = append(a.Questions, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	orows, err := x.t.Query(ctx, `
		SELECT o.question_id, o.id, o.text, o.is_correct
		  FROM question_options o JOIN attempt_questions aq ON aq.question_id = o.question_id
		 WHERE aq.attempt_id = $1 ORDER BY o.question_id, o.position`, id)
	if err != nil {
		return nil, err
	}
	defer orows.Close()
	for orows.Next() {
		var qid string
		var o domain.Option
		if err := orows.Scan(&qid, &o.ID, &o.Text, &o.Correct); err != nil {
			return nil, err
		}
		if i, ok := idx[qid]; ok {
			a.Questions[i].Options = append(a.Questions[i].Options, o)
		}
	}
	return &a, orows.Err()
}

func (x *tx) SaveShown(ctx context.Context, attemptID, questionID string, at time.Time) error {
	return x.exec(ctx, `UPDATE attempt_questions SET first_shown_at = $3
		WHERE attempt_id = $1 AND question_id = $2 AND first_shown_at IS NULL`, attemptID, questionID, at)
}

func (x *tx) SaveAnswer(ctx context.Context, a *domain.Attempt, log app.AnswerLog) error {
	var ans *domain.Answer
	for _, q := range a.Questions {
		if q.QuestionID == log.QuestionID {
			ans = q.Answer
		}
	}
	if ans == nil {
		return fmt.Errorf("answer missing on aggregate")
	}
	r := log.Result
	if err := x.exec(ctx, `
		INSERT INTO answers (attempt_id, question_id, selected, answer_hash, saved_at, change_count)
		VALUES ($1,$2,$3::uuid[],$4,$5,$6)
		ON CONFLICT (attempt_id, question_id) DO UPDATE
		   SET selected = EXCLUDED.selected, answer_hash = EXCLUDED.answer_hash,
		       saved_at = EXCLUDED.saved_at, change_count = EXCLUDED.change_count`,
		a.ID, log.QuestionID, ans.Selected, ans.Hash, ans.SavedAt, ans.Changes); err != nil {
		return err
	}
	if err := x.exec(ctx, `
		INSERT INTO answer_log (attempt_id, question_id, selected, answer_hash, is_change, saved_at,
		                        time_since_shown_ms, time_since_attempt_start_ms, event_id)
		VALUES ($1,$2,$3::uuid[],$4,$5,$6,$7,$8,$9)`,
		a.ID, log.QuestionID, r.Selected, r.Hash, r.IsChange, r.SavedAt, r.TimeSinceShownMs, r.TimeSinceAttemptStartMs, log.EventID); err != nil {
		return err
	}
	return x.TouchAttempt(ctx, a)
}

func (x *tx) TouchAttempt(ctx context.Context, a *domain.Attempt) error {
	return x.exec(ctx, `UPDATE attempts SET last_activity_at = $2, answers_changed = $3 WHERE id = $1 AND status = 'in_progress'`,
		a.ID, a.LastActivityAt, a.AnswersChanged)
}

func (x *tx) Finalize(ctx context.Context, a *domain.Attempt, f domain.Finalization) error {
	tag, err := x.t.Exec(ctx, `
		UPDATE attempts SET status = $2, finished_at = $3, score = $4, max_score = $5,
		       answers_changed = $6, last_activity_at = $7
		 WHERE id = $1 AND status = 'in_progress'`,
		a.ID, a.Status, a.FinishedAt, a.Score, a.MaxScore, a.AnswersChanged, a.LastActivityAt)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrAttemptFinalized
	}
	var key *string
	if f.IdempotencyKey != "" {
		key = &f.IdempotencyKey
	}
	return x.exec(ctx, `INSERT INTO attempt_finalizations (attempt_id, user_id, reason, idempotency_key, finalized_at, event_id)
		VALUES ($1,$2,$3,$4,$5,$6)`, f.AttemptID, f.UserID, f.Reason, key, f.FinalizedAt, f.EventID)
}

func scanFinalization(row pgx.Row) (*domain.Finalization, error) {
	var f domain.Finalization
	var key *string
	if err := row.Scan(&f.AttemptID, &f.UserID, &f.Reason, &key, &f.FinalizedAt, &f.EventID); err != nil {
		return nil, mapErr(err)
	}
	if key != nil {
		f.IdempotencyKey = *key
	}
	return &f, nil
}

func (x *tx) GetFinalization(ctx context.Context, attemptID string) (*domain.Finalization, error) {
	return scanFinalization(x.t.QueryRow(ctx, `SELECT attempt_id, user_id, reason, idempotency_key, finalized_at, event_id
		FROM attempt_finalizations WHERE attempt_id = $1`, attemptID))
}

func (x *tx) FinalizationByKey(ctx context.Context, userID, key string) (*domain.Finalization, error) {
	return scanFinalization(x.t.QueryRow(ctx, `SELECT attempt_id, user_id, reason, idempotency_key, finalized_at, event_id
		FROM attempt_finalizations WHERE user_id = $1 AND idempotency_key = $2`, userID, key))
}

func (x *tx) ExpiredAttempts(ctx context.Context, before time.Time, limit int) ([]string, error) {
	rows, err := x.t.Query(ctx, `SELECT id FROM attempts WHERE status = 'in_progress' AND NOT sim AND deadline_at < $1
		ORDER BY deadline_at LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (x *tx) Enqueue(ctx context.Context, topic, key string, env events.Envelope) error {
	return outbox.Enqueue(ctx, x.t, topic, key, env)
}
