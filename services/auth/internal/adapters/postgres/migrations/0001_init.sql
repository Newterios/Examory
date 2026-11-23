-- auth service schema (database "auth"). Invariants are enforced here, not
-- only in code.

CREATE TABLE users (
    id             uuid        PRIMARY KEY,
    email          text        NOT NULL,
    display_name   text        NOT NULL,
    role           text        NOT NULL,
    status         text        NOT NULL DEFAULT 'active',
    password_hash  text        NOT NULL,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    deactivated_at timestamptz,
    CONSTRAINT users_email_key UNIQUE (email),
    -- Case-insensitive uniqueness: only lower-case addresses are stored.
    CONSTRAINT users_email_check CHECK (length(email) BETWEEN 3 AND 254 AND email = lower(email) AND email ~ '^[^@[:space:]]+@[^@[:space:]]+$'),
    CONSTRAINT users_display_name_check CHECK (length(display_name) BETWEEN 1 AND 100),
    CONSTRAINT users_role_check CHECK (role IN ('student', 'teacher', 'admin', 'system')),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'deactivated')),
    CONSTRAINT users_password_hash_check CHECK (password_hash LIKE '$argon2id$%' AND length(password_hash) <= 256),
    CONSTRAINT users_deactivated_check CHECK ((status = 'deactivated') = (deactivated_at IS NOT NULL)),
    CONSTRAINT users_time_check CHECK (updated_at >= created_at)
);

CREATE TABLE groups (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    course_id  text,
    created_by uuid        NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL,
    CONSTRAINT groups_name_check CHECK (length(name) BETWEEN 1 AND 100),
    CONSTRAINT groups_course_id_check CHECK (course_id ~ '^[A-Za-z0-9_-]{1,64}$')
);
CREATE UNIQUE INDEX groups_name_key ON groups (lower(name));

-- A user belongs to at most one group (PK on user_id); the access token
-- carries that group as group_id.
CREATE TABLE group_members (
    user_id  uuid        PRIMARY KEY REFERENCES users (id),
    group_id uuid        NOT NULL REFERENCES groups (id),
    added_by uuid        NOT NULL REFERENCES users (id),
    added_at timestamptz NOT NULL
);
CREATE INDEX group_members_group_idx ON group_members (group_id);

-- Only students may be group members, and a member cannot stop being a
-- student while still in a group.
CREATE FUNCTION group_members_student_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.user_id AND role = 'student') THEN
        RAISE EXCEPTION 'only students can be group members' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER group_members_student_only BEFORE INSERT OR UPDATE ON group_members
    FOR EACH ROW EXECUTE FUNCTION group_members_student_only();

CREATE FUNCTION users_member_role_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.role <> 'student' AND EXISTS (SELECT 1 FROM group_members WHERE user_id = NEW.id) THEN
        RAISE EXCEPTION 'remove the user from the group before changing the role' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER users_member_role_guard BEFORE UPDATE OF role ON users
    FOR EACH ROW EXECUTE FUNCTION users_member_role_guard();

-- Security and admin audit trail (NFR-16). Append-only; never holds secrets.
CREATE TABLE audit_log (
    id          uuid        PRIMARY KEY,
    at          timestamptz NOT NULL,
    actor_id    uuid,
    action      text        NOT NULL,
    target_type text,
    target_id   uuid,
    outcome     text        NOT NULL,
    ip          inet,
    detail      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT audit_action_check CHECK (action ~ '^[a-z_]{2,32}\.[a-z_]{2,32}$'),
    CONSTRAINT audit_outcome_check CHECK (outcome IN ('success', 'failure', 'denied')),
    CONSTRAINT audit_target_type_check CHECK (target_type IN ('user', 'group')),
    CONSTRAINT audit_detail_check CHECK (jsonb_typeof(detail) = 'object' AND pg_column_size(detail) <= 2048)
);
CREATE INDEX audit_log_at_idx ON audit_log (at);
CREATE INDEX audit_log_target_idx ON audit_log (target_id, at);

CREATE FUNCTION audit_log_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only' USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER audit_log_append_only BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();
