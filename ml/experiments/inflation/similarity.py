"""Post hoc stage ``similarity``: a classical answer-similarity statistic (AnswerSim).

Definition (pre-specified in the amendment of 15 Nov 2026, not tuned). For attempts i, j of
different students in the same cohort on the same quiz/variant sharing an item set S with
|S| >= 10, m_k = 1 if both final selected options on item k are identical. The null match
probability is p_k = sum_o f_ko^2, with f_ko the relative frequency of option o on item k
among all attempts of the TRAIN cohorts (no labels used).

    z_ij = (sum_k m_k - sum_k p_k) / sqrt(sum_k p_k (1 - p_k))

Attempt score = max_j z_ij; attempts without an eligible partner get ``NO_PARTNER_SCORE``,
which lies below every attainable z. Secondary variant ``answersim_incorrect``: the same
statistic with the sum restricted to items where attempt i's final answer is incorrect
("identical incorrect answers"). The answer key is not in the event log; it is recovered from
the submitted score (number of correct final answers) and verified exactly on every attempt.
Submission times are never used.
"""
from __future__ import annotations

import time
from collections import Counter, defaultdict
from typing import Any

import numpy as np

from ml.anomaly.features import item_key
from ml.anomaly.train import choose_threshold

from .common import FPR_TARGETS, relative
from .data import Dataset, IntegrityError
from .pipeline import (
    SPLIT_CODES,
    evaluate_table,
    load_scores,
    make_manifest,
    save_scores,
    split_column,
)

MIN_SHARED_ITEMS = 10
NO_PARTNER_SCORE = -1e9
DETECTORS = ("answersim", "answersim_incorrect")


def option_frequencies(attempts: list[dict[str, Any]]) -> dict[str, dict[str, float]]:
    """f_ko: relative frequency of each final option per item over the given attempts."""
    counts: dict[str, Counter] = defaultdict(Counter)
    for attempt in attempts:
        for question, answer in attempt["final_hashes"].items():
            counts[item_key(attempt, question)][answer] += 1
    return {k: {o: n / sum(c.values()) for o, n in c.items()} for k, c in counts.items()}


def null_match_probability(frequencies: dict[str, dict[str, float]]) -> dict[str, float]:
    return {k: float(sum(f * f for f in values.values())) for k, values in frequencies.items()}


def infer_keys(attempts: list[dict[str, Any]]) -> dict[str, str]:
    """Answer key per item from the submitted scores; raises unless it reproduces every score.

    Candidate: the option whose choosers have the highest mean number of correct answers.
    The candidate key is accepted only if, for every attempt, the number of final answers equal
    to the key equals the submitted score (score_percent * items / 100).
    """
    by_item: dict[str, dict[str, list[float]]] = defaultdict(lambda: defaultdict(list))
    correct = []
    for attempt in attempts:
        n = len(attempt["final_hashes"])
        count = attempt["score_percent"] * n / 100
        if abs(count - round(count)) > 1e-6:
            raise IntegrityError(f"{attempt['attempt_id']}: score is not an integer count")
        correct.append(round(count))
        for question, answer in attempt["final_hashes"].items():
            by_item[item_key(attempt, question)][answer].append(count)
    keys = {k: max(options, key=lambda o: (np.mean(options[o]), o)) for k, options in by_item.items()}
    for attempt, count in zip(attempts, correct, strict=True):
        hits = sum(answer == keys[item_key(attempt, q)] for q, answer in attempt["final_hashes"].items())
        if hits != count:
            raise IntegrityError(f"answer key inferred from scores does not reproduce the score of "
                                 f"{attempt['attempt_id']} ({hits} != {count})")
    return keys


def pair_z(i_answers: np.ndarray, j_answers: np.ndarray, p: np.ndarray, mask: np.ndarray) -> float | None:
    """z of one pair over the items in ``mask`` (helper for tests and documentation)."""
    m = (i_answers == j_answers) & mask
    expected, variance = float(p[mask].sum()), float((p * (1 - p))[mask].sum())
    if variance <= 0:
        return None
    return (float(m.sum()) - expected) / np.sqrt(variance)


