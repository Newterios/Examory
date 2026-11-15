"""Experiment stages.

python -m ml.experiments.inflation.run main|transfer|robust-detectors|dose|sensitivity|prevalence|ednet
python -m ml.experiments.inflation.run supervised|similarity      (post hoc, exploratory; README amendment log)
python -m ml.experiments.inflation.run fi-dose|protocol|baseline   (post hoc, after independent review)
"""
from __future__ import annotations

import argparse
import os
import sys
import time
from concurrent.futures import ProcessPoolExecutor
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

for _variable in ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS", "VECLIB_MAXIMUM_THREADS"):
    os.environ.setdefault(_variable, "1")

import numpy as np

from .common import (
    CONDITIONS,
    DEFAULT_SEEDS,
    EXP_DATA,
    INTENSITIES,
    MAIN_DESIGN,
    MAX_WORKERS,
    PREVALENCE_RATE,
    SIM_MECHANISMS,
    STAGES_DIR,
    TRANSFER_PAIRS,
    provenance,
    read_json,
    relative,
    sha256_file,
    write_json,
)
from .compact import slim_stage
from .data import (
    Dataset,
    IntegrityError,
    assert_paired,
    ensure_legacy,
    ensure_sim,
    load_dataset,
    remove_dataset,
    resolve_params,
)
from .pipeline import (
    evaluate_table,
    feature_frame,
    fit_frozen,
    load_scores,
    make_manifest,
    manipulation_check,
    operating_points,
    robust_models,
    save_scores,
    score_all,
    threshold_table,
)

SENSITIVITY_SEED = 20261114
SENSITIVITY_PARAMETERS = (
    # name, (low, high, log_uniform), how to write it into mechanisms.json
    ("preknowledge.naive_prob", (0.0, 1.0, False)),
    ("preknowledge.speedup_high", (0.3, 0.9, False)),
    ("lookup.duration_median_s", (10.0, 90.0, True)),
    ("lookup.verify_mode_prob", (0.0, 1.0, False)),
    ("lookup.propensity", (0.3, 1.0, False)),
    ("copy_peer.coverage", (0.3, 1.0, False)),
    ("copy_peer.relay_median_s", (2.0, 30.0, True)),
    ("copy_peer.start_lag_median_s", (5.0, 300.0, True)),
    ("proxy.theta_mean", (0.5, 2.0, False)),
    ("copy_peer.sync_submit_prob", (0.0, 1.0, False)),
)


# --------------------------------------------------------------------------- data


def _datasets(seed: int, cfg: dict[str, Any], conditions: tuple[str, ...]) -> dict[str, Dataset]:
    design = cfg["design"]
    result = {}
    if "LEGACY" in conditions:
        csv, record = ensure_legacy(seed, design, Path(cfg["exp_root"]))
        result["LEGACY"] = load_dataset("LEGACY", csv, seed, record)
    sim_conditions = [c for c in conditions if c.startswith("SIM-")]
    if sim_conditions:
        worlds = tuple(cfg.get("worlds") or ("honest", "fi", "bl"))
        directory, record = ensure_sim(seed, design, worlds, Path(cfg["params"]), Path(cfg["mechanisms"]),
                                       cfg.get("item_sampling"), cfg.get("intensity"), Path(cfg["exp_root"]),
                                       Path(cfg["sim_binary"]), cfg.get("sim_extra_args"))
        record = dict(record, manifest_sha256=sha256_file(directory / "manifest.json"))
        if cfg.get("manipulation") and "honest" in worlds and "SIM-HONEST" not in sim_conditions:
            sim_conditions = [*sim_conditions, "SIM-HONEST"]
        for condition in sim_conditions:
            world = {"SIM-FI": "fi", "SIM-BL": "bl", "SIM-HONEST": "honest"}[condition]
            result[condition] = load_dataset(condition, directory / f"{world}.csv", seed, record,
                                             directory / "truth.csv")
    return result


def _data_record(dataset: Dataset) -> dict[str, Any]:
    record = dataset.record
    return {"csv": relative(dataset.csv), "csv_sha256": record["sha256"][dataset.csv.name],
            "command": record.get("command"), "simulator_manifest_sha256": record.get("manifest_sha256"),
            "manifest_verified_files": record.get("manifest_verified_files"),
            "params": record.get("params"), "mechanisms": record.get("mechanisms")}


