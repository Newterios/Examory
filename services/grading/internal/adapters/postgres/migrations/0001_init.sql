-- grading schema v1 (M4). Invariants in the schema: one review per risk,
-- risk status mirrors its review and changes only open -> confirmed|dismissed,
-- reviews and the audit log are append-only. There is deliberately no grade
-- table: a risk signal never changes a grade (FR-26).

CREATE TABLE attempts (
    attempt_id      text PRIMARY KEY CHECK (attempt_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    student_id      text        NOT NULL CHECK (student_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    quiz_id         text        NOT NULL CHECK (quiz_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    variant_id      text        NOT NULL CHECK (variant_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    course_id       text        NOT NULL CHECK (course_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    group_id        text        NOT NULL CHECK (group_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    score           integer     NOT NULL CHECK (score >= 0),
    max_score       integer     NOT NULL CHECK (max_score > 0),
    duration_ms     bigint      NOT NULL CHECK (duration_ms >= 0),
    answers_changed integer     NOT NULL CHECK (answers_changed >= 0),
    reason          text        NOT NULL CHECK (reason IN ('', 'submitted', 'timed_out')),
    submitted_at    timestamptz NOT NULL,
    event_id        uuid        NOT NULL UNIQUE,
    CHECK (score <= max_score)
);
CREATE INDEX attempts_quiz_idx ON attempts (quiz_id);

CREATE TABLE risks (
    id            uuid PRIMARY KEY,
    risk_id       text             NOT NULL UNIQUE CHECK (risk_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    event_id      uuid             NOT NULL UNIQUE,
    category      text             NOT NULL CHECK (category IN ('risk.behavior_anomaly')),
    target_type   text             NOT NULL CHECK (target_type IN ('submission', 'attempt', 'exam_session', 'variant')),
    target_id     text             NOT NULL CHECK (target_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    score         double precision NOT NULL CHECK (score >= 0 AND score <= 1),
    level         text             NOT NULL CHECK (level IN ('low', 'medium', 'high')),
    model_name    text             NOT NULL CHECK (char_length(model_name) BETWEEN 1 AND 64),
    model_version text             NOT NULL CHECK (char_length(model_version) BETWEEN 1 AND 32),
    explanation   text             NOT NULL CHECK (char_length(explanation) BETWEEN 1 AND 4000),
    evidence      jsonb            NOT NULL CHECK (jsonb_typeof(evidence) = 'object' AND pg_column_size(evidence) <= 65536),
    course_id     text CHECK (course_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    group_id      text CHECK (group_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    producer      text             NOT NULL CHECK (char_length(producer) BETWEEN 1 AND 64),
    occurred_at   timestamptz      NOT NULL,
    received_at   timestamptz      NOT NULL,
    status        text             NOT NULL CHECK (status IN ('open', 'confirmed', 'dismissed')),
    version       integer          NOT NULL CHECK (version >= 1)
);
CREATE INDEX risks_status_idx ON risks (status, id);
CREATE INDEX risks_course_idx ON risks (course_id, id);
CREATE INDEX risks_target_idx ON risks (target_type, target_id);

-- Exactly one review per risk (primary key on risk).
CREATE TABLE reviews (
    risk_pk         uuid PRIMARY KEY REFERENCES risks (id),
    decision        text        NOT NULL CHECK (decision IN ('confirmed', 'dismissed')),
    comment         text        NOT NULL CHECK (char_length(comment) BETWEEN 1 AND 2000),
    reviewer_id     uuid        NOT NULL,
    reviewer_role   text        NOT NULL CHECK (reviewer_role = 'teacher'),
    reviewed_at     timestamptz NOT NULL,
    idempotency_key text        NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
    event_id        uuid        NOT NULL UNIQUE,
    UNIQUE (reviewer_id, idempotency_key)
);

CREATE FUNCTION grading_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'P0001';
END $$;
CREATE TRIGGER reviews_append_only BEFORE UPDATE OR DELETE ON reviews
    FOR EACH ROW EXECUTE FUNCTION grading_append_only();

-- Risk content is immutable; status moves once, to the decision of its review.
CREATE FUNCTION risks_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    d text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'risks cannot be deleted' USING ERRCODE = 'P0001';
    END IF;
    IF (NEW.id, NEW.risk_id, NEW.event_id, NEW.category, NEW.target_type, NEW.target_id, NEW.score, NEW.level,
        NEW.model_name, NEW.model_version, NEW.explanation, NEW.evidence, NEW.course_id, NEW.group_id,
        NEW.producer, NEW.occurred_at, NEW.received_at)
       IS DISTINCT FROM
       (OLD.id, OLD.risk_id, OLD.event_id, OLD.category, OLD.target_type, OLD.target_id, OLD.score, OLD.level,
        OLD.model_name, OLD.model_version, OLD.explanation, OLD.evidence, OLD.course_id, OLD.group_id,
        OLD.producer, OLD.occurred_at, OLD.received_at) THEN
        RAISE EXCEPTION 'risk signal content is immutable' USING ERRCODE = 'P0001';
    END IF;
    IF OLD.status <> 'open' THEN
        RAISE EXCEPTION 'risk % is already reviewed', OLD.id USING ERRCODE = 'P0001';
    END IF;
    SELECT decision INTO d FROM reviews WHERE risk_pk = NEW.id;
    IF NEW.status <> 'open' AND d IS DISTINCT FROM NEW.status THEN
        RAISE EXCEPTION 'risk status must match its review' USING ERRCODE = 'P0001';
    END IF;
    IF NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'risk version must increase by one' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER risks_guard BEFORE UPDATE OR DELETE ON risks
    FOR EACH ROW EXECUTE FUNCTION risks_guard();

-- NFR-16: actor, target, action, timestamp, outcome. Ids and codes only.
CREATE TABLE audit_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at          timestamptz NOT NULL,
    actor_id    text        NOT NULL CHECK (char_length(actor_id) BETWEEN 1 AND 64),
    actor_role  text        NOT NULL CHECK (actor_role IN ('student', 'teacher', 'admin', 'system')),
    action      text        NOT NULL CHECK (char_length(action) BETWEEN 1 AND 64),
    target_type text        NOT NULL CHECK (char_length(target_type) BETWEEN 1 AND 32),
    target_id   text        NOT NULL CHECK (char_length(target_id) BETWEEN 1 AND 64),
    outcome     text        NOT NULL CHECK (char_length(outcome) BETWEEN 1 AND 64),
    detail      jsonb       NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail) = 'object')
);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id);
CREATE TRIGGER audit_log_append_only BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION grading_append_only();

-- Dead-letter log of events grading could not accept (FR-19: reported, not
-- applied). Excerpts are capped; they may contain identifiers.
CREATE TABLE rejected_events (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    received_at timestamptz NOT NULL,
    topic       text        NOT NULL CHECK (char_length(topic) BETWEEN 1 AND 249),
    kafka_partition integer,
    kafka_offset    bigint,
    event_id    text CHECK (char_length(event_id) <= 64),
    event_type  text CHECK (char_length(event_type) <= 64),
    reason      text        NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 1000),
    excerpt     text        NOT NULL CHECK (char_length(excerpt) <= 4096)
);
