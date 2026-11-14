# ml/ednet/extract.py
"""Stream EdNet-KT2 once and cache one row per TOEIC Part-5 response.

Only single-question Part-5 bundles are kept (the bundle structure is verified
from ``contents/questions.csv``; multi-question Part-5 bundles, if any, are
dropped). Every ``respond`` of a kept bundle is retained so that downstream code
can rebuild the answer/change sequence. The zip members are read directly; no
files are extracted. Timestamps are EdNet's (maintainer-shifted) Unix ms and are
used only as relative times.

Malformed Part-5 bundles are dropped and counted: missing enter/submit, submit
for a different bundle, respond for a different question, an answer outside
a-d, a bundle without any respond, or non-monotone timestamps inside the bundle.
KT2 sometimes logs a bundle's respond AFTER its submit; such a bundle counts as
``no_respond`` and the late line as ``orphan_respond_line_any_part`` (orphan
lines are counted over all parts, not only Part 5).
``is_repeat`` marks a response to a question the learner already answered
earlier in the log (any source).

Output (``--output``, default ml/datasets/raw/ednet/derived/part5_responses.npz):
rows sorted by (user, ordinal) with columns user, question, ordinal (index of
the bundle in the learner's full enter stream, all parts), enter_ts,
first_respond_ts, final_respond_ts, submit_ts, next_enter_ts (-1 = none),
source, platform (codes into common.SOURCES / PLATFORMS), n_resp, n_changes,
first_answer, final_answer (0..3 = a..d), first_correct, final_correct,
first_answer_time_ms, gap_ms (next enter - submit, -1 = none), is_repeat, is_calibration,
resp_start; ragged resp_ts, resp_ans. A ``.meta.json`` sidecar holds counts.

Example: .venv/bin/python -m ml.ednet.extract --max-users 2000
"""
from __future__ import annotations

import argparse
import io
import itertools
import os
import time
import zipfile
from collections import Counter
from multiprocessing import Pool
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

from ml.ednet.common import (
    CONTENTS_ZIP,
    DERIVED,
    KT2_ZIP,
    OPTIONS,
    PLATFORMS,
    SOURCES,
    file_sha256,
    is_calibration_user,
    ragged_index,
    write_json,
)

HEADER = "timestamp,action_type,item_id,source,user_answer,platform"
ROW_FIELDS = ("user", "question", "ordinal", "enter_ts", "submit_ts", "next_enter_ts",
              "source", "platform", "n_resp")
_P5: dict[str, int] = {}


def load_questions(contents_zip: str | Path) -> dict[str, Any]:
    """Part-5 bundle -> question map and the answer key, with structure checks."""
    with zipfile.ZipFile(contents_zip) as archive, archive.open("contents/questions.csv") as stream:
        questions = pd.read_csv(stream, dtype=str, keep_default_na=False)
    questions["part"] = questions["part"].astype(int)
    part5 = questions[questions.part == 5]
    per_bundle = part5.groupby("bundle_id").question_id.count()
    multi = per_bundle[per_bundle > 1].index
    # A Part-5 bundle must not share a bundle_id with questions of another part.
    mixed = set(questions[(questions.part != 5) & questions.bundle_id.isin(part5.bundle_id)].bundle_id)
    keep = part5[~part5.bundle_id.isin(multi) & ~part5.bundle_id.isin(mixed)
                 & part5.correct_answer.isin(list(OPTIONS))]
    key = {int(q[1:]): OPTIONS.index(a) for q, a in zip(questions.question_id, questions.correct_answer,
                                                         strict=True) if a in OPTIONS}
    return {
        "bundle_to_question": {b: int(q[1:]) for b, q in zip(keep.bundle_id, keep.question_id, strict=True)},
        "answer_key": key,
        "report": {"questions_total": len(questions), "part5_questions": len(part5),
                   "part5_bundles": int(per_bundle.size),
                   "part5_multi_question_bundles_dropped": len(multi),
                   "part5_bundles_mixed_with_other_parts_dropped": len(mixed),
                   "part5_single_question_bundles_kept": len(keep),
                   "part5_bad_answer_key": int((~part5.correct_answer.isin(list(OPTIONS))).sum())},
    }


