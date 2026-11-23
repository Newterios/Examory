CREATE TABLE assignments (
 id uuid PRIMARY KEY, teacher_id uuid NOT NULL, course_id uuid NOT NULL, group_id uuid NOT NULL, title text NOT NULL, instructions text NOT NULL,
 deadline timestamptz NOT NULL, max_attempts integer NOT NULL CHECK(max_attempts BETWEEN 1 AND 100),
 max_bytes bigint NOT NULL CHECK(max_bytes BETWEEN 1 AND 2097152), accepted_types text[] NOT NULL,
 kind text NOT NULL CHECK(kind IN ('text','code','file')), language text NOT NULL, created_at timestamptz NOT NULL
);
CREATE INDEX assignments_course ON assignments(course_id,id);
CREATE INDEX assignments_group ON assignments(group_id,id);
CREATE TABLE submissions (
 id uuid PRIMARY KEY, assignment_id uuid NOT NULL REFERENCES assignments, user_id uuid NOT NULL,
 attempt integer NOT NULL CHECK(attempt>0), submitted_at timestamptz NOT NULL, late boolean NOT NULL,
 request_key text NOT NULL, request_hash text NOT NULL, files jsonb NOT NULL, text_ref text NOT NULL,
 UNIQUE(assignment_id,user_id,attempt), UNIQUE(assignment_id,user_id,request_key)
);
CREATE INDEX submissions_assignment ON submissions(assignment_id,user_id,id);
