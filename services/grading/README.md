# grading service

Risk signals and the teacher review workflow of the M4 scenario (project_management.md §3a). `grading` consumes `risk.behavior_anomaly` from `ai.risk` and `quiz.submitted` from `lms.quizzes`, stores each signal next to the attempt it concerns, lets a teacher confirm or dismiss it with a reason, and publishes `risk.reviewed` to `lms.grades`.

**Human in the loop (FR-26).** A risk signal never changes a grade, imposes a penalty, blocks a student or declares misconduct. This service has no grade table and no code path from a signal or a review to a grade. Tests assert this: attempt data is unchanged after a review; no `grade.*` event is produced; the student's quiz result is unchanged in the end-to-end test. Grade entry (FR-17, `grade.assigned`) is a separate, future teacher action.

## Architecture

```
cmd/grading/main.go              composition root (`grading healthcheck` sub-command)
internal/domain                  Risk, Review, AttemptSummary, review state machine, validation
internal/app                     ingestion (consumer) and review use cases on ports
internal/app/apptest             in-memory store emulating the schema constraints
internal/adapters/postgres       pgx store, embedded migrations (schema + triggers)
internal/adapters/http           Gin routes, ETag/If-Match, problem mapping
internal/adapters/schemas        embedded copies of the event JSON Schemas (drift-checked against docs/)
integration/                     Postgres store test and the full M4 end-to-end test (tag `integration`)
```

Ingestion runs through `platform/consumer` (consumer group `grading`, topics `ai.risk` and `lms.quizzes`):
- Every event is applied at most once per `event_id`: a `processed_events` row is written in the same transaction as the effect (NFR-12).
- A signal is also deduplicated by its contract `risk_id`, so a scorer that re-emits the same risk under a new `event_id` creates no second signal.
- Incoming `risk.behavior_anomaly` events are validated against `risk.behavior_anomaly.v1.schema.json` before use. Invalid ones are written to `rejected_events` (the dead-letter log: reason and capped excerpt) and acknowledged, so they never block the partition (FR-19).
- Envelopes that fail parsing, and events whose handler fails permanently, reach the same table through the consumer's `Rejected` hook.
- Other `risk.*` categories are acknowledged and ignored for now.

## API

Bearer token required. Teachers and admins read; **only teachers review**: TS §2 gives admins no academic decision authority, and FR-25 names the teacher. Students and `system` get 403 on every route and can never see a risk signal, their own included (FR-27 release is not built). Errors are RFC 9457 problems.

| Method | Path | Role | Description |
|---|---|---|---|
| GET | `/v1/risks?status=&course_id=&quiz_id=&limit=&cursor=` | teacher, admin | Signals with attempt summary and review. `status` is `open\|confirmed\|dismissed`. Keyset pagination: `limit` ≤ 100; `cursor` is the last `id`, `next_cursor` is returned. No evidence in lists. |
| GET | `/v1/risks/{id}` | teacher, admin | Signal with evidence, explanation, model, attempt summary (student, quiz, score, duration, changes), `processing_ms` (submit → signal received), review, `notice`. Response header `ETag: "v<version>"`. **Every call is audited** (`risk.evidence_viewed`). |
| POST | `/v1/risks/{id}/review` | teacher | Body `{"decision":"confirmed\|dismissed","comment":"1..2000 chars"}`. Headers `If-Match: "v<version>"` (428 if missing, 412 if stale) and `Idempotency-Key` (8–128 of `[A-Za-z0-9_.:-]`). Exactly one review per risk: a second one is 409 `already_reviewed`. Replaying the same key and body returns the stored result with `Idempotent-Replayed: true`; reusing a key with a different body is 422. Writes the review, the status change, an audit record and `risk.reviewed`, all in one transaction. |
| GET | `/healthz`, `/readyz` | — | Liveness; readiness (Postgres, Redis, JWKS). Not routed by the gateway. |

Every risk response carries `notice`: a signal is a relative ranking for teacher review, not a finding of misconduct; absence of a signal is not evidence of integrity.

### `risk.reviewed` (published to `lms.grades`)

```json
{
  "risk_id": "<contract risk_id>",
  "target_type": "attempt",
  "target_id": "<attempt_id>",
  "decision": "confirmed|dismissed",
  "reviewed_by": "<teacher id>",
  "comment": "…"
}
```

- The envelope actor is the reviewing teacher (`role: teacher`).
- Context `course_id` and `group_id` are copied from the signal.
- Partition key is `context.course_id` (a course-level event, events.md §1), or `risk_id` when the course is unknown.
- The payload validates against `docs/03_architecture/schemas/risk.reviewed.v1.schema.json`.

## Schema invariants (`migrations/0001_init.sql`)

- `reviews` has `PRIMARY KEY (risk_pk)`, so there is exactly one review per risk. It is append-only, and `UNIQUE (reviewer_id, idempotency_key)`.
- A trigger on `risks` enforces:
  - signal content is immutable;
  - status changes only once, from `open`;
  - the new status must equal the decision of the risk's review;
  - the version must increase by exactly 1 (optimistic concurrency);
  - risks cannot be deleted.
