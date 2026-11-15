"""Post hoc stages added after independent review of the draft (README amendment of 15 Nov 2026).

* ``fi-dose``   injection magnitude λ (generated mechanisms file) × behaviour-level intensity grid
* ``protocol``  label-free threshold, contaminated fitting, final-attempt evaluation; observability
                of injected/behavioural cheating (event stream identical to the honest counterfactual)
* ``baseline``  honest-baseline variants (exam-like revision rate, per-attempt item sampling,
                fixed closing time with the 0.3.0 simulator)
* sensitivity v2 helpers (decorrelated Latin hypercube, injection parameters)
"""
from __future__ import annotations

import json
import re
import shutil
import subprocess
import tempfile
import time
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

from .common import read_json, relative, sha256_file, slot_of, write_json
from .data import Dataset, IntegrityError, assert_paired
from .metrics import full_metrics
from .pipeline import (
    SPLIT_CODES,
    evaluate_table,
    feature_frame,
    fit_frozen,
    load_scores,
    make_manifest,
    operating_points,
    save_scores,
    score_all,
    threshold_table,
)

FI_DOSE_LAMBDAS = (0.25, 0.5, 0.75, 1.0)
FI_DOSE_PARAMETERS = ("rapid_item_prob", "changes_prob_low", "changes_prob_high", "collusion_copy_prob",
                      "score_jump_low", "score_jump_high")
LABEL_FREE_QUANTILE = {"0.05": 0.95, "0.01": 0.99}
PROTOCOL_VARIANTS = ("standard", "label_free", "contaminated")
REVISION_TARGET, REVISION_TOLERANCE = 0.84, 0.005
SIM_START_AT = datetime(2026, 10, 1, 9, 0, tzinfo=UTC)  # examory-sim default --start-at
CLOSING_QUANTILES = (0.90, 0.75)
AUTO_SUBMIT_JITTER_MS = 2000  # examory-sim 0.3.0 default (params.DefaultAutoSubmitJitterMs)
LHS_MAX_ABS_SPEARMAN = 0.10


# --------------------------------------------------------------------------- mechanism / parameter files


def write_fi_dose_mechanisms(lam: float, base: Path, path: Path) -> Path:
    """λ multiplies the injection magnitudes; every other parameter is copied unchanged."""
    mechanisms = read_json(base)
    fi = mechanisms["fi"]
    for name in FI_DOSE_PARAMETERS:
        fi[name] = min(1.0, fi[name] * lam)
    write_json(path, mechanisms)
    return path


def write_revision_params(multiplier: float, base: Path, path: Path) -> Path:
    """Scale the honest revision propensity: p1 = min(0.95, r_i * change_mult_*), so multiplying both
    change multipliers by m is the same as multiplying r_i by m (while the 0.95 cap does not bind)."""
    params = read_json(base)
    params["revision"]["change_mult_wrong"] *= multiplier
    params["revision"]["change_mult_right"] *= multiplier
    write_json(path, params)
    return path


def _generate_honest(binary: Path, params: Path, mechanisms: Path, seed: int, design: dict[str, Any],
                     out: Path, extra: list[str] | None = None) -> Path:
    command = [str(binary), "generate", "--students", str(design["students"]), "--attempts",
               str(design["attempts"]), "--questions", str(design["questions"]), "--anomaly-rate",
               repr(design["anomaly_rate"]), "--seed", str(seed), "--params", str(params),
               "--mechanisms", str(mechanisms), "--worlds", "honest", "--out-dir", str(out), "--force",
               *(extra or [])]
    result = subprocess.run(command, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"generator failed: {' '.join(command)}\n{result.stderr[-2000:]}")
    return out / "honest.csv"


def _submissions(csv: Path) -> pd.DataFrame:
    frame = pd.read_csv(csv, dtype=str, keep_default_na=False, usecols=["event_type", "occurred_at", "payload"])
    frame = frame[frame.event_type == "quiz.submitted"]
    payload = frame.payload.map(json.loads)
    return pd.DataFrame({"attempt_id": payload.map(lambda p: p["attempt_id"]),
                         "changes": payload.map(lambda p: p["answers_changed"]).astype(int),
                         "max_score": payload.map(lambda p: p["max_score"]).astype(int),
                         "submitted_at": pd.to_datetime(frame.occurred_at.to_numpy(), utc=True)})


