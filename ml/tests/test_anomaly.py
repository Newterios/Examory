# ml/tests/test_anomaly.py
"""Regression checks for the behavioral anomaly baseline."""
from __future__ import annotations

import csv
import json
import subprocess
import sys
import tempfile
import unittest
import uuid
from pathlib import Path

import numpy as np

from ml.anomaly.evaluate import evaluate as evaluate_behavior
from ml.anomaly.features import (
    FEATURE_COLUMNS,
    build_features,
    fit_reference,
    read_attempts,
)
from ml.anomaly.split import create_manifest
from ml.anomaly.to_risk_event import risk_event
from ml.anomaly.train import choose_threshold, train


class AnomalyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.temporary = tempfile.TemporaryDirectory(prefix="examory-regression-")
        cls.root = Path(cls.temporary.name)
        cls.csv = cls.root / "quiz.csv"
        subprocess.run([sys.executable, "ml/datasets/generate_quiz_behavior.py", "--students", "120",
                        "--seed", "42", "--output", str(cls.csv)], check=True, capture_output=True)
        cls.attempts = read_attempts(cls.csv)


    @classmethod
    def tearDownClass(cls) -> None:
        cls.temporary.cleanup()


    def test_dedup_and_conflicts(self) -> None:
        with self.csv.open(encoding="utf-8", newline="") as stream:
            reader = csv.DictReader(stream)
            rows, headers = list(reader), reader.fieldnames
        duplicate = self.root / "duplicate.csv"
        with duplicate.open("w", encoding="utf-8", newline="") as stream:
            writer = csv.DictWriter(stream, headers)
            writer.writeheader()
            writer.writerows([rows[0], *rows])
        self.assertEqual(len(read_attempts(duplicate)), len(self.attempts))
        conflict = dict(rows[0], producer="conflicting-producer")
        with duplicate.open("a", encoding="utf-8", newline="") as stream:
            csv.DictWriter(stream, headers).writerow(conflict)
        with self.assertRaisesRegex(ValueError, "Conflicting"):
            read_attempts(duplicate)


    def test_history_and_peer_isolation(self) -> None:
        reference = fit_reference([a for a in self.attempts if a["is_anomaly"] == 0])
        frame = build_features(self.attempts, reference)
        self.assertEqual(int(frame.history_missing.sum()), 120)
        self.assertTrue(frame.loc[frame.history_missing == 1, "z_score"].isna().all())
        self.assertTrue({"is_anomaly", "anomaly_type", "attempt_id"}.isdisjoint(FEATURE_COLUMNS))
        by_id = {a["attempt_id"]: a for a in self.attempts}
        for row in frame.to_dict("records"):
            for peer in row["synchronous_with"]:
                other = by_id[peer]
                self.assertEqual(row["group_id"], other["group_id"])
                self.assertLessEqual(abs((row["submitted_at"] - other["submitted_at"]).total_seconds()), 5)
        for user in frame.user_id.unique():
            records = [a for a in self.attempts if a["user_id"] == user]
            self.assertAlmostEqual(records[1]["score_delta"], records[1]["score_percent"] - records[0]["score_percent"])


    def test_ties_respect_fpr(self) -> None:
        labels = np.array([0] * 100 + [1] * 10)
        scores = np.array([.3] * 94 + [.8] * 6 + [.9] * 10)
        result = choose_threshold(labels, scores, .05)
        self.assertEqual(result["validation_fpr"], 0)
        self.assertEqual(result["validation_recall"], 1)
        self.assertEqual(result["threshold"], .9)


    def test_behavior_end_to_end(self) -> None:
        manifest = create_manifest(self.csv, 42)
        partitions = list(manifest["groups"].values())
        self.assertFalse(set(partitions[0]) & set(partitions[1]))
        self.assertFalse(set(partitions[0]) & set(partitions[2]))
        path = self.root / "split.json"
        path.write_text(json.dumps(manifest), encoding="utf-8")
        artifact = train(self.csv, path, self.root / "behavior")
        model = artifact["models"]["iforest"]
        self.assertEqual(model["estimator"].n_estimators, 200)
        self.assertEqual(model["estimator"].contamination, "auto")
        self.assertLessEqual(model["operating_points"]["0.05"]["validation_fpr"], .05)
        evaluation = evaluate_behavior(self.csv, self.root / "behavior/model.pkl", self.root / "behavior.md", 10, 42)
        self.assertEqual(evaluation["rows"][0]["n"], manifest["counts"]["test"]["attempts"])
        row = build_features(self.attempts, artifact["reference"]).iloc[0].to_dict()
        event = risk_event(row, .9, model, artifact["version"])
        self.assertEqual(uuid.UUID(event["event_id"]).version, 7)
        self.assertIsNone(event["payload"]["evidence"]["features"]["z_score"])
        self.assertTrue(0 <= event["payload"]["score"] <= 1)
        json.dumps(event, allow_nan=False)


if __name__ == "__main__":
    unittest.main()
