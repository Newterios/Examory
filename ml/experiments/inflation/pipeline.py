"""Fit the frozen detectors (via ``ml.anomaly.train.train``), score, and evaluate.

Leakage rules enforced here:
* timing/answer references and models are fitted on normal training cohorts only
  (``train.train`` does this; we re-check the row selection);
* thresholds come only from validation cohorts (``choose_threshold``);
* metrics are computed only on test cohorts.
"""
from __future__ import annotations

import json
import shutil
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.covariance import MinCovDet
from sklearn.neighbors import LocalOutlierFactor
from sklearn.preprocessing import StandardScaler
from sklearn.svm import OneClassSVM

from ml.anomaly.features import BASE_FEATURES, FEATURE_COLUMNS, build_features
from ml.anomaly.split import split_groups
from ml.anomaly.train import choose_threshold, matrix, score_model, train

from .common import FPR_TARGETS, ROC_GRID, WITHIN_RUN_BOOTSTRAP, sha256_file, slot_of
from .data import Dataset, IntegrityError
from .metrics import full_metrics

SPLIT_CODES = {"train": 0, "validation": 1, "test": 2}
# Truth fields used for SIM breakdowns (mechanism -> truth.csv column).
# For copy_peer, every truth column whose name contains "sync" is also used (pair-level
# synchronous-submission flag added to the simulator; skipped automatically when absent).
BREAKDOWN_FIELDS = {"copy_peer": ("role",), "preknowledge": ("mech_flag",), "lookup": ("mech_flag",),
                    "proxy": ()}


def make_manifest(dataset: Dataset, seed: int) -> dict[str, Any]:
    """Same content as ``ml.anomaly.split.create_manifest`` without re-reading the CSV."""
    students: dict[str, str] = {}
    for attempt in dataset.attempts:
        if students.setdefault(attempt["user_id"], attempt["group_id"]) != attempt["group_id"]:
            raise IntegrityError("A student spans multiple cohorts")
    groups = split_groups([a["group_id"] for a in dataset.attempts], seed)
    return {"schema_version": 1, "seed": seed, "ratios": [.6, .2, .2],
            "csv_sha256": sha256_file(dataset.csv), "groups": groups,
            "attempt_ids": {name: sorted(a["attempt_id"] for a in dataset.attempts if a["group_id"] in values)
                            for name, values in groups.items()},
            "counts": {name: {"cohorts": len(values),
                              "attempts": sum(a["group_id"] in values for a in dataset.attempts),
                              "anomalies": sum(a["is_anomaly"] for a in dataset.attempts
                                               if a["group_id"] in values)}
                       for name, values in groups.items()}}


def fit_frozen(dataset: Dataset, manifest: dict[str, Any], workdir: Path, keep_model: bool = False
               ) -> dict[str, Any]:
    workdir.mkdir(parents=True, exist_ok=True)
    manifest_path = workdir / "split.json"
    manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
    artifact = train(dataset.csv, manifest_path, workdir / "model")
    if not keep_model:  # the pickle is large and fully reproducible; the artifact stays in memory
        shutil.rmtree(workdir / "model", ignore_errors=True)
        manifest_path.unlink(missing_ok=True)
    if artifact["normal_train_attempts"] != sum(
            a["group_id"] in manifest["groups"]["train"] and a["is_anomaly"] == 0 for a in dataset.attempts):
        raise IntegrityError("training rows are not exactly the normal training-cohort attempts")
    return artifact


def split_column(frame: pd.DataFrame, manifest: dict[str, Any]) -> np.ndarray:
    lookup = {g: SPLIT_CODES[name] for name, groups in manifest["groups"].items() for g in groups}
    codes = frame.group_id.map(lookup)
    if codes.isna().any():
        raise IntegrityError("attempts outside the cohort manifest")
    return codes.to_numpy(dtype=int)


def feature_frame(dataset: Dataset, reference: dict[str, Any], manifest: dict[str, Any]) -> pd.DataFrame:
    frame = build_features(dataset.attempts, reference)
    frame["split"] = split_column(frame, manifest)
    frame["is_anomaly"] = frame.is_anomaly.astype(int)
    frame["is_final_attempt"] = frame.is_final_attempt.astype(bool)
    if dataset.truth is not None:
        truth = dataset.truth.reindex(frame.attempt_id)
        for column in truth.columns:
            frame[f"truth_{column}"] = truth[column].to_numpy()
    return frame