def honest_changes(csv: Path) -> dict[str, float]:
    sub = _submissions(csv)
    per20 = sub.changes * 20 / sub.max_score
    return {"mean": float(per20.mean()), "p95": float(np.quantile(per20, 0.95)), "attempts": len(sub)}


def revision_multiplier(base_params: Path, mechanisms: Path, binary: Path, seed: int, design: dict[str, Any],
                        workdir: Path) -> dict[str, Any]:
    """Bisection on log m until |mean changes per 20 items − 0.84| <= 0.005 (honest world of ``seed``)."""
    workdir.mkdir(parents=True, exist_ok=True)
    history = []

    def evaluate(m: float) -> dict[str, float]:
        params = write_revision_params(m, base_params, workdir / "params.json")
        out = Path(tempfile.mkdtemp(prefix="revision.", dir=workdir))
        try:
            stats = honest_changes(_generate_honest(binary, params, mechanisms, seed, design, out))
        finally:
            shutil.rmtree(out, ignore_errors=True)
        history.append({"multiplier": m, **stats})
        return stats

    calibrated = evaluate(1.0)
    low, high = 1e-4, 1.0
    for _ in range(60):
        middle = float(np.exp((np.log(low) + np.log(high)) / 2))
        stats = evaluate(middle)
        if abs(stats["mean"] - REVISION_TARGET) <= REVISION_TOLERANCE:
            break
        low, high = (middle, high) if stats["mean"] < REVISION_TARGET else (low, middle)
    else:
        raise RuntimeError("revision-rate bisection did not converge")
    (workdir / "params.json").unlink(missing_ok=True)
    return {"multiplier": middle, "mean_changes_per_20": stats["mean"], "p95_changes_per_20": stats["p95"],
            "calibrated_mean_changes_per_20": calibrated["mean"], "calibrated_p95_changes_per_20": calibrated["p95"],
            "target": REVISION_TARGET, "tolerance": REVISION_TOLERANCE, "seed": seed, "iterations": history}


def natural_submission_quantiles(binary: Path, params: Path, mechanisms: Path, seed: int, design: dict[str, Any],
                                 workdir: Path) -> dict[str, Any]:
    """Quantiles of honest submission time relative to the sitting opening (start-at + k days)."""
    workdir.mkdir(parents=True, exist_ok=True)
    out = Path(tempfile.mkdtemp(prefix="closing.", dir=workdir))
    try:
        csv = _generate_honest(binary, params, mechanisms, seed, design, out)
        sub = _submissions(csv)
        truth = pd.read_csv(out / "truth.csv", dtype=str, keep_default_na=False)
        index = truth[truth.world == "honest"].set_index("attempt_id").attempt_index.astype(int)
        opening = [SIM_START_AT + timedelta(days=int(k)) for k in index.reindex(sub.attempt_id)]
        delta = pd.Series(sub.submitted_at.to_numpy()) - pd.Series(pd.to_datetime(opening, utc=True))
        relative_ms = (delta.dt.total_seconds() * 1000).round().to_numpy()
    finally:
        shutil.rmtree(out, ignore_errors=True)
    return {"seed": seed, "n": len(relative_ms), "median_ms": float(np.median(relative_ms)),
            **{f"q{int(100 * q)}_ms": float(np.quantile(relative_ms, q)) for q in CLOSING_QUANTILES},
            "max_ms": float(np.max(relative_ms))}


# --------------------------------------------------------------------------- per-run extras


def items_affected(dataset: Dataset) -> dict[str, Any]:
    truth = dataset.truth
    if truth is None or "items_affected" not in truth or "mechanism" not in truth:
        return {}
    cheaters = truth[truth.is_anomaly.astype(int) == 1]
    out = {}
    for mechanism, rows in cheaters.groupby("mechanism"):
        values = pd.to_numeric(rows["items_affected"], errors="coerce")
        out[mechanism] = {"n": len(rows), "mean_items_affected": float(values.mean()),
                          "share_zero_items": float((values == 0).mean())}
    return out


