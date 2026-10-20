# ml/datasets/generate_quiz_behavior.py
"""Generate synthetic Examory quiz events; labels describe injected attempts.

Only the final attempt can be anomalous. Nested envelope values are JSON CSV
cells following docs/03_architecture/events.md. question_index is zero-based;
score counts correct answers, and answers_changed counts change events. A
session identifies an exam context and is distinct from payload.attempt_id.
Python 3.12 or newer; no third-party dependencies.
Example: python ml/datasets/generate_quiz_behavior.py --output out.csv
"""

import argparse
import csv
import hashlib
import json
import math
import random
import sys
import uuid
from collections import Counter
from datetime import UTC, datetime, timedelta
from pathlib import Path

TYPES = ("rapid_response", "excessive_changes", "collusion", "score_jump")
COLUMNS = (
    "event_id", "event_type", "version", "occurred_at", "producer",
    "actor", "context", "payload", "is_anomaly", "anomaly_type",
)
PROFILES = ((18000, 0.65, 0.08), (38000, 0.65, 0.25), (28000, 0.65, 0.15))
DEFAULT_WEIGHTS = "rapid_response=.35,excessive_changes=.25,collusion=.25,score_jump=.15"
EPOCH = datetime(1970, 1, 1, tzinfo=UTC)


def weights_arg(value):
    try:
        pairs = [item.split("=") for item in value.split(",")]
        result = {name: float(weight) for name, weight in pairs}
        if len(pairs) != len(TYPES) or set(result) != set(TYPES):
            raise ValueError
        if any(not math.isfinite(x) or x < 0 for x in result.values()):
            raise ValueError
        if not math.isfinite(sum(result.values())) or sum(result.values()) <= 0:
            raise ValueError
        return result
    except (ValueError, TypeError):
        raise argparse.ArgumentTypeError(
            "provide all four distinct name=nonnegative-weight entries; "
            "at least one must be positive"
        ) from None


