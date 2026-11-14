"""Compact committed stage files.

The full per-run output (every metric, per-seed ROC grids, generator commands) is
written under ``ml/datasets/raw/exp/stages_full/``. The committed file keeps, per
run, the per-seed scalar metrics in columnar form (``fields`` + value lists) and,
for the main stage, ROC curves only as mean and 2.5/97.5 % band over seeds on the
fixed FPR grid. ``expand_stage`` restores the nested layout used by ``analyze``.
"""
from __future__ import annotations

from collections import defaultdict
from typing import Any

import numpy as np

from .common import ROC_GRID

FIELD_PATHS: dict[str, tuple[str, ...]] = {
    "n_pos": ("n_pos",), "n_neg": ("n_neg",),
    "recall@5": ("at_0.05", "recall"), "fpr@5": ("at_0.05", "fpr"), "precision@5": ("at_0.05", "precision"),
    "f1@5": ("at_0.05", "f1"), "alerts_per_100@5": ("at_0.05", "alerts_per_100"),
    "recall@1": ("at_0.01", "recall"), "fpr@1": ("at_0.01", "fpr"), "precision@1": ("at_0.01", "precision"),
    "roc_auc": ("roc_auc",), "pauc_std": ("pauc_std",), "average_precision": ("average_precision",),
    "ci_recall_low": ("cohort_ci_0.05", "recall", "low"), "ci_recall_high": ("cohort_ci_0.05", "recall", "high"),
}
ALL_FIELDS = list(FIELD_PATHS)
STAGE_FIELDS = {
    "transfer": ["n_pos", "n_neg", "recall@5", "fpr@5", "recall@1", "fpr@1", "pauc_std"],
    "dose": ["n_pos", "n_neg", "recall@5", "fpr@5", "pauc_std"],
    "sensitivity": ["n_pos", "recall@5", "pauc_std"],
    "prevalence": ["n_pos", "n_neg", "recall@5", "fpr@5", "precision@5", "alerts_per_100@5", "pauc_std",
                   "ci_recall_low", "ci_recall_high"],
    "robust-detectors": ["n_pos", "n_neg", "recall@5", "fpr@5", "precision@5", "recall@1", "fpr@1", "roc_auc",
                         "pauc_std", "average_precision", "ci_recall_low", "ci_recall_high"],
    "supervised": ["n_pos", "n_neg", "recall@5", "fpr@5", "precision@5", "recall@1", "fpr@1", "roc_auc",
                   "pauc_std", "average_precision", "ci_recall_low", "ci_recall_high"],
    "similarity": ["n_pos", "n_neg", "recall@5", "fpr@5", "precision@5", "recall@1", "fpr@1", "roc_auc",
                   "pauc_std", "average_precision", "ci_recall_low", "ci_recall_high"],
}
RUN_KEYS = ("seed", "condition", "source", "target", "tag", "config_id", "intensity", "split_counts",
            "manipulation", "eligibility", "fit", "wall_seconds")


def _get(row: dict[str, Any] | None, path: tuple[str, ...]) -> Any:
    for key in path:
        if not isinstance(row, dict):
            return None
        row = row.get(key)
    return row


def _set(target: dict[str, Any], path: tuple[str, ...], value: Any) -> None:
    for key in path[:-1]:
        target = target.setdefault(key, {})
    target[path[-1]] = value


def _data(record: dict[str, Any] | None) -> dict[str, Any] | None:
    if not record:
        return None
    # 16-hex-digit prefixes identify the inputs; full hashes are in stages_full/ and generation.json
    return {k: (record.get(k) or "")[:16] or None for k in ("csv_sha256", "simulator_manifest_sha256")}


def roc_bands(runs: list[dict[str, Any]]) -> dict[str, Any]:
    curves: dict[str, list[list[float]]] = defaultdict(list)
    for run in runs:
        key = run.get("condition") or f"{run.get('source')}>{run.get('target')}"
        for detector, subsets in run["metrics"].items():
            for subset, row in subsets.items():
                if row.get("roc_tpr_grid"):
                    curves[f"{key}|{detector}|{subset}"].append(row["roc_tpr_grid"])
    out = {}
    for key, values in curves.items():
        data = np.asarray(values, dtype=float)
        out[key] = {"n": len(values), "mean": data.mean(axis=0).tolist(),
                    "low": np.quantile(data, .025, axis=0).tolist(),
                    "high": np.quantile(data, .975, axis=0).tolist()}
    return out


def slim_stage(stage: dict[str, Any]) -> dict[str, Any]:
    name = stage["stage"]
    if name == "ednet":
        return stage
    fields = STAGE_FIELDS.get(name, ALL_FIELDS)
    runs = []
    for run in stage["runs"]:
        slim = {k: run[k] for k in RUN_KEYS if k in run
                and not (k == "split_counts" and name in ("dose", "sensitivity", "transfer"))}
        slim["data"] = _data(run.get("data"))
        if "target_data" in run:
            slim["data"] = {"source": _data(run["source_data"]), "target": _data(run["target_data"])}
        slim["op"] = {det: {rate: [p["threshold"], p["validation_fpr"]] for rate, p in points.items()}
                      for det, points in run["operating_points"].items()}
        slim["m"] = {det: {subset: [_get(row, FIELD_PATHS[f]) for f in fields] for subset, row in subsets.items()}
                     for det, subsets in run["metrics"].items()}
        runs.append(slim)
    out = {k: v for k, v in stage.items() if k != "runs"}
    out["format"] = {"version": 1, "fields": fields, "op": ["threshold", "validation_fpr"],
                     "note": "per-seed scalar metrics; full output in ml/datasets/raw/exp/stages_full/"}
    if name == "main":
        out["roc"] = {"grid": ROC_GRID.tolist(), "curves": roc_bands(stage["runs"])}
    out["runs"] = runs
    return out


def expand_stage(stage: dict[str, Any]) -> dict[str, Any]:
    """Inverse of ``slim_stage`` for the fields that were kept (no-op on full files)."""
    if "format" not in stage:
        return stage
    fields = stage["format"]["fields"]
    runs = []
    for slim in stage["runs"]:
        run = {k: v for k, v in slim.items() if k not in ("m", "op")}
        run["operating_points"] = {det: {rate: {"threshold": v[0], "validation_fpr": v[1]}
                                         for rate, v in points.items()} for det, points in slim["op"].items()}
        run["metrics"] = {}
        for det, subsets in slim["m"].items():
            run["metrics"][det] = {}
            for subset, values in subsets.items():
                row: dict[str, Any] = {}
                for field, value in zip(fields, values, strict=True):
                    _set(row, FIELD_PATHS[field], value)
                run["metrics"][det][subset] = row
        runs.append(run)
    return dict(stage, runs=runs)


def main() -> None:
    """Re-derive the committed stage files from ml/datasets/raw/exp/stages_full/ (no re-scoring)."""
    import argparse

    from .common import EXP_DATA, STAGES_DIR, read_json, write_json
    parser = argparse.ArgumentParser(description=main.__doc__)
    parser.add_argument("--full-dir", default=str(EXP_DATA / "stages_full"))
    parser.add_argument("--out-dir", default=str(STAGES_DIR))
    args = parser.parse_args()
    from pathlib import Path
    for path in sorted(Path(args.full_dir).glob("*.json")):
        write_json(Path(args.out_dir) / path.name, slim_stage(read_json(path)), compact=True)


if __name__ == "__main__":
    main()
