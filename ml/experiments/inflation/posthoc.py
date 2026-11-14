"""Aggregation of the post hoc stages ``supervised`` and ``similarity`` (exploratory).

Both stages were added on 15 Nov 2026 after the main results had been seen (README,
amendment log). Intervals are percentile bootstraps over seeds and paired contrasts use the
same helpers (``stats.mean_ci``, ``stats.paired_difference``) and analysis seed as ``analyze``.
p-values are two-sided Wilcoxon signed-rank tests, unadjusted (no Holm family is declared).
"""
from __future__ import annotations

from pathlib import Path
from typing import Any

import numpy as np

from .common import BL_TYPES, DETECTORS, FI_TYPES, SLOT_LABEL, SLOTS
from .stats import mean_ci, paired_difference

SUPERVISED_DETECTORS = ("hgb", "hgb_no_peers", "logreg", "logreg_no_peers")
SUPERVISED_LABEL = {"hgb": "HGB + peers (15)", "hgb_no_peers": "HGB (12)", "logreg": "LogReg + peers (15)",
                    "logreg_no_peers": "LogReg (12)"}
PRIMARY = "hgb"
SIMILARITY_DETECTORS = ("answersim", "answersim_incorrect")
SIMILARITY_LABEL = {"answersim": "AnswerSim", "answersim_incorrect": "AnswerSim (incorrect only)"}
UNSUPERVISED_LABEL = {"quantile_rules": "Rules", "iforest_no_peers": "IF", "iforest": "IF + peers"}
SYNC = ("truth:copy_peer:sync_submit=0", "truth:copy_peer:sync_submit=1")
SYNC_LABEL = {SYNC[0]: "copy_peer, non-sync pairs", SYNC[1]: "copy_peer, sync pairs"}
SHORT_SLOT = {"S1": "rapid /\npreknow.", "S2": "changes /\nlookup", "S3": "collusion /\ncopy peer",
              "S4": "score jump /\nproxy"}
CELL_METRICS = ("recall@5", "fpr@5", "recall@1", "fpr@1", "roc_auc", "pauc_std", "average_precision")
SHORT = {"SIM-FI": "FI", "SIM-BL": "BL", "LEGACY": "LEGACY", "SIM-HONEST": "HONEST"}


def _subsets() -> list[str]:
    return ["all"] + [f"slot:{s}" for s in SLOTS]


def _type(world: str, subset: str) -> str:
    if subset in SYNC_LABEL:
        return SYNC_LABEL[subset]
    if subset == "all":
        return "all cheating"
    if subset == "final":
        return "final attempts"
    slot = subset[5:]
    return f"{slot} {(BL_TYPES if world == 'SIM-BL' else FI_TYPES)[SLOTS.index(slot)]}"


# --------------------------------------------------------------------------- supervised


def best_unsupervised(main: dict[str, Any], seeds: list[int], subset: str, series) -> tuple[str, list]:
    """Unsupervised detector with the highest mean SIM-BL recall@5 % on ``subset`` in the main stage."""
    best, best_mean, best_values = None, -1.0, []
    for detector in DETECTORS:
        values = series(main["runs"], seeds, detector, subset, "recall@5", condition="SIM-BL")
        finite = [v for v in values if v is not None]
        if finite and float(np.mean(finite)) > best_mean:
            best, best_mean, best_values = detector, float(np.mean(finite)), values
    return best, best_values


def main_pairing(stage: dict[str, Any], main: dict[str, Any]) -> dict[str, Any]:
    """Check that the supervised SIM-BL test data are the main stage's SIM-BL data (same CSV hash)."""
    main_hash = {r["seed"]: (r.get("data") or {}).get("csv_sha256") for r in main["runs"]
                 if r.get("condition") == "SIM-BL"}
    ours = {r["seed"]: ((r.get("data") or {}).get("target") or r.get("target_data") or {}).get("csv_sha256")
            for r in stage["runs"] if r.get("target") == "SIM-BL"}
    matched = [s for s in sorted(ours) if ours[s] and main_hash.get(s) and
               (ours[s].startswith(main_hash[s]) or main_hash[s].startswith(ours[s]))]
    return {"seeds": len(ours), "matched": len(matched), "all_matched": len(matched) == len(ours) > 0}