def feature_diagnostics(dataset: Dataset, frame: pd.DataFrame, reference: dict[str, Any]) -> dict[str, Any]:
    """How the history-based, item-reference and peer features behave on this dataset."""
    from ml.anomaly.features import item_key
    responses = [item_key(a, q) for a in dataset.attempts for q in a["first_times"]]
    own = np.mean([k in reference["item_fast_ms"] for k in responses])
    by_id = {a["attempt_id"]: a for a in dataset.attempts}
    common, empty = [], 0
    for attempt_id, partners in zip(frame.attempt_id, frame.synchronous_with, strict=True):
        for other in partners:
            if other <= attempt_id:
                continue
            shared = set(by_id[attempt_id]["final_hashes"]) & set(by_id[other]["final_hashes"])
            common.append(len(shared))
            empty += not shared
    z = frame.z_score.to_numpy(dtype=float)
    normal = frame.is_anomaly.to_numpy() == 0
    extra: dict[str, Any] = {"share_normal_attempts_synchronous": float((frame.synchronous_count[normal] > 0).mean())}
    truth = dataset.truth
    if truth is not None and "auto_submitted" in truth:
        auto = truth.reindex(frame.attempt_id)["auto_submitted"].astype(str).to_numpy() == "1"
        extra.update({"share_auto_submitted": float(auto.mean()),
                      "share_auto_submitted_normal": float(auto[normal].mean()),
                      "share_auto_submitted_cheaters": float(auto[~normal].mean()) if (~normal).any() else None})
    return {**extra, "attempts": len(frame), "distinct_items": len(set(responses)),
            "share_history_missing": float(frame.history_missing.mean()),
            "share_z_score_defined": float(np.isfinite(z).mean()),
            "z_score_sd": float(np.nanstd(z)) if np.isfinite(z).any() else None,
            "share_responses_with_item_time_reference": float(own),
            "items_with_time_reference": len(reference["item_fast_ms"]),
            "share_attempts_synchronous": float((frame.synchronous_count > 0).mean()),
            "synchronous_pairs": len(common),
            "mean_common_items_synchronous_pairs": float(np.mean(common)) if common else None,
            "share_synchronous_pairs_without_common_items": empty / len(common) if common else None,
            "mean_max_answer_agreement": float(frame.max_answer_agreement.mean())}


def _attempt_events(csv: Path, ids: set[str]) -> dict[str, list[tuple[str, str, str]]]:
    frame = pd.read_csv(csv, dtype=str, keep_default_na=False,
                        usecols=["event_id", "event_type", "occurred_at", "payload"])
    pattern = re.compile(r'"attempt_id":"([^"]+)"')
    attempt = frame.payload.str.extract(pattern, expand=False)
    keep = attempt.isin(ids)
    events: dict[str, list[tuple[str, str, str]]] = {}
    for a, row in zip(attempt[keep], frame[keep].itertuples(index=False), strict=True):
        events.setdefault(a, []).append((row.event_id, row.event_type, row.occurred_at, row.payload))
    return {k: sorted(v) for k, v in events.items()}


def unobservable_cheating(dataset: Dataset, honest: Dataset) -> dict[str, Any]:
    """Cheater attempts whose event stream equals the honest counterfactual (labels aside).

    Compared per attempt: every event's id, type, timestamp and payload (answer hash, timings,
    score, duration, change count). Such an attempt is labelled cheating but is observationally an
    honest attempt; it stays a positive in every recall denominator.
    """
    cheaters = {a["attempt_id"]: a["anomaly_type"] for a in dataset.attempts if a["is_anomaly"]}
    ours = _attempt_events(dataset.csv, set(cheaters))
    theirs = _attempt_events(honest.csv, set(cheaters))
    by_type: dict[str, dict[str, Any]] = {}
    identical = []
    score = {a["attempt_id"]: a["score_percent"] for a in dataset.attempts}
    honest_score = {a["attempt_id"]: a["score_percent"] for a in honest.attempts}
    zero = {}
    if dataset.truth is not None and "items_affected" in dataset.truth:
        zero = {k: v == "0" for k, v in dataset.truth["items_affected"].items()}
    for attempt_id, kind in cheaters.items():
        row = by_type.setdefault(kind, {"n": 0, "event_identical": 0, "score_unchanged": 0, "zero_items_affected": 0})
        row["n"] += 1
        same = ours.get(attempt_id) == theirs.get(attempt_id)
        row["event_identical"] += same
        row["score_unchanged"] += score[attempt_id] == honest_score[attempt_id]
        row["zero_items_affected"] += bool(zero.get(attempt_id))
        if same:
            identical.append(attempt_id)
    return {"by_type": by_type, "identical_ids": sorted(identical)}


