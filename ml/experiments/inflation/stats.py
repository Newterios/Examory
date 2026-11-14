"""Across-seed inference: bootstrap intervals, rank tests, Holm, Latin hypercube."""
from __future__ import annotations

import math
from collections.abc import Sequence
from typing import Any

import numpy as np
from scipy import stats as scipy_stats

BOOTSTRAP_REPEATS = 10_000


def _finite(values: Sequence[float | None]) -> np.ndarray:
    return np.asarray([v for v in values if v is not None and math.isfinite(v)], dtype=float)


def mean_ci(values: Sequence[float | None], repeats: int = BOOTSTRAP_REPEATS, seed: int = 0) -> dict[str, Any]:
    """Mean over seeds with a percentile bootstrap 95 % CI (seeds are the replicates)."""
    data = _finite(values)
    if len(data) == 0:
        return {"mean": None, "low": None, "high": None, "n": 0, "sd": None}
    rng = np.random.default_rng(seed)
    means = data[rng.integers(0, len(data), size=(repeats, len(data)))].mean(axis=1)
    return {"mean": float(data.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n": len(data),
            "sd": float(data.std(ddof=1)) if len(data) > 1 else None}


def paired_difference(x: Sequence[float | None], y: Sequence[float | None],
                      repeats: int = BOOTSTRAP_REPEATS, seed: int = 0) -> dict[str, Any]:
    """Mean of x−y paired by index: paired bootstrap CI + two-sided Wilcoxon signed-rank."""
    if len(x) != len(y):
        raise ValueError("paired samples must have equal length")
    pairs = [(a, b) for a, b in zip(x, y, strict=True) if a is not None and b is not None]
    if not pairs:
        return {"mean": None, "low": None, "high": None, "n": 0, "p": None, "test": "wilcoxon"}
    diff = np.asarray([a - b for a, b in pairs], dtype=float)
    rng = np.random.default_rng(seed)
    means = diff[rng.integers(0, len(diff), size=(repeats, len(diff)))].mean(axis=1)
    if np.all(diff == 0):
        p, statistic = 1.0, None
    else:
        test = scipy_stats.wilcoxon(diff, zero_method="wilcox", alternative="two-sided")
        p, statistic = float(test.pvalue), float(test.statistic)
    return {"mean": float(diff.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n": len(diff), "p": p,
            "statistic": statistic, "test": "wilcoxon", "n_positive": int((diff > 0).sum()),
            "n_negative": int((diff < 0).sum())}


def unpaired_difference(x: Sequence[float | None], y: Sequence[float | None],
                        repeats: int = BOOTSTRAP_REPEATS, seed: int = 0) -> dict[str, Any]:
    """mean(x)−mean(y): independent bootstrap CI + two-sided Mann–Whitney U."""
    a, b = _finite(x), _finite(y)
    if len(a) == 0 or len(b) == 0:
        return {"mean": None, "low": None, "high": None, "n_x": len(a), "n_y": len(b), "p": None,
                "test": "mann-whitney"}
    rng = np.random.default_rng(seed)
    means = (a[rng.integers(0, len(a), size=(repeats, len(a)))].mean(axis=1)
             - b[rng.integers(0, len(b), size=(repeats, len(b)))].mean(axis=1))
    if np.all(a == a[0]) and np.all(b == b[0]) and a[0] == b[0]:
        p, statistic = 1.0, None
    else:
        test = scipy_stats.mannwhitneyu(a, b, alternative="two-sided")
        p, statistic = float(test.pvalue), float(test.statistic)
    return {"mean": float(a.mean() - b.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n_x": len(a), "n_y": len(b),
            "p": p, "statistic": statistic, "test": "mann-whitney"}


def holm(pvalues: Sequence[float | None]) -> list[float | None]:
    """Holm–Bonferroni step-down adjusted p-values; None entries are not tests."""
    indexed = [(p, i) for i, p in enumerate(pvalues) if p is not None]
    m = len(indexed)
    adjusted: list[float | None] = [None] * len(pvalues)
    running = 0.0
    for rank, (p, i) in enumerate(sorted(indexed)):
        running = max(running, min(1.0, (m - rank) * p))
        adjusted[i] = running
    return adjusted


def spearman(x: Sequence[float], y: Sequence[float | None]) -> dict[str, Any]:
    pairs = [(a, b) for a, b in zip(x, y, strict=True) if b is not None]
    if len(pairs) < 3:
        return {"rho": None, "p": None, "n": len(pairs)}
    a, b = np.asarray(pairs, dtype=float).T
    if np.all(a == a[0]) or np.all(b == b[0]):
        return {"rho": None, "p": None, "n": len(pairs)}
    result = scipy_stats.spearmanr(a, b)
    return {"rho": float(result.statistic), "p": float(result.pvalue), "n": len(pairs)}


def latin_hypercube(n: int, dimensions: int, seed: int) -> np.ndarray:
    """n points in [0,1)^d; each margin has exactly one point per stratum [k/n,(k+1)/n)."""
    if n < 1 or dimensions < 1:
        raise ValueError("n and dimensions must be positive")
    rng = np.random.default_rng(seed)
    unit = np.empty((n, dimensions))
    for j in range(dimensions):
        unit[:, j] = (rng.permutation(n) + rng.random(n)) / n
    return unit


def scale_unit(unit: np.ndarray, bounds: Sequence[tuple[float, float, bool]]) -> np.ndarray:
    """Map unit samples to (low, high, log_uniform) bounds."""
    out = np.empty_like(unit)
    for j, (low, high, log_scale) in enumerate(bounds):
        if log_scale:
            out[:, j] = np.exp(np.log(low) + unit[:, j] * (np.log(high) - np.log(low)))
        else:
            out[:, j] = low + unit[:, j] * (high - low)
    return out
