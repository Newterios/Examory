"""Core of the anomaly scorer, independent of Kafka (tested with fakes)."""
from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Any, Protocol

from .quiz_reference import QuizReferences
from .schema import EventValidator
from .scoring import Outcome, Scorer
from .store import OutboxRow, Store
from .watermark import is_ready

RISK_TOPIC = "ai.risk"
QUIZ_TYPES = {"quiz.started", "quiz.answered", "quiz.submitted"}
MAX_EVENT_BYTES = 64 * 1024

log = logging.getLogger("anomaly.processor")


class Publisher(Protocol):
    def publish(self, rows: list[OutboxRow]) -> list[str]:
        """Publish rows; return the risk_ids the broker acknowledged."""


@dataclass
class Stats:
    stored: int = 0
    duplicates: int = 0
    ignored: int = 0
    rejected: int = 0
    flagged: int = 0
    scored: int = 0
    not_scorable: int = 0


def parse_time(value: str) -> datetime:
    t = datetime.fromisoformat(value)
    if t.tzinfo is None:
        raise ValueError("occurred_at without timezone")
    return t


class Processor:
    def __init__(self, store: Store, scorer: Scorer, validator: EventValidator, lateness: timedelta,
                 references: QuizReferences | None = None) -> None:
        self.store, self.scorer, self.validator, self.lateness = store, scorer, validator, lateness
        self.stats = Stats()
        self.references = references

    def handle(self, raw: bytes) -> str:
        """Ingest one Kafka record value. Never raises for bad input: invalid
        records are logged (dead-letter log) and acknowledged."""
        if len(raw) > MAX_EVENT_BYTES:
            return self._reject("record too large", None)
        try:
            env = json.loads(raw)
        except (ValueError, UnicodeDecodeError):
            return self._reject("not JSON", None)
        if not isinstance(env, dict):
            return self._reject("not a JSON object", None)
        if env.get("event_type") not in QUIZ_TYPES:
            self.stats.ignored += 1
            return "ignored"
        errors = self.validator.errors(env)
        if errors:
            return self._reject("schema: " + "; ".join(errors[:3]), env)
        try:
            occurred = parse_time(env["occurred_at"])
        except ValueError:
            return self._reject("bad occurred_at", env)
        if self.store.add_event(env, occurred):
            self.stats.stored += 1
            return "stored"
        self.stats.duplicates += 1
        return "duplicate"

    def _reject(self, reason: str, env: dict[str, Any] | None) -> str:
        self.stats.rejected += 1
        log.warning("event rejected", extra={"reason": reason[:300],
                                             "event_id": (env or {}).get("event_id"),
                                             "event_type": (env or {}).get("event_type")})
        return "rejected"

    def score_ready(self, limit: int = 50) -> int:
        """Score every pending attempt whose watermark condition holds."""
        watermark, now = self.store.watermark(), self.store.now()
        done = 0
        for p in self.store.pending(limit):
            readiness = is_ready(p.submitted_at, watermark, p.received_at, now, self.lateness)
            if not readiness.ready:
                continue
            live = self.references.get(p.quiz_id, p.submitted_at) if self.references is not None else None
            try:
                cohort = self.store.cohort_events(p.group_id, p.quiz_id)
                if live is None:
                    outcome = self.scorer.score(p.attempt_id, cohort)
                    if self.references is not None and outcome.status in {"scored", "flagged"}:
                        outcome.status, outcome.event, outcome.reason = "scored", None, "cold_start_no_publish"
                else:
                    version, reference = live
                    outcome = self.scorer.score(p.attempt_id, cohort, reference)
                    if outcome.status in {"scored", "flagged"}:
                        outcome.reason = "quiz_reference:" + version
                    if outcome.event is not None:
                        outcome.event["payload"]["evidence"]["features"]["reference_version"] = version
            except Exception as exc:
                log.exception("scoring failed", extra={"attempt_id": p.attempt_id})
                outcome = Outcome("not_scorable", reason=f"scoring error: {type(exc).__name__}")
            self._finish(p.attempt_id, outcome, readiness.reason)
            done += 1
        return done

    def _finish(self, attempt_id: str, outcome: Outcome, why: str) -> None:
        outbox, event_id = None, None
        if outcome.event is not None:
            errors = self.validator.errors(outcome.event)
            if errors:  # never publish an event that violates the contract
                outcome = Outcome("not_scorable", outcome.raw_score, outcome.level,
                                  reason="produced event invalid: " + "; ".join(errors[:3]))
            else:
                payload = outcome.event["payload"]
                event_id = outcome.event["event_id"]
                outbox = OutboxRow(payload["risk_id"], RISK_TOPIC, payload["target_id"],
                                   json.dumps(outcome.event, ensure_ascii=False, allow_nan=False, separators=(",", ":")))
        detail = outcome.reason or f"ready by {why}; cohort={outcome.cohort_attempts}; excluded={len(outcome.excluded)}"
        if self.store.finish(attempt_id, outcome.status, outcome.raw_score, outcome.level, detail, outbox, event_id):
            setattr(self.stats, outcome.status, getattr(self.stats, outcome.status) + 1)
            log.info("attempt assessed", extra={"attempt_id": attempt_id, "status": outcome.status,
                                                "level": outcome.level, "ready_by": why})

    def publish(self, publisher: Publisher, limit: int = 100) -> int:
        rows = self.store.unpublished(limit)
        if not rows:
            return 0
        acked = publisher.publish(rows)
        self.store.mark_published(acked)
        return len(acked)