def answer_similarity(attempts: list[dict[str, Any]], null_p: dict[str, float],
                      keys: dict[str, str] | None = None, min_items: int = MIN_SHARED_ITEMS
                      ) -> dict[str, Any]:
    """Scores (attempt order) for AnswerSim and, if ``keys`` is given, AnswerSim-incorrect.

    Items without a null probability (never answered in the train cohorts) are not part of S.
    A pair is eligible when |S| >= ``min_items`` and the variance term is positive (for the
    incorrect-only variant: positive over i's incorrect items in S).
    """
    blocks: dict[tuple, list[int]] = defaultdict(list)
    for index, attempt in enumerate(attempts):
        blocks[(attempt["group_id"], attempt["quiz_id"], attempt["variant_id"])].append(index)
    scores = {name: np.full(len(attempts), NO_PARTNER_SCORE) for name in DETECTORS if keys or name == "answersim"}
    shared_sizes, partners = np.zeros(len(attempts), dtype=int), np.zeros(len(attempts), dtype=int)
    for indices in blocks.values():
        items = sorted({item_key(attempts[i], q) for i in indices for q in attempts[i]["final_hashes"]})
        column = {k: c for c, k in enumerate(items)}
        codes = np.full((len(indices), len(items)), -1, dtype=np.int64)
        wrong = np.zeros((len(indices), len(items)), dtype=bool)
        vocabulary: dict[str, int] = {}
        for row, i in enumerate(indices):
            for question, answer in attempts[i]["final_hashes"].items():
                key = item_key(attempts[i], question)
                codes[row, column[key]] = vocabulary.setdefault(answer, len(vocabulary))
                if keys is not None:
                    wrong[row, column[key]] = answer != keys[key]
        p = np.array([null_p.get(k, np.nan) for k in items])
        known = np.isfinite(p)
        p = np.where(known, p, 0.0)
        q = p * (1 - p)
        present = (codes >= 0) & known
        users = np.array([attempts[i]["user_id"] for i in indices])
        for row, i in enumerate(indices):
            shared = present & present[row]                       # (partners, items)
            match = shared & (codes == codes[row])
            other = users != users[row]
            size = shared.sum(axis=1)
            base = other & (size >= min_items)
            shared_sizes[i] = int(size[other].max()) if other.any() else 0
            partners[i] = int(base.sum())
            variants = {"answersim": shared}
            if keys is not None:
                variants["answersim_incorrect"] = shared & wrong[row]
            for name, items_mask in variants.items():
                variance = (items_mask * q).sum(axis=1)
                eligible = base & (variance > 0)
                if not eligible.any():
                    continue
                z = (((match & items_mask).sum(axis=1) - (items_mask * p).sum(axis=1))[eligible]
                     / np.sqrt(variance[eligible]))
                scores[name][i] = float(z.max())
    return {"scores": scores, "max_shared_items": shared_sizes, "eligible_partners": partners}


def eligibility_summary(result: dict[str, Any], split: np.ndarray) -> dict[str, Any]:
    test = split == SPLIT_CODES["test"]
    sizes, partners = result["max_shared_items"], result["eligible_partners"]
    out = {"min_shared_items": MIN_SHARED_ITEMS,
           "max_shared_items_with_another_student": {
               "min": int(sizes.min()), "median": float(np.median(sizes)), "max": int(sizes.max())},
           "eligible_partners": {"min": int(partners.min()), "median": float(np.median(partners)),
                                 "max": int(partners.max())},
           "share_with_eligible_partner": float(np.mean(partners > 0)),
           "share_with_eligible_partner_test": float(np.mean(partners[test] > 0)) if test.any() else None}
    for name, values in result["scores"].items():
        out[f"share_no_partner_score:{name}"] = float(np.mean(values == NO_PARTNER_SCORE))
    return out


