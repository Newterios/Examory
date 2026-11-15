"""Aggregation, tables and figures of the stages added after independent review (15 Nov 2026).

fi-dose, protocol, baseline, sensitivity v2, sub-population recall (main stage), the
low-prevalence diagnostic and the list of p-values that crossed 0.05 after the test change.
Seeds are the replicates; intervals are percentile bootstraps over seeds; paired p-values are
exact sign-flip permutation tests (``stats.paired_difference``), unadjusted unless stated.
"""
from __future__ import annotations

from pathlib import Path
from typing import Any

import numpy as np

from .common import BL_TYPES, DETECTORS, FI_TYPES, SLOT_LABEL, SLOTS
from .stats import mean_ci, paired_difference, prcc

SEED = 7
LABEL = {"quantile_rules": "Rules", "iforest_no_peers": "IF", "iforest": "IF + peers"}
SHORT_SLOT = {"S1": "rapid / preknow.", "S2": "changes / lookup", "S3": "collusion / copy peer",
              "S4": "score jump / proxy"}


def _a():
    from . import analyze
    return analyze


def _slot_name(slot: str) -> str:
    return f"{slot} {SLOT_LABEL[slot]}"


def _seeds(stage: dict[str, Any]) -> list[int]:
    return sorted(set(stage["seeds"]))


# --------------------------------------------------------------------------- A. fi-dose


def _g(value: Any) -> str:
    return value if isinstance(value, str) else f"{value:g}"


def fi_dose_summary(stage: dict[str, Any], main: dict[str, Any] | None, dose: dict[str, Any] | None
                    ) -> dict[str, Any]:
    a = _a()
    runs, seeds = stage["runs"], _seeds(stage)
    lambdas, intensities = stage["lambdas"], list(stage["intensities"])
    if main:  # the main stage's SIM-BL (mechanism defaults, no --intensity) on the same seeds
        intensities.append("default")
    fi, bl, delta = {}, {}, {}
    for detector in DETECTORS:
        for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
            for lam in lambdas:
                values = a.series(runs, seeds, detector, subset, "recall@5", condition="SIM-FI", **{"lambda": lam})
                fi[f"{detector}|{subset}|{lam}"] = {**mean_ci(values, seed=SEED), "values": values}
            for x in intensities:
                values = (a.series(main["runs"], seeds, detector, subset, "recall@5", condition="SIM-BL")
                          if x == "default" else
                          a.series(runs, seeds, detector, subset, "recall@5", condition="SIM-BL", intensity=x))
                bl[f"{detector}|{subset}|{x}"] = {**mean_ci(values, seed=SEED), "values": values}
            for lam in lambdas:
                for x in intensities:
                    delta[f"{detector}|{subset}|{lam}|{x}"] = paired_difference(
                        fi[f"{detector}|{subset}|{lam}"]["values"], bl[f"{detector}|{subset}|{x}"]["values"],
                        seed=SEED)
    for cells in (fi, bl):
        for cell in cells.values():
            cell.pop("values")
    scale = []
    for condition, key, grid, types in (("SIM-FI", "lambda", lambdas, FI_TYPES),
                                        ("SIM-BL", "intensity", intensities, BL_TYPES)):
        for value in grid:
            if value == "default":
                continue
            chosen = [r for r in runs if r["condition"] == condition and r.get(key) == value]
            for kind in types:
                score = [r["manipulation"][kind]["score_points"] for r in chosen if kind in (r.get("manipulation") or {})]
                items = [r["items_affected"][kind]["mean_items_affected"] for r in chosen
                         if kind in (r.get("items_affected") or {})]
                zero = [r["items_affected"][kind]["share_zero_items"] for r in chosen
                        if kind in (r.get("items_affected") or {})]
                changed = [r["manipulation"][kind]["share_changed"] for r in chosen
                           if kind in (r.get("manipulation") or {})]
                scale.append({"world": condition, "dose": f"{key} = {value:g}", "value": value, "type": kind,
                              "slot": SLOTS[types.index(kind)], "score_gain_points": mean_ci(score, seed=SEED),
                              "items_affected": mean_ci(items, seed=SEED),
                              "share_zero_items_affected": mean_ci(zero, seed=SEED),
                              "share_any_observable_changed": mean_ci(changed, seed=SEED)})
    checks = {}
    if main:
        diffs = []
        for detector in DETECTORS:
            for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
                ours = a.series(runs, seeds, detector, subset, "recall@5", condition="SIM-FI", **{"lambda": 1.0})
                ref = a.series(main["runs"], seeds, detector, subset, "recall@5", condition="SIM-FI")
                diffs += [abs(x - y) for x, y in zip(ours, ref, strict=True) if x is not None and y is not None]
        main_hash = {r["seed"]: (r.get("data") or {}).get("csv_sha256") for r in main["runs"] if r["condition"] == "SIM-FI"}
        ours_hash = {r["seed"]: (r.get("data") or {}).get("csv_sha256") for r in runs
                     if r["condition"] == "SIM-FI" and r.get("lambda") == 1.0}
        checks["lambda_1_vs_main_SIM-FI"] = {"max_abs_recall_difference": max(diffs) if diffs else None,
                                             "csv_hash_equal_seeds": sum(main_hash.get(s) == h for s, h in ours_hash.items()),
                                             "seeds": len(ours_hash)}
    if dose:
        diffs = []
        for detector in DETECTORS:
            for x in stage["intensities"]:
                for slot in SLOTS:
                    ours = a.series(runs, seeds, detector, f"slot:{slot}", "recall@5", condition="SIM-BL", intensity=x)
                    ref = a.series(dose["runs"], seeds, detector, f"slot:{slot}", "recall@5", condition="SIM-BL",
                                   intensity=x)
                    diffs += [abs(p - q) for p, q in zip(ours, ref, strict=True) if p is not None and q is not None]
        checks["SIM-BL_vs_dose_stage"] = {"max_abs_recall_difference": max(diffs) if diffs else None,
                                          "compared": len(diffs)}
        fi_dose_values = {r["intensity"]: r["data"]["csv_sha256"] for r in dose["runs"]
                          if r["condition"] == "SIM-FI" and r["seed"] == seeds[0]}
        checks["dose_stage_SIM-FI_csv_identical_across_intensities"] = len(set(fi_dose_values.values())) == 1
    return {"lambdas": lambdas, "intensities": intensities, "seeds": seeds, "fi": fi, "bl": bl, "delta": delta,
            "dose_scale": scale, "checks": checks, "scaled_fi_parameters": stage.get("scaled_fi_parameters")}


