# ml/anomaly/evaluate.py
"""Evaluate frozen models on held-out cohorts; never tune thresholds on test."""
from __future__ import annotations

import argparse
import json
import time
import tracemalloc
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.metrics import auc, average_precision_score, precision_recall_curve

try:
    from .features import build_features, file_sha256, read_attempts, write_json
    from .train import load_artifact, score_model
except ImportError:
    from features import build_features, file_sha256, read_attempts, write_json
    from train import load_artifact, score_model


def metrics(labels: np.ndarray, scores: np.ndarray, threshold: float) -> dict[str, Any]:
    predicted = scores >= threshold
    positive = labels == 1
    tp, fp = int(np.sum(predicted & positive)), int(np.sum(predicted & ~positive))
    fn, tn = int(np.sum(~predicted & positive)), int(np.sum(~predicted & ~positive))
    result = {"n": len(labels), "tp": tp, "fp": fp, "fn": fn, "tn": tn,
              "precision": tp / (tp + fp) if tp + fp else None,
              "recall": tp / (tp + fn) if tp + fn else None,
              "f1": 2 * tp / (2 * tp + fp + fn) if 2 * tp + fp + fn else None,
              "fpr": fp / (fp + tn) if fp + tn else None,
              "alerts_per_100": 100 * (tp + fp) / len(labels) if len(labels) else None,
              "pr_auc": None, "average_precision": None}
    if positive.any() and (~positive).any():
        precision, recall, _ = precision_recall_curve(labels, scores)
        result["pr_auc"] = float(auc(recall, precision))
        result["average_precision"] = float(average_precision_score(labels, scores))
    return result


def bootstrap(frame: pd.DataFrame, scores: np.ndarray, threshold: float,
              repeats: int, seed: int) -> dict[str, Any]:
    groups = [np.flatnonzero(frame.group_id.to_numpy() == group) for group in sorted(frame.group_id.unique())]
    names = ("precision", "recall", "f1", "fpr", "pr_auc")
    values: dict[str, list[float]] = {name: [] for name in names}
    if len(groups) < 2:
        return {name: None for name in names}
    rng = np.random.default_rng(seed)
    labels = frame.is_anomaly.to_numpy(dtype=int)
    for _ in range(repeats):
        indices = np.concatenate([groups[i] for i in rng.integers(0, len(groups), len(groups))])
        row = metrics(labels[indices], scores[indices], threshold)
        for name in names:
            if row[name] is not None:
                values[name].append(row[name])
    return {name: {"low": float(np.quantile(v, .025)), "high": float(np.quantile(v, .975)),
                   "valid_replicates": len(v)} if v else None for name, v in values.items()}


def fmt(value: Any) -> str:
    return "NA" if value is None else f"{value:.4f}" if isinstance(value, float) else str(value)


