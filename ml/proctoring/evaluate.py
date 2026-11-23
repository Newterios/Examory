"""Aggregate time-based precision and recall over authorized, annotated clips."""

import argparse
import json
from pathlib import Path

from ml.proctoring.detect import analyze


def intervals(rows, kind, duration):
    merged = []
    for a, b in sorted((r["from_ms"], r["to_ms"]) for r in rows if r["kind"] == kind):
        if (
            not isinstance(a, int)
            or not isinstance(b, int)
            or not 0 <= a < b <= duration
        ):
            raise ValueError("Invalid annotation interval")
        if merged and a <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], b)
        else:
            merged.append([a, b])
    return merged


def evaluate(index):
    totals = {kind: [0, 0, 0] for kind in ("absent", "multiple_faces")}
    clips = json.loads(index.read_text(encoding="utf-8"))
    if not clips:
        raise ValueError("At least one authorized clip required")
    for clip in clips:
        consent = json.loads(
            (index.parent / clip["consent"]).read_text(encoding="utf-8")
        )
        result = analyze(index.parent / clip["video"], consent)
        segments = (
            result["risk"]["payload"]["evidence"]["segments"] if result["risk"] else []
        )
        truth = clip["annotations"]
        if any(r["kind"] not in totals for r in truth):
            raise ValueError("Unknown annotation type")
        for kind, counts in totals.items():
            pred = intervals(segments, kind, result["duration_ms"])
            observed = intervals(truth, kind, result["duration_ms"])
            counts[0] += sum(
                max(0, min(b, d) - max(a, c)) for a, b in pred for c, d in observed
            )
            counts[1] += sum(b - a for a, b in pred)
            counts[2] += sum(b - a for a, b in observed)
    return {
        "clips": len(clips),
        "by_kind": {
            k: {
                "true_positive_ms": tp,
                "predicted_ms": p,
                "annotated_ms": t,
                "precision": tp / p if p else None,
                "recall": tp / t if t else None,
            }
            for k, (tp, p, t) in totals.items()
        },
    }


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--index", required=True, type=Path)
    ap.add_argument("--report", required=True, type=Path)
    a = ap.parse_args()
    a.report.write_text(
        json.dumps(evaluate(a.index), indent=2) + "\n", encoding="utf-8"
    )
