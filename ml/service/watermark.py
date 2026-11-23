"""Readiness rule for scoring a submitted attempt.

The frozen feature pipeline (ml/anomaly/features.py) compares an attempt with
peers of the same group, quiz and variant whose submissions lie within +-5 s
of event time, so an attempt may only be scored once every such peer
submission can have arrived: the *event-time watermark* must pass
``submitted_at + 5 s``.

Watermark = the latest ``occurred_at`` of any quiz event stored so far. That
is only an approximation of a per-partition watermark (``lms.quizzes`` has
several partitions); the bounded-lateness fallback below covers both lagging
partitions and a quiet stream (the last submissions of a session would
otherwise never become ready): an attempt is also ready once ``lateness``
of processing time has passed since its submission was *received*.
"""
from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta

PEER_WINDOW = timedelta(seconds=5)


@dataclass(frozen=True)
class Readiness:
    ready: bool
    reason: str  # "watermark" | "lateness" | "waiting"


def is_ready(submitted_at: datetime, watermark: datetime | None, received_at: datetime,
             now: datetime, lateness: timedelta) -> Readiness:
    """All datetimes must be timezone-aware UTC."""
    for value in (submitted_at, received_at, now) + ((watermark,) if watermark else ()):
        if value.tzinfo is None:
            raise ValueError("timezone-aware datetimes required")
    if watermark is not None and watermark >= submitted_at + PEER_WINDOW:
        return Readiness(True, "watermark")
    if now >= received_at + PEER_WINDOW + lateness:
        return Readiness(True, "lateness")
    return Readiness(False, "waiting")
