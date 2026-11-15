"""Aggregate stage outputs into summary.json, Markdown tables and figures.

python -m ml.experiments.inflation.analyze [--stages-dir DIR] [--out-dir DIR] [--no-figures]

Seeds are the independent replicates: every interval below is a percentile
bootstrap over seeds (10 000 resamples, fixed RNG seed) unless stated otherwise.
"""
from __future__ import annotations

import argparse
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any

import numpy as np

from . import posthoc, review_analysis
from .common import (
    BL_TYPES,
    CONDITIONS,
    DETECTORS,
    FI_TYPES,
    RESULTS,
    ROBUST_DETECTORS,
    SLOT_LABEL,
    SLOT_OF_TYPE,
    SLOTS,
    provenance,
    read_json,
    relative,
    write_json,
)
from .compact import expand_stage
from .stats import holm, mean_ci, paired_difference, spearman, unpaired_difference

METRICS = {
    "recall@5": ("at_0.05", "recall"), "fpr@5": ("at_0.05", "fpr"), "precision@5": ("at_0.05", "precision"),
    "f1@5": ("at_0.05", "f1"), "alerts_per_100@5": ("at_0.05", "alerts_per_100"),
    "recall@1": ("at_0.01", "recall"), "fpr@1": ("at_0.01", "fpr"), "precision@1": ("at_0.01", "precision"),
    "roc_auc": ("roc_auc",), "pauc_std": ("pauc_std",), "average_precision": ("average_precision",),
}
PP_METRICS = {"recall@5", "fpr@5", "precision@5", "recall@1", "fpr@1", "precision@1", "f1@5"}
H1_SLOTS = ("S3", "S4")  # copy_peer (collusion slot) and proxy (score-jump slot)
H1_SUPPORT_PP, H1_REJECT_PP = 15.0, 5.0
CONDITION_STYLE = {"LEGACY": ("#E69F00", "s", "--"), "SIM-FI": ("#0072B2", "^", ":"),
                   "SIM-BL": ("#D55E00", "o", "-")}
DETECTOR_STYLE = {"iforest": ("#0072B2", "o", "-"), "iforest_no_peers": ("#56B4E9", "^", "--"),
                  "quantile_rules": ("#D55E00", "s", ":"), "quantile_rules_no_peers": ("#E69F00", "D", "-."),
                  "lof": ("#009E73", "v", "-"), "lof_no_peers": ("#009E73", "v", "--"),
                  "ocsvm": ("#CC79A7", "P", "-"), "ocsvm_no_peers": ("#CC79A7", "P", "--"),
                  "mcd": ("#000000", "X", "-"), "mcd_no_peers": ("#000000", "X", "--")}
SEED = 7
DETECTOR_LABEL = {"quantile_rules": "Rules", "iforest_no_peers": "IF", "iforest": "IF + peers",
                  "quantile_rules_no_peers": "Rules (no peers)"}
FIGURE_DETECTORS = ("quantile_rules", "iforest_no_peers", "iforest")
CONDITION_LABEL = {"LEGACY": "Legacy generator", "SIM-FI": "Feature-aligned injection (simulated normals)",
                   "SIM-BL": "Behaviour-level cheating"}
DEGENERATE_VALIDATION_FPR = 0.025
REALISM_ORDER = ("LEGACY", "SIM-defaults", "SIM-calibrated", "SIM-HONEST")
SHORT_SLOT = {"S1": "rapid /\npreknow.", "S2": "changes /\nlookup", "S3": "collusion /\ncopy peer",
              "S4": "score jump /\nproxy"}


# --------------------------------------------------------------------------- helpers


def get(row: dict[str, Any] | None, path: tuple[str, ...]) -> float | None:
    value: Any = row
    for key in path:
        if not isinstance(value, dict) or key not in value:
            return None
        value = value[key]
    return value if isinstance(value, (int, float)) and not isinstance(value, bool) else None


def series(runs: list[dict[str, Any]], seeds: list[int], detector: str, subset: str, metric: str,
           **match: Any) -> list[float | None]:
    """One value per seed (seed order), None when a seed lacks the subset."""
    by_seed = {}
    for run in runs:
        if all(run.get(k) == v for k, v in match.items()):
            by_seed[run["seed"]] = get(run["metrics"].get(detector, {}).get(subset), METRICS[metric])
    return [by_seed.get(seed) for seed in seeds]


def scale(metric: str, value: float | None) -> float | None:
    return None if value is None else (100 * value if metric in PP_METRICS else value)


def fmt_ci(summary: dict[str, Any] | None, metric: str = "", digits: int | None = None) -> str:
    if not summary or summary.get("mean") is None:
        return "NA"
    pp = metric in PP_METRICS
    digits = digits if digits is not None else (1 if pp else 3)
    f = (lambda x: f"{100 * x:.{digits}f}") if pp else (lambda x: f"{x:.{digits}f}")
    return f"{f(summary['mean'])} [{f(summary['low'])}, {f(summary['high'])}]"


def fmt_mcse(summary: dict[str, Any] | None, metric: str = "") -> str:
    if not summary or summary.get("mcse") is None:
        return "NA"
    return f"{100 * summary['mcse']:.2f}" if metric in PP_METRICS else f"{summary['mcse']:.4f}"


def fmt_p(p: float | None) -> str:
    if p is None:
        return "NA"
    return "<0.0001" if p < 1e-4 else f"{p:.4f}"


def table(header: list[str], rows: list[list[str]], align: str | None = None) -> str:
    align = align or "|" + "|".join(["---"] + ["---:"] * (len(header) - 1)) + "|"
    lines = ["| " + " | ".join(header) + " |", align]
    lines += ["| " + " | ".join(map(str, row)) + " |" for row in rows]
    return "\n".join(lines)


def cohort_ci_width(runs: list[dict[str, Any]], detector: str, subset: str, condition: str) -> float | None:
    widths = []
    for run in runs:
        if run.get("condition") != condition:
            continue
        ci = (run["metrics"].get(detector, {}).get(subset) or {}).get("cohort_ci_0.05", {}) or {}
        recall = ci.get("recall")
        if recall:
            widths.append(recall["high"] - recall["low"])
    return float(np.median(widths)) if widths else None


def slot_type(condition: str, slot: str) -> str:
    types = BL_TYPES if condition == "SIM-BL" else FI_TYPES
    return types[SLOTS.index(slot)]


# --------------------------------------------------------------------------- per-stage analyses


def summarize_cells(stage: dict[str, Any], detectors: tuple[str, ...]) -> dict[str, Any]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    conditions = [c for c in CONDITIONS if any(r["condition"] == c for r in runs)]
    subsets = ["all", "final"] + [f"slot:{s}" for s in SLOTS]
    cells: dict[str, Any] = {}
    for condition in conditions:
        for detector in detectors:
            for subset in subsets:
                key = f"{condition}|{detector}|{subset}"
                cells[key] = {metric: mean_ci(series(runs, seeds, detector, subset, metric, condition=condition),
                                              seed=SEED) for metric in METRICS}
                # metrics not stored for this stage (compact format) are omitted rather than kept as None
                cells[key] = {m: v for m, v in cells[key].items() if v["mean"] is not None}
                cells[key]["median_within_run_cohort_ci_width_recall@5"] = cohort_ci_width(
                    runs, detector, subset, condition)
                fprs = [r["operating_points"][detector]["0.05"]["validation_fpr"] for r in runs
                        if r["condition"] == condition and detector in r["operating_points"]]
                cells[key]["degenerate_seeds"] = sum(f < DEGENERATE_VALIDATION_FPR for f in fprs)
                cells[key]["n_seeds"] = len(fprs)
                if subset.startswith("slot:"):
                    cells[key]["type"] = slot_type(condition, subset[5:])
    return {"conditions": conditions, "seeds": seeds, "cells": cells}


