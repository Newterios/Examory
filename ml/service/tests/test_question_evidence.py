"""Run the service and frozen Isolation Forest on the same validated cohort."""
from __future__ import annotations

import hashlib
import io
import unittest
import uuid
from datetime import timedelta
from typing import Any
from unittest.mock import patch

import numpy as np
from sklearn.ensemble import IsolationForest

from ml.anomaly.features import build_features, item_key, read_attempts
from ml.anomaly.to_risk_event import risk_event
from ml.anomaly.train import score_model
from ml.service.scoring import Scorer, deterministic_risk_id, to_csv
from ml.service.tests.test_service import T0, live_attempt


def revise_at_same_time(events: list[dict[str, Any]], index: int) -> None:
    original = events[index]
    first_id = uuid.UUID(original["event_id"]).int & ~((1 << 62) - 1)
    original["event_id"] = str(uuid.UUID(int=first_id))
    revised = {**original, "event_id": str(uuid.UUID(int=first_id + 1)), "payload": {
        **original["payload"], "is_change": True, "answer_hash": hashlib.sha256(b"revised").hexdigest(),
    }}
    events.insert(index + 1, revised)
    events[-1]["payload"]["answers_changed"] += 1


class QuestionEvidenceTests(unittest.TestCase):
    def setUp(self) -> None:
        target = live_attempt("student-a", "group", "quiz", "v", T0, [100, 79, 90], ["a", "b", "c"])
        # The order of viewing differs from the order in the quiz.
        for event, index in zip(target[:3], (2, 0, 1), strict=True):
            event["payload"]["question_index"] = index
        revise_at_same_time(target, 0)
        peer = live_attempt("student-b", "group", "quiz", "v", T0 + timedelta(seconds=1), [100, 79, 90], ["a", "b", "c"])
        revise_at_same_time(peer, 1)
        self.target = target[0]["payload"]["attempt_id"]
        self.cohort = {peer[0]["payload"]["attempt_id"]: peer, self.target: list(reversed(target))}
        attempts = read_attempts(io.StringIO(to_csv(target + peer)))
        self.attempts = attempts
        self.item = item_key(next(a for a in attempts if a["attempt_id"] == self.target), "quiz-q0")
        self.reference = {"global_fast_ms": 80.0, "item_fast_ms": {self.item: 100.0}, "answer_counts": {}, "delta_sd_floor": 5.0}
        # A real fitted estimator; threshold choices let these small fixtures exercise both publication paths.
        train = np.column_stack((np.linspace(0.0, 1.0, 20), np.zeros(20)))
        estimator = IsolationForest(n_estimators=8, random_state=42).fit(train)
        self.model = {"kind": "iforest", "columns": ["fast_answer_fraction"], "medians": np.zeros(1), "bounds": {},
                      "estimator": estimator, "train_scores": -estimator.score_samples(train),
                      "medium_threshold": 0.0, "high_threshold": 0.0}
        self.artifact = {"version": "1.0", "reference": self.reference, "models": {"iforest": self.model}}

    def test_flagged_rows_match_train_thresholds_and_original_risk(self) -> None:
        self.reference.update(item_median_ms={self.item: 190.0}, global_median_ms=150.0)
        frame = build_features(self.attempts, self.reference)
        index = frame.index[frame.attempt_id == self.target][0]
        raw = float(score_model(frame, self.model)[index])
        frozen = risk_event(frame.iloc[index].to_dict(), raw, self.model, self.artifact["version"])["payload"]
        outcome = Scorer(self.artifact).score(self.target, self.cohort)
        self.assertEqual((outcome.status, outcome.raw_score, outcome.level), ("flagged", raw, frozen["level"]))
        enriched = outcome.event["payload"]
        evidence = enriched["evidence"]["features"].pop("per_question")
        self.assertEqual(enriched["risk_id"], deterministic_risk_id(self.artifact["version"], self.target))
        self.assertEqual({k: v for k, v in enriched.items() if k != "risk_id"},
                         {k: v for k, v in frozen.items() if k != "risk_id"})
        self.assertEqual(evidence, [
            {"question_index": 0, "first_answer_time_ms": 79, "fast_threshold_ms": 80.0, "reference_median_ms": 150.0,
             "reference_scope": "global_train", "is_fast": True, "answer_changed": False},
            {"question_index": 1, "first_answer_time_ms": 90, "fast_threshold_ms": 80.0, "reference_median_ms": 150.0,
             "reference_scope": "global_train", "is_fast": False, "answer_changed": False},
            {"question_index": 2, "first_answer_time_ms": 100, "fast_threshold_ms": 100.0, "reference_median_ms": 190.0,
             "reference_scope": "item_train", "is_fast": False, "answer_changed": True},
        ])
        self.assertEqual(sum(q["is_fast"] for q in evidence) / len(evidence), frame.at[index, "fast_answer_fraction"])

    def test_legacy_reference_still_has_p10_and_stable_scores(self) -> None:
        legacy = Scorer(self.artifact).score(self.target, self.cohort)
        rows = legacy.event["payload"]["evidence"]["features"]["per_question"]
        self.assertTrue(all(r["reference_scope"] == "unavailable" and r["reference_median_ms"] is None for r in rows))
        self.assertEqual([r["fast_threshold_ms"] for r in rows], [80., 80., 100.])
        self.reference.update(item_median_ms={}, global_median_ms=0.0)
        current = Scorer(self.artifact).score(self.target, self.cohort)
        self.assertEqual((legacy.raw_score, legacy.level, legacy.status), (current.raw_score, current.level, current.status))
        self.assertTrue(all(r["reference_scope"] == "global_train" and r["reference_median_ms"] == 0.0
                            for r in current.event["payload"]["evidence"]["features"]["per_question"]))

    def test_only_flagged_attempts_build_question_evidence(self) -> None:
        self.model.update(medium_threshold=2.0, high_threshold=3.0)
        with patch("ml.service.scoring.question_evidence", side_effect=AssertionError("unexpected enrichment")):
            self.assertEqual(Scorer(self.artifact).score(self.target, self.cohort).status, "scored")
            self.assertEqual(Scorer(self.artifact).score("missing", self.cohort).status, "not_scorable")
