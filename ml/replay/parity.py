"""Compare an isolated research replay with an independent frozen-model run on its source world."""
from __future__ import annotations

import argparse
import csv
import hashlib
import io
import json
import math
import os
import time
import uuid
from collections import Counter, defaultdict
from collections.abc import Iterable
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import numpy as np
import psycopg
from psycopg.rows import dict_row

from ml.anomaly.features import (
    FEATURE_COLUMNS,
    build_features,
    read_attempts,
    write_json,
)
from ml.anomaly.train import empirical_cdf, score_model
from ml.service.artifact import load_verified, sha256_of
from ml.service.scoring import to_csv

Record = dict[str, Any]


class Differences:
    """Public counters, max absolute errors, and synthetic attempt ordinals only."""

    def __init__(self, tolerance: float) -> None:
        if not math.isfinite(tolerance) or tolerance < 0:
            raise ValueError("tolerance must be finite and nonnegative")
        self.tolerance = tolerance
        self.compared: Counter[str] = Counter()
        self.mismatched: Counter[str] = Counter()
        self.maximum: dict[str, float] = {}
        self.failed: dict[int, set[str]] = defaultdict(set)

    def check(self, row: int, field: str, expected: Any, observed: Any, *, numeric: bool = False) -> None:
        self.compared[field] += 1
        if numeric and expected is not None and observed is not None:
            delta = abs(float(expected) - float(observed))
            if math.isfinite(delta):
                self.maximum[field] = max(delta, self.maximum.get(field, 0.0))
            match = math.isfinite(delta) and delta <= self.tolerance
        else:
            match = expected == observed
        if not match:
            self.mismatched[field] += 1
            self.failed[row].add(field)

    def report(self) -> Record:
        return {"comparisons": dict(sorted(self.compared.items())), "mismatches": dict(sorted(self.mismatched.items())),
                "max_absolute_difference": dict(sorted(self.maximum.items())),
                "mismatched_attempts": [{"attempt_number": i, "fields": sorted(fields)}
                                        for i, fields in sorted(self.failed.items())]}


def score_world(attempts: list[Record], artifact: Record) -> dict[str, Record]:
    model = artifact["models"]["iforest"]
    frame = build_features(attempts, artifact["reference"])
    raw = score_model(frame, model)
    result = {}
    for row, value, rank in zip(frame.to_dict("records"), raw, empirical_cdf(model["train_scores"], raw), strict=True):
        if not math.isfinite(value):
            raise ValueError("non-finite offline model output")
        level = "high" if value >= model["high_threshold"] else "medium" if value >= model["medium_threshold"] else "low"
        features = {key: None if np.isnan(row[key]) else float(row[key]) for key in FEATURE_COLUMNS}
        result[row["attempt_id"]] = {"features": features, "peers": sorted(row["synchronous_with"]),
                                     "raw_score": float(value), "score": float(rank), "level": level,
                                     "status": "scored" if level == "low" else "flagged",
                                     "submitted_at": row["submitted_at"].to_pydatetime()}
    return result


def reconstruct(events: dict[str, list[Record]], artifact: Record) -> tuple[dict[str, Record], set[str]]:
    """Use the runtime CSV adapter and the retained quiz stream; rebuild the final cohort feature view."""
    accepted = []
    excluded = set()
    for key, group in events.items():
        try:
            read_attempts(io.StringIO(to_csv(group)))
        except (ValueError, TypeError, KeyError):
            excluded.add(key)
        else:
            accepted.extend(group)
    if not accepted:
        return {}, excluded
    return score_world(read_attempts(io.StringIO(to_csv(accepted))), artifact), excluded