def parse_user(text: str, user: int, p5: dict[str, int]) -> tuple[list[tuple], list[int], list[int], Counter]:
    """Parse one KT2 user file; returns (rows, respond ts, respond answers, malformed)."""
    rows: list[tuple] = []
    flat_ts: list[int] = []
    flat_ans: list[int] = []
    bad: Counter = Counter()
    lines = text.splitlines()
    if not lines or lines[0].strip() != HEADER:
        bad["bad_header_file"] += 1
        return rows, flat_ts, flat_ans, bad
    ordinal = -1
    cur: list | None = None  # [bundle, ordinal, enter_ts, source, platform, question, responds, reason]
    awaiting_next: int | None = None  # index into rows waiting for the next enter ts

    def close(reason: str | None, submit_ts: int | None) -> None:
        nonlocal awaiting_next
        if cur is None or cur[5] is None:
            return
        responds, reason = cur[6], reason or cur[7]
        if reason is None and not responds:
            reason = "no_respond"
        if reason is None:
            times = [cur[2]] + [r[0] for r in responds] + [submit_ts]
            if any(b < a for a, b in itertools.pairwise(times)):
                reason = "non_monotone_ts"
        if reason is not None:
            bad[reason] += 1
            return
        rows.append((user, cur[5], cur[1], cur[2], submit_ts, -1, cur[3], cur[4], len(responds)))
        for ts, answer in responds:
            flat_ts.append(ts)
            flat_ans.append(answer)
        awaiting_next = len(rows) - 1

    for line in lines[1:]:
        if not line:
            continue
        parts = line.split(",")
        if len(parts) != 6:
            bad["bad_line"] += 1
            continue
        try:
            ts = int(parts[0])
        except ValueError:
            bad["bad_line"] += 1
            continue
        action, item = parts[1], parts[2]
        if action == "enter":
            if cur is not None:
                close("missing_submit", None)
            ordinal += 1
            if awaiting_next is not None:
                previous = rows[awaiting_next]
                rows[awaiting_next] = previous[:5] + (ts,) + previous[6:]
                awaiting_next = None
            question = p5.get(item)
            source = SOURCES.index(parts[3]) if parts[3] in SOURCES else SOURCES.index("other")
            platform = PLATFORMS.index(parts[5]) if parts[5] in PLATFORMS else PLATFORMS.index("other")
            cur = [item, ordinal, ts, source, platform, question, [], None]
        elif action == "respond":
            if cur is None:
                bad["orphan_respond_line_any_part"] += 1
                continue
            if cur[5] is None:
                continue
            if item != f"q{cur[5]}":
                cur[7] = cur[7] or "foreign_respond"
            elif parts[4] not in OPTIONS or not parts[4]:
                cur[7] = cur[7] or "bad_answer"
            else:
                cur[6].append((ts, OPTIONS.index(parts[4])))
        elif action == "submit":
            if cur is None:
                bad["orphan_submit_line_any_part"] += 1
                continue
            if item != cur[0]:
                close("submit_mismatch", None)
            else:
                close(None, ts)
            cur = None
        else:
            bad["unknown_action_line"] += 1
    if cur is not None:
        close("missing_submit", None)
    return rows, flat_ts, flat_ans, bad


def _init(p5: dict[str, int]) -> None:
    global _P5
    _P5 = p5


def _process(job: tuple[str, list[str]]) -> dict[str, Any]:
    zip_path, names = job
    rows: list[tuple] = []
    flat_ts: list[int] = []
    flat_ans: list[int] = []
    starts: list[int] = []
    bad: Counter = Counter()
    with zipfile.ZipFile(zip_path) as archive:
        for name in names:
            user = int(Path(name).stem[1:])
            text = io.TextIOWrapper(archive.open(name), encoding="utf-8").read()
            r, t, a, b = parse_user(text, user, _P5)
            offset = len(flat_ts)
            for row in r:
                starts.append(offset)
                offset += row[-1]
            rows.extend(r)
            flat_ts.extend(t)
            flat_ans.extend(a)
            bad.update(b)
    array = np.array(rows, dtype=np.int64).reshape(-1, len(ROW_FIELDS))
    out = {field: array[:, i] for i, field in enumerate(ROW_FIELDS)}
    out["resp_start"] = np.array(starts, dtype=np.int64)
    out["resp_ts"] = np.array(flat_ts, dtype=np.int64)
    out["resp_ans"] = np.array(flat_ans, dtype=np.int8)
    out["malformed"] = bad
    out["files"] = len(names)
    return out


def user_members(zip_path: str | Path, max_users: int | None) -> list[str]:
    with zipfile.ZipFile(zip_path) as archive:
        names = sorted((n for n in archive.namelist()
                        if n.startswith("KT2/u") and n.endswith(".csv") and "/._" not in n),
                       key=lambda n: int(Path(n).stem[1:]))
    return names[:max_users] if max_users else names


