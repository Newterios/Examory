"""Exercise the health port and bounded, privacy-preserving status projection."""
from __future__ import annotations

import io
import json
import sqlite3
import unittest
from contextlib import closing, redirect_stderr
from decimal import Decimal
from http.client import HTTPConnection
from unittest.mock import Mock, patch

import psycopg

from ml.service import monitoring
from ml.service.metrics import Metrics
from ml.service.monitoring import MonitoringUnavailable, PostgresReader, serve_health


class MonitoringHTTPTests(unittest.TestCase):
    def setUp(self) -> None:
        self.health = Mock()
        self.health.alive.return_value = True
        self.reader = Mock()
        self.server = serve_health(self.health, 0, self.reader, Metrics())
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)

    def request(self, path: str, method: str = "GET") -> tuple[int, dict[str, str], bytes]:
        with closing(HTTPConnection("127.0.0.1", self.server.server_port, timeout=2)) as conn:
            conn.request(method, path)
            response = conn.getresponse()
            return response.status, dict(response.getheaders()), response.read()

    def test_health_is_independent_of_database(self) -> None:
        for alive, status, label in [(True, 200, "ok"), (False, 503, "unavailable")]:
            self.health.alive.return_value = alive
            code, headers, data = self.request("/healthz")
            self.assertEqual((code, json.loads(data)), (status, {"status": label}))
            self.assertEqual(headers["Cache-Control"], "no-store")
        self.assertFalse(self.reader.mock_calls)

    def test_metrics_and_status_error_when_read_side_is_unavailable(self) -> None:
        self.reader.snapshot.side_effect = MonitoringUnavailable("private database detail")
        self.reader.status.side_effect = MonitoringUnavailable("private database detail")
        for path in ["/metrics", "/attempts/private-id/status"]:
            code, _, body = self.request(path)
            self.assertEqual((code, json.loads(body)), (503, {"status": "unavailable"}))
        self.assertEqual(self.request("/healthz")[0], 200)

    def test_metrics_content_type_and_counts(self) -> None:
        self.reader.snapshot.return_value = {"pending": 1, "scored": 0, "flagged": 2, "error": {},
                                             "not_scorable": {}, "watermark_lag_seconds": None}
        code, headers, body = self.request("/metrics")
        self.assertEqual(code, 200)
        self.assertEqual(headers["Content-Type"], "text/plain; version=0.0.4; charset=utf-8")
        self.assertIn(b'examory_anomaly_attempts{status="flagged"} 2\n', body)
        self.assertIn(b"examory_anomaly_watermark_lag_seconds NaN\n", body)

    def test_status_privacy_and_unknown_ids(self) -> None:
        private = "student-private-77"
        states = [{"status": s} for s in ("pending", "scored", "flagged")]
        states += [{"status": "not_scorable", "reason": "unsupported_input"}, {"status": "error", "reason": "scoring"}]
        with redirect_stderr(io.StringIO()) as logs:
            for state in states:
                self.reader.status.return_value = state
                code, _, body = self.request(f"/attempts/{private}/status")
                self.assertEqual((code, json.loads(body)), (200, state))
                self.reader.status.assert_called_with(private)
            self.reader.status.return_value = None
            code, _, body = self.request(f"/attempts/{private}/status")
            self.assertEqual((code, json.loads(body)), (404, {"status": "not_found"}))
            self.reader.status.reset_mock()
            for path in (f"/unknown/{private}", f"/attempts/{private}%2Fmore/status",
                         f"/attempts/{private}/status?answer=private", "/attempts/" + "a" * 65 + "/status"):
                self.assertEqual(self.request(path)[0], 404)
            self.reader.status.assert_not_called()
        self.assertNotIn(private, logs.getvalue())

    def test_methods_do_not_echo_request_text(self) -> None:
        with redirect_stderr(io.StringIO()) as logs:
            for method in ("POST", "HEAD", "PRIVATE-TEXT"):
                code, headers, body = self.request("/attempts/private-id/status", method)
                self.assertEqual((code, headers["Allow"]), (405, "GET"))
                if method == "HEAD":
                    self.assertEqual(body, b"")
                else:
                    self.assertEqual(json.loads(body), {"status": "method_not_allowed"})
        self.assertEqual(logs.getvalue(), "")
        self.assertFalse(self.reader.mock_calls)


