"""Stage ``ednet``: frozen detectors on unlabelled real EdNet pseudo-attempts.

Assumption (stated in every output): cheating is negligible in an ungraded
self-study tutor, so the share of flagged EdNet attempts estimates the realized
false-positive rate. This cannot be verified with EdNet; it is an assumption.

P1 transfer: detectors and thresholds fitted on LEGACY / SIM-honest, applied to
EdNet test cohorts. P2 refit: fitted on EdNet train cohorts, threshold on EdNet
validation cohorts (all labels 0), flag rate on EdNet test cohorts.
Realism: classifier two-sample test real-vs-synthetic + per-feature distances.
"""
from __future__ import annotations

import csv
import json
import sys
import time
from collections import Counter
from concurrent.futures import ProcessPoolExecutor
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from scipy import stats as scipy_stats
from sklearn.ensemble import HistGradientBoostingClassifier
from sklearn.metrics import roc_auc_score
from sklearn.model_selection import StratifiedGroupKFold

from ml.anomaly.features import BASE_FEATURES, read_attempts
from ml.anomaly.train import choose_threshold, matrix, score_model

from .common import (
    DEFAULT_SEEDS,
    EDNET_DIR,
    EXP_DATA,
    MAIN_DESIGN,
    SIM_BINARY,
    SIM_DEFAULT_PARAMS,
    SIM_MECHANISMS,
    WITHIN_RUN_BOOTSTRAP,
    provenance,
    read_json,
    relative,
    sha256_file,
)
from .data import (
    Dataset,
    ensure_legacy,
    ensure_sim,
    load_dataset,
    remove_dataset,
    resolve_params,
)
from .metrics import cohort_bootstrap
from .pipeline import feature_frame as _feature_frame
from .pipeline import fit_frozen, make_manifest

ASSUMPTION = ("EdNet has no cheating labels. Flag rate is interpreted as realized FPR under the "
              "assumption that cheating is negligible in an ungraded self-study tutor (EdNet/Santa). "
              "This assumption is not verifiable from the data.")
PEER_NOTE = ("iforest and quantile_rules include peer features (synchrony/agreement within 5 s). EdNet "
             "learners do not sit a common timed quiz, so these features are not meaningful on EdNet; "
             "their rows are reported but flagged. quantile_rules_no_peers is a harness-constructed "
             "detector (frozen train.quantile_scores on BASE_FEATURES), not part of the frozen pipeline.")
DEFAULT_EDNET_SEEDS = DEFAULT_SEEDS[:10]
DEFAULT_MAX_ATTEMPTS = 30_000
C2ST_MAX_PER_CLASS = 6_000
SUBSAMPLE_SEED = 20261114
STRATA = ("score_percent", "median_answer_time_ms", "fast_answer_fraction")
# Primary C2ST features: behaviour only. history_missing and z_score depend on how many attempts each
# user has (a design parameter), so they are used only in the secondary all-BASE_FEATURES C2ST.
BEHAVIOURAL_FEATURES = [f for f in BASE_FEATURES if f not in ("history_missing", "z_score")]


def find_ednet_csv() -> Path | None:
    candidates = sorted(p for p in EDNET_DIR.rglob("*.csv") if Path(str(p) + ".meta.json").exists())
    if len(candidates) > 1:
        raise SystemExit("several EdNet CSVs found; pass --ednet-csv: " + ", ".join(map(relative, candidates)))
    return candidates[0] if candidates else None


