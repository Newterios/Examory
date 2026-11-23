"""Bounded scorer metrics in Prometheus text exposition format 0.0.4."""
from __future__ import annotations

import math
from bisect import bisect_left
from itertools import accumulate
from threading import Lock
from typing import TypedDict

EVENT_OUTCOMES = ("stored", "duplicate", "ignored", "rejected")
NOT_SCORABLE_REASONS = ("unsupported_input", "no_events", "non_finite_score", "other")
ERROR_REASONS = ("scoring", "invalid_output", "other")


class Snapshot(TypedDict):
    pending: int
    scored: int
    flagged: int
    not_scorable: dict[str, int]
    error: dict[str, int]
    watermark_lag_seconds: float | None


class Histogram:
    """Mutated and rendered under the Metrics lock. Store disjoint buckets."""

    def __init__(self, bounds: tuple[float, ...]) -> None:
        self.bounds = bounds
        self.counts = [0] * (len(bounds) + 1)
        self.total = 0.0

    def observe(self, seconds: float) -> None:
        if not math.isfinite(seconds) or seconds < 0:
            raise ValueError("Histogram observations must be finite, nonnegative seconds")
        self.counts[bisect_left(self.bounds, seconds)] += 1
        self.total += seconds

    def lines(self, name: str, description: str) -> list[str]:
        lines = [f"# HELP {name} {description}", f"# TYPE {name} histogram"]
        labels = [str(bound) for bound in self.bounds] + ["+Inf"]
        for label, count in zip(labels, accumulate(self.counts), strict=True):
            lines.append(f'{name}_bucket{{le="{label}"}} {count}')
        lines.extend([f"{name}_sum {self.total}", f"{name}_count {sum(self.counts)}"])
        return lines


class Metrics:
    def __init__(self) -> None:
        self._lock = Lock()
        self._events = dict.fromkeys(EVENT_OUTCOMES, 0)
        self._scoring = Histogram((0.005, 0.025, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60))
        self._publication = Histogram((0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 1800, 3600))

    def record_event(self, outcome: str) -> None:
        with self._lock:
            self._events[outcome] += 1

    def observe_scoring(self, seconds: float) -> None:
        with self._lock:
            self._scoring.observe(seconds)

    def observe_publication(self, seconds: float) -> None:
        with self._lock:
            self._publication.observe(seconds)

    def render(self, snapshot: Snapshot) -> str:
        name = "examory_anomaly_events_total"
        with self._lock:
            lines = [f"# HELP {name} Kafka records handled by this scorer process.", f"# TYPE {name} counter"]
            for outcome, count in self._events.items():
                lines.append(f'{name}{{outcome="{outcome}"}} {count}')
            lines.extend(self._scoring.lines("examory_anomaly_scoring_duration_seconds",
                                             "Duration of a ready attempt scoring operation."))
            lines.extend(self._publication.lines("examory_anomaly_submission_to_publication_seconds",
                                                 "Elapsed time from submission to recorded broker acknowledgement."))

        name = "examory_anomaly_attempts"
        lines.extend([f"# HELP {name} Attempts in each persisted assessment state.", f"# TYPE {name} gauge"])
        for status in ("pending", "scored", "flagged"):
            lines.append(f'{name}{{status="{status}"}} {snapshot[status]}')
        for status in ("not_scorable", "error"):
            lines.append(f'{name}{{status="{status}"}} {sum(snapshot[status].values())}')
        name = "examory_anomaly_attempts_by_reason"
        lines.extend([f"# HELP {name} Unscorable or failed attempts by reason.", f"# TYPE {name} gauge"])
        for status, reasons in (("not_scorable", NOT_SCORABLE_REASONS), ("error", ERROR_REASONS)):
            for reason in reasons:
                count = snapshot[status].get(reason, 0)
                lines.append(f'{name}{{status="{status}",reason="{reason}"}} {count}')
        name = "examory_anomaly_watermark_lag_seconds"
        lag = snapshot["watermark_lag_seconds"]
        lines.extend([f"# HELP {name} Age of the latest stored quiz event at scrape time.",
                      f"# TYPE {name} gauge", f"{name} {lag if lag is not None else 'NaN'}"])
        return "\n".join(lines) + "\n"
