"""Synthetic text fixtures for the scorer and its durable PostgreSQL lifecycle."""

import os
import unittest
import uuid
from datetime import UTC, datetime
from pathlib import Path

import psycopg

from ml.integrity_service.engine import compare
from ml.integrity_service.store import ObjectUnavailable, Store
from ml.service.schema import EventValidator

TEXT = " ".join(f"слово{i}é" for i in range(130))


def submitted(sid, author, second, assignment="exercise"):
    now = datetime(2026, 11, 1, 12, 0, second, tzinfo=UTC)
    timestamp = now.isoformat(timespec="milliseconds").replace("+00:00", "Z")
    bits = (
        (int(now.timestamp() * 1000) << 80)
        | (7 << 76)
        | (2 << 62)
        | (uuid.uuid4().int & ((1 << 62) - 1))
    )
    return {
        "event_id": str(uuid.UUID(int=bits)),
        "event_type": "submission.created",
        "version": 1,
        "occurred_at": timestamp,
        "producer": "assignments",
        "actor": {"user_id": author, "role": "student"},
        "context": {"course_id": "course", "group_id": "group", "session_id": None},
        "payload": {
            "submission_id": sid,
            "assignment_id": assignment,
            "attempt": 1,
            "kind": "text",
            "language": "ru",
            "files": [],
            "text_ref": f"minio://submissions/{sid}/extracted.txt",
            "submitted_at": timestamp,
            "deadline_at": "2026-11-02T12:00:00Z",
        },
    }


class SimilarityTests(unittest.TestCase):
    def test_direction_scores_offsets_and_templates(self):
        earlier, later = submitted("source", "alice", 1), submitted("target", "bob", 2)
        # Deliver the later paper first and change one word in the target.
        changed = "🙂 " + TEXT.replace("слово62é", "исправлено")
        risk = compare((later, changed), (earlier, TEXT), {})
        p = risk["payload"]
        self.assertEqual(p["target_id"], "target")
        self.assertEqual(p["evidence"]["matches"][0]["submission_id"], "source")
        for span in p["evidence"]["matches"][0]["overlap_spans"]:
            self.assertEqual(
                changed[slice(*span["target"])], TEXT[slice(*span["source"])]
            )
        self.assertEqual(
            compare((earlier, TEXT), (later, changed), {})["payload"]["risk_id"],
            p["risk_id"],
        )
        self.assertIsNone(compare((earlier, TEXT), (later, TEXT), {"exercise": TEXT}))
        self.assertIsNone(compare((earlier, TEXT), (later, "different " * 100), {}))
        self.assertIsNone(compare((earlier, TEXT), (later, "too short"), {}))
        self.assertIsNone(
            compare((earlier, TEXT), (submitted("self", "alice", 2), TEXT), {})
        )


@unittest.skipUnless(
    os.getenv("TEST_INTEGRITY_DATABASE_URL"), "set a disposable PostgreSQL DSN"
)
class DurableTests(unittest.TestCase):
    def test_retry_duplicates_scope_and_ack(self):
        schema = "fixture_" + uuid.uuid4().hex
        validator = EventValidator(
            Path(os.getenv("SCHEMA_DIR", "docs/03_architecture/schemas"))
        )

        def validate(event):
            self.assertEqual(validator.errors(event), [])

        with psycopg.connect(
            os.environ["TEST_INTEGRITY_DATABASE_URL"], autocommit=True
        ) as db:
            db.execute(f"CREATE SCHEMA {schema}")
            self.addCleanup(self.drop_schema, schema)
            db.execute(f"SET search_path TO {schema}")
            store = Store(db, "assignment", {})
            events = [
                submitted("later", "b", 5),
                submitted("early", "a", 1),
                submitted("outside", "c", 6, "unrelated"),
                submitted("short", "d", 3),
            ]
            for e in events:
                validate(e)
                store.accept(e)
                store.accept(e)

            def read(event):
                return (
                    "short text"
                    if event["payload"]["submission_id"] == "short"
                    else TEXT
                )

            def unavailable(_):
                raise ObjectUnavailable

            self.assertTrue(store.work(unavailable, validate))
            self.assertEqual(
                db.execute(
                    "SELECT state FROM integrity_docs WHERE id='later'"
                ).fetchone()[0],
                "pending",
            )
            db.execute("UPDATE integrity_docs SET retry_at=now()")
            while store.work(read, validate):
                pass
            states = dict(db.execute("SELECT id,state FROM integrity_docs").fetchall())
            self.assertEqual(
                states,
                {
                    "early": "indexed",
                    "later": "indexed",
                    "outside": "indexed",
                    "short": "abstained",
                },
            )
            self.assertEqual(
                db.execute("SELECT count(*) FROM integrity_outbox").fetchone()[0], 1
            )
            attempts = []

            def fail(key, event):
                attempts.append((key, event))
                raise RuntimeError("Broker temporarily unavailable")

            with self.assertRaises(RuntimeError):
                store.publish_one(fail)
            store.publish_one(lambda k, e: attempts.append((k, e)))
            self.assertEqual(attempts[0], attempts[1])
            self.assertEqual(attempts[1][0], "later")
            self.assertFalse(store.publish_one(fail))

    @staticmethod
    def drop_schema(schema):
        with psycopg.connect(
            os.environ["TEST_INTEGRITY_DATABASE_URL"], autocommit=True
        ) as db:
            db.execute(f"DROP SCHEMA {schema} CASCADE")