def subsample_cohorts(source: Path, max_attempts: int | None, root: Path) -> tuple[Path, dict[str, Any]]:
    """Keep whole cohorts in a fixed random order until max_attempts is reached."""
    source_sha = sha256_file(source)
    attempts_per_group: Counter = Counter()
    with source.open(encoding="utf-8", newline="") as handle:
        for row in csv.DictReader(handle):
            if row["event_type"] == "quiz.submitted":
                attempts_per_group[json.loads(row["context"])["group_id"]] += 1
    total = sum(attempts_per_group.values())
    info = {"source": relative(source), "source_sha256": source_sha, "source_attempts": total,
            "source_cohorts": len(attempts_per_group), "max_attempts": max_attempts,
            "subsample_seed": SUBSAMPLE_SEED}
    if max_attempts is None or total <= max_attempts:
        return source, dict(info, subsampled=False, attempts=total, cohorts=len(attempts_per_group))
    order = np.random.default_rng(SUBSAMPLE_SEED).permutation(sorted(attempts_per_group)).tolist()
    keep, count = set(), 0
    for group in order:
        if count >= max_attempts:
            break
        keep.add(group)
        count += attempts_per_group[group]
    target = root / "ednet" / f"subsample_{source_sha[:12]}_{max_attempts}.csv"
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        temporary = target.with_suffix(".tmp")
        with source.open(encoding="utf-8", newline="") as handle, temporary.open(
                "w", encoding="utf-8", newline="") as out:
            reader = csv.DictReader(handle)
            writer = csv.DictWriter(out, fieldnames=reader.fieldnames)
            writer.writeheader()
            for row in reader:
                if json.loads(row["context"])["group_id"] in keep:
                    writer.writerow(row)
        temporary.rename(target)
    return target, dict(info, subsampled=True, attempts=count, cohorts=len(keep),
                        subsample=relative(target), subsample_sha256=sha256_file(target))


def _quantile_no_peers(frame: pd.DataFrame) -> dict[str, Any]:
    training = frame[(frame.split == 0) & (frame.is_anomaly == 0)]
    validation = frame[frame.split == 1]
    values, medians = matrix(training, BASE_FEATURES)
    model = {"kind": "quantile_rules", "columns": BASE_FEATURES, "medians": medians,
             "sorted_train": np.sort(values, axis=0)}
    scores = _score(validation, model)
    labels = validation.is_anomaly.to_numpy(dtype=int)
    model["operating_points"] = {str(r): choose_threshold(labels, scores, r) for r in (0.05, 0.01)}
    return model


def _score(frame: pd.DataFrame, model: dict[str, Any]) -> np.ndarray:
    return score_model(frame, model)


def _models(artifact: dict[str, Any], frame: pd.DataFrame) -> dict[str, dict[str, Any]]:
    models = dict(artifact["models"])
    models["quantile_rules_no_peers"] = _quantile_no_peers(frame)
    return models


def _strata_edges(train: pd.DataFrame) -> dict[str, list[float]]:
    return {c: [float(x) for x in np.quantile(train[c].to_numpy(dtype=float), [.25, .5, .75])] for c in STRATA}


def _flag_summary(test: pd.DataFrame, flagged: np.ndarray, edges: dict[str, list[float]], seed: int,
                  strata_values: pd.DataFrame) -> dict[str, Any]:
    groups = test.group_id.to_numpy()
    zeros = np.zeros(len(test), dtype=int)
    ci = cohort_bootstrap(groups, zeros, flagged, WITHIN_RUN_BOOTSTRAP, seed)["fpr"]
    result = {"n": len(test), "flagged": int(flagged.sum()), "flag_rate": float(flagged.mean()),
              "cohort_ci": ci, "strata": {}}
    for column in STRATA:
        values = strata_values[column].to_numpy(dtype=float)
        bins = np.searchsorted(edges[column], values, side="left")  # Qk: (edge[k-1], edge[k]]; Q1 includes ties
        result["strata"][column] = {f"Q{b + 1}": {"n": int((bins == b).sum()),
                                                  "flag_rate": float(flagged[bins == b].mean())
                                                  if (bins == b).any() else None} for b in range(4)}
    change = strata_values.changes_count.to_numpy() > 0
    result["strata"]["any_change"] = {str(k): {"n": int((change == k).sum()),
                                               "flag_rate": float(flagged[change == k].mean())
                                               if (change == k).any() else None} for k in (False, True)}
    return result


