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

1. **Ingest.** A record on `lms.quizzes` is parsed and validated against `docs/03_architecture/schemas` (envelope plus `quiz.*` v1). For invalid records the scorer logs a fixed rejection reason and acknowledges the record. Valid records are inserted into `quiz_events` with `ON CONFLICT (event_id) DO NOTHING`. A `quiz.submitted` also creates a `pending` row in `attempts`. The Kafka offset is stored only after the DB commit (at-least-once delivery, idempotent storage).
2. **Watermark** (`watermark.py`). Peer features compare an attempt with peers whose submissions lie within ±5 s of event time, so an attempt becomes ready when either condition holds:
   - the event-time watermark (latest stored `occurred_at`) reaches `submitted_at + 5 s`;
   - or, for quiet streams and lagging partitions, `ANOMALY_LATENESS_SECONDS` (default 5) of processing time have passed after `submitted_at + 5 s` was received.

   Limitation: this is a global watermark, not a per-partition one. A peer submission delayed by more than the lateness bound is missed.
3. **Score** (`scoring.py`).
   - The cohort is every stored attempt with the same group and quiz, which also covers the student's earlier attempts (score-delta history).
   - Events are written in the generator CSV layout and passed to `features.read_attempts` as a file-like object, then to `build_features` with the artifact's reference and to `train.score_model` with the `iforest` model.
   - Before scoring, each attempt is checked on its own. Attempts the frozen reader rejects are excluded from the cohort; if the target is one of them, it is recorded as `not_scorable` with the reason.
4. **Emit.**
   - A raw score ≥ the medium threshold yields `to_risk_event.risk_event(...)`, unchanged except for the risk id. Severity is medium/high per the artifact thresholds, and the explanation is the frozen list of marginal outliers.
   - `risk_id` is replaced by a deterministic UUIDv5 of model name, artifact version and attempt id, so re-processing can never create a second signal.
   - The event is validated against `risk.behavior_anomaly.v1.schema.json` and stored in `risk_outbox` in the same transaction that marks the attempt `flagged`. Below the threshold the attempt is marked `scored` (level `low`) and no event is published (events.md: absence of a risk is not evidence of integrity).
5. **Publish.** Outbox rows go to `ai.risk` through an idempotent producer (acks=all), keyed by the target attempt id, and are marked once the broker acknowledges them. On a crash the same bytes, with the same `event_id`, are re-sent; grading deduplicates by `event_id` and `risk_id`.

### Input contract of the frozen pipeline v1

`read_attempts` was written for the synthetic generator. Live attempts are scorable only if all of the following hold:
- every question is answered;
- each question is worth exactly 1 point, so `max_score` equals the number of questions;
- the payload keys are exactly the v1 sets. The additive `quiz.submitted.reason` field is dropped before scoring.

Attempts that break these rules are recorded as `not_scorable` and never crash the batch. Event timestamps must agree with `duration_ms` to the millisecond; the quizzes service guarantees this by truncating its clock to milliseconds.

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
| `ANOMALY_HEALTH_PORT` | `8081` | Internal listener for liveness, metrics and attempt status. Compose keeps this port inside its network. |
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

## Processing status for grading (FR-28)

Call `GET /attempts/{attempt_id}/status` on the scorer's internal listener. The ID is the
`quiz.submitted` payload's `attempt_id`. An attempt enters the store when its submission
is ingested. A successful lookup returns HTTP 200 with a small JSON object.

| Status | Meaning for the caller |
|---|---|
| `pending` | The submission was received and awaits scoring. |
| `scored` | The scorer completed the assessment without opening a risk signal. |
| `flagged` | The scorer created a risk outbox record. Delivery and teacher review have their own lifecycle. |
| `not_scorable` | The model could not score this input. A bounded `reason` identifies unsupported input, missing events, a non-finite score or another limitation. |
| `error` | Processing failed. The `reason` is `scoring`, `invalid_output` or `other`. |

For example, a failed model call returns `{"status":"error","reason":"scoring"}`.
HTTP 404 means the attempt is unknown to this scorer. It may precede ingestion.
HTTP 503 means the observation could not be served. Grading should keep these
outcomes distinct from a completed assessment. Existing rows with older text
diagnostics map to the same public states. Responses omit identifiers, answers,
model evidence and internal diagnostic text. Grading must check the caller's
access to an attempt before requesting its status.

## Metrics

`GET /metrics` uses the [Prometheus text format](https://prometheus.io/docs/instrumenting/exposition_formats/).
Labels come from fixed service vocabularies. Counters and histograms cover the
current process lifetime. Gauges read durable state from PostgreSQL in a
read-only snapshot. A failed database read makes the scrape return HTTP 503.
Liveness at `/healthz` continues to track the main loop independently.

| Metric | Type | What one observation represents |
|---|---|---|
| `examory_anomaly_events_total{outcome}` | Counter | One handled Kafka record. Outcomes are `stored`, `duplicate`, `ignored` and `rejected`. A DB failure can be retried before the record is counted. |
| `examory_anomaly_attempts{status}` | Gauge | Number of stored attempts currently in each of the five public states. |
| `examory_anomaly_attempts_by_reason{status,reason}` | Gauge | Current counts for the fixed reasons under `not_scorable` and `error`. |
| `examory_anomaly_scoring_duration_seconds` | Histogram | Elapsed time from starting the cohort read to finishing the model call for one ready attempt, including failed calls. |
| `examory_anomaly_submission_to_publication_seconds` | Histogram | Time from the submitted event's `occurred_at` to the locally recorded broker acknowledgement of its risk. Only a newly marked outbox row contributes. Negative differences from clock skew are clamped to zero. |
| `examory_anomaly_watermark_lag_seconds` | Gauge | Database time minus the latest stored quiz event time, floored at zero. The value is `NaN` before the first event and increases during quiet periods. |

Scrape each process for counters and histograms. With multiple replicas sharing
one database, read the attempt gauges from one replica or take the maximum across
replicas to avoid counting the same rows twice. Publication timings reflect alerts
with acknowledged delivery. A process crash between the database commit and the
in-memory observation can lose a histogram sample. The persisted outbox still
records the acknowledgement. A delivery retried after a lost acknowledgement
uses the first acknowledgement the scorer could durably record.

With the stack running, this command reads the metrics inside the scorer's
container using the Python interpreter shipped in the image.

```sh
docker compose exec -T anomaly-scorer python -c "from urllib.request import urlopen; print(urlopen('http://127.0.0.1:8081/metrics', timeout=5).read().decode())"
```

An internal diagnostic client with curl can read both routes:

```sh
curl --fail http://anomaly-scorer:8081/metrics
curl --fail "http://anomaly-scorer:8081/attempts/${ATTEMPT_ID}/status"
```

For [D5](../../docs/07_analysis/technology_comparison.md), the scoring histogram includes
a two-second bucket. Its samples cover ready submitted attempts. A VPS load test
must measure raw answer-event latency and CPU separately to evaluate the stated
20-events-per-second target.

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
- The scoring loop uses a single database connection and rereads the cohort for each attempt. Monitoring uses at most four separate, read-only connections per process.
- Grading can query the internal status endpoint. Connecting it to a teacher-facing FR-28 view remains a grading task.
