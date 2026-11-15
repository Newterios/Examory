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
        return {"mean": None, "low": None, "high": None, "n": 0, "sd": None, "mcse": None}
    rng = np.random.default_rng(seed)
    means = data[rng.integers(0, len(data), size=(repeats, len(data)))].mean(axis=1)
    return {"mean": float(data.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n": len(data),
            "sd": float(data.std(ddof=1)) if len(data) > 1 else None,
            "mcse": float(data.std(ddof=1) / np.sqrt(len(data))) if len(data) > 1 else None}


EXACT_SIGN_FLIP_MAX_N = 20
SIGN_FLIP_MC_REPEATS = 200_000


def sign_flip_p(diff: Sequence[float], seed: int = 0) -> float:
    """Two-sided paired sign-flip permutation test of mean(diff) = 0.

    Statistic: |sum of differences|. For n <= 20 all 2**n sign patterns are enumerated (exact
    p = share of patterns with |sum| >= observed, the observed pattern included). Larger n
    (not used by the harness) fall back to a fixed-seed Monte Carlo with the +1 correction.
    """
    d = np.asarray(diff, dtype=float)
    if len(d) == 0:
        raise ValueError("no differences")
    observed = abs(float(d.sum()))
    tolerance = 1e-9 * max(1.0, float(np.abs(d).sum()))
    if len(d) <= EXACT_SIGN_FLIP_MAX_N:
        sums = np.zeros(1)
        for value in d:
            sums = np.concatenate([sums + value, sums - value])
        return float(np.mean(np.abs(sums) >= observed - tolerance))
    rng = np.random.default_rng(seed)
    signs = rng.choice((-1.0, 1.0), size=(SIGN_FLIP_MC_REPEATS, len(d)))
    hits = int(np.sum(np.abs(signs @ d) >= observed - tolerance))
    return (hits + 1) / (SIGN_FLIP_MC_REPEATS + 1)


def paired_difference(x: Sequence[float | None], y: Sequence[float | None],
                      repeats: int = BOOTSTRAP_REPEATS, seed: int = 0) -> dict[str, Any]:
    """Mean of x−y paired by index: paired bootstrap CI + exact two-sided sign-flip permutation test.

    ``mcse`` is the Monte Carlo standard error SD(differences)/sqrt(n).
    """
    if len(x) != len(y):
        raise ValueError("paired samples must have equal length")
    pairs = [(a, b) for a, b in zip(x, y, strict=True) if a is not None and b is not None]
    if not pairs:
        return {"mean": None, "low": None, "high": None, "n": 0, "p": None, "test": "sign-flip", "mcse": None}
    diff = np.asarray([a - b for a, b in pairs], dtype=float)
    rng = np.random.default_rng(seed)
    means = diff[rng.integers(0, len(diff), size=(repeats, len(diff)))].mean(axis=1)
    p = 1.0 if np.all(diff == 0) else sign_flip_p(diff, seed)
    # Previous method, kept only to list p-values that crossed 0.05 when the test was changed.
    p_wilcoxon = 1.0 if np.all(diff == 0) else float(
        scipy_stats.wilcoxon(diff, zero_method="wilcox", alternative="two-sided").pvalue)
    return {"mean": float(diff.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n": len(diff), "p": p, "p_wilcoxon": p_wilcoxon,
            "statistic": float(diff.sum()), "test": "sign-flip" if len(diff) <= EXACT_SIGN_FLIP_MAX_N
            else "sign-flip-mc", "n_positive": int((diff > 0).sum()), "n_negative": int((diff < 0).sum()),
            "sd": float(diff.std(ddof=1)) if len(diff) > 1 else None,
            "mcse": float(diff.std(ddof=1) / np.sqrt(len(diff))) if len(diff) > 1 else None}


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
    mcse = (float(np.sqrt(a.var(ddof=1) / len(a) + b.var(ddof=1) / len(b)))
            if len(a) > 1 and len(b) > 1 else None)
    return {"mean": float(a.mean() - b.mean()), "low": float(np.quantile(means, .025)),
            "high": float(np.quantile(means, .975)), "n_x": len(a), "n_y": len(b),
            "p": p, "statistic": statistic, "test": "mann-whitney", "mcse": mcse}


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


def _ranks(values: np.ndarray) -> np.ndarray:
    return scipy_stats.rankdata(values, axis=0)


def prcc(parameters: np.ndarray, outcome: Sequence[float | None]) -> list[dict[str, Any]]:
    """Partial rank correlation of each parameter column with the outcome, all columns jointly.

    Ranks of x_j and y are regressed (OLS with intercept) on the ranks of the other parameters;
    PRCC_j is the Pearson correlation of the two residual vectors. Two-sided p from
    t = r √((n − 2 − k) / (1 − r²)), df = n − 2 − k, k = number of other parameters.
    Rows with a missing outcome are dropped.
    """
    x = np.asarray(parameters, dtype=float)
    keep = np.asarray([v is not None for v in outcome])
    y = np.asarray([v for v in outcome if v is not None], dtype=float)
    x = x[keep]
    n, p = x.shape
    k = p - 1
    if n - 2 - k < 1 or np.all(y == y[0]):
        return [{"prcc": None, "p": None, "n": n, "df": n - 2 - k} for _ in range(p)]
    rx, ry = _ranks(x), _ranks(y)
    out = []
    for j in range(p):
        others = np.column_stack([np.ones(n), np.delete(rx, j, axis=1)])
        res_x = rx[:, j] - others @ np.linalg.lstsq(others, rx[:, j], rcond=None)[0]
        res_y = ry - others @ np.linalg.lstsq(others, ry, rcond=None)[0]
        denominator = np.sqrt((res_x ** 2).sum() * (res_y ** 2).sum())
        r = float((res_x * res_y).sum() / denominator) if denominator > 0 else 0.0
        df = n - 2 - k
        t = r * np.sqrt(df / max(1e-300, 1 - r * r))
        out.append({"prcc": r, "p": float(2 * scipy_stats.t.sf(abs(t), df)), "n": n, "df": df})
    return out