def _apply(models: dict[str, dict[str, Any]], frame: pd.DataFrame, edges: dict[str, list[float]],
           seed: int, own: pd.DataFrame) -> dict[str, Any]:
    """Flag rates on EdNet test cohorts; strata always use EdNet's own-reference features."""
    test = frame[frame.split == 2].reset_index(drop=True)
    if test.is_anomaly.any():
        raise ValueError("EdNet rows must all carry is_anomaly=0")
    strata_values = own.set_index("attempt_id").loc[test.attempt_id].reset_index()
    out = {}
    for name, model in models.items():
        scores = _score(test, model)
        out[name] = {}
        for rate in ("0.05", "0.01"):
            threshold = float(model["operating_points"][rate]["threshold"])
            out[name][rate] = _flag_summary(test, scores >= threshold, edges, seed, strata_values)
            if rate != "0.05":  # strata are reported at the primary 5 % operating point only
                out[name][rate].pop("strata")
            out[name][rate]["threshold"] = threshold
            out[name][rate]["source_validation_fpr"] = model["operating_points"][rate]["validation_fpr"]
    return out


def _bootstrap_users_auc(labels: np.ndarray, scores: np.ndarray, users: np.ndarray, repeats: int, seed: int
                         ) -> dict[str, float]:
    names, index = np.unique(users, return_inverse=True)
    members = [np.flatnonzero(index == i) for i in range(len(names))]
    rng = np.random.default_rng(seed)
    values = []
    for _ in range(repeats):
        rows = np.concatenate([members[i] for i in rng.integers(0, len(names), len(names))])
        if len(np.unique(labels[rows])) == 2:
            values.append(roc_auc_score(labels[rows], scores[rows]))
    return {"low": float(np.quantile(values, .025)), "high": float(np.quantile(values, .975)),
            "valid_replicates": len(values)}


def c2st(real: pd.DataFrame, synthetic: pd.DataFrame, seed: int,
         features: list[str] = BEHAVIOURAL_FEATURES) -> dict[str, Any]:
    """Classifier two-sample test; 5-fold CV grouped by user."""
    rng = np.random.default_rng(seed)
    n = min(len(real), len(synthetic), C2ST_MAX_PER_CLASS)
    # Subsample whole users: shuffle users, take attempts until n.
    def take(frame: pd.DataFrame) -> pd.DataFrame:
        users = rng.permutation(np.unique(frame.user_id.to_numpy()))
        counts = frame.groupby("user_id").size()
        cumulative = np.cumsum(counts.loc[users].to_numpy())
        chosen = set(users[:int(np.searchsorted(cumulative, n)) + 1])
        return frame[frame.user_id.isin(chosen)]
    a, b = take(real), take(synthetic)
    x = np.vstack([a[features].to_numpy(dtype=float), b[features].to_numpy(dtype=float)])
    y = np.r_[np.zeros(len(a), dtype=int), np.ones(len(b), dtype=int)]
    users = np.r_[("real:" + a.user_id.astype(str)).to_numpy(), ("syn:" + b.user_id.astype(str)).to_numpy()]
    oof = np.zeros(len(y))
    folds = StratifiedGroupKFold(n_splits=5, shuffle=True, random_state=seed)
    for train_index, test_index in folds.split(x, y, users):
        classifier = HistGradientBoostingClassifier(random_state=seed).fit(x[train_index], y[train_index])
        oof[test_index] = classifier.predict_proba(x[test_index])[:, 1]
    auc = float(roc_auc_score(y, oof))
    distances = {}
    for column in BASE_FEATURES:
        u = a[column].to_numpy(dtype=float)
        v = b[column].to_numpy(dtype=float)
        u, v = u[np.isfinite(u)], v[np.isfinite(v)]
        if len(u) == 0 or len(v) == 0:
            distances[column] = {"ks": None, "wasserstein_std": None}
            continue
        sd = float(np.std(u)) or float(np.std(np.r_[u, v])) or 1.0
        distances[column] = {"ks": float(scipy_stats.ks_2samp(u, v).statistic),
                             "wasserstein_std": float(scipy_stats.wasserstein_distance(u, v) / sd),
                             "real_median": float(np.median(u)), "synthetic_median": float(np.median(v))}
    return {"auc": auc, "auc_user_bootstrap_ci": _bootstrap_users_auc(y, oof, users, 1000, seed),
            "n_real": len(a), "n_synthetic": len(b), "features": features, "distances": distances,
            "classifier": "HistGradientBoostingClassifier(default, random_state=seed)",
            "cv": "StratifiedGroupKFold(5, shuffle) grouped by user"}


