"""Consume teacher reviews and periodically rebuild per-quiz scoring references."""
from __future__ import annotations

import argparse
import json
import logging
import os
import time
from pathlib import Path

from .kafka_io import make_consumer, record_error
from .quiz_reference import QuizReferences
from .schema import EventValidator, find_dir

log = logging.getLogger("anomaly.reference")


def ingest_review(raw: bytes, repo: QuizReferences, validator: EventValidator) -> str:
    if len(raw) > 64 * 1024:
        return "rejected"
    try:
        record = json.loads(raw)
    except (ValueError, RecursionError):
        return "rejected"
    if not isinstance(record, dict):
        return "rejected"
    if record.get("event_type") != "risk.reviewed":
        return "ignored"
    if validator.errors(record):
        return "rejected"
    repo.review(record)
    return "processed"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("worker", "rebuild", "rollback", "resume", "list"))
    parser.add_argument("--quiz")
    parser.add_argument("--version", help="snapshot digest for rollback")
    args = parser.parse_args()
    if args.command != "worker" and not args.quiz:
        parser.error("--quiz is required")
    if args.command == "rollback" and not args.version:
        parser.error("--version is required for rollback")
    minimum = int(os.getenv("ANOMALY_REFERENCE_MIN_ATTEMPTS", "30"))
    window = int(os.getenv("ANOMALY_REFERENCE_WINDOW", "500"))
    seconds = int(os.getenv("ANOMALY_REFERENCE_REBUILD_SECONDS", "3600"))
    if not 2 <= minimum <= window <= 10000 or seconds < 1:
        parser.error("require 2 <= minimum <= window <= 10000 and a positive rebuild interval")
    repo = QuizReferences(os.environ["ANOMALY_DATABASE_URL"])
    try:
        if args.command in {"rollback", "resume"}:
            repo.set_head(args.quiz, args.version if args.command == "rollback" else None)
            return
        if args.command == "rebuild":
            print(repo.rebuild(args.quiz, minimum, window) or "cold")
            return
        if args.command == "list":
            for row in repo.connection.execute(
                "SELECT v.version,v.source_through,cardinality(v.members),v.version=h.version,h.pinned "
                "FROM quiz_reference_versions v JOIN quiz_reference_heads h USING (quiz_id) "
                "WHERE v.quiz_id=%s ORDER BY v.created_at DESC", (args.quiz,)
            ):
                print(*row)
            return
        schema_dir = os.getenv("ANOMALY_SCHEMA_DIR")
        validator = EventValidator(Path(schema_dir) if schema_dir else find_dir())
        consumer = make_consumer(os.environ["ANOMALY_KAFKA_BROKERS"], "anomaly-live-reference-v1", "lms.grades")
        try:
            next_rebuild = time.monotonic() + seconds
            while True:
                msg = consumer.poll(1)
                if msg is not None and msg.error() is None:
                    if ingest_review(msg.value() or b"", repo, validator) == "rejected":
                        log.warning("rejected malformed reference review")
                    consumer.store_offsets(message=msg)
                elif msg is not None and record_error(msg) is not None:
                    log.warning("reference consumer unavailable")
                if time.monotonic() >= next_rebuild:
                    for (quiz,) in repo.connection.execute("SELECT DISTINCT quiz_id FROM attempts").fetchall():
                        repo.rebuild(quiz, minimum, window)
                    next_rebuild = time.monotonic() + seconds
        except KeyboardInterrupt:
            pass
        finally:
            consumer.close()
    finally:
        repo.close()


if __name__ == "__main__":
    main()
