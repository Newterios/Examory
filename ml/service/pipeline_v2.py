"""Accept sparse and weighted live quizzes while preserving the v1 model feature space."""
from __future__ import annotations

import io
from collections import defaultdict
from typing import Any

import numpy as np

from ml.anomaly.features import read_attempts

POLICY = "observed_answers_weighted_score_v2"


def attempt_v2(events: list[dict[str, Any]]) -> dict[str, Any]:
    from .scoring import to_csv

    unique: dict[str, dict] = {}
    for event in events:
        if unique.setdefault(event["event_id"], event) != event:
            raise ValueError("conflicting event redelivery")
    envelopes = list(unique.values())
    submitted = [e for e in envelopes if e["event_type"] == "quiz.submitted"]
    started = [e for e in envelopes if e["event_type"] == "quiz.started"]
    answers = [e for e in envelopes if e["event_type"] == "quiz.answered"]
    if len(submitted) != 1 or len(started) > 1 or not answers:
        raise ValueError("need one submission and at least one observed first answer")
    sub = submitted[0]["payload"]
    score, maximum = sub["score"], sub["max_score"]
    if type(score) is not int or type(maximum) is not int or not 0 <= score <= maximum or maximum == 0:
        raise ValueError("invalid weighted score")
    reason = sub.get("reason", "submitted")
    if reason not in {"timed_out", "submitted"}:
        raise ValueError("unknown submission reason")

    indices: dict[str, int] = {}
    for event in answers:
        p = event["payload"]
        question, index = p["question_id"], p["question_index"]
        if type(index) is not int or index < 0 or indices.setdefault(question, index) != index:
            raise ValueError("invalid item index")
    if len(set(indices.values())) != len(indices):
        raise ValueError("questions share one index")
    total = None
    if started:
        p = started[0]["payload"]
        if any(p[k] != sub[k] for k in ("attempt_id", "quiz_id", "variant_id")):
            raise ValueError("quiz start and submission disagree")
        total = p["question_count"]
        if type(total) is not int or total < 1 or max(indices.values()) >= total:
            raise ValueError("observed index outside quiz roster")

    # The frozen parser validates IDs, timestamps, identities, hashes and revision
    # sequences. Its internal unit-score view contains only observed questions.
    # Restore actual weighted percentage before any history, model or evidence use.
    dense = {qid: i for i, qid in enumerate(sorted(indices, key=indices.__getitem__))}
    structural = [dict(e, payload=dict(e["payload"], question_index=dense[e["payload"]["question_id"]]))
                  for e in answers]
    structural.append(dict(submitted[0], payload=dict(sub, score=0, max_score=len(indices))))
    attempt = read_attempts(io.StringIO(to_csv(structural)))[0]
    attempt.update(score_percent=100 * score / maximum, submission_reason=reason,
                   answered_questions=len(indices), total_questions=total)
    return attempt


def history_v2(attempts: list[dict[str, Any]]) -> list[dict[str, Any]]:
    attempts.sort(key=lambda a: (a["submitted_at"], a["attempt_id"]))
    by_user: dict[tuple, list[dict]] = defaultdict(list)
    for attempt in attempts:
        key = (attempt["user_id"], attempt["quiz_id"], attempt["variant_id"])
        by_user[key].append(attempt)
    for rows in by_user.values():
        for row in rows:
            prior = [p["score_percent"] for p in rows if p["submitted_at"] < row["submitted_at"]]
            row["history_missing"] = int(not prior)
            row["score_delta"] = row["score_percent"] - float(np.mean(prior)) if prior else None
            row["history_sd"] = float(np.std(prior)) if prior else None
            row["is_final_attempt"] = row["submitted_at"] == rows[-1]["submitted_at"]
    return attempts


def prepare_v2(events_by_attempt: dict[str, list[dict]]) -> tuple[list[dict], dict[str, str]]:
    valid, excluded = [], {}
    for attempt_id, events in events_by_attempt.items():
        try:
            attempt = attempt_v2(events)
            if attempt["attempt_id"] != attempt_id:
                raise ValueError("incorrect attempt routing")
            valid.append(attempt)
        except (ValueError, KeyError, TypeError, OverflowError) as exc:
            excluded[attempt_id] = f"not scorable by pipeline v2: {exc}"[:500]
    return history_v2(valid), excluded