def supervised_summary(stage: dict[str, Any], main: dict[str, Any] | None, series) -> dict[str, Any]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    pairs = [tuple(p) for p in stage["pairs"]]
    cells: dict[str, Any] = {}
    for source, target in pairs:
        subsets = _subsets() + (list(SYNC) if target == "SIM-BL" else [])
        for detector in SUPERVISED_DETECTORS:
            for subset in subsets:
                values = {m: mean_ci(series(runs, seeds, detector, subset, m, source=source, target=target),
                                     seed=7) for m in CELL_METRICS}
                cells[f"{source}>{target}|{detector}|{subset}"] = {m: v for m, v in values.items()
                                                                    if v["mean"] is not None}
    fit = {}
    for detector in SUPERVISED_DETECTORS:
        info = [r["fit"][detector] for r in runs if detector in (r.get("fit") or {})]
        fit[detector] = {"mean_n_iter": float(np.mean([i["n_iter"] for i in info])) if info else None,
                         "all_converged": all(i.get("converged", True) for i in info)}
    contrasts = []
    have = set(pairs)
    for detector in SUPERVISED_DETECTORS:
        for subset in _subsets():
            def cell(source: str, target: str, detector: str = detector, subset: str = subset) -> list:
                return series(runs, seeds, detector, subset, "recall@5", source=source, target=target)
            if {("SIM-FI", "SIM-FI"), ("SIM-FI", "SIM-BL")} <= have:
                contrasts.append({"contrast": "FI→FI − FI→BL", "detector": detector, "subset": subset,
                                  **paired_difference(cell("SIM-FI", "SIM-FI"), cell("SIM-FI", "SIM-BL"), seed=7)})
            if {("SIM-BL", "SIM-BL"), ("SIM-FI", "SIM-BL")} <= have:
                contrasts.append({"contrast": "BL→BL − FI→BL", "detector": detector, "subset": subset,
                                  **paired_difference(cell("SIM-BL", "SIM-BL"), cell("SIM-FI", "SIM-BL"), seed=7)})
            if main and ("SIM-BL", "SIM-BL") in have:
                best, values = best_unsupervised(main, seeds, subset, series)
                if best:
                    contrasts.append({"contrast": "BL→BL − best unsupervised on BL", "detector": detector,
                                      "subset": subset, "comparator": best,
                                      **paired_difference(cell("SIM-BL", "SIM-BL"), values, seed=7)})
    return {"pairs": pairs, "seeds": seeds, "primary": PRIMARY, "cells": cells, "fit": fit,
            "contrasts": contrasts, "main_pairing": main_pairing(stage, main) if main else None,
            "note": "post hoc, exploratory; p-values unadjusted"}


