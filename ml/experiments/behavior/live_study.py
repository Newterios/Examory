"""E-14 paired feature and score parity on complete attempts."""
from __future__ import annotations

import argparse
import csv
import json
from collections import defaultdict
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
