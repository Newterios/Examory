# ml/integrity/evaluate.py
"""PAN-PC-11 closed-family subset evaluation; tune on validation, freeze test."""
from __future__ import annotations

import argparse
import hashlib
import importlib.metadata
import itertools
import json
import platform
import time
import tracemalloc
import xml.etree.ElementTree as ET
from collections import defaultdict
from pathlib import Path
from typing import Any

import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer

try:
    from .minhash import Detector, tokenize
except ImportError:
    from minhash import Detector, tokenize


def words(text: str) -> list[str]:
    return [token.value for token in tokenize(text)]


def read_pan(root: Path) -> tuple[dict[str, Path], dict[tuple[str, str], list[dict]], dict[str, str]]:
    external = list(root.rglob("external-detection-corpus"))
    if len(external) == 1:
        root = external[0]
    paths = {}
    for path in sorted(root.rglob("*.txt")):
        if path.name.startswith(("source-document", "suspicious-document")):
            if path.name in paths:
                raise ValueError("Duplicate PAN filenames; --corpus must select one external-detection corpus")
            paths[path.name] = path
    if not paths:
        raise ValueError("No PAN source-document/suspicious-document .txt files")
    parents = {key: key for key in paths}

    def find(key: str) -> str:
        while parents[key] != key:
            parents[key] = parents[parents[key]]
            key = parents[key]
        return key

    truth: dict[tuple[str, str], list[dict]] = defaultdict(list)
    annotated = set()
    for path in sorted(root.rglob("*.xml")):
        tree = ET.parse(path).getroot()
        target = Path(tree.get("reference", "")).name
        if target not in paths or not target.startswith("suspicious-document"):
            continue
        annotated.add(target)
        for feature in tree.iter("feature"):
            if feature.get("name") != "plagiarism" or not feature.get("source_reference"):
                continue
            source = Path(feature.attrib["source_reference"]).name
            if source not in paths:
                raise ValueError(f"Missing annotated source {source}")
            a, n, b, m = (int(feature.attrib[k]) for k in
                          ("this_offset", "this_length", "source_offset", "source_length"))
            if min(a, b) < 0 or min(n, m) <= 0:
                raise ValueError("Invalid PAN span")
            properties = dict(feature.attrib)
            language = str(properties.get("type", "")).lower()
            obfuscation = str(properties.get("obfuscation", "unknown")).lower()
            if "translat" in language or "translat" in obfuscation:
                category = "translation"
            else:
                category = {"none": "literal", "low": "light", "high": "heavy"}.get(obfuscation, "other_or_unknown")
            truth[(target, source)].append({"target": [a, a + n], "source": [b, b + m],
                                            "category": category, "properties": properties})
            left, right = find(target), find(source)
            parents[max(left, right)] = min(left, right)
    unannotated = {k for k in paths if k.startswith("suspicious-document")} - annotated
    if unannotated:
        raise ValueError(f"{len(unannotated)} suspicious documents lack XML; cannot label them negative")
    return paths, dict(truth), {key: find(key) for key in paths}


