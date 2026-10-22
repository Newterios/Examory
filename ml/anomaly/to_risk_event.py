# ml/anomaly/to_risk_event.py
"""Convert frozen behavior scores into contract-v1 risk.behavior_anomaly JSONL."""
from __future__ import annotations

import argparse
import json
import math
import secrets
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import numpy as np

try:
    from .features import build_features, read_attempts
    from .train import empirical_cdf, load_artifact, score_model
except ImportError:
    from features import build_features, read_attempts
    from train import empirical_cdf, load_artifact, score_model


def envelope(event_type: str, payload: dict[str, Any], context: dict[str, Any],
             at: datetime | None = None) -> dict[str, Any]:
    at = at or datetime.now(UTC)
    if at.tzinfo is None:
        raise ValueError("Event time must be timezone-aware")
    at = at.astimezone(UTC)
    milliseconds = int(at.timestamp() * 1000)
    bits = (milliseconds << 80) | (7 << 76) | (secrets.randbits(12) << 64)
    bits |= (2 << 62) | secrets.randbits(62)
    return {"event_id": str(uuid.UUID(int=bits)), "event_type": event_type, "version": 1,
            "occurred_at": at.isoformat(timespec="milliseconds").replace("+00:00", "Z"),
            "producer": "integrity", "actor": {"user_id": "integrity", "role": "system"},
            "context": context, "payload": payload}


def risk_event(row: dict[str, Any], raw_score: float, model: dict[str, Any],
               version: str) -> dict[str, Any]:
    unusual = []
    for feature, (low, high) in model["bounds"].items():
        value = float(row[feature])
        if math.isfinite(value) and (value < low or value > high):
            unusual.append(f"{feature}={value:.3g} ({'below' if value < low else 'above'} normal-train 1–99% range)")
    explanation = "; ".join(unusual[:5]) or "Joint feature pattern ranked by Isolation Forest; no marginal 1–99% outlier."
    explanation += " Relative anomaly ranking, not a probability or proof of misconduct."
    level = ("high" if raw_score >= model["high_threshold"] else
             "medium" if raw_score >= model["medium_threshold"] else "low")
    z_score = float(row["z_score"])
    payload = {
        "risk_id": str(uuid.uuid4()), "target_type": "attempt", "target_id": row["attempt_id"],
        "score": float(empirical_cdf(model["train_scores"], np.array([raw_score]))[0]),
        "level": level, "model": {"name": "isolation-forest-v1", "version": version},
        "explanation": explanation,
        "evidence": {"features": {"mean_answer_time_ms": float(row["mean_answer_time_ms"]),
                                  "z_score": z_score if math.isfinite(z_score) else None,
                                  "synchronous_with": list(row["synchronous_with"])}},
    }
    return envelope("risk.behavior_anomaly", payload, row["context"])


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, default=Path("ml/datasets/raw/quiz.csv"))
    parser.add_argument("--model", type=Path, default=Path("ml/anomaly/artifacts/model.pkl"))
    parser.add_argument("--output", type=Path, default=Path("ml/anomaly/artifacts/risks.jsonl"))
    parser.add_argument("--all", action="store_true", help="Include low-level assessments")
    args = parser.parse_args()
    artifact = load_artifact(args.model)
    frame = build_features(read_attempts(args.csv), artifact["reference"])
    model = artifact["models"]["iforest"]
    scores = score_model(frame, model)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as stream:
        for row, score in zip(frame.to_dict("records"), scores, strict=True):
            if args.all or score >= model["medium_threshold"]:
                stream.write(json.dumps(risk_event(row, float(score), model, artifact["version"]),
                                        ensure_ascii=False, allow_nan=False) + "\n")


if __name__ == "__main__":
    main()
