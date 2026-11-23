CREATE TABLE grade_revisions (
 attempt_id text NOT NULL REFERENCES attempts(attempt_id),
 revision integer NOT NULL CHECK (revision>0),
 grade double precision NOT NULL CHECK (grade>=0 AND grade<=max_grade),
 max_grade integer NOT NULL CHECK (max_grade>0),
 feedback text NOT NULL CHECK (char_length(feedback)<=2000),
 rubric_scores jsonb NOT NULL CHECK (jsonb_typeof(rubric_scores)='array'),
 graded_by uuid NOT NULL,
 graded_at timestamptz NOT NULL,
 idempotency_key text NOT NULL,
 event_id uuid NOT NULL UNIQUE,
 PRIMARY KEY(attempt_id,revision),
 UNIQUE(graded_by,idempotency_key)
);
CREATE TRIGGER grade_revisions_append_only BEFORE UPDATE OR DELETE ON grade_revisions
 FOR EACH ROW EXECUTE FUNCTION grading_append_only();
