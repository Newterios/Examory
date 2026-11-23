# ml/service — anomaly scoring service (M4)

This service consumes `lms.quizzes` and stores the raw quiz events once each, in its own Postgres database `anomaly`. When an attempt is submitted and its peers can be complete, it scores the attempt with the **frozen** behavior baseline in `ml/anomaly/`, which is reused unchanged. If the score reaches the model's medium threshold, it publishes `risk.behavior_anomaly` to `ai.risk` (FR-41).

> **Security warning — the model artifact is a Python pickle.** Loading a pickle runs whatever code the file's author put in it. Only load an artifact you built yourself with `make anomaly-model` on a trusted machine. Never load one downloaded or received from someone else, and never let the model directory be writable by anyone else.
>
> The service reduces, but cannot remove, this risk:
> - it refuses a missing, non-regular or world-writable file;
> - with `ANOMALY_MODEL_SHA256` set (`make anomaly-model` writes it to `.env`), it checks the SHA-256 **before** unpickling;
> - it then checks `schema_version == 1`, the model structure and the thresholds;
> - it checks that the numpy, pandas and scikit-learn versions match the runtime;
> - the file is mounted read-only into a read-only, non-root container.

## Flow

1. **Ingest.** A record on `lms.quizzes` is parsed and validated against `docs/03_architecture/schemas` (envelope plus `quiz.*` v1). Invalid records are logged (event id, type and reason, never the payload) and acknowledged. Valid ones are inserted into `quiz_events` with `ON CONFLICT (event_id) DO NOTHING`. A `quiz.submitted` also creates a `pending` row in `attempts`. The Kafka offset is stored only after the DB commit (at-least-once delivery, idempotent storage).
2. **Watermark** (`watermark.py`). Peer features compare an attempt with peers whose submissions lie within ±5 s of event time, so an attempt becomes ready when either condition holds:
   - the event-time watermark (latest stored `occurred_at`) reaches `submitted_at + 5 s`;
   - or, for quiet streams and lagging partitions, `ANOMALY_LATENESS_SECONDS` (default 5) of processing time have passed after `submitted_at + 5 s` was received.

   Limitation: this is a global watermark, not a per-partition one. A peer submission delayed by more than the lateness bound is missed.
3. **Score** (`scoring.py`).
   - The cohort is every stored attempt with the same group and quiz, which also covers the student's earlier attempts (score-delta history).
   - Events are written in the generator CSV layout and passed to `features.read_attempts` as a file-like object, then to `build_features` with the artifact's reference and to `train.score_model` with the `iforest` model.
   - Before scoring, each attempt is checked on its own. Attempts the frozen reader rejects are excluded from the cohort; if the target is one of them, it is recorded as `not_scorable` with the reason.
4. **Emit.**
   - A raw score ≥ the medium threshold yields `to_risk_event.risk_event(...)`. The service adds per-question timing observations to its evidence. The artifact thresholds determine severity, and the frozen model supplies the explanation.
   - `risk_id` is replaced by a deterministic UUIDv5 of model name, artifact version and attempt id, so re-processing can never create a second signal.
   - The event is validated against `risk.behavior_anomaly.v1.schema.json` and stored in `risk_outbox` in the same transaction that marks the attempt `flagged`. Below the threshold the attempt is marked `scored` (level `low`) and no event is published (events.md: absence of a risk is not evidence of integrity).
5. **Publish.** Outbox rows go to `ai.risk` through an idempotent producer (acks=all), keyed by the target attempt id, and are marked once the broker acknowledges them. On a crash the same bytes, with the same `event_id`, are re-sent; grading deduplicates by `event_id` and `risk_id`.

### Input contract of the frozen pipeline v1

`read_attempts` was written for the synthetic generator. Live attempts are scorable only if all of the following hold:
- every question is answered;
- each question is worth exactly 1 point, so `max_score` equals the number of questions;
- the payload keys are exactly the v1 sets. The additive `quiz.submitted.reason` field is dropped before scoring.

Attempts that break these rules are recorded as `not_scorable` and never crash the batch. Event timestamps must agree with `duration_ms` to the millisecond; the quizzes service guarantees this by truncating its clock to milliseconds.

### Per-question observations for teachers

