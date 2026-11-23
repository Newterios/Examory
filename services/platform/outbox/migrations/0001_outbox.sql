-- Transactional outbox (platform/outbox). Rows are inserted in the same
-- transaction as the state change and published by the relay in id order.
CREATE TABLE outbox (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id      uuid        NOT NULL UNIQUE,
    topic         text        NOT NULL CHECK (topic ~ '^[a-z0-9._-]{1,249}$'),
    partition_key text        NOT NULL CHECK (length(partition_key) BETWEEN 1 AND 256),
    event_type    text        NOT NULL CHECK (length(event_type) BETWEEN 3 AND 64),
    envelope      bytea       NOT NULL CHECK (length(envelope) BETWEEN 2 AND 1048576),
    created_at    timestamptz NOT NULL DEFAULT now(),
    published_at  timestamptz,
    attempts      integer     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error    text        CHECK (length(last_error) <= 500)
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
CREATE INDEX outbox_published_idx ON outbox (published_at) WHERE published_at IS NOT NULL;
