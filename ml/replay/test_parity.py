"""Small deterministic source-world cases for the replay comparator."""
from __future__ import annotations

import csv
import hashlib
import json
import tempfile
import unittest
import uuid
from datetime import UTC, datetime, timedelta
from pathlib import Path
from unittest.mock import patch

import numpy as np

from ml.replay.parity import (
    Differences,
    compare,
    map_world,
    projected_scores,
    sha256_of,
)


class ConstantModel:
    def score_samples(self, matrix: np.ndarray) -> np.ndarray:
        return np.full(matrix.shape[0], -0.5)


class ParityTests(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.world = Path(temporary.name) / "source.csv"
        self.at = datetime(2025, 10, 1, tzinfo=UTC)
        fields = ("event_id", "event_type", "version", "occurred_at", "producer", "actor", "context", "payload")
        answers = [hashlib.sha256(f'{{"option":{i}}}'.encode()).hexdigest() for i in range(4)]
        options = [f"choice-{i}" for i in range(4)]
        self.manifest = {"users": {"s": "mapped-user-private"}, "groups": {"g": "live-g"},
                         "quizzes": {"quiz": "live-quiz"}, "variants": {"v": "live-v"}, "shift_ms": 700,
                         "attempts": {"attempt": {"id": "mapped-attempt-private", "user_id": "mapped-user-private",
                                                  "group_id": "live-g", "quiz_id": "live-quiz", "variant_id": "live-v",
                                                  "status": "submitted"}},
                         "questions": {"q": {"id": "live-q", "option_ids": options,
                                             "answer_hashes": [hashlib.sha256(x.encode()).hexdigest() for x in options]}}}
        with self.world.open("w", encoding="utf-8", newline="") as stream:
            writer = csv.DictWriter(stream, fieldnames=fields)
            writer.writeheader()
            events = [("quiz.answered", 200, {"question_id": "q", "question_index": 0, "answer_hash": answers[0],
                                             "time_since_shown_ms": 200, "time_since_attempt_start_ms": 200, "is_change": False}),
                      ("quiz.submitted", 300, {"quiz_id": "quiz", "variant_id": "v", "max_score": 1, "score": 1,
                                              "duration_ms": 300, "answers_changed": 0})]
            for kind, ms, payload in events:
                timestamp = self.at + timedelta(milliseconds=ms)
                event_id = uuid.UUID(int=((int(self.at.timestamp()) * 1000 + ms) << 80) | (7 << 76) | (2 << 62))
                writer.writerow({"event_id": str(event_id), "event_type": kind, "version": 1, "producer": "quizzes",
                                 "occurred_at": timestamp.isoformat(), "actor": json.dumps({"user_id": "s"}),
                                 "context": json.dumps({"group_id": "g"}), "payload": json.dumps({"attempt_id": "attempt", **payload})})
        self.manifest["world_sha256"] = sha256_of(self.world)
        self.artifact = {"version": "1.0", "models": {"iforest": {
            "estimator": ConstantModel(), "kind": "iforest", "columns": ["duration_ms"], "medians": np.zeros(1),
            "medium_threshold": 0.6, "high_threshold": 0.8, "train_scores": np.array([0.2, 0.4, 0.5]),
        }}, "reference": {"global_fast_ms": 100, "item_fast_ms": {}, "answer_counts": {}, "delta_sd_floor": 5}}
        self.events = map_world(self.world, self.manifest)
        self.live = "mapped-attempt-private"
        self.events[self.live].insert(0, {"event_type": "quiz.started",
                                          "occurred_at": (self.at + timedelta(milliseconds=700)).isoformat()})
        self.row = {"status": "scored", "level": "low", "raw_score": 0.5, "scored_at": self.at,
                    "received_at": self.at, "published_at": None, "risk": None,
                    "submitted_at": self.at + timedelta(seconds=1)}

    def result(self) -> dict:
        return compare(self.world, self.manifest, self.artifact, {self.live: self.row}, self.events, 1e-9)

    def test_same_world_shift_and_private_output(self) -> None:
        report = self.result()
        self.assertTrue(report["passed"])
        self.assertTrue(report["native_score_parity"])
        self.assertEqual(sum(report["platform_parity"]["mismatches"].values()), 0)
        self.assertNotIn("mapped-user-private", json.dumps(report))
        self.assertNotIn("mapped-attempt-private", json.dumps(report))

    def test_independent_source_detects_input_and_persisted_score_divergence(self) -> None:
        self.events[self.live][1]["payload"]["time_since_shown_ms"] = 150
        self.row["raw_score"] = 0.51
        report = self.result()
        self.assertFalse(report["passed"])
        mismatch = report["platform_parity"]["mismatches"]
        self.assertEqual(mismatch["saved_event_inputs"], 1)
        self.assertEqual(mismatch["reconstructed.mean_answer_time_ms"], 1)
        self.assertEqual(mismatch["raw_score"], 1)
        self.assertAlmostEqual(report["platform_parity"]["max_absolute_difference"]["raw_score"], 0.01)

    def test_incomplete_world_and_invalid_hash_are_rejected(self) -> None:
        self.manifest["attempts"]["extra"] = {**self.manifest["attempts"]["attempt"], "id": "extra-id"}
        with self.assertRaisesRegex(ValueError, "sets differ"):
            map_world(self.world, self.manifest)
        self.manifest["world_sha256"] = "wrong"
        with self.assertRaisesRegex(ValueError, "checksum"):
            map_world(self.world, self.manifest)

    def test_signal_and_nullable_evidence(self) -> None:
        self.artifact["models"]["iforest"]["medium_threshold"] = 0.5
        expected = projected_scores(map_world(self.world, self.manifest), self.artifact)[self.live]
        self.row.update(status="flagged", level="medium", published_at=self.at,
                        risk={"target_id": self.live, "target_type": "attempt", "level": "medium",
                              "model": {"name": "isolation-forest-v1", "version": "1.0"}, "score": expected["score"],
                              "evidence": {"features": {"mean_answer_time_ms": 200, "z_score": None, "synchronous_with": []}}})
        self.assertTrue(self.result()["passed"])
        del self.row["risk"]["evidence"]["features"]["z_score"]
        self.assertEqual(self.result()["platform_parity"]["mismatches"]["signal.z_score.present"], 1)

    def test_nonfinite_numbers_and_tolerance(self) -> None:
        with self.assertRaises(ValueError):
            Differences(float("nan"))
        checks = Differences(0.01)
        checks.check(1, "close", 0.5, 0.505, numeric=True)
        checks.check(1, "invalid", float("inf"), 0.5, numeric=True)
        self.assertEqual(dict(checks.mismatched), {"invalid": 1})

    def test_native_service_numeric_tolerance_is_applied_directly(self) -> None:
        # Each adjacent difference is inside tolerance, but the direct native-to-service gap is twice as wide.
        projected = projected_scores(map_world(self.world, self.manifest), self.artifact)
        projected[self.live]["raw_score"] += .75e-9
        self.row["raw_score"] += 1.5e-9
        with patch("ml.replay.parity.projected_scores", return_value=projected):
            result = self.result()
        self.assertTrue(result["platform_passed"])
        self.assertNotIn("raw_score", result["native_to_platform_namespace"]["mismatches"])
        self.assertFalse(result["native_score_parity"])
        self.assertFalse(result["passed"])
        self.assertEqual(result["native_vs_service"]["mismatches"], {"raw_score": 1})

    def test_positive_peers_agreement_and_signal_cdf(self) -> None:
        # Two different students answer the same question identically and submit one second apart.
        with self.world.open(encoding="utf-8", newline="") as stream:
            reader = csv.DictReader(stream)
            rows, fields = list(reader), reader.fieldnames
        with self.world.open("a", encoding="utf-8", newline="") as stream:
            writer = csv.DictWriter(stream, fieldnames=fields)
            for row in rows:
                row["actor"] = json.dumps({"user_id": "second"})
                row["occurred_at"] = (datetime.fromisoformat(row["occurred_at"]) + timedelta(seconds=1)).isoformat()
                row["event_id"] = str(uuid.UUID(int=uuid.UUID(row["event_id"]).int + (1000 << 80)))
                row["payload"] = json.dumps({**json.loads(row["payload"]), "attempt_id": "second-attempt"})
                writer.writerow(row)
        self.manifest["world_sha256"] = sha256_of(self.world)
        self.manifest["users"]["second"] = "second-user"
        peer = "second-platform-attempt"
        self.manifest["attempts"]["second-attempt"] = {**self.manifest["attempts"]["attempt"], "user_id": "second-user", "id": peer}
        self.artifact["models"]["iforest"]["medium_threshold"] = .5
        expected_events = map_world(self.world, self.manifest)
        projected = projected_scores(expected_events, self.artifact)
        self.assertEqual(projected[self.live]["features"]["synchronous_count"], 1)
        self.assertEqual(projected[self.live]["features"]["max_answer_agreement"], 1)
        self.assertGreater(projected[self.live]["features"]["rarity_agreement"], 0)
        self.assertEqual(projected[self.live]["peers"], [peer])
        states = {}
        for live, expected in projected.items():
            sub = expected["submitted_at"]
            start = sub - timedelta(milliseconds=expected["features"]["duration_ms"])
            expected_events[live].append({"event_type": "quiz.started", "occurred_at": start.isoformat()})
            states[live] = {**self.row, "status": "flagged", "level": "medium", "submitted_at": sub,
                            "published_at": self.at, "risk": {"target_type": "attempt", "target_id": live,
                            "model": {"name": "isolation-forest-v1", "version": "1.0"}, "level": "medium", "score": 1.0,
                            "evidence": {"features": {"mean_answer_time_ms": 200, "z_score": None,
                                                      "synchronous_with": expected["peers"]}}}}
        def compare_peers() -> dict:
            return compare(self.world, self.manifest, self.artifact, states, expected_events, 1e-9)
        self.assertTrue(compare_peers()["passed"])
        states[self.live]["risk"]["score"] = .5
        states[self.live]["risk"]["evidence"]["features"]["synchronous_with"] = []
        expected_events[peer][0]["payload"]["answer_hash"] = self.manifest["questions"]["q"]["answer_hashes"][1]
        errors = compare_peers()["platform_parity"]["mismatches"]
        self.assertEqual(errors["signal.score"], 1)
        self.assertEqual(errors["signal.synchronous_with"], 1)
        self.assertEqual(errors["reconstructed.max_answer_agreement"], 2)
        self.assertEqual(errors["reconstructed.rarity_agreement"], 2)
