# ml/ednet/models.py
"""Estimators used by the EdNet calibration (numpy/scipy only, unit-tested).

* ``fit_beta_binomial`` - (zero-inflated) beta-binomial maximum likelihood for
  per-person counts k_i out of n_i; it separates between-person variation of a
  rate from binomial sampling noise.
* ``fit_irt`` - joint MAP estimation of a 3PL model with a common, fixed lower
  asymptote c: P(y=1) = c + (1-c) * logistic(a_j (theta_i - b_j)), with
  theta ~ N(0, 1) (scale pinned every iteration), weak priors on the item
  slope/intercept. Alternating Fisher scoring (persons, then items with a 2x2
  block per item).
* ``fit_lognormal_rt`` - van der Linden's lognormal response-time model
  log t_ij = beta_j - tau_i + e, e ~ N(0, sigma_j^2), by alternating weighted
  least squares; mean(tau) = 0 identifies the location.
* ``noise_corrected_sd`` - method-of-moments latent SD: var(estimate) minus the
  mean squared standard error.
"""
from __future__ import annotations

from typing import Any

import numpy as np
from scipy.optimize import minimize
from scipy.special import betaln, expit, gammaln


def _patterns(k: np.ndarray, n: np.ndarray, weights: np.ndarray | None = None
              ) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    k, n = np.asarray(k, dtype=np.int64), np.asarray(n, dtype=np.int64)
    if np.any(k < 0) or np.any(k > n) or np.any(n <= 0):
        raise ValueError("Need 0 <= k <= n and n > 0")
    w = np.ones(len(k)) if weights is None else np.asarray(weights, dtype=float)
    codes, inverse = np.unique(np.stack([k, n], axis=1), axis=0, return_inverse=True)
    return codes[:, 0].astype(float), codes[:, 1].astype(float), np.bincount(inverse.ravel(), w)


def beta_binomial_logpmf(k: np.ndarray, n: np.ndarray, a: float, b: float) -> np.ndarray:
    return (gammaln(n + 1) - gammaln(k + 1) - gammaln(n - k + 1)
            + betaln(k + a, n - k + b) - betaln(a, b))


def fit_beta_binomial(k: np.ndarray, n: np.ndarray, zero_inflated: bool = False,
                      weights: np.ndarray | None = None) -> dict[str, float]:
    """MLE of Beta(a, b) for person rates (optionally with a point mass at 0)."""
    kk, nn, ww = _patterns(k, n, weights)
    rate = np.sum(ww * kk) / np.sum(ww * nn)
    positive = kk > 0
    zero_share = np.sum(ww[~positive]) / np.sum(ww)
    mean0 = max(1e-4, min(.9, rate / max(1e-3, 1 - zero_share) if zero_inflated else rate))
    x0 = [np.log(mean0 * 5), np.log((1 - mean0) * 5)]
    if zero_inflated:
        x0 = [np.log(max(zero_share, .02) / max(1 - zero_share, .02))] + x0

    def negative_loglik(x: np.ndarray) -> float:
        if zero_inflated:
            pi, a, b = expit(x[0]), np.exp(x[1]), np.exp(x[2])
        else:
            pi, a, b = 0.0, np.exp(x[0]), np.exp(x[1])
        log_bb = beta_binomial_logpmf(kk, nn, a, b)
        if zero_inflated:
            ll = np.where(positive, np.log1p(-pi) + log_bb,
                          np.logaddexp(np.log(max(pi, 1e-300)), np.log1p(-pi) + log_bb))
        else:
            ll = log_bb
        return -float(np.sum(ww * ll))

    best = None
    for shift in (0.0, 2.0, -2.0):
        start = np.array(x0, dtype=float)
        start[-2:] += shift
        result = minimize(negative_loglik, start, method="L-BFGS-B",
                          bounds=[(-12, 12)] * len(start))
        if best is None or result.fun < best.fun:
            best = result
    x = best.x
    if zero_inflated:
        out = {"p_zero": float(expit(x[0])), "beta_a": float(np.exp(x[1])), "beta_b": float(np.exp(x[2]))}
    else:
        out = {"p_zero": 0.0, "beta_a": float(np.exp(x[0])), "beta_b": float(np.exp(x[1]))}
    out.update({"loglik": float(-best.fun), "converged": bool(best.success),
                "n_persons": float(np.sum(ww)), "pooled_rate": float(rate)})
    return out