def inflation(stage: dict[str, Any], detectors: tuple[str, ...]) -> dict[str, Any]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    present = {r["condition"] for r in runs}
    families: dict[str, list[dict[str, Any]]] = defaultdict(list)
    for metric in ("recall@5", "pauc_std"):
        for detector in detectors:
            for slot in SLOTS:
                subset = f"slot:{slot}"
                bl = series(runs, seeds, detector, subset, metric, condition="SIM-BL")
                if {"SIM-FI", "SIM-BL"} <= present:
                    fi = series(runs, seeds, detector, subset, metric, condition="SIM-FI")
                    test = paired_difference(fi, bl, seed=SEED)
                    families[f"SIM-FI-minus-SIM-BL|{metric}"].append(
                        {"metric": metric, "detector": detector, "slot": slot, **test})
                if {"LEGACY", "SIM-BL"} <= present:
                    legacy = series(runs, seeds, detector, subset, metric, condition="LEGACY")
                    test = unpaired_difference(legacy, bl, seed=SEED)
                    families[f"LEGACY-minus-SIM-BL|{metric}"].append(
                        {"metric": metric, "detector": detector, "slot": slot, **test})
    for rows in families.values():
        for row, adjusted in zip(rows, holm([r["p"] for r in rows]), strict=True):
            row["p_holm"] = adjusted
        if all("p_wilcoxon" in r for r in rows):
            for row, adjusted in zip(rows, holm([r["p_wilcoxon"] for r in rows]), strict=True):
                row["p_holm_wilcoxon"] = adjusted
    return dict(families)


def h1_test(stage: dict[str, Any]) -> dict[str, Any]:
    """Verification plan §0 H1 evaluated on each condition; the pre-specified test is SIM-BL."""
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    out: dict[str, Any] = {"criterion": {"support_pp": H1_SUPPORT_PP, "rules_adopted_below_pp": H1_REJECT_PP,
                                         "metric": "recall at validation-frozen FPR<=5% threshold",
                                         "compared": "iforest (with peer features) minus quantile_rules",
                                         "slots": {s: SLOT_LABEL[s] for s in H1_SLOTS},
                                         "decision_rule": "per slot on the across-seed mean gain: >=15 pp "
                                         "supported, <5 pp rules adopted, otherwise inconclusive; H1 "
                                         "supported only if both slots are supported; rules adopted if "
                                         "both are <5 pp; else mixed/inconclusive"}}
    for condition in CONDITIONS:
        if not any(r["condition"] == condition for r in runs):
            continue
        rows = []
        for slot in H1_SLOTS:
            subset = f"slot:{slot}"
            forest = series(runs, seeds, "iforest", subset, "recall@5", condition=condition)
            rules = series(runs, seeds, "quantile_rules", subset, "recall@5", condition=condition)
            test = paired_difference(forest, rules, seed=SEED)
            secondary = paired_difference(series(runs, seeds, "iforest_no_peers", subset, "recall@5",
                                                 condition=condition), rules, seed=SEED)
            gain = None if test["mean"] is None else 100 * test["mean"]
            verdict = ("NA" if gain is None else "supported" if gain >= H1_SUPPORT_PP else
                       "rules_adopted" if gain < H1_REJECT_PP else "inconclusive")
            rows.append({"slot": slot, "type": slot_type(condition, slot), "gain_pp": gain,
                         "test": test, "verdict": verdict, "iforest_no_peers_minus_rules": secondary,
                         "iforest_recall": mean_ci(forest, seed=SEED), "rules_recall": mean_ci(rules, seed=SEED)})
        for row, adjusted in zip(rows, holm([r["test"]["p"] for r in rows]), strict=True):
            row["p_holm"] = adjusted
        for row, adjusted in zip(rows, holm([r["test"].get("p_wilcoxon") for r in rows]), strict=True):
            row["p_holm_wilcoxon"] = adjusted
        verdicts = {r["verdict"] for r in rows}
        overall = ("H1 supported" if verdicts == {"supported"} else
                   "H1 not supported: rules adopted" if verdicts == {"rules_adopted"} else
                   "H1 mixed/inconclusive")
        out[condition] = {"rows": rows, "outcome": overall}
    out["primary_condition"] = "SIM-BL"
    out["primary_outcome"] = out.get("SIM-BL", {}).get("outcome", "not run (simulator unavailable)")
    return out


def breakdowns(stage: dict[str, Any]) -> list[dict[str, Any]]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    keys = sorted({k for r in runs if r["condition"] == "SIM-BL"
                   for d in r["metrics"].values() for k in d if k.startswith("truth:")
                   and not k.startswith("truth:copy_peer:mech_flag=")})  # duplicates role
    rows = []
    for key in keys:
        for detector in DETECTORS:
            values = series(runs, seeds, detector, key, "recall@5", condition="SIM-BL")
            counts = [((r["metrics"][detector].get(key) or {}).get("n_pos")) for r in runs
                      if r["condition"] == "SIM-BL" and detector in r["metrics"]]
            rows.append({"subset": key, "detector": detector, "recall@5": mean_ci(values, seed=SEED),
                         "pauc_std": mean_ci(series(runs, seeds, detector, key, "pauc_std", condition="SIM-BL"),
                                             seed=SEED),
                         "mean_positives_per_seed": float(np.mean([c for c in counts if c])) if any(counts) else 0})
    return rows


def copy_peer_sync(stage: dict[str, Any]) -> list[dict[str, Any]]:
    """S3 on SIM-FI and SIM-BL vs the SIM-BL non-sync / sync subsets (descriptive, outside Holm)."""
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    rows = []
    columns = {"SIM-FI collusion (all)": ("SIM-FI", "slot:S3"), "SIM-BL copy_peer (all)": ("SIM-BL", "slot:S3"),
               "SIM-BL copy_peer, non-sync pairs (≈ sync_submit_prob = 0)": ("SIM-BL", "truth:copy_peer:sync_submit=0"),
               "SIM-BL copy_peer, sync pairs": ("SIM-BL", "truth:copy_peer:sync_submit=1")}
    for detector in FIGURE_DETECTORS:
        for metric in ("recall@5", "pauc_std"):
            values = {name: series(runs, seeds, detector, subset, metric, condition=condition)
                      for name, (condition, subset) in columns.items()}
            if all(v is None for v in values["SIM-BL copy_peer, non-sync pairs (≈ sync_submit_prob = 0)"]):
                continue
            row = {"detector": detector, "metric": metric,
                   "cells": {name: mean_ci(v, seed=SEED) for name, v in values.items()},
                   "fi_minus_nonsync": paired_difference(
                       values["SIM-FI collusion (all)"],
                       values["SIM-BL copy_peer, non-sync pairs (≈ sync_submit_prob = 0)"], seed=SEED)}
            rows.append(row)
    return rows


MANIPULATION_FIELDS = ("score_points", "duration_s", "answer_changes", "median_first_answer_s", "share_changed")


def manipulation_summary(stage: dict[str, Any]) -> list[dict[str, Any]]:
    rows = []
    for condition in ("SIM-FI", "SIM-BL"):
        runs = [r for r in stage["runs"] if r["condition"] == condition and r.get("manipulation")]
        keys = sorted({k for r in runs for k in r["manipulation"]},
                      key=lambda k: (SLOT_OF_TYPE.get(k.split(":")[0], "S9"), k))
        for key in keys:
            row = {"condition": condition, "group": key, "slot": SLOT_OF_TYPE.get(key.split(":")[0]),
                   "n_per_seed": float(np.mean([r["manipulation"][key]["n"] for r in runs if key in r["manipulation"]]))}
            for field in MANIPULATION_FIELDS:
                row[field] = mean_ci([r["manipulation"][key][field] for r in runs if key in r["manipulation"]],
                                     seed=SEED)
            rows.append(row)
    return rows