def similarity_worker(seed: int, cfg: dict[str, Any]) -> list[dict[str, Any]]:
    from .data import assert_paired
    from .run import _cleanup, _data_record, _datasets, _run_dir
    started = time.perf_counter()
    datasets = _datasets(seed, cfg, tuple(cfg["conditions"]))
    manifests = {c: make_manifest(d, seed) for c, d in datasets.items()}
    if "SIM-FI" in datasets and "SIM-BL" in datasets:
        assert_paired(datasets["SIM-FI"], datasets["SIM-BL"], manifests["SIM-FI"], manifests["SIM-BL"])
    order = [c for c in cfg["conditions"] if c in datasets]
    runs = []
    for condition in order:
        dataset = datasets[condition]
        runs.append(_similarity_run(seed, condition, dataset, manifests[condition], cfg,
                                    _run_dir(cfg, condition, f"seed_{seed}") / "scores.npz", _data_record))
    _cleanup(cfg, datasets)
    for run in runs:
        run["wall_seconds"] = time.perf_counter() - started
    return runs


def similarity_frame(dataset: Dataset, manifest: dict[str, Any]):
    """Identifier/label table in attempt order (the columns ``evaluate_table`` needs)."""
    import pandas as pd
    frame = pd.DataFrame({k: [a[k] for a in dataset.attempts] for k in (
        "attempt_id", "user_id", "group_id", "is_anomaly", "anomaly_type", "is_final_attempt")})
    frame["split"] = split_column(frame, manifest)
    frame["is_anomaly"] = frame.is_anomaly.astype(int)
    frame["is_final_attempt"] = frame.is_final_attempt.astype(bool)
    if dataset.truth is not None:
        truth = dataset.truth.reindex(frame.attempt_id)
        for column in truth.columns:
            frame[f"truth_{column}"] = truth[column].to_numpy()
    return frame


def score_dataset(dataset: Dataset, manifest: dict[str, Any]) -> tuple[Any, dict[str, Any], dict[str, Any]]:
    frame = similarity_frame(dataset, manifest)
    train_groups = set(manifest["groups"]["train"])
    train_attempts = [a for a in dataset.attempts if a["group_id"] in train_groups]
    null_p = null_match_probability(option_frequencies(train_attempts))
    try:
        keys = infer_keys(dataset.attempts)
        key_check = {"verified": True, "items": len(keys)}
    except IntegrityError as error:
        keys, key_check = None, {"verified": False, "error": str(error)}
    result = answer_similarity(dataset.attempts, null_p, keys)
    return frame, result, key_check


def _similarity_run(seed: int, condition: str, dataset: Dataset, manifest: dict[str, Any],
                    cfg: dict[str, Any], cache, data_record) -> dict[str, Any]:
    frame, result, key_check = score_dataset(dataset, manifest)
    split = frame.split.to_numpy()
    validation = split == SPLIT_CODES["validation"]
    labels = frame.is_anomaly.to_numpy(dtype=int)
    points = {name: {str(rate): choose_threshold(labels[validation], values[validation], rate)
                     for rate in FPR_TARGETS} for name, values in result["scores"].items()}
    thresholds = {name: {rate: float(p["threshold"]) for rate, p in by_rate.items()}
                  for name, by_rate in points.items()}
    save_scores(cache, frame, result["scores"], thresholds,
                {"csv_sha256": manifest["csv_sha256"], "seed": seed, "condition": condition,
                 "operating_points": points})
    table, scores, _ = load_scores(cache)
    return {"seed": seed, "condition": condition, "data": data_record(dataset), "split_counts": manifest["counts"],
            "operating_points": points, "score_cache": relative(cache),
            "eligibility": {**eligibility_summary(result, split), "answer_key": key_check},
            "metrics": evaluate_table(table, scores, thresholds, seed, with_breakdowns=True)}
