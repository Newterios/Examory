"""E-14 paired feature/score parity and E-15 chronological quiz-reference experiment."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
from collections import defaultdict
from copy import deepcopy
from pathlib import Path

import numpy as np

from ml.anomaly.features import (
    FEATURE_COLUMNS,
    build_features,
    file_sha256,
    read_attempts,
)
from ml.anomaly.train import score_model
from ml.service.artifact import load_verified
from ml.service.pipeline_v2 import prepare_v2
from ml.service.quiz_reference import fit_quiz


def run(world: Path, model: Path, digest: str, output: Path) -> None:
    artifact = load_verified(model, digest)
    source = world / "bl.csv"
    if file_sha256(source) != artifact["csv_sha256"]:
        raise ValueError("Experiment requires a model trained on training cohorts from the supplied world")
    attempts = read_attempts(source)
    groups = set(artifact["manifest"]["groups"]["test"])
    held_out = [a for a in attempts if a["group_id"] in groups]
    parity_group = min(groups)
    paired = [a for a in held_out if a["group_id"] == parity_group]
    selected = {a["attempt_id"] for a in paired}
    envs: dict[str, list[dict]] = defaultdict(list)
    with source.open(encoding="utf-8", newline="") as stream:
        for row in csv.DictReader(stream):
            p = json.loads(row["payload"])
            if p["attempt_id"] in selected:
                envs[p["attempt_id"]].append({
                    "event_type": row["event_type"], "event_id": row["event_id"], "version": 1,
                    "producer": row["producer"], "occurred_at": row["occurred_at"],
                    "actor": json.loads(row["actor"]), "context": json.loads(row["context"]), "payload": p,
                })
    v2, rejected = prepare_v2(envs)
    if rejected or len(v2) != len(paired):
        raise ValueError("A complete paired attempt was rejected")
    reference = artifact["reference"]
    baseline, current = [build_features(rows, reference).sort_values("attempt_id") for rows in (paired, v2)]
    if baseline.attempt_id.tolist() != current.attempt_id.tolist():
        raise AssertionError("Attempt identifiers differ")
    np.testing.assert_allclose(baseline[FEATURE_COLUMNS], current[FEATURE_COLUMNS], atol=0, rtol=0, equal_nan=True)
    lines = ["# Live input and reference experiments", "", (f"World SHA-256 `{file_sha256(source)}`. "
             f"Fitted model SHA-256 `{digest}`. Training source and held-out cohorts are shared with E-16/E-17."), "",
             "## E-14. Complete-attempt parity", "",
             (f"Smallest held-out cohort by ID, {parity_group}. All {len(paired)} complete simulator attempts were "
             f"accepted by both readers. All {len(FEATURE_COLUMNS)} model inputs matched, including missing-history masks."), "",
             "| Detector | Compared attempts | Maximum absolute raw-score difference | Different alert decisions |",
             "|---|---:|---:|---:|"]
    for name in ("iforest", "quantile_rules"):
        fitted = artifact["models"][name]
        a, b = (score_model(f, fitted) for f in (baseline, current))
        cut = fitted["medium_threshold"]
        lines.append(f"| {name} | {len(a)} | {np.max(np.abs(a-b)):.10g} | {int(np.sum((a>=cut)!=(b>=cut)))} |")

    # Real quiz/variant/question IDs differ from a simulator's training IDs. Remap
    # identifiers only. The input timing, points, behaviour and labels are identical.
    live = deepcopy(held_out)
    for row in live:
        row["quiz_id"], row["variant_id"] = "deployment_quiz", "deployment_variant"
    final = sorted((r for r in live if r["is_final_attempt"]), key=lambda r: r["submitted_at"])
    cut_time = final[len(final) // 2]["submitted_at"]
    prior = [r for r in live if r["submitted_at"] < cut_time]
    prior.sort(key=lambda r: (r["submitted_at"], r["attempt_id"]), reverse=True)
    # Simulate positive teacher decisions known before the snapshot. The rule below
    # deterministically selects approximately half of the earlier positives.
    confirmed = {r["attempt_id"] for r in prior if r["is_anomaly"]
                 and hashlib.sha256(r["attempt_id"].encode()).digest()[0] < 128}
    fitting = [r for r in prior if r["attempt_id"] not in confirmed][:500]
    warm = fit_quiz(deepcopy(fitting), 30)
    if warm is None:
        raise ValueError("Insufficient quiz history")
    frames = [build_features(live, ref) for ref in (reference, warm)]
    mask = (frames[0].submitted_at >= cut_time) & (frames[0].is_anomaly == 0)
    n = int(mask.sum())
    if n == 0:
        raise ValueError("No labelled normal future attempts")
    lines += ["", "## E-15. Normal alerts after warm-up", "",
              ("The test cohorts use a new quiz and variant ID. The estimator and validation cutoffs remain fixed. "
              f"At {cut_time.isoformat()}, the snapshot used {len(fitting)} strictly earlier eligible attempts. "
              f"The replay simulated {len(confirmed)} prior positive teacher confirmations and excluded them. "
              f"The retained history contains {sum(r['is_anomaly'] for r in fitting)} unconfirmed anomalous attempts. "
              "Synthetic labels are used to simulate these decisions and evaluate outcomes. The reference worker "
              "consumes teacher decisions and has no access to simulation labels."), "",
              "| Detector | Future normal attempts | Fixed synthetic FP / FPR | Warm quiz FP / FPR | Change (pp) |",
              "|---|---:|---:|---:|---:|"]
    for name in ("iforest", "quantile_rules"):
        fitted = artifact["models"][name]
        fp = [int((score_model(f.loc[mask], fitted) >= fitted["medium_threshold"]).sum()) for f in frames]
        lines.append(f"| {name} | {n} | {fp[0]} / {100*fp[0]/n:.2f}% | {fp[1]} / {100*fp[1]/n:.2f}% | "
                     f"{100*(fp[1]-fp[0])/n:+.2f} |")
    lines += ["", ("The cold-start policy suppresses publication until a historical snapshot is available. Its zero "
              "alerts during warm-up represent abstention. The warm comparison above counts decisions on the same "
              "future normal attempts. One synthetic quiz provides limited evidence for transfer to real courses."), ""]
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--world-dir", type=Path, required=True)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    run(args.world_dir, args.model, args.sha256, args.output)
