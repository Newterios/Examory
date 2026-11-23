-- quizzes schema v1 (LMS-38). Invariants are enforced here, not only in code:
-- published quizzes are immutable, attempts follow in_progress -> submitted |
-- timed_out, finalized attempts are frozen, one active attempt per student and
-- quiz, and exactly one finalization (hence one quiz.submitted) per attempt.

CREATE TABLE quizzes (
    id             uuid PRIMARY KEY,
    owner_id       uuid        NOT NULL,
    course_id      text        NOT NULL CHECK (course_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    title          text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    time_limit_sec integer     NOT NULL CHECK (time_limit_sec BETWEEN 30 AND 14400),
    max_attempts   integer     NOT NULL CHECK (max_attempts BETWEEN 1 AND 20),
    show_score     boolean     NOT NULL,
    status         text        NOT NULL CHECK (status IN ('draft', 'published')),
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    published_at   timestamptz,
    CHECK ((status = 'published') = (published_at IS NOT NULL))
);
CREATE INDEX quizzes_owner_idx ON quizzes (owner_id, id);

CREATE TABLE variants (
    id      uuid PRIMARY KEY,
    quiz_id uuid    NOT NULL REFERENCES quizzes (id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal BETWEEN 0 AND 19),
    UNIQUE (quiz_id, ordinal),
    UNIQUE (id, quiz_id)
);

CREATE TABLE questions (
    id       uuid PRIMARY KEY,
    quiz_id  uuid    NOT NULL REFERENCES quizzes (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 99),
    text     text    NOT NULL CHECK (char_length(text) BETWEEN 1 AND 2000),
    points   integer NOT NULL CHECK (points BETWEEN 1 AND 100),
    topic_id text CHECK (topic_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    UNIQUE (quiz_id, position),
    UNIQUE (id, quiz_id)
);

CREATE TABLE question_options (
    id          uuid PRIMARY KEY,
    question_id uuid    NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    position    integer NOT NULL CHECK (position BETWEEN 0 AND 7),
    text        text    NOT NULL CHECK (char_length(text) BETWEEN 1 AND 500),
    is_correct  boolean NOT NULL,
    UNIQUE (question_id, position)
);

-- A variant may only contain questions of its own quiz (composite FKs).
CREATE TABLE variant_questions (
    variant_id  uuid    NOT NULL,
    quiz_id     uuid    NOT NULL,
    question_id uuid    NOT NULL,
    position    integer NOT NULL CHECK (position BETWEEN 0 AND 199),
    PRIMARY KEY (variant_id, position),
    UNIQUE (variant_id, question_id),
    FOREIGN KEY (variant_id, quiz_id) REFERENCES variants (id, quiz_id) ON DELETE CASCADE,
    FOREIGN KEY (question_id, quiz_id) REFERENCES questions (id, quiz_id) ON DELETE CASCADE
);

CREATE TABLE quiz_assignments (
    quiz_id     uuid        NOT NULL REFERENCES quizzes (id),
    group_id    uuid        NOT NULL,
    opens_at    timestamptz NOT NULL,
    closes_at   timestamptz NOT NULL,
    assigned_by uuid        NOT NULL,
    assigned_at timestamptz NOT NULL,
    PRIMARY KEY (quiz_id, group_id),
    CHECK (closes_at > opens_at),
    CHECK (closes_at - opens_at <= interval '366 days')
);
CREATE INDEX quiz_assignments_group_idx ON quiz_assignments (group_id, quiz_id);

-- Immutability of published quizzes ------------------------------------------
CREATE FUNCTION quizzes_guard_quiz() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'published' THEN
            RAISE EXCEPTION 'quiz % is published and cannot be deleted', OLD.id USING ERRCODE = 'P0001';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'published' THEN
        RAISE EXCEPTION 'quiz % is published and immutable', OLD.id USING ERRCODE = 'P0001';
    END IF;
    IF NEW.id <> OLD.id OR NEW.owner_id <> OLD.owner_id OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'quiz identity columns are immutable' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER quizzes_guard BEFORE UPDATE OR DELETE ON quizzes
    FOR EACH ROW EXECUTE FUNCTION quizzes_guard_quiz();

CREATE FUNCTION quizzes_guard_content() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    qid uuid;
    st  text;
BEGIN
    IF TG_TABLE_NAME = 'question_options' THEN
        SELECT q.quiz_id INTO qid FROM questions q
         WHERE q.id = CASE WHEN TG_OP = 'DELETE' THEN OLD.question_id ELSE NEW.question_id END;
    ELSIF TG_OP = 'DELETE' THEN
        qid := OLD.quiz_id;
    ELSE
        qid := NEW.quiz_id;
    END IF;
    SELECT status INTO st FROM quizzes WHERE id = qid;
    IF st = 'published' THEN
        RAISE EXCEPTION 'content of published quiz % is immutable', qid USING ERRCODE = 'P0001';
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER variants_guard BEFORE INSERT OR UPDATE OR DELETE ON variants
    FOR EACH ROW EXECUTE FUNCTION quizzes_guard_content();
CREATE TRIGGER questions_guard BEFORE INSERT OR UPDATE OR DELETE ON questions
    FOR EACH ROW EXECUTE FUNCTION quizzes_guard_content();
CREATE TRIGGER question_options_guard BEFORE INSERT OR UPDATE OR DELETE ON question_options
    FOR EACH ROW EXECUTE FUNCTION quizzes_guard_content();
CREATE TRIGGER variant_questions_guard BEFORE INSERT OR UPDATE OR DELETE ON variant_questions
    FOR EACH ROW EXECUTE FUNCTION quizzes_guard_content();

-- Attempts --------------------------------------------------------------------
CREATE TABLE attempts (
    id               uuid PRIMARY KEY,
    quiz_id          uuid        NOT NULL REFERENCES quizzes (id),
    variant_id       uuid        NOT NULL,
    user_id          uuid        NOT NULL,
    group_id         uuid        NOT NULL,
    course_id        text        NOT NULL CHECK (course_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    attempt_no       integer     NOT NULL CHECK (attempt_no BETWEEN 1 AND 20),
    status           text        NOT NULL CHECK (status IN ('in_progress', 'submitted', 'timed_out')),
    sim              boolean     NOT NULL,
    shuffle_seed     bytea       NOT NULL CHECK (length(shuffle_seed) = 16),
    time_limit_sec   integer     NOT NULL CHECK (time_limit_sec BETWEEN 30 AND 14400),
    show_score       boolean     NOT NULL,
    started_at       timestamptz NOT NULL,
    deadline_at      timestamptz NOT NULL CHECK (deadline_at > started_at),
    last_activity_at timestamptz NOT NULL CHECK (last_activity_at >= started_at),
    finished_at      timestamptz CHECK (finished_at >= started_at),
    score            integer CHECK (score >= 0),
    max_score        integer CHECK (max_score > 0),
    answers_changed  integer     NOT NULL DEFAULT 0 CHECK (answers_changed >= 0),
    FOREIGN KEY (variant_id, quiz_id) REFERENCES variants (id, quiz_id),
    UNIQUE (quiz_id, user_id, attempt_no),
    CHECK ((status = 'in_progress') = (finished_at IS NULL)),
    CHECK ((status = 'in_progress') = (score IS NULL)),
    CHECK ((score IS NULL) = (max_score IS NULL)),
    CHECK (score IS NULL OR score <= max_score)
);
-- No parallel active attempt of the same quiz by the same student.
CREATE UNIQUE INDEX attempts_one_active_idx ON attempts (quiz_id, user_id) WHERE status = 'in_progress';
CREATE INDEX attempts_sweep_idx ON attempts (deadline_at) WHERE status = 'in_progress' AND NOT sim;
CREATE INDEX attempts_quiz_idx ON attempts (quiz_id, id);

-- State machine: only in_progress rows change; terminal rows are frozen.
CREATE FUNCTION attempts_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'attempts cannot be deleted' USING ERRCODE = 'P0001';
    END IF;
    IF OLD.status <> 'in_progress' THEN
        RAISE EXCEPTION 'attempt % is finalized', OLD.id USING ERRCODE = 'P0001';
    END IF;
    IF NEW.status NOT IN ('in_progress', 'submitted', 'timed_out') THEN
        RAISE EXCEPTION 'invalid attempt transition' USING ERRCODE = 'P0001';
    END IF;
    IF (NEW.id, NEW.quiz_id, NEW.variant_id, NEW.user_id, NEW.group_id, NEW.attempt_no, NEW.sim,
        NEW.shuffle_seed, NEW.started_at, NEW.deadline_at)
       IS DISTINCT FROM
       (OLD.id, OLD.quiz_id, OLD.variant_id, OLD.user_id, OLD.group_id, OLD.attempt_no, OLD.sim,
        OLD.shuffle_seed, OLD.started_at, OLD.deadline_at) THEN
        RAISE EXCEPTION 'attempt identity columns are immutable' USING ERRCODE = 'P0001';
    END IF;
    IF NEW.last_activity_at < OLD.last_activity_at OR NEW.answers_changed < OLD.answers_changed THEN
        RAISE EXCEPTION 'attempt progress cannot go backwards' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER attempts_guard BEFORE UPDATE OR DELETE ON attempts
    FOR EACH ROW EXECUTE FUNCTION attempts_guard();

CREATE TABLE attempt_questions (
    attempt_id     uuid    NOT NULL REFERENCES attempts (id),
    question_id    uuid    NOT NULL REFERENCES questions (id),
    position       integer NOT NULL CHECK (position BETWEEN 0 AND 199),
    first_shown_at timestamptz,
    PRIMARY KEY (attempt_id, question_id),
    UNIQUE (attempt_id, position)
);

CREATE TABLE answers (
    attempt_id   uuid        NOT NULL,
    question_id  uuid        NOT NULL,
    selected     uuid[]      NOT NULL CHECK (cardinality(selected) BETWEEN 1 AND 8),
    answer_hash  text        NOT NULL CHECK (answer_hash ~ '^[0-9a-f]{64}$'),
    saved_at     timestamptz NOT NULL,
    change_count integer     NOT NULL CHECK (change_count >= 0),
    PRIMARY KEY (attempt_id, question_id),
    FOREIGN KEY (attempt_id, question_id) REFERENCES attempt_questions (attempt_id, question_id)
);

-- Append-only log of accepted answer saves (one row per quiz.answered).
CREATE TABLE answer_log (
    id                          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    attempt_id                  uuid        NOT NULL,
    question_id                 uuid        NOT NULL,
    selected                    uuid[]      NOT NULL CHECK (cardinality(selected) BETWEEN 1 AND 8),
    answer_hash                 text        NOT NULL CHECK (answer_hash ~ '^[0-9a-f]{64}$'),
    is_change                   boolean     NOT NULL,
    saved_at                    timestamptz NOT NULL,
    time_since_shown_ms         bigint      NOT NULL CHECK (time_since_shown_ms >= 0),
    time_since_attempt_start_ms bigint      NOT NULL CHECK (time_since_attempt_start_ms >= 0),
    event_id                    uuid        NOT NULL UNIQUE,
    FOREIGN KEY (attempt_id, question_id) REFERENCES attempt_questions (attempt_id, question_id)
);

-- Children of an attempt may change only while it is in progress; the first
-- shown time is write-once; the answer log is append-only.
CREATE FUNCTION attempt_children_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    st text;
BEGIN
    IF TG_TABLE_NAME = 'answer_log' AND TG_OP <> 'INSERT' THEN
        RAISE EXCEPTION 'answer_log is append-only' USING ERRCODE = 'P0001';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '% rows cannot be deleted', TG_TABLE_NAME USING ERRCODE = 'P0001';
    END IF;
    SELECT status INTO st FROM attempts WHERE id = NEW.attempt_id;
    IF st IS DISTINCT FROM 'in_progress' THEN
        RAISE EXCEPTION 'attempt % is not in progress', NEW.attempt_id USING ERRCODE = 'P0001';
    END IF;
    IF TG_TABLE_NAME = 'attempt_questions' AND TG_OP = 'UPDATE' THEN
        IF OLD.first_shown_at IS NOT NULL AND NEW.first_shown_at IS DISTINCT FROM OLD.first_shown_at THEN
            RAISE EXCEPTION 'first_shown_at is write-once' USING ERRCODE = 'P0001';
        END IF;
        IF (NEW.attempt_id, NEW.question_id, NEW.position) IS DISTINCT FROM (OLD.attempt_id, OLD.question_id, OLD.position) THEN
            RAISE EXCEPTION 'attempt question identity is immutable' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER attempt_questions_guard BEFORE INSERT OR UPDATE OR DELETE ON attempt_questions
    FOR EACH ROW EXECUTE FUNCTION attempt_children_guard();
CREATE TRIGGER answers_guard BEFORE INSERT OR UPDATE OR DELETE ON answers
    FOR EACH ROW EXECUTE FUNCTION attempt_children_guard();
CREATE TRIGGER answer_log_guard BEFORE INSERT OR UPDATE OR DELETE ON answer_log
    FOR EACH ROW EXECUTE FUNCTION attempt_children_guard();

-- Exactly one finalization per attempt (primary key), hence exactly one
-- quiz.submitted. Idempotency keys are unique per student.
CREATE TABLE attempt_finalizations (
    attempt_id      uuid PRIMARY KEY REFERENCES attempts (id),
    user_id         uuid        NOT NULL,
    reason          text        NOT NULL CHECK (reason IN ('submitted', 'timed_out')),
    idempotency_key text CHECK (idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
    finalized_at    timestamptz NOT NULL,
    event_id        uuid        NOT NULL UNIQUE,
    UNIQUE (user_id, idempotency_key),
    CHECK ((reason = 'submitted') = (idempotency_key IS NOT NULL))
);

CREATE FUNCTION finalizations_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'attempt_finalizations is append-only' USING ERRCODE = 'P0001';
END $$;
CREATE TRIGGER attempt_finalizations_guard BEFORE UPDATE OR DELETE ON attempt_finalizations
    FOR EACH ROW EXECUTE FUNCTION finalizations_guard();