class MonitoringDBTests(unittest.TestCase):
    def test_sql_projects_legacy_and_current_details_to_finite_codes(self) -> None:
        # Exercise the actual CASE from both production queries against seeded rows.
        rows = [
            ("pending", "error:scoring", "pending"), ("scored", "private answer", "scored"),
            ("flagged", None, "flagged"),
            ("not_scorable", "scoring error: WithPrivateDetail", "error/scoring"),
            ("not_scorable", "produced event invalid: private diagnostic", "error/invalid_output"),
            ("not_scorable", "error:scoring", "error/scoring"),
            ("not_scorable", "error:invalid_output", "error/invalid_output"),
            ("not_scorable", "error:private", "error/other"),
            ("not_scorable", "no events for the attempt", "not_scorable/no_events"),
            ("not_scorable", "non-finite score", "not_scorable/non_finite_score"),
            ("not_scorable", "non-finite iforest score", "not_scorable/non_finite_score"),
            ("not_scorable", "non-finite quantile_rules score", "not_scorable/non_finite_score"),
            ("not_scorable", "not scorable by frozen pipeline v1: private", "not_scorable/unsupported_input"),
            ("not_scorable", "private", "not_scorable/other"), ("not_scorable", None, "not_scorable/other"),
        ]
        with closing(sqlite3.connect(":memory:")) as db:
            db.create_function("starts_with", 2, lambda s, prefix: s.startswith(prefix) if s is not None else None)
            db.execute("CREATE TABLE attempts(attempt_id, status, detail)")
            for i, (status, detail, expected) in enumerate(rows):
                db.execute("INSERT INTO attempts VALUES (?, ?, ?)", (i, status, detail))
                result = db.execute(monitoring._STATUS.replace("%s", "?"), (i,)).fetchone()
                self.assertEqual(result, (expected,))
            counts = dict(db.execute(monitoring._COUNTS))
        self.assertEqual(sum(counts.values()), len(rows))
        self.assertEqual(counts["error/scoring"], 2)
        self.assertEqual(counts["not_scorable/other"], 2)

    @patch("ml.service.monitoring.psycopg.connect")
    def test_reads_are_bounded_and_parameterized(self, connect: Mock) -> None:
        reader = PostgresReader("test-database", max_connections=1)
        connection = connect.return_value.__enter__.return_value
        cursor = connection.cursor.return_value.__enter__.return_value
        marker = "private/value"
        with reader._cursor(), self.assertRaises(MonitoringUnavailable):
            reader.status(marker)
        self.assertEqual(connect.call_count, 1)
        cursor.fetchone.return_value = ("error/scoring",)
        self.assertEqual(reader.status(marker), {"status": "error", "reason": "scoring"})
        query, params = cursor.execute.call_args.args
        self.assertNotIn(marker, query)
        self.assertEqual(params, (marker,))
        self.assertIn("default_transaction_read_only=on", connect.call_args.kwargs["options"])
        self.assertIn("statement_timeout=", connect.call_args.kwargs["options"])
        self.assertEqual(connection.isolation_level, psycopg.IsolationLevel.REPEATABLE_READ)
        self.assertTrue(0 < connect.call_args.kwargs["connect_timeout"] <= 5)
        cursor.fetchone.return_value = None
        self.assertIsNone(reader.status(marker))
        connect.side_effect = psycopg.OperationalError("private detail")
        with self.assertRaises(MonitoringUnavailable) as failure:
            reader.status(marker)
        self.assertEqual(str(failure.exception), "")
        connect.side_effect = None
        self.assertIsNone(reader.status(marker))

    @patch("ml.service.monitoring.psycopg.connect")
    def test_reader_returns_counts_and_converts_postgres_numeric_lag(self, connect: Mock) -> None:
        cursor = connect.return_value.__enter__.return_value.cursor.return_value.__enter__.return_value
        cursor.fetchall.return_value = [("scored", 2), ("error/invalid_output", 1), ("not_scorable/other", 3)]
        reader = PostgresReader("test-database")
        for lag in (None, Decimal("1.125")):
            cursor.fetchone.return_value = (lag,)
            self.assertEqual(reader.snapshot(), {"scored": 2, "flagged": 0, "pending": 0,
                                                "error": {"invalid_output": 1}, "not_scorable": {"other": 3},
                                                "watermark_lag_seconds": 1.125 if lag is not None else None})
