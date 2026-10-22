# ml/integrity/minhash.py
"""Windowed five-token MinHash retrieval with exact verification and offsets."""
from __future__ import annotations

import argparse
import json
import re
import unicodedata
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from datasketch import MinHash, MinHashLSH


@dataclass(frozen=True)
class Token:
    value: str
    start: int
    end: int


@dataclass
class Window:
    start: int
    end: int
    shingles: set[tuple[str, ...]]
    signature: MinHash


@dataclass
class Document:
    identifier: str
    text: str
    tokens: list[Token]
    windows: list[Window]


def tokenize(text: str) -> list[Token]:
    """NFKC/casefold words with original Python-character half-open offsets.

    Combining marks stay with their original word. Compatibility expansion can
    emit multiple normalized words; each maps to the complete original word.
    Offsets are Unicode code points, not UTF-8 bytes or JavaScript UTF-16 units.
    """
    result = []
    start = None
    for index in range(len(text) + 1):
        word = index < len(text) and (text[index] == "_" or unicodedata.category(text[index])[0] in "LNM")
        if word and start is None:
            start = index
        elif not word and start is not None:
            normalized = unicodedata.normalize("NFKC", text[start:index]).casefold()
            for match in re.finditer(r"\w+", normalized, flags=re.UNICODE):
                result.append(Token(match.group(), start, index))
            start = None
    return result


def shingles(tokens: list[Token], excluded: set[tuple[str, ...]]) -> set[tuple[str, ...]]:
    return {tuple(t.value for t in tokens[i:i + 5]) for i in range(len(tokens) - 4)} - excluded


def document(identifier: str, text: str, seed: int = 42, num_perm: int = 128,
             mode: str = "windows", excluded: set[tuple[str, ...]] | None = None) -> Document:
    if num_perm not in {128, 256} or mode not in {"windows", "whole"}:
        raise ValueError("Use 128/256 permutations and windows/whole mode")
    excluded = excluded or set()
    tokens = tokenize(text)
    windows = []
    starts = [0] if mode == "whole" else range(0, len(tokens), 100)
    for start in starts:
        end = len(tokens) if mode == "whole" else min(len(tokens), start + 200)
        values = shingles(tokens[start:end], excluded)
        if values:
            signature = MinHash(num_perm=num_perm, seed=seed)
            signature.update_batch([json.dumps(value, ensure_ascii=False).encode("utf-8") for value in sorted(values)])
            windows.append(Window(start, end, values, signature))
    return Document(identifier, text, tokens, windows)


def exact_jaccard(left: set, right: set) -> float:
    return len(left & right) / len(left | right) if left or right else 0.0


def token_runs(left: Document, right: Document, a: Window, b: Window,
               excluded: set[tuple[str, ...]]) -> list[tuple[int, int, int, int]]:
    """Recover matching contiguous runs from shared five-shingle positions."""
    positions: dict[tuple[str, ...], list[int]] = defaultdict(list)
    for j in range(b.start, b.end - 4):
        value = tuple(t.value for t in right.tokens[j:j + 5])
        if value not in excluded:
            positions[value].append(j)
    diagonals: dict[int, list[int]] = defaultdict(list)
    for i in range(a.start, a.end - 4):
        value = tuple(t.value for t in left.tokens[i:i + 5])
        for j in positions.get(value, []):
            diagonals[j - i].append(i)
    runs = []
    for delta, starts in diagonals.items():
        begin, end = starts[0], starts[0] + 5
        for start in starts[1:]:
            if start <= end:
                end = max(end, start + 5)
            else:
                runs.append((begin, end, begin + delta, end + delta))
                begin, end = start, start + 5
        runs.append((begin, end, begin + delta, end + delta))
    return runs


def merged_spans(left: Document, right: Document, runs: list[tuple[int, int, int, int]],
                 min_span: int) -> list[dict[str, Any]]:
    diagonals: dict[int, list[tuple[int, int]]] = defaultdict(list)
    for a, b, c, _ in set(runs):
        diagonals[c - a].append((a, b))
    output = []
    for delta, intervals in diagonals.items():
        merged: list[list[int]] = []
        for begin, end in sorted(intervals):
            if merged and begin <= merged[-1][1]:
                merged[-1][1] = max(end, merged[-1][1])
            else:
                merged.append([begin, end])
        for begin, end in merged:
            if end - begin >= min_span:
                output.append({
                    "target": [left.tokens[begin].start, left.tokens[end - 1].end],
                    "source": [right.tokens[begin + delta].start, right.tokens[end + delta - 1].end],
                    "token_count": end - begin,
                })
    # Multiple repeated phrases can legitimately map to several source positions.
    return sorted(output, key=lambda span: (span["target"], span["source"]))