def irt_probability(theta: np.ndarray, a: np.ndarray, b: np.ndarray, c: float) -> np.ndarray:
    return c + (1 - c) * expit(a * (theta - b))


def fit_irt(person: np.ndarray, item: np.ndarray, y: np.ndarray, c: float,
            n_persons: int | None = None, n_items: int | None = None,
            d_sd: float = 3.0, log_a_sd: float = 1.0, max_iter: int = 1000, tol: float = 1e-3,
            init: dict[str, np.ndarray] | None = None, scale: str = "latent") -> dict[str, Any]:
    """Joint MAP 3PL with common fixed c; returns estimates, SEs and log posterior.

    Items use the slope-intercept form z = a theta + d (b = -d / a), which stays
    well conditioned for flat items, with priors log a ~ N(0, log_a_sd^2) and
    d ~ N(0, d_sd^2). Joint MAP alone lets the theta scale collapse (theta
    shrinks while a inflates), so after every person step the population scale
    is pinned: mean 0 and var(theta_hat) + mean(posterior var) = 1 ("latent",
    i.e. the latent SD is 1) or var(theta_hat) = 1 ("estimate"); items are
    transformed so every a theta + d is unchanged.
    """
    person, item = np.asarray(person, np.int64), np.asarray(item, np.int64)
    y = np.asarray(y, dtype=float)
    n_persons = int(n_persons or person.max() + 1)
    n_items = int(n_items or item.max() + 1)
    if init is not None:
        theta, log_a, d = init["theta"].copy(), init["log_a"].copy(), init["d"].copy()
    else:
        p_item = (np.bincount(item, y, n_items) + 1) / (np.bincount(item, minlength=n_items) + 2)
        adjusted = np.clip((p_item - c) / (1 - c), .02, .98)
        d = np.log(adjusted / (1 - adjusted))
        p_person = (np.bincount(person, y, n_persons) + 1) / (np.bincount(person, minlength=n_persons) + 2)
        theta = np.log(p_person / (1 - p_person))
        theta = (theta - theta.mean()) / max(theta.std(), 1e-6) * .8
        log_a = np.zeros(n_items)
    eps = 1e-9

    def parts() -> tuple[np.ndarray, ...]:
        a = np.exp(log_a)
        z = a[item] * theta[person] + d[item]
        s = expit(z)
        p = np.clip(c + (1 - c) * s, eps, 1 - eps)
        w = (1 - c) * s * (1 - s)  # dP/dz
        return a, z, p, w

    def item_blocks(a: np.ndarray, p: np.ndarray, w: np.ndarray) -> tuple[np.ndarray, ...]:
        q = p * (1 - p)
        fisher = w ** 2 / q
        x = a[item] * theta[person]  # dz/dlog a
        i_dd = np.bincount(item, fisher, n_items) + 1 / d_sd ** 2
        i_ll = np.bincount(item, fisher * x ** 2, n_items) + 1 / log_a_sd ** 2
        i_dl = np.bincount(item, fisher * x, n_items)
        return q, x, i_dd, i_ll, i_dl

    def log_posterior(p: np.ndarray) -> tuple[float, float]:
        loglik = float(np.sum(y * np.log(p) + (1 - y) * np.log(1 - p)))
        return loglik, loglik - .5 * float(np.sum(theta ** 2) + np.sum(d ** 2) / d_sd ** 2
                                           + np.sum(log_a ** 2) / log_a_sd ** 2)

    iterations, converged, previous = 0, False, None
    for iterations in range(1, max_iter + 1):
        a, _, p, w = parts()
        q = p * (1 - p)
        grad = np.bincount(person, (y - p) / q * w * a[item], n_persons) - theta
        info = np.bincount(person, (w * a[item]) ** 2 / q, n_persons) + 1.0
        step_theta = np.clip(grad / info, -1, 1)
        theta += step_theta
        if scale != "none":
            centre = theta.mean()
            spread = np.sqrt(theta.var() + (np.mean(1 / info) if scale == "latent" else 0.0))
            theta = (theta - centre) / spread
            d = d + np.exp(log_a) * centre
            log_a = log_a + np.log(spread)
        a, _, p, w = parts()
        q, x, i_dd, i_ll, i_dl = item_blocks(a, p, w)
        r = (y - p) / q * w
        g_d = np.bincount(item, r, n_items) - d / d_sd ** 2
        g_l = np.bincount(item, r * x, n_items) - log_a / log_a_sd ** 2
        det = i_dd * i_ll - i_dl ** 2
        step_d = np.clip((i_ll * g_d - i_dl * g_l) / det, -.5, .5)
        step_l = np.clip((i_dd * g_l - i_dl * g_d) / det, -.5, .5)
        d += step_d
        log_a += step_l
        change = max(np.abs(step_theta).max(), np.abs(step_d).max(), np.abs(step_l).max())
        if change < tol:
            converged = True
            break
        if iterations % 10 == 0:
            current = log_posterior(parts()[2])[1]
            if previous is not None and abs(current - previous) < 1e-8 * abs(current):
                converged = True
                break
            previous = current
    a, _z, p, w = parts()
    loglik, log_post = log_posterior(p)
    q, x, i_dd, i_ll, i_dl = item_blocks(a, p, w)
    det = i_dd * i_ll - i_dl ** 2
    var_d, var_l, cov_dl = i_ll / det, i_dd / det, -i_dl / det
    b = -d / a
    # Delta method for b = -d exp(-log a).
    var_b = var_d / a ** 2 + b ** 2 * var_l + 2 * b * cov_dl / a
    info_theta = np.bincount(person, (w * a[item]) ** 2 / q, n_persons) + 1.0
    return {"theta": theta, "theta_se": 1 / np.sqrt(info_theta), "log_a": log_a, "a": a, "d": d,
            "b": b, "b_se": np.sqrt(np.maximum(var_b, 0)), "log_a_se": np.sqrt(var_l), "c": float(c),
            "loglik": loglik, "log_posterior": log_post, "iterations": iterations,
            "converged": converged}


