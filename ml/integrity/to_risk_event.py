# ml/integrity/to_risk_event.py
"""Produce risk.plagiarism using a frozen validation experiment and real IDs."""
from __future__ import annotations

import argparse
import json
import secrets
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

try:
    from .minhash import Detector
except ImportError:
    from minhash import Detector


def risk_event(submission_id: str, matches: list[dict[str, Any]],
               context: dict[str, Any], experiment: dict[str, Any]) -> dict[str, Any]:
    score = max((float(match["similarity"]) for match in matches), default=0.0)
    medium, high = experiment["levels"]["medium"], experiment["levels"]["high"]
    if not 0 <= score <= 1:
        raise ValueError("Invalid Jaccard score")
    level = "high" if high is not None and score >= high else "medium" if score >= medium else "low"
    at = datetime.now(UTC)
    bits = (int(at.timestamp() * 1000) << 80) | (7 << 76) | (secrets.randbits(12) << 64)
    bits |= (2 << 62) | secrets.randbits(62)
    return {
        "event_id": str(uuid.UUID(int=bits)), "event_type": "risk.plagiarism", "version": 1,
        "occurred_at": at.isoformat(timespec="milliseconds").replace("+00:00", "Z"),
        "producer": "integrity", "actor": {"user_id": "integrity", "role": "system"},
        "context": context,
        "payload": {
            "risk_id": str(uuid.uuid4()), "target_type": "submission", "target_id": submission_id,
            "score": score, "level": level,
            "model": {"name": "minhash-v1", "version": experiment["version"]},
            "explanation": (f"{len(matches)} source submissions have verified contiguous text overlap; "
                            f"maximum local five-shingle Jaccard is {score:.3f}. "
                            "Similarity requires human review of attribution; it is not a misconduct probability."),
            "evidence": {"matches": matches},
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--query", type=Path, required=True, help="Locally resolved extracted text_ref")
    parser.add_argument("--sources", type=Path, required=True)
    parser.add_argument("--source-id-map", type=Path, required=True,
                        help="JSON object: relative UTF-8 .txt filename -> submission_id; prefilter by assignment")
    parser.add_argument("--submission-event", type=Path, required=True, help="submission.created envelope JSON")
    parser.add_argument("--experiment", type=Path, default=Path("ml/integrity/artifacts/experiment.json"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    event = json.loads(args.submission_event.read_text(encoding="utf-8"))
    if event["event_type"] != "submission.created" or event["version"] != 1:
        raise ValueError("Expected submission.created v1")
    if event["payload"]["kind"] not in {"text", "file"}:
        raise ValueError("This is the text baseline; code requires a separate experiment")
    experiment = json.loads(args.experiment.read_text(encoding="utf-8"))
    mapping = json.loads(args.source_id_map.read_text(encoding="utf-8"))
    if not mapping or len(set(mapping.values())) != len(mapping):
        raise ValueError("Source IDs must be nonempty and unique")
    sources = {}
    source_root = args.sources.resolve()
    for relative, identifier in mapping.items():
        path = (source_root / relative).resolve()
        if not path.is_relative_to(source_root):
            raise ValueError("Source path escapes --sources")
        sources[identifier] = path.read_bytes().decode("utf-8")
    detector = Detector(sources, experiment["seed"], experiment["num_perm"],
                        experiment["mode"], experiment.get("templates", []))
    query = detector.prepare(event["payload"]["submission_id"], args.query.read_bytes().decode("utf-8"))
    if not query.windows:
        raise ValueError("Abstention: fewer than five usable non-template tokens; no risk event emitted")
    config = experiment["configurations"]["lsh"]
    matches = detector.verify(query, detector.retrieve(query), config["threshold"], config["min_span"])
    result = risk_event(query.identifier, matches, event.get("context", {}), experiment)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, ensure_ascii=False, allow_nan=False), encoding="utf-8")


if __name__ == "__main__":
    main()
