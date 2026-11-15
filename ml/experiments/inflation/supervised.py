"""Post hoc stage ``supervised``: information ceiling of the frozen feature set.

For a training world X and a target world Y (same seed and cohort partition) the frozen
feature table is built exactly as in the transfer stage: X's normal-train reference
(``train.train`` via ``fit_frozen``), peers within Y. The model matrix is the frozen
``train.matrix`` (log transform, median imputation from X's training rows, missing
indicators) on either all 15 features (``*``) or the 12 attempt features (``*_no_peers``).
Labels: attempt-level binary (any cheating type) of X's train cohorts. Thresholds:
``choose_threshold`` on X's validation cohorts. Metrics: ``evaluate_table`` on Y's test cohorts.

Models, sklearn defaults, no hyper-parameter search:
* ``hgb``    HistGradientBoostingClassifier(random_state=seed)   (primary)
* ``logreg`` StandardScaler + LogisticRegression(max_iter=MAX_ITER) (secondary)
"""
from __future__ import annotations

import time
import warnings
from typing import Any

import numpy as np
import pandas as pd
from sklearn.ensemble import HistGradientBoostingClassifier
from sklearn.exceptions import ConvergenceWarning
from sklearn.linear_model import LogisticRegression
from sklearn.preprocessing import StandardScaler

from ml.anomaly.features import BASE_FEATURES, FEATURE_COLUMNS
from ml.anomaly.train import choose_threshold, matrix

from .common import CONDITIONS, FPR_TARGETS, relative
from .data import IntegrityError, assert_paired
from .pipeline import (
    SPLIT_CODES,
    evaluate_table,
    feature_frame,
    fit_frozen,
    load_scores,
    make_manifest,
    save_scores,
)

MAX_ITER = 10_000
FEATURE_SETS = {"": FEATURE_COLUMNS, "_no_peers": BASE_FEATURES}
MODELS = ("hgb", "logreg")
DETECTORS = tuple(m + s for m in MODELS for s in FEATURE_SETS)
PAIRS = (("SIM-FI", "SIM-FI"), ("SIM-FI", "SIM-BL"), ("SIM-BL", "SIM-FI"), ("SIM-BL", "SIM-BL"),
         ("LEGACY", "SIM-FI"), ("LEGACY", "SIM-BL"))


class SupervisedDetector:
    """A classifier on the frozen model matrix; ``score`` is P(cheating)."""

    def __init__(self, kind: str, columns: list[str], seed: int):
        self.kind, self.columns, self.seed = kind, columns, seed

    def fit(self, training: pd.DataFrame, labels: np.ndarray) -> SupervisedDetector:
        values, self.medians = matrix(training, self.columns)
        if len(set(labels.tolist())) != 2:
            raise ValueError("supervised training needs both classes")
        if self.kind == "hgb":
            self.scaler = None
            self.estimator = HistGradientBoostingClassifier(random_state=self.seed).fit(values, labels)
            self.info = {"n_iter": int(self.estimator.n_iter_)}
        elif self.kind == "logreg":
            self.scaler = StandardScaler().fit(values)
            with warnings.catch_warnings(record=True) as caught:
                warnings.simplefilter("always", ConvergenceWarning)
                self.estimator = LogisticRegression(max_iter=MAX_ITER).fit(self.scaler.transform(values), labels)
            iterations = int(np.max(self.estimator.n_iter_))
            self.info = {"n_iter": iterations, "converged": iterations < MAX_ITER and not any(
                issubclass(w.category, ConvergenceWarning) for w in caught)}
        else:
            raise ValueError(self.kind)
        self.info.update({"train_rows": len(labels), "train_positives": int(labels.sum())})
        return self

    def score(self, frame: pd.DataFrame) -> np.ndarray:
        values, _ = matrix(frame, self.columns, self.medians)
        if self.scaler is not None:
            values = self.scaler.transform(values)
        return self.estimator.predict_proba(values)[:, 1]


