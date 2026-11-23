"""Train the frozen anomaly models and package first-answer medians for FR-42."""
from __future__ import annotations

import argparse
import pickle
import shutil
from pathlib import Path
from statistics import median
from tempfile import TemporaryDirectory
from typing import Any

from ml.anomaly.features import item_key, read_attempts
from ml.anomaly.train import train as train_frozen


def timing_medians(attempts: list[dict[str, Any]], artifact: dict[str, Any]) -> dict[str, Any]:
    """Use the same training cohorts and eligible items as the frozen p10 reference."""
    train_groups = set(artifact["manifest"]["groups"]["train"])
    pooled: list[int] = []
    eligible: dict[str, list[int]] = {key: [] for key in artifact["reference"]["item_fast_ms"]}
    for attempt in attempts:
        if attempt["group_id"] not in train_groups or attempt["is_anomaly"] != 0:
            continue
        for question, first_ms in attempt["first_times"].items():
            pooled.append(first_ms)
            key = item_key(attempt, question)
            if key in eligible:
                eligible[key].append(first_ms)
    return {
        "global_median_ms": float(median(pooled)),
        "item_median_ms": {key: float(median(values)) for key, values in eligible.items()},
    }


def train(csv: Path, manifest: Path, output: Path) -> None:
    output.mkdir(parents=True, exist_ok=True)
    with TemporaryDirectory(prefix=".evidence-train-", dir=output) as directory:
        stage = Path(directory)
        # Both passes read identical bytes even if the input path is regenerated.
        # The frozen trainer verifies their SHA-256 against the supplied split.
        snapshot = stage / "training.csv"
        shutil.copyfile(csv, snapshot)
        artifact = train_frozen(snapshot, manifest, stage)
        artifact["reference"].update(timing_medians(read_attempts(snapshot), artifact))
        model = stage / "model.pkl"
        with model.open("wb") as stream:
            pickle.dump(artifact, stream, protocol=pickle.HIGHEST_PROTOCOL)

        # Each reader opens a complete file. Publish the executable artifact last;
        # the Makefile pins its final digest after this command succeeds.
        thresholds = output / "thresholds.json"
        previous = stage / "previous-thresholds.json"
        if thresholds.exists():
            shutil.copyfile(thresholds, previous)
        (stage / "thresholds.json").replace(thresholds)
        try:
            model.replace(output / "model.pkl")
        except OSError:
            if previous.exists():
                previous.replace(thresholds)
            else:
                thresholds.unlink()
            raise


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    train(args.csv, args.manifest, args.output)


if __name__ == "__main__":
    main()
