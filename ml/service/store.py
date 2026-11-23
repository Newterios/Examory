"""Storage port and its PostgreSQL implementation (database ``anomaly``).

Raw quiz events are stored once per event_id (idempotent ingestion); each
submitted attempt has a scoring state; produced risk events live in an
outbox until the broker acknowledged them (at-least-once, stable event_id).
"""
from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Protocol

import psycopg
from psycopg.rows import tuple_row

from .detectors import ModelScore

SCHEMA = """
CREATE TABLE IF NOT EXISTS quiz_events (
    event_id    uuid PRIMARY KEY,
    event_type  text        NOT NULL CHECK (event_type IN ('quiz.started', 'quiz.answered', 'quiz.submitted')),
    attempt_id  text        NOT NULL CHECK (attempt_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    occurred_at timestamptz NOT NULL,
    envelope    jsonb       NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS quiz_events_attempt_idx ON quiz_events (attempt_id);
CREATE INDEX IF NOT EXISTS quiz_events_time_idx ON quiz_events (occurred_at);
CREATE TABLE IF NOT EXISTS attempts (
    attempt_id   text PRIMARY KEY CHECK (attempt_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    group_id     text        NOT NULL,
    quiz_id      text        NOT NULL,
    student_id   text        NOT NULL,
    submitted_at timestamptz NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now(),
    status       text        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'scored', 'flagged', 'not_scorable')),
    raw_score    double precision,
    level        text CHECK (level IN ('low', 'medium', 'high')),
    detail       text CHECK (char_length(detail) <= 2000),
    scored_at    timestamptz
);
CREATE INDEX IF NOT EXISTS attempts_cohort_idx ON attempts (group_id, quiz_id);
CREATE INDEX IF NOT EXISTS attempts_pending_idx ON attempts (submitted_at) WHERE status = 'pending';
CREATE TABLE IF NOT EXISTS attempt_model_scores (
    attempt_id text NOT NULL REFERENCES attempts(attempt_id),
    model_key text NOT NULL,
    artifact_version text NOT NULL,
    raw_score double precision NOT NULL,
    level text NOT NULL CHECK (level IN ('low', 'medium', 'high')),
    is_shadow boolean NOT NULL,
    PRIMARY KEY (attempt_id, model_key)
);
CREATE TABLE IF NOT EXISTS risk_outbox (
    risk_id      text PRIMARY KEY,
    attempt_id   text        NOT NULL UNIQUE REFERENCES attempts (attempt_id),
    event_id     uuid        NOT NULL UNIQUE,
    topic        text        NOT NULL,
    partition_key text       NOT NULL,
    envelope     text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);
"""
LOCK_KEY = 0x414E4F4D414C59  # "ANOMALY"


@dataclass(frozen=True)
class Pending:
    attempt_id: str
    group_id: str
    quiz_id: str
    submitted_at: datetime
    received_at: datetime


@dataclass(frozen=True)
class OutboxRow:
    risk_id: str
    topic: str
    key: str
    envelope: str


class Store(Protocol):
    def add_event(self, envelope: dict[str, Any], occurred_at: datetime) -> bool: ...
    def watermark(self) -> datetime | None: ...
    def now(self) -> datetime: ...
    def pending(self, limit: int) -> list[Pending]: ...
    def cohort_events(self, group_id: str, quiz_id: str) -> dict[str, list[dict[str, Any]]]: ...
    def finish(self, attempt_id: str, status: str, raw_score: float | None, level: str | None,
               detail: str, outbox: OutboxRow | None, event_id: str | None, scores: tuple[ModelScore, ...]) -> bool: ...
    def unpublished(self, limit: int) -> list[OutboxRow]: ...
    def mark_published(self, risk_ids: list[str]) -> None: ...
    def ping(self) -> None: ...


