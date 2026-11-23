"""Metrics at ingress, scoring and acknowledged outbox publication."""
from __future__ import annotations

import json
import unittest
from datetime import timedelta
from unittest.mock import Mock, call, patch

from ml.service.kafka_io import KafkaPublisher
from ml.service.metrics import Metrics
from ml.service.processor import Processor
from ml.service.scoring import Outcome
from ml.service.store import OutboxRow, Pending
from ml.service.tests.test_service import T0, FakeStore


class ProcessingMetricsTests(unittest.TestCase):
    def setUp(self) -> None:
        self.store, self.scorer, self.validator = Mock(), Mock(), Mock()
        self.metrics = Mock(spec=Metrics)
        self.proc = Processor(self.store, self.scorer, self.validator, timedelta(seconds=5), self.metrics)
        self.validator.errors.return_value = []

    def test_all_ingest_outcomes_and_private_fields(self) -> None:
        personal = "private-student-id-and-answer"
        envelope = {"event_type": "quiz.started", "event_id": personal, "occurred_at": T0.isoformat()}
        record = json.dumps(envelope).encode()
        self.store.add_event.side_effect = [True, False]
        self.assertEqual(self.proc.handle(record), "stored")
        self.assertEqual(self.proc.handle(record), "duplicate")
        self.assertEqual(self.proc.handle(b'{"event_type":"other"}'), "ignored")
        with self.assertLogs("anomaly.processor", level="WARNING") as captured:
            for bad in (dict(envelope, event_type=[personal]), dict(envelope, occurred_at=[personal]),
                        dict(envelope, occurred_at="2026-01-01"), dict(envelope, occurred_at=personal)):
                self.assertEqual(self.proc.handle(json.dumps(bad).encode()), "rejected")
            self.validator.errors.return_value = [personal]
            self.assertEqual(self.proc.handle(record), "rejected")
        self.assertNotIn(personal, str([r.__dict__ for r in captured.records]))
        self.assertEqual(self.metrics.record_event.call_args_list,
                         [call("stored"), call("duplicate"), call("ignored"), *[call("rejected")] * 5])

    def test_failed_attempt_observed_once_with_bounded_status_reason(self) -> None:
        personal = "private-attempt-answer"
        self.store.watermark.return_value = T0 + timedelta(seconds=10)
        self.store.now.return_value = T0 + timedelta(seconds=10)
        self.store.pending.return_value = [Pending(personal, "g", "q", T0, T0)]
        self.scorer.score.side_effect = ValueError(personal)
        with self.assertLogs("anomaly.processor") as captured:
            self.assertEqual(self.proc.score_ready(), 1)
        self.assertNotIn(personal, str([r.__dict__ for r in captured.records]))
        self.assertEqual(self.metrics.observe_scoring.call_count, 1)
        self.assertGreaterEqual(self.metrics.observe_scoring.call_args.args[0], 0)
        self.assertEqual(self.store.finish.call_args.args[4], "error:scoring")
        self.scorer.score.side_effect = None
        self.scorer.score.return_value = Outcome("flagged", event={"event_id": personal})
        self.validator.errors.return_value = [personal]
        self.proc.score_ready()
        self.assertEqual(self.store.finish.call_args.args[4], "error:invalid_output")
        self.assertIsNone(self.store.finish.call_args.args[5])
        self.assertEqual(self.metrics.observe_scoring.call_count, 2)
        self.store.watermark.return_value = self.store.now.return_value = T0
        self.assertEqual(self.proc.score_ready(), 0)
        self.assertEqual(self.metrics.observe_scoring.call_count, 2)

    def test_partial_and_repeated_acknowledgements_record_only_new_delays(self) -> None:
        store = FakeStore()
        for i in ("a", "b"):
            row = OutboxRow(i, "ai.risk", i, "{}")
            store.outbox[i] = {"row": row, "published": False}
            store.attempts[i] = {"submitted_at": T0}
        proc = Processor(store, self.scorer, self.validator, timedelta(seconds=1), self.metrics)
        publisher = Mock()
        publisher.publish.side_effect = [
            {"a": T0 + timedelta(seconds=3)}, {},
            {"a": T0 + timedelta(seconds=7), "b": T0 + timedelta(seconds=8)}]
        self.assertEqual([proc.publish(publisher) for _ in range(4)], [1, 0, 1, 0])
        self.assertEqual(self.metrics.observe_publication.call_args_list, [call(3.), call(8.)])
        self.assertEqual(publisher.publish.call_count, 3)

    def test_late_delivery_report_survives_a_timed_out_flush(self) -> None:
        row = OutboxRow("sensitive_risk_id", "ai.risk", "sensitive_attempt", "{}")
        with patch("ml.service.kafka_io.Producer") as constructor, patch("ml.service.kafka_io.datetime") as clock:
            client = constructor.return_value
            sender = KafkaPublisher("broker")
            self.assertEqual(sender.publish([row]), {})
            first_callback = client.produce.call_args.kwargs["on_delivery"]
            clock.now.side_effect = [T0, T0 + timedelta(seconds=3)]
            def next_flush(_: float) -> int:
                first_callback(None, None)
                client.produce.call_args.kwargs["on_delivery"](None, None)
                return 0
            client.flush.side_effect = next_flush
            self.assertEqual(sender.publish([row]), {row.risk_id: T0})
