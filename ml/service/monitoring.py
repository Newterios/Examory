"""Read-only diagnostics for the scorer's internal health port."""
from __future__ import annotations

import json
import re
from collections.abc import Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import BoundedSemaphore, Thread
from typing import Protocol

import psycopg

from .metrics import Metrics, Snapshot

_STATUS_PATH = re.compile(r"/attempts/([A-Za-z0-9_-]{1,64})/status")
_PROMETHEUS = "text/plain; version=0.0.4; charset=utf-8"

# Return only a fixed public state or reason. Older rows contain free-form diagnostics.
_PUBLIC_STATE = """
CASE
    WHEN status IN ('pending', 'scored', 'flagged') THEN status
    WHEN detail = 'error:scoring' OR starts_with(detail, 'scoring error:') THEN 'error/scoring'
    WHEN detail = 'error:invalid_output' OR starts_with(detail, 'produced event invalid:') THEN 'error/invalid_output'
    WHEN starts_with(detail, 'error:') THEN 'error/other'
    WHEN starts_with(detail, 'not scorable by frozen pipeline v1') THEN 'not_scorable/unsupported_input'
    WHEN detail = 'no events for the attempt' THEN 'not_scorable/no_events'
    WHEN detail IN ('non-finite score', 'non-finite iforest score', 'non-finite quantile_rules score')
        THEN 'not_scorable/non_finite_score'
    ELSE 'not_scorable/other'
END
"""
_STATUS = f"SELECT {_PUBLIC_STATE} FROM attempts WHERE attempt_id = %s"
_COUNTS = f"SELECT {_PUBLIC_STATE}, count(*) FROM attempts GROUP BY 1"
_LAG = """
SELECT CASE WHEN max(occurred_at) IS NULL THEN NULL
            ELSE greatest(0, extract(epoch FROM now() - max(occurred_at))) END
FROM quiz_events
"""


class MonitoringUnavailable(Exception):
    """A diagnostic query was unavailable or the read connection limit was reached."""


class HealthProbe(Protocol):
    def alive(self) -> bool: ...


class Reader(Protocol):
    def status(self, attempt_id: str) -> dict[str, str] | None: ...
    def snapshot(self) -> Snapshot: ...


class PostgresReader:
    def __init__(self, dsn: str, max_connections: int = 4) -> None:
        if max_connections < 1:
            raise ValueError("max_connections must be positive")
        self._dsn = dsn
        self._slots = BoundedSemaphore(max_connections)

    @contextmanager
    def _cursor(self) -> Iterator[psycopg.Cursor]:
        if not self._slots.acquire(blocking=False):
            raise MonitoringUnavailable
        try:
            with psycopg.connect(
                self._dsn, connect_timeout=2,
                options="-c default_transaction_read_only=on -c statement_timeout=2000",
            ) as connection:
                connection.isolation_level = psycopg.IsolationLevel.REPEATABLE_READ
                with connection.cursor() as cursor:
                    yield cursor
        except (psycopg.Error, OSError):
            raise MonitoringUnavailable from None
        finally:
            self._slots.release()

    def status(self, attempt_id: str) -> dict[str, str] | None:
        with self._cursor() as cursor:
            cursor.execute(_STATUS, (attempt_id,))
            row = cursor.fetchone()
        if row is None:
            return None
        state, _, reason = row[0].partition("/")
        return {"status": state, "reason": reason} if reason else {"status": state}

    def snapshot(self) -> Snapshot:
        result: Snapshot = {"pending": 0, "scored": 0, "flagged": 0, "not_scorable": {}, "error": {},
                            "watermark_lag_seconds": None}
        with self._cursor() as cursor:
            cursor.execute(_COUNTS)
            for code, count in cursor.fetchall():
                state, _, reason = code.partition("/")
                if reason:
                    result[state][reason] = count
                else:
                    result[state] = count
            cursor.execute(_LAG)
            lag = cursor.fetchone()[0]
        result["watermark_lag_seconds"] = None if lag is None else float(lag)
        return result


def serve_health(health: HealthProbe, port: int, reader: Reader, metrics: Metrics) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def reply(self, code: int, body: bytes, content_type: str) -> None:
            self.send_response(code)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            if code == 405:
                self.send_header("Allow", "GET")
            self.end_headers()
            if self.command != "HEAD":
                self.wfile.write(body)

        def reply_json(self, code: int, payload: dict[str, str]) -> None:
            self.reply(code, json.dumps(payload).encode("utf-8"), "application/json")

        def do_GET(self) -> None:
            if self.path == "/healthz":
                alive = health.alive()
                self.reply_json(200 if alive else 503, {"status": "ok" if alive else "unavailable"})
                return
            try:
                if self.path == "/metrics":
                    self.reply(200, metrics.render(reader.snapshot()).encode("utf-8"), _PROMETHEUS)
                    return
                match = _STATUS_PATH.fullmatch(self.path)
                if match and (state := reader.status(match[1])) is not None:
                    self.reply_json(200, state)
                    return
            except MonitoringUnavailable:
                self.reply_json(503, {"status": "unavailable"})
                return
            self.reply_json(404, {"status": "not_found"})

        def send_error(self, code: int, message: str | None = None, explain: str | None = None) -> None:
            # BaseHTTPRequestHandler otherwise echoes malformed request lines and method names.
            if code == 501:
                self.reply_json(405, {"status": "method_not_allowed"})
            else:
                self.reply_json(code, {"status": "bad_request"})

        def log_message(self, *_args: object) -> None:
            # Paths, query strings and request text can contain student identifiers.
            pass

    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    Thread(target=server.serve_forever, daemon=True).start()
    return server