class PostgresStore:
    """psycopg 3, parameterised SQL only, one connection (single-threaded loop)."""

    def __init__(self, dsn: str) -> None:
        self._conn = psycopg.connect(dsn, autocommit=False, connect_timeout=5,
                                     options="-c statement_timeout=10000 -c lock_timeout=5000 -c timezone=UTC",
                                     row_factory=tuple_row)

    def migrate(self) -> None:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT pg_advisory_xact_lock(%s)", (LOCK_KEY,))
            cur.execute(SCHEMA)

    def close(self) -> None:
        self._conn.close()

    def ping(self) -> None:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT 1")

    def add_event(self, envelope: dict[str, Any], occurred_at: datetime) -> bool:
        payload = envelope["payload"]
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute(
                "INSERT INTO quiz_events (event_id, event_type, attempt_id, occurred_at, envelope) "
                "VALUES (%s, %s, %s, %s, %s) ON CONFLICT (event_id) DO NOTHING",
                (envelope["event_id"], envelope["event_type"], payload["attempt_id"], occurred_at, json.dumps(envelope)))
            inserted = cur.rowcount == 1
            if envelope["event_type"] == "quiz.submitted":
                cur.execute(
                    "INSERT INTO attempts (attempt_id, group_id, quiz_id, student_id, submitted_at) "
                    "VALUES (%s, %s, %s, %s, %s) ON CONFLICT (attempt_id) DO NOTHING",
                    (payload["attempt_id"], envelope["context"]["group_id"], payload["quiz_id"],
                     envelope["actor"]["user_id"], occurred_at))
        return inserted

    def watermark(self) -> datetime | None:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT max(occurred_at) FROM quiz_events")
            row = cur.fetchone()
            return row[0] if row else None

    def now(self) -> datetime:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT now()")
            return cur.fetchone()[0]

    def pending(self, limit: int) -> list[Pending]:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT attempt_id, group_id, quiz_id, submitted_at, received_at FROM attempts "
                        "WHERE status = 'pending' ORDER BY submitted_at LIMIT %s", (limit,))
            return [Pending(*row) for row in cur.fetchall()]

    def cohort_events(self, group_id: str, quiz_id: str) -> dict[str, list[dict[str, Any]]]:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT e.attempt_id, e.envelope FROM quiz_events e JOIN attempts a ON a.attempt_id = e.attempt_id "
                        "WHERE a.group_id = %s AND a.quiz_id = %s ORDER BY e.occurred_at, e.event_id",
                        (group_id, quiz_id))
            out: dict[str, list[dict[str, Any]]] = {}
            for attempt_id, envelope in cur.fetchall():
                out.setdefault(attempt_id, []).append(envelope)
            return out

    def finish(self, attempt_id: str, status: str, raw_score: float | None, level: str | None,
               detail: str, outbox: OutboxRow | None, event_id: str | None, scores: tuple[ModelScore, ...]) -> bool:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("UPDATE attempts SET status = %s, raw_score = %s, level = %s, detail = %s, scored_at = now() "
                        "WHERE attempt_id = %s AND status = 'pending'",
                        (status, raw_score, level, detail[:2000], attempt_id))
            if cur.rowcount != 1:
                return False  # already scored (another replica or a re-run)
            if scores:
                cur.executemany(
                    "INSERT INTO attempt_model_scores (attempt_id, model_key, artifact_version, raw_score, level, is_shadow) "
                    "VALUES (%s, %s, %s, %s, %s, %s) ON CONFLICT (attempt_id, model_key) DO NOTHING",
                    [(attempt_id, s.key, s.version, s.raw, s.level, s.shadow) for s in scores])
            if outbox is not None:
                cur.execute("INSERT INTO risk_outbox (risk_id, attempt_id, event_id, topic, partition_key, envelope) "
                            "VALUES (%s, %s, %s, %s, %s, %s) ON CONFLICT (risk_id) DO NOTHING",
                            (outbox.risk_id, attempt_id, event_id, outbox.topic, outbox.key, outbox.envelope))
        return True

    def unpublished(self, limit: int) -> list[OutboxRow]:
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("SELECT risk_id, topic, partition_key, envelope FROM risk_outbox WHERE published_at IS NULL "
                        "ORDER BY created_at LIMIT %s", (limit,))
            return [OutboxRow(*row) for row in cur.fetchall()]

    def mark_published(self, risk_ids: list[str]) -> None:
        if not risk_ids:
            return
        with self._conn.transaction(), self._conn.cursor() as cur:
            cur.execute("UPDATE risk_outbox SET published_at = now() WHERE risk_id = ANY(%s)", (risk_ids,))