def _cleanup(cfg: dict[str, Any], datasets: dict[str, Dataset]) -> None:
    """Delete generated datasets once their scores are cached (unless --keep-data)."""
    if cfg.get("keep_data"):
        return
    for directory in {d.csv.parent for d in datasets.values()}:
        remove_dataset(directory, Path(cfg["exp_root"]))


def _fit_condition(dataset: Dataset, manifest: dict[str, Any], workdir: Path, robust: bool,
                   keep: bool = False):
    artifact = fit_frozen(dataset, manifest, workdir, keep_model=keep)
    frame = feature_frame(dataset, artifact["reference"], manifest)
    models = robust_models(frame) if robust else artifact["models"]
    return artifact, frame, models


def _run_dir(cfg: dict[str, Any], *parts: Any) -> Path:
    return Path(cfg["exp_root"]).joinpath("runs", cfg["stage"], *map(str, parts))


# --------------------------------------------------------------------------- workers


def evaluate_worker(seed: int, cfg: dict[str, Any], tag: str = "") -> list[dict[str, Any]]:
    """main / robust-detectors / prevalence / dose / sensitivity: fit, score, evaluate per condition."""
    started = time.perf_counter()
    conditions = tuple(cfg["conditions"])
    datasets = _datasets(seed, cfg, conditions)
    honest = datasets.pop("SIM-HONEST", None)
    manifests = {c: make_manifest(d, seed) for c, d in datasets.items()}
    if "SIM-FI" in datasets and "SIM-BL" in datasets:
        assert_paired(datasets["SIM-FI"], datasets["SIM-BL"], manifests["SIM-FI"], manifests["SIM-BL"])
    runs = []
    diagnostics: dict[str, Any] = {}
    for condition, dataset in datasets.items():
        workdir = _run_dir(cfg, tag or "default", condition, f"seed_{seed}")
        cache = workdir / "scores.npz"
        if cfg.get("from_cache") and cache.exists():
            table, scores, meta = load_scores(cache)
            if meta["csv_sha256"] != manifests[condition]["csv_sha256"]:
                raise IntegrityError(f"{relative(cache)} was scored on a different CSV")
            thresholds, points = meta["thresholds"], meta["operating_points"]
        else:
            artifact, frame, models = _fit_condition(dataset, manifests[condition], workdir, cfg["robust"],
                                                     bool(cfg.get("keep_data")))
            if cfg.get("feature_diagnostics"):
                from .review import feature_diagnostics
                diagnostics[condition] = feature_diagnostics(dataset, frame, artifact["reference"])
            scores = score_all(frame, models)
            thresholds, points = threshold_table(models), operating_points(models)
            save_scores(cache, frame, scores, thresholds,
                        {"csv_sha256": manifests[condition]["csv_sha256"], "seed": seed,
                         "condition": condition, "operating_points": points})
            table, _, _ = load_scores(cache)
        runs.append({"seed": seed, "condition": condition, "tag": tag, "data": _data_record(dataset),
                     "split_counts": manifests[condition]["counts"], "operating_points": points,
                     "score_cache": relative(cache),
                     "metrics": evaluate_table(table, scores, thresholds, seed, cfg.get("breakdowns", True))})
        if honest is not None and condition in ("SIM-FI", "SIM-BL"):
            runs[-1]["manipulation"] = manipulation_check(dataset, honest)
        if cfg.get("items_affected") and dataset.truth is not None:
            from .review import items_affected
            runs[-1]["items_affected"] = items_affected(dataset)
        if condition in diagnostics:
            runs[-1]["feature_diagnostics"] = diagnostics[condition]
    if honest is not None:
        datasets["SIM-HONEST"] = honest
    _cleanup(cfg, datasets)
    for run in runs:
        run["wall_seconds"] = time.perf_counter() - started
    return runs


