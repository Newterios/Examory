package postgres

import (
	"context"
	"encoding/json"
	"github.com/Newterios/Examory/services/grading/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (x *tx) LockGrade(ctx context.Context, attempt, teacher, key string) (*domain.AttemptSummary, error) {
	if _, err := x.t.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 17))", teacher+":"+key); err != nil {
		return nil, err
	}
	var id string
	if err := x.t.QueryRow(ctx, "SELECT attempt_id FROM attempts WHERE attempt_id=$1 FOR UPDATE", attempt).Scan(&id); err != nil {
		return nil, mapErr(err)
	}
	return x.GetAttempt(ctx, id)
}

const gradeColumns = "attempt_id,revision,grade,max_grade,feedback,rubric_scores,graded_by::text,graded_at,idempotency_key,event_id::text"

func scanGrade(row pgx.Row) (*domain.Grade, error) {
	var g domain.Grade
	var criteria []byte
	if err := row.Scan(&g.AttemptID, &g.Revision, &g.Grade, &g.MaxGrade, &g.Feedback, &criteria, &g.GradedBy, &g.At, &g.Key, &g.EventID); err != nil {
		return nil, mapErr(err)
	}
	if err := json.Unmarshal(criteria, &g.Criteria); err != nil {
		return nil, err
	}
	return &g, nil
}
func (x *tx) GradeByKey(ctx context.Context, teacher, key string) (*domain.Grade, error) {
	return scanGrade(x.t.QueryRow(ctx, "SELECT "+gradeColumns+" FROM grade_revisions WHERE graded_by=$1 AND idempotency_key=$2", teacher, key))
}
func (x *tx) LastGradeRevision(ctx context.Context, attempt string) (int, error) {
	var n int
	err := x.t.QueryRow(ctx, "SELECT coalesce(max(revision),0) FROM grade_revisions WHERE attempt_id=$1", attempt).Scan(&n)
	return n, err
}
func (x *tx) InsertGrade(ctx context.Context, g domain.Grade) error {
	raw, err := json.Marshal(g.Criteria)
	if err != nil {
		return err
	}
	_, err = x.t.Exec(ctx, "INSERT INTO grade_revisions(attempt_id,revision,grade,max_grade,feedback,rubric_scores,graded_by,graded_at,idempotency_key,event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", g.AttemptID, g.Revision, g.Grade, g.MaxGrade, g.Feedback, raw, g.GradedBy, g.At, g.Key, g.EventID)
	return mapErr(err)
}
func (x *tx) Grades(ctx context.Context, attempt string, history bool) ([]domain.Grade, error) {
	q := "SELECT " + gradeColumns + " FROM grade_revisions WHERE attempt_id=$1 ORDER BY revision DESC"
	if !history {
		q += " LIMIT 1"
	}
	rows, err := x.t.Query(ctx, q, attempt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Grade
	for rows.Next() {
		g, err := scanGrade(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}
