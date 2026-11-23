"""Score one attempt with the FROZEN pipeline of ml/anomaly (not modified).

Live platform events are written into the generator's CSV layout and passed to
``features.read_attempts`` as a file-like object, then ``build_features`` and
``train.score_model`` run with the artifact's reference and thresholds, and
``to_risk_event.risk_event`` builds the event (marginal-outlier explanation).
The wrapper sets a deterministic risk_id and adds question timing evidence to flagged events.

Input contract of the frozen pipeline v1 (checked per attempt; attempts that
violate it are recorded as not scorable instead of failing the cohort):
  * payload keys exactly as events.md v1 (the additive ``quiz.submitted.reason``
    is dropped before scoring);
  * every question answered and ``max_score`` equal to the number of
    questions (1 point per question);
  * answer timestamps consistent with ``duration_ms`` to the millisecond.
"""
from __future__ import annotations

import csv
import io
import json
import math
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

from ml.anomaly.features import ANSWER_KEYS, SUBMIT_KEYS, build_features, read_attempts
from ml.anomaly.to_risk_event import risk_event
from ml.anomaly.train import score_model

from .evidence import question_evidence
from .pipeline_v2 import prepare_v2

MODEL_KEY = "iforest"
MODEL_NAME = "isolation-forest-v1"  # as named by ml/anomaly/to_risk_event.py
RISK_NAMESPACE = uuid.UUID("6f1c2d3e-4b5a-5c6d-8e7f-90a1b2c3d4e5")
CSV_COLUMNS = ("event_id", "event_type", "version", "occurred_at", "producer", "actor", "context", "payload")
SCORED_TYPES = {"quiz.answered": ANSWER_KEYS, "quiz.submitted": SUBMIT_KEYS}


def deterministic_risk_id(artifact_version: str, attempt_id: str) -> str:
    """Same attempt + same model version -> same risk_id, so re-processing an
    attempt can never create a second signal (grading dedupes on risk_id)."""
    return str(uuid.uuid5(RISK_NAMESPACE, f"risk.behavior_anomaly:{MODEL_NAME}:{artifact_version}:{attempt_id}"))


def level_for(raw_score: float, model: dict[str, Any]) -> str:
    """Severity per the artifact thresholds (same rule as to_risk_event)."""
    if raw_score >= model["high_threshold"]:
        return "high"
    if raw_score >= model["medium_threshold"]:
        return "medium"
    return "low"


def _sort_key(env: dict[str, Any]) -> tuple[datetime, str]:
    return datetime.fromisoformat(env["occurred_at"]), env["event_id"]


def to_csv(envelopes: list[dict[str, Any]]) -> str:
    """Generator CSV layout: JSON object cells, chronological order, payloads
    restricted to the v1 key sets the frozen reader requires."""
    rows = []
    for env in sorted((e for e in envelopes if e.get("event_type") in SCORED_TYPES), key=_sort_key):
        keys = SCORED_TYPES[env["event_type"]]
        payload = {k: v for k, v in env["payload"].items() if k in keys}
        rows.append({
            "event_id": env["event_id"], "event_type": env["event_type"], "version": str(env["version"]),
            "occurred_at": env["occurred_at"], "producer": env["producer"],
            "actor": json.dumps(env["actor"], separators=(",", ":")),
            "context": json.dumps(env["context"], separators=(",", ":")),
            "payload": json.dumps(payload, separators=(",", ":")),
        })
    out = io.StringIO()
    writer = csv.DictWriter(out, fieldnames=CSV_COLUMNS)
    writer.writeheader()
    writer.writerows(rows)
    return out.getvalue()


def check_attempt(events: list[dict[str, Any]]) -> str | None:
    """Return None if the frozen reader accepts this attempt, else the reason."""
    try:
        read_attempts(io.StringIO(to_csv(events)))
    except (ValueError, TypeError, KeyError) as exc:
        return f"not scorable by frozen pipeline v1: {exc}"[:500]
    return None


@dataclass
class Outcome:
    status: str  # "flagged" | "scored" | "not_scorable"
    raw_score: float | None = None
    level: str | None = None
    event: dict[str, Any] | None = None
    reason: str = ""
    cohort_attempts: int = 0
    excluded: dict[str, str] = field(default_factory=dict)


class Scorer:
    """Wraps a verified artifact (see artifact.py)."""

    def __init__(self, artifact: dict[str, Any]) -> None:
        self.artifact = artifact
        self.model = artifact["models"][MODEL_KEY]
        self.version = str(artifact["version"])

    def score(self, target_attempt_id: str, events_by_attempt: dict[str, list[dict[str, Any]]],
              reference: dict[str, Any] | None = None) -> Outcome:
        """Score the target against its cohort (same group and quiz, including
        the student's earlier attempts for the score-delta history)."""
        target = events_by_attempt.get(target_attempt_id)
        if not target:
            return Outcome("not_scorable", reason="no events for the attempt")
        if self.artifact.get("schema_version", 1) == 2:
            attempts, excluded = prepare_v2(events_by_attempt)
            if target_attempt_id in excluded:
                return Outcome("not_scorable", reason=excluded[target_attempt_id])
        else:
            reason = check_attempt(target)
            if reason:
                return Outcome("not_scorable", reason=reason)
            valid: list[dict[str, Any]] = []
            excluded = {}
            for attempt_id, events in events_by_attempt.items():
                problem = None if attempt_id == target_attempt_id else check_attempt(events)
                if problem:
                    excluded[attempt_id] = problem
                else:
                    valid.extend(events)
            attempts = read_attempts(io.StringIO(to_csv(valid)))
        if reference is None:
            reference = self.artifact["reference"]
        frame = build_features(attempts, reference)
        scores = score_model(frame, self.model)
        rows = frame.to_dict("records")
        index = next(i for i, row in enumerate(rows) if row["attempt_id"] == target_attempt_id)
        raw = float(scores[index])
        if not math.isfinite(raw):
            return Outcome("not_scorable", reason="non-finite score", cohort_attempts=len(rows), excluded=excluded)
        level = level_for(raw, self.model)
        if level == "low":
            return Outcome("scored", raw, level, cohort_attempts=len(rows), excluded=excluded)
        event = risk_event(rows[index], raw, self.model, self.version)
        event["payload"]["evidence"]["features"]["per_question"] = question_evidence(
            attempts[index], target, reference
        )
        if self.artifact.get("schema_version", 1) == 2:
            event["payload"]["evidence"]["features"].update({
                key: attempts[index][key] for key in ("submission_reason", "answered_questions", "total_questions")
            })
        event["payload"]["risk_id"] = deterministic_risk_id(self.version, target_attempt_id)
        if event["payload"]["level"] != level:  # both derive from the same thresholds
            raise AssertionError("severity mapping diverged from the frozen pipeline")
        return Outcome("flagged", raw, level, event, cohort_attempts=len(rows), excluded=excluded)