def ednet_worker(job: tuple[int, dict[str, Any]]) -> dict[str, Any]:
    seed, cfg = job
    started = time.perf_counter()
    root = Path(cfg["exp_root"])
    work = root / "runs" / "ednet" / f"seed_{seed}"
    ednet_csv = Path(cfg["ednet_csv"])
    ednet = Dataset("EDNET", ednet_csv, seed, read_attempts(ednet_csv), None,
                    {"sha256": {ednet_csv.name: sha256_file(ednet_csv)}})
    if any(a["is_anomaly"] != 0 for a in ednet.attempts):
        raise ValueError("EdNet CSV must have is_anomaly=0 everywhere")
    ednet_manifest = make_manifest(ednet, seed)
    # P2 refit (all labels zero -> anomaly-free threshold).
    artifact_e = fit_frozen(ednet, ednet_manifest, work / "refit")
    frame_e = _feature_frame(ednet, artifact_e["reference"], ednet_manifest)
    edges = _strata_edges(frame_e[frame_e.split == 0])
    result: dict[str, Any] = {"seed": seed, "ednet_split_counts": ednet_manifest["counts"],
                              "strata_edges_from_ednet_train": edges}
    result["P2_refit"] = _apply(_models(artifact_e, frame_e), frame_e, edges, seed, frame_e)
    sources: dict[str, Dataset] = {}
    csv_l, record_l = ensure_legacy(seed, cfg["design"], root)
    sources["LEGACY"] = load_dataset("LEGACY", csv_l, seed, record_l)
    if cfg["sim"]:
        # SIM source = honest world, per-attempt item sampling, calibrated params (primary) and the
        # uncalibrated defaults (secondary) when they differ.
        variants = [("SIM-calibrated", cfg["params"])]
        if Path(cfg["params"]).resolve() != SIM_DEFAULT_PARAMS.resolve():
            variants.append(("SIM-defaults", str(SIM_DEFAULT_PARAMS)))
        for name, params in variants:
            directory, record_s = ensure_sim(seed, cfg["design"], ("honest",), Path(params),
                                             Path(cfg["mechanisms"]), "per-attempt", None, root,
                                             Path(cfg["sim_binary"]))
            record_s = dict(record_s, manifest_sha256=sha256_file(directory / "manifest.json"))
            sources[name] = load_dataset("SIM-HONEST", directory / "honest.csv", seed, record_s,
                                         directory / "truth.csv")
            if any(a["is_anomaly"] for a in sources[name].attempts):
                raise ValueError("honest world contains labelled anomalies")
    result["P1_transfer"], result["realism"], result["source_data"] = {}, {}, {}
    result["realism_all_base"] = {}
    real_base = frame_e[frame_e.split == 0]
    for name, dataset in sources.items():
        manifest = make_manifest(dataset, seed)
        artifact = fit_frozen(dataset, manifest, work / name)
        own = _feature_frame(dataset, artifact["reference"], manifest)
        models = _models(artifact, own)
        transferred = _feature_frame(ednet, artifact["reference"], ednet_manifest)
        result["P1_transfer"][name] = _apply(models, transferred, edges, seed, frame_e)
        normal_train = own[(own.split == 0) & (own.is_anomaly == 0)]
        result["realism"][name] = c2st(real_base, normal_train, seed)
        result["realism_all_base"][name] = {k: v for k, v in c2st(real_base, normal_train, seed,
                                                                   BASE_FEATURES).items() if k != "distances"}
        result["source_data"][name] = {"csv_sha256": dataset.record["sha256"][dataset.csv.name],
                                       "simulator_manifest_sha256": dataset.record.get("manifest_sha256"),
                                       "params": dataset.record.get("params")}
        if not cfg.get("keep_data"):
            remove_dataset(dataset.csv.parent, root)
    result["wall_seconds"] = time.perf_counter() - started
    return result


