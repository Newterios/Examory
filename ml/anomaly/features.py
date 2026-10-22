# ml/anomaly/features.py
"""Attempt features. Labels and identifiers never enter FEATURE_COLUMNS.

CSV actor/context/payload cells are JSON objects. Timing references and answer
frequencies must be fitted on normal training cohorts, never on the full CSV.
Peer features require a complete event watermark five seconds after submission.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import uuid
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

BASE_FEATURES = [
    "mean_answer_time_ms", "median_answer_time_ms", "p10_answer_time_ms",
    "answer_time_variance_ms2", "fast_answer_fraction", "final_answer_time_ms",
    "duration_ms", "changes_count", "changes_fraction", "score_percent",
    "z_score", "history_missing",
]
PEER_FEATURES = ["synchronous_count", "max_answer_agreement", "rarity_agreement"]
FEATURE_COLUMNS = BASE_FEATURES + PEER_FEATURES
LOG_FEATURES = [
    "mean_answer_time_ms", "median_answer_time_ms", "p10_answer_time_ms",
    "answer_time_variance_ms2", "final_answer_time_ms", "duration_ms",
    "changes_count", "synchronous_count", "rarity_agreement",
]
ANSWER_KEYS = {
    "attempt_id", "question_id", "question_index", "answer_hash", "is_change",
    "time_since_shown_ms", "time_since_attempt_start_ms",
}
SUBMIT_KEYS = {
    "attempt_id", "quiz_id", "variant_id", "score", "max_score",
    "duration_ms", "answers_changed",
}


def file_sha256(path: str | Path) -> str:
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def write_json(path: str | Path, value: Any) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False, allow_nan=False),
                    encoding="utf-8")


def read_attempts(path: str | Path) -> list[dict[str, Any]]:
    """Validate generator semantics, deduplicate, and join by payload.attempt_id."""
    frame = pd.read_csv(path, dtype=str, keep_default_na=False)
    required = {"event_id", "event_type", "version", "occurred_at", "producer",
                "actor", "context", "payload"}
    if not required <= set(frame):
        raise ValueError(f"Missing CSV columns: {required - set(frame)}")
    # Identical re-deliveries are harmless; conflicting IDs are invalid input.
    unique = frame.drop_duplicates()
    if unique.event_id.duplicated().any():
        raise ValueError("Conflicting rows share event_id")
    grouped: dict[str, list[dict[str, Any]]] = defaultdict(list)
    previous_time = None
    for row in unique.to_dict("records"):
        if row["event_type"] not in {"quiz.answered", "quiz.submitted"}:
            continue
        for field in ("actor", "context", "payload"):
            row[field] = json.loads(row[field])
            if not isinstance(row[field], dict):
                raise TypeError(f"{field} must be a JSON object")
        timestamp = pd.Timestamp(row["occurred_at"])
        event_id = uuid.UUID(row["event_id"])
        if timestamp.tzinfo is None or timestamp.utcoffset().total_seconds() != 0:
            raise ValueError("occurred_at must be UTC")
        if event_id.version != 7 or event_id.int >> 80 != timestamp.value // 1_000_000:
            raise ValueError("UUIDv7 timestamp differs from occurred_at")
        if previous_time is not None and timestamp < previous_time:
            raise ValueError("CSV must be chronological after deduplication")
        previous_time = timestamp
        row["timestamp"] = timestamp
        payload = row["payload"]
        expected = ANSWER_KEYS if row["event_type"] == "quiz.answered" else SUBMIT_KEYS
        if set(payload) != expected or row["version"] != "1":
            raise ValueError("Unsupported quiz payload schema/version")
        if not row["context"].get("group_id") or not row["actor"].get("user_id"):
            raise ValueError("group_id and user_id are required for cohort evaluation")
        if row.get("is_anomaly", "") not in {"", "0", "1"}:
            raise ValueError("is_anomaly must be 0, 1 or absent")
        grouped[payload["attempt_id"]].append(row)

    attempts = []
    for attempt_id, events in grouped.items():
        submissions = [e for e in events if e["event_type"] == "quiz.submitted"]
        answers = [e for e in events if e["event_type"] == "quiz.answered"]
        if len(submissions) != 1 or not answers:
            raise ValueError(f"{attempt_id}: need one submission and answered questions")
        submission = submissions[0]
        payload = submission["payload"]
        for event in events:
            if (event["actor"] != submission["actor"]
                    or event["context"] != submission["context"]
                    or event.get("is_anomaly") != submission.get("is_anomaly")
                    or event.get("anomaly_type") != submission.get("anomaly_type")):
                raise ValueError(f"{attempt_id}: inconsistent identity/context/labels")
        if not (0 <= payload["score"] <= payload["max_score"] and payload["max_score"] > 0):
            raise ValueError("Invalid score/max_score")
        if payload["duration_ms"] <= 0:
            raise ValueError("duration_ms must be positive")
        answers.sort(key=lambda e: (e["payload"]["time_since_attempt_start_ms"], e["timestamp"]))
        per_question: dict[str, list[dict[str, Any]]] = defaultdict(list)
        for event in answers:
            answer = event["payload"]
            elapsed, shown = answer["time_since_attempt_start_ms"], answer["time_since_shown_ms"]
            if not (0 <= shown <= elapsed <= payload["duration_ms"]):
                raise ValueError("Invalid answer timing")
            expected_time = submission["timestamp"] - pd.Timedelta(
                milliseconds=payload["duration_ms"] - elapsed)
            if abs((event["timestamp"] - expected_time).total_seconds()) > .001:
                raise ValueError("Answer timestamp disagrees with elapsed time")
            if (not isinstance(answer["is_change"], bool)
                    or not isinstance(answer["question_index"], int)
                    or answer["question_index"] < 0):
                raise ValueError("Invalid is_change/question_index")
            digest = answer["answer_hash"]
            if len(digest) != 64 or any(c not in "0123456789abcdef" for c in digest):
                raise ValueError("answer_hash must be lowercase SHA-256")
            per_question[answer["question_id"]].append(answer)
        first_times, final_hashes, final_times, indices = {}, {}, {}, []
        for question_id, sequence in per_question.items():
            if sequence[0]["is_change"] or any(not a["is_change"] for a in sequence[1:]):
                raise ValueError("Each question requires one initial answer, then revisions")
            indices.append(sequence[0]["question_index"])
            if any(a["question_index"] != indices[-1] for a in sequence):
                raise ValueError("Question index changes within attempt")
            times = [a["time_since_shown_ms"] for a in sequence]
            if times != sorted(times):
                raise ValueError("Cumulative question time decreases")
            first_times[question_id] = times[0]
            final_times[question_id] = times[-1]
            final_hashes[question_id] = sequence[-1]["answer_hash"]
        if sorted(indices) != list(range(len(indices))) or len(indices) != payload["max_score"]:
            raise ValueError("Generator question indices/max_score disagree")
        changes = sum(a["payload"]["is_change"] for a in answers)
        if changes != payload["answers_changed"]:
            raise ValueError("answers_changed differs from revision count")
        label = submission.get("is_anomaly", "")
        anomaly_type = submission.get("anomaly_type", "")
        if label and ((label == "0") != (anomaly_type == "none")):
            raise ValueError("Inconsistent synthetic label/type")
        attempts.append({
            "attempt_id": attempt_id, "user_id": submission["actor"]["user_id"],
            "group_id": submission["context"]["group_id"], "context": submission["context"],
            "quiz_id": payload["quiz_id"], "variant_id": payload["variant_id"],
            "submitted_at": submission["timestamp"], "duration_ms": payload["duration_ms"],
            "changes_count": changes, "changes_fraction": changes / len(answers),
            "score_percent": 100 * payload["score"] / payload["max_score"],
            "first_times": first_times, "final_hashes": final_hashes,
            "final_answer_time_ms": final_times[answers[-1]["payload"]["question_id"]],
            "is_anomaly": int(label) if label else None, "anomaly_type": anomaly_type,
        })
    if not attempts:
        raise ValueError("No complete quiz attempts")
    attempts.sort(key=lambda a: (a["submitted_at"], a["attempt_id"]))
    history: dict[tuple, list[dict[str, Any]]] = defaultdict(list)
    for attempt in attempts:
        key = (attempt["user_id"], attempt["quiz_id"], attempt["variant_id"])
        earlier = [a["score_percent"] for a in history[key]
                   if a["submitted_at"] < attempt["submitted_at"]]
        attempt["history_missing"] = int(not earlier)
        attempt["score_delta"] = attempt["score_percent"] - float(np.mean(earlier)) if earlier else None
        attempt["history_sd"] = float(np.std(earlier)) if earlier else None
        history[key].append(attempt)
    for sequence in history.values():
        latest = max(a["submitted_at"] for a in sequence)
        for attempt in sequence:
            attempt["is_final_attempt"] = attempt["submitted_at"] == latest
    return attempts


def item_key(attempt: dict[str, Any], question: str) -> str:
    return json.dumps([attempt["quiz_id"], attempt["variant_id"], question])


def fit_reference(attempts: list[dict[str, Any]]) -> dict[str, Any]:
    """Caller passes only the selected normal training attempts."""
    if not attempts:
        raise ValueError("No normal training attempts")
    timings: dict[str, list[float]] = defaultdict(list)
    counts: dict[str, Counter] = defaultdict(Counter)
    for attempt in attempts:
        for question, value in attempt["first_times"].items():
            timings[item_key(attempt, question)].append(value)
            counts[item_key(attempt, question)][attempt["final_hashes"][question]] += 1
    all_times = [v for values in timings.values() for v in values]
    deltas = [a["score_delta"] for a in attempts if a["score_delta"] is not None]
    return {
        "global_fast_ms": float(np.quantile(all_times, .1)),
        "item_fast_ms": {k: float(np.quantile(v, .1)) for k, v in timings.items() if len(v) >= 30},
        "answer_counts": {k: dict(v) for k, v in counts.items()},
        "delta_sd_floor": max(5.0, float(np.std(deltas)) if deltas else 5.0),
        "min_item_observations": 30, "peer_window_seconds": 5,
    }


def build_features(attempts: list[dict[str, Any]], reference: dict[str, Any]) -> pd.DataFrame:
    records = []
    peers: dict[tuple, list[int]] = defaultdict(list)
    for i, attempt in enumerate(attempts):
        values = np.asarray(list(attempt["first_times"].values()), dtype=float)
        thresholds = [reference["item_fast_ms"].get(item_key(attempt, q), reference["global_fast_ms"])
                      for q in attempt["first_times"]]
        record = {k: attempt[k] for k in (
            "attempt_id", "user_id", "group_id", "context", "quiz_id", "variant_id",
            "submitted_at", "duration_ms", "changes_count", "changes_fraction",
            "score_percent", "history_missing", "is_anomaly", "anomaly_type",
            "is_final_attempt", "final_answer_time_ms",
        )}
        record.update({
            "mean_answer_time_ms": float(values.mean()),
            "median_answer_time_ms": float(np.median(values)),
            "p10_answer_time_ms": float(np.quantile(values, .1)),
            "answer_time_variance_ms2": float(values.var()),
            "fast_answer_fraction": float(np.mean(values < thresholds)),
            "z_score": (attempt["score_delta"] / max(reference["delta_sd_floor"], attempt["history_sd"])
                        if not attempt["history_missing"] else np.nan),
            "synchronous_with": [], "synchronous_count": 0,
            "max_answer_agreement": 0.0, "rarity_agreement": 0.0,
        })
        records.append(record)
        peers[(attempt["group_id"], attempt["quiz_id"], attempt["variant_id"])].append(i)
    for indices in peers.values():
        indices.sort(key=lambda i: attempts[i]["submitted_at"])
        for offset, i in enumerate(indices):
            left = attempts[i]
            for j in indices[offset + 1:]:
                right = attempts[j]
                if (right["submitted_at"] - left["submitted_at"]).total_seconds() > 5:
                    break
                if left["user_id"] == right["user_id"]:
                    continue
                common = set(left["final_hashes"]) & set(right["final_hashes"])
                equal = [q for q in common if left["final_hashes"][q] == right["final_hashes"][q]]
                agreement = len(equal) / len(common) if common else 0.0
                rarity = 0.0
                for question in equal:
                    counts = reference["answer_counts"].get(item_key(left, question), {})
                    probability = (counts.get(left["final_hashes"][question], 0) + 1) / (sum(counts.values()) + 4)
                    rarity += -math.log(probability) / len(common)
                for source, other in ((i, j), (j, i)):
                    records[source]["synchronous_with"].append(attempts[other]["attempt_id"])
                    records[source]["synchronous_count"] += 1
                    records[source]["max_answer_agreement"] = max(records[source]["max_answer_agreement"], agreement)
                    records[source]["rarity_agreement"] = max(records[source]["rarity_agreement"], rarity)
    return pd.DataFrame(records)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--reference-output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    if manifest["csv_sha256"] != file_sha256(args.csv):
        raise ValueError("CSV differs from split manifest")
    attempts = read_attempts(args.csv)
    reference = fit_reference([a for a in attempts if a["group_id"] in manifest["groups"]["train"]
                               and a["is_anomaly"] == 0])
    frame = build_features(attempts, reference)
    for column in ("context", "synchronous_with"):
        frame[column] = frame[column].map(json.dumps)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    frame.to_csv(args.output, index=False)
    write_json(args.reference_output, reference)


if __name__ == "__main__":
    main()
