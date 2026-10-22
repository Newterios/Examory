# ml/anomaly/train.py
"""Train normal-only Isolation Forests and freeze validation operating points."""
from __future__ import annotations

import argparse
import importlib.metadata
import json
import pickle
import platform
import subprocess
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest

try:
    from .features import (
        BASE_FEATURES,
        FEATURE_COLUMNS,
        LOG_FEATURES,
        build_features,
        file_sha256,
        fit_reference,
        read_attempts,
        write_json,
    )
except ImportError:
    from features import (
        BASE_FEATURES,
        FEATURE_COLUMNS,
        LOG_FEATURES,
        build_features,
        file_sha256,
        fit_reference,
        read_attempts,
        write_json,
    )


def matrix(frame: pd.DataFrame, columns: list[str], medians: np.ndarray | None = None
           ) -> tuple[np.ndarray, np.ndarray]:
    values = frame[columns].to_numpy(dtype=float, copy=True)
    missing = ~np.isfinite(values)
    values[missing] = np.nan
    for index, column in enumerate(columns):
        if column in LOG_FEATURES:
            if np.any(values[:, index] < 0):
                raise ValueError(f"Negative value for {column}")
            values[:, index] = np.log1p(values[:, index])
    if medians is None:
        medians = np.array([np.median(v[np.isfinite(v)]) if np.isfinite(v).any() else 0.0
                            for v in values.T])
    values = np.where(missing, medians, values)
    return np.hstack([values, missing.astype(float)]), medians


def empirical_cdf(reference: np.ndarray, scores: np.ndarray) -> np.ndarray:
    return np.searchsorted(np.sort(reference), scores, side="right") / len(reference)


def choose_threshold(labels: np.ndarray, scores: np.ndarray, max_fpr: float) -> dict[str, Any]:
    """Prediction uses >=; ties are never randomly split to achieve target FPR."""
    negative = labels == 0
    if not negative.any():
        raise ValueError("Validation requires normal attempts")
    choices = np.append(np.unique(scores), np.nextafter(scores.max(), np.inf))
    candidates = []
    for threshold in choices:
        prediction = scores >= threshold
        fp = int(np.sum(prediction & negative))
        tp = int(np.sum(prediction & (labels == 1)))
        if fp / negative.sum() <= max_fpr + 1e-12:
            # If there are positives maximize recall, then minimize false alarms.
            key = (tp, -fp, threshold) if np.any(labels == 1) else (fp, -threshold, 0)
            candidates.append((key, float(threshold), tp, fp))
    _, threshold, tp, fp = max(candidates)
    return {"threshold": threshold, "validation_fpr": fp / int(negative.sum()),
            "validation_recall": tp / int(np.sum(labels == 1)) if np.any(labels == 1) else None,
            "validation_negatives": int(negative.sum()), "target_fpr": max_fpr,
            "supported": int(negative.sum()) >= round(1 / max_fpr)}


def quantile_scores(values: np.ndarray, sorted_train: np.ndarray) -> np.ndarray:
    # Mid-rank avoids treating a constant feature as an extreme observation.
    result = np.zeros(len(values))
    for i in range(values.shape[1]):
        ref = sorted_train[:, i]
        rank = (np.searchsorted(ref, values[:, i], side="left")
                + np.searchsorted(ref, values[:, i], side="right")) / (2 * len(ref))
        result = np.maximum(result, 2 * np.abs(rank - .5))
    return result


def score_model(frame: pd.DataFrame, model: dict[str, Any]) -> np.ndarray:
    values, _ = matrix(frame, model["columns"], model["medians"])
    if model["kind"] == "quantile_rules":
        return quantile_scores(values, model["sorted_train"])
    return -model["estimator"].score_samples(values)


def load_artifact(path: str | Path) -> dict[str, Any]:
    """Only load trusted artifacts produced locally by this program (pickle)."""
    with Path(path).open("rb") as stream:
        artifact = pickle.load(stream)
    if artifact["schema_version"] != 1:
        raise ValueError("Unsupported artifact schema")
    return artifact


