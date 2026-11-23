"""Sparse grades, historical references, review validation and snapshot rollback."""
from __future__ import annotations

import importlib.metadata
import json
import os
import pickle
import subprocess
import sys
import tempfile
import time
import unittest
import uuid
from copy import deepcopy
from datetime import timedelta
from pathlib import Path
from unittest.mock import Mock

import numpy as np
from sklearn.ensemble import IsolationForest

from ml.anomaly.to_risk_event import envelope
from ml.service.artifact import (
    PINNED_LIBRARIES,
    ArtifactError,
    load_verified,
    sha256_of,
)
from ml.service.evidence import question_evidence
from ml.service.pipeline_v2 import POLICY, attempt_v2, history_v2, prepare_v2
from ml.service.processor import Processor
from ml.service.quiz_reference import QuizReferences, fit_quiz
from ml.service.reference_worker import ingest_review
from ml.service.schema import EventValidator, find_dir
from ml.service.scoring import Scorer
from ml.service.store import PostgresStore
from ml.service.tests.test_service import T0, FakeStore, live_attempt
from ml.service.upgrade_v2 import upgrade


def model() -> dict:
    values = np.column_stack((np.arange(12), np.zeros(12)))
    est = IsolationForest(n_estimators=8, random_state=42).fit(values)
    return {"schema_version": 2, "input_policy": POLICY, "version": "0.1.0-v2",
            "reference": {"global_fast_ms": 100, "item_fast_ms": {}, "answer_counts": {}, "delta_sd_floor": 5},
            "models": {"iforest": {"kind": "iforest", "columns": ["score_percent"], "medians": [0],
                                   "estimator": est, "train_scores": -est.score_samples(values), "bounds": {},
                                   "medium_threshold": 0.0, "high_threshold": 0.0}}}


def review_event(aid: str, decision: str = "confirmed") -> dict:
    env = envelope("risk.reviewed", {"risk_id": "risk_test", "target_type": "attempt", "target_id": aid,
                                     "decision": decision, "reviewed_by": "teacher", "comment": ""},
                   {"course_id": "c", "group_id": "g", "session_id": None})
    env.update(producer="grading", actor={"user_id": "teacher", "role": "teacher"})
    return env


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

    def test_cold_suppresses_then_warm_publishes(self):
        store, refs = FakeStore(), Mock()
        refs.get.return_value = None
        p = Processor(store, Scorer(model()), self.validator, timedelta(0), references=refs)
        evs = live_attempt("u1", "g", "quiz", "v", T0, [100], ["x"])
        for e in evs:
            p.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(seconds=6)
        p.score_ready()
        self.assertFalse(store.outbox)
        aid = evs[-1]["payload"]["attempt_id"]
        self.assertEqual(store.attempts[aid]["detail"], "cold_start_no_publish")
        later = live_attempt("u2", "g", "quiz", "v", T0 + timedelta(minutes=1), [900], ["y"])
        earlier = live_attempt("u0", "g", "quiz", "v", T0 - timedelta(minutes=1), [250], ["z"])
        pool = [attempt_v2(earlier), attempt_v2(evs)]
        self.assertIsNone(fit_quiz(pool, 3))
        ref = fit_quiz(pool, 2)
        self.assertEqual(question_evidence(attempt_v2(evs), evs, ref)[0]["reference_scope"], "global_quiz_history")
        refs.get.return_value = ("digest", ref)
        for e in later:
            p.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(days=1)
        p.score_ready()
        self.assertEqual(len(store.outbox), 1)
        event = json.loads(store.unpublished(1)[0].envelope)
        self.assertEqual(event["payload"]["evidence"]["features"]["reference_version"], "digest")

    def test_teacher_review_contract(self):
        repo = Mock()
        env = review_event("valid_attempt")
        self.assertEqual(ingest_review(json.dumps(env).encode(), repo, self.validator), "processed")
        repo.review.assert_called_once()
        env["actor"]["role"] = "student"
        self.assertEqual(ingest_review(json.dumps(env).encode(), repo, self.validator), "rejected")
        self.assertEqual(ingest_review(b"not-json", repo, self.validator), "rejected")
        repo.review.assert_called_once()


