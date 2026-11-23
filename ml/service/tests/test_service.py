"""Unit tests for the anomaly scoring service (fakes; no Kafka, no Postgres).

Run from the repository root:  python -m unittest discover -s ml/service/tests -v
"""
from __future__ import annotations

import csv
import hashlib
import json
import os
import pickle
import secrets
import subprocess
import sys
import tempfile
import unittest
import uuid
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

from ml.anomaly.features import write_json
from ml.anomaly.split import create_manifest
from ml.anomaly.train import train
from ml.service.artifact import ArtifactError, load_verified, sha256_of
from ml.service.detectors import DEFAULT_MODEL, NAMES, ModelScore, explain_rules
from ml.service.processor import Processor
from ml.service.schema import EventValidator, find_dir
from ml.service.scoring import Scorer, check_attempt, deterministic_risk_id, level_for
from ml.service.store import OutboxRow, Pending
from ml.service.watermark import is_ready

T0 = datetime(2026, 11, 20, 9, 0, tzinfo=UTC)


class FakeStore:
    """In-memory Store with the same idempotency rules as PostgresStore."""

    def __init__(self) -> None:
        self.events: dict[str, dict[str, Any]] = {}
        self.attempts: dict[str, dict[str, Any]] = {}
        self.outbox: dict[str, dict[str, Any]] = {}
        self.clock = T0

    def add_event(self, envelope: dict[str, Any], occurred_at: datetime) -> bool:
        if envelope["event_id"] in self.events:
            return False
        self.events[envelope["event_id"]] = {"env": envelope, "at": occurred_at}
        if envelope["event_type"] == "quiz.submitted":
            p = envelope["payload"]
            self.attempts.setdefault(p["attempt_id"], {
                "group_id": envelope["context"]["group_id"], "quiz_id": p["quiz_id"], "submitted_at": occurred_at,
                "received_at": self.clock, "status": "pending", "level": None, "raw": None, "detail": ""})
        return True

    def watermark(self) -> datetime | None:
        return max((e["at"] for e in self.events.values()), default=None)

    def now(self) -> datetime:
        return self.clock

    def pending(self, limit: int) -> list[Pending]:
        rows = [Pending(a, v["group_id"], v["quiz_id"], v["submitted_at"], v["received_at"])
                for a, v in self.attempts.items() if v["status"] == "pending"]
        return sorted(rows, key=lambda p: p.submitted_at)[:limit]

    def cohort_events(self, group_id: str, quiz_id: str) -> dict[str, list[dict[str, Any]]]:
        members = {a for a, v in self.attempts.items() if v["group_id"] == group_id and v["quiz_id"] == quiz_id}
        out: dict[str, list[dict[str, Any]]] = {}
        for e in sorted(self.events.values(), key=lambda e: (e["at"], e["env"]["event_id"])):
            if e["env"]["payload"]["attempt_id"] in members:
                out.setdefault(e["env"]["payload"]["attempt_id"], []).append(e["env"])
        return out

    def finish(self, attempt_id: str, status: str, raw: float | None, level: str | None, detail: str,
               outbox: OutboxRow | None, event_id: str | None, scores: tuple[ModelScore, ...]) -> bool:
        a = self.attempts[attempt_id]
        if a["status"] != "pending":
            return False
        a.update(status=status, raw=raw, level=level, detail=detail, model_scores=scores)
        if outbox is not None and outbox.risk_id not in self.outbox:
            self.outbox[outbox.risk_id] = {"row": outbox, "event_id": event_id, "published": False}
        return True

    def unpublished(self, limit: int) -> list[OutboxRow]:
        return [v["row"] for v in self.outbox.values() if not v["published"]][:limit]

    def mark_published(self, ids: list[str]) -> None:
        for i in ids:
            self.outbox[i]["published"] = True

    def ping(self) -> None:
        return None


class FakePublisher:
    def __init__(self, fail: set[str] | None = None) -> None:
        self.fail, self.sent = fail or set(), []

    def publish(self, rows: list[OutboxRow]) -> list[str]:
        self.sent.extend(rows)
        return [r.risk_id for r in rows if r.risk_id not in self.fail]