def subset_split(paths: dict[str, Path], truth: dict, families: dict[str, str],
                 max_documents: int, seed: int) -> tuple[dict[str, list[str]], dict[str, Any]]:
    members: dict[str, list[str]] = defaultdict(list)
    for key, family in families.items():
        members[family].append(key)
    positive_families = {families[target] for target, _ in truth}
    rng = np.random.default_rng(seed)
    eligible = [f for f in sorted(members) if len(members[f]) <= max_documents // 5]
    positives = rng.permutation([f for f in eligible if f in positive_families]).tolist()
    negatives = rng.permutation([f for f in eligible if f not in positive_families]).tolist()
    if len(positives) < 5:
        raise ValueError("Need >=5 closed positive families within the size budget. Increase --max-documents; giant components are never split.")
    selected, used = [], 0
    # Reserve positive families, then mix remaining families without looking at scores.
    order = positives[:5] + rng.permutation(positives[5:] + negatives).tolist()
    for family in order:
        if used + len(members[family]) <= max_documents:
            selected.append(family)
            used += len(members[family])
    allocation = {"train": [], "validation": [], "test": []}
    for positive in (True, False):
        values = [f for f in selected if (f in positive_families) == positive]
        n = len(values)
        a, b = round(.6 * n), round(.8 * n)
        for name, subset in zip(allocation, (values[:a], values[a:b], values[b:]), strict=True):
            allocation[name].extend(subset)
    splits = {name: sorted(k for family in values for k in members[family]) for name, values in allocation.items()}
    manifest = {"seed": seed, "max_documents": max_documents, "family_splits": allocation,
                "document_splits": splits, "selected_documents": used, "total_documents": len(paths),
                "selected_families": len(selected), "total_families": len(members),
                "oversized_families_excluded": len(members) - len(eligible),
                "selection": "Closed families, positive-family stratification; oversized families excluded. Not full-corpus performance.",
                "files": {k: {"sha256": hashlib.sha256(paths[k].read_bytes()).hexdigest()}
                          for keys in splits.values() for k in keys}}
    return splits, manifest


def pair_metrics(predicted: set[tuple], truth: set[tuple], universe: set[tuple]) -> dict[str, Any]:
    tp, fp, fn = len(predicted & truth), len(predicted - truth), len(truth - predicted)
    tn = len(universe - truth - predicted)
    return {"pairs": len(universe), "positives": len(truth), "tp": tp, "fp": fp, "fn": fn, "tn": tn,
            "precision": tp / (tp + fp) if tp + fp else None,
            "recall": tp / len(truth) if truth else None,
            "f1": 2 * tp / (2 * tp + fp + fn) if 2 * tp + fp + fn else None,
            "fpr": fp / (fp + tn) if fp + tn else None}


def interval_length(intervals: list[list[int]]) -> int:
    total, end = 0, -1
    for left, right in sorted(intervals):
        total += max(0, right - max(left, end))
        end = max(end, right)
    return total


def span_metrics(predictions: dict[tuple, list[dict]], truth: dict[tuple, list[dict]], side: str) -> dict[str, Any]:
    detected = relevant = overlap = 0
    for pair in set(predictions) | set(truth):
        predicted = [span[side] for span in predictions.get(pair, [])]
        gold = [span[side] for span in truth.get(pair, [])]
        detected += interval_length(predicted)
        relevant += interval_length(gold)
        intersections = [[max(a, c), min(b, d)] for a, b in predicted for c, d in gold if max(a, c) < min(b, d)]
        overlap += interval_length(intersections)
    return {"precision": overlap / detected if detected else None,
            "recall": overlap / relevant if relevant else None,
            "predicted_characters": detected, "gold_characters": relevant, "overlap_characters": overlap}


def cached_partition(texts: dict[str, str], keys: list[str], seed: int, mode: str,
                     permutations: int, templates: list[str]) -> dict[str, Any]:
    sources = {key: texts[key] for key in keys if key.startswith("source-document")}
    queries = {key: texts[key] for key in keys if key.startswith("suspicious-document")}
    if not sources or not queries:
        raise ValueError("Each split needs sources and suspicious documents; increase subset size")
    started = time.perf_counter()
    detector = Detector(sources, seed, permutations, mode, templates)
    documents = {key: detector.prepare(key, value) for key, value in queries.items()}
    lsh = {key: detector.retrieve(doc) for key, doc in documents.items()}
    lsh_seconds = time.perf_counter() - started
    started = time.perf_counter()
    exhaustive = {key: detector.retrieve(doc, exhaustive=True) for key, doc in documents.items()}
    return {"detector": detector, "queries": documents, "lsh": lsh, "exhaustive": exhaustive,
            "universe": set(itertools.product(queries, sources)),
            "lsh_seconds": lsh_seconds, "exhaustive_seconds": time.perf_counter() - started,
            "coverage": sum(bool(doc.windows) for doc in documents.values()) / len(documents)}


def predictions(cache: dict[str, Any], method: str, threshold: float, min_span: int) -> dict[tuple, list[dict]]:
    result = {}
    for key, doc in cache["queries"].items():
        matches = cache["detector"].verify(doc, cache[method][key], threshold, min_span)
        for match in matches:
            result[(key, match["submission_id"])] = match["overlap_spans"]
    return result


def tune(cache: dict[str, Any], truth: set[tuple], method: str) -> tuple[float, int, list[dict]]:
    rows = []
    for threshold, minimum in itertools.product((.50, .65, .80), (20, 50)):
        row = pair_metrics(set(predictions(cache, method, threshold, minimum)), truth, cache["universe"])
        rows.append({"threshold": threshold, "min_span": minimum, **row})
    best = max(rows, key=lambda r: (r["f1"] or 0, r["precision"] or 0, r["threshold"], r["min_span"]))
    return best["threshold"], best["min_span"], rows


def constructed_checks(source_id: str, source_text: str, config: dict[str, Any],
                       seed: int, mode: str, permutations: int) -> list[dict[str, Any]]:
    """Separate assumed-legitimate stress cases; never contaminate PAN labels."""
    tokens = tokenize(source_text)
    passage = source_text[tokens[0].start:tokens[min(100, len(tokens)) - 1].end] if tokens else ""
    quoted = f'Attributed quotation from {source_id}: "{passage}". '
    quoted += " ".join(f"independentdiscussion{i}" for i in range(100))
    template = " ".join(f"providedassignmentinstruction{i}" for i in range(180))
    source = template + " " + " ".join(f"sourceoriginal{i}" for i in range(30))
    target = template + " " + " ".join(f"targetoriginal{i}" for i in range(30))
    rows = []
    for name, original, query_text, allowlist in (
        ("attributed_quotation", source_text, quoted, []),
        ("shared_teacher_template_without_allowlist", source, target, []),
        ("shared_teacher_template_with_allowlist", source, target, [template]),
    ):
        detector = Detector({"constructed_source": original}, seed, permutations, mode, allowlist)
        query = detector.prepare("constructed_target", query_text)
        flagged = bool(detector.verify(query, detector.retrieve(query), config["threshold"], config["min_span"]))
        rows.append({"case": name, "pairs": 1, "assumed_legitimate": True,
                     "flagged": int(flagged), "false_positive_under_assumption": int(flagged)})
    return rows


def evaluate(args: argparse.Namespace) -> dict[str, Any]:
    started = time.perf_counter()
    tracemalloc.start()
    paths, all_truth, families = read_pan(args.corpus)
    splits, manifest = subset_split(paths, all_truth, families, args.max_documents, args.seed)
    selected = {key for partition in splits.values() for key in partition}
    # Keep exact source bytes for checksums, newline characters for PAN offsets.
    texts = {key: paths[key].read_bytes().decode("utf-8-sig") for key in selected}
    truth = {pair: spans for pair, spans in all_truth.items() if set(pair) <= selected}
    for (target, source), spans in truth.items():
        if any(s["target"][1] > len(texts[target]) or s["source"][1] > len(texts[source]) for s in spans):
            raise ValueError("PAN offsets exceed decoded text; check corpus encoding/version")
    templates = json.loads(args.templates.read_text(encoding="utf-8")) if args.templates else []
    vectorizer = TfidfVectorizer(tokenizer=words, token_pattern=None, lowercase=False, sublinear_tf=True)
    vectorizer.fit([texts[key] for key in splits["train"]])
    caches = {name: cached_partition(texts, splits[name], args.seed, args.mode, args.num_perm, templates)
              for name in ("validation", "test")}
    gold = {name: {pair: spans for pair, spans in truth.items() if pair in caches[name]["universe"]}
            for name in caches}
    if any(not values for values in gold.values()):
        raise ValueError("Validation/test need positive pairs; enlarge the subset")
    results, configurations, grid = [], {}, {}
    for method in ("lsh", "exhaustive"):
        threshold, minimum, rows = tune(caches["validation"], set(gold["validation"]), method)
        configurations[method] = {"threshold": threshold, "min_span": minimum}
        grid[method] = rows
        predicted = predictions(caches["test"], method, threshold, minimum)
        candidates = {(query, source) for query, sources in caches["test"][method].items() for source in sources}
        row = {"method": method, **pair_metrics(set(predicted), set(gold["test"]), caches["test"]["universe"]),
               "candidate_recall": len(candidates & set(gold["test"])) / len(gold["test"]),
               "coverage": caches["test"]["coverage"],
               "target_spans": span_metrics(predicted, gold["test"], "target"),
               "source_spans": span_metrics(predicted, gold["test"], "source")}
        results.append(row)
    # Freeze the LSH settings for a second exhaustive run isolating retrieval loss.
    config = configurations["lsh"]
    exact_same = predictions(caches["test"], "exhaustive", config["threshold"], config["min_span"])
    results.append({"method": "exhaustive_lsh_settings", **pair_metrics(set(exact_same), set(gold["test"]), caches["test"]["universe"]),
                    "candidate_recall": results[1]["candidate_recall"], "coverage": caches["test"]["coverage"]})
    cosine = {}
    for name, cache in caches.items():
        queries, sources = sorted(cache["queries"]), sorted(cache["detector"].sources)
        matrix = (vectorizer.transform([texts[k] for k in queries])
                  @ vectorizer.transform([texts[k] for k in sources]).T).toarray()
        cosine[name] = {(q, s): float(matrix[i, j]) for i, q in enumerate(queries) for j, s in enumerate(sources)}
    cosine_rows = []
    for threshold in np.arange(.1, 1, .1):
        predicted = {pair for pair, value in cosine["validation"].items() if value >= threshold}
        cosine_rows.append({"threshold": float(threshold), **pair_metrics(predicted, set(gold["validation"]), caches["validation"]["universe"])})
    best = max(cosine_rows, key=lambda r: (r["f1"] or 0, r["precision"] or 0, r["threshold"]))
    configurations["tfidf_cosine"] = {"threshold": best["threshold"]}
    grid["tfidf_cosine"] = cosine_rows
    cosine_predictions = {pair for pair, value in cosine["test"].items() if value >= best["threshold"]}
    results.append({"method": "tfidf_cosine", **pair_metrics(cosine_predictions, set(gold["test"]), caches["test"]["universe"]),
                    "candidate_recall": 1.0, "coverage": None})
    breakdown = []
    methods = {"lsh": set(predictions(caches["test"], "lsh", **config)),
               "exhaustive_lsh_settings": set(exact_same), "tfidf_cosine": cosine_predictions}
    negatives = caches["test"]["universe"] - set(gold["test"])
    for category in ("literal", "light", "heavy", "translation", "other_or_unknown"):
        positives = {pair for pair, spans in gold["test"].items() if any(s["category"] == category for s in spans)}
        universe = negatives | positives
        for name, predicted in methods.items():
            breakdown.append({"method": name, "category": category, **pair_metrics(predicted & universe, positives, universe)})
    # High-level cut: smallest validation similarity >= medium with precision >= .90.
    medium = config["threshold"]
    high_options = [r["threshold"] for r in grid["lsh"] if r["min_span"] == config["min_span"]
                    and r["threshold"] >= medium and r["precision"] is not None and r["precision"] >= .9]
    high = min(high_options) if high_options else None
    example_source = min(caches["test"]["detector"].sources)
    constructed = constructed_checks(example_source, texts[example_source], config,
                                     args.seed, args.mode, args.num_perm)
    _, peak = tracemalloc.get_traced_memory()
    tracemalloc.stop()
    artifact = {"version": "0.1.0", "seed": args.seed, "mode": args.mode, "num_perm": args.num_perm,
                "environment": {"python": platform.python_version(), "platform": platform.platform(),
                                "processor": platform.processor(),
                                "libraries": {name: importlib.metadata.version(name) for name in
                                              ("numpy", "pandas", "scikit-learn", "datasketch")},
                                "source_sha256": {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
                                                  for p in sorted(Path(__file__).parent.glob("*.py"))}},
                "templates": templates, "configurations": configurations,
                "levels": {"medium": medium, "high": high}, "validation_grid": grid,
                "test": results, "breakdown": breakdown, "constructed_checks": constructed, "manifest": manifest,
                "seconds": time.perf_counter() - started, "python_peak_bytes": peak,
                "timings": {k: {n: c[n] for n in ("lsh_seconds", "exhaustive_seconds")} for k, c in caches.items()}}
    args.artifacts.mkdir(parents=True, exist_ok=True)
    (args.artifacts / "experiment.json").write_text(json.dumps(artifact, indent=2, allow_nan=False), encoding="utf-8")
    lines = ["<!-- ml/integrity/results.md -->", "# PAN-PC-11 closed-family subset results", "",
             manifest["selection"], f"Selected {len(selected)} of {len(paths)} documents; seed {args.seed}.", "",
             "| Method | Pairs | Positives | TP | FP | FN | TN | Candidate recall | Precision | Recall | F1 | FPR |",
             "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|"]
    def cell(value: Any) -> str:
        return "NA" if value is None else f"{value:.4f}" if isinstance(value, float) else str(value)
    columns = ("method", "pairs", "positives", "tp", "fp", "fn", "tn", "candidate_recall", "precision", "recall", "f1", "fpr")
    lines += ["| " + " | ".join(cell(row[k]) for k in columns) + " |" for row in results]
    lines += ["", "## Validation-selected settings", "", "```json", json.dumps(configurations, indent=2), "```",
              "", "## Annotation categories (pairs can belong to multiple categories)", "",
              "| Method | Category | Positive pairs | Precision | Recall | F1 |", "|---|---|---:|---:|---:|---:|"]
    lines += ["| " + " | ".join(cell(row[k]) for k in ("method", "category", "positives", "precision", "recall", "f1")) + " |" for row in breakdown]
    lines += ["", "## Pair-conditioned character-span micro metrics", "",
              "| Method | Side | Precision | Recall |", "|---|---|---:|---:|"]
    for row in results[:2]:
        for side in ("target", "source"):
            span = row[side + "_spans"]
            lines.append(f"| {row['method']} | {side} | {cell(span['precision'])} | {cell(span['recall'])} |")
    lines += ["", "## Constructed hard negatives, separate from PAN", "",
              "| Case | Pairs | Flagged | False positives under construction assumption |", "|---|---:|---:|---:|"]
    lines += [f"| {row['case']} | 1 | {row['flagged']} | {row['false_positive_under_assumption']} |" for row in constructed]
    lines += ["", f"Query coverage (>=5 non-template tokens): {caches['test']['coverage']:.4f}.",
              f"Total wall time: {artifact['seconds']:.2f} s; Python-tracked peak: {peak} bytes (not total RSS).",
              "TF-IDF vocabulary/IDF fit only on train; exhaustive comparisons use identical held-out pairs.",
              "Span metrics are not official PAN plagdet/granularity. Missed candidates count as false negatives.",
              "The three constructed stress cases assume attributed quotation/provided templates are legitimate. They are not independently adjudicated or PAN truth.",
              "Detailed grids, hashes, manifests and retrieval timings: artifacts/experiment.json.", ""]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines), encoding="utf-8")
    return artifact


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", type=Path, required=True)
    parser.add_argument("--max-documents", type=int, default=120)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--mode", choices=("windows", "whole"), default="windows")
    parser.add_argument("--num-perm", type=int, choices=(128, 256), default=128)
    parser.add_argument("--templates", type=Path)
    parser.add_argument("--artifacts", type=Path, default=Path("ml/integrity/artifacts"))
    parser.add_argument("--output", type=Path, default=Path("ml/integrity/results.md"))
    args = parser.parse_args()
    if args.max_documents < 10:
        parser.error("--max-documents must be >=10")
    result = evaluate(args)
    print(json.dumps(result["test"], indent=2))


if __name__ == "__main__":
    main()