def utc_arg(value):
    try:
        result = datetime.fromisoformat(value)
        if result.tzinfo is None:
            raise ValueError
        result = result.astimezone(UTC)
        if result < EPOCH:
            raise ValueError
        return result.replace(microsecond=(result.microsecond // 1000) * 1000)
    except ValueError:
        raise argparse.ArgumentTypeError(
            "use an ISO 8601 datetime with timezone, on or after 1970-01-01 UTC"
        ) from None


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--students", type=int, default=300)
    parser.add_argument("--attempts", type=int, default=3)
    parser.add_argument("--questions", type=int, default=20)
    parser.add_argument("--anomaly-rate", type=float, default=0.12,
                        help="fraction of ALL attempts; maximum 1/attempts")
    parser.add_argument("--anomaly-weights", type=weights_arg, default=DEFAULT_WEIGHTS,
                        help="relative weights: " + DEFAULT_WEIGHTS)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--start-at", type=utc_arg, default="2026-10-01T09:00:00Z")
    parser.add_argument("--output", type=Path, default=Path("quiz_behavior.csv"))
    args = parser.parse_args(argv)
    if min(args.students, args.attempts, args.questions) < 1:
        parser.error("students, attempts and questions must be positive")
    if not math.isfinite(args.anomaly_rate) or not 0 <= args.anomaly_rate <= 1 / args.attempts:
        parser.error("anomaly-rate must lie in [0, 1/attempts]")
    if args.attempts < 2 and args.anomaly_rate > 0 and args.anomaly_weights["score_jump"] > 0:
        parser.error("score_jump requires at least two attempts; set its weight to zero")
    return args


def identifier(seed, *parts):
    return str(uuid.uuid5(uuid.NAMESPACE_URL, f"examory:{seed}:" + ":".join(map(str, parts))))


def event_identifier(seed, occurred_at, *parts):
    """Deterministic UUID v7 with the event's Unix-millisecond timestamp."""
    delta = occurred_at - EPOCH
    timestamp_ms = delta.days * 86400000 + delta.seconds * 1000 + delta.microseconds // 1000
    material = json.dumps([seed, *parts], separators=(",", ":")).encode("utf-8")
    random_bits = int.from_bytes(hashlib.sha256(material).digest(), "big") & ((1 << 74) - 1)
    value = (
        (timestamp_ms << 80)
        | (7 << 76)
        | ((random_bits >> 62) << 64)
        | (2 << 62)
        | (random_bits & ((1 << 62) - 1))
    )
    return str(uuid.UUID(int=value))


def answer_digest(option):
    """Hash only the canonical JSON answer, independent of user and question."""
    normalized = json.dumps(
        {"option": option}, sort_keys=True, separators=(",", ":"), ensure_ascii=False
    )
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


def make_labels(args, rng):
    """Pair within fixed cohorts of 20; return final-attempt labels and partners."""
    requested = int(args.students * args.attempts * args.anomaly_rate + 0.5)
    total_weight = sum(args.anomaly_weights.values())
    quotas = {k: requested * (args.anomaly_weights[k] / total_weight) for k in TYPES}
    counts = {k: int(quotas[k]) for k in TYPES}
    order = sorted(TYPES, key=lambda k: quotas[k] - counts[k], reverse=True)
    for kind in order[:requested - sum(counts.values())]:
        counts[kind] += 1
    if counts["collusion"] % 2:
        counts["collusion"] -= 1
        alternatives = [k for k in TYPES if k != "collusion" and args.anomaly_weights[k] > 0]
        if alternatives:
            counts[max(alternatives, key=lambda k: args.anomaly_weights[k])] += 1
    candidates = []
    for base in range(0, args.students, 20):
        members = list(range(base, min(base + 20, args.students)))
        rng.shuffle(members)
        candidates.extend(zip(members[::2], members[1::2]))
    rng.shuffle(candidates)
    labels, partners = {}, {}
    for first, second in candidates[:counts["collusion"] // 2]:
        labels[first] = labels[second] = "collusion"
        partners[first] = partners[second] = min(first, second)
    remaining = [student for student in range(args.students) if student not in labels]
    rng.shuffle(remaining)
    for kind in TYPES:
        if kind != "collusion":
            for _ in range(counts[kind]):
                labels[remaining.pop()] = kind
    return labels, partners


def generate(args):
    rng = random.Random(args.seed)
    labels, partners = make_labels(args, rng)
    profiles = [rng.choices(PROFILES, weights=(0.25, 0.35, 0.40))[0] for _ in range(args.students)]
    speeds = [rng.lognormvariate(0, 0.15) for _ in range(args.students)]
    abilities = [min(0.93, max(0.25, rng.betavariate(7, 3))) for _ in range(args.students)]
    keys = [rng.randrange(4) for _ in range(args.questions)]
    difficulties = [rng.uniform(-0.15, 0.15) for _ in range(args.questions)]
    rows = []
    shared = {}
    for leader in sorted(set(partners.values())):
        forced_wrong = set(rng.sample(range(args.questions), max(1, round(args.questions * 0.30))))
        options, durations, gaps = [], [], []
        for q, correct in enumerate(keys):
            wrong = [option for option in range(4) if option != correct]
            options.append(wrong[-1] if q in forced_wrong else
                           (correct if rng.random() < 0.85 else rng.choice(wrong)))
            durations.append(max(1000, round(rng.lognormvariate(math.log(28000), 0.65))))
            gaps.append(rng.randint(600, 2500))
        shared[leader] = (forced_wrong, options, durations, gaps,
                          rng.randint(0, 600000), rng.randint(8000, 45000))

    def emit(student, attempt, kind, elapsed, event_type, payload, suffix):
        session = identifier(args.seed, "exam_session", student, attempt)
        occurred_at = start + timedelta(milliseconds=elapsed)
        rows.append({
            "event_id": event_identifier(args.seed, occurred_at, student, attempt, suffix),
            "event_type": event_type, "version": 1,
            "occurred_at": occurred_at.isoformat(
                timespec="milliseconds").replace("+00:00", "Z"),
            "producer": "quizzes",
            "actor": {"user_id": f"student_{student + 1:05d}", "role": "student"},
            "context": {"course_id": "course_001", "group_id": f"group_{student // 20 + 1:03d}",
                        "session_id": session},
            "payload": payload, "is_anomaly": int(kind != "none"), "anomaly_type": kind,
        })

    for attempt in range(args.attempts):
        quiz = "quiz_001"
        variant = "variant_001"
        for student in range(args.students):
            attempt_id = identifier(args.seed, "attempt", student, attempt)
            kind = labels.get(student, "none") if attempt == args.attempts - 1 else "none"
            median, sigma, change_probability = profiles[student]
            median *= speeds[student]
            pair = shared[partners[student]] if kind == "collusion" else None
            offset = pair[4] + rng.randint(0, 8000) if pair else rng.randint(0, 600000)
            start = args.start_at + timedelta(days=attempt, milliseconds=offset)
            elapsed, changes, correct_count = 0, 0, 0
            ability = abilities[student] + 0.015 * attempt
            if kind == "score_jump":
                ability += rng.uniform(0.18, 0.32)
            for q, correct in enumerate(keys):
                question = f"question_{q + 1:03d}"
                wrong = [option for option in range(4) if option != correct]
                probability = min(0.98, max(0.05, ability - difficulties[q]))
                option = correct if rng.random() < probability else rng.choices(wrong, (0.7, 0.2, 0.1))[0]
                duration = max(1000, round(rng.lognormvariate(math.log(median), sigma)))
                gap = rng.randint(600, 2500)
                if kind == "rapid_response" and rng.random() < 0.65:
                    duration = max(500, round(duration * rng.uniform(0.18, 0.55)))
                if pair:
                    if q in pair[0] or rng.random() < 0.90:
                        option = pair[1][q]
                    duration = max(1000, round(pair[2][q] * rng.uniform(0.97, 1.03)))
                    gap = pair[3][q]
                p_change = rng.uniform(0.45, 0.75) if kind == "excessive_changes" else change_probability
                count = rng.randint(1, 4 if kind == "excessive_changes" else 2) if rng.random() < p_change else 0
                changes += count
                correct_count += int(option == correct)
                sequence = [option]
                for _ in range(count):
                    sequence.append(rng.choice([x for x in range(4) if x != sequence[-1]]))
                sequence.reverse()
                for revision, answer in enumerate(sequence):
                    since_shown = duration * (revision + 1) // len(sequence)
                    answer_hash = answer_digest(answer)
                    emit(student, attempt, kind, elapsed + since_shown, "quiz.answered", {
                        "attempt_id": attempt_id, "question_id": question, "question_index": q,
                        "time_since_shown_ms": since_shown, "is_change": revision > 0,
                        "time_since_attempt_start_ms": elapsed + since_shown,
                        "answer_hash": answer_hash,
                    }, f"answer:{q}:{revision}")
                elapsed += duration + gap
            if pair:
                target = sum(math.ceil(d * 1.03) for d in pair[2]) + sum(pair[3]) + 8000 + pair[5]
                elapsed = target - (offset - pair[4]) + rng.randint(0, 1000)
            else:
                elapsed += max(1000, round(rng.lognormvariate(math.log(20000), 0.8)))
            emit(student, attempt, kind, elapsed, "quiz.submitted", {
                "attempt_id": attempt_id, "quiz_id": quiz, "variant_id": variant,
                "score": correct_count, "max_score": args.questions,
                "duration_ms": elapsed, "answers_changed": changes,
            }, "submitted")
    rows.sort(key=lambda row: (row["occurred_at"], row["event_id"]))
    return rows


def main(argv=None):
    args = parse_args(argv)
    rows = generate(args)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=COLUMNS)
        writer.writeheader()
        for row in rows:
            writer.writerow({key: json.dumps(value, separators=(",", ":")) if isinstance(value, dict)
                             else value for key, value in row.items()})
    counts = Counter(row["anomaly_type"] for row in rows if row["event_type"] == "quiz.submitted")
    print(json.dumps({"output": str(args.output), "events": len(rows),
                      "attempts_by_type": dict(sorted(counts.items()))}), file=sys.stderr)


if __name__ == "__main__":
    main()
