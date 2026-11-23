"""Core of the anomaly scorer, independent of Kafka (tested with fakes)."""
from __future__ import annotations

import json
import logging
import time
from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import Never, Protocol

from .metrics import Metrics
from .schema import EventValidator
from .scoring import Outcome, Scorer
from .store import OutboxRow, Store
from .watermark import is_ready

RISK_TOPIC = "ai.risk"
QUIZ_TYPES = {"quiz.started", "quiz.answered", "quiz.submitted"}
MAX_EVENT_BYTES = 64 * 1024

log = logging.getLogger("anomaly.processor")


class Publisher(Protocol):
    def publish(self, rows: list[OutboxRow]) -> dict[str, datetime]:
        """Return the UTC delivery-report timestamp for each acknowledged risk."""


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


def _invalid_number(_: str) -> Never:
    raise ValueError("non-finite JSON number")


class Processor:
    def __init__(self, store: Store, scorer: Scorer, validator: EventValidator, lateness: timedelta,
                 metrics: Metrics | None = None) -> None:
        self.store, self.scorer, self.validator, self.lateness = store, scorer, validator, lateness
        self.stats = Stats()
        self.metrics = metrics if metrics is not None else Metrics()

    def handle(self, raw: bytes) -> str:
        """Ingest one Kafka record value. Never raises for bad input: invalid
        records are logged (dead-letter log) and acknowledged."""
        if len(raw) > MAX_EVENT_BYTES:
            return self._reject("record too large")
        try:
            env = json.loads(raw, parse_constant=_invalid_number)
        except (ValueError, RecursionError):
            return self._reject("invalid JSON")
        if not isinstance(env, dict):
            return self._reject("not a JSON object")
        event_type = env.get("event_type")
        if not isinstance(event_type, str):
            return self._reject("invalid event type")
        if event_type not in QUIZ_TYPES:
            return self._count("ignored")
        errors = self.validator.errors(env)
        if errors:
            return self._reject("schema mismatch")
        try:
            occurred = parse_time(env["occurred_at"])
        except (KeyError, ValueError, TypeError, OverflowError):
            return self._reject("invalid timestamp")
        if self.store.add_event(env, occurred):
            return self._count("stored")
        return self._count("duplicate")

    def _count(self, result: str) -> str:
        field = "duplicates" if result == "duplicate" else result
        setattr(self.stats, field, getattr(self.stats, field) + 1)
        self.metrics.record_event(result)
        return result

    def _reject(self, reason: str) -> str:
        log.warning("event rejected", extra={"reason": reason})
        return self._count("rejected")

    def score_ready(self, limit: int = 50) -> int:
        """Score every pending attempt whose watermark condition holds."""
        watermark, now = self.store.watermark(), self.store.now()
        done = 0
        for p in self.store.pending(limit):
            readiness = is_ready(p.submitted_at, watermark, p.received_at, now, self.lateness)
            if not readiness.ready:
                continue
            started = time.monotonic()
            try:
                outcome = self.scorer.score(p.attempt_id, self.store.cohort_events(p.group_id, p.quiz_id))
            except Exception:  # noqa: BLE001 - record one bounded status without exposing exception arguments
                log.error("scoring failed")
                outcome = Outcome("not_scorable", reason="error:scoring")
            finally:
                self.metrics.observe_scoring(time.monotonic() - started)
            self._finish(p.attempt_id, outcome, readiness.reason)
            done += 1
        return done

    def _finish(self, attempt_id: str, outcome: Outcome, why: str) -> None:
        outbox, event_id = None, None
        if outcome.event is not None:
            try:
                if self.validator.errors(outcome.event):
                    raise ValueError("risk event schema mismatch")
                encoded = json.dumps(outcome.event, ensure_ascii=False, allow_nan=False, separators=(",", ":"))
                payload = outcome.event["payload"]
                event_id = outcome.event["event_id"]
                outbox = OutboxRow(payload["risk_id"], RISK_TOPIC, payload["target_id"], encoded)
            except (ValueError, KeyError, TypeError, RecursionError):
                outcome = Outcome("not_scorable", outcome.raw_score, outcome.level, reason="error:invalid_output")
        detail = outcome.reason or f"ready by {why}; cohort={outcome.cohort_attempts}; excluded={len(outcome.excluded)}"
        if self.store.finish(attempt_id, outcome.status, outcome.raw_score, outcome.level, detail, outbox, event_id):
            setattr(self.stats, outcome.status, getattr(self.stats, outcome.status) + 1)
            log.info("attempt assessed", extra={"status": outcome.status, "level": outcome.level, "ready_by": why})

    def publish(self, publisher: Publisher, limit: int = 100) -> int:
        rows = self.store.unpublished(limit)
        if not rows:
            return 0
        acked = publisher.publish(rows)
        delays = self.store.mark_published(acked)
        for seconds in delays:
            self.metrics.observe_publication(seconds)
        return len(delays)