class Detector:
    def __init__(self, sources: dict[str, str], seed: int = 42, num_perm: int = 128,
                 mode: str = "windows", templates: list[str] | None = None) -> None:
        self.seed, self.num_perm, self.mode = seed, num_perm, mode
        self.excluded = set().union(*(shingles(tokenize(t), set()) for t in templates or []))
        self.sources = {key: document(key, value, seed, num_perm, mode, self.excluded)
                        for key, value in sources.items()}
        self.index = MinHashLSH(num_perm=num_perm, params=(num_perm // 4, 4))
        for key, source in self.sources.items():
            for index, window in enumerate(source.windows):
                self.index.insert((key, index), window.signature)

    def prepare(self, key: str, text: str) -> Document:
        return document(key, text, self.seed, self.num_perm, self.mode, self.excluded)

    def retrieve(self, query: Document, exhaustive: bool = False) -> dict[str, list[dict[str, Any]]]:
        """Retain all exact window scores; validation can tune without reindexing."""
        pairs: dict[str, list[dict[str, Any]]] = defaultdict(list)
        for window in query.windows:
            candidates = ([(key, i) for key, doc in self.sources.items() for i in range(len(doc.windows))]
                          if exhaustive else self.index.query(window.signature))
            for key, index in sorted(candidates):
                if key == query.identifier:
                    continue
                source = self.sources[key]
                other = source.windows[index]
                similarity = exact_jaccard(window.shingles, other.shingles)
                # Candidate membership must survive even if exact overlap is zero.
                pairs[key].append({"similarity": similarity,
                                   "runs": token_runs(query, source, window, other, self.excluded)
                                   if similarity >= .5 else []})
        return dict(pairs)

    def verify(self, query: Document, candidates: dict[str, list[dict[str, Any]]],
               threshold: float = .65, min_span: int = 20) -> list[dict[str, Any]]:
        if not .5 <= threshold <= 1 or min_span < 5:
            raise ValueError("Exact threshold must be >=.5; minimum span >=5 tokens")
        matches = []
        for key, hits in candidates.items():
            accepted = [hit for hit in hits if hit["similarity"] >= threshold]
            runs = [run for hit in accepted for run in hit["runs"]]
            spans = merged_spans(query, self.sources[key], runs, min_span)
            if spans:
                matches.append({"submission_id": key,
                                "similarity": max(hit["similarity"] for hit in accepted),
                                "overlap_spans": spans})
        return sorted(matches, key=lambda match: (-match["similarity"], match["submission_id"]))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--query", type=Path, required=True)
    parser.add_argument("--sources", type=Path, required=True, help="Directory of UTF-8 .txt files")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--threshold", type=float, default=.65)
    parser.add_argument("--min-span", type=int, default=20)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--num-perm", type=int, choices=(128, 256), default=128)
    parser.add_argument("--mode", choices=("windows", "whole"), default="windows")
    parser.add_argument("--templates", type=Path, help="JSON list of teacher-provided template texts")
    args = parser.parse_args()
    sources = {p.relative_to(args.sources).as_posix(): p.read_bytes().decode("utf-8") for p in sorted(args.sources.rglob("*.txt"))}
    if not sources:
        parser.error("No source .txt files")
    templates = json.loads(args.templates.read_text(encoding="utf-8")) if args.templates else []
    detector = Detector(sources, args.seed, args.num_perm, args.mode, templates)
    query = detector.prepare(args.query.name, args.query.read_bytes().decode("utf-8"))
    candidates = detector.retrieve(query)
    result = {"submission_id": query.identifier, "covered": bool(query.windows),
              "candidates": sorted(candidates),
              "matches": detector.verify(query, candidates, args.threshold, args.min_span)}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, ensure_ascii=False), encoding="utf-8")


if __name__ == "__main__":
    main()
