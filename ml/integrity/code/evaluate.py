"""Tune Java winnowing on validation and evaluate once on held-out labelled pairs."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path

import numpy as np

from .algorithm import exact_token_match, prepare, similarity

POSITIVE_TYPES = ("T1", "T2", "T3")
GRID = (0.10, 0.20, 0.30, 0.40, 0.50, 0.60, 0.70, 0.80, 0.90, 1.00)
METRICS = ("precision", "recall", "F1", "FPR")


@dataclass(frozen=True)
class Pair:
    left: str
    right: str
    kind: str
    split: str
    group: str


def load_pairs(root: Path) -> tuple[dict[str, str], list[Pair]]:
    functions: dict[str, str] = {}
    with (root / "functions.jsonl").open(encoding="utf-8") as stream:
        for line in stream:
            data = json.loads(line)
            key, code = str(data["id"]), data["code"]
            if not key or key in functions or not isinstance(code, str) or not code.strip():
                raise ValueError(f"Missing or duplicate function: {key}")
            functions[key] = code
    pairs: list[Pair] = []
    seen: set[tuple[str, str]] = set()
    content_split: dict[str, str] = {}
    group_split: dict[str, str] = {}
    with (root / "pairs.csv").open(encoding="utf-8", newline="") as stream:
        for row in csv.DictReader(stream):
            left, right = sorted((row["left_id"], row["right_id"]))
            kind, split = row["clone_type"], row["split"]
            group = row["group"]
            if not group or kind not in (*POSITIVE_TYPES, "nonclone") or split not in ("validation", "test"):
                raise ValueError(f"Invalid group, clone type or split: {row}")
            if left == right or (left, right) in seen or not {left, right} <= functions.keys():
                raise ValueError(f"Self-pair, duplicate pair or unknown function: {left}, {right}")
            seen.add((left, right))
            if group_split.setdefault(group, split) != split:
                raise ValueError(f"Sampling group crosses splits: {group}")
            for key in (left, right):
                if content_split.setdefault(functions[key], split) != split:
                    raise ValueError(f"Function text occurs in both splits: {key}")
            pairs.append(Pair(left, right, kind, split, group))
    for split in ("validation", "test"):
        if {p.kind for p in pairs if p.split == split} != {*POSITIVE_TYPES, "nonclone"}:
            raise ValueError(f"Expected T1, T2, T3 and labelled negatives in {split}")
    return functions, pairs


def rates(counts: np.ndarray) -> np.ndarray:
    """Precision, recall, F1 and FPR for one or many (TP, FP, FN, TN) rows."""
    tp, fp, fn, tn = counts.T
    den = np.column_stack((tp + fp, tp + fn, 2 * tp + fp + fn, fp + tn))
    num = np.column_stack((tp, tp, 2 * tp, fp))
    return np.divide(num, den, out=np.full(den.shape, np.nan), where=den != 0)


def counts(labels: np.ndarray, flagged: np.ndarray) -> np.ndarray:
    return np.asarray([(labels & flagged).sum(), (~labels & flagged).sum(),
                       (labels & ~flagged).sum(), (~labels & ~flagged).sum()])


def tune(labels: np.ndarray, scores: np.ndarray) -> tuple[float, list[dict]]:
    grid = []
    for threshold in GRID:
        precision, _, f1, _ = rates(counts(labels, scores >= threshold))[0]
        grid.append({"threshold": threshold, "F1": float(f1),
                     "precision": float(precision) if np.isfinite(precision) else None})
    best = max(grid, key=lambda r: (r["F1"], r["precision"] or 0, r["threshold"]))
    return best["threshold"], grid


def summarize(pairs: list[Pair], predictions: np.ndarray, repeats: int, seed: int) -> dict:
    labels = np.asarray([p.kind != "nonclone" for p in pairs])
    point = counts(labels, predictions)
    by_group: dict[str, list[int]] = defaultdict(list)
    for i, pair in enumerate(pairs):
        by_group[pair.group].append(i)
    groups = np.stack([counts(labels[ii], predictions[ii]) for ii in by_group.values()])
    rng = np.random.default_rng(seed)
    draws = []
    for _ in range(repeats):
        ids = rng.integers(len(groups), size=len(groups))
        draws.append(groups[ids].sum(axis=0))
    metrics = {}
    intervals = {}
    for name, estimate, column in zip(METRICS, rates(point)[0], rates(np.stack(draws)).T, strict=True):
        valid = column[np.isfinite(column)]
        metrics[name] = float(estimate) if np.isfinite(estimate) else None
        intervals[name] = {"ci95": np.quantile(valid, [.025, .975]).tolist() if valid.size else None,
                           "defined": int(valid.size)}
    return {"TP": int(point[0]), "FP": int(point[1]), "FN": int(point[2]), "TN": int(point[3]),
            "groups": len(groups), "metrics": metrics, "intervals": intervals}


def evaluate(corpus: Path, k: int, w: int, repeats: int, seed: int) -> dict:
    if repeats < 1 or k < 1 or w < 1:
        raise ValueError("k, w and the number of bootstrap draws must be positive")
    functions, pairs = load_pairs(corpus)
    documents = {key: prepare(code, k=k, w=w) for key, code in functions.items()}
    val = [p for p in pairs if p.split == "validation"]
    validation_scores = np.array([similarity(documents[p.left], documents[p.right]) for p in val])
    threshold, grid = tune(np.array([p.kind != "nonclone" for p in val]), validation_scores)
    test = [p for p in pairs if p.split == "test"]
    scores = np.array([similarity(documents[p.left], documents[p.right]) for p in test])
    exact = np.array([exact_token_match(documents[p.left], documents[p.right]) for p in test])
    rows = []
    for method, predicted in (("Winnowing", scores >= threshold), ("Exact tokens", exact)):
        for kind in ("All", *POSITIVE_TYPES):
            indices = [i for i, p in enumerate(test) if kind == "All" or p.kind in (kind, "nonclone")]
            stats = summarize([test[i] for i in indices], predicted[indices], repeats, seed)
            rows.append({"method": method, "type": kind, **stats})
    hashes = {}
    for name in ("functions.jsonl", "pairs.csv", "provenance.json"):
        with (corpus / name).open("rb") as stream:
            hashes[name] = hashlib.file_digest(stream, "sha256").hexdigest()
    return {"threshold": threshold, "grid": grid, "k": k, "w": w, "bootstrap_seed": seed,
            "repeats": repeats, "rows": rows, "sha256": hashes,
            "validation_pairs": len(val), "test_pairs": len(test), "functions": len(functions)}


def markdown(result: dict) -> str:
    lines = ["# Java code similarity on a BigCloneBench subset", "",
             (f"The prepared subset contains {result['functions']} functions, "
             f"{result['validation_pairs']} validation pairs and {result['test_pairs']} held-out test pairs. "
             "All metrics refer to supplied pairs. Repository-wide candidate retrieval was outside this experiment."), "",
             (f"Winnowing used {result['k']}-token grams and minima over {result['w']} grams. "
             f"Validation selected a Jaccard threshold of **{result['threshold']:.2f}** by F1, then precision, "
             "then the higher threshold. The exact-token comparator uses full lexical token equality. "
             "Both methods were frozen before scoring test pairs."), "",
             "| Method | Test subset | TP | FP | FN | TN | Precision | Recall | F1 | FPR |",
             "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|"]
    def fmt(v: float | None) -> str:
        return "NA" if v is None else f"{v:.4f}"
    for row in result["rows"]:
        cells = [row["method"], row["type"], *(row[n] for n in ("TP", "FP", "FN", "TN")),
                 *(fmt(row["metrics"][n]) for n in METRICS)]
        lines.append("| " + " | ".join(map(str, cells)) + " |")
    lines += ["", ("Each type-specific row compares positives of that type with the same labelled nonclones. "
              "The negative denominator and FPR are consequently shared. Precision reflects this sampling ratio. "
              "An undefined denominator is marked NA."), "", "## Uncertainty", "",
              (f"Percentile 95% intervals use {result['repeats']} draws of test sampling groups with replacement "
              f"(seed {result['bootstrap_seed']}). Each draw retains the selected groups' paired labels and scores. "
              "Only draws with a defined metric contribute to its interval."), "",
              "| Method | Subset | Groups | Precision | Recall | F1 | FPR |",
              "|---|---|---:|---|---|---|---|"]
    for row in result["rows"]:
        values = []
        for name in METRICS:
            ci = row["intervals"][name]["ci95"]
            values.append(f"[{ci[0]:.3f}, {ci[1]:.3f}]" if ci is not None else "NA")
        cells = [row["method"], row["type"], row["groups"], *values]
        lines.append("| " + " | ".join(map(str, cells)) + " |")
    overall = result["rows"][0]
    lines += ["", (f"Winnowing flagged {overall['FP']} of the {overall['FP'] + overall['TN']} labelled "
              "nonclone pairs. The overall FPR interval is wide because related methods share sampling groups. "
              "The T3 stratum covers strongly and moderately similar clones. The release combines weak T3 with T4, "
              "so this evaluation gives no separate estimate for weak T3. A benchmark clone label describes "
              "similarity between methods. A teacher still needs the assignment and source context to assess a submission."),
              "", "## Reproduction", "", ("See the [evaluation instructions](README.md) for dataset provenance, "
              "selection rules and the commands. Local input checksums (SHA-256) identify this sample."), ""]
    lines.extend(f"- `{name}`: `{value}`" for name, value in result["sha256"].items())
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", type=Path, required=True)
    parser.add_argument("--k", type=int, default=5)
    parser.add_argument("--w", type=int, default=4)
    parser.add_argument("--bootstrap", type=int, default=1000)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--output", type=Path, default=Path("ml/integrity/code/results.md"))
    parser.add_argument("--artifact", type=Path, default=Path("ml/integrity/artifacts/code/experiment.json"))
    args = parser.parse_args()
    result = evaluate(args.corpus, args.k, args.w, args.bootstrap, args.seed)
    for path in (args.output, args.artifact):
        path.parent.mkdir(parents=True, exist_ok=True)
    args.artifact.write_text(json.dumps(result, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    args.output.write_text(markdown(result), encoding="utf-8")


if __name__ == "__main__":
    main()
