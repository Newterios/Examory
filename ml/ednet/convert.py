# ml/ednet/convert.py
"""Convert EdNet evaluation users' Part-5 sittings into the Examory quiz event CSV.

A pseudo-attempt is a run of exactly 20 consecutive Part-5 first-exposure
responses of one learner inside one sitting (see ``common.find_windows``:
adjacent bundles, every submit -> next enter gap <= --max-gap-ms, no bundle
longer than --max-gap-ms). Windows are non-overlapping, cut from the start of
each sitting, and at most --max-attempts-per-user (earliest first) are kept.

Event mapping (docs/03_architecture/events.md):
* the first ``respond`` of a bundle -> ``quiz.answered`` with is_change=false;
* every later respond whose option differs from the immediately preceding
  respond -> ``quiz.answered`` with is_change=true (re-tapping the selected
  option is not a change and emits nothing);
* time_since_shown_ms = respond ts - that bundle's enter ts (cumulative);
  time_since_attempt_start_ms = respond ts - first enter ts of the window;
* ``quiz.submitted`` at the last bundle's submit ts; duration_ms = submit -
  window start; score = number of correct final answers; max_score = 20;
  answers_changed = number of change events.
answer_hash follows ``generate_quiz_behavior.answer_digest`` ({"option": N},
a->0 ... d->3); event_id is a deterministic UUIDv7 with the generator's bit
layout and the event's millisecond timestamp.

These are UNLABELLED REAL ATTEMPTS. EdNet contains no cheating labels; every row
carries is_anomaly=0 / anomaly_type=none only because the frozen detector
pipeline requires the columns. They must not be read as verified-honest.
Only evaluation users (hash split, see common.is_calibration_user) are used.

Example: .venv/bin/python -m ml.ednet.convert --users 20000 \
    --output ml/datasets/raw/ednet/derived/ednet_eval.csv
(--users is an upper bound: at the default --max-gap-ms only ~10.7k evaluation
users have a full window, and all of them are then used.)
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import time
import uuid
from collections import Counter
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import numpy as np

from ml.ednet.common import (
    DERIVED,
    SAMPLE_SALT,
    SOURCES,
    WINDOW_SIZE,
    change_events,
    file_sha256,
    find_windows,
    hash_array,
    load_table,
    subset,
    write_json,
)

COLUMNS = ("event_id", "event_type", "version", "occurred_at", "producer",
           "actor", "context", "payload", "is_anomaly", "anomaly_type")
EPOCH = datetime(1970, 1, 1, tzinfo=UTC)
SEED = "ednet-kt2-part5"
QUIZ_ID, VARIANT_ID, COURSE_ID = "ednet_part5", "variant_001", "ednet"
COHORT_SIZE = 20


def identifier(*parts: Any) -> str:
    return str(uuid.uuid5(uuid.NAMESPACE_URL, f"examory:{SEED}:" + ":".join(map(str, parts))))


def event_identifier(timestamp_ms: int, *parts: Any) -> str:
    """Deterministic UUIDv7; same bit layout as generate_quiz_behavior.event_identifier."""
    material = json.dumps([SEED, *parts], separators=(",", ":")).encode("utf-8")
    random_bits = int.from_bytes(hashlib.sha256(material).digest(), "big") & ((1 << 74) - 1)
    value = ((int(timestamp_ms) << 80) | (7 << 76) | ((random_bits >> 62) << 64)
             | (2 << 62) | (random_bits & ((1 << 62) - 1)))
    return str(uuid.UUID(int=value))


def answer_digest(option: int) -> str:
    normalized = json.dumps({"option": int(option)}, sort_keys=True, separators=(",", ":"),
                            ensure_ascii=False)
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


def iso_ms(timestamp_ms: int) -> str:
    moment = EPOCH + timedelta(milliseconds=int(timestamp_ms))
    return moment.isoformat(timespec="milliseconds").replace("+00:00", "Z")


def window_problem(table: dict[str, np.ndarray], rows: np.ndarray) -> str | None:
    """Reason why a window cannot satisfy ml.anomaly.features.read_attempts, else None."""
    if len(set(table["question"][rows].tolist())) != len(rows):
        return "duplicate_question"
    if table["submit_ts"][rows[-1]] - table["enter_ts"][rows[0]] <= 0:
        return "non_positive_duration"
    for row in rows:
        times = [ts for ts, _ in change_events(table, int(row))]
        # Equal timestamps would make the initial/change order depend on event_id.
        if len(times) != len(set(times)):
            return "tied_answer_timestamps"
    return None


def window_events(table: dict[str, np.ndarray], rows: np.ndarray, cohort: str) -> list[dict[str, Any]]:
    user = int(table["user"][rows[0]])
    start = int(table["enter_ts"][rows[0]])
    anchor = int(table["ordinal"][rows[0]])
    attempt_id = identifier("attempt", user, anchor)
    actor = {"user_id": str(user), "role": "student"}
    context = {"course_id": COURSE_ID, "group_id": cohort,
               "session_id": identifier("session", user, anchor)}

    def row(ts: int, event_type: str, payload: dict[str, Any], suffix: str) -> dict[str, Any]:
        return {"event_id": event_identifier(ts, user, anchor, suffix), "event_type": event_type,
                "version": 1, "occurred_at": iso_ms(ts), "producer": "quizzes", "actor": actor,
                "context": context, "payload": payload, "is_anomaly": 0, "anomaly_type": "none"}

    events, changes = [], 0
    for index, r in enumerate(rows):
        enter, question = int(table["enter_ts"][r]), f"q{int(table['question'][r])}"
        for k, (ts, option) in enumerate(change_events(table, int(r))):
            changes += k > 0
            events.append(row(ts, "quiz.answered", {
                "attempt_id": attempt_id, "question_id": question, "question_index": index,
                "answer_hash": answer_digest(option), "is_change": k > 0,
                "time_since_shown_ms": ts - enter, "time_since_attempt_start_ms": ts - start,
            }, f"answer:{index}:{k}"))
    submit = int(table["submit_ts"][rows[-1]])
    events.append(row(submit, "quiz.submitted", {
        "attempt_id": attempt_id, "quiz_id": QUIZ_ID, "variant_id": VARIANT_ID,
        "score": int(table["final_correct"][rows].sum()), "max_score": len(rows),
        "duration_ms": submit - start, "answers_changed": changes,
    }, "submitted"))
    return events


def select_attempts(table: dict[str, np.ndarray], users: int, max_attempts: int,
                    max_gap_ms: int) -> tuple[list[tuple[int, list[np.ndarray]]], dict[str, Any]]:
    windows = find_windows(table, max_gap_ms, WINDOW_SIZE)
    drops: Counter = Counter()
    per_user: dict[int, list[np.ndarray]] = {}
    for rows, user in zip(windows["rows"], windows["user"], strict=True):
        problem = window_problem(table, rows)
        if problem:
            drops[problem] += 1
            continue
        per_user.setdefault(int(user), []).append(rows)
    eligible = np.array(sorted(per_user), dtype=np.int64)
    rank = hash_array(SAMPLE_SALT, eligible) if len(eligible) else np.zeros(0)
    order = eligible[np.lexsort((eligible, rank))]
    chosen = order[:users]
    available = Counter(min(len(per_user[int(u)]), 10) for u in chosen)
    selected = [(int(u), per_user[int(u)][:max_attempts]) for u in chosen]
    stats = {
        "evaluation_users_with_part5": len(np.unique(table["user"])),
        "windows_found": len(windows["rows"]),
        "windows_dropped": dict(sorted(drops.items())),
        "eligible_users": len(eligible),
        "sampled_users": len(chosen),
        "windows_per_sampled_user_before_cap": {("10+" if k == 10 else str(k)): v
                                                for k, v in sorted(available.items())},
    }
    return selected, stats


def convert(table_path: str | Path, output: str | Path, users: int = 20000, max_attempts: int = 5,
            max_gap_ms: int = 300_000) -> dict[str, Any]:
    started = time.time()
    table = load_table(table_path)
    if not table["is_calibration"].any() and not (~table["is_calibration"]).any():
        raise ValueError("Empty response table")
    table = subset(table, ~table["is_calibration"])
    selected, stats = select_attempts(table, users, max_attempts, max_gap_ms)
    events: list[dict[str, Any]] = []
    attempts_per_user: Counter = Counter()
    source_counts: Counter = Counter()
    for position, (_, windows) in enumerate(selected):
        cohort = f"group_{position // COHORT_SIZE + 1:05d}"
        attempts_per_user[len(windows)] += 1
        for rows in windows:
            events.extend(window_events(table, rows, cohort))
            source_counts.update(np.asarray(SOURCES)[table["source"][rows]].tolist())
    events.sort(key=lambda e: (e["occurred_at"], e["event_id"]))
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=COLUMNS)
        writer.writeheader()
        for event in events:
            writer.writerow({k: json.dumps(v, separators=(",", ":")) if isinstance(v, dict) else v
                             for k, v in event.items()})
    submitted = [e for e in events if e["event_type"] == "quiz.submitted"]
    extract_meta = Path(table_path).with_suffix(".meta.json")
    inputs = {"response_table": str(table_path), "response_table_sha256": file_sha256(table_path)}
    if extract_meta.exists():
        inputs.update({k: v for k, v in json.loads(extract_meta.read_text())["inputs"].items()})
    meta = {
        "description": "Unlabelled real EdNet-KT2 Part-5 pseudo-attempts (evaluation users); "
                       "is_anomaly=0 is a placeholder, not a verified label.",
        "parameters": {"users": users, "max_attempts_per_user": max_attempts, "max_gap_ms": max_gap_ms,
                       "window_size": WINDOW_SIZE, "cohort_size": COHORT_SIZE, "quiz_id": QUIZ_ID,
                       "variant_id": VARIANT_ID, "seed": SEED},
        "inputs": inputs, **stats,
        "attempts": len(submitted), "events": len(events),
        "answer_events": len(events) - len(submitted),
        "change_events": sum(e["payload"]["answers_changed"] for e in submitted),
        "cohorts": -(-len(selected) // COHORT_SIZE),
        "attempts_per_user": {str(k): v for k, v in sorted(attempts_per_user.items())},
        "responses_by_source": dict(sorted(source_counts.items())),
        "mean_score": round(float(np.mean([e["payload"]["score"] for e in submitted])), 3) if submitted else None,
        "median_duration_ms": float(np.median([e["payload"]["duration_ms"] for e in submitted])) if submitted else None,
        "output": str(output), "output_sha256": file_sha256(output),
        "seconds": round(time.time() - started, 1),
    }
    write_json(Path(str(output) + ".meta.json"), meta)
    return meta


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--table", type=Path, default=DERIVED)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--users", type=int, default=20000)
    parser.add_argument("--max-attempts-per-user", type=int, default=5)
    parser.add_argument("--max-gap-ms", type=int, default=300_000)
    args = parser.parse_args()
    meta = convert(args.table, args.output, args.users, args.max_attempts_per_user, args.max_gap_ms)
    print(json.dumps({k: meta[k] for k in ("eligible_users", "sampled_users", "attempts", "events",
                                            "windows_dropped", "seconds")}))


if __name__ == "__main__":
    main()
