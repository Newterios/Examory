"""Durable inbox, LSH buckets and transactional risk outbox."""

from __future__ import annotations

import hashlib
import json
from collections.abc import Callable

import psycopg
from psycopg.types.json import Jsonb

from ml.integrity_service.engine import MIN_SPAN, band_keys, compare, prepare

DDL = """
CREATE TABLE IF NOT EXISTS integrity_config (singleton boolean PRIMARY KEY CHECK(singleton), digest text NOT NULL);
CREATE TABLE IF NOT EXISTS integrity_docs (
    id text PRIMARY KEY, event_id uuid UNIQUE NOT NULL, scope text NOT NULL, author_id text NOT NULL,
    event jsonb NOT NULL, state text NOT NULL DEFAULT 'pending', retry_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(), content_sha text);
CREATE INDEX IF NOT EXISTS integrity_pending ON integrity_docs(retry_at,created_at) WHERE state='pending';
CREATE TABLE IF NOT EXISTS integrity_bands (
    scope text NOT NULL, band bytea NOT NULL, doc_id text REFERENCES integrity_docs, PRIMARY KEY(scope,band,doc_id));
CREATE TABLE IF NOT EXISTS integrity_outbox (
    risk_id uuid PRIMARY KEY, target_id text NOT NULL, event jsonb NOT NULL, published boolean NOT NULL DEFAULT false);
"""


class ObjectUnavailable(Exception):
    """Temporary failure to read an immutable object."""


class UnsupportedText(Exception):
    """The object exceeds the accepted size or cannot be decoded."""


class Store:
    def __init__(self, db: psycopg.Connection, scope: str, templates: dict[str, str]):
        if scope not in {"assignment", "course"}:
            raise ValueError("Scope must be assignment or course")
        self.db, self.scope, self.templates = db, scope, templates
        digest = hashlib.sha256(
            json.dumps([1, scope, templates], sort_keys=True).encode()
        ).hexdigest()
        with db.transaction():
            db.execute(DDL)
            db.execute(
                "INSERT INTO integrity_config VALUES(true,%s) ON CONFLICT DO NOTHING",
                (digest,),
            )
            if (
                db.execute(
                    "SELECT digest FROM integrity_config WHERE singleton"
                ).fetchone()[0]
                != digest
            ):
                raise ValueError(
                    "A different template set or scope requires a fresh index and inbox replay"
                )

    def accept(self, event: dict) -> None:
        p = event["payload"]
        course = event["context"]["course_id"]
        if not course:
            raise ValueError("A course is required for indexing")
        scope = json.dumps(
            [course, p["assignment_id"]] if self.scope == "assignment" else [course]
        )
        ref = f"minio://submissions/{p['submission_id']}/extracted.txt"
        if p["text_ref"] not in (None, ref):
            raise ValueError("Unexpected object key")
        state = (
            "pending"
            if p["kind"] in {"text", "file"} and p["text_ref"]
            else "unsupported"
        )
        with self.db.transaction():
            self.db.execute(
                """INSERT INTO integrity_docs(id,event_id,scope,author_id,event,state)
                VALUES(%s,%s,%s,%s,%s,%s) ON CONFLICT DO NOTHING""",
                (
                    p["submission_id"],
                    event["event_id"],
                    scope,
                    event["actor"]["user_id"],
                    Jsonb(event),
                    state,
                ),
            )

    def work(
        self, read: Callable[[dict], str], validate: Callable[[dict], None]
    ) -> bool:
        with self.db.transaction():
            row = self.db.execute("""SELECT id,scope,event FROM integrity_docs WHERE state='pending'
                AND retry_at<=now() ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED""").fetchone()
            if not row:
                return False
            key, scope, event = row
            self.db.execute(
                "SELECT pg_advisory_xact_lock(hashtextextended(%s,0))", (scope,)
            )
            try:
                text = read(event)
                doc = prepare(event, text, self.templates)
                buckets = band_keys(doc)
                if len(doc.tokens) < MIN_SPAN or not buckets:
                    self.db.execute(
                        "UPDATE integrity_docs SET state='abstained' WHERE id=%s",
                        (key,),
                    )
                    return True
                candidates = self.db.execute(
                    """SELECT DISTINCT d.id,d.event,d.content_sha
                    FROM integrity_bands b JOIN integrity_docs d ON d.id=b.doc_id
                    WHERE b.scope=%s AND b.band=ANY(%s::bytea[]) AND d.author_id<>%s""",
                    (scope, buckets, event["actor"]["user_id"]),
                ).fetchall()
                signals = []
                for _, previous, digest in candidates:
                    try:
                        source = read(previous)
                    except UnsupportedText as exc:
                        raise ObjectUnavailable(
                            "Restore the previously indexed immutable object"
                        ) from exc
                    if hashlib.sha256(source.encode()).hexdigest() != digest:
                        raise ObjectUnavailable("Indexed content has changed")
                    result = compare((event, text), (previous, source), self.templates)
                    if result:
                        validate(result)
                        signals.append(result)
            except ObjectUnavailable:
                self.db.execute(
                    "UPDATE integrity_docs SET retry_at=now()+interval '5 seconds' WHERE id=%s",
                    (key,),
                )
                return True
            except UnsupportedText:
                self.db.execute(
                    "UPDATE integrity_docs SET state='unsupported' WHERE id=%s", (key,)
                )
                return True
            for result in signals:
                payload = result["payload"]
                self.db.execute(
                    "INSERT INTO integrity_outbox(risk_id,target_id,event) VALUES(%s,%s,%s) ON CONFLICT DO NOTHING",
                    (payload["risk_id"], payload["target_id"], Jsonb(result)),
                )
            with self.db.cursor() as cursor:
                cursor.executemany(
                    "INSERT INTO integrity_bands VALUES(%s,%s,%s) ON CONFLICT DO NOTHING",
                    [(scope, band, key) for band in buckets],
                )
            self.db.execute(
                "UPDATE integrity_docs SET state='indexed',content_sha=%s WHERE id=%s",
                (hashlib.sha256(text.encode()).hexdigest(), key),
            )
            return True

    def publish_one(self, send: Callable[[str, dict], None]) -> bool:
        with self.db.transaction():
            row = self.db.execute("""SELECT risk_id,target_id,event FROM integrity_outbox WHERE NOT published
                                      ORDER BY risk_id LIMIT 1 FOR UPDATE SKIP LOCKED""").fetchone()
            if not row:
                return False
            send(row[1], row[2])
            self.db.execute(
                "UPDATE integrity_outbox SET published=true WHERE risk_id=%s", (row[0],)
            )
            return True