- `audit_log` is append-only and records actor, role, action, target, timestamp and outcome. Details hold ids and codes only, never comments or evidence (NFR-16).
- CHECKs bound every text, the score range [0, 1], levels and statuses.
- Least privilege: role `grading_svc` owns only the database `grading`.

## Configuration

| Variable | Default | Notes |
|---|---|---|
| `GRADING_HTTP_ADDR` | `:8080` | |
| `GRADING_DATABASE_URL` | — (required, secret) | |
| `GRADING_REDIS_ADDR` / `GRADING_REDIS_PASSWORD` | — | rate limits |
| `GRADING_KAFKA_BROKERS` | — (required) | |
| `GRADING_JWKS_URL` | — (required) | auth JWKS |
| `GRADING_JWT_ISSUER` / `GRADING_JWT_AUDIENCE` | `examory-auth` / `examory` | |
| `GRADING_CONSUMER_GROUP` | `grading` | also the dedup namespace |
| `GRADING_RATE_LIMIT_PER_IP` / `_PER_SUBJECT` | 600 / 300 per minute | |
| `GRADING_TRUSTED_PROXIES`, `GRADING_CORS_ORIGINS`, `GRADING_LOG_LEVEL`, `GRADING_DEV_MODE` | | as in the other services |

## Run and test

```bash
cd services/grading && go test -race ./...     # domain, app (fakes), HTTP, schema drift check
make integration                               # includes TestStoreAgainstPostgres (real DB, triggers, concurrency)
make m4 && make e2e                            # full M4 scenario through the gateway (see root README)
```

## Requirements

| ID | Status |
|---|---|
| FR-24 | **Covered** for behavior signals: category, severity (`level`), score, explanation, evidence, `processing_ms` and review state. No per-teacher case assignment (see limitations). |
| FR-25 | **Covered**: confirm/dismiss with a required reason; reviewer and timestamp recorded; `risk.reviewed` published. |
| FR-26 | **Covered** by design and tests (no grade path). |
| FR-27 | **Not covered**: no student-visible outcomes and no clarification flow. |
| FR-28 | **Partial**: signals are listed with review state, and the notice says that absence of a signal is not proof of integrity. Pending, failed and no-signal states per attempt are not exposed by grading; the scorer records them (`not_scorable`, `scored`) in its own database only. |
| FR-41 | Via `ml/service`: `quiz.answered` and `quiz.submitted` are consumed, and explainable `risk.behavior_anomaly` signals are returned. |
| FR-42 | **Partial**: the evidence shown to teachers is mean answer time, z-score, synchronous peers and the explanation of marginal outliers, with no verdict. Per-question timings are not shown. |
| FR-45 | **Partial**: behavior signals only; no variant-leak signals yet. |
| FR-46 (should) | **Partial**: filters by review state, course and quiz. No filter by severity, group or time window. |
| FR-19 | **Covered** for consumed events: invalid events are dead-letter logged, not applied. |
| NFR-5 | Role checks and negative tests (student and system get 403; admin cannot review). No course-scoped authorization. |
| NFR-12 | **Covered**: dedup by `event_id` and by `risk_id`; re-delivery is tested end to end. |
| NFR-16 | **Covered** for receipt, evidence views, reviews and rejected review attempts. There is no audit-read API. |

## Security model

| Threat | Mitigation |
|---|---|
| Students learning they were flagged, or seeing others' evidence | Route-level role check: students and system get 403; lists carry no evidence |
| Forged or poisoned risk events | Schema validation plus domain checks; invalid events are dead-lettered. Kafka has no authentication or ACLs in dev, so anyone on the broker network can publish (see limitations) |
| Duplicate or replayed events | `processed_events` per `event_id`; `risk_id` unique |
| Double or contradictory reviews | Primary key on the review row, row lock, `If-Match` version, trigger on status, idempotency key per reviewer |
| Tampering with the history | Signals immutable; reviews and audit append-only (triggers) |
| Evidence access without trace | Every detail read is audited |
| Automatic sanctions | No grade code path; the notice is on every response; tests |
| Injection, oversized input | pgx parameters only, strict JSON, 16 KiB review body, 2000-character comment, pagination ≤ 100, UUID path ids, rate limits |

## Dependencies

| Dependency | Why |
|---|---|
| `services/platform` | consumer helper, outbox, migrator, HTTP and JWT verification |
| gin | HTTP framework |
| pgx/v5 | Postgres |
| franz-go | Kafka, via platform |
| go-redis | rate limits |
| santhosh-tekuri/jsonschema/v6 | **runtime** validation of incoming risk events, via `platform/eventschema.LoadFS` |

## Known limitations

- **No teacher↔course scoping:** there is no `courses` service yet, so any teacher can list and review any risk. The reviewer id is recorded in the audit log and in the event actor, which makes this traceable, but it is not prevented.
- No student-facing outcome or clarification flow (FR-27); no `risk.plagiarism` or `risk.variant_leak` handling yet.
- The embedded schema copies must be refreshed with `make sync-schemas` when the docs change. A test fails on drift.
- No metrics for consumer lag (NFR-11). Rejected events can only be inspected in the database.