def training_labels(frame: pd.DataFrame) -> np.ndarray:
    """Binary label of the training world: truth.csv ``is_anomaly`` when present (checked against the CSV)."""
    labels = frame.is_anomaly.to_numpy(dtype=int)
    if "truth_is_anomaly" in frame:
        truth = frame.truth_is_anomaly.astype(int).to_numpy()
        if not np.array_equal(truth, labels):
            raise IntegrityError("truth is_anomaly differs from the CSV label")
        labels = truth
    return labels


def fit_supervised(frame: pd.DataFrame, seed: int) -> dict[str, dict[str, Any]]:
    """Fit on train cohorts; operating points on validation cohorts of the same (training) world."""
    split = frame.split.to_numpy()
    labels = training_labels(frame)
    train, validation = split == SPLIT_CODES["train"], split == SPLIT_CODES["validation"]
    models = {}
    for kind in MODELS:
        for suffix, columns in FEATURE_SETS.items():
            detector = SupervisedDetector(kind, columns, seed).fit(frame[train], labels[train])
            scores = detector.score(frame[validation])
            models[kind + suffix] = {"detector": detector, "operating_points": {
                str(rate): choose_threshold(labels[validation], scores, rate) for rate in FPR_TARGETS}}
    return models


def supervised_worker(seed: int, cfg: dict[str, Any]) -> list[dict[str, Any]]:
    from .run import _cleanup, _data_record, _datasets, _run_dir
    started = time.perf_counter()
    needed = tuple(sorted({c for pair in cfg["pairs"] for c in pair}, key=CONDITIONS.index))
    datasets = _datasets(seed, cfg, needed)
    manifests = {c: make_manifest(d, seed) for c, d in datasets.items()}
    if "SIM-FI" in datasets and "SIM-BL" in datasets:
        assert_paired(datasets["SIM-FI"], datasets["SIM-BL"], manifests["SIM-FI"], manifests["SIM-BL"])
    fitted = {}
    for source in dict.fromkeys(s for s, _ in cfg["pairs"]):
        artifact = fit_frozen(datasets[source], manifests[source], _run_dir(cfg, "fit", source, f"seed_{seed}"),
                              keep_model=bool(cfg.get("keep_data")))
        frame = feature_frame(datasets[source], artifact["reference"], manifests[source])
        fitted[source] = (artifact, frame, fit_supervised(frame, seed))
    runs = []
    for source, target in cfg["pairs"]:
        artifact, source_frame, models = fitted[source]
        manifest_x, manifest_y = manifests[source], manifests[target]
        groups_x = {g for part in manifest_x["groups"].values() for g in part}
        groups_y = {g for part in manifest_y["groups"].values() for g in part}
        if groups_x == groups_y and manifest_x["groups"] != manifest_y["groups"]:
            raise IntegrityError("same cohorts but different partitions: test cohorts could leak")
        # Same feature construction as the transfer stage: X's reference, peers within Y.
        frame = source_frame if source == target else feature_frame(datasets[target], artifact["reference"],
                                                                     manifest_y)
        scores = {name: model["detector"].score(frame) for name, model in models.items()}
        thresholds = {name: {str(rate): float(model["operating_points"][str(rate)]["threshold"])
                             for rate in FPR_TARGETS} for name, model in models.items()}
        cache = _run_dir(cfg, "pairs", f"{source}__{target}", f"seed_{seed}") / "scores.npz"
        points = {name: model["operating_points"] for name, model in models.items()}
        save_scores(cache, frame, scores, thresholds, {"source": source, "target": target, "seed": seed,
                                                       "csv_sha256": manifest_y["csv_sha256"],
                                                       "operating_points": points})
        table, cached, _ = load_scores(cache)
        runs.append({"seed": seed, "source": source, "target": target, "score_cache": relative(cache),
                     "source_data": _data_record(datasets[source]), "target_data": _data_record(datasets[target]),
                     "operating_points": points,
                     "fit": {name: model["detector"].info for name, model in models.items()},
                     "metrics": evaluate_table(table, cached, thresholds, seed, with_breakdowns=True)})
    _cleanup(cfg, datasets)
    for run in runs:
        run["wall_seconds"] = time.perf_counter() - started
    return runs
