"""Regression checks for replay-scoped wall-clock and cgroup measurements."""
import csv
import tempfile
import unittest
from datetime import UTC, datetime
from pathlib import Path
from unittest.mock import patch

from ml.replay import load_report as report


def stamp(seconds):
    return datetime.fromtimestamp(seconds, tz=UTC).isoformat()


class LoadReportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.samples = Path(self.temp.name) / "samples.csv"
        with self.samples.open("w", newline="") as stream:
            writer = csv.writer(stream)
            writer.writerow(["wall_epoch_s", "monotonic_s", "cpu_usage_usec", "memory_bytes"])
            writer.writerows([(t, t + 5, cpu, mem * 2**20) for t, cpu, mem in
                              [(0, 0, 4), (2, 1000000, 8), (4, 4000000, 12)]])

    def test_clips_resource_intervals_and_weights_by_elapsed_time(self):
        result = report.resource_summary(self.samples, 1, 3)
        self.assertEqual(result["mean_cpu_percent_one_core"], 100)
        self.assertEqual(result["peak_sampling_interval_cpu_percent_one_core"], 150)
        self.assertEqual(result["mean_memory_mib"], 8)
        self.assertEqual(result["sampled_peak_memory_mib"], 10)
        with self.assertRaises(ValueError):
            report.resource_summary(self.samples, 1, 5)

    def test_reports_delayed_ack_and_verifies_durable_event_counts(self):
        kinds = ["quiz.started", "quiz.answered", "quiz.submitted", "quiz.started"]
        manifest = {
            "mode": "load", "rate": 2, "duration_ms": 2000, "window_start": stamp(1), "window_end": stamp(3),
            "actions": [{"kind": k, "http_status": 200, "sent_at": stamp(1 + i / 2),
                         "scheduled_at": stamp(1 + i / 2)} for i, k in enumerate(kinds)],
            "attempts": {"one": {"id": "a", "status": "submitted"}, "two": {"id": "b", "status": "partial"}},
        }
        counts = {"quiz.started": 2, "quiz.answered": 1, "quiz.submitted": 1}
        with (patch.object(report, "queue_times", return_value={"a": 2.1}),
              patch.object(report, "scoring_state", return_value={"a": ("flagged", 3.2, 3.5)}),
              patch.object(report, "event_counts", return_value=counts) as counted):
            result = report.measure(manifest, self.samples, "pg", 0)
            self.assertTrue(result["complete"])
            self.assertAlmostEqual(result["submit_to_signal_ms"]["p95"], 1400)
            self.assertEqual(result["submitted_not_yet_scored_at_window_end"], 1)
            self.assertEqual(result["successful_event_dispatches_per_60s"], [4])
            self.assertIn("'b'", counted.call_args.args[1])
            counted.side_effect = [counts, {"quiz.started": 2}]
            self.assertFalse(report.measure(manifest, self.samples, "pg", 0)["complete"])

    def test_empty_submissions_are_reported_without_inventing_latency(self):
        m = {"mode": "load", "rate": 1, "duration_ms": 1000, "actions": [], "attempts": {},
             "window_start": stamp(1), "window_end": stamp(2)}
        result = report.measure(m, self.samples, "pg", 0)
        self.assertFalse(result["complete"])
        self.assertIsNone(result["submit_to_signal_ms"]["p99"])
        self.assertEqual(result["published_signals"], 0)

    def test_quantiles_and_identifier_validation(self):
        self.assertEqual(report.percentiles([200, 0]), {"p50": 100., "p95": 190., "p99": 198.})
        with self.assertRaises(ValueError):
            report.sql_ids(["a'b"])


if __name__ == "__main__":
    unittest.main()