def observe(dsn: str, ids: list[str], wait_seconds: float) -> tuple[dict[str, Record], dict[str, list[Record]]]:
    """One read-only snapshot per poll, limited to the replay's own attempts."""
    deadline = time.monotonic() + wait_seconds
    with psycopg.connect(dsn, connect_timeout=4, options="-c default_transaction_read_only=on -c statement_timeout=15000",
                         row_factory=dict_row) as conn:
        conn.isolation_level = psycopg.IsolationLevel.REPEATABLE_READ
        while True:
            with conn.transaction(), conn.cursor() as cursor:
                cursor.execute("""
                    SELECT a.attempt_id, a.status, a.raw_score, a.level, a.submitted_at, a.received_at,
                           a.scored_at, o.published_at, o.envelope::jsonb->'payload' AS risk
                    FROM attempts a LEFT JOIN risk_outbox o USING(attempt_id)
                    WHERE a.attempt_id = ANY(%s)
                """, (ids,))
                attempts = {row["attempt_id"]: row for row in cursor}
                finished = len(attempts) == len(ids) and all(
                    a["status"] != "pending" and (a["status"] != "flagged" or a["published_at"] is not None)
                    for a in attempts.values()
                )
                if finished or time.monotonic() >= deadline:
                    cursor.execute("""
                        SELECT attempt_id, envelope FROM quiz_events WHERE attempt_id = ANY(%s)
                        ORDER BY occurred_at, event_id
                    """, (ids,))
                    events: dict[str, list[Record]] = defaultdict(list)
                    for row in cursor:
                        events[row["attempt_id"]].append(row["envelope"])
                    return attempts, dict(events)
            time.sleep(min(0.5, max(0, deadline - time.monotonic())))


def compare_row(number: int, expected: Record, actual: Record, recovered: Record | None, attempt_id: str,
                artifact_version: str, checks: Differences) -> None:
    for field in ("status", "level", "submitted_at", "raw_score"):
        checks.check(number, field, expected[field], actual[field], numeric=field == "raw_score")
    checks.check(number, "scoring_wall_clock_order", True,
                 actual["scored_at"] is not None and actual["scored_at"] >= actual["received_at"])
    checks.check(number, "reconstructable", True, recovered is not None)
    if recovered is not None:
        for key in FEATURE_COLUMNS:
            checks.check(number, "reconstructed." + key, expected["features"][key], recovered["features"][key],
                         numeric=True)
        checks.check(number, "reconstructed.synchronous_with", expected["peers"], recovered["peers"])
    flagged = expected["status"] == "flagged"
    payload = actual["risk"]
    checks.check(number, "signal_present", flagged, payload is not None)
    if payload is None:
        return
    checks.check(number, "publication_wall_clock_order", True,
                 actual["published_at"] is not None and actual["scored_at"] is not None
                 and actual["published_at"] >= actual["scored_at"])
    checks.check(number, "signal.target_id", attempt_id, payload.get("target_id"))
    checks.check(number, "signal.target_type", "attempt", payload.get("target_type"))
    checks.check(number, "signal.model", {"name": "isolation-forest-v1", "version": artifact_version}, payload.get("model"))
    checks.check(number, "signal.score", expected["score"], payload.get("score"), numeric=True)
    checks.check(number, "signal.level", expected["level"], payload.get("level"))
    evidence = payload.get("evidence", {}).get("features", {})
    for key in ("mean_answer_time_ms", "z_score"):
        checks.check(number, "signal." + key + ".present", True, key in evidence)
        checks.check(number, "signal." + key, expected["features"][key], evidence.get(key), numeric=True)
    peers = evidence.get("synchronous_with")
    checks.check(number, "signal.synchronous_with", expected["peers"], sorted(peers) if isinstance(peers, list) else None)


