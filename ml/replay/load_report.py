"""Measure real submission-to-signal delays, drain and scorer resources for one replay."""
from __future__ import annotations

import argparse
import csv
import itertools
import json
import math
import re
import statistics
import subprocess
import time
from collections import Counter
from datetime import datetime
from pathlib import Path


def epoch(stamp: str) -> float:
    dt = datetime.fromisoformat(stamp)
    if dt.tzinfo is None:
        raise ValueError("Replay wall timestamps must include a timezone")
    return dt.timestamp()


def percentiles(values: list[float]) -> dict[str, float | None]:
    ordered = sorted(values)
    result = {}
    for q in (50, 95, 99):
        if ordered:
            pos = (len(ordered) - 1) * q / 100
            lo = math.floor(pos)
            result[f"p{q}"] = ordered[lo] + (pos - lo) * (ordered[math.ceil(pos)] - ordered[lo])
        else:
            result[f"p{q}"] = None
    return result


def resource_summary(path: Path, start: float, end: float) -> dict:
    with path.open(encoding="utf-8", newline="") as stream:
        rows = [tuple(float(row[k]) for k in ("wall_epoch_s", "monotonic_s", "cpu_usage_usec", "memory_bytes"))
                for row in csv.DictReader(stream)]
    if len(rows) < 2 or not rows[0][0] <= start < end <= rows[-1][0]:
        raise ValueError("Resource samples must bracket the entire replay window")
    total_cpu = total_time = memory_integral = max_memory = max_gap = 0.
    interval_cpu = []
    for (t0, m0, c0, b0), (t1, m1, c1, b1) in itertools.pairwise(rows):
        if t1 <= t0 or m1 <= m0 or c1 < c0:
            raise ValueError("Clock or cgroup counter changed discontinuously")
        left, right = max(t0, start), min(t1, end)
        if right <= left:
            continue
        lo, hi = (left - t0) / (t1 - t0), (right - t0) / (t1 - t0)
        dt = (hi - lo) * (m1 - m0)
        dc = (hi - lo) * (c1 - c0) / 1e6
        left_mem, right_mem = b0 + lo * (b1 - b0), b0 + hi * (b1 - b0)
        total_cpu += dc
        total_time += dt
        memory_integral += dt * (left_mem + right_mem) / 2
        max_memory = max(max_memory, left_mem, right_mem)
        max_gap = max(max_gap, t1 - t0)
        interval_cpu.append(100 * dc / dt)
    return {"mean_cpu_percent_one_core": 100 * total_cpu / total_time,
            "peak_sampling_interval_cpu_percent_one_core": max(interval_cpu),
            "mean_memory_mib": memory_integral / total_time / 2**20,
            "sampled_peak_memory_mib": max_memory / 2**20, "cpu_seconds": total_cpu,
            "intervals": len(interval_cpu), "max_sample_gap_s": max_gap}


def query(container: str, database: str, statement: str) -> list:
    # Use a PostgreSQL local socket in the isolated Compose container. No password in argv.
    cmd = ["docker", "exec", "-i", container, "psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1",
           "-U", "examory", "-d", database]
    sql = "BEGIN READ ONLY;\nSET LOCAL statement_timeout='20s';\n" + statement + ";\nCOMMIT;\n"
    out = subprocess.run(cmd, input=sql, text=True, check=True, capture_output=True, timeout=35)
    return [json.loads(line) for line in out.stdout.splitlines() if line.strip()]