def final_only_metrics(table: pd.DataFrame, scores: dict[str, np.ndarray], thresholds: dict[str, dict[str, float]],
                       seed: int) -> dict[str, dict[str, Any]]:
    """Positives exist only among final attempts: negatives restricted to final normal attempts too."""
    test = (table.split.to_numpy() == SPLIT_CODES["test"]) & table.is_final_attempt.to_numpy(dtype=bool)
    labels = table.is_anomaly.to_numpy(dtype=int)
    types = table.anomaly_type.to_numpy()
    groups = table.group_id.to_numpy()
    subsets = {"final_only:all": test}
    for kind in sorted(set(types[test & (labels == 1)])):
        subsets[f"final_only:slot:{slot_of(kind)}"] = test & ((labels == 0) | (types == kind))
    return {name: {subset: full_metrics(labels[mask], values[mask], thresholds[name], groups[mask], 0, seed, None)
                   for subset, mask in subsets.items()} for name, values in scores.items()}


def _subset_metrics(table, scores, thresholds, seed, name: str, mask: np.ndarray) -> dict[str, Any]:
    labels = table.is_anomaly.to_numpy(dtype=int)
    return {det: full_metrics(labels[mask], values[mask], thresholds[det], table.group_id.to_numpy()[mask], 0,
                              seed, None) for det, values in scores.items()}


# --------------------------------------------------------------------------- protocol stage


def _contaminated(dataset: Dataset, manifest: dict[str, Any], workdir: Path) -> Dataset:
    """Same data, but train-cohort cheaters presented to the frozen pipeline as normal attempts."""
    train = set(manifest["groups"]["train"])
    frame = pd.read_csv(dataset.csv, dtype=str, keep_default_na=False)
    group = frame.context.str.extract(r'"group_id":"([^"]+)"', expand=False)
    mask = group.isin(train) & (frame.is_anomaly == "1")
    frame.loc[mask, "is_anomaly"] = "0"
    frame.loc[mask, "anomaly_type"] = "none"
    workdir.mkdir(parents=True, exist_ok=True)
    path = workdir / "contaminated.csv"
    frame.to_csv(path, index=False)
    attempts = []
    for a in dataset.attempts:
        a = dict(a)
        if a["group_id"] in train and a["is_anomaly"]:
            a["is_anomaly"], a["anomaly_type"] = 0, "none"
        attempts.append(a)
    return Dataset(dataset.condition, path, dataset.seed, attempts, None, {})


def label_free_points(table: pd.DataFrame, scores: dict[str, np.ndarray]) -> dict[str, Any]:
    validation = (table.split.to_numpy() == SPLIT_CODES["validation"]) & (table.is_anomaly.to_numpy() == 0)
    points = {}
    for name, values in scores.items():
        reference = values[validation]
        points[name] = {}
        for rate, q in LABEL_FREE_QUANTILE.items():
            threshold = float(np.quantile(reference, q))
            points[name][rate] = {"threshold": threshold, "validation_fpr": float(np.mean(reference >= threshold)),
                                  "rule": f"numpy.quantile(validation normal scores, {q}) (linear)"}
    return points