def md_fi_dose(summary: dict[str, Any]) -> str:
    a = _a()
    lambdas, intensities = summary["lambdas"], summary["intensities"]
    parts = ["# Post hoc: injection dose λ × behaviour-level intensity (exploratory)", "",
             ("**Added after independent review (amendment of 15 Nov 2026).** SIM-FI: injection magnitude λ "
              f"multiplies `fi.{', fi.'.join(summary['scaled_fi_parameters'] or [])}` in a generated mechanisms file "
              "(probabilities capped at 1; all other fi parameters fixed, incl. the 6 forced-wrong collusion items, "
              "the submit window and the rapid time scale). SIM-BL: simulator `--intensity` (the dose stage's grid), "
              "which does not act on the fi world. Seeds "
              f"{min(summary['seeds'])}–{max(summary['seeds'])} (n = {len(summary['seeds'])}). Recall at the "
              "validation-frozen 5 % FPR threshold, %, mean [95 % CI over seeds]."), ""]
    checks = summary["checks"]
    if checks:
        c1 = checks.get("lambda_1_vs_main_SIM-FI", {})
        c2 = checks.get("SIM-BL_vs_dose_stage", {})
        parts += [(f"Checks: λ = 1 reproduces the main stage's SIM-FI data (CSV hash equal in "
                   f"{c1.get('csv_hash_equal_seeds')}/{c1.get('seeds')} seeds; max |Δ recall| = "
                   f"{c1.get('max_abs_recall_difference')}); SIM-BL equals the dose stage (max |Δ recall| = "
                   f"{c2.get('max_abs_recall_difference')} over {c2.get('compared')} seed × cells); the dose "
                   f"stage's SIM-FI CSV is identical at every intensity: "
                   f"{checks.get('dose_stage_SIM-FI_csv_identical_across_intensities')}."), ""]
    for world, cells, grid, symbol in (("SIM-FI recall(λ)", summary["fi"], lambdas, "λ"),
                                       ("SIM-BL recall(intensity)", summary["bl"], intensities, "x")):
        rows = [[LABEL[d], "all" if s == "all" else _slot_name(s[5:])]
                + [a.fmt_ci(cells[f"{d}|{s}|{v}"], "recall@5") for v in grid]
                for d in DETECTORS for s in ["all"] + [f"slot:{k}" for k in SLOTS]]
        parts += [f"## {world}", "", a.table(["Detector", "Subset"] + [f"{symbol} = {_g(v)}" for v in grid], rows,
                                             "|---|---|" + "---:|" * len(grid)), ""]
    parts += ["## Δ = recall_FI(λ) − recall_BL(x), pp, paired by seed", "",
              ("Rows λ, columns x. * = paired-bootstrap 95 % CI excludes 0 (descriptive; no tests). Note that "
               "`--intensity x` *replaces* each mechanism's coverage/propensity (default 0.65 preknowledge, 0.70 "
               "lookup, 0.80 copy_peer; proxy gain scale 1), so x = 1 is a stronger behaviour-level dose than the "
               "main stage; column `x = default` is the main stage's SIM-BL (no `--intensity`) on the same seeds, and "
               "the main-stage contrast is the cell λ = 1, x = default."), ""]
    for d in DETECTORS:
        for slot in SLOTS:
            rows = []
            for lam in lambdas:
                row = [f"λ = {lam:g}"]
                for x in intensities:
                    c = summary["delta"][f"{d}|slot:{slot}|{lam}|{x}"]
                    star = "*" if c["low"] is not None and (c["low"] > 0 or c["high"] < 0) else ""
                    row.append(a.fmt_ci(c, "recall@5") + star)
                rows.append(row)
            parts += [f"### {LABEL[d]}, {_slot_name(slot)}", "",
                      a.table([""] + [f"x = {_g(x)}" for x in intensities], rows, "|---|" + "---:|" * len(intensities)),
                      ""]
    rows = [[r["world"], r["dose"], f"{r['slot']} {r['type']}", a.fmt_ci(r["score_gain_points"], "", 2),
             a.fmt_ci(r["items_affected"], "", 2), a.fmt_ci(r["share_zero_items_affected"], "", 3),
             a.fmt_ci(r["share_any_observable_changed"], "", 3)] for r in summary["dose_scale"]]
    parts += ["## Common dose scale: what the cheaters' attempts changed at each grid point", "",
              ("Mean over seeds [95 % CI] of: score gain = mean paired difference cheater − the same attempt in the "
               "honest world (points of 20); items affected = mean `truth.csv:items_affected` (recalled / looked up / "
               "copied items for SIM-BL, all 20 for proxy; scaled / replaced / shared-option / correctness-flipped "
               "items for SIM-FI); share with zero affected items; share whose score, duration, change count or "
               "median first-answer time differs from the counterfactual."), "",
              a.table(["World", "Dose", "Type", "score gain (pts)", "items affected", "share 0 items",
                       "share observably changed"], rows, "|---|---|---|---:|---:|---:|---:|"), ""]
    return "\n".join(parts)


def figure_fi_dose(summary: dict[str, Any], directory: Path) -> list[str]:
    a = _a()
    plt = a._plt()
    lambdas, intensities = summary["lambdas"], summary["intensities"]
    fig, axes = plt.subplots(len(DETECTORS), len(SLOTS), figsize=(10, 7.2), squeeze=False)
    limit = max(abs(c["mean"]) for c in summary["delta"].values() if c["mean"] is not None) * 100
    for i, d in enumerate(DETECTORS):
        for j, slot in enumerate(SLOTS):
            grid = np.array([[100 * (summary["delta"][f"{d}|slot:{slot}|{lam}|{x}"]["mean"] or 0) for x in intensities]
                             for lam in lambdas])
            ax = axes[i][j]
            image = ax.imshow(grid, cmap="RdBu_r", vmin=-limit, vmax=limit, origin="lower", aspect="auto")
            numeric = [k for k, x in enumerate(intensities) if x != "default"]
            part = grid[:, numeric]
            if part.min() < 0 < part.max():  # contour only over the ordered intensity grid
                ax.contour(numeric, range(len(lambdas)), part, levels=[0], colors="black", linewidths=1.0)
            if len(numeric) < len(intensities):
                ax.axvline(len(numeric) - 0.5, color="black", lw=1.5)
            for p, lam in enumerate(lambdas):
                for q, x in enumerate(intensities):
                    c = summary["delta"][f"{d}|slot:{slot}|{lam}|{x}"]
                    significant = c["low"] is not None and (c["low"] > 0 or c["high"] < 0)
                    ax.text(q, p, f"{grid[p, q]:.0f}{'' if significant else '°'}", ha="center", va="center",
                            fontsize=5.5, color="black")
            ax.set_xticks(range(len(intensities)), [_g(x) for x in intensities], fontsize=6)
            ax.set_yticks(range(len(lambdas)), [f"{v:g}" for v in lambdas], fontsize=6)
            if i == len(DETECTORS) - 1:
                ax.set_xlabel("behaviour-level intensity x", fontsize=7)
            if j == 0:
                ax.set_ylabel(f"{LABEL[d]}\ninjection dose λ", fontsize=7)
            if i == 0:
                ax.text(0.5, 1.04, SHORT_SLOT[slot], transform=ax.transAxes, ha="center", va="bottom", fontsize=7)
    cbar = fig.colorbar(image, ax=axes, fraction=0.02, pad=0.02)
    cbar.set_label("Δ recall = FI(λ) − BL(x), pp (° : 95 % CI includes 0; black line: Δ = 0)", fontsize=7)
    paths = a.save(fig, directory, "F8_fi_dose")
    plt.close(fig)
    return paths


