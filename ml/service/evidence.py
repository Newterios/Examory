"""Teacher-facing timing evidence for a validated, flagged quiz attempt."""
from __future__ import annotations

from typing import Any

from ml.anomaly.features import item_key


def question_evidence(attempt: dict[str, Any], target_events: list[dict[str, Any]],
                      reference: dict[str, Any]) -> list[dict[str, Any]]:
    """Use the same item-specific or global training p10 as fast_answer_fraction."""
    indices: dict[str, int] = {}
    changed: set[str] = set()
    for event in target_events:
        payload = event["payload"]
        if event["event_type"] != "quiz.answered" or payload["attempt_id"] != attempt["attempt_id"]:
            continue
        question = payload["question_id"]
        indices[question] = payload["question_index"]
        if payload["is_change"]:
            changed.add(question)

    item_medians = reference.get("item_median_ms", {})
    global_median = reference.get("global_median_ms")
    result = []
    for question in sorted(attempt["first_times"], key=indices.__getitem__):
        elapsed = attempt["first_times"][question]
        key = item_key(attempt, question)
        threshold = reference["item_fast_ms"].get(key, reference["global_fast_ms"])
        median = item_medians.get(key)
        if median is not None:
            scope = "item_train"
        else:
            median = global_median
            scope = "global_train" if median is not None else "unavailable"
        result.append({
            "question_index": indices[question],
            "first_answer_time_ms": elapsed,
            "reference_median_ms": median,
            "reference_scope": scope,
            "fast_threshold_ms": threshold,
            "is_fast": elapsed < threshold,
            "answer_changed": question in changed,
        })
    return result
