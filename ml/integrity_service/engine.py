"""MinHash retrieval keys and direction-aware evidence for immutable submissions."""

from __future__ import annotations

import hashlib
import uuid
from datetime import datetime
from typing import Any

from ml.integrity.minhash import Detector, Document, document, shingles, tokenize
from ml.integrity.to_risk_event import risk_event

VERSION = "1.0.0"
MIN_SPAN = 50
THRESHOLD = 0.80  # Seed-42 PAN validation operating point (ml/integrity/results.md).
EXPERIMENT = {"version": VERSION, "levels": {"medium": THRESHOLD, "high": None}}


def prepare(event: dict, text: str, templates: dict[str, str]) -> Document:
    p = event["payload"]
    excluded = shingles(tokenize(templates.get(p["assignment_id"], "")), set())
    return document(p["submission_id"], text, excluded=excluded)


def band_keys(doc: Document) -> list[bytes]:
    """Band position is part of the signature. Persist only deduplicated bucket keys."""
    keys = set()
    for window in doc.windows:
        for band, rows in enumerate(window.signature.hashvalues.reshape(32, 4)):
            keys.add(
                hashlib.sha256(bytes([band]) + rows.astype(">u8").tobytes()).digest()
            )
    return sorted(keys)


def compare(
    left: tuple[dict, str], right: tuple[dict, str], templates: dict[str, str]
) -> dict[str, Any] | None:
    def order(record: tuple[dict, str]) -> tuple:
        payload = record[0]["payload"]
        return datetime.fromisoformat(payload["submitted_at"]), payload["submission_id"]

    (source, source_text), (target, target_text) = sorted((left, right), key=order)
    if source["actor"]["user_id"] == target["actor"]["user_id"]:
        return None
    s, t = source["payload"], target["payload"]
    boilerplate = [
        templates[key]
        for key in {s["assignment_id"], t["assignment_id"]}
        if key in templates
    ]
    detector = Detector({s["submission_id"]: source_text}, templates=boilerplate)
    query = detector.prepare(t["submission_id"], target_text)
    matches = detector.verify(
        query, detector.retrieve(query, exhaustive=True), THRESHOLD, MIN_SPAN
    )
    if not matches:
        return None
    for match in matches:
        match["overlap_spans"] = sorted(
            match["overlap_spans"],
            key=lambda span: (-span["token_count"], span["target"], span["source"]),
        )[:20]
    result = risk_event(t["submission_id"], matches, target["context"], EXPERIMENT)
    name = f"examory:plagiarism:{VERSION}:{s['submission_id']}:{t['submission_id']}"
    result["payload"]["risk_id"] = str(uuid.uuid5(uuid.NAMESPACE_URL, name))
    return result
