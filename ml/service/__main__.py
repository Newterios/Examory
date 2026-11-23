"""Anomaly scoring service: lms.quizzes -> risk.behavior_anomaly on ai.risk.

Run: python -m ml.service   (configuration from environment, see README).
"""
from __future__ import annotations

import json
import logging
import os
import signal
import sys
import threading
import time
from datetime import timedelta
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from .artifact import ArtifactError, expected_sha_from_env, load_verified
from .kafka_io import KafkaPublisher, make_consumer, record_error
from .processor import Processor
from .schema import EventValidator, find_dir
from .scoring import Scorer
from .store import PostgresStore

QUIZ_TOPIC = "lms.quizzes"
_RESERVED = set(logging.LogRecord("", 0, "", 0, "", (), None).__dict__) | {"message", "asctime"}


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        out = {"time": self.formatTime(record, "%Y-%m-%dT%H:%M:%S%z"), "level": record.levelname,
               "msg": record.getMessage(), "service": "anomaly-scorer", "logger": record.name}
        for key, value in record.__dict__.items():
            if key not in _RESERVED and not any(s in key.lower() for s in ("password", "dsn", "token", "secret")):
                out[key] = value
        if record.exc_info:
            out["exc_type"] = record.exc_info[0].__name__ if record.exc_info[0] else None
        return json.dumps(out, default=str)


def env(name: str, default: str | None = None) -> str:
    value = os.environ.get(name, default)
    if value is None or value == "":
        raise SystemExit(f"anomaly-scorer: {name} is required")
    return value


class Health:
    def __init__(self) -> None:
        self.last_loop = time.monotonic()
        self.lock = threading.Lock()

    def beat(self) -> None:
        with self.lock:
            self.last_loop = time.monotonic()

    def alive(self) -> bool:
        with self.lock:
            return time.monotonic() - self.last_loop < 60


def serve_health(health: Health, port: int) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            ok = self.path == "/healthz" and health.alive()
            body = json.dumps({"status": "ok" if ok else "unavailable"}).encode()
            self.send_response(200 if ok else 503)
            self.send_header("Content-Type", "application/json")
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args: object) -> None:
            return

    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


def main() -> int:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JsonFormatter())
    logging.basicConfig(level=os.environ.get("ANOMALY_LOG_LEVEL", "INFO").upper(), handlers=[handler])
    log = logging.getLogger("anomaly")
    try:
        artifact = load_verified(env("ANOMALY_MODEL_PATH"), expected_sha_from_env())
    except ArtifactError as exc:
        log.error("refusing to start: %s", exc)
        return 2
    if not expected_sha_from_env():
        log.warning("ANOMALY_MODEL_SHA256 not set: the pickle artifact is loaded without an integrity pin")
    schema_dir = Path(os.environ.get("ANOMALY_SCHEMA_DIR", "")) if os.environ.get("ANOMALY_SCHEMA_DIR") else find_dir()
    validator = EventValidator(schema_dir)
    store = PostgresStore(env("ANOMALY_DATABASE_URL"))
    store.migrate()
    lateness = timedelta(seconds=float(os.environ.get("ANOMALY_LATENESS_SECONDS", "5")))
    processor = Processor(store, Scorer(artifact), validator, lateness)
    brokers = env("ANOMALY_KAFKA_BROKERS")
    consumer = make_consumer(brokers, os.environ.get("ANOMALY_CONSUMER_GROUP", "anomaly-scorer"), QUIZ_TOPIC)
    publisher = KafkaPublisher(brokers)
    health = Health()
    server = serve_health(health, int(os.environ.get("ANOMALY_HEALTH_PORT", "8081")))
    stopping = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stopping.set())
    log.info("anomaly scorer started", extra={"model_version": artifact["version"],
                                              "medium_threshold": artifact["models"]["iforest"]["medium_threshold"],
                                              "high_threshold": artifact["models"]["iforest"]["high_threshold"]})
    last_tick = 0.0
    try:
        while not stopping.is_set():
            msg = consumer.poll(0.5)
            if msg is not None:
                err = record_error(msg)
                if err is not None:
                    log.warning("kafka error", extra={"error": str(err)})
                else:
                    processor.handle(msg.value() or b"")
                    consumer.store_offsets(message=msg)  # after the DB commit: at-least-once
            if time.monotonic() - last_tick >= 1.0:
                processor.score_ready()
                processor.publish(publisher)
                last_tick = time.monotonic()
                health.beat()
    finally:
        log.info("anomaly scorer stopping", extra={"stats": processor.stats.__dict__})
        consumer.close()
        publisher.close()
        store.close()
        server.shutdown()
    return 0


if __name__ == "__main__":
    sys.exit(main())