def bias_correct_irt(fit: dict[str, Any], person: np.ndarray, item: np.ndarray,
                     rng: np.random.Generator, reps: int = 5) -> dict[str, Any]:
    """Parametric-bootstrap bias correction of JML item parameters on the observed design.

    Responses are simulated from the fitted model (same persons, items and
    design), refitted with the same c, and the first-order bias of (log a, d) is
    removed: corrected = 2 * estimate - mean(replicates). Returns corrected a,
    b, d, log_a plus the replicate arrays (for SEs/diagnostics).
    """
    c = fit["c"]
    eta = fit["a"][item] * fit["theta"][person] + fit["d"][item]
    probability = c + (1 - c) * expit(eta)
    boot_d, boot_l = [], []
    for _ in range(reps):
        y_sim = (rng.random(len(probability)) < probability).astype(float)
        refit = fit_irt(person, item, y_sim, c, n_persons=len(fit["theta"]), n_items=len(fit["a"]), init=fit)
        boot_d.append(refit["d"])
        boot_l.append(refit["log_a"])
    boot_d, boot_l = np.array(boot_d), np.array(boot_l)
    d = 2 * fit["d"] - boot_d.mean(axis=0)
    log_a = 2 * fit["log_a"] - boot_l.mean(axis=0)
    a = np.exp(log_a)
    return {"a": a, "b": -d / a, "d": d, "log_a": log_a, "boot_d": boot_d, "boot_log_a": boot_l,
            "boot_b": -boot_d / np.exp(boot_l)}


