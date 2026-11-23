"""Offline face-presence observations on an explicitly authorized local recording."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import uuid
from collections.abc import Iterable
from datetime import UTC, datetime
from pathlib import Path

import cv2


def segments(
    samples: Iterable[tuple[int, int]], end_ms: int, min_ms: int, gap_ms: int
) -> list[dict]:
    """Sample-and-hold intervals. Only bridge a short one-face interval between equal states."""
    intervals = []
    previous_time = -1
    for t, count in samples:
        if t <= previous_time or t < 0 or t >= end_ms or count < 0:
            raise ValueError("Invalid timestamp or face count")
        kind = "absent" if count == 0 else "multiple_faces" if count > 1 else "one"
        if not intervals or kind != intervals[-1]["kind"]:
            if intervals:
                intervals[-1]["to_ms"] = t
            intervals.append({"from_ms": t, "to_ms": end_ms, "kind": kind})
        previous_time = t
    result = []
    for i, row in enumerate(intervals):
        if row["kind"] == "one":
            continue
        if (
            result
            and result[-1]["kind"] == row["kind"]
            and i
            and intervals[i - 1]["kind"] == "one"
            and row["from_ms"] - result[-1]["to_ms"] <= gap_ms
        ):
            result[-1]["to_ms"] = row["to_ms"]
        else:
            result.append(row.copy())
    return [row for row in result if row["to_ms"] - row["from_ms"] >= min_ms]


def observe(path: Path, cadence: int) -> tuple[list[tuple[int, int]], int]:
    cascade = Path(cv2.data.haarcascades) / "haarcascade_frontalface_default.xml"
    storage = cv2.FileStorage(
        cascade.read_text(encoding="utf-8"),
        cv2.FILE_STORAGE_READ | cv2.FILE_STORAGE_MEMORY,
    )
    detector = cv2.CascadeClassifier()
    loaded = detector.read(storage.getFirstTopLevelNode())
    storage.release()
    if not loaded:
        raise RuntimeError("OpenCV frontal-face cascade unavailable")
    cap = cv2.VideoCapture(str(path))
    observations = []
    try:
        fps = cap.get(cv2.CAP_PROP_FPS)
        if not cap.isOpened() or not math.isfinite(fps) or fps <= 0:
            raise ValueError("Recording has no decodable frame rate")
        previous = -1.0
        first = None
        while True:
            ok, frame = cap.read()
            if not ok:
                break
            pts = cap.get(cv2.CAP_PROP_POS_MSEC)
            if first is None:
                first = pts
            pts -= first
            if not math.isfinite(pts) or pts <= previous:
                raise ValueError("Recording lacks increasing presentation timestamps")
            previous = pts
            t = round(pts)
            if observations and t - observations[-1][0] < cadence:
                continue
            gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
            faces = detector.detectMultiScale(
                gray, scaleFactor=1.1, minNeighbors=5, minSize=(30, 30)
            )
            observations.append((t, len(faces)))
        if not observations:
            raise ValueError("Recording has no frames")
        return observations, max(round(previous + 1000 / fps), observations[-1][0] + 1)
    finally:
        cap.release()


def analyze(
    video: Path,
    record: dict,
    min_ms: int = 1000,
    gap_ms: int = 500,
    sample_ms: int = 200,
) -> dict:
    if min_ms < 1 or gap_ms < 0 or sample_ms < 1:
        raise ValueError("Invalid segmentation parameters")
    if record.get("webcam_consent") is not True or not record.get("exam_session_id"):
        raise ValueError("An authorized session is required")
    if record.get("context", {}).get("session_id") != record["exam_session_id"]:
        raise ValueError("Consent and event must refer to the same session")
    with video.open("rb") as stream:
        checksum = hashlib.file_digest(stream, "sha256").hexdigest()
    if checksum != record.get("video_sha256"):
        raise ValueError("Recording differs from its consent manifest")
    obs, duration = observe(video, sample_ms)
    found = segments(obs, duration, min_ms, gap_ms)
    risk = None
    if found:
        session = record["exam_session_id"]
        fraction = sum(s["to_ms"] - s["from_ms"] for s in found) / duration
        now = datetime.now(UTC)
        bits = (
            (int(now.timestamp() * 1000) << 80)
            | (7 << 76)
            | (2 << 62)
            | (uuid.uuid4().int & ((1 << 62) - 1))
        )
        payload = {
            "risk_id": str(
                uuid.uuid5(
                    uuid.NAMESPACE_URL,
                    f"examory:presence-v1:{session}:{checksum}:{min_ms}:{gap_ms}:{sample_ms}",
                )
            ),
            "target_type": "exam_session",
            "target_id": session,
            "score": fraction,
            "level": "medium",
            "model": {"name": "opencv-frontal-presence", "version": "1.0"},
            "explanation": "A face-count interval warrants human review. Score is the observed fraction of recording time.",
            "evidence": {"segments": found},
        }
        risk = {
            "event_id": str(uuid.UUID(int=bits)),
            "event_type": "risk.proctoring",
            "version": 1,
            "occurred_at": now.isoformat(timespec="milliseconds").replace(
                "+00:00", "Z"
            ),
            "producer": "proctoring",
            "actor": {"role": "system", "user_id": "proctoring"},
            "context": record["context"],
            "payload": payload,
        }
    return {"duration_ms": duration, "sample_count": len(obs), "risk": risk}


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--video", required=True, type=Path)
    p.add_argument("--consent", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--min-ms", type=int, default=1000)
    p.add_argument("--gap-ms", type=int, default=500)
    p.add_argument("--sample-ms", type=int, default=200)
    a = p.parse_args()
    result = analyze(
        a.video,
        json.loads(a.consent.read_text(encoding="utf-8")),
        a.min_ms,
        a.gap_ms,
        a.sample_ms,
    )
    a.output.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
