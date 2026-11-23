"""Convert school-side extracts of finished Moodle quizzes to de-identified v1 events."""

from __future__ import annotations

import argparse
import csv
import hashlib
import hmac
import json
import os
import tempfile
import uuid
from collections import Counter, defaultdict
from datetime import UTC, datetime, timedelta
from decimal import Decimal
from pathlib import Path

ATTEMPT = {
    "attempt_id",
    "user_id",
    "course_id",
    "group_id",
    "quiz_id",
    "variant_id",
    "start",
    "finish",
    "score",
    "max_score",
    "question_count",
    "time_limit_sec",
    "reason",
}
STEP = {"attempt_id", "question_id", "slot", "sequence", "shown", "saved", "options"}


def csv_rows(path: Path, fields: set[str]):
    with path.open(encoding="utf-8-sig", newline="") as f:
        reader = csv.DictReader(f)
        if not fields.issubset(reader.fieldnames or []):
            raise ValueError("Missing required extract columns")
        for row in reader:
            yield {key: row[key] for key in fields}


def millis(s: str) -> int:
    """Moodle Unix seconds (fractional allowed) or an ISO timestamp with explicit offset."""
    if s.replace(".", "", 1).isdigit():
        return int(Decimal(s) * 1000)
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        raise ValueError("Timezone required")
    return int((dt - datetime(1970, 1, 1, tzinfo=UTC)) // timedelta(milliseconds=1))


class Pseudonyms:
    def __init__(self, key: bytes, namespace: str):
        if len(key) < 32 or not namespace:
            raise ValueError("Use at least 32 random key bytes and a study namespace")
        self.key, self.namespace = key, namespace
        # One study-wide shift preserves event ordering and peer windows.
        self.shift = (
            1 + int.from_bytes(self.mac("shift", "days")[:4], "big") % 365
        ) * 86400000

    def mac(self, kind: str, value: str) -> bytes:
        msg = json.dumps([self.namespace, kind, value], ensure_ascii=False).encode()
        return hmac.digest(self.key, msg, "sha256")

    def id(self, kind: str, value: str) -> str:
        if not value:
            raise ValueError("Required source identifier missing")
        return self.mac(kind, value)[:16].hex()

    def event(
        self, kind: str, t: int, actor: dict, context: dict, p: dict, seq: int
    ) -> dict:
        shifted = t + self.shift
        bits = int.from_bytes(
            self.mac("event", f"{p['attempt_id']}/{kind}/{seq}")[:10], "big"
        )
        value = (shifted << 80) | (7 << 76) | (bits & ((1 << 76) - 1))
        value = (value & ~(3 << 62)) | (2 << 62)
        at = datetime(1970, 1, 1, tzinfo=UTC) + timedelta(milliseconds=shifted)
        return {
            "event_id": str(uuid.UUID(int=value)),
            "event_type": "quiz." + kind,
            "version": 1,
            "occurred_at": at.isoformat(timespec="milliseconds").replace("+00:00", "Z"),
            "producer": "quizzes",
            "actor": actor,
            "context": context,
            "payload": p,
        }


def convert(
    attempts: Path,
    steps: Path,
    output: Path,
    key: bytes,
    namespace: str,
    scale: int = 1,
) -> dict:
    if scale < 1:
        raise ValueError("Grade scale must be positive")
    ids = Pseudonyms(key, namespace)
    histories = defaultdict(list)
    for row in csv_rows(steps, STEP):
        histories[row["attempt_id"]].append(row)
    events = []
    seen = set()
    counts = Counter()
    for a in csv_rows(attempts, ATTEMPT):
        raw = a["attempt_id"]
        if raw in seen:
            raise ValueError("Duplicate attempt")
        seen.add(raw)
        history = histories.pop(raw, [])
        begin, end = millis(a["start"]), millis(a["finish"])
        n, limit = int(a["question_count"]), int(a["time_limit_sec"])
        pts = [Decimal(a[k]) * scale for k in ("score", "max_score")]
        if (
            end < begin
            or not 0 <= pts[0] <= pts[1]
            or pts[1] < 1
            or any(x != int(x) for x in pts)
        ):
            raise ValueError("Invalid duration or grade units")
        if (
            not 1 <= n <= 200
            or limit < 0
            or a["reason"] not in {"submitted", "timed_out"}
        ):
            raise ValueError("Unsupported attempt metadata")
        actor = {"user_id": ids.id("user", a["user_id"]), "role": "student"}
        ctx = {
            "course_id": ids.id("course", a["course_id"]),
            "group_id": ids.id("group", a["group_id"]),
            "session_id": None,
        }
        common = {
            "attempt_id": ids.id("attempt", raw),
            "quiz_id": ids.id("quiz", a["quiz_id"]),
            "variant_id": ids.id("variant", a["quiz_id"] + "/" + a["variant_id"]),
        }
        if 1 <= limit <= 86400:
            events.append(
                ids.event(
                    "started",
                    begin,
                    actor,
                    ctx,
                    {
                        **common,
                        "question_count": n,
                        "time_limit_sec": limit,
                        "exam_session_id": None,
                    },
                    0,
                )
            )
        else:
            counts["starts_without_representable_limit"] += 1
        changes = 0
        saved = {}
        slots = {}
        marks = set()
        displays = {}
        for step in history:
            if step["shown"]:
                t = millis(step["shown"])
                if displays.setdefault(step["question_id"], t) != t:
                    raise ValueError("Inconsistent first display time")
        for i, step in enumerate(
            sorted(
                history,
                key=lambda r: (millis(r["saved"]), int(r["sequence"]), int(r["slot"])),
            )
        ):
            question = ids.id("question", step["question_id"])
            slot, sequence = int(step["slot"]), int(step["sequence"])
            when = millis(step["saved"])
            if (
                (slot, sequence) in marks
                or not 1 <= slot <= n
                or slots.setdefault(slot, question) != question
            ):
                raise ValueError("Conflicting question history")
            marks.add((slot, sequence))
            if not begin <= when <= end:
                raise ValueError("Save outside attempt")
            choices = json.loads(step["options"])
            if not isinstance(choices, list) or not all(
                isinstance(o, str) and o for o in choices
            ):
                raise ValueError("Provide an array of stable selected option IDs")
            normalized = sorted(
                {ids.id("option", o.strip().casefold()) for o in choices}
            )
            if not normalized:
                raise ValueError("Blank selections require separate response semantics")
            answer = hashlib.sha256(",".join(normalized).encode()).hexdigest()
            if saved.get(question) == answer:
                continue
            changed = question in saved
            changes += changed
            saved[question] = answer
            shown = displays.get(step["question_id"])
            if shown is None:
                counts["saves_without_display_observation"] += 1
                continue
            if not begin <= shown <= when:
                raise ValueError("Invalid first display time")
            events.append(
                ids.event(
                    "answered",
                    when,
                    actor,
                    ctx,
                    {
                        "attempt_id": common["attempt_id"],
                        "question_id": question,
                        "question_index": slot - 1,
                        "answer_hash": answer,
                        "is_change": changed,
                        "time_since_shown_ms": when - shown,
                        "time_since_attempt_start_ms": when - begin,
                    },
                    i,
                )
            )
        events.append(
            ids.event(
                "submitted",
                end,
                actor,
                ctx,
                {
                    **common,
                    "score": int(pts[0]),
                    "max_score": int(pts[1]),
                    "duration_ms": end - begin,
                    "answers_changed": changes,
                    "reason": a["reason"],
                },
                0,
            )
        )
    if histories:
        raise ValueError("Response history without a matching attempt")
    order = {"quiz.started": 0, "quiz.answered": 1, "quiz.submitted": 2}
    events.sort(key=lambda e: (e["occurred_at"], order[e["event_type"]]))
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w", newline="", encoding="utf-8", dir=output.parent, delete=False
    ) as f:
        temp = Path(f.name)
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "event_id",
                "event_type",
                "version",
                "occurred_at",
                "producer",
                "actor",
                "context",
                "payload",
            ],
        )
        writer.writeheader()
        for e in events:
            writer.writerow(
                {
                    k: json.dumps(v, separators=(",", ":"))
                    if isinstance(v, dict)
                    else v
                    for k, v in e.items()
                }
            )
    try:
        os.replace(temp, output)
    finally:
        temp.unlink(missing_ok=True)
    return {"attempts": len(seen), "events": len(events), **counts}


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    for key in ("attempts", "steps", "output", "key-file"):
        p.add_argument("--" + key, type=Path, required=True)
    p.add_argument("--namespace", required=True)
    p.add_argument("--score-scale", type=int, default=1)
    args = p.parse_args()
    print(
        json.dumps(
            convert(
                args.attempts,
                args.steps,
                args.output,
                args.key_file.read_bytes(),
                args.namespace,
                args.score_scale,
            )
        )
    )