def manipulation_check(dataset: Dataset, honest: Dataset) -> dict[str, Any]:
    """Mean paired difference cheater attempt − its honest counterfactual (same attempt_id)."""
    base = {a["attempt_id"]: a for a in honest.attempts}

    def observables(a: dict[str, Any]) -> np.ndarray:
        times = list(a["first_times"].values())
        return np.array([a["score_percent"] * len(times) / 100, a["duration_ms"] / 1000,
                         a["changes_count"], float(np.median(times)) / 1000])

    groups: dict[str, list[np.ndarray]] = {}
    for a in dataset.attempts:
        if not a["is_anomaly"]:
            continue
        if a["attempt_id"] not in base:
            raise IntegrityError(f"no honest counterfactual for {a['attempt_id']}")
        diff = observables(a) - observables(base[a["attempt_id"]])
        keys = [a["anomaly_type"]]
        truth = dataset.truth
        if truth is not None and "sync_submit" in truth and a["anomaly_type"] == "copy_peer":
            keys.append(f"copy_peer:sync_submit={truth.loc[a['attempt_id'], 'sync_submit']}")
        for key in keys:
            groups.setdefault(key, []).append(diff)
    names = ("score_points", "duration_s", "answer_changes", "median_first_answer_s")
    return {key: {"n": len(rows), **dict(zip(names, np.mean(rows, axis=0).tolist(), strict=True)),
                  "share_changed": float(np.mean(np.any(np.abs(rows) > 1e-9, axis=1)))}
            for key, rows in groups.items()}


def thresholds_of(model: dict[str, Any]) -> dict[str, float]:
    return {str(rate): float(model["operating_points"][str(rate)]["threshold"]) for rate in FPR_TARGETS}


class RobustDetector:
    """Normal-train-only unsupervised detector on the frozen model matrix."""

    def __init__(self, kind: str, columns: list[str]):
        self.kind, self.columns = kind, columns

    def fit(self, training: pd.DataFrame) -> RobustDetector:
        values, self.medians = matrix(training, self.columns)
        keep = values.std(axis=0) > 0
        # Exact duplicate columns (e.g. a missing indicator equal to history_missing) make the
        # covariance singular; keep the first copy.
        _, first = np.unique(values[:, keep].round(12), axis=1, return_index=True)
        self.keep = np.flatnonzero(keep)[np.sort(first)]
        self.scaler = StandardScaler().fit(values[:, self.keep])
        x = self.scaler.transform(values[:, self.keep])
        if self.kind == "lof":
            self.estimator = LocalOutlierFactor(novelty=True).fit(x)
        elif self.kind == "ocsvm":
            self.estimator = OneClassSVM(kernel="rbf", gamma="scale", nu=0.05).fit(x)
        elif self.kind == "mcd":
            self.estimator = MinCovDet(random_state=42).fit(x)
        else:
            raise ValueError(self.kind)
        return self

    def score(self, frame: pd.DataFrame) -> np.ndarray:
        values, _ = matrix(frame, self.columns, self.medians)
        x = self.scaler.transform(values[:, self.keep])
        if self.kind == "lof":
            return -self.estimator.score_samples(x)
        if self.kind == "ocsvm":
            return -self.estimator.decision_function(x)
        return self.estimator.mahalanobis(x)


def robust_models(frame: pd.DataFrame) -> dict[str, dict[str, Any]]:
    training = frame[(frame.split == 0) & (frame.is_anomaly == 0)]
    validation = frame[frame.split == 1]
    labels = validation.is_anomaly.to_numpy(dtype=int)
    models = {}
    for kind in ("lof", "ocsvm", "mcd"):
        for suffix, columns in (("", FEATURE_COLUMNS), ("_no_peers", BASE_FEATURES)):
            detector = RobustDetector(kind, columns).fit(training)
            scores = detector.score(validation)
            models[kind + suffix] = {"detector": detector, "operating_points": {
                str(rate): choose_threshold(labels, scores, rate) for rate in FPR_TARGETS}}
    return models


