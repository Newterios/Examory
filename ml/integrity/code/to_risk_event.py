"""Compare two resolved Java submissions and write a risk.plagiarism v1 event."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

from ml.integrity.to_risk_event import risk_event

from .algorithm import overlap_spans, prepare, similarity, template_grams


def compare_submissions(
    target_id: str,
    target_text: str,
    source_id: str,
    source_text: str,
    context: dict[str, Any],
    threshold: float,
    high: float | None = None,
    template: str = "",
) -> dict[str, Any] | None:
    if not target_id.strip() or not source_id.strip() or source_id == target_id:
        raise ValueError("Distinct nonempty submission IDs are required")
    if not 0 < threshold <= 1 or (high is not None and not threshold <= high <= 1):
        raise ValueError("Expected 0 < medium <= high <= 1 (high is optional)")
    if not isinstance(context, dict) or any(
        key not in context
        or (context[key] is not None and not isinstance(context[key], str))
        for key in ("course_id", "group_id", "session_id")
    ):
        raise ValueError(
            "Context requires course_id, group_id and session_id as strings or null"
        )
    excluded = template_grams(template)
    target, source = (
        prepare(text, excluded=excluded) for text in (target_text, source_text)
    )
    score = similarity(target, source)
    if score < threshold:
        return None
    spans = overlap_spans(target, source)
    if not spans:
        return None
    match = {"submission_id": source_id, "similarity": score, "overlap_spans": spans}
    event = risk_event(
        target_id,
        [match],
        context,
        {"version": "0.1.0", "levels": {"medium": threshold, "high": high}},
    )
    event["payload"]["model"]["name"] = "java-winnowing-v1"
    event["payload"]["explanation"] = (
        f"The best pair of code windows shares {score:.1%} of the distinct selected "
        "fingerprints in their union after identifier and literal normalization. "
        "Review the marked passages alongside the assignment instructions."
    )
    return event


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", type=Path, required=True)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--target-id", required=True)
    parser.add_argument("--source-id", required=True)
    parser.add_argument(
        "--context",
        required=True,
        type=Path,
        help="JSON context from submission.created",
    )
    parser.add_argument(
        "--threshold",
        required=True,
        type=float,
        help="Medium cutoff frozen on validation",
    )
    parser.add_argument(
        "--high-threshold",
        type=float,
        help="Optional second cutoff frozen on validation",
    )
    parser.add_argument(
        "--template", type=Path, help="Optional instructor-supplied UTF-8 Java template"
    )
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    # Reading bytes preserves CRLF and every code point used by the offsets.
    event = compare_submissions(
        args.target_id,
        args.target.read_bytes().decode("utf-8"),
        args.source_id,
        args.source.read_bytes().decode("utf-8"),
        json.loads(args.context.read_bytes()),
        args.threshold,
        args.high_threshold,
        args.template.read_bytes().decode("utf-8") if args.template else "",
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(event, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


if __name__ == "__main__":
    main()
