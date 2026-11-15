# ml/ednet/simulate.py
"""Minimal Python re-implementation of the honest learner model (fit diagnostics).

Semantics (shared with the parameter file, sim/params/default.json schema):
* person: (theta, tau) bivariate normal with SDs theta_sd, tau_sd and
  correlation theta_tau_corr; disengagement d ~ 0 w.p. p_zero else
  Beta(beta_a, beta_b); revision propensity r ~ Beta(revise.beta_a, beta_b);
  per-person practice gain g ~ N(practice_gain_mean, practice_gain_sd), so in
  the person's k-th attempt (k = 0, 1, ...) ability is theta + k * g.
* item response: with probability d a rapid guess (time ~ lognormal
  rapid_guess, option uniform over the 4 options); otherwise the learner knows
  the answer w.p. logistic(a (theta - b)) and else guesses correctly w.p. c, i.e.
  P(correct) = c + (1 - c) logistic(a (theta - b)); a wrong option is drawn by
  the item's distractor_weights; time ~ lognormal(beta - tau, sigma).
  Every time is floored at min_response_ms.
* revisions (engaged responses only): P(change) = min(.95, r * mult) with
  mult = change_mult_wrong or change_mult_right by the first answer. A change
  from a wrong answer goes to the key w.p. wrong_to_right, else to another
  distractor; a change from the key goes to a distractor. After each change a
  further change follows w.p. second_change_prob. Each change happens after a
  lognormal delay (revision.delay_*) measured from the previous answer event.
* timing within an attempt: item k+1 is shown ``gap`` after the last answer
  event of item k; the attempt is submitted ``review`` after the last answer
  event of the last item.
"""
from __future__ import annotations

from typing import Any

import numpy as np
from scipy.special import expit


def simulate_windows(model: dict[str, Any], items: np.ndarray, person: np.ndarray, rank: np.ndarray,
                     rng: np.random.Generator) -> dict[str, np.ndarray]:
    """Simulate attempts; ``items`` (W x L) indexes the model's item arrays."""
    items = np.asarray(items)
    n_windows, length = items.shape
    persons, person_index = np.unique(person, return_inverse=True)
    n = len(persons)
    p = model["person"]
    cov = [[p["theta_sd"] ** 2, p["theta_tau_corr"] * p["theta_sd"] * p["tau_sd"]],
           [p["theta_tau_corr"] * p["theta_sd"] * p["tau_sd"], p["tau_sd"] ** 2]]
    latent = rng.multivariate_normal([0.0, 0.0], cov, size=n)
    theta, tau = latent[:, 0], latent[:, 1]
    dis = p["disengage"]
    d = np.where(rng.random(n) < dis["p_zero"], 0.0, rng.beta(dis["beta_a"], dis["beta_b"], n))
    r = rng.beta(p["revise"]["beta_a"], p["revise"]["beta_b"], n)
    gain = rng.normal(p["practice_gain_mean"], p["practice_gain_sd"], n)

    th = (theta[person_index] + rank * gain[person_index])[:, None]
    ta = tau[person_index][:, None]
    a, b, c = model["a"][items], model["b"][items], model["c"]
    beta, sigma = model["beta"][items], model["sigma"][items]
    weights = model["distractor_weights"][items]  # W x L x 3
    shape = (n_windows, length)
    rapid = rng.random(shape) < d[person_index][:, None]
    rg = model["rapid_guess"]
    floor = model["min_response_ms"]
    times = np.where(rapid, rng.lognormal(rg["log_ms_mean"], rg["log_sd"], shape),
                     np.exp(beta - ta + sigma * rng.standard_normal(shape)))
    times = np.maximum(times, floor)
    p_correct = c + (1 - c) * expit(a * (th - b))
    correct = np.where(rapid, rng.random(shape) < .25, rng.random(shape) < p_correct)
    # Option 0 = key; 1..3 = distractors in descending weight order.
    cumulative = np.cumsum(weights, axis=2)
    draw = (rng.random(shape)[..., None] > cumulative).sum(axis=2) + 1
    rapid_option = rng.integers(0, 4, shape)
    option = np.where(rapid, rapid_option, np.where(correct, 0, np.minimum(draw, 3)))
    correct = option == 0

    rev = model["revision"]
    mult = np.where(correct, rev["change_mult_right"], rev["change_mult_wrong"])
    p_change = np.minimum(.95, r[person_index][:, None] * mult)
    changed = ~rapid & (rng.random(shape) < p_change)
    n_changes = np.zeros(shape, dtype=np.int64)
    last_time = times.copy()
    final = option.copy()
    for w, k in zip(*np.nonzero(changed), strict=True):
        current, elapsed, count = int(option[w, k]), float(times[w, k]), 0
        while True:
            count += 1
            elapsed += float(rng.lognormal(rev["delay_log_ms_mean"], rev["delay_log_sd"]))
            if current == 0:
                current = int(rng.choice([1, 2, 3], p=weights[w, k]))
            elif rng.random() < rev["wrong_to_right"]:
                current = 0
            else:
                others = [o for o in (1, 2, 3) if o != current]
                share = weights[w, k][[o - 1 for o in others]]
                current = int(rng.choice(others, p=share / share.sum()))
            if rng.random() >= rev["second_change_prob"]:
                break
        n_changes[w, k], last_time[w, k], final[w, k] = count, elapsed, current
    gaps = rng.lognormal(model["gap"]["log_ms_mean"], model["gap"]["log_sd"], (n_windows, length - 1))
    review = rng.lognormal(model["review"]["log_ms_mean"], model["review"]["log_sd"], n_windows)
    duration = last_time.sum(axis=1) + gaps.sum(axis=1) + review
    return {"first_times": times, "first_correct": correct, "final_correct": final == 0,
            "n_changes": n_changes, "rapid": rapid, "duration": duration, "gaps": gaps,
            "review": review, "change_delay_total": last_time - times}
