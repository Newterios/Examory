"""Check the provenance of the training-time question medians and model publication."""
from __future__ import annotations

import csv
import json
import tempfile
import unittest
import uuid
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import patch

import numpy as np

from ml.anomaly.features import build_features, file_sha256, item_key, read_attempts
from ml.anomaly.train import load_artifact, score_model
from ml.service import train


def write_fixture(directory: Path) -> tuple[Path, Path]:
    source = directory / "events.csv"
    groups = {"train": ["train"], "validation": ["validation"], "test": ["test"]}
    # One observation per question per normal attempt; all revisions take another 5s.
    cases = [("train", "common", i, 0) for i in range(1, 31)]
    cases += [
        ("train", "sparse", 900, 0), ("train", "common", 8000, 1),
        ("validation", "common", 7000, 0), ("validation", "common", 9000, 1),
        ("test", "unseen", 7000, 0), ("test", "common", 9000, 1),
    ]
    with source.open("w", newline="", encoding="utf-8") as stream:
        writer = csv.writer(stream)
        writer.writerow(("event_id", "event_type", "version", "occurred_at", "producer",
                         "actor", "context", "payload", "is_anomaly", "anomaly_type"))
        for i, (group, question, first_ms, label) in enumerate(cases):
            start_ms = 1_700_000_000_000 + 60_000 * i
            common = {"attempt_id": str(i), "question_id": question, "question_index": 0}
            initial = dict(common, time_since_shown_ms=first_ms, time_since_attempt_start_ms=first_ms,
                           is_change=False, answer_hash="0" * 64)
            changed = dict(common, time_since_shown_ms=first_ms + 5000, time_since_attempt_start_ms=first_ms + 5000,
                           is_change=True, answer_hash="1" * 64)
            submit = {"attempt_id": str(i), "quiz_id": "quiz", "variant_id": "variant", "max_score": 1,
                      "score": i % 2, "duration_ms": first_ms + 6000, "answers_changed": 1}
            for elapsed, kind, data in ((first_ms, "quiz.answered", initial),
                                        (first_ms + 5000, "quiz.answered", changed),
                                        (first_ms + 6000, "quiz.submitted", submit)):
                ms = start_ms + elapsed
                event_id = uuid.UUID(int=(ms << 80) | (7 << 76) | (2 << 62))
                timestamp = datetime.fromtimestamp(ms / 1000, UTC).isoformat(timespec="milliseconds")
                writer.writerow((event_id, kind, 1, timestamp, "quizzes", json.dumps({"user_id": str(i)}),
                                 json.dumps({"group_id": group}), json.dumps(data), label,
                                 "synthetic" if label else "none"))
    manifest = directory / "split.json"
    manifest.write_text(json.dumps({"csv_sha256": file_sha256(source), "groups": groups}), encoding="utf-8")
    return source, manifest


class EvidenceReferenceTests(unittest.TestCase):
    def test_training_population_and_scores(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, manifest = write_fixture(root)
            output = root / "output"
            original = root / "frozen.pkl"
            # Capture the single training pass before the new metadata is packaged.
            frozen = train.train_frozen

            def capture(source: Path, split: Path, stage: Path) -> dict:
                artifact = frozen(source, split, stage)
                original.write_bytes((stage / "model.pkl").read_bytes())
                return artifact

            with patch.object(train, "train_frozen", side_effect=capture) as trainer:
                train.train(source, manifest, output)
            trainer.assert_called_once()
            base, packaged = load_artifact(original), load_artifact(output / "model.pkl")
            reference = packaged["reference"]
            attempts = read_attempts(source)
            common_key = item_key(attempts[0], "common")
            self.assertEqual(reference["item_median_ms"], {common_key: 15.5})
            self.assertEqual(reference["global_median_ms"], 16.0)
            self.assertEqual(reference.keys() - base["reference"].keys(), {"item_median_ms", "global_median_ms"})
            self.assertEqual({key: reference[key] for key in base["reference"]}, base["reference"])
            for name in base["models"]:
                before = score_model(build_features(attempts, base["reference"]), base["models"][name])
                after = score_model(build_features(attempts, reference), packaged["models"][name])
                np.testing.assert_array_equal(before, after)

    def test_failed_build_preserves_existing_model_and_thresholds(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, manifest = write_fixture(root)
            (root / "model.pkl").write_bytes(b"prior model")
            (root / "thresholds.json").write_bytes(b"prior thresholds")
            replace = Path.replace

            def fail_model(path: Path, destination: Path) -> Path:
                if destination == root / "model.pkl":
                    raise OSError("could not replace final model")
                return replace(path, destination)

            with patch.object(Path, "replace", fail_model), self.assertRaisesRegex(OSError, "final model"):
                train.train(source, manifest, root)
            self.assertEqual((root / "model.pkl").read_bytes(), b"prior model")
            self.assertEqual((root / "thresholds.json").read_bytes(), b"prior thresholds")
            self.assertEqual(list(root.glob(".evidence-train-*")), [])

            manifest.write_text(json.dumps({"csv_sha256": "wrong"}), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "differs from manifest"):
                train.train(source, manifest, root)
            self.assertEqual((root / "model.pkl").read_bytes(), b"prior model")
