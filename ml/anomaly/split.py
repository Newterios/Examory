# ml/anomaly/split.py
"""Split complete cohorts 60/20/20; preserve students and score histories."""
from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

import numpy as np

try:
    from .features import file_sha256, read_attempts, write_json
except ImportError:
    from features import file_sha256, read_attempts, write_json


def split_groups(groups: list[str], seed: int = 42) -> dict[str, list[str]]:
    unique = np.asarray(sorted(set(groups)))
    if len(unique) < 5:
        raise ValueError("At least five cohorts are required for a 60/20/20 split")
    shuffled = np.random.default_rng(seed).permutation(unique).tolist()
    # Largest remainder allocation; exact ratios when cohort count is divisible by 5.
    sizes = np.array([.6, .2, .2]) * len(unique)
    counts = np.floor(sizes).astype(int)
    for index in np.argsort(-(sizes - counts), kind="stable")[:len(unique) - counts.sum()]:
        counts[index] += 1
    a, b = int(counts[0]), int(counts[0] + counts[1])
    return dict(zip(("train", "validation", "test"), (shuffled[:a], shuffled[a:b], shuffled[b:]), strict=True))


def create_manifest(csv: Path, seed: int) -> dict[str, Any]:
    attempts = read_attempts(csv)
    students: dict[str, str] = {}
    for attempt in attempts:
        previous = students.setdefault(attempt["user_id"], attempt["group_id"])
        if previous != attempt["group_id"]:
            raise ValueError("A student spans multiple cohorts; merge cohorts before splitting")
        if attempt["is_anomaly"] is None:
            raise ValueError("Experimental splitting requires labels")
    groups = split_groups([a["group_id"] for a in attempts], seed)
    return {
        "schema_version": 1, "seed": seed, "ratios": [.6, .2, .2],
        "csv_sha256": file_sha256(csv), "groups": groups,
        "attempt_ids": {name: sorted(a["attempt_id"] for a in attempts if a["group_id"] in values)
                        for name, values in groups.items()},
        "counts": {name: {"cohorts": len(values),
                          "attempts": sum(a["group_id"] in values for a in attempts),
                          "anomalies": sum(a["is_anomaly"] for a in attempts if a["group_id"] in values)}
                   for name, values in groups.items()},
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--csv", type=Path, default=Path("ml/datasets/raw/quiz.csv"))
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--output", type=Path, default=Path("ml/anomaly/artifacts/split.json"))
    args = parser.parse_args()
    manifest = create_manifest(args.csv, args.seed)
    write_json(args.output, manifest)
    print(manifest["counts"])


if __name__ == "__main__":
    main()