# --------------------------------------------------------------------------- B. sub-populations


def subpopulation_summary(main: dict[str, Any], protocol: dict[str, Any] | None) -> dict[str, Any]:
    a = _a()
    runs, seeds = main["runs"], _seeds(main)
    subsets = {"preknowledge, naive": "truth:preknowledge:mech_flag=naive",
               "preknowledge, disguising (strategic)": "truth:preknowledge:mech_flag=disguising",
               "preknowledge, all": "slot:S1",
               "copy_peer, non-sync pairs": "truth:copy_peer:sync_submit=0",
               "copy_peer, sync pairs": "truth:copy_peer:sync_submit=1", "copy_peer, all": "slot:S3"}
    rows = []
    for name, subset in subsets.items():
        positives = [((r["metrics"][DETECTORS[0]].get(subset) or {}).get("n_pos")) for r in runs
                     if r["condition"] == "SIM-BL"]
        row = {"subset": name, "key": subset,
               "positives_per_seed": float(np.mean([p for p in positives if p is not None])) if any(positives) else 0}
        for d in DETECTORS:
            row[d] = mean_ci(a.series(runs, seeds, d, subset, "recall@5", condition="SIM-BL"), seed=SEED)
        rows.append(row)
    unobservable = []
    if protocol:
        standard = [r for r in protocol["runs"] if r.get("variant") == "standard" and r.get("unobservable")]
        for condition in ("SIM-FI", "SIM-BL"):
            chosen = [r for r in standard if r["condition"] == condition]
            kinds = sorted({k for r in chosen for k in r["unobservable"]})
            for kind in kinds:
                cells = [r["unobservable"][kind] for r in chosen if kind in r["unobservable"]]
                n = sum(c["n"] for c in cells)
                entry = {"world": condition, "type": kind, "cheaters": n, "seeds": len(cells),
                         "event_identical": sum(c["event_identical"] for c in cells),
                         "score_unchanged": sum(c["score_unchanged"] for c in cells),
                         "zero_items_affected": sum(c["zero_items_affected"] for c in cells)}
                unobservable.append(entry)
    return {"rows": rows, "unobservable": unobservable, "seeds": seeds}