def run_ednet(args) -> dict[str, Any]:
    ednet_csv = Path(args.ednet_csv) if args.ednet_csv else find_ednet_csv()
    if ednet_csv is None or not ednet_csv.exists():
        raise SystemExit("EdNet CSV not found; pass --ednet-csv (looked under ml/datasets/raw/ednet/)")
    meta_path = Path(str(ednet_csv) + ".meta.json")
    meta = read_json(meta_path) if meta_path.exists() else None
    if meta and meta.get("output_sha256") and meta["output_sha256"] != sha256_file(ednet_csv):
        raise SystemExit("EdNet CSV SHA-256 differs from its .meta.json")
    max_attempts = args.ednet_max_attempts if args.ednet_max_attempts is not None else DEFAULT_MAX_ATTEMPTS
    used_csv, subsample = subsample_cohorts(ednet_csv, max_attempts if max_attempts > 0 else None,
                                            Path(args.exp_root))
    binary = Path(args.sim_binary) if args.sim_binary else SIM_BINARY
    params, params_source = resolve_params(args.params)
    design = {"students": args.students, "attempts": args.attempts, "questions": args.questions,
              "anomaly_rate": args.anomaly_rate if args.anomaly_rate is not None else MAIN_DESIGN["anomaly_rate"]}
    seeds = args.ednet_seeds or args.seeds or DEFAULT_EDNET_SEEDS
    cfg = {"ednet_csv": str(used_csv), "design": design, "exp_root": args.exp_root, "sim": binary.exists(),
           "params": str(params), "mechanisms": args.mechanisms or str(SIM_MECHANISMS), "sim_binary": str(binary),
           "keep_data": args.keep_data}
    workers = min(args.workers, 4, len(seeds))  # each worker holds the EdNet attempts in memory
    jobs = [(s, cfg) for s in seeds]
    if workers > 1:
        with ProcessPoolExecutor(max_workers=workers) as pool:
            runs = list(pool.map(ednet_worker, jobs))
    else:
        runs = [ednet_worker(job) for job in jobs]
    if not binary.exists():
        print("ednet: simulator not built; SIM-HONEST source skipped", file=sys.stderr)
    return {"stage": "ednet", "arguments": vars(args), "seeds": seeds, "design": design,
            "assumption": ASSUMPTION, "peer_feature_note": PEER_NOTE,
            "ednet": {"csv": relative(ednet_csv), "sha256": sha256_file(ednet_csv),
                      "meta": relative(meta_path) if meta else None,
                      "meta_sha256": sha256_file(meta_path) if meta else None, "subsample": subsample},
            "simulator": {"available": binary.exists(), "item_sampling": "per-attempt", "world": "honest",
                          "binary_sha256": sha256_file(binary) if binary.exists() else None},
            "params": {"path": relative(params), "source": params_source,
                       "sha256": sha256_file(params) if params.exists() else None},
            "legacy_threshold_note": "LEGACY thresholds use LEGACY validation labels (as deployed); "
                                     "SIM-honest and EdNet thresholds use all-zero validation labels.",
            "provenance": provenance(), "runs": runs, "exp_data": relative(EXP_DATA)}
