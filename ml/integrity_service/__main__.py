"""Consume submissions on a private Kafka network and publish verified literal matches."""

import json
import logging
import os
import signal
from pathlib import Path
from urllib.parse import urlsplit

import psycopg
import urllib3
from confluent_kafka import Consumer, Producer
from minio import Minio
from minio.error import MinioException

from ml.integrity_service.store import ObjectUnavailable, Store, UnsupportedText
from ml.service.schema import EventValidator

log = logging.getLogger(__name__)


class TextObjects:
    def __init__(self):
        self.client = Minio(
            os.environ["MINIO_ENDPOINT"],
            os.environ["MINIO_ACCESS_KEY"],
            os.environ["MINIO_SECRET_KEY"],
            secure=os.getenv("MINIO_TLS", "true") == "true",
            http_client=urllib3.PoolManager(
                timeout=urllib3.Timeout(connect=3, read=8), retries=False
            ),
        )

    def read(self, event: dict) -> str:
        key = urlsplit(event["payload"]["text_ref"]).path.lstrip("/")
        try:
            obj = self.client.get_object("submissions", key)
            try:
                content = obj.read(65537)
            finally:
                obj.close()
                obj.release_conn()
        except (MinioException, urllib3.exceptions.HTTPError, OSError) as exc:
            raise ObjectUnavailable from exc
        if len(content) > 65536:
            raise UnsupportedText("64 KiB input limit")
        try:
            return content.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise UnsupportedText("Expected UTF-8") from exc


def main() -> None:
    brokers = os.environ["KAFKA_BROKERS"]
    consumer = Consumer(
        {
            "bootstrap.servers": brokers,
            "group.id": os.getenv("INTEGRITY_GROUP", "text-integrity-v1"),
            "auto.offset.reset": "earliest",
            "enable.auto.commit": False,
        }
    )
    producer = Producer(
        {
            "bootstrap.servers": brokers,
            "enable.idempotence": True,
            "message.timeout.ms": 10000,
        }
    )
    validator = EventValidator(Path(os.getenv("SCHEMA_DIR", "/app/schemas")))
    objects = TextObjects()
    templates_path = os.getenv("INTEGRITY_TEMPLATES")
    templates = (
        json.loads(Path(templates_path).read_text(encoding="utf-8"))
        if templates_path
        else {}
    )
    if not isinstance(templates, dict) or not all(
        isinstance(v, str) for v in templates.values()
    ):
        raise ValueError("Expected assignment IDs mapped to template text")
    stopping = False

    def stop(*_):
        nonlocal stopping
        stopping = True

    def validate(event: dict):
        if validator.errors(event):
            raise ValueError("Invalid event schema")

    def send(target: str, event: dict):
        status = []
        producer.produce(
            "ai.risk",
            key=target.encode(),
            value=json.dumps(event).encode(),
            headers={"event_type": b"risk.plagiarism"},
            on_delivery=lambda err, _: status.append(err),
        )
        if producer.flush(12) or status != [None]:
            raise RuntimeError("Risk publication was not acknowledged")

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        with psycopg.connect(
            os.environ["INTEGRITY_DATABASE_URL"], autocommit=True
        ) as db:
            store = Store(db, os.getenv("INTEGRITY_SCOPE", "assignment"), templates)
            consumer.subscribe(["lms.submissions"])
            while not stopping:
                msg = consumer.poll(0.2)
                if msg is not None and not msg.error():
                    try:
                        event = json.loads(msg.value())
                        if not isinstance(event, dict):
                            raise TypeError("Expected object")
                        if event.get("event_type") == "submission.created":
                            validate(event)
                            store.accept(event)
                    except (UnicodeError, ValueError, TypeError):
                        log.warning(
                            "Rejected malformed submission at partition %d offset %d",
                            msg.partition(),
                            msg.offset(),
                        )
                    consumer.commit(message=msg, asynchronous=False)
                store.work(objects.read, validate)
                store.publish_one(send)
    finally:
        consumer.close()
        producer.flush(5)


if __name__ == "__main__":
    main()