def protocol_worker(seed: int, cfg: dict[str, Any]) -> list[dict[str, Any]]:
    from .run import _cleanup, _data_record, _datasets, _fit_condition, _run_dir
    started = time.perf_counter()
    datasets = _datasets(seed, cfg, tuple(cfg["conditions"]))
    honest = datasets.pop("SIM-HONEST", None)
    manifests = {c: make_manifest(d, seed) for c, d in datasets.items()}
    if "SIM-FI" in datasets and "SIM-BL" in datasets:
        assert_paired(datasets["SIM-FI"], datasets["SIM-BL"], manifests["SIM-FI"], manifests["SIM-BL"])
    runs = []
    for condition, dataset in datasets.items():
        manifest = manifests[condition]
        workdir = _run_dir(cfg, condition, f"seed_{seed}")
        keep = bool(cfg.get("keep_data"))
        artifact, frame, models = _fit_condition(dataset, manifest, workdir / "standard", False, keep)
        diagnostics = feature_diagnostics(dataset, frame, artifact["reference"])
        scores = score_all(frame, models)
        cache = workdir / "scores.npz"
        save_scores(cache, frame, scores, threshold_table(models), {"csv_sha256": manifest["csv_sha256"],
                                                                     "seed": seed, "condition": condition})
        table, _, _ = load_scores(cache)
        unobservable = unobservable_cheating(dataset, honest) if honest is not None and condition != "LEGACY" else None

        contaminated = _contaminated(dataset, manifest, workdir / "contaminated")
        artifact_c = fit_frozen(contaminated, make_manifest(contaminated, seed), workdir / "contaminated", keep)
        shutil.rmtree(workdir / "contaminated", ignore_errors=True)
        frame_c = feature_frame(dataset, artifact_c["reference"], manifest)
        if list(frame_c.attempt_id) != list(table.attempt_id):
            raise IntegrityError("contaminated frame is not aligned with the standard one")
        variants = {"standard": (scores, operating_points(models)),
                    "label_free": (scores, label_free_points(table, scores)),
                    "contaminated": (score_all(frame_c, artifact_c["models"]), operating_points(artifact_c["models"]))}
        for variant, (variant_scores, points) in variants.items():
            thresholds = {name: {rate: float(p["threshold"]) for rate, p in by_rate.items()}
                          for name, by_rate in points.items()}
            metrics = evaluate_table(table, variant_scores, thresholds, seed, condition == "SIM-BL")
            for name, rows in final_only_metrics(table, variant_scores, thresholds, seed).items():
                metrics[name].update(rows)
            if unobservable and unobservable["identical_ids"]:
                identical = table.attempt_id.isin(set(unobservable["identical_ids"])).to_numpy()
                test = table.split.to_numpy() == SPLIT_CODES["test"]
                normal = table.is_anomaly.to_numpy() == 0
                for kind in sorted(set(table.anomaly_type[identical & test])):
                    mask = test & (normal | (identical & (table.anomaly_type.to_numpy() == kind)))
                    for name, row in _subset_metrics(table, variant_scores, thresholds, seed,
                                                     kind, mask).items():
                        metrics[name][f"unobservable:{kind}"] = {"type": kind, **row}
            run = {"seed": seed, "condition": condition, "tag": variant, "variant": variant,
                   "data": _data_record(dataset), "operating_points": points, "metrics": metrics}
            if variant == "standard":
                run.update({"split_counts": manifest["counts"], "feature_diagnostics": diagnostics,
                            "score_cache": relative(cache)})
                if unobservable:
                    run["unobservable"] = unobservable["by_type"]
            if variant == "contaminated":
                run["contaminated_train_positives"] = manifest["counts"]["train"]["anomalies"]
            runs.append(run)
    if honest is not None:
        datasets["SIM-HONEST"] = honest
    _cleanup(cfg, datasets)
    for run in runs:
        run["wall_seconds"] = time.perf_counter() - started
    return runs


# --------------------------------------------------------------------------- fi-dose and baseline workers


def fi_dose_worker(job: tuple[str, float, int, dict[str, Any]]) -> list[dict[str, Any]]:
    from .run import evaluate_worker
    kind, value, seed, cfg = job
    if kind == "fi":
        sub = dict(cfg, mechanisms=cfg["fi_mechanisms"][str(value)], worlds=("honest", "fi"),
                   conditions=("SIM-FI",), intensity=None)
        runs = evaluate_worker(seed, sub, tag=f"lambda_{value}")
        for run in runs:
            run["lambda"] = value
    else:
        sub = dict(cfg, worlds=("honest", "bl"), conditions=("SIM-BL",), intensity=value)
        runs = evaluate_worker(seed, sub, tag=f"intensity_{value}")
        for run in runs:
            run["intensity"] = value
    return runs


def baseline_worker(job: tuple[str, int, dict[str, Any]]) -> list[dict[str, Any]]:
    from .run import evaluate_worker
    variant, seed, cfg = job
    runs = evaluate_worker(seed, dict(cfg, **cfg["variants"][variant]), tag=variant)
    for run in runs:
        run["variant"] = variant
    return runs


# --------------------------------------------------------------------------- sensitivity v2


SENSITIVITY_V2_SEED = 20261115
SENSITIVITY_V2_FI_PARAMETERS = (
    ("fi.rapid_item_prob", (0.30, 0.90, False)),
    ("fi.changes_prob_multiplier", (0.5, 1.25, False)),
    ("fi.collusion_submit_window_ms", (500.0, 20000.0, True)),
    ("fi.score_jump_multiplier", (0.5, 1.5, False)),
)


def spearman_matrix(unit: np.ndarray) -> np.ndarray:
    ranks = np.argsort(np.argsort(unit, axis=0), axis=0).astype(float)
    ranks -= ranks.mean(axis=0)
    ranks /= np.sqrt((ranks ** 2).sum(axis=0))
    return ranks.T @ ranks