def uuid7(at: datetime) -> str:
    ms = int(at.timestamp() * 1000)
    bits = (ms << 80) | (7 << 76) | (secrets.randbits(12) << 64) | (2 << 62) | secrets.randbits(62)
    return str(uuid.UUID(int=bits))


def ts(at: datetime) -> str:
    return at.isoformat(timespec="milliseconds").replace("+00:00", "Z")


def live_attempt(student: str, group: str, quiz: str, variant: str, start: datetime, first_ms: list[int],
                 options: list[str], gap_ms: int = 300) -> list[dict[str, Any]]:
    """Platform-shaped events (UUIDs, additive `reason`) of one fully answered
    attempt with 1 point per question, as the quizzes service emits them."""
    attempt = str(uuid.uuid4())
    actor = {"user_id": student, "role": "student"}
    context = {"course_id": "c_live", "group_id": group, "session_id": None}
    events, elapsed = [], 0
    for i, (ms, opt) in enumerate(zip(first_ms, options, strict=True)):
        elapsed += gap_ms + ms
        at = start + timedelta(milliseconds=elapsed)
        events.append({"event_id": uuid7(at), "event_type": "quiz.answered", "version": 1, "occurred_at": ts(at),
                       "producer": "quizzes", "actor": actor, "context": context,
                       "payload": {"attempt_id": attempt, "question_id": f"{quiz}-q{i}", "question_index": i,
                                   "answer_hash": hashlib.sha256(opt.encode()).hexdigest(), "is_change": False,
                                   "time_since_shown_ms": ms, "time_since_attempt_start_ms": elapsed}})
    duration = elapsed + 1000
    at = start + timedelta(milliseconds=duration)
    events.append({"event_id": uuid7(at), "event_type": "quiz.submitted", "version": 1, "occurred_at": ts(at),
                   "producer": "quizzes", "actor": actor, "context": context,
                   "payload": {"attempt_id": attempt, "quiz_id": quiz, "variant_id": variant, "score": len(first_ms) // 2,
                               "max_score": len(first_ms), "duration_ms": duration, "answers_changed": 0,
                               "reason": "submitted"}})
    return events


def csv_envelopes(path: Path) -> list[dict[str, Any]]:
    out = []
    with path.open(newline="", encoding="utf-8") as stream:
        for row in csv.DictReader(stream):
            env = {k: row[k] for k in ("event_id", "event_type", "occurred_at", "producer")}
            env["version"] = int(row["version"])
            for k in ("actor", "context", "payload"):
                env[k] = json.loads(row[k])
            out.append(env)
    return out


class ServiceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.tmp = tempfile.TemporaryDirectory(prefix="anomaly-svc-")
        root = Path(cls.tmp.name)
        cls.csv = root / "quiz.csv"
        subprocess.run([sys.executable, "ml/datasets/generate_quiz_behavior.py", "--students", "120", "--seed", "42",
                        "--output", str(cls.csv)], check=True, capture_output=True)
        write_json(root / "split.json", create_manifest(cls.csv, 42))
        train(cls.csv, root / "split.json", root / "model")
        cls.model_path = root / "model" / "model.pkl"
        os.chmod(cls.model_path, 0o600)
        cls.artifact = load_verified(cls.model_path)
        cls.validator = EventValidator(find_dir())
        cls.envelopes = csv_envelopes(cls.csv)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.tmp.cleanup()

    def processor(self, publish_model: str = DEFAULT_MODEL) -> tuple[Processor, FakeStore]:
        store = FakeStore()
        return Processor(store, Scorer(self.artifact, publish_model), self.validator, timedelta(seconds=5)), store

    # --- watermark -------------------------------------------------------
    def test_watermark(self) -> None:
        sub, recv = T0, T0
        cases = [
            (T0 + timedelta(seconds=4.999), recv, False, "waiting"),
            (T0 + timedelta(seconds=5), recv, True, "watermark"),
            (None, recv, False, "waiting"),
            (T0, recv - timedelta(seconds=10), True, "lateness"),  # quiet stream: processing-time bound
        ]
        for watermark, received, ready, reason in cases:
            r = is_ready(sub, watermark, received, T0 + timedelta(seconds=1), timedelta(seconds=5))
            self.assertEqual((r.ready, r.reason), (ready, reason), (watermark, received))
        with self.assertRaises(ValueError):
            is_ready(datetime(2026, 1, 1), None, T0, T0, timedelta(0))

    def test_waits_for_peers_then_scores(self) -> None:
        proc, store = self.processor()
        events = live_attempt("s1", "g1", "quiz-w", "v1", T0, [20000] * 20, ["a"] * 20)
        for e in events:
            proc.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(seconds=1)  # processing time 1 s after receipt; watermark == own submission
        self.assertEqual(proc.score_ready(), 0, "must wait for the 5 s event-time watermark")
        later = live_attempt("s2", "g1", "quiz-w", "v1", T0 + timedelta(seconds=30), [20000] * 20, ["b"] * 20)
        for e in later:
            proc.handle(json.dumps(e).encode())
        self.assertEqual(proc.score_ready(), 1, "watermark passed for the first attempt only")

    # --- ingestion -------------------------------------------------------
    def test_idempotent_ingest_and_rejections(self) -> None:
        proc, store = self.processor()
        sample = self.envelopes[:200]
        results = [proc.handle(json.dumps(e).encode()) for e in sample + sample]
        self.assertEqual(results.count("stored"), 200)
        self.assertEqual(results.count("duplicate"), 200)
        bad = dict(sample[0], payload=dict(sample[0]["payload"], time_since_shown_ms=-5))
        bad["event_id"] = uuid7(T0)
        for raw in (b"\xff\xfe", b"[1,2]", b"{}", json.dumps(bad).encode(), b"x" * 70000):
            self.assertIn(proc.handle(raw), ("rejected", "ignored"))
        self.assertEqual(proc.handle(json.dumps({"event_type": "user.created"}).encode()), "ignored")
        self.assertEqual(len(store.events), 200)

    # --- scoring, events, determinism -------------------------------------
    def test_scores_generator_cohorts_and_events_match_schema(self) -> None:
        # Preserve the original forest replay regression while the default changes.
        proc, store = self.processor("iforest")
        for e in self.envelopes:
            proc.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(days=400)
        while proc.score_ready(limit=500):
            pass
        statuses = {v["status"] for v in store.attempts.values()}
        self.assertNotIn("pending", statuses)
        self.assertNotIn("not_scorable", statuses, {a: v["detail"] for a, v in store.attempts.items() if v["status"] == "not_scorable"})
        self.assertGreater(len(store.outbox), 0, "the generator injects anomalies; some must be flagged")
        model = self.artifact["models"]["iforest"]
        for risk_id, item in store.outbox.items():
            event = json.loads(item["row"].envelope)
            self.assertEqual(self.validator.errors(event), [], event["payload"]["risk_id"])
            payload = event["payload"]
            self.assertEqual(risk_id, deterministic_risk_id(self.artifact["version"], payload["target_id"]))
            self.assertEqual(item["event_id"], event["event_id"])
            attempt = store.attempts[payload["target_id"]]
            self.assertEqual(attempt["status"], "flagged")
            self.assertIn(payload["level"], ("medium", "high"))
            self.assertEqual(payload["level"], level_for(attempt["raw"], model))
            self.assertIn("not a probability or proof of misconduct", payload["explanation"])
        for v in store.attempts.values():
            if v["status"] == "scored":
                self.assertEqual(v["level"], "low")
                self.assertLess(v["raw"], model["medium_threshold"])
        # Re-processing (e.g. after losing scoring state) never creates a second signal.
        before = dict(store.outbox)
        for v in store.attempts.values():
            v["status"] = "pending"
        while proc.score_ready(limit=500):
            pass
        self.assertEqual(set(store.outbox), set(before))
        # Publishing marks only acknowledged rows; a failed one stays pending.
        ids = list(store.outbox)
        pub = FakePublisher(fail={ids[0]})
        self.assertEqual(proc.publish(pub), len(ids) - 1)
        self.assertEqual([r.risk_id for r in store.unpublished(10)], [ids[0]])
        self.assertEqual(proc.publish(FakePublisher()), 1)

    def test_live_platform_events_are_scorable(self) -> None:
        proc, store = self.processor()
        start = T0
        for i in range(8):  # normal pace, staggered submissions
            for e in live_attempt(f"n{i}", "g-live", "quiz-live", "v1", start + timedelta(minutes=i),
                                  [18000 + 900 * i] * 20, [f"o{j % 4}" for j in range(20)]):
                proc.handle(json.dumps(e).encode())
        for name in ("fast1", "fast2"):  # every answer < 1 s, identical answers, synchronous submit
            for e in live_attempt(name, "g-live", "quiz-live", "v1", start + timedelta(minutes=20), [700] * 20, ["x"] * 20):
                proc.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(hours=1)
        while proc.score_ready():
            pass
        detail = {a: (v["status"], v["level"], round(v["raw"] or 0, 4)) for a, v in store.attempts.items()}
        self.assertTrue(all(v["status"] != "not_scorable" for v in store.attempts.values()), detail)
        flagged = [json.loads(o["row"].envelope)["payload"] for o in store.outbox.values()]
        fast_attempts = {a for a, v in store.attempts.items() if v["raw"] is not None and
                         any(ev["actor"]["user_id"].startswith("fast")
                             for ev in store.cohort_events("g-live", "quiz-live")[a])}
        self.assertEqual(len(fast_attempts), 2)
        self.assertTrue(fast_attempts <= {p["target_id"] for p in flagged},
                        f"demo model did not flag the crafted attempts: {detail}")
        for p in flagged:
            if p["target_id"] in fast_attempts:
                self.assertEqual(len(p["evidence"]["features"]["synchronous_with"]), 1)

    def test_not_scorable_inputs(self) -> None:
        events = live_attempt("s", "g", "q", "v", T0, [5000] * 5, ["a"] * 5)
        self.assertIsNone(check_attempt(events))
        missing = [e for e in events if e["payload"].get("question_index") != 2]  # one unanswered question
        self.assertIn("not scorable", check_attempt(missing))
        points = [dict(e, payload=dict(e["payload"], max_score=10)) if e["event_type"] == "quiz.submitted" else e
                  for e in events]  # 2 points per question
        self.assertIn("not scorable", check_attempt(points))
        proc, store = self.processor()
        for e in missing:
            proc.handle(json.dumps(e).encode())
        store.clock = T0 + timedelta(hours=1)
        proc.score_ready()
        (only,) = store.attempts.values()
        self.assertEqual(only["status"], "not_scorable")
        self.assertEqual(store.outbox, {})

    def test_deterministic_risk_id(self) -> None:
        a = deterministic_risk_id("0.1.0", "att-1")
        self.assertEqual(a, deterministic_risk_id("0.1.0", "att-1"))
        self.assertNotEqual(a, deterministic_risk_id("0.1.0", "att-2"))
        self.assertNotEqual(a, deterministic_risk_id("0.2.0", "att-1"))
        uuid.UUID(a)

    def test_level_mapping(self) -> None:
        model = {"medium_threshold": 0.5, "high_threshold": 0.6}
        for raw, want in ((0.49, "low"), (0.5, "medium"), (0.59, "medium"), (0.6, "high"), (9, "high")):
            self.assertEqual(level_for(raw, model), want)
        equal = {"medium_threshold": 0.5, "high_threshold": 0.5}  # README: equal cutoffs -> no medium cases
        self.assertEqual(level_for(0.5, equal), "high")

    def test_rule_explanation_reports_raw_bounds_and_ties(self) -> None:
        model = {"columns": ["mean_answer_time_ms"], "medians": np.array([np.log1p(20)]),
                 "sorted_train": np.array([[np.log1p(v), 0.] for v in (10, 20, 20, 30)]),
                 "medium_threshold": .75}
        # Both endpoint ties qualify exactly at .75. Values in between do not.
        for value, bound in [(10, "<= 10"), (30, ">= 30")]:
            detail = explain_rules(pd.DataFrame({"mean_answer_time_ms": [value]}), model, .75)
            self.assertIn(bound, detail)
            self.assertIn("mean_answer_time_ms", detail)
            self.assertNotIn(".missing", detail)
        detail = explain_rules(pd.DataFrame({"mean_answer_time_ms": [np.nan]}), model, 1.)
        self.assertIn("mean_answer_time_ms.missing=1 (rule > 0", detail)

    def test_published_and_shadow_model_switch(self) -> None:
        self.assertEqual(Scorer(self.artifact).publish_model, "quantile_rules")
        with self.assertRaisesRegex(ValueError, "ANOMALY_PUBLISH_MODEL"):
            Scorer(self.artifact, "unrecognized")
        events = live_attempt("u", "g", "q", "v", T0, [500] * 20, ["a"] * 20)
        attempt_id = events[-1]["payload"]["attempt_id"]
        # Routing test cutoffs exercise both routes independently of either model's performance.
        for key in NAMES:
            for alert in (True, False):
                store = FakeStore()
                models = {k: dict(v) for k, v in self.artifact["models"].items()}
                for name in NAMES:
                    models[name]["medium_threshold"] = .1 if alert or name != key else 1.1
                    models[name]["high_threshold"] = 1.1
                artifact = dict(self.artifact, models=models)
                proc = Processor(store, Scorer(artifact, key), self.validator, timedelta(seconds=5))
                for event in events:
                    proc.handle(json.dumps(event).encode())
                store.clock += timedelta(hours=1)
                self.assertEqual(proc.score_ready(), 1)
                scores = {s.key: s for s in store.attempts[attempt_id]["model_scores"]}
                self.assertEqual(set(scores), set(NAMES))
                other = next(k for k in NAMES if k != key)
                self.assertTrue(scores[other].shadow)
                self.assertFalse(scores[key].shadow)
                self.assertEqual(scores[key].raw, store.attempts[attempt_id]["raw"])
                self.assertEqual(scores[other].level, "medium")
                self.assertEqual(len(store.outbox), int(alert))
                if alert:
                    risk = json.loads(next(iter(store.outbox.values()))["row"].envelope)
                    self.assertEqual(self.validator.errors(risk), [])
                    self.assertEqual(risk["payload"]["model"]["name"], NAMES[key])
                    self.assertEqual(risk["payload"]["risk_id"], deterministic_risk_id("0.1.0", attempt_id, key))
                    if key == "quantile_rules":
                        self.assertIn("(rule ", risk["payload"]["explanation"])
                        self.assertNotIn("Isolation Forest", risk["payload"]["explanation"])
                initial = dict(store.outbox)
                restarted = Processor(store, Scorer(artifact, other), self.validator, timedelta(seconds=5))
                for event in events:
                    restarted.handle(json.dumps(event).encode())
                self.assertEqual(restarted.score_ready(), 0)
                self.assertEqual(store.outbox, initial)

    # --- artifact verification -------------------------------------------
    def test_artifact_verification(self) -> None:
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            with self.assertRaises(ArtifactError):
                load_verified(root / "missing.pkl")
            good = root / "model.pkl"
            good.write_bytes(self.model_path.read_bytes())
            os.chmod(good, 0o600)
            load_verified(good, sha256_of(good))
            with self.assertRaises(ArtifactError):
                load_verified(good, "0" * 64)  # pin mismatch is checked before unpickling
            os.chmod(good, 0o666)
            with self.assertRaises(ArtifactError):
                load_verified(good)
            for mutate in (lambda a: a.update(schema_version=2),
                           lambda a: a["environment"]["libraries"].update({"scikit-learn": "0.0.1"}),
                           lambda a: a["models"].pop("iforest"),
                           lambda a: a["models"].pop("quantile_rules"),
                           lambda a: a["models"]["quantile_rules"].update(sorted_train=np.array([]))):
                artifact = pickle.loads(self.model_path.read_bytes())
                mutate(artifact)
                bad = root / "bad.pkl"
                bad.write_bytes(pickle.dumps(artifact))
                os.chmod(bad, 0o600)
                with self.assertRaises(ArtifactError):
                    load_verified(bad)


if __name__ == "__main__":
    unittest.main()