def transfer_worker(seed: int, cfg: dict[str, Any]) -> list[dict[str, Any]]:
    started = time.perf_counter()
    needed = tuple(sorted({c for pair in cfg["pairs"] for c in pair}, key=CONDITIONS.index))
    datasets = _datasets(seed, cfg, needed)
    manifests = {c: make_manifest(d, seed) for c, d in datasets.items()}
    if "SIM-FI" in datasets and "SIM-BL" in datasets:
        assert_paired(datasets["SIM-FI"], datasets["SIM-BL"], manifests["SIM-FI"], manifests["SIM-BL"])
    fitted = {}
    for condition, dataset in datasets.items():
        artifact = fit_frozen(dataset, manifests[condition], _run_dir(cfg, "fit", condition, f"seed_{seed}"),
                              keep_model=bool(cfg.get("keep_data")))
        fitted[condition] = artifact
    runs = []
    for source, target in cfg["pairs"]:
        artifact = fitted[source]
        manifest_x, manifest_y = manifests[source], manifests[target]
        groups_x = {g for part in manifest_x["groups"].values() for g in part}
        groups_y = {g for part in manifest_y["groups"].values() for g in part}
        if groups_x == groups_y and manifest_x["groups"] != manifest_y["groups"]:
            raise IntegrityError("same cohorts but different partitions: test cohorts could leak")
        # Features of Y use X's normal-train reference; peers are computed within Y.
        frame = feature_frame(datasets[target], artifact["reference"], manifest_y)
        scores = score_all(frame, artifact["models"])
        thresholds = threshold_table(artifact["models"])
        cache = _run_dir(cfg, "pairs", f"{source}__{target}", f"seed_{seed}") / "scores.npz"
        save_scores(cache, frame, scores, thresholds, {"source": source, "target": target, "seed": seed,
                                                       "csv_sha256": manifest_y["csv_sha256"]})
        table, _, _ = load_scores(cache)
        runs.append({"seed": seed, "source": source, "target": target, "score_cache": relative(cache),
                     "source_data": _data_record(datasets[source]), "target_data": _data_record(datasets[target]),
                     "operating_points": operating_points(artifact["models"]),
                     "metrics": evaluate_table(table, scores, thresholds, seed, with_breakdowns=False)})
    _cleanup(cfg, datasets)
    for run in runs:
        run["wall_seconds"] = time.perf_counter() - started
    return runs


def _sensitivity_worker(job: tuple[int, int, str, dict[str, Any], dict[str, Any]]) -> list[dict[str, Any]]:
    config_id, seed, mechanisms, _values, cfg = job
    runs = evaluate_worker(seed, dict(cfg, mechanisms=mechanisms), tag=f"config_{config_id:03d}")
    for run in runs:
        run["config_id"] = config_id
    return runs


def _dose_worker(job: tuple[float, int, dict[str, Any]]) -> list[dict[str, Any]]:
    intensity, seed, cfg = job
    runs = evaluate_worker(seed, dict(cfg, intensity=intensity), tag=f"intensity_{intensity}")
    for run in runs:
        run["intensity"] = intensity
    return runs


def _seed_job(job: tuple[int, dict[str, Any]]) -> list[dict[str, Any]]:
    seed, cfg = job
    if cfg["stage"] == "transfer":
        return transfer_worker(seed, cfg)
    if cfg["stage"] == "supervised":
        from .supervised import supervised_worker
        return supervised_worker(seed, cfg)
    if cfg["stage"] == "similarity":
        from .similarity import similarity_worker
        return similarity_worker(seed, cfg)
    if cfg["stage"] == "protocol":
        from .review import protocol_worker
        return protocol_worker(seed, cfg)
    return evaluate_worker(seed, cfg)


# --------------------------------------------------------------------------- driver


def _map(function, jobs: list, workers: int) -> list:
    if workers <= 1 or len(jobs) <= 1:
        return [function(job) for job in jobs]
    with ProcessPoolExecutor(max_workers=min(workers, MAX_WORKERS, len(jobs))) as pool:
        return list(pool.map(function, jobs))


def sensitivity_configs(count: int, seed: int = SENSITIVITY_SEED) -> list[dict[str, float]]:
    from .stats import latin_hypercube, scale_unit
    unit = latin_hypercube(count, len(SENSITIVITY_PARAMETERS), seed)
    values = scale_unit(unit, [bounds for _, bounds in SENSITIVITY_PARAMETERS])
    return [{name: float(v) for (name, _), v in zip(SENSITIVITY_PARAMETERS, row, strict=True)} for row in values]