def evaluate(csv: Path, artifact_path: Path, output: Path, repeats: int, seed: int) -> dict[str, Any]:
    started = time.perf_counter()
    tracemalloc.start()
    artifact = load_artifact(artifact_path)
    if file_sha256(csv) != artifact["csv_sha256"]:
        raise ValueError("Evaluation CSV must match the frozen split")
    frame = build_features(read_attempts(csv), artifact["reference"])
    frame = frame[frame.group_id.isin(artifact["manifest"]["groups"]["test"])].reset_index(drop=True)
    labels = frame.is_anomaly.to_numpy(dtype=int)
    rows, intervals, operating, curves = [], [], [], {}
    for name, model in artifact["models"].items():
        scores = score_model(frame, model)
        threshold = model["medium_threshold"]
        subsets = {"all": np.ones(len(frame), dtype=bool), "final_only": frame.is_final_attempt.to_numpy()}
        for kind in sorted(set(frame.anomaly_type) - {"none", ""}):
            subsets[f"type:{kind}"] = ((labels == 0) | (frame.anomaly_type.to_numpy() == kind))
        # Proxy strata are explicit: latent generator profile is not in the CSV.
        fast_limit = artifact["reference"]["global_fast_ms"]
        subsets["normal_fast_proxy"] = (labels == 0) & (frame.median_answer_time_ms.to_numpy() < fast_limit)
        change_limit = model["bounds"]["changes_fraction"][1]
        subsets["normal_revising_proxy"] = (labels == 0) & (frame.changes_fraction.to_numpy() > change_limit)
        for subset, mask in subsets.items():
            row = {"model": name, "subset": subset, **metrics(labels[mask], scores[mask], threshold)}
            rows.append(row)
            intervals.append({"model": name, "subset": subset,
                              "intervals": bootstrap(frame.loc[mask], scores[mask], threshold, repeats, seed)})
        for target, point in model["operating_points"].items():
            row = metrics(labels, scores, point["threshold"])
            operating.append({"model": name, "validation_target_fpr": target,
                              "validation_fpr": point["validation_fpr"], "supported": point["supported"],
                              "test_recall": row["recall"] if point["supported"] else None,
                              "test_fpr": row["fpr"] if point["supported"] else None})
        if len(np.unique(labels)) == 2:
            precision, recall, thresholds = precision_recall_curve(labels, scores)
            curves[name] = {"precision": precision.tolist(), "recall": recall.tolist(),
                            "thresholds": thresholds.tolist()}
    _, peak = tracemalloc.get_traced_memory()
    tracemalloc.stop()
    result = {"rows": rows, "bootstrap": intervals, "operating_points": operating,
              "seconds": time.perf_counter() - started, "python_peak_bytes": peak,
              "bootstrap_repeats": repeats, "seed": seed, "environment": artifact["environment"],
              "csv_sha256": artifact["csv_sha256"], "pr_curves": curves}
    write_json(artifact_path.parent / "evaluation.json", result)
    lines = ["<!-- ml/anomaly/results.md -->", "# Held-out behavior results", "",
             f"CSV SHA-256: `{artifact['csv_sha256']}`. Cohort bootstrap: {repeats} replicates, seed {seed}.", "",
             "Thresholds were frozen on validation. Test FPR can exceed its validation target.", "",
             "| Model | Subset | N | TP | FP | FN | TN | Precision | Recall | F1 | FPR | PR-AUC | AP |",
             "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|"]
    keys = ("model", "subset", "n", "tp", "fp", "fn", "tn", "precision", "recall", "f1", "fpr", "pr_auc", "average_precision")
    lines += ["| " + " | ".join(fmt(row[k]) for k in keys) + " |" for row in rows]
    lines += ["", "## Validation operating points", "",
              "| Model | Target FPR | Validation FPR | Supported | Test recall | Test FPR |",
              "|---|---:|---:|---|---:|---:|"]
    lines += ["| " + " | ".join(fmt(v) for v in row.values()) + " |" for row in operating]
    lines += ["", "## Cohort bootstrap 95% intervals", "",
              "| Model | Subset | Metric | Low | High | Valid replicates |", "|---|---|---|---:|---:|---:|"]
    for item in intervals:
        for metric, interval in item["intervals"].items():
            values = interval or {"low": None, "high": None, "valid_replicates": 0}
            lines.append("| " + " | ".join(map(fmt, [item["model"], item["subset"], metric, *values.values()])) + " |")
    lines += ["", f"Evaluation time: {result['seconds']:.2f} s; Python-tracked peak memory: {peak} bytes (not total RSS).",
              "PR-AUC is trapezoidal; AP is average precision. Undefined denominators are NA.",
              "Type rows compare that anomaly type with all normal attempts; other anomaly types are excluded.",
              "Fast/revising strata are observed-feature proxies, not the generator's latent profiles.",
              "True collusion-pair precision/recall and latent-profile FPR are unavailable: the CSV has no pair/profile labels.",
              "Injection membership is the ground truth here; it does not establish misconduct.", ""]
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(lines), encoding="utf-8")
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, default=Path("ml/datasets/raw/quiz.csv"))
    parser.add_argument("--model", type=Path, default=Path("ml/anomaly/artifacts/model.pkl"))
    parser.add_argument("--output", type=Path, default=Path("ml/anomaly/results.md"))
    parser.add_argument("--bootstrap", type=int, default=1000)
    parser.add_argument("--seed", type=int, default=42)
    args = parser.parse_args()
    if args.bootstrap < 1:
        parser.error("--bootstrap must be positive")
    result = evaluate(args.csv, args.model, args.output, args.bootstrap, args.seed)
    print(json.dumps(result["rows"][:2], indent=2))


if __name__ == "__main__":
    main()
