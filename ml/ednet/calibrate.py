# ml/ednet/calibrate.py
"""Calibrate the latent learner model on EdNet calibration users.

Input is the response table of ``ml.ednet.extract``; only calibration users
(hash split, ``common.is_calibration_user``) are read - evaluation users are
removed before any estimate. Calibration users are further split by a second
hash into FIT users (80 %, every estimate) and HOLDOUT users (20 %, used only
for the posterior-predictive fit diagnostics). Excluded sources and repeat
exposures are dropped (``common.usable_mask``).

Estimators (details and diagnostics in the generated report):
* rapid guessing - normative threshold NT10 (Wise & Ma 2012): 10 % of the
  item's mean first-answer time (on-task responses <= max-gap), capped at 10 s;
  sensitivity for NT5/15/20 and fixed thresholds. Per-person rapid counts ->
  zero-inflated beta-binomial MLE (``person.disengage``).
* IRT - joint MAP 3PL with one common c chosen by profile log posterior over a
  grid, non-rapid first answers, items >= 500 and persons >= 30 responses,
  parametric-bootstrap bias correction of the item parameters.
* response times - lognormal model log t = beta_j - tau_i + e on non-rapid,
  on-task responses; tau_sd and theta_tau_corr corrected for estimation noise.
* revisions, gaps, review, practice gain - see the report.

Outputs: ``ml/ednet/calibration.json`` (sim/params/default.json schema; only
aggregate item and population parameters) and ``ml/ednet/calibration_report.md``.

Example: .venv/bin/python -m ml.ednet.calibrate
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import time
from pathlib import Path
from typing import Any

import numpy as np
from scipy import stats
from scipy.special import expit

from ml.ednet.common import (
    DEFAULT_MAX_GAP_MS,
    DERIVED,
    EXCLUDED_SOURCES,
    HOLDOUT_SALT,
    SOURCES,
    file_sha256,
    find_windows,
    hash_array,
    is_calibration_user,
    load_table,
    subset,
    usable_mask,
)
from ml.ednet.models import (
    bias_correct_irt,
    fit_beta_binomial,
    fit_irt,
    fit_lognormal_rt,
    lognormal_moments,
    noise_corrected_sd,
    theta_map,
)
from ml.ednet.simulate import simulate_windows

OUT_JSON = Path("ml/ednet/calibration.json")
OUT_REPORT = Path("ml/ednet/calibration_report.md")
C_GRID = (0.0, 0.05, 0.10, 0.125, 0.15, 0.175, 0.20, 0.25, 0.30)
MIN_ITEM_RESPONSES, MIN_PERSON_RESPONSES = 500, 30
MIN_RATE_RESPONSES = 20  # persons used for per-person rate distributions
NT_CAP_MS = 10_000
HOLDOUT_SHARE = 0.2


def r4(x: float) -> float:
    return float(round(float(x), 4))


def ci(values: np.ndarray) -> list[float]:
    values = np.asarray(values, dtype=float)
    return [r4(np.quantile(values, .025)), r4(np.quantile(values, .975))]


def user_bootstrap(user: np.ndarray, rng: np.random.Generator, reps: int):
    """Yield per-row weights for a bootstrap that resamples users."""
    _, inverse = np.unique(user, return_inverse=True)
    n = inverse.max() + 1
    for _ in range(reps):
        counts = np.bincount(rng.integers(0, n, n), minlength=n)
        yield counts[inverse].astype(float)


def item_thresholds(question: np.ndarray, times: np.ndarray, n_questions: int, fraction: float,
                    max_gap_ms: int) -> np.ndarray:
    on_task = times <= max_gap_ms
    count = np.bincount(question[on_task], minlength=n_questions)
    total = np.bincount(question[on_task], times[on_task], minlength=n_questions)
    global_mean = total.sum() / count.sum()
    mean = np.where(count >= 30, total / np.maximum(count, 1), global_mean)
    return np.minimum(fraction * mean, NT_CAP_MS)


def per_user_counts(user: np.ndarray, flag: np.ndarray) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
    users, inverse = np.unique(user, return_inverse=True)
    return users, np.bincount(inverse, flag.astype(float)).astype(int), np.bincount(inverse)


def dense(values: np.ndarray) -> tuple[np.ndarray, np.ndarray]:
    keys, inverse = np.unique(values, return_inverse=True)
    return keys, inverse


def visible_delays(t: dict[str, np.ndarray], rows: np.ndarray) -> np.ndarray:
    """Delay of every change event from the previous visible answer event."""
    mask = np.zeros(len(t["user"]), dtype=bool)
    mask[rows] = True
    row_of = np.repeat(np.arange(len(t["user"])), t["n_resp"])
    ans, ts = t["resp_ans"], t["resp_ts"]
    first = np.zeros(len(ans), dtype=bool)
    first[t["resp_start"]] = True
    visible = first.copy()
    visible[1:] |= ans[1:] != ans[:-1]
    visible &= mask[row_of]
    v_ts, v_first = ts[visible], first[visible]
    delay = np.diff(v_ts)
    return delay[~v_first[1:]]


def fit_irt_profile(person: np.ndarray, item: np.ndarray, y: np.ndarray, grid: tuple[float, ...],
                    log: list[str]) -> tuple[dict[str, Any], list[dict[str, float]]]:
    profile, fits, init = [], {}, None
    for c in sorted(grid, key=lambda v: abs(v - .15)):
        started = time.time()
        fit = fit_irt(person, item, y, c, init=init)
        init = fit
        fits[c] = fit
        profile.append({"c": c, "log_posterior": fit["log_posterior"], "loglik": fit["loglik"],
                        "converged": fit["converged"], "iterations": fit["iterations"],
                        "median_a": float(np.median(fit["a"])), "median_b": float(np.median(fit["b"]))})
        log.append(f"IRT c={c:.3f}: log posterior {fit['log_posterior']:.1f} "
                   f"({fit['iterations']} it, {time.time() - started:.0f}s)")
    profile.sort(key=lambda p: p["c"])
    best = max(profile, key=lambda p: p["log_posterior"])["c"]
    return fits[best], profile


def window_features(first_times: np.ndarray, final_correct: np.ndarray, n_changes: np.ndarray,
                    duration: np.ndarray, rapid: np.ndarray) -> dict[str, np.ndarray]:
    """Per-window features mirroring ml/anomaly/features.py (times in ms)."""
    return {
        "mean_answer_time_ms": first_times.mean(axis=1),
        "median_answer_time_ms": np.median(first_times, axis=1),
        "p10_answer_time_ms": np.quantile(first_times, .1, axis=1),
        "answer_time_variance_ms2": first_times.var(axis=1),
        "score": final_correct.sum(axis=1).astype(float),
        "changes_count": n_changes.sum(axis=1).astype(float),
        "duration_ms": duration.astype(float),
        "rapid_share": rapid.mean(axis=1),
    }


LOG_SCALE = {"mean_answer_time_ms", "median_answer_time_ms", "p10_answer_time_ms",
             "answer_time_variance_ms2", "duration_ms", "first_answer_time_ms"}


def compare(real: np.ndarray, sim: np.ndarray, name: str) -> dict[str, Any]:
    transform = np.log if name in LOG_SCALE else (lambda v: v)
    qs = (.05, .25, .5, .75, .95)
    return {"feature": name, "n_real": len(real), "n_sim": len(sim),
            "ks": r4(stats.ks_2samp(real, sim).statistic),
            "wasserstein": r4(stats.wasserstein_distance(transform(np.maximum(real, 1e-9)),
                                                         transform(np.maximum(sim, 1e-9)))),
            "scale": "log" if name in LOG_SCALE else "raw",
            "real_mean": r4(np.mean(real)), "sim_mean": r4(np.mean(sim)),
            "real_q": [r4(v) for v in np.quantile(real, qs)],
            "sim_q": [r4(v) for v in np.quantile(sim, qs)]}


def calibrate(table_path: Path, out_json: Path, out_report: Path, max_gap_ms: int = DEFAULT_MAX_GAP_MS,
              seed: int = 20261114, bootstrap: int = 200, irt_bootstrap: int = 5,
              sim_reps: int = 3, max_attempts: int = 5, min_a: float = 0.25,
              max_abs_b: float = 4.0) -> dict[str, Any]:
    started = time.time()
    rng = np.random.default_rng(seed)
    log: list[str] = []
    R: dict[str, Any] = {"parameters": {"max_gap_ms": max_gap_ms, "seed": seed, "bootstrap": bootstrap,
                                        "irt_bootstrap": irt_bootstrap, "sim_reps": sim_reps,
                                        "holdout_share": HOLDOUT_SHARE, "c_grid": list(C_GRID),
                                        "min_item_responses": MIN_ITEM_RESPONSES,
                                        "min_person_responses": MIN_PERSON_RESPONSES,
                                        "max_attempts_per_user": max_attempts,
                                        "pool_min_a": min_a, "pool_max_abs_b": max_abs_b}}
    full = load_table(table_path)
    R["table_rows_all_users"] = len(full["user"])
    t = subset(full, full["is_calibration"])
    del full
    if not is_calibration_user(np.unique(t["user"])).all():
        raise AssertionError("Evaluation users leaked into calibration")
    holdout_user = hash_array(HOLDOUT_SALT, t["user"]) < HOLDOUT_SHARE
    usable = usable_mask(t)
    fit = usable & ~holdout_user
    n_q = int(t["question"].max()) + 1
    times = t["first_answer_time_ms"].astype(float)
    on_task = times <= max_gap_ms
    source_names = np.asarray(SOURCES)[t["source"]]
    R["sample"] = {
        "calibration_users": len(np.unique(t["user"])),
        "fit_users": len(np.unique(t["user"][~holdout_user])),
        "holdout_users": len(np.unique(t["user"][holdout_user])),
        "calibration_rows": len(t["user"]),
        "rows_by_source": {s: int((source_names == s).sum()) for s in SOURCES if (source_names == s).any()},
        "excluded_sources": list(EXCLUDED_SOURCES),
        "repeat_rows_excluded": int((t["is_repeat"] & ~np.isin(source_names, EXCLUDED_SOURCES)).sum()),
        "usable_rows": int(usable.sum()), "fit_rows": int(fit.sum()),
        "fit_users_usable": len(np.unique(t["user"][fit])),
        "fit_rows_off_task_gt_max_gap": int((fit & ~on_task).sum()),
    }

    # ---- rapid guessing -------------------------------------------------------------
    thresholds = {}
    sensitivity = []
    for label, fraction in (("NT5", .05), ("NT10", .10), ("NT15", .15), ("NT20", .20)):
        thresholds[label] = item_thresholds(t["question"][fit], times[fit], n_q, fraction, max_gap_ms)
    for label in ("NT5", "NT10", "NT15", "NT20", "fixed_1s", "fixed_2s", "fixed_3s", "fixed_5s"):
        thr = (thresholds[label][t["question"]] if label.startswith("NT")
               else np.full(len(times), float(label.split("_")[1][:-1]) * 1000))
        flag = fit & (times < thr)
        _, k, n = per_user_counts(t["user"][fit], flag[fit])
        sensitivity.append({
            "rule": label, "rapid_share": r4(flag.sum() / fit.sum()),
            "rapid_first_accuracy": r4(t["first_correct"][flag].mean()) if flag.any() else None,
            "nonrapid_first_accuracy": r4(t["first_correct"][fit & ~flag].mean()),
            "users_without_rapid": r4(np.mean(k[n >= MIN_RATE_RESPONSES] == 0)),
            "median_threshold_ms": r4(np.median(thr[fit])),
        })
    R["rapid_sensitivity"] = sensitivity
    nt10 = thresholds["NT10"]
    rapid = times < nt10[t["question"]]
    R["nt10_threshold_ms_quantiles"] = [r4(v) for v in np.quantile(nt10[np.unique(t["question"][fit])],
                                                                    (0, .05, .5, .95, 1))]
    rapid_fit = fit & rapid
    rg = lognormal_moments(times[rapid_fit])
    logs = np.log(np.maximum(times[rapid_fit], 1))
    users_rg = t["user"][rapid_fit]
    boot_mu, boot_sd = [], []
    for w in user_bootstrap(users_rg, rng, bootstrap):
        mu = np.sum(w * logs) / w.sum()
        boot_mu.append(mu)
        boot_sd.append(np.sqrt(np.sum(w * (logs - mu) ** 2) / w.sum()))
    R["rapid_guess"] = {**{k: r4(v) for k, v in rg.items()}, "log_ms_mean_ci": ci(boot_mu),
                        "log_sd_ci": ci(boot_sd),
                        "zero_ms_rows": int((times[rapid_fit] < 1).sum()),
                        "rapid_change_rate": r4((t["n_changes"][rapid_fit] > 0).mean())}
    users_fit, k_rapid, n_fit = per_user_counts(t["user"][fit], rapid[fit])
    keep = n_fit >= MIN_RATE_RESPONSES
    dis = fit_beta_binomial(k_rapid[keep], n_fit[keep], zero_inflated=True)
    dis_boot = [fit_beta_binomial(k_rapid[keep], n_fit[keep], zero_inflated=True, weights=w)
                for w in user_bootstrap(users_fit[keep], rng, max(20, bootstrap // 5))]
    naive_zero = float(np.mean(k_rapid[keep] == 0))
    R["disengage"] = {
        "p_zero": r4(dis["p_zero"]), "beta_a": r4(dis["beta_a"]), "beta_b": r4(dis["beta_b"]),
        "p_zero_ci": ci([b["p_zero"] for b in dis_boot]), "beta_a_ci": ci([b["beta_a"] for b in dis_boot]),
        "beta_b_ci": ci([b["beta_b"] for b in dis_boot]), "n_users": int(keep.sum()),
        "mean_rate_model": r4((1 - dis["p_zero"]) * dis["beta_a"] / (dis["beta_a"] + dis["beta_b"])),
        "mean_rate_observed": r4(k_rapid[keep].sum() / n_fit[keep].sum()),
        "observed_zero_share": r4(naive_zero), "converged": dis["converged"],
        "fit_without_zero_inflation": {k: r4(v) for k, v in fit_beta_binomial(
            k_rapid[keep], n_fit[keep]).items() if k in ("beta_a", "beta_b", "loglik")},
        "loglik": r4(dis["loglik"]),
    }
    log.append(f"rapid guessing done ({time.time() - started:.0f}s)")

    # ---- IRT ------------------------------------------------------------------------
    irt_rows = np.flatnonzero(fit & ~rapid)
    for _ in range(10):
        q_count = np.bincount(t["question"][irt_rows], minlength=n_q)
        _, u_inverse = dense(t["user"][irt_rows])
        u_count = np.bincount(u_inverse)
        ok = (q_count[t["question"][irt_rows]] >= MIN_ITEM_RESPONSES) & (u_count[u_inverse] >= MIN_PERSON_RESPONSES)
        if ok.all():
            break
        irt_rows = irt_rows[ok]
    item_ids, item_idx = dense(t["question"][irt_rows])
    irt_users, person_idx = dense(t["user"][irt_rows])
    y = t["first_correct"][irt_rows].astype(float)
    best, profile = fit_irt_profile(person_idx, item_idx, y, C_GRID, log)
    c_hat = best["c"]
    # Parametric bootstrap on the observed (adaptive) design: bias correction and SEs.
    corrected = bias_correct_irt(best, person_idx, item_idx, rng, irt_bootstrap)
    boot_d, boot_l, b_boot = corrected["boot_d"], corrected["boot_log_a"], corrected["boot_b"]
    a_bc, b_bc, log_a_bc = corrected["a"], corrected["b"], corrected["log_a"]
    # Can the c profile recover c on data simulated from the fit (same adaptive design)?
    p_sim = c_hat + (1 - c_hat) * expit(best["a"][item_idx] * best["theta"][person_idx] + best["d"][item_idx])
    y_sim = (rng.random(len(p_sim)) < p_sim).astype(float)
    _, sim_profile = fit_irt_profile(person_idx, item_idx, y_sim, (0.05, 0.10, 0.15, 0.20, 0.25), log)
    boot_c = [max(sim_profile, key=lambda p: p["log_posterior"])["c"]]
    theta_sd_latent = float(np.sqrt(best["theta"].var() + np.mean(best["theta_se"] ** 2)))
    R["irt"] = {
        "model": "3PL, common fixed c (profile), joint MAP, theta ~ N(0,1) latent scale",
        "rows": len(irt_rows), "items": len(item_ids), "persons": len(irt_users),
        "person_responses_quantiles": [int(v) for v in np.quantile(np.bincount(person_idx), (.1, .5, .9, .99))],
        "profile": [{k: (r4(v) if isinstance(v, float) else v) for k, v in p.items()} for p in profile],
        "c": c_hat, "c_recovered_in_parametric_replicate": boot_c,
        "bias_correction": {
            "median_log_a_bias": r4(np.median(boot_l.mean(axis=0) - best["log_a"])),
            "median_abs_d_bias": r4(np.median(np.abs(boot_d.mean(axis=0) - best["d"]))),
            "slope_b_boot_on_b": r4(np.polyfit(best["b"], np.median(b_boot, axis=0), 1)[0]),
        },
        "median_se_a": r4(np.median(best["log_a_se"] * best["a"])), "median_se_b": r4(np.median(best["b_se"])),
        "median_boot_sd_b": r4(np.median(b_boot.std(axis=0, ddof=1))) if irt_bootstrap > 1 else None,
        "theta_sd_latent_check": r4(theta_sd_latent),
        "theta_hat_sd": r4(best["theta"].std()), "mean_theta_se": r4(np.mean(best["theta_se"])),
        "a_quantiles": [r4(v) for v in np.quantile(a_bc, (.05, .25, .5, .75, .95))],
        "b_quantiles": [r4(v) for v in np.quantile(b_bc, (.05, .25, .5, .75, .95))],
        "items_a_below_0_25": int((a_bc < .25).sum()),
        "items_abs_b_above_4": int((np.abs(b_bc) > 4).sum()),
        "converged": bool(best["converged"]),
    }
    # Fit check: observed vs predicted accuracy by theta decile.
    p_hat = c_hat + (1 - c_hat) * expit(best["a"][item_idx] * best["theta"][person_idx] + best["d"][item_idx])
    decile = np.digitize(best["theta"][person_idx], np.quantile(best["theta"], np.linspace(.1, .9, 9)))
    R["irt"]["calibration_by_theta_decile"] = [
        {"decile": int(g), "observed": r4(y[decile == g].mean()), "predicted": r4(p_hat[decile == g].mean())}
        for g in range(10)]
    log.append(f"IRT done ({time.time() - started:.0f}s)")
    pool_of = np.full(n_q, -1)
    pool_of[item_ids] = np.arange(len(item_ids))

    # ---- response times -------------------------------------------------------------
    rt_rows = irt_rows[on_task[irt_rows]]
    _, rt_u_inverse = dense(t["user"][rt_rows])
    rt_rows = rt_rows[np.bincount(rt_u_inverse)[rt_u_inverse] >= MIN_PERSON_RESPONSES]
    rt_users, rt_person = dense(t["user"][rt_rows])
    rt_item = pool_of[t["question"][rt_rows]]
    log_t = np.log(times[rt_rows])
    rt = fit_lognormal_rt(rt_person, rt_item, log_t, len(rt_users), len(item_ids))
    tau_sd = noise_corrected_sd(rt["tau"], rt["tau_se"])
    resid = log_t - rt["beta"][rt_item] + rt["tau"][rt_person]
    std_resid = resid / rt["sigma"][rt_item]
    # theta-tau correlation on persons present in both fits.
    both, irt_pos, rt_pos = np.intersect1d(irt_users, rt_users, return_indices=True)
    th, th_se = best["theta"][irt_pos], best["theta_se"][irt_pos]
    ta, ta_se = rt["tau"][rt_pos], rt["tau_se"][rt_pos]

    def disattenuated(index: np.ndarray) -> float:
        var_th = th[index].var()
        reliability = var_th / (var_th + np.mean(th_se[index] ** 2))
        sd_theta = np.sqrt(var_th + np.mean(th_se[index] ** 2))
        sd_tau = np.sqrt(max(ta[index].var() - np.mean(ta_se[index] ** 2), 1e-12))
        cov = np.cov(th[index], ta[index])[0, 1]
        return float(np.clip(cov / (reliability * sd_theta * sd_tau), -1, 1))

    all_index = np.arange(len(both))
    rho = disattenuated(all_index)
    rho_boot = [disattenuated(rng.integers(0, len(both), len(both))) for _ in range(bootstrap)]
    tau_boot = []
    for _ in range(bootstrap):
        pick = rng.integers(0, len(rt_users), len(rt_users))
        tau_boot.append(noise_corrected_sd(rt["tau"][pick], rt["tau_se"][pick]))
    many = np.bincount(rt_person)[rt_pos] >= 100
    R["rt"] = {
        "rows": len(rt_rows), "persons": len(rt_users), "items": len(item_ids),
        "beta_quantiles": [r4(v) for v in np.quantile(rt["beta"], (.05, .5, .95))],
        "sigma_quantiles": [r4(v) for v in np.quantile(rt["sigma"], (.05, .5, .95))],
        "median_se_beta": r4(np.median(rt["beta_se"])),
        "tau_hat_sd": r4(rt["tau"].std()), "mean_tau_se2": r4(np.mean(rt["tau_se"] ** 2)),
        "tau_sd": r4(tau_sd), "tau_sd_ci": ci(tau_boot),
        "theta_tau_corr": r4(rho), "theta_tau_corr_ci": ci(rho_boot),
        "theta_tau_raw_corr_all": r4(np.corrcoef(th, ta)[0, 1]),
        "theta_tau_raw_corr_ge100": r4(np.corrcoef(th[many], ta[many])[0, 1]) if many.sum() > 10 else None,
        "persons_ge100": int(many.sum()), "persons_in_both": len(both),
        "std_residual_quantiles": [r4(v) for v in np.quantile(std_resid, (.01, .05, .25, .5, .75, .95, .99))],
        "std_residual_skew": r4(stats.skew(std_resid)), "std_residual_excess_kurtosis": r4(stats.kurtosis(std_resid)),
        "iterations": rt["iterations"],
    }
    log.append(f"RT done ({time.time() - started:.0f}s)")

    # ---- distractors -------------------------------------------------------------------
    # The key is recovered from the table (a first answer flagged correct equals the key).
    key_full = np.full(n_q, -1)
    rows_ok = t["first_correct"]
    key_full[t["question"][rows_ok]] = t["first_answer"][rows_ok]
    wrong_rows = np.flatnonzero(fit & ~rapid & ~t["first_correct"])
    counts = np.zeros((n_q, 4))
    np.add.at(counts, (t["question"][wrong_rows], t["first_answer"][wrong_rows].astype(np.int64)), 1)
    distractors = np.zeros((len(item_ids), 3))
    for j, q in enumerate(item_ids):
        options = [o for o in range(4) if o != key_full[q]]
        share = (counts[q, options] + 1) / (counts[q, options].sum() + 3)
        distractors[j] = np.sort(share)[::-1]
    m = distractors.mean(axis=0)
    total_var = distractors.var(axis=0).sum()
    precision = max(np.sum(m * (1 - m)) / total_var - 1, 1e-3)
    R["distractors"] = {"mean_sorted_shares": [r4(v) for v in m], "dirichlet_precision": r4(precision)}

    # ---- revisions ---------------------------------------------------------------------
    rev_rows = fit & ~rapid
    changed = t["n_changes"] > 0
    first_wrong = ~t["first_correct"]
    p_w = float(changed[rev_rows & first_wrong].mean())
    p_r = float(changed[rev_rows & ~first_wrong].mean())
    users_rev, k_rev, n_rev = per_user_counts(t["user"][rev_rows], changed[rev_rows])
    keep_rev = n_rev >= MIN_RATE_RESPONSES
    revise = fit_beta_binomial(k_rev[keep_rev], n_rev[keep_rev])
    mu_r = revise["beta_a"] / (revise["beta_a"] + revise["beta_b"])
    w2r = float(t["final_correct"][rev_rows & first_wrong & changed].mean())
    second = float((t["n_changes"][rev_rows & changed] >= 2).mean())
    continuation = [r4((t["n_changes"][rev_rows] >= k + 1).sum() / max(1, (t["n_changes"][rev_rows] >= k).sum()))
                    for k in (1, 2, 3)]
    delays = visible_delays(t, np.flatnonzero(rev_rows))
    delay = lognormal_moments(delays[delays > 0])
    boot = {"p_w": [], "p_r": [], "w2r": [], "second": [], "rev_a": [], "rev_b": []}
    for w in user_bootstrap(t["user"][rev_rows], rng, bootstrap):
        cw, fw = changed[rev_rows], first_wrong[rev_rows]
        boot["p_w"].append(np.sum(w * (cw & fw)) / np.sum(w * fw))
        boot["p_r"].append(np.sum(w * (cw & ~fw)) / np.sum(w * ~fw))
        sel = cw & fw
        boot["w2r"].append(np.sum(w[sel] * t["final_correct"][rev_rows][sel]) / np.sum(w[sel]))
        boot["second"].append(np.sum(w[cw] * (t["n_changes"][rev_rows][cw] >= 2)) / np.sum(w[cw]))
    for w in user_bootstrap(users_rev[keep_rev], rng, max(20, bootstrap // 5)):
        fit_b = fit_beta_binomial(k_rev[keep_rev], n_rev[keep_rev], weights=w)
        boot["rev_a"].append(fit_b["beta_a"])
        boot["rev_b"].append(fit_b["beta_b"])
    R["revision"] = {
        "p_change_given_wrong": r4(p_w), "p_change_given_wrong_ci": ci(boot["p_w"]),
        "p_change_given_right": r4(p_r), "p_change_given_right_ci": ci(boot["p_r"]),
        "revise_beta_a": r4(revise["beta_a"]), "revise_beta_a_ci": ci(boot["rev_a"]),
        "revise_beta_b": r4(revise["beta_b"]), "revise_beta_b_ci": ci(boot["rev_b"]),
        "revise_mean": r4(mu_r), "revise_users": int(keep_rev.sum()),
        "change_mult_wrong": r4(p_w / mu_r), "change_mult_right": r4(p_r / mu_r),
        "wrong_to_right": r4(w2r), "wrong_to_right_ci": ci(boot["w2r"]),
        "second_change_prob": r4(second), "second_change_prob_ci": ci(boot["second"]),
        "continuation_p_ge_k_plus_1_given_ge_k": continuation,
        "right_to_wrong_final": r4(1 - t["final_correct"][rev_rows & ~first_wrong & changed].mean()),
        "delay_log_ms_mean": r4(delay["log_ms_mean"]), "delay_log_sd": r4(delay["log_sd"]),
        "delay_n": delay["n"], "delay_zero_dropped": int((delays <= 0).sum()),
        "share_retap_only": r4(((t["n_resp"] > 1) & ~changed)[rev_rows].mean()),
        "change_rate_overall": r4(changed[rev_rows].mean()),
    }
    log.append(f"revisions done ({time.time() - started:.0f}s)")

    # ---- windows: gaps, review, practice gain (fit users) ---------------------------------
    fit_table = subset(t, ~holdout_user)
    win = find_windows(fit_table, max_gap_ms)
    rows = win["rows"]
    ft = fit_table
    gap_full = ft["enter_ts"][rows[:, 1:]] - ft["final_respond_ts"][rows[:, :-1]]
    gap_submit = ft["enter_ts"][rows[:, 1:]] - ft["submit_ts"][rows[:, :-1]]
    submit_lag = ft["submit_ts"][rows] - ft["final_respond_ts"][rows]
    review_vals = submit_lag[:, -1]
    adjacent = ft["ordinal"][rows[:, 1:]] == ft["ordinal"][rows[:, :-1]] + 1
    gap = lognormal_moments(gap_full.ravel())
    review = lognormal_moments(review_vals)
    gap_boot, review_boot = [], []
    for _ in range(bootstrap):
        pick = rng.integers(0, len(rows), len(rows))
        g = np.log(np.maximum(gap_full[pick].ravel(), 1))
        gap_boot.append(g.mean())
        review_boot.append(np.log(np.maximum(review_vals[pick], 1)).mean())
    R["windows_fit"] = {"windows": len(rows), "users": len(np.unique(win["user"])),
                        "adjacent_transition_share": r4(adjacent.mean())}
    R["gap"] = {**{k: r4(v) for k, v in gap.items()}, "log_ms_mean_ci": ci(gap_boot),
                "definition": "next enter - last respond of the previous item (submit lag + submit->enter)",
                "submit_to_enter": {k: r4(v) for k, v in lognormal_moments(gap_submit.ravel()).items()},
                "submit_lag": {k: r4(v) for k, v in lognormal_moments(submit_lag.ravel()).items()},
                "quantiles_s": [r4(v / 1000) for v in np.quantile(gap_full, (.05, .25, .5, .75, .95))]}
    R["review"] = {**{k: r4(v) for k, v in review.items()}, "log_ms_mean_ci": ci(review_boot),
                   "definition": "submit of the 20th bundle - its last respond"}

    # Practice gain: change of window-level ability between consecutive windows.
    a_items, b_items = a_bc, b_bc
    flat = rows.ravel()
    w_id = np.repeat(np.arange(len(rows)), rows.shape[1])
    q_pool = pool_of[ft["question"][flat]]
    ok = (q_pool >= 0) & (ft["first_answer_time_ms"][flat] >= nt10[ft["question"][flat]])
    counts_w = np.bincount(w_id[ok], minlength=len(rows))
    theta_w, se_w = theta_map(a_items[q_pool[ok]], b_items[q_pool[ok]], c_hat,
                              ft["first_correct"][flat][ok].astype(float), w_id[ok], len(rows))
    valid_w = counts_w >= 15
    same_user = win["user"][1:] == win["user"][:-1]
    consecutive = same_user & (win["window_rank"][1:] == win["window_rank"][:-1] + 1)
    pair = consecutive & valid_w[1:] & valid_w[:-1]
    delta = (theta_w[1:] - theta_w[:-1])[pair]
    noise = (se_w[1:] ** 2 + se_w[:-1] ** 2)[pair]
    pair_users = win["user"][1:][pair]
    days = ((ft["enter_ts"][rows[1:, 0]] - ft["submit_ts"][rows[:-1, -1]]) / 86_400_000)[pair]
    gain_boot_mean, gain_boot_sd = [], []
    for w in user_bootstrap(pair_users, rng, bootstrap):
        mean = np.sum(w * delta) / w.sum()
        gain_boot_mean.append(mean)
        gain_boot_sd.append(np.sqrt(max(np.sum(w * (delta - mean) ** 2) / w.sum() - np.sum(w * noise) / w.sum(), 0)))
    same_day = days < 1
    R["practice_gain"] = {
        "pairs": int(pair.sum()), "users": len(np.unique(pair_users)),
        "mean": r4(delta.mean()), "mean_ci": ci(gain_boot_mean),
        "sd_noise_corrected": r4(np.sqrt(max(delta.var() - noise.mean(), 0))), "sd_ci": ci(gain_boot_sd),
        "raw_sd": r4(delta.std()), "mean_noise_var": r4(noise.mean()),
        "median_days_between": r4(np.median(days)),
        "mean_same_day": r4(delta[same_day].mean()) if same_day.any() else None,
        "mean_later_day": r4(delta[~same_day].mean()) if (~same_day).any() else None,
        "pairs_same_day": int(same_day.sum()),
        "estimator": "MAP theta per window (fixed bias-corrected items, N(0, 3^2) prior, >=15 non-rapid "
                     "pool items); gain = theta_{w+1} - theta_w; sd = sqrt(var(delta) - mean(se_w^2 + se_{w+1}^2))",
    }
    log.append(f"windows/gain done ({time.time() - started:.0f}s)")

    # ---- min response time -------------------------------------------------------------
    q_all = np.quantile(times[fit], (.0001, .001, .01))
    R["min_response"] = {"q0.01pct_ms": r4(q_all[0]), "q0.1pct_ms": r4(q_all[1]), "q1pct_ms": r4(q_all[2]),
                         "share_below_500ms": r4(np.mean(times[fit] < 500))}
    min_response_ms = int(max(1, round(q_all[1])))

    # ---- pool retention (post-fit export rule) -----------------------------------------
    # Data-quality rule fixed before any detector result on calibrated data was seen: an item
    # with a < min_a has an essentially flat ICC, so its difficulty is not identified (b runs
    # off to large |b| along the a*b ridge); items with |b| > max_abs_b lie outside the range
    # the learner population informs. Such items are fitted (they keep the person estimates
    # consistent) but not exported; item_prior is computed from the retained pool only.
    low_a = a_bc < min_a
    far_b = np.abs(b_bc) > max_abs_b
    keep = ~low_a & ~far_b
    R["retention"] = {"fitted_items": len(item_ids), "kept": int(keep.sum()),
                      "dropped_any": int((~keep).sum()), "dropped_low_a": int(low_a.sum()),
                      "dropped_abs_b": int(far_b.sum()), "dropped_both": int((low_a & far_b).sum()),
                      "dropped_abs_b_only": int((far_b & ~low_a).sum()),
                      "dropped_low_a_only": int((low_a & ~far_b).sum()),
                      "min_a": min_a, "max_abs_b": max_abs_b,
                      "dropped_items_responses_share": r4(np.isin(item_idx, np.flatnonzero(~keep)).mean())}
    kept = np.flatnonzero(keep)
    pool_ids = item_ids[kept]
    pa, pb, pla = a_bc[kept], b_bc[kept], log_a_bc[kept]
    pbeta, psigma, pdist = rt["beta"][kept], rt["sigma"][kept], distractors[kept]
    pm = pdist.mean(axis=0)
    pprecision = max(np.sum(pm * (1 - pm)) / pdist.var(axis=0).sum() - 1, 1e-3)
    export_of = np.full(n_q, -1)
    export_of[pool_ids] = np.arange(len(pool_ids))

    # ---- assemble parameters -----------------------------------------------------------
    log_sigma = np.log(psigma)
    items = [{"id": f"q{int(q)}", "a": r4(pa[j]), "b": r4(pb[j]), "c": r4(c_hat),
              "beta": r4(pbeta[j]), "sigma": r4(psigma[j]),
              "distractor_weights": [r4(v) for v in pdist[j]]} for j, q in enumerate(pool_ids)]
    for item in items:  # keep rounding from breaking the unit sum
        item["distractor_weights"][-1] = r4(1 - sum(item["distractor_weights"][:-1]))
    pg = R["practice_gain"]
    params = {
        "schema_version": 1,
        "source": {
            "kind": "ednet-kt2-part5",
            "n_users": len(np.union1d(irt_users, rt_users)),
            "n_responses": int(fit.sum()), "n_items": len(items), "n_items_fitted": len(item_ids),
            "items_dropped": {"low_a": R["retention"]["dropped_low_a"],
                              "abs_b": R["retention"]["dropped_abs_b"],
                              "both": R["retention"]["dropped_both"],
                              "total": R["retention"]["dropped_any"]},
            "n_irt_responses": len(irt_rows), "n_rt_responses": len(rt_rows),
            "inputs": {"response_table_sha256": file_sha256(table_path), **_extract_inputs(table_path)},
            "date": dt.date.today().isoformat(),
            "thresholds": {"rapid": "NT10: 10% of item mean first-answer time (on-task), capped at 10 s",
                           "max_gap_ms": max_gap_ms, "min_item_responses": MIN_ITEM_RESPONSES,
                           "min_person_responses": MIN_PERSON_RESPONSES,
                           "pool_retention": (f"export item only if bias-corrected a >= {min_a} and |b| <= "
                                              f"{max_abs_b}; a < {min_a} means b is not identified. Data-quality "
                                              "rule fixed before any detector result on calibrated data was "
                                              "seen; item_prior uses the retained pool only")},
            "note": ("Calibrated on EdNet-KT2 TOEIC Part-5 calibration users (hash split; evaluation users "
                     "never read), fit subset only. EdNet is a self-study tutor, not an exam. Times are "
                     "first-answer times (enter -> first respond). gap = next enter - last respond of the "
                     "previous item (includes the per-item submit tap and reading the explanation). "
                     "EdNet has no end-of-quiz review: review = time from the last respond of the 20th item "
                     "to its submit (the per-item submit lag), i.e. it is NOT a review phase. "
                     "Item parameters are JML/MAP 3PL with parametric-bootstrap bias correction; see "
                     "ml/ednet/calibration_report.md."),
        },
        "person": {
            "theta_sd": 1.0, "tau_sd": r4(tau_sd), "theta_tau_corr": r4(rho),
            "practice_gain_mean": pg["mean"], "practice_gain_sd": pg["sd_noise_corrected"],
            "disengage": {"p_zero": r4(dis["p_zero"]), "beta_a": r4(dis["beta_a"]), "beta_b": r4(dis["beta_b"])},
            "revise": {"beta_a": r4(revise["beta_a"]), "beta_b": r4(revise["beta_b"])},
        },
        "item_prior": {
            "b_mean": r4(pb.mean()), "b_sd": r4(pb.std()),
            "log_a_mean": r4(pla.mean()), "log_a_sd": r4(pla.std()), "c": r4(c_hat),
            "beta_mean": r4(pbeta.mean()), "beta_sd": r4(pbeta.std()),
            "log_sigma_mean": r4(log_sigma.mean()), "log_sigma_sd": r4(log_sigma.std()),
            "distractor_alpha": [r4(v * pprecision) for v in pm],
        },
        "items": items,
        "rapid_guess": {"log_ms_mean": r4(rg["log_ms_mean"]), "log_sd": r4(rg["log_sd"])},
        "revision": {
            "change_mult_wrong": R["revision"]["change_mult_wrong"],
            "change_mult_right": R["revision"]["change_mult_right"],
            "wrong_to_right": r4(w2r), "delay_log_ms_mean": r4(delay["log_ms_mean"]),
            "delay_log_sd": r4(delay["log_sd"]), "second_change_prob": r4(second),
        },
        "gap": {"log_ms_mean": r4(gap["log_ms_mean"]), "log_sd": r4(gap["log_sd"])},
        "review": {"log_ms_mean": r4(review["log_ms_mean"]), "log_sd": r4(review["log_sd"])},
        "min_response_ms": min_response_ms,
    }

    # ---- posterior-predictive diagnostics on holdout windows ---------------------------
    hold = subset(t, holdout_user)
    hw = find_windows(hold, max_gap_ms)
    capped = hw["window_rank"] < max_attempts
    h_rows, h_user, h_rank = hw["rows"][capped], hw["user"][capped], hw["window_rank"][capped]
    h_question = hold["question"][h_rows]
    in_pool_w = export_of[h_question] >= 0
    # Non-pool items (< 500 responses, or dropped by the retention rule) get parameters drawn
    # from the exported item_prior, as the simulator would do for unseen items; the
    # >= 15-pool-item subset limits that dependence.
    extra_q = np.unique(h_question[~in_pool_w])
    ip = params["item_prior"]
    n_extra = len(extra_q)
    ext_index = export_of.copy()
    ext_index[extra_q] = len(pool_ids) + np.arange(n_extra)
    dirichlet = np.sort(rng.dirichlet(ip["distractor_alpha"], n_extra), axis=1)[:, ::-1] if n_extra else np.zeros((0, 3))
    model = {"person": params["person"], "c": c_hat,
             "a": np.r_[pa, np.exp(rng.normal(ip["log_a_mean"], ip["log_a_sd"], n_extra))],
             "b": np.r_[pb, rng.normal(ip["b_mean"], ip["b_sd"], n_extra)],
             "beta": np.r_[pbeta, rng.normal(ip["beta_mean"], ip["beta_sd"], n_extra)],
             "sigma": np.r_[psigma, np.exp(rng.normal(ip["log_sigma_mean"], ip["log_sigma_sd"], n_extra))],
             "distractor_weights": np.vstack([pdist, dirichlet]), "rapid_guess": params["rapid_guess"],
             "revision": params["revision"], "gap": params["gap"], "review": params["review"],
             "min_response_ms": min_response_ms}
    pool_count = in_pool_w.sum(axis=1)
    R["holdout_windows"] = {"windows": len(hw["rows"]), "after_cap": int(capped.sum()),
                            "users": len(np.unique(h_user)),
                            "pool_item_share": r4(in_pool_w.mean()),
                            "windows_all_pool": int((pool_count == h_rows.shape[1]).sum()),
                            "windows_ge15_pool": int((pool_count >= 15).sum()),
                            "non_pool_accuracy_real": r4(hold["first_correct"][h_rows][~in_pool_w].mean()),
                            "pool_accuracy_real": r4(hold["first_correct"][h_rows][in_pool_w].mean())}
    real_times_all = hold["first_answer_time_ms"][h_rows].astype(float)
    real_duration_all = hold["submit_ts"][h_rows[:, -1]] - hold["enter_ts"][h_rows[:, 0]]
    sims = [simulate_windows(model, ext_index[h_question], h_user, h_rank, rng) for _ in range(sim_reps)]
    for label, sel in (("all", np.ones(len(h_rows), dtype=bool)), ("ge15_pool", pool_count >= 15)):
        if not sel.any():
            continue
        real_times = real_times_all[sel]
        real_rapid = real_times < nt10[h_question[sel]]
        real = window_features(real_times, hold["final_correct"][h_rows[sel]], hold["n_changes"][h_rows[sel]],
                               real_duration_all[sel], real_rapid)
        sim_times = np.concatenate([x["first_times"][sel] for x in sims])
        sim_rapid_flag = sim_times < nt10[np.concatenate([h_question[sel]] * sim_reps)]
        sim = window_features(sim_times, np.concatenate([x["final_correct"][sel] for x in sims]),
                              np.concatenate([x["n_changes"][sel] for x in sims]),
                              np.concatenate([x["duration"][sel] for x in sims]), sim_rapid_flag)
        comparisons = [compare(real_times.ravel(), sim_times.ravel(), "first_answer_time_ms")]
        comparisons += [compare(real[k], sim[k], k) for k in real]
        qs = (.01, .05, .1, .25, .5, .75, .9, .95, .99)
        R[f"diagnostics_{label}"] = {"windows": int(sel.sum()), "comparisons": comparisons, "extra": {
            "first_answer_time_quantiles_s": {
                "q": list(qs), "real": [r4(v / 1000) for v in np.quantile(real_times, qs)],
                "sim": [r4(v / 1000) for v in np.quantile(sim_times, qs)]},
            "score_sd": {"real": r4(real["score"].std()), "sim": r4(sim["score"].std())},
            "share_windows_without_change": {"real": r4(np.mean(real["changes_count"] == 0)),
                                             "sim": r4(np.mean(sim["changes_count"] == 0))},
            "corr_score_log_mean_time": {
                "real": r4(np.corrcoef(real["score"], np.log(real["mean_answer_time_ms"]))[0, 1]),
                "sim": r4(np.corrcoef(sim["score"], np.log(sim["mean_answer_time_ms"]))[0, 1])},
            "within_window_sd_log_time": {
                "real": r4(np.mean(np.log(real_times).std(axis=1))),
                "sim": r4(np.mean(np.log(sim_times).std(axis=1)))},
            "first_answer_accuracy": {"real": r4(hold["first_correct"][h_rows[sel]].mean()),
                                      "sim": r4(np.concatenate([x["first_correct"][sel] for x in sims]).mean())},
            "real_window_has_rapid_with_change_share": r4(np.mean(
                (real_rapid & (hold["n_changes"][h_rows[sel]] > 0)).any(axis=1))),
        }}
    R["seconds"] = round(time.time() - started, 1)
    R["log"] = log
    write_params(out_json, params)
    out_report.write_text(render_report(R, params), encoding="utf-8")
    return {"params": params, "results": R}


def _extract_inputs(table_path: Path) -> dict[str, str]:
    meta = Path(table_path).with_suffix(".meta.json")
    if not meta.exists():
        return {}
    inputs = json.loads(meta.read_text(encoding="utf-8"))["inputs"]
    return {"kt2_sha256": inputs["kt2_sha256"], "contents_sha256": inputs["contents_sha256"]}


def write_params(path: Path, params: dict[str, Any]) -> None:
    """Indented JSON with one item per line (keeps the committed file reviewable)."""
    body = {k: v for k, v in params.items() if k != "items"}
    text = json.dumps(body, indent=2, ensure_ascii=False, allow_nan=False)
    items = ",\n".join("    " + json.dumps(i, separators=(", ", ": "), allow_nan=False) for i in params["items"])
    marker = '  "item_prior"'
    position = text.index(marker)
    text = text[:position] + '  "items": [\n' + items + "\n  ],\n" + text[position:]
    json.loads(text)  # must stay valid
    path.write_text(text + "\n", encoding="utf-8")


def render_report(R: dict[str, Any], params: dict[str, Any]) -> str:
    """Markdown report; every number comes from ``R``/``params`` of this run."""
    s, p = R["sample"], params
    lines = ["# EdNet calibration report", "",
             (f"Generated by `python -m ml.ednet.calibrate` on {p['source']['date']} "
             f"({R['seconds']} s). Input response table SHA-256 `{p['source']['inputs']['response_table_sha256']}`."),
             "All numbers below are produced by the script; nothing is hand-edited.", "",
             "## Sample", "",
             "| quantity | value |", "|---|---|"]
    for k in ("calibration_users", "fit_users", "holdout_users", "calibration_rows", "repeat_rows_excluded",
              "usable_rows", "fit_rows", "fit_users_usable", "fit_rows_off_task_gt_max_gap"):
        lines.append(f"| {k} | {s[k]:,} |")
    lines += ["", "Calibration rows by source (before exclusions): "
              + ", ".join(f"{k} {v:,}" for k, v in s["rows_by_source"].items())
              + f". Excluded sources: {', '.join(s['excluded_sources'])}; repeat exposures are excluded "
              "from every source. `diagnosis` (placement test, 0.3 % repeats, no adaptivity within a "
              "learner's first items) is KEPT: it is the most exam-like source.", ""]
    lines += ["## Rapid guessing", "",
              ("Threshold rule NT10 (10 % of the item's mean first-answer time over on-task responses, "
              "capped at 10 s). Item thresholds (ms) min/5%/median/95%/max: "
              f"{R['nt10_threshold_ms_quantiles']}."), "",
              "| rule | rapid share | rapid accuracy | non-rapid accuracy | persons w/o rapid | median thr ms |",
              "|---|---|---|---|---|---|"]
    for r in R["rapid_sensitivity"]:
        lines.append(f"| {r['rule']} | {r['rapid_share']} | {r['rapid_first_accuracy']} | "
                     f"{r['nonrapid_first_accuracy']} | {r['users_without_rapid']} | {r['median_threshold_ms']} |")
    rg, d = R["rapid_guess"], R["disengage"]
    lines += ["", ("Rapid accuracy close to the chance level 0.25 supports the threshold as a "
              "disengagement marker; accuracy well above 0.25 would indicate fast but solution-based answers."), "",
              (f"Rapid-guess time lognormal: log_ms_mean {rg['log_ms_mean']} (95 % CI {rg['log_ms_mean_ci']}), "
              f"log_sd {rg['log_sd']} (CI {rg['log_sd_ci']}), n = {rg['n']:,}; user-cluster bootstrap. "
              f"These are moments of the rapid times, which are right-truncated at the item threshold by "
              f"construction. Change rate after a rapid answer: {rg['rapid_change_rate']}."), "",
              (f"Disengagement (zero-inflated beta-binomial MLE over {d['n_users']:,} fit users with >= "
              f"{MIN_RATE_RESPONSES} responses): p_zero {d['p_zero']} (CI {d['p_zero_ci']}), "
              f"beta_a {d['beta_a']} (CI {d['beta_a_ci']}), beta_b {d['beta_b']} (CI {d['beta_b_ci']}). "
              f"Model mean rapid rate {d['mean_rate_model']} vs observed {d['mean_rate_observed']}; observed "
              f"share of users with zero rapid responses {d['observed_zero_share']} (the beta-binomial part "
              f"also produces zeros, so p_zero is smaller than the raw share). Plain beta-binomial for "
              f"comparison: {d['fit_without_zero_inflation']}."), ""]
    irt = R["irt"]
    lines += ["## IRT (3PL, common c)", "",
              (f"{irt['rows']:,} non-rapid first answers, {irt['items']:,} items (>= {MIN_ITEM_RESPONSES} "
              f"responses), {irt['persons']:,} persons (>= {MIN_PERSON_RESPONSES}); responses per person "
              f"10/50/90/99 %: {irt['person_responses_quantiles']}."), "",
              ("Joint MAP with theta ~ N(0,1) and weak priors (log a ~ N(0,1), intercept d ~ N(0,3^2)). "
              "Without a constraint the joint mode collapses the theta scale (theta shrinks, a inflates), so "
              "after every person step the scale is pinned to mean 0 and var(theta_hat) + mean posterior "
              f"variance = 1. Check: implied latent SD {irt['theta_sd_latent_check']} (theta_hat SD "
              f"{irt['theta_hat_sd']}, mean SE {irt['mean_theta_se']})."), "",
              "Profile over the common c (higher log posterior is better):", "",
              "| c | log posterior | loglik | median a | median b | converged |", "|---|---|---|---|---|---|"]
    for row in irt["profile"]:
        lines.append(f"| {row['c']} | {row['log_posterior']:.1f} | {row['loglik']:.1f} | {row['median_a']} | "
                     f"{row['median_b']} | {row['converged']} |")
    bc = irt["bias_correction"]
    lines += ["", (f"Selected c = {irt['c']}. Parametric replicate (data simulated from the fit on the observed "
              f"adaptive design, then re-profiled on a coarser grid) selected c = "
              f"{irt['c_recovered_in_parametric_replicate']}."), "",
              (f"JML incidental-parameter bias, estimated by {R['parameters']['irt_bootstrap']} parametric "
              f"replicates on the same design: median log-a bias {bc['median_log_a_bias']}, median |d| bias "
              f"{bc['median_abs_d_bias']}, slope of replicate b on fitted b {bc['slope_b_boot_on_b']}. "
              "Reported item parameters are bias-corrected (2 x estimate - replicate mean, on log a and d). "
              f"Median analytic SE: a {irt['median_se_a']}, b {irt['median_se_b']}; median replicate SD of b "
              f"{irt['median_boot_sd_b']}."), "",
              (f"Bias-corrected a 5/25/50/75/95 %: {irt['a_quantiles']}; b: {irt['b_quantiles']}. Items with "
              f"a < 0.25 (flat ICC, possibly mis-keyed or near-floor): {irt['items_a_below_0_25']}; items with "
              f"|b| > 4: {irt['items_abs_b_above_4']}. They are fitted but not exported (see Pool retention)."), "",
              "Observed vs predicted accuracy by theta_hat decile (in-sample):", "",
              "| decile | observed | predicted |", "|---|---|---|"]
    for row in irt["calibration_by_theta_decile"]:
        lines.append(f"| {row['decile']} | {row['observed']} | {row['predicted']} |")
    lines += ["", ("**Why JML/MAP with person parameters under adaptive selection.** Santa chooses items using "
              "the learner's previous responses (including other parts and other skills). Given theta_i, "
              "those responses carry no further information about the Part-5 responses (local independence), "
              "so item selection is ignorable when theta_i is conditioned on as a parameter; the likelihood "
              "used here is exactly that conditional likelihood. A marginal (MML) fit with one N(0,1) "
              "population would instead be misspecified, because the persons who receive an item are "
              "selected on information correlated with theta that is not in the Part-5 likelihood. "
              "Remaining biases: (i) JML incidental-parameter bias for persons with few responses "
              "(corrected to first order by the parametric bootstrap above); (ii) MAP shrinkage of "
              "theta_hat (the latent scale is pinned instead of the estimate scale); (iii) selection "
              "concentrates each item on a narrow theta range, so a and c are weakly identified for "
              "many items and lean on the priors; (iv) unidimensionality and a common c are assumptions."), ""]
    rt = R["rt"]
    rr, ipr = R["retention"], params["item_prior"]
    lines += ["## Pool retention (export rule)", "",
              (f"An item is exported to `items` only if its bias-corrected a >= {rr['min_a']} and |b| <= "
               f"{rr['max_abs_b']}. With a < {rr['min_a']} the ICC is almost flat, so b is not identified "
               "(the fit slides along the a*b ridge to |b| far beyond the population range); |b| > "
               f"{rr['max_abs_b']} lies outside the theta range the data inform. This data-quality rule was "
               "fixed before any detector result on calibrated data was seen (it was added because the Go "
               "simulator range-checks b in [-10, 10] and the dropped items are unidentified anyway). "
               "Dropped items stay in the fit, so person, time and practice-gain estimates are unchanged; "
               "`item_prior` is computed from the retained pool only."), "",
              "| quantity | items |", "|---|---|",
              f"| fitted (>= {MIN_ITEM_RESPONSES} responses) | {rr['fitted_items']:,} |",
              f"| dropped: a < {rr['min_a']} | {rr['dropped_low_a']} |",
              f"| dropped: \\|b\\| > {rr['max_abs_b']} | {rr['dropped_abs_b']} |",
              f"| dropped by both criteria | {rr['dropped_both']} |",
              f"| dropped in total | {rr['dropped_any']} |",
              f"| exported | {rr['kept']:,} |", "",
              (f"Dropped items carried {rr['dropped_items_responses_share']} of the IRT responses. Retained-pool "
               f"item_prior: b {ipr['b_mean']} +/- {ipr['b_sd']}, log a {ipr['log_a_mean']} +/- {ipr['log_a_sd']}, "
               f"beta {ipr['beta_mean']} +/- {ipr['beta_sd']}, log sigma {ipr['log_sigma_mean']} +/- "
               f"{ipr['log_sigma_sd']}, distractor_alpha {ipr['distractor_alpha']}."), ""]
    lines += ["## Response times (lognormal)", "",
              (f"{rt['rows']:,} non-rapid, on-task (<= max gap) first answers of {rt['persons']:,} persons on "
              f"the {rt['items']:,} IRT items. beta 5/50/95 %: {rt['beta_quantiles']} (ln ms); sigma: "
              f"{rt['sigma_quantiles']}; median SE(beta) {rt['median_se_beta']}."), "",
              (f"tau: SD of estimates {rt['tau_hat_sd']}, mean squared SE {rt['mean_tau_se2']}; "
              f"noise-corrected tau_sd = sqrt(var - mean SE^2) = **{rt['tau_sd']}** (CI {rt['tau_sd_ci']})."), "",
              (f"theta-tau correlation: raw {rt['theta_tau_raw_corr_all']} over {rt['persons_in_both']:,} "
              f"persons, raw {rt['theta_tau_raw_corr_ge100']} over the {rt['persons_ge100']:,} persons with "
              f">= 100 timed responses; disattenuated **{rt['theta_tau_corr']}** (CI {rt['theta_tau_corr_ci']}). "
              "Disattenuation: theta_hat is a shrunken (MAP) estimate, approx. r*theta + noise with "
              "r = var(theta_hat)/(var(theta_hat) + mean SE^2), and tau_hat is unbiased, so "
              "corr = cov(theta_hat, tau_hat) / (r * sd_theta * sd_tau) with noise-corrected SDs. "
              "Positive values mean abler learners answer faster."), "",
              (f"Standardised residual quantiles 1/5/25/50/75/95/99 %: {rt['std_residual_quantiles']}; skew "
              f"{rt['std_residual_skew']}, excess kurtosis {rt['std_residual_excess_kurtosis']} "
              "(a normal error has 0/0)."), ""]
    rv = R["revision"]
    lines += ["## Revisions", "",
              ("A change is a later respond whose option differs from the preceding respond (re-taps of the "
              f"same option are not changes; share of non-rapid responses with re-taps only: "
              f"{rv['share_retap_only']}). Non-rapid responses of fit users."), "",
              "| quantity | estimate | 95 % CI |", "|---|---|---|",
              f"| P(>=1 change \\| first wrong) | {rv['p_change_given_wrong']} | {rv['p_change_given_wrong_ci']} |",
              f"| P(>=1 change \\| first right) | {rv['p_change_given_right']} | {rv['p_change_given_right_ci']} |",
              f"| revise Beta a | {rv['revise_beta_a']} | {rv['revise_beta_a_ci']} |",
              f"| revise Beta b | {rv['revise_beta_b']} | {rv['revise_beta_b_ci']} |",
              (f"| wrong_to_right = P(final right \\| changed, first wrong) | {rv['wrong_to_right']} | "
              f"{rv['wrong_to_right_ci']} |"),
              f"| second_change_prob = P(>=2 \\| >=1) | {rv['second_change_prob']} | {rv['second_change_prob_ci']} |",
              "", (f"Beta-binomial over {rv['revise_users']:,} persons (counts of items with >= 1 change); "
              f"E[r] = {rv['revise_mean']}, so change_mult_wrong = P(change|wrong)/E[r] = "
              f"{rv['change_mult_wrong']} and change_mult_right = {rv['change_mult_right']} (E[r] * mult "
              "reproduces the two conditional rates before the 0.95 cap). The r distribution is fitted to the "
              "marginal change counts, so it also absorbs between-person accuracy differences."),
              (f"Continuation P(>=k+1 | >=k) for k = 1, 2, 3: {rv['continuation_p_ge_k_plus_1_given_ge_k']} "
              "(the simulator uses one constant continuation probability). Changes from a right first answer "
              f"end wrong in {rv['right_to_wrong_final']} of cases. Change delay (from the previous visible "
              f"answer event): log_ms_mean {rv['delay_log_ms_mean']}, log_sd {rv['delay_log_sd']} "
              f"(n = {rv['delay_n']:,}; {rv['delay_zero_dropped']} zero delays dropped)."), ""]
    g, rvw, wf = R["gap"], R["review"], R["windows_fit"]
    lines += ["## Gaps and end of attempt", "",
              (f"Fit users' sittings: {wf['windows']:,} windows of 20 from {wf['users']:,} users; share of "
              f"within-window transitions between adjacent bundles (no hidden other-part bundle): "
              f"{wf['adjacent_transition_share']}."), "",
              (f"gap = {g['definition']}: log_ms_mean {g['log_ms_mean']} (CI {g['log_ms_mean_ci']}), log_sd "
              f"{g['log_sd']}; quantiles 5/25/50/75/95 % (s): {g['quantiles_s']}. Components: submit lag "
              f"{g['submit_lag']}; submit -> next enter (the brief's definition) {g['submit_to_enter']}."), "",
              (f"review = {rvw['definition']}: log_ms_mean {rvw['log_ms_mean']} (CI {rvw['log_ms_mean_ci']}), "
              f"log_sd {rvw['log_sd']}. EdNet has no end-of-quiz review; this is only the last per-item "
              "submit lag, chosen so that simulated durations match how `ml.ednet.convert` defines "
              "duration_ms (last bundle's submit - first enter)."), ""]
    pg = R["practice_gain"]
    lines += ["## Practice gain", "",
              (f"{pg['estimator']}. {pg['pairs']:,} consecutive window pairs from {pg['users']:,} users; "
              f"median {pg['median_days_between']} days between windows."), "",
              (f"Mean gain {pg['mean']} (CI {pg['mean_ci']}); raw SD {pg['raw_sd']}, mean noise variance "
              f"{pg['mean_noise_var']}, noise-corrected SD {pg['sd_noise_corrected']} (CI {pg['sd_ci']}). "
              f"Same-day pairs ({pg['pairs_same_day']:,}): mean {pg['mean_same_day']}; later-day pairs: "
              f"{pg['mean_later_day']}. In a tutor, consecutive windows also differ in the items the "
              "recommender chose, so this is a gain on the fitted item scale, not a controlled re-test."), ""]
    mr = R["min_response"]
    lines += ["## Minimum response time", "",
              (f"First-answer time quantiles of fit responses: 0.01 % {mr['q0.01pct_ms']} ms, 0.1 % "
              f"{mr['q0.1pct_ms']} ms, 1 % {mr['q1pct_ms']} ms; share < 500 ms {mr['share_below_500ms']}. "
              f"min_response_ms is set to the 0.1 % quantile ({params['min_response_ms']} ms) so that the floor "
              "binds only on the extreme tail."), ""]
    hwin = R["holdout_windows"]
    lines += ["## Fit diagnostics (posterior predictive, holdout calibration users)", "",
              (f"Holdout users' windows: {hwin['windows']:,}; after the {R['parameters']['max_attempts_per_user']}"
              f"-per-user cap {hwin['after_cap']:,} ({hwin['users']:,} users). Only {hwin['pool_item_share']} of "
              f"their item slots are exported pool items (>= {MIN_ITEM_RESPONSES} responses and passing the "
              "retention rule); "
              f"{hwin['windows_all_pool']} windows consist of pool items only and {hwin['windows_ge15_pool']:,} "
              "have >= 15. Non-pool items get parameters drawn independently from the fitted `item_prior` "
              "(as the simulator would for unseen items). Real first-answer accuracy on pool items "
              f"{hwin['pool_accuracy_real']}, on non-pool items {hwin['non_pool_accuracy_real']}."), "",
              (f"For every real window the model (simulate.py) generates {R['parameters']['sim_reps']} synthetic "
              "windows on the same 20 items, drawing new persons from the fitted population (one person per "
              "real user, practice gain applied across that user's windows). Nothing here was used for fitting."), ""]
    for label, title in (("all", "All holdout windows"), ("ge15_pool", "Windows with >= 15 pool items")):
        block = R.get(f"diagnostics_{label}")
        if block is None:
            continue
        lines += [f"### {title} ({block['windows']:,} real windows)", "",
                  "| feature | scale | KS | W1 | real mean | sim mean | real q5/25/50/75/95 | sim q5/25/50/75/95 |",
                  "|---|---|---|---|---|---|---|---|"]
        for c in block["comparisons"]:
            lines.append(f"| {c['feature']} | {c['scale']} | {c['ks']} | {c['wasserstein']} | {c['real_mean']} | "
                         f"{c['sim_mean']} | {c['real_q']} | {c['sim_q']} |")
        ex = block["extra"]
        fq = ex["first_answer_time_quantiles_s"]
        lines += ["", "First-answer time quantiles (s):", "", "| q | " + " | ".join(map(str, fq["q"])) + " |",
                  "|---" * (len(fq["q"]) + 1) + "|",
                  "| real | " + " | ".join(map(str, fq["real"])) + " |",
                  "| sim | " + " | ".join(map(str, fq["sim"])) + " |", "",
                  "| check | real | sim |", "|---|---|---|"]
        for k in ("score_sd", "share_windows_without_change", "corr_score_log_mean_time",
                  "within_window_sd_log_time", "first_answer_accuracy"):
            lines.append(f"| {k} | {ex[k]['real']} | {ex[k]['sim']} |")
        lines += ["", (f"Real windows with a rapid answer that was later changed (never simulated): "
                  f"{ex['real_window_has_rapid_with_change_share']}."), ""]
    lines += ["W1 = Wasserstein-1 distance on the stated scale (natural log of ms for times).",
              ("Reading guide: KS < 0.05 close match; 0.05-0.15 visible, moderate misfit; > 0.15 substantial "
              "misfit that the paper must acknowledge."), "",
              "## Run log", ""] + [f"- {x}" for x in R["log"]] + [""]
    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--table", type=Path, default=DERIVED)
    parser.add_argument("--output", type=Path, default=OUT_JSON)
    parser.add_argument("--report", type=Path, default=OUT_REPORT)
    parser.add_argument("--max-gap-ms", type=int, default=DEFAULT_MAX_GAP_MS)
    parser.add_argument("--seed", type=int, default=20261114)
    parser.add_argument("--bootstrap", type=int, default=200)
    parser.add_argument("--irt-bootstrap", type=int, default=5)
    parser.add_argument("--sim-reps", type=int, default=3)
    parser.add_argument("--min-a", type=float, default=0.25,
                        help="pool retention: export items with bias-corrected a >= this")
    parser.add_argument("--max-abs-b", type=float, default=4.0,
                        help="pool retention: export items with |b| <= this")
    parser.add_argument("--results-json", type=Path, default=None,
                        help="optional path for the full diagnostics dict (JSON)")
    args = parser.parse_args()
    out = calibrate(args.table, args.output, args.report, args.max_gap_ms, args.seed, args.bootstrap,
                    args.irt_bootstrap, args.sim_reps, min_a=args.min_a, max_abs_b=args.max_abs_b)
    if args.results_json:
        args.results_json.write_text(json.dumps(out["results"], indent=2, default=float), encoding="utf-8")
    print(json.dumps({k: v for k, v in out["params"].items() if k != "items"}, indent=2))


if __name__ == "__main__":
    main()