def write_mechanisms(values: dict[str, float], base: Path, path: Path) -> Path:
    mechanisms = read_json(base)
    pre, lookup, copy, proxy = (mechanisms[k] for k in ("preknowledge", "lookup", "copy_peer", "proxy"))
    pre["naive_prob"] = values["preknowledge.naive_prob"]
    pre["speedup_high"] = values["preknowledge.speedup_high"]
    pre["speedup_low"] = 0.4 * values["preknowledge.speedup_high"]
    lookup["duration_log_ms_mean"] = float(np.log(values["lookup.duration_median_s"] * 1000))
    lookup["verify_mode_prob"] = values["lookup.verify_mode_prob"]
    lookup["propensity"] = values["lookup.propensity"]
    copy["coverage"] = values["copy_peer.coverage"]
    copy["relay_log_ms_mean"] = float(np.log(values["copy_peer.relay_median_s"] * 1000))
    copy["start_lag_log_ms_mean"] = float(np.log(values["copy_peer.start_lag_median_s"] * 1000))
    proxy["theta_mean"] = values["proxy.theta_mean"]
    copy["sync_submit_prob"] = values["copy_peer.sync_submit_prob"]
    write_json(path, mechanisms)
    return path


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("stage", choices=["main", "transfer", "robust-detectors", "dose", "sensitivity",
                                          "prevalence", "ednet", "supervised", "similarity", "fi-dose",
                                          "protocol", "baseline"])
    parser.add_argument("--seeds", type=int, nargs="+", default=None,
                        help="default 1001..1020 (dose, fi-dose: 1001..1010; sensitivity: v1 1001..1003, "
                             "v2 1001..1010)")
    parser.add_argument("--conditions", nargs="+", choices=CONDITIONS, default=None,
                        help="default: all three when the simulator is built, else LEGACY only")
    parser.add_argument("--students", type=int, default=MAIN_DESIGN["students"])
    parser.add_argument("--attempts", type=int, default=MAIN_DESIGN["attempts"])
    parser.add_argument("--questions", type=int, default=MAIN_DESIGN["questions"])
    parser.add_argument("--anomaly-rate", type=float, default=None)
    parser.add_argument("--params", default=None, help="simulator params (default: calibration if present)")
    parser.add_argument("--mechanisms", default=str(SIM_MECHANISMS))
    parser.add_argument("--sim-binary", default=None)
    parser.add_argument("--configs", type=int, default=None, help="sensitivity: Latin-hypercube size (v1 40, v2 60)")
    parser.add_argument("--design", choices=["v1", "v2"], default="v2",
                        help="sensitivity design: v2 (default; 14 parameters incl. injection, decorrelated LHS, "
                             "default configuration) or the original v1")
    parser.add_argument("--lambdas", type=float, nargs="+", default=None, help="fi-dose: injection multipliers")
    parser.add_argument("--variants", nargs="+", default=None,
                        choices=["revision", "per_attempt", "closing_p90", "closing_p75"],
                        help="baseline variants (default: all; closing_* need --sim-binary-next)")
    parser.add_argument("--sim-binary-next", default=None,
                        help="examory-sim >= 0.3.0 for baseline closing_* (default sim/bin/examory-sim-next)")
    parser.add_argument("--intensities", type=float, nargs="+", default=list(INTENSITIES))
    parser.add_argument("--workers", type=int, default=MAX_WORKERS)
    parser.add_argument("--exp-root", default=str(EXP_DATA))
    parser.add_argument("--output", default=None, help="stage JSON (default results/stages/<stage>.json)")
    parser.add_argument("--from-cache", action="store_true", help="re-evaluate cached .npz scores")
    parser.add_argument("--keep-data", action="store_true",
                        help="keep generated CSVs and model pickles (default: delete once scores are cached)")
    parser.add_argument("--ednet-csv", default=None)
    parser.add_argument("--ednet-max-attempts", type=int, default=None)
    parser.add_argument("--ednet-seeds", type=int, nargs="+", default=None)
    args = parser.parse_args(argv)
    if args.workers < 1:
        parser.error("--workers must be positive")
    return args