def train(csv: Path, manifest_path: Path, output: Path) -> dict[str, Any]:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if file_sha256(csv) != manifest["csv_sha256"]:
        raise ValueError("CSV differs from manifest")
    partitions = list(manifest["groups"].values())
    all_groups = [group for partition in partitions for group in partition]
    if len(set(all_groups)) != len(all_groups):
        raise ValueError("Overlapping cohort partitions")
    attempts = read_attempts(csv)
    if {a["group_id"] for a in attempts} != set(all_groups):
        raise ValueError("Manifest does not cover all cohorts")
    normal = [a for a in attempts if a["group_id"] in manifest["groups"]["train"] and a["is_anomaly"] == 0]
    if len(normal) < 2:
        raise ValueError("Need at least two normal training attempts")
    reference = fit_reference(normal)
    frame = build_features(attempts, reference)
    training = frame[frame.group_id.isin(manifest["groups"]["train"]) & (frame.is_anomaly == 0)]
    validation = frame[frame.group_id.isin(manifest["groups"]["validation"])]
    labels = validation.is_anomaly.to_numpy(dtype=int)
    models = {}
    for name, columns in (("iforest", FEATURE_COLUMNS), ("iforest_no_peers", BASE_FEATURES),
                          ("quantile_rules", FEATURE_COLUMNS)):
        values, medians = matrix(training, columns)
        model = {"kind": name, "columns": columns, "medians": medians}
        if name == "quantile_rules":
            model["sorted_train"] = np.sort(values, axis=0)
        else:
            estimator = IsolationForest(n_estimators=200, max_samples=min(256, len(training)),
                                        contamination="auto", random_state=42)
            estimator.fit(values)
            model["estimator"] = estimator
        model["train_scores"] = np.sort(score_model(training, model))
        validation_scores = score_model(validation, model)
        model["operating_points"] = {str(rate): choose_threshold(labels, validation_scores, rate)
                                     for rate in (.05, .01)}
        model["medium_threshold"] = model["operating_points"]["0.05"]["threshold"]
        model["high_threshold"] = max(model["medium_threshold"], model["operating_points"]["0.01"]["threshold"])
        model["bounds"] = {column: [float(training[column].quantile(.01)), float(training[column].quantile(.99))]
                           for column in columns if training[column].notna().any()}
        models[name] = model
    try:
        revision = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=False).stdout.strip()
    except FileNotFoundError:
        revision = ""
    artifact = {
        "schema_version": 1, "version": "0.1.0", "reference": reference,
        "models": models, "manifest": manifest, "csv_sha256": manifest["csv_sha256"],
        "environment": {"python": platform.python_version(), "platform": platform.platform(),
                        "processor": platform.processor(), "code_revision": revision or "uncommitted",
                        "source_sha256": {p.name: file_sha256(p) for p in sorted(Path(__file__).parent.glob("*.py"))},
                        "libraries": {p: importlib.metadata.version(p)
                                      for p in ("numpy", "pandas", "scikit-learn", "datasketch")}},
        "normal_train_attempts": len(training),
    }
    output.mkdir(parents=True, exist_ok=True)
    with (output / "model.pkl").open("wb") as stream:
        pickle.dump(artifact, stream, protocol=pickle.HIGHEST_PROTOCOL)
    write_json(output / "thresholds.json", {
        "version": artifact["version"], "csv_sha256": artifact["csv_sha256"],
        "normal_train_attempts": len(training), "environment": artifact["environment"],
        "models": {name: {"operating_points": model["operating_points"],
                          "medium_raw_score": model["medium_threshold"],
                          "high_raw_score": model["high_threshold"],
                          "risk_score_definition": "empirical CDF of normal training anomaly scores"}
                   for name, model in models.items()},
    })
    return artifact


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, default=Path("ml/datasets/raw/quiz.csv"))
    parser.add_argument("--manifest", type=Path, default=Path("ml/anomaly/artifacts/split.json"))
    parser.add_argument("--output", type=Path, default=Path("ml/anomaly/artifacts"))
    args = parser.parse_args()
    artifact = train(args.csv, args.manifest, args.output)
    print(json.dumps({k: v["operating_points"] for k, v in artifact["models"].items()}, indent=2))


if __name__ == "__main__":
    main()
