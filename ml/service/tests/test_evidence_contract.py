"""Compatibility of the optional FR-42 evidence with the v1 risk contract."""
import unittest

from ml.anomaly.to_risk_event import envelope
from ml.service.schema import EventValidator, find_dir


class EvidenceContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.validator = EventValidator(find_dir())

    def setUp(self) -> None:
        self.features = {"mean_answer_time_ms": 500, "z_score": None, "synchronous_with": []}
        self.event = envelope(
            "risk.behavior_anomaly",
            {"risk_id": "risk_fixture", "target_type": "attempt", "target_id": "attempt_fixture",
             "score": 0.96, "level": "medium", "model": {"name": "isolation-forest-v1", "version": "0.1.0"},
             "explanation": "Synthetic timing observation", "evidence": {"features": self.features}},
            {"group_id": "group_fixture", "course_id": "course_fixture", "session_id": None},
        )
        self.row = {"question_index": 0, "first_answer_time_ms": 500, "reference_median_ms": 2400.5,
                    "reference_scope": "item_train", "fast_threshold_ms": 750.5,
                    "is_fast": True, "answer_changed": False}

    def test_v1_with_and_without_per_question(self) -> None:
        self.assertEqual(self.validator.errors(self.event), [])
        for scope, value in (("item_train", 2400.5), ("global_train", 4000), ("unavailable", None)):
            with self.subTest(scope=scope):
                self.features["per_question"] = [dict(self.row, reference_scope=scope, reference_median_ms=value)]
                self.assertEqual(self.validator.errors(self.event), [])

    def test_rejects_invalid_question_evidence(self) -> None:
        for change in ({"question_index": 200}, {"first_answer_time_ms": -1}, {"is_fast": 1},
                       {"answer_changed": "true"}, {"reference_median_ms": None},
                       {"reference_scope": "unavailable"}, {"answer_hash": "fixture"}):
            with self.subTest(change=change):
                self.features["per_question"] = [self.row | change]
                self.assertTrue(self.validator.errors(self.event))
        self.features["per_question"] = [self.row] * 201
        self.assertTrue(self.validator.errors(self.event))