The scorer adds one row to `payload.evidence.features.per_question` for each question in a flagged attempt. Rows use the student's zero-based variant order. Each row reports the first saved answer's time since display, the numeric cutoff used by `fast_answer_fraction`, whether the first answer was strictly faster than that cutoff, and whether the student later changed the answer.

`make anomaly-model` runs the frozen trainer once. The service packaging step uses the same verified CSV and normal training groups to compute medians of first-answer times. It stores the medians in the SHA-pinned model alongside the existing tenth-percentile cutoffs. A quiz, variant and question with at least 30 normal training observations gets an item median. All other questions use the pooled training median, including live question IDs absent from training. Existing artifacts still supply the original cutoff. For those artifacts, the row reports `reference_median_ms: null` with `reference_scope: "unavailable"`.

The optional v1 field and its units are documented in [the event contract](../../docs/03_architecture/events.md#riskbehavior_anomaly). Refresh the consumer's embedded copy with `make sync-schemas` after changing the canonical schema.

## Configuration

| Variable | Default | Notes |
|---|---|---|
| `ANOMALY_MODEL_PATH` | — (required) | the service refuses to start without a valid artifact |
| `ANOMALY_MODEL_SHA256` | empty | strongly recommended; checked before unpickling. A warning is logged when it is unset. |
| `ANOMALY_DATABASE_URL` | — (required, secret) | role `anomaly_svc`, database `anomaly` (tables are created at start under an advisory lock) |
| `ANOMALY_KAFKA_BROKERS` | — (required) | |
| `ANOMALY_CONSUMER_GROUP` | `anomaly-scorer` | |
| `ANOMALY_LATENESS_SECONDS` | `5` | processing-time bound of the watermark fallback |
| `ANOMALY_SCHEMA_DIR` | found from the repository | `/app/schemas` in the image |
| `ANOMALY_HEALTH_PORT` | `8081` | `GET /healthz` returns 200 while the main loop is alive (not published to the host) |
| `ANOMALY_LOG_LEVEL` | `INFO` | JSON logs; payloads, answers and secrets are never logged |

## Run and test

```bash
make anomaly-model      # builds the image, then generates data (generator, 2000 students, seed 42),
                        # splits and trains INSIDE the image (same library versions as runtime) into
                        # ml/service/model/ (git-ignored), and pins the SHA-256 in .env
docker compose up -d --build anomaly-scorer
python -m pip install -r ml/service/requirements.txt && make py-test     # unit tests (Python 3.12+)
python -m ruff check ml analytics
```

The unit tests (`ml/service/tests/test_service.py`, run with `python -m unittest`) train a small throwaway artifact (120 generator students) and use fakes:
- the watermark rule;
- idempotent ingestion and rejection of invalid input without crashing;
- scoring every generator cohort, with every produced event validated against the JSON Schema;
- the deterministic `risk_id` and no duplicate signal on re-processing;
- the severity mapping;
- publish/ack;
- live platform-shaped events (UUIDs, the `reason` field): two crafted fast, identical, synchronous attempts are flagged;
- `not_scorable` inputs;
- artifact verification: SHA pin, world-writable file, schema version, library mismatch, missing model.

## Dependencies (pinned in `requirements.txt`)

- numpy, pandas, scikit-learn, datasketch: identical to `ml/requirements.txt` (the frozen pipeline and the pickle).
- `confluent-kafka` 2.16.1: maintained librdkafka client with an idempotent producer.
- `psycopg[binary]` 3.3.6.
- `jsonschema` 4.26.0: event validation.

`pip-audit -r ml/service/requirements.txt`: no known vulnerabilities (November 2026). Hashes are not pinned (no `--require-hashes`).

## Known limitations

- A global watermark plus a lateness bound, not a per-partition watermark. Attempts are scored once; late peers do not trigger re-scoring.
- Only 1-point, fully answered quizzes are scorable (frozen pipeline v1).
- The demo model is trained on synthetic generator data. Item timing references do not exist for live quiz ids, so the global fast-answer threshold is used.
- The score is a relative ranking, not a probability (events.md §4). The model has not been validated on real students.
- Single-threaded loop with one DB connection; the cohort is re-read and re-parsed for every attempt (quadratic in cohort size; fine for class-sized cohorts).
- No metrics. The `attempts.status` and `detail` columns are the only record of pending, not-scorable and no-signal outcomes, and they are not exposed through an API (FR-28 partial).