def sql_ids(ids: list[str]) -> str:
    if not ids or not all(isinstance(key, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,64}", key) for key in ids):
        raise ValueError("Expected at least one valid replay attempt identifier")
    return ", ".join("'" + key + "'" for key in ids)


def queue_times(container: str, keys: str) -> dict[str, float]:
    rows = query(container, "quizzes", f"""
        WITH submissions AS (
            SELECT convert_from(envelope, 'UTF8')::jsonb #>> '{{payload,attempt_id}}' AS attempt_id, created_at
            FROM outbox WHERE event_type = 'quiz.submitted'
        )
        SELECT json_build_array(attempt_id, extract(epoch FROM created_at))
        FROM submissions WHERE attempt_id IN ({keys})
    """)
    if len(rows) != len(dict(rows)):
        raise ValueError("Multiple emitted submissions for a replay attempt")
    return dict(rows)


def scoring_state(container: str, keys: str) -> dict:
    rows = query(container, "anomaly", f"""
        SELECT json_build_array(a.attempt_id, a.status, extract(epoch FROM a.scored_at),
                                extract(epoch FROM r.published_at))
        FROM attempts a LEFT JOIN risk_outbox r USING (attempt_id)
        WHERE a.attempt_id IN ({keys})
    """)
    return {key: (status, scored_at, published_at) for key, status, scored_at, published_at in rows}


def event_counts(container: str, keys: str, *, received: bool) -> dict[str, int]:
    if received:
        database = "anomaly"
        source = "quiz_events"
        attempt = "attempt_id"
    else:
        database = "quizzes"
        source = "outbox"
        attempt = "convert_from(envelope, 'UTF8')::jsonb #>> '{payload,attempt_id}'"
    return dict(query(container, database, f"""
        SELECT json_build_array(event_type, count(*)) FROM {source}
        WHERE event_type IN ('quiz.started','quiz.answered','quiz.submitted')
          AND ({attempt}) IN ({keys}) GROUP BY event_type
    """))


def measure(manifest: dict, samples: Path, postgres: str, drain_seconds: int) -> dict:
    start, end = (epoch(manifest[key]) for key in ("window_start", "window_end"))
    if manifest["mode"] != "load" or not start < end:
        raise ValueError("Expected a finished load replay with a positive duration")
    acts = manifest["actions"]
    events = [a for a in acts if a["kind"] in ("quiz.started", "quiz.answered", "quiz.submitted")]
    successes = [a for a in events if 200 <= a.get("http_status", 0) < 300 and not a.get("error")]
    ids = [a["id"] for a in manifest["attempts"].values() if a.get("status") == "submitted"]
    if len(set(ids)) != len(ids):
        raise ValueError("Multiple source attempts map to one platform attempt")
    keys = sql_ids(ids) if ids else None
    started = [a["id"] for a in manifest["attempts"].values() if a.get("id")]
    started_keys = sql_ids(started) if started else None
    queued = queue_times(postgres, keys) if keys else {}
    emitted = event_counts(postgres, started_keys, received=False) if started_keys else {}
    until = time.monotonic() + drain_seconds
    while True:
        states = scoring_state(postgres, keys) if keys else {}
        waiting = sum(1 for key in ids if key not in states or states[key][0] == "pending"
                      or (states[key][0] == "flagged" and states[key][2] is None))
        received = event_counts(postgres, started_keys, received=True) if started_keys and not waiting else {}
        if (waiting == 0 and received == emitted) or time.monotonic() >= until:
            if waiting and started_keys:
                received = event_counts(postgres, started_keys, received=True)
            break
        time.sleep(min(2, max(0, until - time.monotonic())))
    published = {key: ts for key, (_, _, ts) in states.items() if ts is not None}
    latency = [1000 * (ts - queued[key]) for key, ts in published.items() if key in queued]
    if any(x < 0 for x in latency):
        raise ValueError("Negative wall-clock latency: verify that the platform clocks agree")
    dispatches = [epoch(a["sent_at"]) for a in successes]
    lags = [1000 * (epoch(a["sent_at"]) - epoch(a["scheduled_at"])) for a in successes]
    duration = manifest["duration_ms"] / 1000
    target = round(manifest["rate"] * duration)
    buckets = [0] * math.ceil(duration / 60)
    for stamp in dispatches:
        age = stamp - start
        if 0 <= age < duration:
            buckets[int(age // 60)] += 1
    acked_submits = sum(a["kind"] == "quiz.submitted" for a in successes)
    failures = sum(bool(a.get("error")) or not 200 <= a.get("http_status", 0) < 300 for a in acts)
    terminal_by_end = sum(scored is not None and scored <= end for _, scored, _ in states.values())
    submitted_by_end = sum(t <= end for t in queued.values())
    last_ack = max(published.values(), default=None)
    terminal_times = [t for _, t, _ in states.values() if t is not None]
    accepted = dict(Counter(a["kind"] for a in successes))
    complete = (not manifest.get("error") and bool(ids) and len(events) == target and len(successes) == target
                and received == emitted == accepted and failures == 0 and waiting == 0
                and len(ids) == acked_submits == len(queued) and len(latency) == len(published))
    return {"complete": complete, "window_start": manifest["window_start"], "window_end": manifest["window_end"],
            "target_rate": manifest["rate"], "target_duration_s": duration, "measured_window_s": end - start,
            "scheduled_event_target": target, "scheduled_events": len(events), "successful_events": len(successes),
            "http_failures_including_shown": failures, "events_per_measured_second": len(successes) / (end - start),
            "successful_event_dispatches_per_60s": buckets, "dispatched_outside_nominal_window": len(successes) - sum(buckets),
            "successful_events_dispatched_per_nominal_second": sum(buckets) / duration,
            "dispatch_delay_from_scheduled_slot_ms": {**percentiles(lags), "max": max(lags, default=None)},
            "successful_event_actions": accepted, "quiz_outbox_events": emitted, "scorer_inbox_events": received,
            "shown_requests": sum(a["kind"] == "question.shown" for a in acts),
            "manifest_attempts_by_status": dict(Counter(a["status"] for a in manifest["attempts"].values())),
            "confirmed_submissions": len(queued), "final_attempt_states": dict(Counter(v[0] for v in states.values())),
            "unresolved_at_drain_timeout": waiting, "submitted_not_yet_scored_at_window_end": submitted_by_end - terminal_by_end,
            "published_signals": len(published), "signals_published_by_window_end": sum(t <= end for t in published.values()),
            "seconds_after_window_to_last_ack": max(0, last_ack - end) if last_ack is not None else None,
            "seconds_after_window_to_last_scoring": max(0, max(terminal_times) - end) if terminal_times else None,
            "submit_to_signal_ms": {**percentiles(latency), "n": len(latency),
                                    "mean": statistics.fmean(latency) if latency else None},
            "scorer_resources": resource_summary(samples, start, end)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--samples", type=Path, required=True)
    parser.add_argument("--postgres", required=True, help="Local research project PostgreSQL container ID")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--drain-seconds", type=int, default=900)
    args = parser.parse_args()
    if args.drain_seconds < 0:
        parser.error("--drain-seconds must be nonnegative")
    metadata = subprocess.check_output(["docker", "inspect", "--format",
                                        '{{index .Config.Labels "com.docker.compose.service"}}', args.postgres], text=True)
    if metadata.strip() != "postgres":
        parser.error("--postgres must identify the local Compose postgres service")
    result = measure(json.loads(args.manifest.read_text(encoding="utf-8")), args.samples,
                     args.postgres, args.drain_seconds)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    print(json.dumps(result, allow_nan=False))
    if not result["complete"]:
        raise SystemExit(2)


if __name__ == "__main__":
    main()