def md_copy_peer(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return ""
    names = list(rows[0]["cells"])
    body = [[DETECTOR_LABEL[r["detector"]], r["metric"]] + [fmt_ci(r["cells"][n], r["metric"]) for n in names]
            + [fmt_ci(r["fi_minus_nonsync"], r["metric"])] for r in rows]
    return "\n".join([
        "## SIM-BL, copy_peer with sync_submit_prob = 0 (subset: non-synchronously submitting pairs)", "",
        ("**Subset analysis, not a separate simulation:** the non-sync column restricts SIM-BL copy_peer cheaters to "
        "pairs that did not submit within ~2 s (truth.csv `sync_submit = 0`), against all normal test attempts. "
        "Last column: SIM-FI collusion − SIM-BL non-sync copy_peer, paired by seed (bootstrap CI; descriptive, "
        "outside the Holm family). Recall in %, pAUC standardized."), "",
        table(["Detector", "Metric"] + names + ["SIM-FI − non-sync"], body,
              "|---|---|" + "---:|" * (len(names) + 1)), ""])


def md_manipulation(rows: list[dict[str, Any]]) -> str:
    if not rows:
        return ""
    body = [[r["condition"], r["group"], f"{r['n_per_seed']:.1f}"]
            + [fmt_ci(r[f], "", 2) for f in MANIPULATION_FIELDS[:4]] + [fmt_ci(r["share_changed"], "", 2)]
            for r in rows]
    return "\n".join([
        "## Manipulation check: what each injection/mechanism changed in observable terms", "",
        ("Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same "
        "student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out "
        "of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share "
        "of cheater attempts in which at least one of the four observables differs from the counterfactual."), "",
        table(["World", "Type", "cheaters/seed", "Δ score (pts)", "Δ duration (s)", "Δ answer changes",
               "Δ median first-answer time (s)", "share changed"], body, "|---|---|---:|---:|---:|---:|---:|---:|"),
        ""])


def transfer_summary(stage: dict[str, Any]) -> list[dict[str, Any]]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    rows = []
    for source, target in stage.get("pairs", []):
        for detector in DETECTORS:
            match = {"source": source, "target": target}
            row = {"source": source, "target": target, "detector": detector,
                   "fpr@5": mean_ci(series(runs, seeds, detector, "all", "fpr@5", **match), seed=SEED),
                   "recall@5": mean_ci(series(runs, seeds, detector, "all", "recall@5", **match), seed=SEED),
                   "fpr@1": mean_ci(series(runs, seeds, detector, "all", "fpr@1", **match), seed=SEED),
                   "slots": {}}
            for slot in SLOTS:
                row["slots"][slot] = mean_ci(series(runs, seeds, detector, f"slot:{slot}", "recall@5", **match),
                                             seed=SEED)
            rows.append(row)
    return rows


def dose_summary(stage: dict[str, Any]) -> list[dict[str, Any]]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    rows = []
    for intensity in stage.get("intensities", []):
        for condition in ("SIM-BL", "SIM-FI"):
            for detector in DETECTORS:
                for slot in SLOTS:
                    values = series(runs, seeds, detector, f"slot:{slot}", "recall@5",
                                    condition=condition, intensity=intensity)
                    rows.append({"intensity": intensity, "condition": condition, "detector": detector,
                                 "slot": slot, "type": slot_type(condition, slot),
                                 "recall@5": mean_ci(values, seed=SEED),
                                 "fpr@5": mean_ci(series(runs, seeds, detector, "all", "fpr@5",
                                                         condition=condition, intensity=intensity), seed=SEED)})
    return rows


def sensitivity_summary(stage: dict[str, Any]) -> dict[str, Any]:
    runs = stage["runs"]
    configs = stage.get("configs", [])
    names = [p["name"] for p in stage.get("parameters", [])]
    deltas: dict[str, dict[str, list[float | None]]] = defaultdict(dict)
    for metric in ("recall@5", "pauc_std"):
        for detector in DETECTORS:
            for slot in SLOTS:
                values = []
                for index in range(len(configs)):
                    seeds = sorted({r["seed"] for r in runs if r["config_id"] == index})
                    fi = series(runs, seeds, detector, f"slot:{slot}", metric, condition="SIM-FI", config_id=index)
                    bl = series(runs, seeds, detector, f"slot:{slot}", metric, condition="SIM-BL", config_id=index)
                    diff = [a - b for a, b in zip(fi, bl, strict=True) if a is not None and b is not None]
                    values.append(float(np.mean(diff)) if diff else None)
                deltas[metric][f"{detector}|{slot}"] = values
    summary: dict[str, Any] = {"n_configs": len(configs), "parameters": stage.get("parameters"), "cells": {}}
    for metric, by_cell in deltas.items():
        for cell, values in by_cell.items():
            data = np.asarray([v for v in values if v is not None], dtype=float)
            factor = 100 if metric == "recall@5" else 1
            entry: dict[str, Any] = {"metric": metric, "deltas": values}
            if len(data):
                entry.update({"median": float(np.median(data)) * factor,
                              "q25": float(np.quantile(data, .25)) * factor,
                              "q75": float(np.quantile(data, .75)) * factor,
                              "min": float(data.min()) * factor, "max": float(data.max()) * factor,
                              "fraction_gt_0": float(np.mean(data > 0)),
                              "fraction_gt_10pp": float(np.mean(data > 0.10)) if metric == "recall@5" else None})
            entry["spearman"] = {name: spearman([c[name] for c in configs], values) for name in names}
            summary["cells"][f"{metric}|{cell}"] = entry
    return summary


def prevalence_summary(stage: dict[str, Any]) -> dict[str, Any]:
    cells = summarize_cells(stage, DETECTORS)
    return cells


def ednet_summary(stage: dict[str, Any]) -> dict[str, Any]:
    runs = stage["runs"]
    out: dict[str, Any] = {"assumption": stage["assumption"], "peer_feature_note": stage["peer_feature_note"],
                           "ednet": stage["ednet"], "params": stage["params"], "flag_rates": {}, "realism": {},
                           "strata": {}}
    present = {s for r in runs for s in r["P1_transfer"]}
    sources = ["P2_refit"] + [f"P1:{s}" for s in REALISM_ORDER if s in present]
    detectors = sorted({d for r in runs for d in r["P2_refit"]})
    for source in sources:
        for detector in detectors:
            for rate in ("0.05", "0.01"):
                values, widths, ci_seeds = [], [], []
                for run in runs:
                    block = run["P2_refit"] if source == "P2_refit" else run["P1_transfer"].get(source[3:], {})
                    cell = block.get(detector, {}).get(rate)
                    if cell:
                        values.append(cell["flag_rate"])
                        if cell["cohort_ci"]:
                            widths.append((cell["cohort_ci"]["low"], cell["cohort_ci"]["high"]))
                            ci_seeds.append(run["seed"])
                out["flag_rates"][f"{source}|{detector}|{rate}"] = {
                    **mean_ci(values, seed=SEED),
                    "first_seed_cohort_ci": list(widths[0]) if widths else None,
                    "first_seed": ci_seeds[0] if ci_seeds else None}
            strata: dict[str, Any] = {}
            for run in runs:
                block = run["P2_refit"] if source == "P2_refit" else run["P1_transfer"].get(source[3:], {})
                cell = block.get(detector, {}).get("0.05")
                if not cell:
                    continue
                for column, bins in cell["strata"].items():
                    for name, value in bins.items():
                        strata.setdefault(f"{column}:{name}", []).append(value["flag_rate"])
            out["strata"][f"{source}|{detector}"] = {k: mean_ci(v, seed=SEED) for k, v in strata.items()}
    order = [s for s in REALISM_ORDER if any(s in r["realism"] for r in runs)]
    for source in order:
        aucs = [r["realism"][source]["auc"] for r in runs if source in r["realism"]]
        secondary = [r["realism_all_base"][source]["auc"] for r in runs if source in r.get("realism_all_base", {})]
        first_run = next(r for r in runs if source in r["realism"])
        first = first_run["realism"][source]
        first_all_base = (first_run.get("realism_all_base") or {}).get(source, {})
        features = first["distances"]
        out["realism"][source] = {
            "c2st_auc": mean_ci(aucs, seed=SEED), "c2st_features": first["features"],
            "c2st_auc_all_base": mean_ci(secondary, seed=SEED),
            "first_seed_user_bootstrap_ci": first["auc_user_bootstrap_ci"],
            "first_seed_user_bootstrap_ci_all_base": first_all_base.get("auc_user_bootstrap_ci"),
            "first_seed": first_run["seed"],
            "n_real": first["n_real"], "n_synthetic": first["n_synthetic"],
            "distances": {f: {"ks": mean_ci([r["realism"][source]["distances"][f]["ks"] for r in runs
                                             if source in r["realism"]], seed=SEED),
                              "wasserstein_std": mean_ci([r["realism"][source]["distances"][f]["wasserstein_std"]
                                                          for r in runs if source in r["realism"]], seed=SEED)}
                          for f in features}}
    return out


# --------------------------------------------------------------------------- Markdown


def md_main(cells: dict[str, Any], detectors: tuple[str, ...], title: str, note: str) -> str:
    conditions = cells["conditions"]
    parts = [f"# {title}", "", note, ""]
    if any(c.get("degenerate_seeds") for c in cells["cells"].values()):
        parts += [("**† UNRELIABLE:** in the marked k/n seeds the detector's validation FPR at the 5 % target was "
                  f"below {100 * DEGENERATE_VALIDATION_FPR:.1f} % (degenerate score distribution, e.g. a singular "
                  "MCD covariance), so the frozen threshold is not a 5 % operating point."), ""]
    for metric, label in (("recall@5", "Recall (%) at the validation-frozen 5 % FPR threshold"),
                          ("fpr@5", "Realized test FPR (%) at the validation-frozen 5 % threshold"),
                          ("recall@1", "Recall (%) at the validation-frozen 1 % FPR threshold"),
                          ("fpr@1", "Realized test FPR (%) at the 1 % threshold"),
                          ("precision@5", "Precision (%) at the 5 % threshold"),
                          ("pauc_std", "Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)"),
                          ("roc_auc", "ROC-AUC"), ("average_precision", "Average precision")):
        rows = []
        for detector in detectors:
            for subset in ["all", "final"] + [f"slot:{s}" for s in SLOTS]:
                if metric.startswith("fpr") and subset.startswith("slot:"):
                    continue  # identical negatives for every slot
                name = subset if not subset.startswith("slot:") else f"{subset[5:]} {SLOT_LABEL[subset[5:]]}"
                row = [detector, name]
                for c in conditions:
                    cell = cells["cells"].get(f"{c}|{detector}|{subset}", {})
                    text = fmt_ci(cell.get(metric), metric)
                    if cell.get("degenerate_seeds"):
                        text += f" † {cell['degenerate_seeds']}/{cell['n_seeds']}"
                    row += [text, fmt_mcse(cell.get(metric), metric)]
                rows.append(row)
        header = ["Detector", "Subset"] + [h for c in conditions for h in (c, f"MCSE {c}")]
        parts += [f"## {label}", "", table(header, rows, "|---|---|" + "---:|" * (2 * len(conditions))), ""]
    parts.append("Cells: mean over seeds [95 % percentile-bootstrap CI over seeds]; MCSE = Monte Carlo standard "
                 "error of the mean, SD of the per-seed values / √n (same units). Slot rows compare that "
                 "slot's cheaters with all normal test attempts (LEGACY/SIM-FI type ↔ SIM-BL mechanism).")
    rows = []
    for detector in detectors:
        for slot in SLOTS:
            widths = [cells["cells"].get(f"{c}|{detector}|slot:{slot}", {}).get(
                "median_within_run_cohort_ci_width_recall@5") for c in conditions]
            rows.append([detector, slot] + ["NA" if w is None else f"{100 * w:.1f}" for w in widths])
    parts += ["", "## Median width (pp) of the within-run cohort-bootstrap 95 % CI of recall@5 %", "",
              "What a single-dataset study would report as its uncertainty.", "",
              table(["Detector", "Slot"] + conditions, rows, "|---|---|" + "---:|" * len(conditions)), ""]
    return "\n".join(parts)


def md_inflation(families: dict[str, Any]) -> str:
    sizes = sorted({len(rows) for rows in families.values()})
    size = " or ".join(str(n) for n in sizes)
    parts = ["# Inflation of detector performance by feature-aligned injection", "",
             ("Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in "
             "standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, exact two-sided "
             "sign-flip permutation test on the per-seed differences, all 2^n sign patterns). LEGACY − SIM-BL is "
             "unpaired (independent bootstrap CI, two-sided Mann–Whitney U). MCSE = Monte Carlo standard error of "
             "Δ (paired: SD of the per-seed differences / √n; unpaired: √(s²_x/n_x + s²_y/n_y)). Holm correction "
             f"within each family (one family per comparison × metric) of {size} slot × detector tests "
             "(number of detectors × 4 slots); the family size is printed with each block."), ""]
    for name, rows in families.items():
        comparison, metric = name.split("|")
        body = [[r["detector"], f"{r['slot']} {SLOT_LABEL[r['slot']]}", fmt_ci(r, metric), fmt_mcse(r, metric),
                 str(r.get("n", r.get("n_x"))), fmt_p(r["p"]), fmt_p(r["p_holm"])] for r in rows]
        parts += [f"## {comparison}, {metric} (Holm family of {len(rows)})", "",
                  table(["Detector", "Slot", "Δ mean [95 % CI]", "MCSE", "n", "p", "p (Holm)"], body,
                        "|---|---|---:|---:|---:|---:|---:|"), ""]
    return "\n".join(parts)


def md_h1(h1: dict[str, Any]) -> str:
    parts = ["# Hypothesis H1 (verification plan §0)", "",
             (f"Criterion: {h1['criterion']['compared']}, {h1['criterion']['metric']}; slots "
             f"{', '.join(f'{k} ({v})' for k, v in h1['criterion']['slots'].items())}. "
             f"Decision rule (pre-specified): {h1['criterion']['decision_rule']}."), "",
             f"**Primary (SIM-BL) outcome: {h1['primary_outcome']}**", ""]
    rows = []
    for condition in CONDITIONS:
        if condition not in h1:
            continue
        for r in h1[condition]["rows"]:
            rows.append([condition, f"{r['slot']} {r['type']}", fmt_ci(r["iforest_recall"], "recall@5"),
                         fmt_ci(r["rules_recall"], "recall@5"), fmt_ci(r["test"], "recall@5"), fmt_p(r["test"]["p"]),
                         fmt_p(r["p_holm"]), r["verdict"], fmt_ci(r["iforest_no_peers_minus_rules"], "recall@5")])
        rows.append([condition, "**overall**", "", "", "", "", "", h1[condition]["outcome"], ""])
    parts.append(table(["Condition", "Slot", "iforest recall %", "rules recall %", "gain pp [CI]", "p (sign-flip)",
                        "p (Holm, 2 slots)", "verdict", "iforest_no_peers − rules pp"], rows,
                       "|---|---|---:|---:|---:|---:|---:|---|---:|"))
    return "\n".join(parts) + "\n"


def md_breakdown(rows: list[dict[str, Any]]) -> str:
    body = [[r["subset"].replace("truth:", ""), r["detector"], fmt_ci(r["recall@5"], "recall@5"),
             fmt_ci(r["pauc_std"], "pauc_std"), f"{r['mean_positives_per_seed']:.1f}"] for r in rows]
    return "\n".join(["# SIM-BL recall by latent truth sub-mechanism", "",
                      "Each row: that sub-group's cheaters vs all normal test attempts; mean over seeds [95 % CI].",
                      "", table(["Sub-mechanism", "Detector", "Recall@5 %", "pAUC (std)", "positives/seed"], body,
                                "|---|---|---:|---:|---:|"), ""])


def md_transfer(rows: list[dict[str, Any]]) -> str:
    body = [[f"{r['source']} → {r['target']}", r["detector"], fmt_ci(r["fpr@5"], "fpr@5"),
             fmt_ci(r["recall@5"], "recall@5")] + [fmt_ci(r["slots"][s], "recall@5") for s in SLOTS]
            for r in rows]
    return "\n".join(["# Transfer: validated on X, deployed on Y", "",
                      ("Reference, model and 5 % threshold from X (train/validation cohorts); applied to the test "
                      "cohorts of Y (same seed and cohort partition). Values in %, mean over seeds [95 % CI]."), "",
                      table(["X → Y", "Detector", "realized FPR", "recall (all)"]
                            + [f"recall {s} (Y type)" for s in SLOTS], body,
                            "|---|---|---:|---:|" + "---:|" * len(SLOTS)), ""])


def md_dose(rows: list[dict[str, Any]]) -> str:
    body = [[f"{r['intensity']:g}", r["condition"], r["detector"], f"{r['slot']} {r['type']}",
             fmt_ci(r["recall@5"], "recall@5"), fmt_ci(r["fpr@5"], "fpr@5")] for r in rows]
    return "\n".join(["# Dose–response: recall at the frozen 5 % threshold vs mechanism intensity", "",
                      ("--intensity overrides mechanism coverage/propensity (and the proxy gain scale) in the "
                      "simulator. Mean over seeds [95 % CI], %."), "",
                      table(["Intensity", "World", "Detector", "Slot", "Recall", "FPR"], body,
                            "|---:|---|---|---|---:|---:|"), ""])


def md_sensitivity(summary: dict[str, Any]) -> str:
    rows, cors = [], []
    names = [p["name"] for p in summary["parameters"] or []]
    for key, cell in summary["cells"].items():
        metric, detector, slot = key.split("|")
        if "median" not in cell:
            continue
        unit = "pp" if metric == "recall@5" else ""
        rows.append([metric, detector, f"{slot} {SLOT_LABEL[slot]}",
                     f"{cell['median']:.3g}{unit} [{cell['q25']:.3g}, {cell['q75']:.3g}]",
                     f"{cell['min']:.3g} … {cell['max']:.3g}", f"{100 * cell['fraction_gt_0']:.0f} %",
                     "NA" if cell["fraction_gt_10pp"] is None else f"{100 * cell['fraction_gt_10pp']:.0f} %"])
        cors.append([metric, detector, slot] + [
            "NA" if cell["spearman"][n]["rho"] is None else
            f"{cell['spearman'][n]['rho']:+.2f}{'*' if cell['spearman'][n]['p'] < 0.05 else ''}" for n in names])
    return "\n".join([
        "# Sensitivity of inflation to assumed mechanism parameters", "",
        (f"{summary['n_configs']} Latin-hypercube configurations × 3 seeds. Δ per configuration = mean over its "
        "seeds of [metric(SIM-FI) − metric(SIM-BL)] (paired)."), "",
        table(["Metric", "Detector", "Slot", "median Δ [IQR]", "range", "configs Δ>0", "configs Δ>10 pp"], rows,
              "|---|---|---|---:|---:|---:|---:|"), "",
        "## Spearman ρ between parameter and Δ (* nominal p < 0.05, uncorrected; exploratory)", "",
        table(["Metric", "Detector", "Slot"] + names, cors, "|---|---|---|" + "---:|" * len(names)), ""])


def md_prevalence(cells: dict[str, Any]) -> str:
    conditions = cells["conditions"]
    rows = []
    for detector in DETECTORS:
        for subset in ["all", "final"] + [f"slot:{s}" for s in SLOTS]:
            cell = {c: cells["cells"].get(f"{c}|{detector}|{subset}", {}) for c in conditions}
            rows.append([detector, subset] + [
                f"P {fmt_ci(cell[c].get('precision@5'), 'precision@5')}; R {fmt_ci(cell[c].get('recall@5'), 'recall@5')}; "
                f"alerts/100 {fmt_ci(cell[c].get('alerts_per_100@5'), 'alerts', 2)}" for c in conditions])
    return "\n".join(["# Low prevalence (anomaly-rate 0.0167 ≈ 5 % of final attempts)", "",
                      ("Precision (P, %), recall (R, %) and alerts per 100 test attempts at the frozen 5 % threshold; "
                      "mean over seeds [95 % CI]."), "",
                      table(["Detector", "Subset"] + conditions, rows, "|---|---|" + "---|" * len(conditions)), ""])


def md_ednet(summary: dict[str, Any]) -> str:
    parts = ["# EdNet (real, unlabelled) — flag rates and realism", "", f"**Assumption.** {summary['assumption']}",
             "", f"**Peer features.** {summary['peer_feature_note']}", "",
             (f"EdNet input: `{summary['ednet']['csv']}` (SHA-256 `{summary['ednet']['sha256'][:16]}…`); "
             f"subsample: {summary['ednet']['subsample']}. Simulator params: `{summary['params']['path']}` "
             f"({summary['params']['source']})."), "", "## Flag rate on EdNet test cohorts (%)", ""]
    rows = []
    first_seeds = {v.get("first_seed") for v in summary["flag_rates"].values() if v.get("first_seed") is not None}
    first_seed = first_seeds.pop() if len(first_seeds) == 1 else "first"
    for key, value in summary["flag_rates"].items():
        source, detector, rate = key.split("|")
        ci = value.get("first_seed_cohort_ci")
        rows.append([source, detector, f"{100 * float(rate):.0f} %", fmt_ci(value, "fpr@5"),
                     "NA" if not ci else f"[{100 * ci[0]:.1f}, {100 * ci[1]:.1f}]",
                     "peer features" if detector in ("iforest", "quantile_rules") else ""])
    n_seeds = max((v.get("n") or 0) for v in summary["flag_rates"].values()) if summary["flag_rates"] else 0
    parts += [(f"Two different intervals: the across-seed interval is the 95 % percentile bootstrap of the mean "
               f"flag rate over the {n_seeds} seeds (between-replicate variation); the cohort-bootstrap interval "
               f"is the within-run 95 % interval (500 resamples of EdNet test cohorts) of seed {first_seed} only "
               "(sampling variation of one deployment)."), "",
              table(["Fitted on", "Detector", "nominal FPR", "flag rate: mean over seeds [across-seed 95 % CI]",
                     f"flag rate: cohort-bootstrap 95 % CI (seed {first_seed})", "flag"], rows,
                    "|---|---|---:|---:|---:|---|"), ""]
    parts += ["## Who is flagged (5 % threshold): flag rate (%) by stratum", "",
              ("Strata use EdNet's own-reference features for every row (so P1 and P2 strata are the same "
               "attempts). Quartiles of score, median answer time and fast-answer fraction use cut points from "
               "EdNet training cohorts; a quartile can be empty or merged when the cut points tie."), ""]
    rows = []
    for key, strata in summary["strata"].items():
        source, detector = key.split("|")
        if detector in ("iforest", "quantile_rules"):
            continue
        rows.append([source, detector] + [fmt_ci(strata.get(k), "fpr@5") for k in sorted(strata)])
        header = sorted(strata)
    if rows:
        parts += [table(["Fitted on", "Detector"] + header, rows, "|---|---|" + "---:|" * len(header)), ""]
    sources = list(summary["realism"])
    first = summary["realism"][sources[0]] if sources else {}
    parts += ["## Realism: classifier two-sample test (real EdNet vs synthetic normal attempts)", "",
              ("AUC 0.5 = indistinguishable. HistGradientBoosting, 5-fold StratifiedGroupKFold by user. "
               f"Primary: behavioural features only ({', '.join(first.get('c2st_features', []))}); secondary: all "
               "BASE_FEATURES incl. history_missing and z_score, which reflect the attempts-per-user design. "
               "SIM sources are the honest world with --item-sampling per-attempt."), ""]
    def user_ci(source: str, key: str) -> str:
        ci = summary["realism"][source].get(key)
        return "NA" if not ci else f"[{ci['low']:.3f}, {ci['high']:.3f}]"
    seed_label = summary["realism"][sources[0]].get("first_seed", "first") if sources else "first"
    rows = [["C2ST AUC, behavioural features (primary): mean over seeds [across-seed 95 % CI]"]
            + [fmt_ci(summary["realism"][s]["c2st_auc"]) for s in sources],
            [f"C2ST AUC, behavioural features (primary): user-bootstrap 95 % CI (seed {seed_label})"]
            + [user_ci(s, "first_seed_user_bootstrap_ci") for s in sources],
            ["C2ST AUC, all base features (secondary): mean over seeds [across-seed 95 % CI]"]
            + [fmt_ci(summary["realism"][s]["c2st_auc_all_base"]) for s in sources],
            [f"C2ST AUC, all base features (secondary): user-bootstrap 95 % CI (seed {seed_label})"]
            + [user_ci(s, "first_seed_user_bootstrap_ci_all_base") for s in sources],
            ["n real / synthetic (per seed)"]
            + [f"{summary['realism'][s]['n_real']}/{summary['realism'][s]['n_synthetic']}" for s in sources]]
    parts += [table([""] + sources, rows, "|---|" + "---:|" * len(sources)), ""]
    features = list(first.get("distances", {}))
    for measure, label in (("ks", "KS statistic"), ("wasserstein_std", "Wasserstein distance / SD(EdNet)")):
        rows = [[f] + [fmt_ci(summary["realism"][s]["distances"][f][measure], "", 2) for s in sources]
                for f in features]
        parts += [f"## Per-feature distance to EdNet: {label} (mean over seeds [CI])", "",
                  table(["Feature"] + sources, rows, "|---|" + "---:|" * len(sources)), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- figures


def _plt():
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    plt.rcParams.update({"font.size": 8, "axes.labelsize": 8, "legend.fontsize": 7, "xtick.labelsize": 7,
                         "ytick.labelsize": 7, "axes.spines.top": False, "axes.spines.right": False,
                         "savefig.bbox": "tight", "pdf.fonttype": 42, "ps.fonttype": 42})
    return plt


def save(fig, directory: Path, name: str) -> list[str]:
    directory.mkdir(parents=True, exist_ok=True)
    paths = []
    for extension in ("png", "pdf"):
        path = directory / f"{name}.{extension}"
        fig.savefig(path, dpi=300, metadata={"CreationDate": None} if extension == "pdf" else {})
        if extension == "png":  # 256-colour palette keeps line art identical and the file small
            from PIL import Image
            with Image.open(path) as image:
                image.convert("RGB").quantize(colors=256, method=Image.Quantize.MEDIANCUT).save(
                    path, optimize=True)
        paths.append(relative(path))
    return paths


def figure_recall(cells: dict[str, Any], detectors: tuple[str, ...], directory: Path, name: str) -> list[str]:
    plt = _plt()
    conditions = cells["conditions"]
    columns = min(len(detectors), 3)
    rows_n = -(-len(detectors) // columns)
    fig, axes = plt.subplots(rows_n, columns, figsize=(3.1 * columns, 2.8 * rows_n), sharey=True, squeeze=False)
    axes = [list(np.ravel(axes))]
    for ax, detector in zip(axes[0], detectors, strict=True):
        for k, condition in enumerate(conditions):
            color, marker, _ = CONDITION_STYLE[condition]
            xs, ys, lo, hi = [], [], [], []
            for i, slot in enumerate(SLOTS):
                summary = cells["cells"].get(f"{condition}|{detector}|slot:{slot}", {}).get("recall@5")
                if summary and summary["mean"] is not None:
                    xs.append(i + (k - (len(conditions) - 1) / 2) * 0.22)
                    ys.append(100 * summary["mean"])
                    lo.append(100 * (summary["mean"] - summary["low"]))
                    hi.append(100 * (summary["high"] - summary["mean"]))
            ax.errorbar(xs, ys, yerr=[lo, hi], fmt=marker, color=color, mfc=color if k != 1 else "white",
                        ms=4, capsize=2, lw=0.8, label=CONDITION_LABEL.get(condition, condition))
        ax.set_xticks(range(len(SLOTS)), [f"{s}\n{SHORT_SLOT[s]}" for s in SLOTS], fontsize=6)
        ax.set_ylim(-3, 103)
        ax.text(0.5, 1.03, DETECTOR_LABEL.get(detector, detector), transform=ax.transAxes, ha="center",
                va="bottom", fontsize=7.5)
        ax.grid(axis="y", lw=0.3, alpha=0.5)
    for index in range(0, len(axes[0]), columns):
        axes[0][index].set_ylabel("Recall at frozen 5 % FPR (%)")
    if rows_n > 1:
        fig.subplots_adjust(hspace=0.6)
    handles, labels = axes[0][0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="lower center", ncol=len(conditions), frameon=False,
               bbox_to_anchor=(0.5, -0.12 / rows_n), fontsize=6.5)
    paths = save(fig, directory, name)
    plt.close(fig)
    return paths


def figure_roc(stage: dict[str, Any], directory: Path) -> list[str]:
    plt = _plt()
    runs = stage["runs"]
    curves_all = stage["roc"]["curves"]
    grid = np.asarray(stage["roc"]["grid"])
    conditions = [c for c in CONDITIONS if any(r["condition"] == c for r in runs)]
    subsets = ["all"] + [f"slot:{s}" for s in SLOTS]
    fig, axes = plt.subplots(len(subsets), len(conditions), figsize=(2.3 * len(conditions), 1.9 * len(subsets)),
                             sharex=True, sharey=True, squeeze=False)
    for i, subset in enumerate(subsets):
        for j, condition in enumerate(conditions):
            ax = axes[i][j]
            for detector in FIGURE_DETECTORS:
                band = curves_all.get(f"{condition}|{detector}|{subset}")
                if not band:
                    continue
                color, _, style = DETECTOR_STYLE[detector]
                ax.plot(100 * grid, 100 * np.asarray(band["mean"]), style, color=color, lw=1,
                        label=DETECTOR_LABEL[detector])
                ax.fill_between(100 * grid, 100 * np.asarray(band["low"]), 100 * np.asarray(band["high"]),
                                color=color, alpha=0.15, lw=0)
            ax.plot([0, 10], [0, 10], color="grey", lw=0.5)
            ax.axvline(5, color="grey", lw=0.4, ls=(0, (1, 2)))
            label = condition if subset == "all" else f"{condition}: {slot_type(condition, subset[5:])}"
            ax.text(0.5, 1.02, label, transform=ax.transAxes, ha="center", va="bottom", fontsize=6.5)
            ax.set_ylim(0, 100)
            if i == len(subsets) - 1:
                ax.set_xlabel("FPR (%)")
            if j == 0:
                ax.set_ylabel(("all anomalies" if subset == "all" else subset[5:]) + "\nTPR (%)")
    handles, labels = axes[0][0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="lower center", ncol=len(labels), frameon=False, bbox_to_anchor=(0.5, -0.02))
    fig.subplots_adjust(hspace=0.35)
    paths = save(fig, directory, "F2_roc_zoom")
    plt.close(fig)
    return paths


def figure_transfer(rows: list[dict[str, Any]], directory: Path) -> list[str]:
    plt = _plt()
    present = [c for c in CONDITIONS if any(r["source"] == c or r["target"] == c for r in rows)]
    fig, axes = plt.subplots(2, len(DETECTORS), figsize=(3.3 * len(DETECTORS), 5.6), squeeze=False,
                             gridspec_kw={"hspace": 0.55, "wspace": 0.35})
    for j, detector in enumerate(DETECTORS):
        for i, metric in enumerate(("fpr@5", "recall@5")):
            grid = np.full((len(present), len(present)), np.nan)
            for r in rows:
                if r["detector"] == detector and r[metric]["mean"] is not None:
                    grid[present.index(r["source"]), present.index(r["target"])] = 100 * r[metric]["mean"]
            ax = axes[i][j]
            cmap = plt.get_cmap("cividis").copy()
            cmap.set_bad("#eeeeee")
            image = ax.imshow(grid, cmap=cmap, vmin=0, vmax=20 if metric == "fpr@5" else 100)
            for a in range(len(present)):
                for b in range(len(present)):
                    if np.isfinite(grid[a, b]):
                        ax.text(b, a, f"{grid[a, b]:.1f}", ha="center", va="center", fontsize=7,
                                color="white" if grid[a, b] < (10 if metric == "fpr@5" else 50) else "black")
            ax.set_xticks(range(len(present)), present, rotation=30)
            ax.set_yticks(range(len(present)), present if j == 0 else [""] * len(present))
            ax.set_xlabel("deployed on (Y)")
            if j == 0:
                ax.set_ylabel("validated on (X)")
            ax.text(0.5, 1.03, f"{detector}: {'realized FPR' if metric == 'fpr@5' else 'recall'} (%)",
                    transform=ax.transAxes, ha="center", va="bottom", fontsize=6.5)
            fig.colorbar(image, ax=ax, fraction=0.046, pad=0.04)
    paths = save(fig, directory, "F3_transfer")
    plt.close(fig)
    return paths


def figure_dose(rows: list[dict[str, Any]], directory: Path) -> list[str]:
    plt = _plt()
    fig, axes = plt.subplots(1, len(SLOTS), figsize=(9, 2.4), sharey=True)
    for ax, slot in zip(axes, SLOTS, strict=True):
        for detector in FIGURE_DETECTORS:
            data = sorted((r["intensity"], r["recall@5"]) for r in rows
                          if r["slot"] == slot and r["detector"] == detector and r["condition"] == "SIM-BL")
            if not data or any(d[1]["mean"] is None for d in data):
                continue
            x = [d[0] for d in data]
            color, marker, style = DETECTOR_STYLE[detector]
            ax.plot(x, [100 * d[1]["mean"] for d in data], style, marker=marker, ms=3, color=color, lw=1,
                    label=DETECTOR_LABEL[detector])
            ax.fill_between(x, [100 * d[1]["low"] for d in data], [100 * d[1]["high"] for d in data],
                            color=color, alpha=0.15, lw=0)
        ax.axhline(5, color="grey", lw=0.4, ls=(0, (1, 2)))
        ax.set_xlabel("mechanism intensity")
        ax.text(0.5, 1.02, BL_TYPES[SLOTS.index(slot)], transform=ax.transAxes, ha="center", va="bottom",
                fontsize=7)
        ax.set_ylim(0, 100)
    axes[0].set_ylabel("SIM-BL recall at frozen 5 % FPR (%)")
    handles, labels = axes[0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="lower center", ncol=len(labels), frameon=False, bbox_to_anchor=(0.5, -0.1))
    paths = save(fig, directory, "F4_dose_response")
    plt.close(fig)
    return paths


def figure_sensitivity(summary: dict[str, Any], directory: Path) -> list[str]:
    plt = _plt()
    fig, axes = plt.subplots(1, len(SLOTS), figsize=(9, 2.6), sharey=True)
    rng = np.random.default_rng(SEED)
    for ax, slot in zip(axes, SLOTS, strict=True):
        for k, detector in enumerate(FIGURE_DETECTORS):
            values = [100 * v for v in summary["cells"][f"recall@5|{detector}|{slot}"]["deltas"] if v is not None]
            if not values:
                continue
            color, marker, _ = DETECTOR_STYLE[detector]
            ax.boxplot([values], positions=[k], widths=0.5, showfliers=False,
                       medianprops={"color": "black"}, boxprops={"lw": 0.7}, whiskerprops={"lw": 0.7})
            ax.scatter(k + rng.uniform(-0.15, 0.15, len(values)), values, s=6, marker=marker, color=color,
                       alpha=0.7, lw=0)
        ax.axhline(0, color="black", lw=0.5)
        ax.axhline(10, color="grey", lw=0.5, ls="--")
        ax.set_xticks(range(len(FIGURE_DETECTORS)), [DETECTOR_LABEL[d] for d in FIGURE_DETECTORS])
        ax.text(0.5, 1.02, SLOT_LABEL[slot], transform=ax.transAxes, ha="center", va="bottom", fontsize=6.5)
    axes[0].set_ylabel("Δ recall, SIM-FI − SIM-BL (pp)")
    paths = save(fig, directory, "F5_sensitivity")
    plt.close(fig)
    return paths


def figure_ednet(summary: dict[str, Any], directory: Path) -> list[str]:
    plt = _plt()
    fig, axes = plt.subplots(1, 3, figsize=(12.5, 3.2), gridspec_kw={"width_ratios": [2.4, 1.0, 2], "wspace": 0.75})
    sources = list(dict.fromkeys(k.split("|")[0] for k in summary["flag_rates"]))
    detectors = sorted({k.split("|")[1] for k in summary["flag_rates"]})
    ax = axes[0]
    for k, detector in enumerate(detectors):
        color, marker, _ = DETECTOR_STYLE.get(detector, ("grey", "o", "-"))
        for i, source in enumerate(sources):
            value = summary["flag_rates"].get(f"{source}|{detector}|0.05")
            if not value or value["mean"] is None:
                continue
            x = i + (k - (len(detectors) - 1) / 2) * 0.18
            ax.errorbar([x], [100 * value["mean"]], yerr=[[100 * (value["mean"] - value["low"])],
                                                          [100 * (value["high"] - value["mean"])]],
                        fmt=marker, color=color, mfc="white" if detector in ("iforest", "quantile_rules") else color,
                        ms=4, capsize=2, lw=0.8, label=DETECTOR_LABEL.get(detector, detector) if i == 0 else None)
    ax.axhline(5, color="grey", lw=0.6, ls="--")
    ax.set_xticks(range(len(sources)), [s.replace("P1:", "P1 ").replace("P2_refit", "P2 EdNet") for s in sources],
                  rotation=20)
    ax.set_ylabel("EdNet test flag rate at nominal 5 % (%)")
    ax.legend(frameon=False, fontsize=6)
    ax = axes[1]
    realism = summary["realism"]
    for i, (source, value) in enumerate(realism.items()):
        auc = value["c2st_auc"]
        if auc["mean"] is None:
            continue
        ax.errorbar([i], [auc["mean"]], yerr=[[auc["mean"] - auc["low"]], [auc["high"] - auc["mean"]]],
                    fmt="o", color="black", ms=4, capsize=2)
    ax.axhline(0.5, color="grey", lw=0.6, ls="--")
    ax.set_xticks(range(len(realism)), list(realism), rotation=30)
    ax.set_xlim(-0.6, len(realism) - 0.4)
    ax.set_ylim(0.45, 1.01)
    ax.set_ylabel("C2ST AUC (real vs synthetic)")
    ax = axes[2]
    features = sorted({f for v in realism.values() for f in v["distances"]})
    hatches = ["", "////", "...."]
    colours = ["#BBBBBB", "#666666", "#111111"]
    width = 0.8 / max(len(realism), 1)
    for k, (source, value) in enumerate(realism.items()):
        ys = [value["distances"][f]["wasserstein_std"]["mean"] or 0 for f in features]
        ax.barh(np.arange(len(features)) + (k - (len(realism) - 1) / 2) * width, ys, height=width,
                color=colours[k % 3], hatch=hatches[k % 3], edgecolor="white", label=source)
    ax.set_yticks(range(len(features)), features)
    ax.set_xlabel("Wasserstein distance / SD(EdNet)")
    ax.legend(frameon=False, fontsize=6)
    paths = save(fig, directory, "F6_ednet")
    plt.close(fig)
    return paths


# --------------------------------------------------------------------------- driver


def analyze(stages_dir: Path, out_dir: Path, figures: bool = True) -> dict[str, Any]:
    tables_dir, figures_dir = out_dir / "tables", out_dir / "figures"
    tables_dir.mkdir(parents=True, exist_ok=True)
    summary: dict[str, Any] = {"provenance": provenance(), "stages": {}, "tables": [], "figures": [],
                               "analysis_seed": SEED, "missing_stages": []}

    def load(name: str) -> dict[str, Any] | None:
        path = stages_dir / f"{name}.json"
        if not path.exists():
            summary["missing_stages"].append(name)
            return None
        stage = expand_stage(read_json(path))
        summary["stages"][name] = {"path": relative(path), "seeds": stage.get("seeds"),
                                   "conditions": stage.get("conditions"), "params": stage.get("params"),
                                   "wall_seconds": (stage.get("timing") or {}).get("wall_seconds")}
        return stage

    def write(name: str, text: str) -> None:
        path = tables_dir / f"{name}.md"
        path.write_text(text, encoding="utf-8")
        summary["tables"].append(relative(path))

    main = load("main")
    if main:
        cells = summarize_cells(main, DETECTORS)
        families = inflation(main, DETECTORS)
        h1 = h1_test(main)
        summary["main"] = cells
        summary["inflation"] = families
        summary["h1"] = h1
        sync_rows = copy_peer_sync(main)
        manipulation_rows = manipulation_summary(main)
        summary["copy_peer_sync"] = sync_rows
        summary["manipulation"] = manipulation_rows
        write("main", md_main(cells, DETECTORS, "Main experiment: detector performance by condition",
                              f"Design {main['design']}; seeds {min(main['seeds'])}–{max(main['seeds'])} "
                              f"(n = {len(main['seeds'])}); simulator params `{main['params']['path']}` "
                              f"({main['params']['source']}).")
              + "\n" + md_copy_peer(sync_rows) + "\n" + md_manipulation(manipulation_rows))
        write("inflation", md_inflation(families))
        write("h1", md_h1(h1))
        rows = breakdowns(main)
        summary["breakdowns"] = rows
        if rows:
            write("breakdown", md_breakdown(rows))
        if figures:
            summary["figures"] += figure_recall(cells, FIGURE_DETECTORS, figures_dir, "F1_recall_by_slot")
            summary["figures"] += figure_roc(main, figures_dir)
    transfer = load("transfer")
    if transfer:
        rows = transfer_summary(transfer)
        summary["transfer"] = rows
        write("transfer", md_transfer(rows))
        if figures:
            summary["figures"] += figure_transfer(rows, figures_dir)
    robust = load("robust_detectors")
    if robust:
        cells = summarize_cells(robust, ROBUST_DETECTORS)
        families = inflation(robust, ROBUST_DETECTORS)
        summary["robust_detectors"] = {"cells": cells, "inflation": families}
        write("robust_detectors", md_main(cells, ROBUST_DETECTORS, "Appendix: other unsupervised detectors",
                                          "LOF (novelty), One-class SVM (RBF, nu = 0.05, standardized), robust "
                                          "Mahalanobis (MinCovDet) on the frozen model matrix, with and without "
                                          "peer features; same protocol as the main experiment."))
        write("robust_inflation", md_inflation(families))
        if figures:
            summary["figures"] += figure_recall(cells, ROBUST_DETECTORS, figures_dir, "FA1_recall_robust")
    dose = load("dose")
    if dose:
        rows = dose_summary(dose)
        summary["dose"] = rows
        write("dose", md_dose(rows))
        if figures:
            summary["figures"] += figure_dose(rows, figures_dir)
    sensitivity = load("sensitivity")
    if sensitivity and sensitivity.get("design") == "v2":  # replaces v1 (stages/sensitivity_v1.json)
        result = review_analysis.sensitivity_v2_summary(sensitivity)
        summary["sensitivity"] = result
        write("sensitivity", review_analysis.md_sensitivity_v2(result))
        if figures:
            summary["figures"] += review_analysis.figure_sensitivity_v2(result, figures_dir)
    elif sensitivity:
        result = sensitivity_summary(sensitivity)
        summary["sensitivity"] = result
        write("sensitivity", md_sensitivity(result))
        if figures:
            summary["figures"] += figure_sensitivity(result, figures_dir)
    prevalence = load("prevalence")
    if prevalence:
        cells = prevalence_summary(prevalence)
        summary["prevalence"] = cells
        write("prevalence", md_prevalence(cells))
    ednet = load("ednet")
    if ednet:
        result = ednet_summary(ednet)
        summary["ednet"] = result
        write("ednet", md_ednet(result))
        if figures:
            summary["figures"] += figure_ednet(result, figures_dir)
    # Post hoc, exploratory stages (README amendment of 15 Nov 2026); appended after the pre-planned ones.
    supervised = load("supervised")
    if supervised:
        result = posthoc.supervised_summary(supervised, main, series)
        summary["supervised"] = result
        write("supervised", posthoc.md_supervised(result, fmt_ci, fmt_p, table))
        if figures:
            summary["figures"] += posthoc.figure_supervised(result, summary.get("main"), figures_dir, _plt, save)
    similarity = load("similarity")
    if similarity:
        result = posthoc.similarity_summary(similarity, main, series)
        summary["similarity"] = result
        write("similarity", posthoc.md_similarity(result, fmt_ci, table))
    # Stages and tables added after independent review of the draft (README amendment of 15 Nov 2026).
    fi_dose = load("fi_dose")
    if fi_dose:
        result = review_analysis.fi_dose_summary(fi_dose, main, dose)
        summary["fi_dose"] = result
        write("fi_dose", review_analysis.md_fi_dose(result))
        if figures:
            summary["figures"] += review_analysis.figure_fi_dose(result, figures_dir)
    protocol = load("protocol")
    if protocol:
        result = review_analysis.protocol_summary(protocol, main)
        summary["protocol"] = result
        write("protocol", review_analysis.md_protocol(result))
    if main:
        result = review_analysis.subpopulation_summary(main, protocol)
        summary["subpopulations"] = result
        write("subpopulations", review_analysis.md_subpopulations(result))
    baseline = load("baseline")
    if baseline:
        result = review_analysis.baseline_summary(baseline, main)
        summary["baseline"] = result
        write("baseline", review_analysis.md_baseline(
            result, (summary.get("protocol") or {}).get("feature_diagnostics")))
    if main and prevalence:
        result = review_analysis.prevalence_diagnostic(main, prevalence)
        summary["prevalence_diagnostic"] = result
        write("prevalence_diagnostic", review_analysis.md_prevalence_diagnostic(result))
    crossed = review_analysis.crossed_p_values(summary)
    summary["pvalue_changes"] = crossed
    write("pvalue_changes", review_analysis.md_crossed(
        crossed, ["inflation", "h1", "copy_peer_sync", "robust_detectors", "supervised"]))
    write_json(out_dir / "summary.json", summary, compact=True, digits=4)
    return summary


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stages-dir", type=Path, default=RESULTS / "stages")
    parser.add_argument("--out-dir", type=Path, default=RESULTS)
    parser.add_argument("--no-figures", action="store_true")
    args = parser.parse_args(argv)
    summary = analyze(args.stages_dir, args.out_dir, not args.no_figures)
    print(f"analyze: {len(summary['tables'])} tables, {len(summary['figures'])} figure files; "
          f"missing stages: {summary['missing_stages']}", file=sys.stderr)
    if "h1" in summary:
        print(f"H1 primary outcome: {summary['h1']['primary_outcome']}", file=sys.stderr)


if __name__ == "__main__":
    main()