@unittest.skipUnless(os.environ.get("E15_TEST_DATABASE_URL"), "requires dedicated test PostgreSQL")
class ReferenceDatabaseTests(unittest.TestCase):
    @unittest.skipUnless(os.environ.get("E15_TEST_BROKERS"), "requires dedicated test Kafka")
    def test_real_review_worker_ingests_confirmation(self):
        from confluent_kafka import Producer

        dsn = os.environ["E15_TEST_DATABASE_URL"]
        store = PostgresStore(dsn)
        self.addCleanup(store.close)
        store.migrate()
        repo = QuizReferences(dsn)
        self.addCleanup(repo.close)
        brokers = os.environ["E15_TEST_BROKERS"]
        env = os.environ | {"ANOMALY_DATABASE_URL": dsn, "ANOMALY_KAFKA_BROKERS": brokers,
                            "ANOMALY_REFERENCE_REBUILD_SECONDS": "60"}
        proc = subprocess.Popen([sys.executable, "-m", "ml.service.reference_worker", "worker"], env=env)
        try:
            aid = str(uuid.uuid4())
            kafka = Producer({"bootstrap.servers": brokers})
            kafka.produce("lms.grades", key=aid, value=json.dumps(review_event(aid)).encode())
            self.assertEqual(kafka.flush(12), 0)
            end = time.monotonic() + 20
            while time.monotonic() < end:
                self.assertIsNone(proc.poll(), "review worker exited early")
                if repo.connection.execute("SELECT 1 FROM reference_exclusions WHERE attempt_id=%s", (aid,)).fetchone():
                    break
                time.sleep(.2)
            else:
                self.fail("valid broker confirmation never reached the exclusion table")
        finally:
            proc.terminate()
            proc.wait(timeout=8)

    def test_rollback_exclusions_and_asof(self):
        dsn = os.environ["E15_TEST_DATABASE_URL"]
        store = PostgresStore(dsn)
        self.addCleanup(store.close)
        store.migrate()
        repo = QuizReferences(dsn)
        self.addCleanup(repo.close)
        quiz = "quiz_" + uuid.uuid4().hex
        def insert(n: int) -> str:
            events = live_attempt(f"s{n}", "g", quiz, "v", T0 + timedelta(minutes=n), [100+n], ["A"])
            for event in events:
                store.add_event(event, T0)
            aid = events[-1]["payload"]["attempt_id"]
            store.finish(aid, "scored", .1, "low", "cold_start_no_publish", None, None)
            return aid
        insert(1)
        self.assertIsNone(repo.rebuild(quiz, 2, 50))
        insert(2)
        old = repo.rebuild(quiz, 2, 50)
        self.assertIsNone(repo.get(quiz, T0))
        last = insert(3)
        new = repo.rebuild(quiz, 2, 50)
        future = T0 + timedelta(days=1)
        self.assertNotEqual(old, new)
        self.assertEqual(repo.get(quiz, future)[0], new)
        repo.set_head(quiz, old)
        repo.rebuild(quiz, 2, 50)
        self.assertEqual(repo.get(quiz, future)[0], old)
        repo.set_head(quiz, None)
        repo.rebuild(quiz, 2, 50)
        self.assertEqual(repo.get(quiz, future)[0], new)
        repo.review(review_event(last, "dismissed"))
        self.assertEqual(repo.get(quiz, future)[0], new)
        record = review_event(last)
        repo.review(record)
        repo.review(record)
        self.assertIsNone(repo.get(quiz, future))
        with self.assertRaisesRegex(ValueError, "eligible"):
            repo.set_head(quiz, new)
        self.assertEqual(repo.rebuild(quiz, 2, 50), old)
        self.assertEqual(repo.get(quiz, future)[0], old)
