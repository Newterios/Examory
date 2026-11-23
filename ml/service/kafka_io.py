"""Kafka adapters (confluent-kafka / librdkafka)."""
from __future__ import annotations

import logging
from datetime import UTC, datetime
from typing import Any

from confluent_kafka import Consumer, KafkaError, Message, Producer

from .store import OutboxRow

log = logging.getLogger("anomaly.kafka")


def make_consumer(brokers: str, group: str, topic: str) -> Consumer:
    consumer = Consumer({
        "bootstrap.servers": brokers,
        "group.id": group,
        "client.id": "anomaly-scorer",
        "auto.offset.reset": "earliest",
        # At-least-once: offsets are stored only after the DB commit of the
        # event (store_offsets) and committed periodically by librdkafka.
        "enable.auto.commit": True,
        "enable.auto.offset.store": False,
        "auto.commit.interval.ms": 1000,
        "session.timeout.ms": 15000,
        "max.poll.interval.ms": 300000,
        "socket.timeout.ms": 10000,
    })
    consumer.subscribe([topic])
    return consumer


def record_error(msg: Message) -> KafkaError | None:
    err = msg.error()
    if err is None:
        return None
    if err.code() == KafkaError._PARTITION_EOF:
        return None
    return err


class KafkaPublisher:
    """Idempotent producer (acks=all); waits for delivery reports."""

    def __init__(self, brokers: str, timeout_s: float = 10.0) -> None:
        self._producer = Producer({
            "bootstrap.servers": brokers,
            "client.id": "anomaly-scorer",
            "enable.idempotence": True,
            "acks": "all",
            "message.timeout.ms": int(timeout_s * 1000),
            "socket.timeout.ms": 10000,
        })
        self._timeout = timeout_s
        self._acknowledged: dict[str, datetime] = {}

    def publish(self, rows: list[OutboxRow]) -> dict[str, datetime]:
        def report(err: Any, _msg: Message, risk_id: str) -> None:
            if err is None:
                # A callback from a previous timed-out flush can arrive while
                # polling this batch. Keep the earliest successful report.
                self._acknowledged.setdefault(risk_id, datetime.now(UTC))
            else:
                log.warning("risk event delivery failed")

        for row in rows:
            self._producer.produce(row.topic, value=row.envelope.encode("utf-8"), key=row.key.encode("utf-8"),
                                   headers={"event_type": b"risk.behavior_anomaly"},
                                   on_delivery=lambda err, msg, rid=row.risk_id: report(err, msg, rid))
        self._producer.flush(self._timeout + 2)
        acked, self._acknowledged = self._acknowledged, {}
        return acked

    def close(self) -> None:
        self._producer.flush(5)