def score_all(frame: pd.DataFrame, models: dict[str, dict[str, Any]]) -> dict[str, np.ndarray]:
    scores = {}
    for name, model in models.items():
        scores[name] = model["detector"].score(frame) if "detector" in model else score_model(frame, model)
    return scores


def save_scores(path: Path, frame: pd.DataFrame, scores: dict[str, np.ndarray],
                thresholds: dict[str, dict[str, float]], meta: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    def text(column: str) -> np.ndarray:
        return np.asarray([str(v) for v in frame[column].tolist()], dtype=str)

    arrays = {"split": frame.split.to_numpy(dtype=int), "is_anomaly": frame.is_anomaly.to_numpy(dtype=int),
              "is_final_attempt": frame.is_final_attempt.to_numpy(dtype=bool)}
    for column in ("attempt_id", "user_id", "group_id", "anomaly_type"):
        arrays[column] = text(column)
    for column in frame.columns:
        if column.startswith("truth_"):
            arrays[column] = text(column)
    for name, values in scores.items():
        arrays[f"score__{name}"] = np.asarray(values, dtype=float)
    arrays["meta_json"] = np.asarray(json.dumps({"thresholds": thresholds, **meta}, default=str))
    np.savez_compressed(path, **arrays)


def load_scores(path: Path) -> tuple[pd.DataFrame, dict[str, np.ndarray], dict[str, Any]]:
    with np.load(path, allow_pickle=False) as data:
        meta = json.loads(str(data["meta_json"]))
        scores = {k[len("score__"):]: data[k] for k in data.files if k.startswith("score__")}
        table = pd.DataFrame({k: data[k] for k in data.files
                              if not k.startswith("score__") and k != "meta_json"})
    return table, scores, meta


def evaluate_table(table: pd.DataFrame, scores: dict[str, np.ndarray],
                   thresholds: dict[str, dict[str, float]], seed: int,
                   with_breakdowns: bool = True, bootstrap: int = WITHIN_RUN_BOOTSTRAP) -> dict[str, Any]:
    """Metrics on test cohorts only, for every detector and subset."""
    test = (table.split.to_numpy() == SPLIT_CODES["test"])
    labels_all = table.is_anomaly.to_numpy(dtype=int)
    types = table.anomaly_type.to_numpy()
    normal = labels_all == 0
    subsets: dict[str, tuple[np.ndarray, dict[str, Any], bool]] = {
        "all": (test, {}, True),
        "final": (test & table.is_final_attempt.to_numpy(dtype=bool), {}, False),
    }
    for kind in sorted(set(types[test & ~normal])):
        slot = slot_of(kind)
        key = f"slot:{slot}" if slot else f"type:{kind}"
        subsets[key] = (test & (normal | (types == kind)), {"type": kind, "slot": slot}, True)
        if with_breakdowns:
            fields = list(BREAKDOWN_FIELDS.get(kind, ()))
            if kind == "copy_peer":
                fields += [c[len("truth_"):] for c in table.columns if c.startswith("truth_") and "sync" in c]
                # simulator >= 0.3.0 with a closing time (absent in every earlier dataset)
                fields += [c[len("truth_"):] for c in table.columns if c == "truth_auto_submitted"]
            for field in fields:
                column = f"truth_{field}"
                if column not in table:
                    continue
                values = table[column].to_numpy()
                for value in sorted(set(values[test & (types == kind)])):
                    mask = test & (normal | ((types == kind) & (values == value)))
                    subsets[f"truth:{kind}:{field}={value}"] = (mask, {"type": kind, "slot": slot,
                                                                      "field": field, "value": value}, False)
    groups = table.group_id.to_numpy()
    result: dict[str, Any] = {}
    for name, detector_scores in scores.items():
        rows = {}
        for subset, (mask, info, curve) in subsets.items():
            row = full_metrics(labels_all[mask], detector_scores[mask], thresholds[name], groups[mask],
                               bootstrap, seed, ROC_GRID if curve else None)
            rows[subset] = {**info, **row}
        result[name] = rows
    return result


def threshold_table(models: dict[str, dict[str, Any]]) -> dict[str, dict[str, float]]:
    return {name: thresholds_of(model) for name, model in models.items()}


def operating_points(models: dict[str, dict[str, Any]]) -> dict[str, Any]:
    return {name: model["operating_points"] for name, model in models.items()}