def md_supervised(summary: dict[str, Any], fmt_ci, fmt_p, table) -> str:
    pairs = summary["pairs"]
    parts = ["# Post hoc: supervised detectors on the frozen feature set (exploratory)", "",
             ("**Added after the main results had been seen (amendment of 15 Nov 2026); exploratory, p-values "
              "unadjusted.** Feature table: the frozen pipeline's (`build_features` with the training world's "
              "normal-train reference, `train.matrix`), 15 features (+ peers) or 12 attempt features. Models with "
              "scikit-learn defaults, no tuning: HistGradientBoostingClassifier (`random_state = seed`, primary) and "
              "LogisticRegression on standardized features (secondary). Labels: any cheating vs none, training "
              "world's train cohorts. Threshold: `choose_threshold` (max recall s.t. FPR ≤ 5 %) on the training "
              "world's validation cohorts. Metrics on the target world's test cohorts. Mean over seeds [95 % "
              "percentile-bootstrap CI over seeds]; recall and FPR in %."), ""]
    if summary.get("main_pairing"):
        mp = summary["main_pairing"]
        parts += [(f"SIM-BL test data identical to the main stage's (CSV SHA-256 prefix) in {mp['matched']} of "
                   f"{mp['seeds']} seeds."), ""]
    for metric, label in (("recall@5", "Recall (%) at the training-world validation 5 % FPR threshold"),
                          ("pauc_std", "Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)"),
                          ("roc_auc", "ROC-AUC")):
        rows = []
        for detector in SUPERVISED_DETECTORS:
            for source, target in pairs:
                key = f"{source}>{target}|{detector}"
                row = [SUPERVISED_LABEL[detector], f"{SHORT[source]} → {SHORT[target]}"]
                if metric == "recall@5":
                    row.append(fmt_ci(summary["cells"][f"{key}|all"].get("fpr@5"), "fpr@5"))
                row += [fmt_ci(summary["cells"][f"{key}|{s}"].get(metric), metric) for s in _subsets()]
                rows.append(row)
        header = ["Model", "Train → test"] + (["realized FPR"] if metric == "recall@5" else []) + [
            "all"] + [f"{s} {SLOT_LABEL[s]}" for s in SLOTS]
        parts += [f"## {label}", "", table(header, rows, "|---|---|" + "---:|" * (len(header) - 2)), ""]
    rows = []
    for detector in SUPERVISED_DETECTORS:
        for source, target in pairs:
            if target != "SIM-BL":
                continue
            key = f"{source}>{target}|{detector}"
            rows.append([SUPERVISED_LABEL[detector], f"{SHORT[source]} → BL"]
                        + [fmt_ci(summary["cells"].get(f"{key}|{s}", {}).get("recall@5"), "recall@5") for s in SYNC])
    parts += ["## SIM-BL copy_peer by synchronous submission (recall %, 5 % threshold)", "",
              table(["Model", "Train → test"] + [SYNC_LABEL[s] for s in SYNC], rows, "|---|---|---:|---:|"), ""]
    rows = []
    for c in summary["contrasts"]:
        rows.append([c["contrast"], SUPERVISED_LABEL[c["detector"]] + (" (primary)" if c["detector"] == PRIMARY
                                                                        else ""),
                     "all" if c["subset"] == "all" else f"{c['subset'][5:]} {SLOT_LABEL[c['subset'][5:]]}",
                     UNSUPERVISED_LABEL.get(c.get("comparator"), ""), fmt_ci(c, "recall@5"), str(c["n"]),
                     fmt_p(c["p"])])
    parts += ["## Pre-declared contrasts (recall@5 %, pp; paired by seed)", "",
              ("Paired percentile-bootstrap CI (10 000 resamples) and two-sided Wilcoxon signed-rank p, "
               "unadjusted. Slot rows compare the slot's FI type in FI→FI with its BL mechanism in FI→BL. "
               "'Best unsupervised' = the main-stage detector (Rules, IF, IF + peers) with the highest mean "
               "SIM-BL recall on that subset (chosen on the same seeds, which favours the comparator)."), "",
              table(["Contrast", "Model", "Subset", "comparator", "Δ mean [95 % CI]", "n", "p"], rows,
                    "|---|---|---|---|---:|---:|---:|"), ""]
    fit = summary["fit"]
    parts += [("Fit diagnostics: " + "; ".join(
        f"{SUPERVISED_LABEL[d]} mean iterations {fit[d]['mean_n_iter']:.1f}"
        + ("" if d.startswith("hgb") else f", all converged: {fit[d]['all_converged']}")
        for d in SUPERVISED_DETECTORS if fit[d]["mean_n_iter"] is not None) + "."), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- similarity


def similarity_summary(stage: dict[str, Any], main: dict[str, Any] | None, series) -> dict[str, Any]:
    runs, seeds = stage["runs"], sorted(set(stage["seeds"]))
    conditions = [c for c in stage["conditions"] if any(r["condition"] == c for r in runs)]
    cells: dict[str, Any] = {}
    for condition in conditions:
        subsets = ["all", "final"] + ([f"slot:{s}" for s in SLOTS] if condition != "SIM-HONEST" else []) + (
            list(SYNC) if condition == "SIM-BL" else [])
        for detector in SIMILARITY_DETECTORS:
            for subset in subsets:
                values = {m: mean_ci(series(runs, seeds, detector, subset, m, condition=condition), seed=7)
                          for m in CELL_METRICS}
                cells[f"{condition}|{detector}|{subset}"] = {m: v for m, v in values.items()
                                                              if v["mean"] is not None}
            fprs = [r["operating_points"][detector]["0.05"]["validation_fpr"] for r in runs
                    if r["condition"] == condition]
            cells[f"{condition}|{detector}|all"]["validation_fpr@5"] = mean_ci(fprs, seed=7)
    eligibility = {}
    for condition in conditions:
        rows = [r["eligibility"] for r in runs if r["condition"] == condition and r.get("eligibility")]
        if not rows:
            continue
        eligibility[condition] = {
            "max_shared_items_min": min(r["max_shared_items_with_another_student"]["min"] for r in rows),
            "max_shared_items_median": float(np.median([r["max_shared_items_with_another_student"]["median"]
                                                        for r in rows])),
            "eligible_partners_min": min(r["eligible_partners"]["min"] for r in rows),
            "eligible_partners_median": float(np.median([r["eligible_partners"]["median"] for r in rows])),
            "share_with_eligible_partner_min": min(r["share_with_eligible_partner"] for r in rows),
            "share_no_partner_incorrect_mean": float(np.mean([r.get("share_no_partner_score:answersim_incorrect", 0)
                                                              for r in rows])),
            "answer_key_verified": sum(bool((r.get("answer_key") or {}).get("verified")) for r in rows),
            "runs": len(rows)}
    reference = {}
    if main:
        main_seeds = sorted(set(main["seeds"]))
        for detector in DETECTORS:
            for subset in ("all", "slot:S3", *SYNC):
                reference[f"SIM-BL|{detector}|{subset}"] = mean_ci(
                    series(main["runs"], main_seeds, detector, subset, "recall@5", condition="SIM-BL"), seed=7)
            for condition in ("SIM-FI", "LEGACY"):
                reference[f"{condition}|{detector}|slot:S3"] = mean_ci(
                    series(main["runs"], main_seeds, detector, "slot:S3", "recall@5", condition=condition), seed=7)
    return {"conditions": conditions, "seeds": seeds, "cells": cells, "eligibility": eligibility,
            "main_reference_recall@5": reference, "item_sampling": stage.get("item_sampling"),
            "min_shared_items": stage.get("min_shared_items"), "note": "post hoc, exploratory"}


def md_similarity(summary: dict[str, Any], fmt_ci, table) -> str:
    conditions = [c for c in summary["conditions"] if c != "SIM-HONEST"]
    parts = ["# Post hoc: answer-similarity detector AnswerSim (exploratory)", "",
             ("**Added after the main results had been seen (amendment of 15 Nov 2026); exploratory.** For two "
              "attempts of different students in the same cohort and quiz sharing ≥ 10 items: z = (matches − Σ p_k) / "
              "√Σ p_k(1 − p_k), p_k = Σ_o f_ko² from the option frequencies of all train-cohort attempts (no labels); "
              "attempt score = max over partners. Secondary: the same sum over items where the scored attempt's "
              "final answer is wrong (key recovered from the submitted scores and verified on every attempt). "
              "Submission times are not used. Same protocol as the main stage: threshold = max validation recall "
              "s.t. validation FPR ≤ 5 %, metrics on test cohorts; mean over seeds [95 % CI]; recall and FPR in %."),
             ""]
    elig = summary["eligibility"]
    if elig:
        rows = [[c, str(e["max_shared_items_min"]), f"{e['max_shared_items_median']:g}",
                 f"{e['eligible_partners_min']}", f"{e['eligible_partners_median']:g}",
                 f"{100 * e['share_with_eligible_partner_min']:.1f}",
                 f"{100 * e['share_no_partner_incorrect_mean']:.2f}", f"{e['answer_key_verified']}/{e['runs']}"]
                for c, e in elig.items()]
        parts += [f"## Item sharing (`--item-sampling` {summary.get('item_sampling')})", "",
                  table(["World", "min |S| (best partner)", "median |S|", "min eligible partners",
                         "median eligible partners", "% attempts with a partner (min over seeds)",
                         "% without wrong-answer partner (incorrect variant)", "answer key verified"], rows,
                        "|---|---:|---:|---:|---:|---:|---:|---:|"), ""]
    for metric, label in (("recall@5", "Recall (%) at the validation-frozen 5 % FPR threshold"),
                          ("pauc_std", "Standardized partial AUC, FPR ∈ [0, 0.05]"), ("roc_auc", "ROC-AUC")):
        rows = []
        for detector in SIMILARITY_DETECTORS:
            for subset in ["all"] + [f"slot:{s}" for s in SLOTS] + list(SYNC):
                name = subset if subset == "all" else (SYNC_LABEL.get(subset)
                                                       or f"{subset[5:]} {SLOT_LABEL[subset[5:]]}")
                rows.append([SIMILARITY_LABEL[detector], name] + [
                    fmt_ci(summary["cells"].get(f"{c}|{detector}|{subset}", {}).get(metric), metric)
                    for c in conditions])
        parts += [f"## {label}", "", table(["Detector", "Subset"] + conditions, rows,
                                           "|---|---|" + "---:|" * len(conditions)), ""]
    rows = []
    for detector in SIMILARITY_DETECTORS:
        for condition in summary["conditions"]:
            cell = summary["cells"].get(f"{condition}|{detector}|all", {})
            rows.append([SIMILARITY_LABEL[detector], condition, fmt_ci(cell.get("validation_fpr@5"), "fpr@5"),
                         fmt_ci(cell.get("fpr@5"), "fpr@5"), fmt_ci(cell.get("fpr@1"), "fpr@1")])
    parts += ["## Operating point: validation and realized test FPR (%)", "",
              ("The score is discrete (with one shared item set the null terms are the same for every pair, so z "
               "is a monotone function of the match count); the largest threshold step with validation FPR ≤ 5 % "
               "can lie well below 5 %. SIM-HONEST: honest world (no cheaters), threshold from its own validation "
               "cohorts."), "",
              table(["Detector", "World", "validation FPR @5 %", "test FPR @5 %", "test FPR @1 %"], rows,
                    "|---|---|---:|---:|---:|"), ""]
    reference = summary.get("main_reference_recall@5") or {}
    if reference:
        rows = []
        for subset in ("all", "slot:S3", *SYNC):
            name = "all cheating" if subset == "all" else SYNC_LABEL.get(subset, "S3 copy_peer (all pairs)")
            row = [name] + [fmt_ci(summary["cells"].get(f"SIM-BL|{d}|{subset}", {}).get("recall@5"), "recall@5")
                            for d in SIMILARITY_DETECTORS]
            row += [fmt_ci(reference.get(f"SIM-BL|{d}|{subset}"), "recall@5") for d in DETECTORS]
            rows.append(row)
        parts += ["## SIM-BL: AnswerSim next to the main-stage detectors (recall %, 5 % threshold)", "",
                  "Main-stage columns are taken from `main.json` (same seeds and data); descriptive, no test.", "",
                  table(["Subset"] + [SIMILARITY_LABEL[d] for d in SIMILARITY_DETECTORS]
                        + [f"{UNSUPERVISED_LABEL[d]} (main)" for d in DETECTORS], rows,
                        "|---|" + "---:|" * (len(SIMILARITY_DETECTORS) + len(DETECTORS))), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- figure


def figure_supervised(summary: dict[str, Any], main_cells: dict[str, Any] | None, directory: Path, plt_factory,
                      save) -> list[str]:
    plt = plt_factory()
    series_spec = [("SIM-FI>SIM-FI", "FI → FI", "#0072B2", "^", "white"),
                   ("SIM-FI>SIM-BL", "FI → BL", "#0072B2", "^", "#0072B2"),
                   ("SIM-BL>SIM-BL", "BL → BL", "#D55E00", "o", "#D55E00")]
    # LEGACY -> BL is left out of the figure: its realized test FPR is far above 5 % (table).
    detectors = ("hgb", "hgb_no_peers")
    fig, axes = plt.subplots(1, len(detectors), figsize=(3.4 * len(detectors), 2.8), sharey=True, squeeze=False)
    for ax, detector in zip(axes[0], detectors, strict=True):
        specs = [s for s in series_spec if f"{s[0]}|{detector}|all" in summary["cells"]]
        n = len(specs) + (1 if main_cells else 0)
        for k, (pair, label, color, marker, face) in enumerate(specs):
            xs, ys, lo, hi = [], [], [], []
            for i, slot in enumerate(SLOTS):
                cell = summary["cells"][f"{pair}|{detector}|slot:{slot}"].get("recall@5")
                if cell:
                    xs.append(i + (k - (n - 1) / 2) * 0.16)
                    ys.append(100 * cell["mean"])
                    lo.append(100 * (cell["mean"] - cell["low"]))
                    hi.append(100 * (cell["high"] - cell["mean"]))
            ax.errorbar(xs, ys, yerr=[lo, hi], fmt=marker, color=color, mfc=face, ms=4, capsize=2, lw=0.8,
                        label=label)
        if main_cells:
            xs, ys = [], []
            for i, slot in enumerate(SLOTS):
                best = max((main_cells["cells"].get(f"SIM-BL|{d}|slot:{slot}", {}).get("recall@5", {}).get("mean")
                            or 0.0) for d in DETECTORS)
                xs.append(i + (n - 1 - (n - 1) / 2) * 0.16)
                ys.append(100 * best)
            ax.plot(xs, ys, "x", color="black", ms=5, label="best unsupervised on BL (main)")
        ax.set_xticks(range(len(SLOTS)), [f"{s}\n{SHORT_SLOT[s]}" for s in SLOTS], fontsize=6)
        ax.set_ylim(-3, 103)
        ax.text(0.5, 1.03, f"{SUPERVISED_LABEL[detector]} features, post hoc", transform=ax.transAxes, ha="center",
                va="bottom", fontsize=7.5)
        ax.grid(axis="y", lw=0.3, alpha=0.5)
    axes[0][0].set_ylabel("Recall at validation 5 % FPR (%)")
    handles, labels = axes[0][0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="lower center", ncol=len(labels), frameon=False, bbox_to_anchor=(0.5, -0.12),
               fontsize=6.5)
    paths = save(fig, directory, "F7_supervised")
    plt.close(fig)
    return paths
