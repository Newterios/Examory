"""Sparse grades, historical references, review validation and snapshot rollback."""
from __future__ import annotations

import importlib.metadata
import pickle
import tempfile
import unittest
import uuid
from copy import deepcopy
from datetime import timedelta
from pathlib import Path

import numpy as np
from sklearn.ensemble import IsolationForest

from ml.service.artifact import (
    PINNED_LIBRARIES,
    ArtifactError,
    load_verified,
    sha256_of,
)
from ml.service.pipeline_v2 import POLICY, attempt_v2, history_v2, prepare_v2
from ml.service.schema import EventValidator, find_dir
from ml.service.scoring import Scorer
from ml.service.tests.test_service import T0, live_attempt
from ml.service.upgrade_v2 import upgrade


def model() -> dict:
    values = np.column_stack((np.arange(12), np.zeros(12)))
    est = IsolationForest(n_estimators=8, random_state=42).fit(values)
    return {"schema_version": 2, "input_policy": POLICY, "version": "0.1.0-v2",
            "reference": {"global_fast_ms": 100, "item_fast_ms": {}, "answer_counts": {}, "delta_sd_floor": 5},
            "models": {"iforest": {"kind": "iforest", "columns": ["score_percent"], "medians": [0],
                                   "estimator": est, "train_scores": -est.score_samples(values), "bounds": {},
                                   "medium_threshold": 0.0, "high_threshold": 0.0}}}


class LivePipelineTests(unittest.TestCase):
    def setUp(self):
        self.validator = EventValidator(find_dir())

    def test_trusted_upgrade_and_loader(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            old, new = root / "v1.pkl", root / "v2.pkl"
            base = model() | {"schema_version": 1, "version": "0.1.0",
                              "environment": {"libraries": {k: importlib.metadata.version(k) for k in PINNED_LIBRARIES}}}
            base.pop("input_policy")
            old.write_bytes(pickle.dumps(base))
            old.chmod(0o444)
            digest = sha256_of(old)
            with self.assertRaises(ArtifactError):
                upgrade(old, "0"*64, new)
            converted = upgrade(old, digest, new)
            new.chmod(0o444)
            self.assertEqual(load_verified(new, converted)["parent_sha256"], digest)
            self.assertEqual(load_verified(old, digest)["schema_version"], 1)
            with self.assertRaises(FileExistsError):
                upgrade(old, digest, new)
            old.chmod(0o600)
            new.chmod(0o600)

    def test_partial_weighted_timeout(self):
        events = live_attempt("u", "g", "quiz", "v", T0, [1500, 2500], ["x", "y"])
        events[1]["payload"]["question_index"] = 4
        events[-1]["payload"].update(score=4, max_score=10, reason="timed_out")
        started = deepcopy(events[0])
        started.update(event_type="quiz.started", event_id=str(uuid.uuid4()))
        started["payload"] = {"attempt_id": events[-1]["payload"]["attempt_id"], "quiz_id": "quiz", "variant_id": "v",
                                  "question_count": 7, "time_limit_sec": 3600, "exam_session_id": None}
        events.insert(0, started)
        aid = events[-1]["payload"]["attempt_id"]
        source = deepcopy(events)
        a = attempt_v2(events)
        self.assertEqual((a["score_percent"], a["answered_questions"], a["total_questions"]), (40, 2, 7))
        self.assertEqual(prepare_v2({aid: [events[-1]]})[0], [])
        self.assertEqual(events, source)
        outcome = Scorer(model()).score(aid, {aid: events})
        self.assertEqual(outcome.status, "flagged")
        self.assertEqual(self.validator.errors(outcome.event), [])
        f = outcome.event["payload"]["evidence"]["features"]
        self.assertEqual(f["submission_reason"], "timed_out")
        self.assertEqual([e["question_index"] for e in f["per_question"]], [0, 4])
        old = model() | {"schema_version": 1}
        self.assertEqual(Scorer(old).score(aid, {aid: events}).status, "not_scorable")

    def test_actual_score_history_and_equal_time(self):
        first = live_attempt("u", "g", "quiz", "v", T0, [100], ["A"])
        second = live_attempt("u", "g", "quiz", "v", T0 + timedelta(days=1), [100], ["A"])
        first[-1]["payload"].update(score=3, max_score=4)
        second[-1]["payload"].update(score=4, max_score=4)
        rows = history_v2([attempt_v2(second), attempt_v2(first), attempt_v2(first)])
        self.assertEqual([a["history_missing"] for a in rows], [1, 1, 0])
        self.assertEqual(rows[-1]["score_delta"], 25)