def map_world(world: Path, manifest: Record) -> dict[str, list[Record]]:
    if sha256_of(world) != manifest["world_sha256"]:
        raise ValueError("source world checksum differs from the replay manifest")
    ids = [a["id"] for a in manifest["attempts"].values()]
    if not ids or len(set(ids)) != len(ids) or any(not value for value in ids):
        raise ValueError("expected a nonempty bijection of source and platform attempts")
    option_number = {hashlib.sha256(f'{{"option":{i}}}'.encode()).hexdigest(): i for i in range(4)}
    questions: dict[str, tuple[str, list[str]]] = {}
    for key, mapping in manifest["questions"].items():
        # The platform hashes the normalized option UUID, the simulator hashes {"option":N}.
        options = mapping["option_ids"]
        hashes = [hashlib.sha256(option.strip().lower().encode()).hexdigest() for option in options]
        if len(set(hashes)) != 4 or len(hashes) != 4 or hashes != mapping["answer_hashes"]:
            raise ValueError("invalid question option map")
        questions[key] = mapping["id"], hashes
    shift = timedelta(milliseconds=manifest["shift_ms"])
    expected: dict[str, list[Record]] = defaultdict(list)
    epoch = datetime(1970, 1, 1, tzinfo=UTC)
    seen = set()
    with world.open(encoding="utf-8", newline="") as stream:
        for row in csv.DictReader(stream):
            payload = json.loads(row["payload"])
            source = payload["attempt_id"]
            seen.add(source)
            attempt = manifest["attempts"][source]
            actor, context = json.loads(row["actor"]), json.loads(row["context"])
            actor["user_id"] = manifest["users"][actor["user_id"]]
            context["group_id"] = manifest["groups"][context["group_id"]]
            if (actor["user_id"], context["group_id"]) != (attempt["user_id"], attempt["group_id"]):
                raise ValueError("inconsistent actor or group mapping")
            payload["attempt_id"] = attempt["id"]
            if row["event_type"] == "quiz.submitted":
                payload["quiz_id"] = manifest["quizzes"][payload["quiz_id"]]
                payload["variant_id"] = manifest["variants"][payload["variant_id"]]
                if (payload["quiz_id"], payload["variant_id"]) != (attempt["quiz_id"], attempt["variant_id"]):
                    raise ValueError("inconsistent quiz or variant mapping")
            elif row["event_type"] == "quiz.answered":
                question_id, hashes = questions[payload["question_id"]]
                payload["question_id"] = question_id
                payload["answer_hash"] = hashes[option_number[payload["answer_hash"]]]
            else:
                raise ValueError("unsupported source world event")
            at = datetime.fromisoformat(row["occurred_at"]) + shift
            ms = (at - epoch) // timedelta(milliseconds=1)
            event_id = uuid.UUID(int=(ms << 80) | (uuid.UUID(row["event_id"]).int % (1 << 80)))
            expected[attempt["id"]].append({"event_id": str(event_id), "event_type": row["event_type"],
                                            "version": int(row["version"]), "occurred_at": at.isoformat(),
                                            "producer": row["producer"], "actor": actor, "context": context,
                                            "payload": payload})
    if seen != set(manifest["attempts"]):
        raise ValueError("world and replay attempt sets differ")
    return dict(expected)


def event_signature(event: Record) -> tuple:
    """Input parity on observable fields; course/session IDs and UUID entropy are outside the model."""
    p = event["payload"]
    kind = event["event_type"]
    fields = ("question_id", "question_index", "time_since_shown_ms", "time_since_attempt_start_ms", "is_change", "answer_hash"
              ) if kind == "quiz.answered" else ("quiz_id", "variant_id", "duration_ms", "answers_changed", "score", "max_score")
    return (datetime.fromisoformat(event["occurred_at"]), kind, event["actor"]["user_id"], event["context"]["group_id"],
            p["attempt_id"], *(p[field] for field in fields))


def signatures(events: Iterable[Record]) -> Counter[tuple]:
    return Counter(event_signature(e) for e in events if e["event_type"] in {"quiz.answered", "quiz.submitted"})


def projected_scores(expected_events: dict[str, list[Record]], artifact: Record) -> dict[str, Record]:
    # Feed independently projected source events straight to the frozen reader (not the runtime adapter).
    out = io.StringIO()
    columns = ("event_id", "event_type", "version", "occurred_at", "producer", "actor", "context", "payload")
    writer = csv.DictWriter(out, fieldnames=columns)
    writer.writeheader()
    all_events = (e for group in expected_events.values() for e in group)
    for event in sorted(all_events, key=lambda e: (e["occurred_at"], e["event_id"])):
        writer.writerow({k: json.dumps(v) if k in ("actor", "context", "payload") else v for k, v in event.items()})
    out.seek(0)
    return score_world(read_attempts(out), artifact)


