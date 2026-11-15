# ml/ednet/common.py
"""Shared helpers for the EdNet pipeline: paths, hashing, user split, windows.

The per-response table produced by ``ml.ednet.extract`` is a dict of numpy
arrays (one row per Part-5 response, sorted by user then bundle ordinal) plus
ragged arrays ``resp_ts``/``resp_ans`` holding every ``respond`` action.
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any

import numpy as np

RAW_DIR = Path("ml/datasets/raw/ednet")
KT2_ZIP = RAW_DIR / "EdNet-KT2.zip"
CONTENTS_ZIP = RAW_DIR / "contents.zip"
DERIVED = RAW_DIR / "derived" / "part5_responses.npz"

# Fixed code tables keep the cache layout independent of worker order.
SOURCES = ("adaptive_offer", "archive", "diagnosis", "in_review", "my_note", "review",
           "review_quiz", "sprint", "tutor", "other")
PLATFORMS = ("mobile", "web", "other")
OPTIONS = "abcd"
WINDOW_SIZE = 20
DEFAULT_MAX_GAP_MS = 300_000
# Sources excluded from every downstream step because they re-solve questions
# the learner has already seen (answers are partly remembered, not solved).
# KT2 spells the brief's ``in_review`` as ``review`` (97.6 % repeat exposures in
# a 3,000-user sample); ``my_note`` replays questions the learner bookmarked.
# Independently of source, repeat exposures (``is_repeat``) are excluded too.
EXCLUDED_SOURCES = ("in_review", "review", "my_note")

SPLIT_SALT = "examory-ednet-split-v1"
SAMPLE_SALT = "examory-ednet-sample-v1"
HOLDOUT_SALT = "examory-ednet-calibration-holdout-v1"


def file_sha256(path: str | Path) -> str:
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def write_json(path: str | Path, value: Any) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False, allow_nan=False) + "\n",
                    encoding="utf-8")


def unit_hash(salt: str, user: int) -> float:
    """Stable uniform number in [0, 1) from SHA-256 (independent of PYTHONHASHSEED)."""
    digest = hashlib.sha256(f"{salt}:{int(user)}".encode()).digest()
    return int.from_bytes(digest[:8], "big") / 2.0 ** 64


def hash_array(salt: str, users: np.ndarray) -> np.ndarray:
    unique, inverse = np.unique(np.asarray(users, dtype=np.int64), return_inverse=True)
    values = np.fromiter((unit_hash(salt, u) for u in unique), dtype=float, count=len(unique))
    return values[inverse]


def is_calibration_user(users: np.ndarray) -> np.ndarray:
    """50/50 user-level split: True = calibration user, False = evaluation user."""
    return hash_array(SPLIT_SALT, users) < 0.5


def load_table(path: str | Path = DERIVED) -> dict[str, np.ndarray]:
    with np.load(path, allow_pickle=False) as data:
        return {key: data[key] for key in data.files}


def subset(table: dict[str, np.ndarray], mask: np.ndarray) -> dict[str, np.ndarray]:
    """Select rows (and their ragged responds), re-basing ``resp_start``."""
    rows = {k: v[mask] for k, v in table.items() if k not in {"resp_ts", "resp_ans"}
            and v.shape[:1] == mask.shape}
    starts, counts = table["resp_start"][mask], table["n_resp"][mask]
    index = ragged_index(starts, counts)
    rows["resp_ts"] = table["resp_ts"][index]
    rows["resp_ans"] = table["resp_ans"][index]
    rows["resp_start"] = np.concatenate([[0], np.cumsum(counts)[:-1]]).astype(np.int64)
    return rows


def ragged_index(starts: np.ndarray, counts: np.ndarray) -> np.ndarray:
    """Concatenate ``arange(s, s + c)`` for every row without a Python loop."""
    counts = np.asarray(counts, dtype=np.int64)
    total = int(counts.sum())
    if total == 0:
        return np.zeros(0, dtype=np.int64)
    offsets = np.repeat(np.asarray(starts, dtype=np.int64) - np.concatenate([[0], np.cumsum(counts)[:-1]]),
                        counts)
    return offsets + np.arange(total, dtype=np.int64)


def change_events(table: dict[str, np.ndarray], row: int) -> list[tuple[int, int]]:
    """Visible answer events of one response: [(ts, option), ...].

    The first respond is the initial answer; a later respond is a change only if
    its option differs from the immediately preceding respond (a re-tap of the
    same option is not a change and emits no event).
    """
    start, count = int(table["resp_start"][row]), int(table["n_resp"][row])
    ts = table["resp_ts"][start:start + count]
    ans = table["resp_ans"][start:start + count]
    events = [(int(ts[0]), int(ans[0]))]
    for k in range(1, count):
        if ans[k] != ans[k - 1]:
            events.append((int(ts[k]), int(ans[k])))
    return events


def usable_mask(table: dict[str, np.ndarray]) -> np.ndarray:
    """Rows eligible for any analysis: first exposures from non-excluded sources."""
    source_names = np.asarray(SOURCES)[table["source"]]
    return ~np.isin(source_names, EXCLUDED_SOURCES) & ~table["is_repeat"]


def find_windows(table: dict[str, np.ndarray], max_gap_ms: int = DEFAULT_MAX_GAP_MS,
                 size: int = WINDOW_SIZE, require_adjacent: bool = False) -> dict[str, np.ndarray]:
    """Non-overlapping pseudo-attempts of ``size`` consecutive Part-5 responses.

    Rows must be sorted by (user, ordinal). Excluded sources and repeat
    exposures are removed first; the remaining rows form the learner's Part-5
    stream. Two neighbouring rows of that stream belong to the same sitting when
    the gap from the previous submit to the next enter lies in [0, max_gap_ms]
    (other bundles answered inside that gap are invisible time; with
    ``require_adjacent=True`` they break the sitting instead). A response whose
    own enter -> submit time exceeds max_gap_ms is an interruption and is
    removed, which also breaks the sitting. Windows are cut greedily from the
    start of every sitting. Returns ``rows`` (n_windows x size row indices),
    ``user`` and ``window_rank`` (0-based order of the window within its user).
    """
    user, ordinal = table["user"], table["ordinal"]
    valid = usable_mask(table)
    valid &= (table["submit_ts"] - table["enter_ts"]) <= max_gap_ms
    index = np.flatnonzero(valid)
    if len(index) == 0:
        empty = np.zeros((0, size), dtype=np.int64)
        return {"rows": empty, "user": np.zeros(0, np.int64), "window_rank": np.zeros(0, np.int64)}
    u, o = user[index], ordinal[index]
    gap = table["enter_ts"][index][1:] - table["submit_ts"][index][:-1]
    linked = np.zeros(len(index), dtype=bool)  # linked[i]: index[i] continues from index[i-1]
    linked[1:] = (u[1:] == u[:-1]) & (gap >= 0) & (gap <= max_gap_ms)
    if require_adjacent:
        linked[1:] &= o[1:] == o[:-1] + 1
    segment = np.cumsum(~linked)
    starts = np.flatnonzero(~linked)
    position = np.arange(len(index)) - starts[segment - 1]
    seg_len = np.diff(np.append(starts, len(index)))[segment - 1]
    full = position < (seg_len // size) * size
    chosen = index[full].reshape(-1, size)
    window_user = user[chosen[:, 0]]
    rank = np.zeros(len(chosen), dtype=np.int64)
    if len(chosen):
        new_user = np.r_[True, window_user[1:] != window_user[:-1]]
        first = np.flatnonzero(new_user)
        rank = np.arange(len(chosen)) - first[np.cumsum(new_user) - 1]
    return {"rows": chosen, "user": window_user, "window_rank": rank}