def md_subpopulations(summary: dict[str, Any]) -> str:
    a = _a()
    rows = [[r["subset"], f"{r['positives_per_seed']:.1f}"] + [a.fmt_ci(r[d], "recall@5") for d in DETECTORS]
            for r in summary["rows"]]
    parts = ["# Post hoc: recall by behavioural sub-population (main stage, SIM-BL)", "",
             ("**Added after independent review (amendment of 15 Nov 2026).** From `main.json` (20 seeds, frozen "
              "detectors, 5 % threshold). `truth.csv:mech_flag` records the pre-knowledge strategy per cheater: "
              "`naive` (recalled items answered at a sped-up time) or `disguising` (recalled items at the honest "
              "time draw), which is the strategic variant. Each row: that sub-group's test cheaters vs all normal "
              "test attempts; recall %, mean [95 % CI over seeds]; positives = mean test cheaters per seed."), "",
             a.table(["Sub-population", "positives/seed"] + [LABEL[d] for d in DETECTORS], rows,
                     "|---|---:|" + "---:|" * len(DETECTORS)), ""]
    if summary["unobservable"]:
        rows = [[u["world"], u["type"], str(u["cheaters"]), f"{u['event_identical']} ({100 * u['event_identical'] / u['cheaters']:.2f} %)",
                 f"{u['score_unchanged']} ({100 * u['score_unchanged'] / u['cheaters']:.1f} %)",
                 f"{u['zero_items_affected']} ({100 * u['zero_items_affected'] / u['cheaters']:.2f} %)"]
                for u in summary["unobservable"]]
        parts += ["## Cheating with no observable change (all cohorts, 20 seeds pooled)", "",
                  ("From the `protocol` stage (same data as the main stage). *Event-identical*: every quiz.answered / "
                   "quiz.submitted event of the cheater attempt (event id, timestamp, payload incl. answer hashes, "
                   "score, duration, change count) equals the same attempt in the honest world, i.e. the attempt is "
                   "observationally honest. *Score unchanged*: submitted score equals the counterfactual's. *0 items "
                   "affected*: `truth.csv:items_affected = 0` (for score_jump: no item's correctness flipped). All of "
                   "them remain labelled positives (is_anomaly = 1) and count in every recall denominator; nothing is "
                   "excluded."), "",
                  a.table(["World", "Type", "cheaters", "event-identical", "score unchanged", "0 items affected"], rows,
                          "|---|---|---:|---:|---:|---:|"), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- C. protocol


PROTOCOL_LABEL = {"standard": "main protocol", "label_free": "(i) label-free threshold",
                  "contaminated": "(ii) contaminated fitting"}


def protocol_summary(stage: dict[str, Any], main: dict[str, Any] | None) -> dict[str, Any]:
    a = _a()
    runs, seeds = stage["runs"], _seeds(stage)
    conditions = [c for c in ("LEGACY", "SIM-FI", "SIM-BL") if any(r["condition"] == c for r in runs)]
    variants = [v for v in stage["variants"] if any(r.get("variant") == v for r in runs)]
    cells, deltas = {}, []
    subsets = ["all"] + [f"slot:{s}" for s in SLOTS]
    for variant in variants:
        for condition in conditions:
            for d in DETECTORS:
                for subset in subsets + ["final_only:all"] + [f"final_only:slot:{s}" for s in SLOTS]:
                    cells[f"{variant}|{condition}|{d}|{subset}"] = {
                        m: mean_ci(a.series(runs, seeds, d, subset, m, condition=condition, variant=variant), seed=SEED)
                        for m in ("recall@5", "fpr@5", "precision@5")}
        if {"SIM-FI", "SIM-BL"} <= set(conditions):
            for d in DETECTORS:
                for subset in subsets:
                    test = paired_difference(
                        a.series(runs, seeds, d, subset, "recall@5", condition="SIM-FI", variant=variant),
                        a.series(runs, seeds, d, subset, "recall@5", condition="SIM-BL", variant=variant), seed=SEED)
                    deltas.append({"variant": variant, "detector": d, "subset": subset, **test})
    check = None
    if main:
        diffs = []
        for condition in conditions:
            for d in DETECTORS:
                for subset in subsets:
                    ours = a.series(runs, seeds, d, subset, "recall@5", condition=condition, variant="standard")
                    ref = a.series(main["runs"], seeds, d, subset, "recall@5", condition=condition)
                    diffs += [abs(x - y) for x, y in zip(ours, ref, strict=True) if x is not None and y is not None]
        check = {"max_abs_recall_difference_vs_main": max(diffs) if diffs else None, "compared": len(diffs)}
    diagnostics = {}
    for condition in conditions:
        rows = [r["feature_diagnostics"] for r in runs if r["condition"] == condition and r.get("feature_diagnostics")]
        if rows:
            diagnostics[condition] = {k: float(np.mean([r[k] for r in rows if r[k] is not None]))
                                      for k in rows[0] if isinstance(rows[0][k], (int, float))}
    return {"conditions": conditions, "variants": variants, "seeds": seeds, "cells": cells, "deltas": deltas,
            "standard_vs_main": check, "feature_diagnostics": diagnostics,
            "label_free_quantiles": stage.get("label_free_quantiles")}


def md_protocol(summary: dict[str, Any]) -> str:
    a = _a()
    conditions, variants = summary["conditions"], summary["variants"]
    parts = ["# Post hoc: robustness of the evaluation protocol (exploratory)", "",
             ("**Added after independent review (amendment of 15 Nov 2026).** Main configuration, seeds "
              f"{min(summary['seeds'])}–{max(summary['seeds'])}, frozen detectors. (i) label-free threshold: "
              "`numpy.quantile` (linear interpolation) at 0.95 of the scores of validation normal attempts, no positives "
              "used; (ii) contaminated fitting: train-cohort cheaters presented to the frozen pipeline as normal "
              "(reference statistics and models fitted on all train-cohort attempts), validation and test untouched, "
              "threshold rule of the main stage; (iii) final-attempt evaluation: FPR and precision against final "
              "normal attempts only (positives exist only among final attempts). Mean over seeds [95 % CI], %."), ""]
    if summary.get("standard_vs_main"):
        c = summary["standard_vs_main"]
        parts += [(f"Check: the `main protocol` rows reproduce `main.json` (max |Δ recall| = "
                   f"{c['max_abs_recall_difference_vs_main']} over {c['compared']} seed × cells)."), ""]
    for variant in variants:
        rows = []
        for d in DETECTORS:
            for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
                name = "all" if subset == "all" else _slot_name(subset[5:])
                rows.append([LABEL[d], name] + [a.fmt_ci(summary["cells"][f"{variant}|{c}|{d}|{subset}"]["recall@5"],
                                                         "recall@5") for c in conditions])
            rows.append([LABEL[d], "realized FPR (all attempts)"] + [
                a.fmt_ci(summary["cells"][f"{variant}|{c}|{d}|all"]["fpr@5"], "fpr@5") for c in conditions])
        parts += [f"## Recall, {PROTOCOL_LABEL[variant]}", "",
                  a.table(["Detector", "Subset"] + conditions, rows, "|---|---|" + "---:|" * len(conditions)), ""]
    rows = []
    for d in DETECTORS:
        for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
            row = [LABEL[d], "all" if subset == "all" else _slot_name(subset[5:])]
            for variant in variants:
                t = next(x for x in summary["deltas"] if x["variant"] == variant and x["detector"] == d
                         and x["subset"] == subset)
                row += [a.fmt_ci(t, "recall@5"), a.fmt_p(t["p"])]
            rows.append(row)
    header = ["Detector", "Subset"] + [h for v in variants for h in (f"Δ {PROTOCOL_LABEL[v]}", "p")]
    parts += ["## Δ = recall(SIM-FI) − recall(SIM-BL), pp, paired by seed", "",
              "Exact sign-flip permutation p, unadjusted (exploratory).", "",
              a.table(header, rows, "|---|---|" + "---:|" * (2 * len(variants))), ""]
    for variant in variants:
        rows = []
        for d in DETECTORS:
            for c in conditions:
                base = summary["cells"][f"{variant}|{c}|{d}|all"]
                final = summary["cells"][f"{variant}|{c}|{d}|final_only:all"]
                row = [LABEL[d], c, a.fmt_ci(base["fpr@5"], "fpr@5"), a.fmt_ci(final["fpr@5"], "fpr@5"),
                       a.fmt_ci(base["precision@5"], "precision@5"), a.fmt_ci(final["precision@5"], "precision@5")]
                row += [a.fmt_ci(summary["cells"][f"{variant}|{c}|{d}|final_only:slot:{s}"]["precision@5"], "precision@5")
                        for s in SLOTS]
                rows.append(row)
        parts += [f"## (iii) Final-attempt evaluation, {PROTOCOL_LABEL[variant]}", "",
                  ("FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts "
                   "vs final test normal attempts only. Recall is unchanged (all positives are final attempts). "
                   "Slot columns: precision of the slot's cheaters vs final normal attempts."), "",
                  a.table(["Detector", "Condition", "FPR, all attempts", "FPR, final attempts", "precision, all",
                           "precision, final"] + [f"precision final, {s}" for s in SLOTS], rows,
                          "|---|---|" + "---:|" * (4 + len(SLOTS))), ""]
        if variant != "standard":
            break  # final-attempt view shown for the main protocol and the label-free threshold only
    return "\n".join(parts)


# --------------------------------------------------------------------------- E. baseline


COPY_SPLITS = ("truth:copy_peer:sync_submit=0", "truth:copy_peer:sync_submit=1",
               "truth:copy_peer:auto_submitted=0", "truth:copy_peer:auto_submitted=1")
COPY_SPLIT_LABEL = dict(zip(COPY_SPLITS, ("non-sync pairs", "sync pairs", "cheater not auto-submitted",
                                          "cheater auto-submitted"), strict=True))
BASELINE_LABEL = {"revision": "(a) exam-like revision rate", "per_attempt": "(b) per-attempt item sampling",
                  "closing_p90": "(c) closing time at P90", "closing_p75": "(c) closing time at P75"}


def baseline_summary(stage: dict[str, Any], main: dict[str, Any] | None) -> dict[str, Any]:
    a = _a()
    runs, seeds = stage["runs"], _seeds(stage)
    variants = [v for v in BASELINE_LABEL if any(r.get("variant") == v for r in runs)]
    cells, deltas = {}, []
    subsets = ["all"] + [f"slot:{s}" for s in SLOTS]
    sources = [("main", main["runs"] if main else [], {})] + [(v, runs, {"variant": v}) for v in variants]
    for name, pool, match in sources:
        for d in DETECTORS:
            for subset in subsets:
                fi = a.series(pool, seeds, d, subset, "recall@5", condition="SIM-FI", **match)
                bl = a.series(pool, seeds, d, subset, "recall@5", condition="SIM-BL", **match)
                cells[f"{name}|SIM-FI|{d}|{subset}"] = mean_ci(fi, seed=SEED)
                cells[f"{name}|SIM-BL|{d}|{subset}"] = mean_ci(bl, seed=SEED)
                deltas.append({"variant": name, "detector": d, "subset": subset, **paired_difference(fi, bl, seed=SEED)})
            for c in ("SIM-FI", "SIM-BL"):
                cells[f"{name}|{c}|{d}|fpr"] = mean_ci(a.series(pool, seeds, d, "all", "fpr@5", condition=c, **match),
                                                       seed=SEED)
            for subset in COPY_SPLITS:
                values = a.series(pool, seeds, d, subset, "recall@5", condition="SIM-BL", **match)
                if any(v is not None for v in values):
                    positives = [((r["metrics"][d].get(subset) or {}).get("n_pos")) for r in pool
                                 if r["condition"] == "SIM-BL" and all(r.get(k) == v for k, v in match.items())]
                    cells[f"{name}|SIM-BL|{d}|{subset}"] = {
                        **mean_ci(values, seed=SEED),
                        "positives_per_seed": float(np.mean([x or 0 for x in positives])) if positives else 0.0}
    manipulation = {}
    for v in variants:
        manipulation[v] = a.manipulation_summary({"runs": [r for r in runs if r.get("variant") == v]})
    diagnostics = {}
    for v in variants:
        for c in ("SIM-FI", "SIM-BL"):
            rows = [r["feature_diagnostics"] for r in runs if r.get("variant") == v and r["condition"] == c
                    and r.get("feature_diagnostics")]
            if rows:
                diagnostics[f"{v}|{c}"] = {k: float(np.mean([r[k] for r in rows if r[k] is not None]))
                                           for k in rows[0] if isinstance(rows[0][k], (int, float))}
    return {"variants": variants, "seeds": seeds, "cells": cells, "deltas": deltas, "manipulation": manipulation,
            "feature_diagnostics": diagnostics, "info": stage.get("variants_info") or stage.get("variants"),
            "skipped": stage.get("skipped"), "natural_submission": stage.get("natural_submission")}


def md_baseline(summary: dict[str, Any], protocol_diagnostics: dict[str, Any] | None) -> str:
    a = _a()
    variants = summary["variants"]
    names = ["main"] + variants
    label = {"main": "main stage (calibrated)", **BASELINE_LABEL}
    info = summary.get("info") or {}
    parts = ["# Post hoc: honest-baseline variants (exploratory)", "",
             ("**Added after independent review (amendment of 15 Nov 2026).** Each variant is a complete main-protocol "
              f"run (seeds {min(summary['seeds'])}–{max(summary['seeds'])}, frozen detectors, SIM-FI and SIM-BL); the "
              "first column repeats the main stage for reference. Recall at the validation 5 % threshold, %, mean "
              "[95 % CI]; Δ = SIM-FI − SIM-BL paired by seed with exact sign-flip p (unadjusted)."), ""]
    rev = info.get("revision")
    if rev:
        parts += [(f"(a) Revision multiplier m = {rev['multiplier']:.4f} on both `revision.change_mult_*` "
                   f"(≡ scaling the honest revision propensity r_i while r_i·mult < 0.95), found by bisection on "
                   f"log m on the honest world of seed {rev['seed']}: mean answer changes per 20-item attempt "
                   f"{rev['mean_changes_per_20']:.3f} (target {rev['target']} ± {rev['tolerance']}), 95th percentile "
                   f"{rev['p95_changes_per_20']:.1f}; calibrated parameters (m = 1): mean "
                   f"{rev['calibrated_mean_changes_per_20']:.3f}, 95th percentile {rev['calibrated_p95_changes_per_20']:.1f}."),
                  ""]
    for name in variants:
        if name.startswith("closing") and name in info:
            c = info[name]
            parts += [(f"{BASELINE_LABEL[name]}: `--close-after-ms {c['close_after_ms']}` "
                      f"`--auto-submit-jitter-ms {c['auto_submit_jitter_ms']}` (examory-sim-next)."), ""]
    if summary.get("skipped"):
        parts += ["Not run: " + "; ".join(f"{k} ({v})" for k, v in summary["skipped"].items()) + ".", ""]
    rows = []
    for d in DETECTORS:
        for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
            row = [LABEL[d], "all" if subset == "all" else _slot_name(subset[5:])]
            for n in names:
                row.append(f"{a.fmt_ci(summary['cells'][f'{n}|SIM-FI|{d}|{subset}'], 'recall@5')} / "
                           f"{a.fmt_ci(summary['cells'][f'{n}|SIM-BL|{d}|{subset}'], 'recall@5')}")
            rows.append(row)
        rows.append([LABEL[d], "realized FPR"] + [
            f"{a.fmt_ci(summary['cells'][f'{n}|SIM-FI|{d}|fpr'], 'fpr@5')} / "
            f"{a.fmt_ci(summary['cells'][f'{n}|SIM-BL|{d}|fpr'], 'fpr@5')}" for n in names])
    parts += ["## Recall SIM-FI / SIM-BL", "", a.table(["Detector", "Subset"] + [label[n] for n in names], rows,
                                                        "|---|---|" + "---:|" * len(names)), ""]
    rows = []
    for d in DETECTORS:
        for subset in ["all"] + [f"slot:{s}" for s in SLOTS]:
            row = [LABEL[d], "all" if subset == "all" else _slot_name(subset[5:])]
            for n in names:
                t = next(x for x in summary["deltas"] if x["variant"] == n and x["detector"] == d and x["subset"] == subset)
                row.append(f"{a.fmt_ci(t, 'recall@5')}, p {a.fmt_p(t['p'])}")
            rows.append(row)
    parts += ["## Δ = SIM-FI − SIM-BL (pp)", "", a.table(["Detector", "Subset"] + [label[n] for n in names], rows,
                                                         "|---|---|" + "---:|" * len(names)), ""]
    rows = []
    for d in DETECTORS:
        for subset in COPY_SPLITS:
            row = [LABEL[d], COPY_SPLIT_LABEL[subset]]
            present = False
            for n in names:
                cell = summary["cells"].get(f"{n}|SIM-BL|{d}|{subset}")
                present |= cell is not None
                row.append("—" if cell is None else
                           f"{a.fmt_ci(cell, 'recall@5')} (n {cell['positives_per_seed']:.1f})")
            if present:
                rows.append(row)
    parts += ["## SIM-BL copy_peer recall by synchronous submission and by auto-submission", "",
              ("Recall %, mean [95 % CI]; n = mean test cheaters of the sub-group per seed. Auto-submission exists only "
               "with a closing time (simulator 0.3.0, `truth.csv:auto_submitted`)."), "",
              a.table(["Detector", "Sub-group"] + [label[n] for n in names], rows, "|---|---|" + "---:|" * len(names)),
              ""]
    for v in variants:
        parts.append(a.md_manipulation(summary["manipulation"][v]).replace(
            "## Manipulation check: what each injection/mechanism changed in observable terms",
            f"## Manipulation check, {BASELINE_LABEL[v]}"))
    keys = [("share_auto_submitted", "share of attempts auto-submitted"),
            ("share_auto_submitted_normal", "share of normal attempts auto-submitted"),
            ("share_auto_submitted_cheaters", "share of cheater attempts auto-submitted"),
            ("share_normal_attempts_synchronous", "share of normal attempts with a synchronous peer (5 s)"),
            ("share_history_missing", "share without score history (history_missing)"),
            ("share_z_score_defined", "share with z_score defined"),
            ("distinct_items", "distinct items"),
            ("share_responses_with_item_time_reference", "share of responses with an own item timing reference"),
            ("share_attempts_synchronous", "share of attempts with a synchronous peer (5 s)"),
            ("mean_common_items_synchronous_pairs", "mean common items of synchronous pairs"),
            ("share_synchronous_pairs_without_common_items", "share of synchronous pairs with no common item"),
            ("mean_max_answer_agreement", "mean max_answer_agreement")]
    columns = []
    if protocol_diagnostics:
        columns += [("main configuration (protocol stage), " + c, protocol_diagnostics[c]) for c in ("SIM-FI", "SIM-BL")
                    if c in protocol_diagnostics]
    columns += [(f"{BASELINE_LABEL[k.split('|')[0]]}, {k.split('|')[1]}", v)
                for k, v in summary["feature_diagnostics"].items()]
    if columns:
        rows = [[text] + ["—" if c[1].get(key) is None else f"{c[1][key]:.3g}" for c in columns] for key, text in keys]
        parts += ["## History-, item- and peer-based features under each baseline (mean over seeds)", "",
                  ("History features (`history_missing`, `z_score`) are keyed by (user, quiz_id, variant_id); the "
                   "simulator keeps one quiz_id/variant_id under per-attempt sampling, so score histories stay "
                   "defined although the items differ between a student's attempts."), "",
                  a.table(["Quantity"] + [c[0] for c in columns], rows, "|---|" + "---:|" * len(columns)), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- D. sensitivity v2


def sensitivity_v2_summary(stage: dict[str, Any]) -> dict[str, Any]:
    a = _a()
    runs, configs = stage["runs"], stage["configs"]
    names = [p["name"] for p in stage["parameters"]]
    seeds = _seeds(stage)
    matrix = np.asarray([[c[n] for n in names] for c in configs], dtype=float)
    cells: dict[str, Any] = {}
    n_tests = 0
    for metric in ("recall@5", "pauc_std"):
        for d in DETECTORS:
            for slot in SLOTS:
                per_config = {"fi": [], "bl": [], "delta": []}
                for index in range(len(configs)):
                    fi = a.series(runs, seeds, d, f"slot:{slot}", metric, condition="SIM-FI", config_id=index)
                    bl = a.series(runs, seeds, d, f"slot:{slot}", metric, condition="SIM-BL", config_id=index)
                    diff = [x - y for x, y in zip(fi, bl, strict=True) if x is not None and y is not None]
                    per_config["fi"].append(float(np.mean([x for x in fi if x is not None])) if any(
                        x is not None for x in fi) else None)
                    per_config["bl"].append(float(np.mean([y for y in bl if y is not None])) if any(
                        y is not None for y in bl) else None)
                    per_config["delta"].append(float(np.mean(diff)) if diff else None)
                fi_d = a.series(runs, seeds, d, f"slot:{slot}", metric, condition="SIM-FI", config_id=-1)
                bl_d = a.series(runs, seeds, d, f"slot:{slot}", metric, condition="SIM-BL", config_id=-1)
                default = paired_difference(fi_d, bl_d, seed=SEED)
                data = np.asarray([v for v in per_config["delta"] if v is not None], dtype=float)
                factor = 100 if metric == "recall@5" else 1
                entry: dict[str, Any] = {"metric": metric, **per_config, "default": default,
                                         "default_fi": mean_ci(fi_d, seed=SEED), "default_bl": mean_ci(bl_d, seed=SEED)}
                if len(data):
                    entry.update({"median": float(np.median(data)) * factor, "q25": float(np.quantile(data, .25)) * factor,
                                  "q75": float(np.quantile(data, .75)) * factor, "min": float(data.min()) * factor,
                                  "max": float(data.max()) * factor, "fraction_gt_0": float(np.mean(data > 0)),
                                  "fraction_gt_10pp": float(np.mean(data > 0.10)) if metric == "recall@5" else None})
                entry["prcc"] = dict(zip(names, prcc(matrix, per_config["delta"]), strict=True))
                n_tests += len(names)
                cells[f"{metric}|{d}|{slot}"] = entry
    return {"n_configs": len(configs), "seeds": seeds, "parameters": stage["parameters"], "lhs": {
        k: v for k, v in (stage.get("lhs") or {}).items() if k != "spearman_matrix"}, "cells": cells,
        "prcc_tests": n_tests}


def md_sensitivity_v2(summary: dict[str, Any]) -> str:
    a = _a()
    names = [p["name"] for p in summary["parameters"]]
    lhs = summary["lhs"]
    rows, prows, crows = [], [], []
    for key, cell in summary["cells"].items():
        metric, d, slot = key.split("|")
        if "median" not in cell:
            continue
        unit = "pp" if metric == "recall@5" else ""
        rows.append([metric, LABEL[d], _slot_name(slot), f"{cell['median']:.3g}{unit} [{cell['q25']:.3g}, {cell['q75']:.3g}]",
                     f"{cell['min']:.3g} … {cell['max']:.3g}", f"{100 * cell['fraction_gt_0']:.0f} %",
                     "NA" if cell["fraction_gt_10pp"] is None else f"{100 * cell['fraction_gt_10pp']:.0f} %",
                     a.fmt_ci(cell["default"], metric)])
        prows.append([metric, LABEL[d], slot] + [
            "NA" if cell["prcc"][n]["prcc"] is None else
            f"{cell['prcc'][n]['prcc']:+.2f}{'*' if cell['prcc'][n]['p'] < 0.05 else ''}" for n in names])
        if metric == "recall@5":
            fi = [v for v in cell["fi"] if v is not None]
            bl = [v for v in cell["bl"] if v is not None]
            crows.append([LABEL[d], _slot_name(slot), f"{100 * np.median(fi):.1f} [{100 * np.min(fi):.1f}, {100 * np.max(fi):.1f}]",
                          f"{100 * np.median(bl):.1f} [{100 * np.min(bl):.1f}, {100 * np.max(bl):.1f}]",
                          a.fmt_ci(cell["default_fi"], "recall@5"), a.fmt_ci(cell["default_bl"], "recall@5")])
    return "\n".join([
        "# Sensitivity of inflation to assumed mechanism and injection parameters (v2)", "",
        ("**Replaces v1 after independent review (amendment of 15 Nov 2026); v1 is kept as "
         "`stages/sensitivity_v1.json`.** "
         f"{summary['n_configs']} Latin-hypercube configurations × {len(summary['seeds'])} seeds "
         f"({min(summary['seeds'])}–{max(summary['seeds'])}) over {len(names)} parameters (10 behaviour-level as in v1 + "
         "4 injection parameters). Columns decorrelated by within-column swaps (RNG seed "
         f"{lhs.get('seed')}): max |Spearman ρ| between parameter columns {lhs.get('initial_max_abs_spearman', float('nan')):.3f} "
         f"→ {lhs.get('max_abs_spearman', float('nan')):.3f} (target ≤ {lhs.get('target')}). Δ per configuration = mean over "
         "its seeds of metric(SIM-FI) − metric(SIM-BL), paired. The default configuration (`sim/params/mechanisms.json`) "
         "was run on the same seeds; its Δ is shown with a paired bootstrap CI."), "",
        a.table(["Metric", "Detector", "Slot", "median Δ [IQR]", "range", "configs Δ>0", "configs Δ>10 pp",
                 "default config Δ [CI]"], rows, "|---|---|---|---:|---:|---:|---:|---:|"), "",
        "## Recall per configuration: median [min, max] over configurations, and the default configuration", "",
        a.table(["Detector", "Slot", "SIM-FI recall %", "SIM-BL recall %", "default SIM-FI", "default SIM-BL"], crows,
                "|---|---|---:|---:|---:|---:|"), "",
        (f"## Partial rank correlation (PRCC, all {len(names)} parameters jointly) between parameter and Δ "
         f"(* nominal p < 0.05; {summary['prcc_tests']} tests, uncorrected; exploratory)"), "",
        a.table(["Metric", "Detector", "Slot"] + names, prows, "|---|---|---|" + "---:|" * len(names)), ""])


def figure_sensitivity_v2(summary: dict[str, Any], directory: Path) -> list[str]:
    a = _a()
    plt = a._plt()
    fig, axes = plt.subplots(1, len(SLOTS), figsize=(9, 2.6), sharey=True)
    rng = np.random.default_rng(SEED)
    for ax, slot in zip(axes, SLOTS, strict=True):
        for k, d in enumerate(("quantile_rules", "iforest_no_peers", "iforest")):
            cell = summary["cells"][f"recall@5|{d}|{slot}"]
            values = [100 * v for v in cell["delta"] if v is not None]
            color, marker, _ = a.DETECTOR_STYLE[d]
            ax.boxplot([values], positions=[k], widths=0.5, showfliers=False, medianprops={"color": "black"},
                       boxprops={"lw": 0.7}, whiskerprops={"lw": 0.7})
            ax.scatter(k + rng.uniform(-0.15, 0.15, len(values)), values, s=5, marker=marker, color=color, alpha=0.6,
                       lw=0)
            default = cell["default"]
            if default["mean"] is not None:
                ax.errorbar([k + 0.32], [100 * default["mean"]], yerr=[[100 * (default["mean"] - default["low"])],
                                                                        [100 * (default["high"] - default["mean"])]],
                            fmt="*", color="black", ms=7, capsize=2, lw=0.8,
                            label="default configuration [95 % CI]" if (slot == SLOTS[0] and k == 0) else None)
        ax.axhline(0, color="black", lw=0.5)
        ax.axhline(10, color="grey", lw=0.5, ls="--")
        ax.set_xticks(range(3), [LABEL[d] for d in ("quantile_rules", "iforest_no_peers", "iforest")])
        ax.text(0.5, 1.02, SLOT_LABEL[slot], transform=ax.transAxes, ha="center", va="bottom", fontsize=6.5)
    axes[0].set_ylabel("Δ recall, SIM-FI − SIM-BL (pp)")
    handles, labels = axes[0].get_legend_handles_labels()
    fig.legend(handles, labels, loc="lower center", frameon=False, bbox_to_anchor=(0.5, -0.08), fontsize=6.5)
    paths = a.save(fig, directory, "F5_sensitivity")
    plt.close(fig)
    return paths


# --------------------------------------------------------------------------- H. prevalence diagnostic


def prevalence_diagnostic(main: dict[str, Any], prevalence: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    subsets = {"all copy_peer": "slot:S3", "sync pairs": "truth:copy_peer:sync_submit=1",
               "non-sync pairs": "truth:copy_peer:sync_submit=0"}
    for name, stage in (("main", main), ("prevalence", prevalence)):
        runs = [r for r in stage["runs"] if r["condition"] == "SIM-BL"]
        block: dict[str, Any] = {"anomaly_rate": stage["design"]["anomaly_rate"], "seeds": len(runs)}
        for d in ("quantile_rules", "iforest"):
            rows = {}
            for label, subset in subsets.items():
                npos = [((r["metrics"][d].get(subset) or {}).get("n_pos") or 0) for r in runs]
                recall = [(r["metrics"][d].get(subset) or {}).get("at_0.05", {}).get("recall") for r in runs]
                tp = sum(round(n * rc) for n, rc in zip(npos, recall, strict=True) if rc is not None)
                with_pos = [rc for rc in recall if rc is not None]
                rows[label] = {"positives_total": int(sum(npos)), "seeds_with_positives": len(with_pos),
                               "mean_per_seed_recall": float(np.mean(with_pos)) if with_pos else None,
                               "pooled_recall": tp / sum(npos) if sum(npos) else None, "detected": tp}
            rows["sync_share_of_positives"] = (rows["sync pairs"]["positives_total"]
                                               / rows["all copy_peer"]["positives_total"])
            rows["validation_fpr@5"] = float(np.mean([r["operating_points"][d]["0.05"]["validation_fpr"] for r in runs]))
            rows["test_fpr@5"] = float(np.mean([r["metrics"][d]["all"]["at_0.05"]["fpr"] for r in runs]))
            rows["threshold@5"] = float(np.mean([r["operating_points"][d]["0.05"]["threshold"] for r in runs]))
            block[d] = rows
        out[name] = block
    for d in ("quantile_rules", "iforest"):
        m, p = out["main"][d], out["prevalence"][d]
        share = p["sync_share_of_positives"]
        out[f"{d}|composition_expected"] = (share * m["sync pairs"]["pooled_recall"]
                                            + (1 - share) * m["non-sync pairs"]["pooled_recall"])
    return out


def md_prevalence_diagnostic(diag: dict[str, Any]) -> str:
    parts = ["# Diagnostic: Rules recall on copy_peer, low prevalence (82.5 %) vs main (65.5 %)", "",
             ("**Added after independent review (amendment of 15 Nov 2026).** SIM-BL, 20 seeds each. *Mean per-seed "
              "recall* is what `prevalence.md` / `main.md` report (seeds without test positives drop out); *pooled "
              "recall* = detected / all test positives over seeds. *Composition-expected* = the main stage's pooled "
              "sync and non-sync recall weighted by the low-prevalence sync share."), ""]
    rows = []
    for d in ("quantile_rules", "iforest"):
        for stage in ("main", "prevalence"):
            b = diag[stage][d]
            for label in ("all copy_peer", "sync pairs", "non-sync pairs"):
                c = b[label]
                rows.append([LABEL[d], f"{stage} (rate {diag[stage]['anomaly_rate']})", label, str(c["positives_total"]),
                             f"{c['seeds_with_positives']}/{diag[stage]['seeds']}",
                             "NA" if c["mean_per_seed_recall"] is None else f"{100 * c['mean_per_seed_recall']:.1f}",
                             "NA" if c["pooled_recall"] is None else f"{100 * c['pooled_recall']:.1f}"])
            rows.append([LABEL[d], stage, "sync share of positives", f"{100 * b['sync_share_of_positives']:.1f} %",
                         "", f"validation FPR {100 * b['validation_fpr@5']:.2f} %",
                         f"test FPR {100 * b['test_fpr@5']:.2f} %"])
        rows.append([LABEL[d], "prevalence", "composition-expected pooled recall", "", "",
                     "", f"{100 * diag[f'{d}|composition_expected']:.1f}"])
    parts += [_a().table(["Detector", "Stage", "Subset", "test positives (all seeds)", "seeds with positives",
                          "mean per-seed recall %", "pooled recall %"], rows, "|---|---|---|---:|---:|---:|---:|"), ""]
    return "\n".join(parts)


# --------------------------------------------------------------------------- G. p-values that crossed 0.05


def _walk_p(prefix: str, value: Any, out: dict[str, tuple[float, float]]) -> None:
    if isinstance(value, dict):
        if isinstance(value.get("p"), (int, float)) and isinstance(value.get("p_wilcoxon"), (int, float)):
            out[prefix] = (float(value["p_wilcoxon"]), float(value["p"]))
        if isinstance(value.get("p_holm"), (int, float)) and isinstance(value.get("p_holm_wilcoxon"), (int, float)):
            out[prefix + " (Holm)"] = (float(value["p_holm_wilcoxon"]), float(value["p_holm"]))
        for k, v in value.items():
            _walk_p(f"{prefix}/{k}", v, out)
    elif isinstance(value, list):
        for i, v in enumerate(value):
            ident = ""
            if isinstance(v, dict):
                ident = "|".join(str(v[k]) for k in ("contrast", "variant", "metric", "detector", "slot", "subset")
                                 if k in v)
            _walk_p(f"{prefix}[{ident or i}]", v, out)


def crossed_p_values(summary: dict[str, Any]) -> list[dict[str, Any]]:
    """Every paired test in the summary whose side of 0.05 differs between Wilcoxon and sign-flip."""
    pairs: dict[str, tuple[float, float]] = {}
    for key in ("inflation", "h1", "copy_peer_sync", "robust_detectors", "supervised", "protocol", "baseline"):
        if key in summary:
            _walk_p(key, summary[key], pairs)
    rows = []
    for key, (p0, p1) in sorted(pairs.items()):
        if (p0 < 0.05) != (p1 < 0.05):
            rows.append({"cell": key, "p_wilcoxon": p0, "p_sign_flip": p1,
                         "direction": "now < 0.05" if p1 < 0.05 else "now ≥ 0.05"})
    return rows


def md_crossed(rows: list[dict[str, Any]], committed: list[str]) -> str:
    a = _a()
    body = [[r["cell"], a.fmt_p(r["p_wilcoxon"]), a.fmt_p(r["p_sign_flip"]), r["direction"]] for r in rows
            if r["cell"].split("/")[0].split("[")[0] in committed]
    other = [[r["cell"], a.fmt_p(r["p_wilcoxon"]), a.fmt_p(r["p_sign_flip"]), r["direction"]] for r in rows
             if r["cell"].split("/")[0].split("[")[0] not in committed]
    parts = ["# p-values that crossed 0.05 when Wilcoxon was replaced by the exact sign-flip test", "",
             ("Every paired test in `summary.json`, raw and Holm-adjusted where a Holm family exists. p (Wilcoxon) is "
              "the previous method (SciPy `wilcoxon`, two-sided, exact or normal approximation as SciPy chooses), "
              "recomputed on the same per-seed values; p (sign-flip) is the exact paired sign-flip permutation test "
              "now reported everywhere. Mann–Whitney (unpaired) p-values did not change and are not listed."), "",
             "## Tables committed before the review (inflation, robust_inflation, h1, supervised; summary.json copy_peer_sync)", "",
             a.table(["Cell", "p (Wilcoxon)", "p (sign-flip)", "direction"], body, "|---|---:|---:|---|")
             if body else "None.", ""]
    if other:
        parts += ["## New post hoc tables (for information: these were never reported with Wilcoxon)", "",
                  a.table(["Cell", "p (Wilcoxon)", "p (sign-flip)", "direction"], other, "|---|---:|---:|---|"), ""]
    return "\n".join(parts)