def main(argv: list[str] | None = None) -> dict[str, Any]:
    args = parse_args(argv)
    from .common import SIM_BINARY
    started_at = datetime.now(UTC).isoformat()
    started = time.perf_counter()
    if args.stage == "ednet":
        from .ednet import run_ednet
        result = run_ednet(args)
    else:
        result = _run_simulation_stage(args, Path(args.sim_binary) if args.sim_binary else SIM_BINARY)
    result["timing"] = {"started_at": started_at, "finished_at": datetime.now(UTC).isoformat(),
                        "wall_seconds": time.perf_counter() - started}
    name = f"{args.stage.replace('-', '_')}.json"
    output = Path(args.output) if args.output else STAGES_DIR / name
    write_json(Path(args.exp_root) / "stages_full" / name, result, compact=True)
    write_json(output, slim_stage(result), compact=True)
    print(f"{args.stage}: wrote {relative(output)} in {result['timing']['wall_seconds']:.1f} s", file=sys.stderr)
    return result


def _run_simulation_stage(args: argparse.Namespace, binary: Path) -> dict[str, Any]:
    from ml.experiments.inflation import common
    have_sim = binary.exists()
    if args.stage in ("supervised", "similarity", "fi-dose", "protocol", "baseline") and args.from_cache:
        raise SystemExit(f"{args.stage} does not support --from-cache (it always refits and rescores)")
    if args.stage in ("dose", "sensitivity", "fi-dose", "baseline") and not have_sim:
        raise SystemExit(f"{args.stage} needs the simulator binary {relative(binary)}")
    conditions = tuple(args.conditions) if args.conditions else (CONDITIONS if have_sim else ("LEGACY",))
    if any(c.startswith("SIM-") for c in conditions) and not have_sim:
        raise SystemExit(f"SIM conditions need the simulator binary {relative(binary)}")
    params, params_source = resolve_params(args.params)
    rate = args.anomaly_rate if args.anomaly_rate is not None else (
        PREVALENCE_RATE if args.stage == "prevalence" else MAIN_DESIGN["anomaly_rate"])
    design = {"students": args.students, "attempts": args.attempts, "questions": args.questions,
              "anomaly_rate": rate}
    default_seeds = {"dose": DEFAULT_SEEDS[:10], "fi-dose": DEFAULT_SEEDS[:10],
                     "sensitivity": DEFAULT_SEEDS[:10] if args.design == "v2" else DEFAULT_SEEDS[:3]
                     }.get(args.stage, DEFAULT_SEEDS)
    seeds = args.seeds or default_seeds
    cfg = {"stage": args.stage, "design": design, "params": str(params), "mechanisms": args.mechanisms,
           "exp_root": args.exp_root, "sim_binary": str(binary), "conditions": conditions,
           "robust": args.stage == "robust-detectors", "from_cache": args.from_cache,
           "breakdowns": args.stage in ("main", "prevalence"), "keep_data": args.keep_data,
           "manipulation": args.stage == "main"}
    extra: dict[str, Any] = {}
    if args.stage == "transfer":
        pairs = [p for p in TRANSFER_PAIRS if p[0] in conditions and p[1] in conditions]
        cfg["pairs"] = pairs
        extra["pairs"] = pairs
        runs = [r for chunk in _map(_seed_job, [(s, cfg) for s in seeds], args.workers) for r in chunk]
    elif args.stage == "supervised":
        from .supervised import MAX_ITER, PAIRS
        pairs = [p for p in PAIRS if p[0] in conditions and p[1] in conditions]
        cfg["pairs"] = pairs
        extra.update({"pairs": pairs, "models": {
            "hgb": "HistGradientBoostingClassifier(random_state=seed), sklearn defaults",
            "logreg": f"StandardScaler + LogisticRegression(max_iter={MAX_ITER}), sklearn defaults"},
            "feature_sets": {"": "FEATURE_COLUMNS (12 attempt + 3 peer)", "_no_peers": "BASE_FEATURES (12)"},
            "post_hoc": True})
        runs = [r for chunk in _map(_seed_job, [(s, cfg) for s in seeds], args.workers) for r in chunk]
    elif args.stage == "similarity":
        from .similarity import MIN_SHARED_ITEMS, NO_PARTNER_SCORE
        if any(c.startswith("SIM-") for c in conditions):
            cfg["conditions"] = (*conditions, "SIM-HONEST")
        extra.update({"min_shared_items": MIN_SHARED_ITEMS, "no_partner_score": NO_PARTNER_SCORE,
                      "item_sampling": cfg.get("item_sampling") or "simulator default (fixed)", "post_hoc": True})
        runs = [r for chunk in _map(_seed_job, [(s, cfg) for s in seeds], args.workers) for r in chunk]
    elif args.stage == "dose":
        cfg["conditions"] = ("SIM-FI", "SIM-BL")
        cfg["worlds"] = ("fi", "bl")
        jobs = [(i, s, cfg) for i in args.intensities for s in seeds]
        runs = [r for chunk in _map(_dose_worker, jobs, args.workers) for r in chunk]
        extra["intensities"] = args.intensities
    elif args.stage == "fi-dose":
        from .review import (
            FI_DOSE_LAMBDAS,
            FI_DOSE_PARAMETERS,
            fi_dose_worker,
            file_record,
            write_fi_dose_mechanisms,
        )
        lambdas = args.lambdas or list(FI_DOSE_LAMBDAS)
        directory = Path(args.exp_root) / "fi_dose_mechanisms"
        cfg.update({"conditions": ("SIM-FI", "SIM-BL"), "manipulation": True, "items_affected": True,
                    "breakdowns": False, "fi_mechanisms": {
                        str(lam): str(write_fi_dose_mechanisms(lam, Path(args.mechanisms),
                                                               directory / f"lambda_{lam}.json"))
                        for lam in lambdas}})
        jobs = [("fi", lam, s, cfg) for lam in lambdas for s in seeds]
        jobs += [("bl", x, s, cfg) for x in args.intensities for s in seeds]
        runs = [r for chunk in _map(fi_dose_worker, jobs, args.workers) for r in chunk]
        extra.update({"lambdas": lambdas, "intensities": args.intensities, "scaled_fi_parameters": FI_DOSE_PARAMETERS,
                      "mechanism_files": {k: file_record(Path(v)) for k, v in cfg["fi_mechanisms"].items()},
                      "post_hoc": True})
    elif args.stage == "protocol":
        cfg.update({"conditions": (*conditions, "SIM-HONEST") if any(c.startswith("SIM-") for c in conditions)
                    else conditions})
        from .review import LABEL_FREE_QUANTILE, PROTOCOL_VARIANTS
        extra.update({"variants": PROTOCOL_VARIANTS, "label_free_quantiles": LABEL_FREE_QUANTILE, "post_hoc": True})
        runs = [r for chunk in _map(_seed_job, [(s, cfg) for s in seeds], args.workers) for r in chunk]
    elif args.stage == "baseline":
        runs, info = _baseline(args, cfg, seeds, params, design)
        extra.update(info)
    elif args.stage == "sensitivity" and args.design == "v2":
        from .review import (
            file_record,
            sensitivity_v2_configs,
            sensitivity_v2_worker,
            write_mechanisms_v2,
        )
        cfg["conditions"] = ("SIM-FI", "SIM-BL")
        cfg["worlds"] = ("fi", "bl")
        configs, parameters, lhs = sensitivity_v2_configs(args.configs or 60, SENSITIVITY_PARAMETERS)
        directory = Path(args.exp_root) / "sensitivity_v2_mechanisms"
        jobs = [(-1, s, str(args.mechanisms), cfg) for s in seeds]
        files = {}
        for index, values in enumerate(configs):
            path = write_mechanisms_v2(values, Path(args.mechanisms), directory / f"config_{index:03d}.json")
            files[index] = file_record(path)
            jobs += [(index, s, str(path), cfg) for s in seeds]
        runs = [r for chunk in _map(sensitivity_v2_worker, jobs, args.workers) for r in chunk]
        extra.update({"design": "v2", "lhs": lhs, "parameters": [
            {"name": n, "low": b[0], "high": b[1], "log_uniform": b[2]} for n, b in parameters],
            "configs": configs, "mechanism_files": files, "default_config_id": -1, "post_hoc": True})
    elif args.stage == "sensitivity":
        cfg["conditions"] = ("SIM-FI", "SIM-BL")
        cfg["worlds"] = ("fi", "bl")
        configs = sensitivity_configs(args.configs or 40)
        directory = Path(args.exp_root) / "sensitivity_mechanisms"
        jobs = []
        files = {}
        for index, values in enumerate(configs):
            path = write_mechanisms(values, Path(args.mechanisms), directory / f"config_{index:03d}.json")
            files[index] = {"path": relative(path), "sha256": sha256_file(path)}
            jobs += [(index, s, str(path), values, cfg) for s in seeds]
        runs = [r for chunk in _map(_sensitivity_worker, jobs, args.workers) for r in chunk]
        extra.update({"lhs_seed": SENSITIVITY_SEED, "parameters": [
            {"name": n, "low": b[0], "high": b[1], "log_uniform": b[2]} for n, b in SENSITIVITY_PARAMETERS],
            "configs": configs, "mechanism_files": files, "design": "v1"})
    else:
        runs = [r for chunk in _map(_seed_job, [(s, cfg) for s in seeds], args.workers) for r in chunk]
    return {"stage": args.stage, "arguments": vars(args), "seeds": seeds, "design": design,
            "conditions": list(cfg["conditions"]),
            "simulator": {"binary": relative(binary), "available": have_sim,
                          "binary_sha256": sha256_file(binary) if have_sim else None},
            "params": {"path": relative(params), "source": params_source,
                       "sha256": sha256_file(params) if params.exists() else None},
            "mechanisms": {"path": relative(args.mechanisms), "sha256": sha256_file(args.mechanisms)},
            "legacy_generator_sha256": sha256_file(common.LEGACY_GENERATOR),
            "provenance": provenance(), **extra, "runs": runs}


