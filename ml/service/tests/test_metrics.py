"""Check the scrape contract without external infrastructure or a metrics library."""
from __future__ import annotations

import unittest
from concurrent.futures import ThreadPoolExecutor

from ml.service.metrics import Metrics, Snapshot


def snapshot() -> Snapshot:
    return {
        "pending": 2, "scored": 4, "flagged": 1, "watermark_lag_seconds": None,
        "not_scorable": {"unsupported_input": 3}, "error": {"scoring": 1, "invalid_output": 2},
    }


def samples(text: str) -> dict[str, float]:
    return {key: float(value) for line in text.splitlines() if not line.startswith("#")
            for key, value in [line.rsplit(" ", 1)]}


class MetricsTests(unittest.TestCase):
    def test_histograms_are_cumulative_and_include_the_boundary(self) -> None:
        metrics = Metrics()
        for seconds in (0, 2, 125):
            metrics.observe_scoring(seconds)
        metrics.observe_publication(4.25)
        text = metrics.render(snapshot())
        values = samples(text)
        prefix = "examory_anomaly_scoring_duration_seconds"
        self.assertIn(f"# TYPE {prefix} histogram\n", text)
        self.assertEqual(values[prefix + '_bucket{le="2"}'], 2)
        self.assertEqual(values[prefix + '_bucket{le="60"}'], 2)
        self.assertEqual(values[prefix + '_bucket{le="+Inf"}'], 3)
        self.assertEqual(values[prefix + "_count"], 3)
        self.assertEqual(values[prefix + "_sum"], 127)
        buckets = [v for k, v in values.items() if k.startswith(prefix + "_bucket")]
        self.assertEqual(buckets, sorted(buckets))
        self.assertEqual(values["examory_anomaly_submission_to_publication_seconds_sum"], 4.25)
        self.assertTrue(text.endswith("\n"))

    def test_db_snapshot_gauges_and_process_reset(self) -> None:
        metrics = Metrics()
        metrics.record_event("stored")
        metrics.record_event("rejected")
        state = snapshot()
        text = metrics.render(state)
        self.assertIn("examory_anomaly_watermark_lag_seconds NaN\n", text)
        values = samples(text)
        self.assertEqual(values['examory_anomaly_attempts{status="scored"}'], 4)
        self.assertEqual(values['examory_anomaly_attempts{status="not_scorable"}'], 3)
        self.assertEqual(values['examory_anomaly_attempts{status="error"}'], 3)
        self.assertEqual(values['examory_anomaly_attempts_by_reason{status="error",reason="scoring"}'], 1)
        key = 'examory_anomaly_events_total{outcome="rejected"}'
        self.assertEqual(values[key], 1)
        state["watermark_lag_seconds"] = 2.25
        after_restart = samples(Metrics().render(state))
        self.assertEqual(after_restart[key], 0)
        self.assertEqual(after_restart['examory_anomaly_attempts{status="flagged"}'], 1)
        self.assertEqual(after_restart["examory_anomaly_watermark_lag_seconds"], 2.25)

    def test_concurrent_observations_and_scrapes(self) -> None:
        metrics = Metrics()
        def update(_: int) -> None:
            for _ in range(40):
                metrics.record_event("duplicate")
                metrics.observe_publication(.25)
                values = samples(metrics.render(snapshot()))
                prefix = "examory_anomaly_submission_to_publication_seconds"
                self.assertEqual(values[prefix + '_bucket{le="+Inf"}'], values[prefix + "_count"])
        with ThreadPoolExecutor(max_workers=4) as pool:
            list(pool.map(update, range(8)))
        values = samples(metrics.render(snapshot()))
        self.assertEqual(values['examory_anomaly_events_total{outcome="duplicate"}'], 320)
        self.assertEqual(values["examory_anomaly_submission_to_publication_seconds_sum"], 80)
        for value in (-1.0, float("nan"), float("inf")):
            with self.assertRaises(ValueError):
                metrics.observe_scoring(value)
