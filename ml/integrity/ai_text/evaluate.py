"""Offline E-18 comparison on HC3 English and a held-out human learner corpus."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
from collections import Counter
from pathlib import Path

import numpy as np
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import make_pipeline
from sklearn.preprocessing import StandardScaler

WORDS = re.compile(r"\b[\w'-]+\b")


def digest(text: str) -> bytes:
    return hashlib.sha256(" ".join(text.casefold().split()).encode()).digest()


def load_hc3(path: Path) -> list[tuple]:
    rows = []
    seen = {}
    with path.open(encoding="utf-8") as stream:
        for line in stream:
            doc = json.loads(line)
            prompt = digest(doc["question"])
            bucket = int.from_bytes(prompt[:8], "big") % 10
            split = "train" if bucket < 6 else "validation" if bucket < 8 else "test"
            for klass, key in enumerate(("human_answers", "chatgpt_answers")):
                for answer in doc[key]:
                    if not answer.strip():
                        continue
                    hashed = digest(answer)
                    if hashed in seen:
                        if seen[hashed] != klass:
                            seen[hashed] = -1
                        continue
                    seen[hashed] = klass
                    rows.append((split, doc["source"], prompt, klass, answer, hashed))
    return [row[:5] for row in rows if seen[row[5]] >= 0]


def trigrams(text: str) -> np.ndarray:
    # Fixed ASCII alphabet plus a shared non-ASCII symbol. Zero denotes BOS/EOS.
    chars = np.frombuffer(
        text.lower().encode("ascii", "replace"), dtype=np.uint8
    ).astype(np.int32)
    chars = np.concatenate(([0, 0], chars, [0]))
    return (chars[:-2] * 128 + chars[1:-1]) * 128 + chars[2:]


class Perplexity:
    """Add-one-smoothed character trigram LM fit only on generated training answers."""

    def __init__(self, generated):
        counts = np.ones(128**3, dtype=np.int64)
        for text in generated:
            np.add.at(counts, trigrams(text), 1)
        totals = counts.reshape(-1, 128).sum(axis=1)
        self.logp = np.log(counts) - np.repeat(np.log(totals), 128)

    def score(self, text: str) -> float:
        return float(np.exp(-self.logp[trigrams(text)].mean()))


def style(text: str) -> list[float]:
    words = WORDS.findall(text.lower())
    lengths = [len(w) for w in words] or [0]
    sentences = [
        len(WORDS.findall(s)) for s in re.split(r"[!?.]+", text) if s.strip()
    ] or [0]
    n = max(len(words), 1)
    return [
        math.log1p(n),
        sum(lengths) / n,
        len(set(words)) / n,
        sum(nchar >= 8 for nchar in lengths) / n,
        float(np.mean(sentences)),
        float(np.std(sentences)),
        text.count(",") / n,
        text.count("!") / n,
        text.count("?") / n,
        text.count("\n") / n,
        sum(ch.isupper() for ch in text) / max(1, len(text)),
    ]


def threshold(scores, labels, max_fpr=0.05):
    order = np.argsort(-scores, kind="stable")
    x, y = scores[order], labels[order]
    ends = np.flatnonzero(np.r_[x[1:] != x[:-1], True])
    false = np.cumsum(y == 0)[ends]
    true = np.cumsum(y == 1)[ends]
    ok = np.flatnonzero(false <= max_fpr * sum(labels == 0))
    if not len(ok) or not max(true[ok]):
        return float(np.nextafter(x[0], np.inf))
    best = ok[np.argmax(true[ok])]
    return float(x[ends[best]])


def false_positives(positive, group):
    _, keys = np.unique(group, return_inverse=True)
    den = np.bincount(keys)
    num = np.bincount(keys, weights=positive)
    rng = np.random.default_rng(42)
    rates = []
    for _ in range(1000):
        ix = rng.integers(0, len(den), size=len(den))
        rates.append(num[ix].sum() / den[ix].sum())
    return f"{positive.sum()}/{len(positive)} ({positive.mean():.1%}, {np.percentile(rates, 2.5):.1%}–{np.percentile(rates, 97.5):.1%})"


def evaluate(hc3: Path, learners: Path, out: Path):
    rows = load_hc3(hc3)
    external = json.loads(learners.read_text(encoding="utf-8"))
    rows += [
        ("external", "TOEFL", digest(d["document"]), 0, d["document"]) for d in external
    ]
    splits, sources, prompts, y, texts = zip(*rows, strict=True)
    splits, sources, prompts, y = map(np.asarray, (splits, sources, prompts, y))
    training, validation, test = (splits == s for s in ("train", "validation", "test"))
    lm = Perplexity(t for i, t in enumerate(texts) if training[i] and y[i])
    ppl = np.array([lm.score(t) for t in texts])
    X = np.array([style(t) for t in texts])
    classifier = make_pipeline(
        StandardScaler(), LogisticRegression(max_iter=500, random_state=42)
    )
    classifier.fit(X[training], y[training])
    lines = [
        "# Offline English AI-text indicators",
        "",
        "Each normalized HC3 prompt belongs to one deterministic 60/20/20 partition. Identical answer texts occur once, and answers with conflicting labels are discarded. A character trigram model estimates perplexity under generated training answers. The second model fits logistic regression on eleven length, vocabulary, sentence and punctuation features. Each method maximizes validation recall subject to a 5% human FPR budget. Thresholds are frozen before test.",
        "",
        "| Partition | Human | Generated |",
        "|---|---:|---:|",
    ]
    for name in ("train", "validation", "test", "external"):
        counts = Counter(y[splits == name])
        lines.append(f"| {name} | {counts[0]} | {counts[1]} |")
    lines.extend(
        [
            "",
            "| Method | Frozen rule | Test human FP / total (FPR, 95% CI) | Test AI recall | TOEFL human FP / total (FPR, 95% CI) |",
            "|---|---|---|---:|---|",
        ]
    )
    for name, scores in (
        ("Perplexity", -ppl),
        ("Stylometry", classifier.predict_proba(X)[:, 1]),
    ):
        cut = threshold(scores[validation], y[validation])
        flagged = scores >= cut
        human, ai, learner = test & (y == 0), test & (y == 1), splits == "external"
        rule = f"PPL ≤ {-cut:.5f}" if name == "Perplexity" else f"score ≥ {cut:.5f}"
        lines.append(
            f"| {name} | {rule} | {false_positives(flagged[human], prompts[human])} | {flagged[ai].mean():.1%} | {false_positives(flagged[learner], prompts[learner])} |"
        )
    lines.extend(
        [
            "",
            "Intervals use 1,000 bootstrap samples of prompts for HC3 and essays for TOEFL. HC3 answers have no native-language labels. The external set contains human TOEFL essays from the authors’ bias study. Its topic, genre and length differ from HC3 answers, so a difference between the two FPRs cannot isolate the effect of language background. The experiment covers English only and produces no platform event.",
            "",
            "[HC3 English, Guo et al.](https://huggingface.co/datasets/Hello-SimpleAI/HC3/tree/4d0ff18143b5a7e1b1e79beb540c04549d1e59d3) · [TOEFL human essays, Liang et al.](https://github.com/Weixin-Liang/ChatGPT-Detector-Bias/tree/1d05ed8242d4694a0ed7b73a79c52be8864470f9/Data_and_Results/Human_Data/TOEFL_real_91)",
            "",
            "Reproduce with the original English `all.jsonl` and the TOEFL directory’s `data.json`. Raw answers and essays stay outside Git.",
            "",
            "```sh",
            "python -m ml.integrity.ai_text.evaluate --hc3 /path/to/all.jsonl --learners /path/to/data.json --out ml/integrity/ai_text/results.md",
            "```",
            "",
        ]
    )
    out.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    for arg in ("hc3", "learners", "out"):
        p.add_argument("--" + arg, type=Path, required=True)
    args = p.parse_args()
    evaluate(args.hc3, args.learners, args.out)