def theta_map(item_a: np.ndarray, item_b: np.ndarray, c: float, y: np.ndarray, group: np.ndarray,
              n_groups: int, prior_sd: float = 3.0, iters: int = 50) -> tuple[np.ndarray, np.ndarray]:
    """MAP ability per group (e.g. per window) with fixed item parameters."""
    theta = np.zeros(n_groups)
    for _ in range(iters):
        s = expit(item_a * (theta[group] - item_b))
        p = np.clip(c + (1 - c) * s, 1e-9, 1 - 1e-9)
        w = (1 - c) * s * (1 - s) * item_a
        grad = np.bincount(group, (y - p) / (p * (1 - p)) * w, n_groups) - theta / prior_sd ** 2
        info = np.bincount(group, w ** 2 / (p * (1 - p)), n_groups) + 1 / prior_sd ** 2
        step = np.clip(grad / info, -1, 1)
        theta += step
        if np.abs(step).max() < 1e-6:
            break
    s = expit(item_a * (theta[group] - item_b))
    p = np.clip(c + (1 - c) * s, 1e-9, 1 - 1e-9)
    w = (1 - c) * s * (1 - s) * item_a
    info = np.bincount(group, w ** 2 / (p * (1 - p)), n_groups) + 1 / prior_sd ** 2
    return theta, 1 / np.sqrt(info)


def fit_lognormal_rt(person: np.ndarray, item: np.ndarray, log_t: np.ndarray,
                     n_persons: int | None = None, n_items: int | None = None,
                     max_iter: int = 200, tol: float = 1e-6) -> dict[str, Any]:
    """Alternating WLS for log t = beta_j - tau_i + e_ij, e ~ N(0, sigma_j^2)."""
    person, item = np.asarray(person, np.int64), np.asarray(item, np.int64)
    log_t = np.asarray(log_t, dtype=float)
    n_persons = int(n_persons or person.max() + 1)
    n_items = int(n_items or item.max() + 1)
    n_j = np.bincount(item, minlength=n_items).astype(float)
    tau = np.zeros(n_persons)
    sigma2 = np.ones(n_items)
    beta = np.bincount(item, log_t, n_items) / np.maximum(n_j, 1)
    iterations = 0
    for iterations in range(1, max_iter + 1):
        beta_new = np.bincount(item, log_t + tau[person], n_items) / np.maximum(n_j, 1)
        resid = log_t - beta_new[item] + tau[person]
        sigma2 = np.bincount(item, resid ** 2, n_items) / np.maximum(n_j - 1, 1)
        weight = 1 / sigma2[item]
        tau_new = (np.bincount(person, weight * (beta_new[item] - log_t), n_persons)
                   / np.bincount(person, weight, n_persons))
        shift = tau_new.mean()
        tau_new -= shift
        beta_new -= shift
        change = max(np.abs(tau_new - tau).max(), np.abs(beta_new - beta).max())
        tau, beta = tau_new, beta_new
        if change < tol:
            break
    sigma = np.sqrt(sigma2)
    tau_se = 1 / np.sqrt(np.bincount(person, 1 / sigma2[item], n_persons))
    return {"beta": beta, "sigma": sigma, "beta_se": sigma / np.sqrt(np.maximum(n_j, 1)),
            "tau": tau, "tau_se": tau_se, "iterations": iterations}


def noise_corrected_sd(estimates: np.ndarray, se: np.ndarray) -> float:
    return float(np.sqrt(max(0.0, np.var(estimates) - np.mean(np.square(se)))))


def lognormal_moments(values: np.ndarray) -> dict[str, float]:
    logs = np.log(np.maximum(np.asarray(values, dtype=float), 1.0))
    return {"log_ms_mean": float(logs.mean()), "log_sd": float(logs.std(ddof=1)), "n": len(logs)}