def _baseline(args: argparse.Namespace, cfg: dict[str, Any], seeds: list[int], params: Path,
              design: dict[str, Any]) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    """Honest-baseline variants, each a complete main-protocol run on SIM-FI and SIM-BL."""
    from .common import REPO
    from .review import (
        AUTO_SUBMIT_JITTER_MS,
        CLOSING_QUANTILES,
        baseline_worker,
        file_record,
        natural_submission_quantiles,
        revision_multiplier,
        write_revision_params,
    )
    requested = args.variants or ["revision", "per_attempt", "closing_p90", "closing_p75"]
    root = Path(args.exp_root) / "baseline"
    nxt = Path(args.sim_binary_next) if args.sim_binary_next else REPO / "sim" / "bin" / "examory-sim-next"
    cfg.update({"conditions": ("SIM-FI", "SIM-BL"), "manipulation": True, "feature_diagnostics": True,
                "breakdowns": True, "variants": {}})
    info: dict[str, Any] = {"variants": {}, "skipped": {}, "post_hoc": True}
    mechanisms = Path(args.mechanisms)
    if "revision" in requested:
        result = revision_multiplier(params, mechanisms, Path(cfg["sim_binary"]), seeds[0] if 1001 not in seeds
                                     else 1001, design, root / "revision_search")
        path = write_revision_params(result["multiplier"], params, root / "params_revision.json")
        cfg["variants"]["revision"] = {"params": str(path)}
        info["variants"]["revision"] = {**result, "params_file": file_record(path)}
    if "per_attempt" in requested:
        cfg["variants"]["per_attempt"] = {"item_sampling": "per-attempt"}
        info["variants"]["per_attempt"] = {"item_sampling": "per-attempt"}
    closing = [v for v in requested if v.startswith("closing_")]
    if closing and not nxt.exists():
        info["skipped"].update({v: f"{relative(nxt)} not built" for v in closing})
    elif closing:
        estimate = natural_submission_quantiles(nxt, params, mechanisms, 1001, design, root / "closing_search")
        info["natural_submission"] = {**estimate, "binary": file_record(nxt)}
        for q in CLOSING_QUANTILES:
            name = f"closing_p{int(100 * q)}"
            if name not in closing:
                continue
            close = round(estimate[f"q{int(100 * q)}_ms"])
            flags = ["--close-after-ms", str(close), "--auto-submit-jitter-ms", str(AUTO_SUBMIT_JITTER_MS)]
            cfg["variants"][name] = {"sim_binary": str(nxt), "sim_extra_args": flags}
            info["variants"][name] = {"close_after_ms": close, "auto_submit_jitter_ms": AUTO_SUBMIT_JITTER_MS,
                                      "quantile": q, "binary": file_record(nxt)}
    jobs = [(v, s, cfg) for v in cfg["variants"] for s in seeds]
    runs = [r for chunk in _map(baseline_worker, jobs, args.workers) for r in chunk]
    return runs, info


if __name__ == "__main__":
    main()
