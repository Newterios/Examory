"""Versioned per-quiz timing and answer references for opt-in live scoring."""
from __future__ import annotations

import hashlib
import json
from collections import defaultdict
from datetime import datetime
from typing import Any

import psycopg
from psycopg.types.json import Jsonb

from ml.anomaly.features import fit_reference, item_key
from ml.service.pipeline_v2 import attempt_v2, history_v2

DDL = """
CREATE TABLE IF NOT EXISTS reference_exclusions (
    attempt_id text PRIMARY KEY, reviewed_event_id uuid NOT NULL, received_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS quiz_reference_versions (
    quiz_id text NOT NULL, version text NOT NULL, source_through timestamptz NOT NULL,
    members text[] NOT NULL, reference_data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (quiz_id, version)
);
CREATE TABLE IF NOT EXISTS quiz_reference_heads (
    quiz_id text PRIMARY KEY, version text NOT NULL, pinned boolean NOT NULL DEFAULT false,
    FOREIGN KEY (quiz_id, version) REFERENCES quiz_reference_versions(quiz_id, version)
);
"""


def fit_quiz(rows: list[dict[str, Any]], minimum: int) -> dict[str, Any] | None:
    """Rows are completed, previously observed attempts after confirmed reviews are removed."""
    from statistics import median

    if minimum < 2:
        raise ValueError("minimum must be at least two attempts")
    if len(rows) < minimum:
        return None
    if len({r["quiz_id"] for r in rows}) != 1:
        raise ValueError("reference must cover exactly one quiz")
    reference = fit_reference(history_v2(rows))
    grouped: dict[str, list[int]] = defaultdict(list)
    for row in rows:
        for q, value in row["first_times"].items():
            grouped[item_key(row, q)].append(value)
    reference["global_median_ms"] = float(median(v for values in grouped.values() for v in values))
    reference["item_median_ms"] = {key: float(median(grouped[key])) for key in reference["item_fast_ms"]}
    reference["population"] = "quiz_history"
    return reference


class QuizReferences:
    def __init__(self, dsn: str) -> None:
        self.connection = psycopg.connect(dsn, autocommit=True)
        self.connection.execute(DDL)

    def close(self) -> None:
        self.connection.close()

    def review(self, env: dict) -> None:
        p = env["payload"]
        if p["target_type"] == "attempt" and p["decision"] == "confirmed":
            self.connection.execute(
                "INSERT INTO reference_exclusions(attempt_id,reviewed_event_id) VALUES(%s,%s) "
                "ON CONFLICT(attempt_id) DO NOTHING", (p["target_id"], env["event_id"]))

    def get(self, quiz: str, submitted: datetime) -> tuple[str, dict] | None:
        # A later confirmation disqualifies a snapshot immediately, including a pinned rollback.
        return self.connection.execute(
            "SELECT v.version,v.reference_data FROM quiz_reference_heads h JOIN quiz_reference_versions v "
            "USING(quiz_id,version) WHERE h.quiz_id=%s AND v.source_through<%s AND NOT EXISTS "
            "(SELECT 1 FROM reference_exclusions x WHERE x.attempt_id=ANY(v.members))", (quiz, submitted)).fetchone()

    def rebuild(self, quiz: str, minimum: int, window: int) -> str | None:
        if not 2 <= minimum <= window:
            raise ValueError("require 2 <= minimum <= window")
        with self.connection.transaction():
            # An advisory lock serializes concurrent rebuilds and operator rollback for one quiz.
            self.connection.execute("SELECT pg_advisory_xact_lock(hashtextextended(%s,0))", (quiz,))
            selected = self.connection.execute(
                "SELECT a.attempt_id FROM attempts a WHERE quiz_id=%s AND status IN ('scored','flagged') "
                "AND NOT EXISTS (SELECT 1 FROM reference_exclusions x WHERE x.attempt_id=a.attempt_id) "
                "ORDER BY submitted_at DESC,attempt_id LIMIT %s", (quiz, window)).fetchall()
            if len(selected) < minimum:
                return None
            data: dict[str, list] = defaultdict(list)
            for aid, env in self.connection.execute(
                "SELECT attempt_id,envelope FROM quiz_events WHERE attempt_id=ANY(%s) ORDER BY occurred_at,event_id",
                ([r[0] for r in selected],)
            ):
                data[aid].append(env)
            rows = []
            for events in data.values():
                try:
                    rows.append(attempt_v2(events))
                except (ValueError, KeyError, TypeError, OverflowError):
                    continue
            reference = fit_quiz(rows, minimum)
            if reference is None:
                return None
            members = sorted(r["attempt_id"] for r in rows)
            through = max(r["submitted_at"] for r in rows)
            signature = json.dumps([quiz, members, through.isoformat(), reference], sort_keys=True,
                                   allow_nan=False, separators=(",", ":"))
            version = hashlib.sha256(signature.encode()).hexdigest()
            self.connection.execute(
                "INSERT INTO quiz_reference_versions(quiz_id,version,source_through,members,reference_data) "
                "VALUES(%s,%s,%s,%s,%s) ON CONFLICT DO NOTHING", (quiz, version, through, members, Jsonb(reference)))
            self.connection.execute(
                "INSERT INTO quiz_reference_heads(quiz_id,version) VALUES(%s,%s) ON CONFLICT (quiz_id) "
                "DO UPDATE SET version=excluded.version WHERE NOT quiz_reference_heads.pinned", (quiz, version))
            return version

    def set_head(self, quiz: str, version: str | None) -> None:
        with self.connection.transaction():
            self.connection.execute("SELECT pg_advisory_xact_lock(hashtextextended(%s,0))", (quiz,))
            if version is None:
                result = self.connection.execute("UPDATE quiz_reference_heads SET pinned=false WHERE quiz_id=%s", (quiz,))
            else:
                result = self.connection.execute(
                    "UPDATE quiz_reference_heads h SET version=v.version,pinned=true FROM quiz_reference_versions v "
                    "WHERE h.quiz_id=v.quiz_id AND v.quiz_id=%s AND v.version=%s AND NOT EXISTS "
                    "(SELECT 1 FROM reference_exclusions x WHERE x.attempt_id=ANY(v.members))", (quiz, version))
            if result.rowcount != 1:
                raise ValueError("No eligible reference version or head for this quiz")
