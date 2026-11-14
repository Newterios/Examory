"""Detection metrics. Predictions use ``score >= threshold`` like the frozen pipeline.

Undefined quantities (no positives, no negatives, empty denominators) are None.
"""
from __future__ import annotations

from typing import Any

import numpy as np
from sklearn.metrics import average_precision_score


def _validate(labels: np.ndarray, scores: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    labels = np.asarray(labels).astype(int)
    scores = np.asarray(scores, dtype=float)
    if labels.shape != scores.shape or labels.ndim != 1:
        raise ValueError("labels and scores must be 1-D arrays of equal length")
    if not set(np.unique(labels)) <= {0, 1}:
        raise ValueError("labels must be 0/1")
    if not np.all(np.isfinite(scores)):
        raise ValueError("scores must be finite")
    return labels, scores


def at_threshold(labels: np.ndarray, scores: np.ndarray, threshold: float) -> dict[str, Any]:
    labels, scores = _validate(labels, scores)
    predicted = scores >= threshold
    positive = labels == 1
    tp, fp = int(np.sum(predicted & positive)), int(np.sum(predicted & ~positive))
    fn, tn = int(np.sum(~predicted & positive)), int(np.sum(~predicted & ~positive))
    n = len(labels)
    return {"tp": tp, "fp": fp, "fn": fn, "tn": tn,
            "precision": tp / (tp + fp) if tp + fp else None,
            "recall": tp / (tp + fn) if tp + fn else None,
            "f1": 2 * tp / (2 * tp + fp + fn) if tp + fn else None,
            "fpr": fp / (fp + tn) if fp + tn else None,
            "alerts_per_100": 100 * (tp + fp) / n if n else None}


def roc_points(labels: np.ndarray, scores: np.ndarray) -> tuple[np.ndarray, np.ndarray] | None:
    """ROC vertices for descending distinct thresholds; ties form one diagonal step."""
    labels, scores = _validate(labels, scores)
    positives, negatives = int(labels.sum()), int((labels == 0).sum())
    if positives == 0 or negatives == 0:
        return None
    order = np.argsort(-scores, kind="mergesort")
    sorted_scores, sorted_labels = scores[order], labels[order]
    last_of_tie = np.r_[np.flatnonzero(np.diff(sorted_scores)), len(scores) - 1]
    tps = np.cumsum(sorted_labels)[last_of_tie]
    fps = (last_of_tie + 1) - tps
    fpr = np.r_[0.0, fps / negatives]
    tpr = np.r_[0.0, tps / positives]
    return fpr, tpr


def roc_auc(labels: np.ndarray, scores: np.ndarray) -> float | None:
    points = roc_points(labels, scores)
    if points is None:
        return None
    fpr, tpr = points
    return float(np.trapezoid(tpr, fpr))


def standardized_pauc(labels: np.ndarray, scores: np.ndarray, max_fpr: float = 0.05) -> float | None:
    """McClish (1989) standardized partial AUC over FPR in [0, max_fpr].

    0.5 for a chance-level ROC, 1 for a perfect one (same definition as
    ``sklearn.metrics.roc_auc_score(max_fpr=...)``).
    """
    if not 0 < max_fpr <= 1:
        raise ValueError("max_fpr must lie in (0, 1]")
    points = roc_points(labels, scores)
    if points is None:
        return None
    fpr, tpr = points
    if max_fpr == 1:
        return float(np.trapezoid(tpr, fpr))
    stop = int(np.searchsorted(fpr, max_fpr, side="right"))
    x = np.r_[fpr[:stop], max_fpr]
    y = np.r_[tpr[:stop], np.interp(max_fpr, fpr[stop - 1:stop + 1], tpr[stop - 1:stop + 1])]
    partial = float(np.trapezoid(y, x))
    low, high = 0.5 * max_fpr ** 2, max_fpr
    return 0.5 * (1 + (partial - low) / (high - low))


def tpr_on_grid(labels: np.ndarray, scores: np.ndarray, grid: np.ndarray) -> list[float] | None:
    """Linearly interpolated ROC at FPR grid points (upper point on vertical steps)."""
    points = roc_points(labels, scores)
    if points is None:
        return None
    fpr, tpr = points
    result = []
    for value in grid:
        i = int(np.searchsorted(fpr, value, side="right")) - 1
        if fpr[i] == value or i == len(fpr) - 1:
            result.append(float(tpr[i]))
        else:
            weight = (value - fpr[i]) / (fpr[i + 1] - fpr[i])
            result.append(float(tpr[i] + weight * (tpr[i + 1] - tpr[i])))
    return result


def average_precision(labels: np.ndarray, scores: np.ndarray) -> float | None:
    labels, scores = _validate(labels, scores)
    if labels.sum() == 0:
        return None
    return float(average_precision_score(labels, scores))


def cohort_bootstrap(groups: np.ndarray, labels: np.ndarray, predicted: np.ndarray,
                     repeats: int, seed: int) -> dict[str, Any]:
    """Percentile 95 % intervals resampling whole cohorts with replacement."""
    groups = np.asarray(groups)
    labels = np.asarray(labels).astype(int)
    predicted = np.asarray(predicted, dtype=bool)
    names, index = np.unique(groups, return_inverse=True)
    if len(names) < 2:
        return {"recall": None, "fpr": None, "precision": None}
    counts = np.zeros((len(names), 4))
    np.add.at(counts[:, 0], index, predicted & (labels == 1))
    np.add.at(counts[:, 1], index, predicted & (labels == 0))
    np.add.at(counts[:, 2], index, labels == 1)
    np.add.at(counts[:, 3], index, labels == 0)
    rng = np.random.default_rng(seed)
    draws = rng.integers(0, len(names), size=(repeats, len(names)))
    weights = np.zeros((repeats, len(names)))
    np.add.at(weights, (np.repeat(np.arange(repeats), len(names)), draws.ravel()), 1)
    tp, fp, pos, neg = (weights @ counts).T
    result = {}
    for name, numerator, denominator in (("recall", tp, pos), ("fpr", fp, neg), ("precision", tp, tp + fp)):
        valid = denominator > 0
        values = numerator[valid] / denominator[valid]
        result[name] = ({"low": float(np.quantile(values, .025)), "high": float(np.quantile(values, .975)),
                         "valid_replicates": int(valid.sum())} if valid.any() else None)
    return result


def full_metrics(labels: np.ndarray, scores: np.ndarray, thresholds: dict[str, float],
                 groups: np.ndarray | None = None, bootstrap: int = 0, seed: int = 0,
                 grid: np.ndarray | None = None) -> dict[str, Any]:
    labels, scores = _validate(labels, scores)
    row: dict[str, Any] = {"n": len(labels), "n_pos": int(labels.sum()), "n_neg": int((labels == 0).sum()),
                           "roc_auc": roc_auc(labels, scores),
                           "pauc_std": standardized_pauc(labels, scores, 0.05),
                           "average_precision": average_precision(labels, scores)}
    for name, threshold in thresholds.items():
        row[f"at_{name}"] = at_threshold(labels, scores, threshold)
        if bootstrap and groups is not None and name == "0.05":
            row["cohort_ci_0.05"] = cohort_bootstrap(groups, labels, scores >= threshold, bootstrap, seed)
    if grid is not None:
        row["roc_tpr_grid"] = tpr_on_grid(labels, scores, grid)
    return row