def compare(world: Path, manifest: Record, artifact: Record, actual: dict[str, Record],
            events: dict[str, list[Record]], tolerance: float) -> Record:
    expected_events = map_world(world, manifest)
    projected = projected_scores(expected_events, artifact)
    native = score_world(read_attempts(world), artifact)
    reconstructed, excluded = reconstruct(events, artifact)
    platform = Differences(tolerance)
    namespace = Differences(tolerance)
    native_service = Differences(tolerance)
    rows = []
    for n, key in enumerate(sorted(native), 1):
        mapping = manifest["attempts"][key]
        live = mapping["id"]
        expected, original = projected[live], native[key]
        for feature in FEATURE_COLUMNS:
            namespace.check(n, feature, original["features"][feature], expected["features"][feature], numeric=True)
        for field in ("raw_score", "score", "status", "level"):
            namespace.check(n, field, original[field], expected[field], numeric=field in {"raw_score", "score"})
        platform.check(n, "replay_completed", "submitted", mapping["status"])
        platform.check(n, "persisted", True, live in actual)
        observed_events = events.get(live, [])
        platform.check(n, "saved_event_inputs", signatures(expected_events[live]), signatures(observed_events))
        expected_start = expected["submitted_at"] - timedelta(milliseconds=expected["features"]["duration_ms"])
        observed_start = [datetime.fromisoformat(e["occurred_at"]) for e in observed_events if e["event_type"] == "quiz.started"]
        platform.check(n, "started_at", [expected_start], observed_start)
        recovered = reconstructed.get(live)
        state = actual.get(live)
        native_service.check(n, "raw_score", original["raw_score"], state["raw_score"] if state else None, numeric=True)
        if state:
            compare_row(n, expected, state, recovered, live, str(artifact["version"]), platform)
        rows.append({"attempt_number": n, "expected_status": expected["status"],
                     "observed_status": state["status"] if state else "missing", "native_raw": original["raw_score"],
                     "expected_raw": expected["raw_score"], "observed_raw": state["raw_score"] if state else None,
                     "expected_submitted_at": expected["submitted_at"].isoformat(),
                     "observed_submitted_at": state["submitted_at"].isoformat() if state else None,
                     "expected_features": expected["features"], "reconstructed_features": recovered["features"] if recovered else None})
    return {"passed": not platform.failed and not native_service.failed, "platform_passed": not platform.failed,
            "native_score_parity": not native_service.failed, "native_vs_service": native_service.report(),
            "tolerance_absolute": tolerance, "attempts": len(native), "reconstruction_exclusions": len(excluded),
            "observed_status_counts": dict(Counter(a["status"] for a in actual.values())),
            "platform_parity": platform.report(), "native_to_platform_namespace": namespace.report(), "rows": rows}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--world", required=True, type=Path)
    parser.add_argument("--manifest", required=True, type=Path, help="replay client manifest for this same CSV")
    parser.add_argument("--model", required=True, type=Path, help="same trusted local artifact used by the scorer")
    parser.add_argument("--sha256", default=os.getenv("ANOMALY_MODEL_SHA256"), help="trusted model SHA-256 pin")
    parser.add_argument("--dsn-env", default="ANOMALY_DATABASE_URL", help="name of the research database DSN variable")
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--tolerance", type=float, default=1e-9)
    parser.add_argument("--wait-seconds", type=float, default=180)
    args = parser.parse_args()
    if not args.sha256 or len(args.sha256) != 64 or not os.getenv(args.dsn_env):
        parser.error("set the model SHA-256 pin and research database environment variable")
    if not math.isfinite(args.wait_seconds) or args.wait_seconds < 0:
        parser.error("--wait-seconds must be finite and nonnegative")
    try:
        artifact = load_verified(args.model, args.sha256)
        manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
        if manifest.get("mode") != "parity":
            raise ValueError("expected a parity-mode replay")
        actual, events = observe(os.environ[args.dsn_env], [m["id"] for m in manifest["attempts"].values()], args.wait_seconds)
        result = compare(args.world, manifest, artifact, actual, events, args.tolerance)
        result.update({"world_sha256": sha256_of(args.world), "manifest_sha256": sha256_of(args.manifest),
                       "artifact_sha256": args.sha256.lower(), "artifact_version": str(artifact["version"])})
        write_json(args.report, result)
    except (ValueError, KeyError, TypeError, OSError, RuntimeError, psycopg.Error) as exc:
        # Input-validation and SQL exception messages may contain complete rows or a database address.
        parser.exit(2, f"Parity inputs or read-only snapshot could not be verified ({type(exc).__name__}).\n")
    print(json.dumps({"passed": result["passed"], "platform_passed": result["platform_passed"], "native_score_parity": result["native_score_parity"],
                      "attempts": result["attempts"]}))
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