def build_table(parts: list[dict[str, Any]], answer_key: dict[int, int]) -> dict[str, np.ndarray]:
    table: dict[str, np.ndarray] = {}
    for field in ROW_FIELDS:
        table[field] = np.concatenate([p[field] for p in parts]) if parts else np.zeros(0, np.int64)
    shifts = np.cumsum([0] + [len(p["resp_ts"]) for p in parts[:-1]])
    resp_start = np.concatenate([p["resp_start"] + s for p, s in zip(parts, shifts, strict=True)])
    resp_ts = np.concatenate([p["resp_ts"] for p in parts])
    resp_ans = np.concatenate([p["resp_ans"] for p in parts])
    order = np.lexsort((table["ordinal"], table["user"]))
    table = {k: v[order] for k, v in table.items()}
    resp_start, n_resp = resp_start[order], table["n_resp"]
    index = ragged_index(resp_start, n_resp)
    resp_ts, resp_ans = resp_ts[index], resp_ans[index]
    resp_start = np.concatenate([[0], np.cumsum(n_resp)[:-1]]).astype(np.int64)
    last = resp_start + n_resp - 1
    first_answer, final_answer = resp_ans[resp_start], resp_ans[last]
    # Changes: later responds whose option differs from the preceding respond.
    differs = np.zeros(len(resp_ans), dtype=np.int64)
    differs[1:] = resp_ans[1:] != resp_ans[:-1]
    differs[resp_start] = 0
    cumulative = np.cumsum(differs)
    n_changes = cumulative[last] - cumulative[resp_start]
    key = np.array([answer_key[int(q)] for q in table["question"]], dtype=np.int8)
    gap = np.where(table["next_enter_ts"] >= 0, table["next_enter_ts"] - table["submit_ts"], -1)
    # Repeat exposure: the learner already answered this question earlier (rows are time-ordered).
    pair = table["user"].astype(np.int64) * 100_000 + table["question"]
    _, first_seen = np.unique(pair, return_index=True)
    is_repeat = np.ones(len(pair), dtype=bool)
    is_repeat[first_seen] = False
    table.update({
        "question": table["question"].astype(np.int32),
        "ordinal": table["ordinal"].astype(np.int32),
        "source": table["source"].astype(np.int8), "platform": table["platform"].astype(np.int8),
        "n_resp": n_resp.astype(np.int32), "n_changes": n_changes.astype(np.int32),
        "first_respond_ts": resp_ts[resp_start], "final_respond_ts": resp_ts[last],
        "first_answer": first_answer, "final_answer": final_answer,
        "first_correct": first_answer == key, "final_correct": final_answer == key,
        "first_answer_time_ms": resp_ts[resp_start] - table["enter_ts"],
        "gap_ms": gap, "is_repeat": is_repeat, "is_calibration": is_calibration_user(table["user"]),
        "resp_start": resp_start, "resp_ts": resp_ts, "resp_ans": resp_ans,
    })
    return table


def extract(kt2_zip: str | Path, contents_zip: str | Path, output: str | Path,
            max_users: int | None = None, workers: int | None = None) -> dict[str, Any]:
    started = time.time()
    questions = load_questions(contents_zip)
    names = user_members(kt2_zip, max_users)
    workers = max(1, min(workers or (os.cpu_count() or 2) - 2, len(names) or 1))
    chunk = max(1, min(500, -(-len(names) // (workers * 8))))
    jobs = [(str(kt2_zip), names[i:i + chunk]) for i in range(0, len(names), chunk)]
    if workers == 1:
        _init(questions["bundle_to_question"])
        parts = [_process(job) for job in jobs]
    else:
        with Pool(workers, initializer=_init, initargs=(questions["bundle_to_question"],)) as pool:
            parts = pool.map(_process, jobs)
    malformed: Counter = Counter()
    for part in parts:
        malformed.update(part.pop("malformed"))
    table = build_table(parts, questions["answer_key"])
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    np.savez_compressed(output, **table)
    source_names = np.asarray(SOURCES)[table["source"]]
    users = np.unique(table["user"])
    meta = {
        "inputs": {"kt2_zip": str(kt2_zip), "kt2_sha256": file_sha256(kt2_zip),
                   "contents_zip": str(contents_zip), "contents_sha256": file_sha256(contents_zip)},
        "max_users": max_users, "user_files_read": len(names),
        "questions": questions["report"],
        "responses": len(table["user"]), "users_with_part5": len(users),
        "calibration_users": int(is_calibration_user(users).sum()),
        "responses_by_source": {s: int((source_names == s).sum()) for s in SOURCES},
        "repeat_share_by_source": {s: round(float(table["is_repeat"][source_names == s].mean()), 4)
                                   for s in SOURCES if (source_names == s).any()},
        "responses_by_platform": {p: int((table["platform"] == i).sum()) for i, p in enumerate(PLATFORMS)},
        "malformed_part5_bundles_dropped": dict(sorted(malformed.items())),
        "responds_total": len(table["resp_ts"]),
        "responses_with_change": int((table["n_changes"] > 0).sum()),
        "negative_gap_rows": int(((table["next_enter_ts"] >= 0)
                                  & (table["next_enter_ts"] < table["submit_ts"])).sum()),
        "output": str(output), "output_sha256": file_sha256(output),
        "seconds": round(time.time() - started, 1), "workers": workers,
    }
    write_json(output.with_suffix(".meta.json"), meta)
    return meta


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--kt2", type=Path, default=KT2_ZIP)
    parser.add_argument("--contents", type=Path, default=CONTENTS_ZIP)
    parser.add_argument("--output", type=Path, default=DERIVED)
    parser.add_argument("--max-users", type=int, default=None, help="first N user files (by id)")
    parser.add_argument("--workers", type=int, default=None)
    args = parser.parse_args()
    meta = extract(args.kt2, args.contents, args.output, args.max_users, args.workers)
    print({k: meta[k] for k in ("responses", "users_with_part5", "responses_by_source",
                                "malformed_part5_bundles_dropped", "seconds")})


if __name__ == "__main__":
    main()