def decorrelated_lhs(n: int, d: int, seed: int, max_abs: float = LHS_MAX_ABS_SPEARMAN,
                     max_iterations: int = 200_000) -> tuple[np.ndarray, dict[str, Any]]:
    """Latin hypercube whose columns have max |Spearman ρ| <= ``max_abs``.

    Start from ``stats.latin_hypercube`` and swap two entries within one column of the currently
    worst pair (a within-column swap keeps every margin stratified); accept a swap when it lowers
    the maximum |ρ|, or keeps it and lowers the sum of squared ρ. Fixed RNG seed, deterministic.
    """
    from .stats import latin_hypercube
    unit = latin_hypercube(n, d, seed)
    rng = np.random.default_rng(seed + 1)
    off = ~np.eye(d, dtype=bool)

    def objective(matrix: np.ndarray) -> tuple[float, float]:
        values = np.abs(matrix[off])
        return float(values.max()), float((values ** 2).sum())

    rho = spearman_matrix(unit)
    start = objective(rho)[0]
    current = objective(rho)
    iterations = 0
    while current[0] > max_abs and iterations < max_iterations:
        iterations += 1
        masked = np.where(off, np.abs(rho), -1)
        i, j = np.unravel_index(np.argmax(masked), masked.shape)
        column = (i, j)[rng.integers(2)]
        a, b = rng.choice(n, size=2, replace=False)
        unit[[a, b], column] = unit[[b, a], column]
        candidate = spearman_matrix(unit)
        value = objective(candidate)
        if value[0] < current[0] - 1e-12 or (abs(value[0] - current[0]) <= 1e-12 and value[1] < current[1]):
            rho, current = candidate, value
        else:
            unit[[a, b], column] = unit[[b, a], column]
    if current[0] > max_abs:
        raise RuntimeError(f"could not reach max |Spearman| <= {max_abs} (got {current[0]:.3f})")
    return unit, {"seed": seed, "initial_max_abs_spearman": start, "max_abs_spearman": current[0],
                  "iterations": iterations, "target": max_abs}


def sensitivity_v2_configs(count: int, base_parameters, seed: int = SENSITIVITY_V2_SEED
                           ) -> tuple[list[dict[str, float]], list[tuple], dict[str, Any]]:
    from .stats import scale_unit
    parameters = list(base_parameters) + list(SENSITIVITY_V2_FI_PARAMETERS)
    unit, info = decorrelated_lhs(count, len(parameters), seed)
    values = scale_unit(unit, [bounds for _, bounds in parameters])
    configs = [{name: float(v) for (name, _), v in zip(parameters, row, strict=True)} for row in values]
    info["spearman_matrix"] = spearman_matrix(unit).round(4).tolist()
    return configs, parameters, info


def write_mechanisms_v2(values: dict[str, float], base: Path, path: Path) -> Path:
    from .run import write_mechanisms
    write_mechanisms(values, base, path)
    mechanisms = read_json(path)
    fi, defaults = mechanisms["fi"], read_json(base)["fi"]
    fi["rapid_item_prob"] = values["fi.rapid_item_prob"]
    m = values["fi.changes_prob_multiplier"]
    fi["changes_prob_low"] = min(1.0, defaults["changes_prob_low"] * m)
    fi["changes_prob_high"] = min(1.0, defaults["changes_prob_high"] * m)
    fi["collusion_submit_window_ms"] = round(values["fi.collusion_submit_window_ms"])
    s = values["fi.score_jump_multiplier"]
    fi["score_jump_low"] = defaults["score_jump_low"] * s
    fi["score_jump_high"] = defaults["score_jump_high"] * s
    write_json(path, mechanisms)
    return path


def sensitivity_v2_worker(job: tuple[int, int, str, dict[str, Any]]) -> list[dict[str, Any]]:
    from .run import evaluate_worker
    config_id, seed, mechanisms, cfg = job
    tag = "default" if config_id < 0 else f"config_{config_id:03d}"
    runs = evaluate_worker(seed, dict(cfg, mechanisms=mechanisms), tag=tag)
    for run in runs:
        run["config_id"] = config_id
    return runs


def file_record(path: Path) -> dict[str, str]:
    return {"path": relative(path), "sha256": sha256_file(path)}

