-- Idempotent consumers (platform/consumer, NFR-12): one row per processed
-- event per consumer group, written in the same transaction as the effect.
CREATE TABLE processed_events (
    consumer     text        NOT NULL CHECK (length(consumer) BETWEEN 1 AND 128),
    event_id     uuid        NOT NULL,
    event_type   text        NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
