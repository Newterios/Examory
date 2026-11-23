"""Reproduce E-16 on the SIM-BL world selected by its generation manifest."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from tempfile import TemporaryDirectory

import pandas as pd

from ml.anomaly.features import build_features, file_sha256, read_attempts
from ml.anomaly.train import score_model
from ml.experiments.inflation.data import Dataset
from ml.experiments.inflation.pipeline import fit_frozen, make_manifest


def evaluate(directory: Path, output: Path) -> None:
    manifest = json.loads((directory / "manifest.json").read_text(encoding="utf-8"))
    for kind, name in (("bl", "bl.csv"), ("truth", "truth.csv")):
        if file_sha256(directory / name) != manifest["outputs"][kind]["sha256"]:
            raise ValueError(f"Source digest mismatch for {name}")
    path = directory / "bl.csv"
    dataset = Dataset("SIM-BL", path, manifest["args"]["seed"], read_attempts(path))
    split = make_manifest(dataset, dataset.seed)
    with TemporaryDirectory() as tmp:
        artifact = fit_frozen(dataset, split, Path(tmp))
    frame = build_features(dataset.attempts, artifact["reference"])
    truth = pd.read_csv(directory / "truth.csv")
    truth = truth.loc[truth.world == "bl", ["attempt_id", "user_id", "theta", "tau", "revise"]]
    frame = frame.merge(truth, on=["attempt_id", "user_id"], validate="one_to_one", how="left")
    if frame[["theta", "tau", "revise"]].isna().any().any():
        raise ValueError("SIM-BL truth rows missing")
    train, test = [frame.group_id.isin(split["groups"][p]) for p in ("train", "test")]
    normal = frame.is_anomaly == 0
    scores = {}
    cutoffs = {}
    for key in ("iforest", "quantile_rules"):
        model = artifact["models"][key]
        scores[key] = score_model(frame, model)
        cutoffs[key] = model["medium_threshold"]

    people = frame[train & normal].drop_duplicates("user_id")
    fast, strong = people.tau.quantile(.75), people.theta.quantile(.75)
    slow, revise = people.tau.quantile(.25), people.revise.quantile(.75)
    strata = {"All normal": normal, "Fast and strong": (frame.tau >= fast) & (frame.theta >= strong),
              "Slow": frame.tau <= slow, "Frequent reviser": frame.revise >= revise}
    lines = ["# False positives in student strata", "",
             (f"SIM-BL {manifest['version']}, seed {dataset.seed}, {manifest['args']['students']} students, "
             f"{manifest['args']['attempts']} attempts each, {manifest['args']['questions']} fixed questions. "
             "The cohorts are disjoint across training, validation and test."), "",
             f"World SHA-256 `{file_sha256(path)}`. Truth SHA-256 `{file_sha256(directory / 'truth.csv')}`.", "",
             "## E-16. False positives in normal attempts", "",
             ("Strata use simulator traits solely for reporting. Quartiles were fixed on unique training students "
             "with a normal observation. The fast-and-strong group crosses the upper quartiles of speed and "
             "initial ability. Slow uses the bottom speed quartile. Frequent reviser uses the top revision-propensity "
             "quartile. Strata can overlap. Counts include every labelled-normal test attempt."), "",
             "| Stratum | Normal attempts | Students | Forest FP | Forest FPR | Rules FP | Rules FPR |",
             "|---|---:|---:|---:|---:|---:|---:|"]
    fprs: dict[str, dict[str, float]] = {key: {} for key in scores}
    for name, sub in strata.items():
        rows = normal & test & sub
        fp = {k: int((scores[k][rows] >= cutoffs[k]).sum()) for k in scores}
        n = int(rows.sum())
        if not n:
            raise ValueError(f"no test attempts for stratum {name}")
        for key, false_positives in fp.items():
            fprs[key][name] = 100 * false_positives / n
        lines.append(f"| {name} | {n} | {frame[rows].user_id.nunique()} | {fp['iforest']} | "
                     f"{fprs['iforest'][name]:.2f}% | {fp['quantile_rules']} | {fprs['quantile_rules'][name]:.2f}% |")
    lines.append("")
    for key, values in fprs.items():
        highest = max(list(strata)[1:], key=values.__getitem__)
        lines.append(f"{key}: {highest.lower()} had the highest subgroup FPR, {values[highest]:.2f}%, "
                     f"{values[highest] - values['All normal']:+.2f} percentage points versus all normal attempts.")
        lines.append("")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--world-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    evaluate(args.world_dir, args.output)
