# quizzes service

Question bank, quizzes and variants, assignment to groups, timed attempts, per-question answers with **server-side timing**, scoring, and the `quiz.started` / `quiz.answered` / `quiz.submitted` events on `lms.quizzes` (FR-11–FR-14, FR-15 partially, NFR-5, NFR-12). These events feed the anomaly module of the M4 scenario (project_management.md §3a).

## Architecture

```
cmd/quizzes/main.go            composition root (config, wiring, lifecycle, `quizzes healthcheck`)
internal/domain                Quiz/Variant/Question/Option, Assignment, Attempt state machine, timing,
                               answer_hash, scoring, shuffling, domain errors (no framework imports)
internal/app                   use cases on ports (Store/Tx); student views without the marking key
internal/app/apptest           in-memory Store emulating the schema's constraints (unit tests)
internal/adapters/postgres     pgx Store; embedded migrations (schema + triggers)
internal/adapters/http         Gin routes, DTOs, error mapping, simulation-clock gate
integration/                   tests against real Postgres/Redis/Kafka and the gateway (tag `integration`)
```

Every state change and its event are written in **one** transaction (`outbox` table from `services/platform/outbox`). Each attempt mutation first locks the attempt row (`SELECT … FOR UPDATE`), so one attempt's transactions commit serially and its outbox rows are in commit order; the relay publishes rows of one partition key in order. Partition key is `actor.user_id` (the student), so **every `quiz.answered` of an attempt precedes its `quiz.submitted`** on the topic.

## API

Bearer access token from `auth` required on every `/v1` route. Errors are RFC 9457 problems with a stable `code`; malformed ids return 404; bodies are strict JSON (unknown fields rejected, single object, `Content-Type: application/json`; 512 KiB for quizzes, 8 KiB otherwise).

| Method | Path | Role | Description |
|---|---|---|---|
| POST | `/v1/quizzes` | teacher, admin | create a draft (below). 422 lists invalid fields |
| GET | `/v1/quizzes?limit=&cursor=` | any | teacher: own quizzes; admin: all; student: published quizzes assigned to the token's group |
| GET | `/v1/quizzes/{quizId}` | any | author view with marking key (owner or admin); student view without questions (only if assigned) |
| PUT | `/v1/quizzes/{quizId}` | owner, admin | replace a draft; 409 `quiz_not_draft` after publish |
| POST | `/v1/quizzes/{quizId}/publish` | owner, admin | makes the quiz immutable |
| PUT | `/v1/quizzes/{quizId}/assignments/{groupId}` | owner, admin | `{opens_at, closes_at}` (upsert); quiz must be published |
| GET | `/v1/quizzes/{quizId}/assignments` | owner, admin | assigned groups |
| GET | `/v1/quizzes/{quizId}/attempts?limit=&cursor=` | owner, admin | attempts with status and raw score |
| POST | `/v1/quizzes/{quizId}/attempts` | student | start; checks assignment of the **token's** group, window, `max_attempts`, no parallel active attempt; publishes `quiz.started`; 201 attempt view |
| GET | `/v1/attempts/{attemptId}` | student (own) | attempt view: questions, options in a per-attempt shuffled order, current selections, `remaining_ms`; no correctness data |
| POST | `/v1/attempts/{attemptId}/questions/{questionId}/shown` | student (own) | idempotent; records the first display time (server clock) |
| PUT | `/v1/attempts/{attemptId}/answers/{questionId}` | student (own) | `{"selected_option_ids":[…]}`; 409 `question_not_shown` if never shown (timing is never fabricated); identical answer → 200 `unchanged:true`, no event; otherwise `quiz.answered` |
| POST | `/v1/attempts/{attemptId}/submit` | student (own) | header `Idempotency-Key` (8–128 of `[A-Za-z0-9_.:-]`, required); scores and publishes exactly one `quiz.submitted` |
| GET | `/healthz`, `/readyz` | — | liveness; readiness (Postgres, Redis, JWKS). Not routed by the gateway |

Teachers only see and manage their own quizzes; another teacher's quiz is 404 (not 403) so ids cannot be probed. Students only reach their own attempts: every attempt query is scoped by the token's user id, never by request data (another student's attempt is 404).

Create/replace body:

```json
{"title":"Midterm","course_id":"c_10","time_limit_sec":1800,"max_attempts":1,"show_score":true,
 "questions":[{"text":"2+2?","points":2,"topic_id":"t_arith",
               "options":[{"key":"a","text":"3"},{"key":"b","text":"4"}],"correct_keys":["b"]}]}
```

Limits (domain validation, mirrored by `CHECK`s): title ≤ 200, question ≤ 2000, option ≤ 500 characters; 1–100 questions; 2–8 options; ≥ 1 correct; points 1–100; time limit 30 s–4 h; max attempts 1–20; window ≤ 366 days. `key` is a client label (≤ 16 chars) only used to name correct options; the server assigns UUIDv7 ids. For M4 a quiz has one variant containing all questions in order; the schema supports several variants per quiz (rotated by attempt number).

Attempt view (student): `{id, quiz_id, variant_id, attempt_no, status, started_at, deadline_at, finished_at, remaining_ms, server_time, questions:[{id, index, text, points, selection:"single|multiple", shown, options:[{id,text}], selected_option_ids}], result}`. `result` (`score`, `max_score`) is present after finalization when the quiz has `show_score: true`. `selection` reveals only whether more than one option is correct (radio vs checkbox), never which.

Submit response: `{attempt_id, status:"submitted|timed_out", finished_at, score?, max_score?}`; a replay with the same key returns the stored result with `Idempotent-Replayed: true`; after a timeout any key returns the `timed_out` result; a different key on a submitted attempt is 409 `attempt_finalized`; reusing a key on another attempt is 422 `idempotency_key_reused`.

## Attempt state machine and timing

`in_progress → submitted | timed_out`; nothing else. Enforced three times: the domain (`Attempt.Finalize`), the update (`… WHERE status='in_progress'`), and the schema (trigger freezes finalized attempts and their answers; `attempt_finalizations` has `PRIMARY KEY (attempt_id)` and is append-only, so a second `quiz.submitted` cannot be written).

- `deadline_at = started_at + time_limit_sec`, capped at the assignment's `closes_at`.
- Requests are accepted until `deadline_at + QUIZZES_ANSWER_GRACE` (default 2 s, for in-flight saves).
- **Timeout sweeper** (every `QUIZZES_SWEEP_INTERVAL`) finalizes expired real-time attempts as `timed_out`; safe on several replicas (each attempt re-checked under its row lock). Any request touching an expired attempt also finalizes it (lazy timeout). A timed-out attempt finishes at its deadline (never before its last accepted save), so `duration_ms ≤ time limit + grace`.
- `quiz.answered` counting conventions (events.md §3): `question_index` = zero-based position in the variant; `is_change` = false for the first saved answer to a question in the attempt, true for every later *different* answer (A→B→A counts twice; an identical re-save is a no-op without an event); `time_since_shown_ms` = save time − first-shown time (cumulative); `time_since_attempt_start_ms` = save time − `started_at`. `quiz.submitted.answers_changed` = number of `is_change=true` events.
- `answer_hash` = lowercase hex SHA-256 of the canonical answer: the selected option ids, each trimmed and lower-cased, sorted, de-duplicated, joined with `\n` (e.g. `sha256("a\nb")` = `7e18f737…3c78`). Option ids are per question and the same for every student (they are not the shuffled display labels), so equal answers to a question hash equally across students; the hash contains no student, attempt or question id.
- Scoring is all-or-nothing per question (full points iff the selected set equals the correct set); `score`/`max_score` are raw points.
- Option order: sorted by `SHA-256(attempt seed ‖ question id ‖ option id)`, stable within an attempt.

## Simulation clock (research replay) — NOT a production feature

Disabled by default (`QUIZZES_SIM_CLOCK=disabled`). The research simulator replays simulated students through the real pipeline, so their events go through the same validation, storage, outbox and Kafka path as real ones.

When `QUIZZES_SIM_CLOCK=enabled` **and** the caller's token has role `system`:

- `X-Sim-Time: <RFC 3339>` is used as "now" for that request (truncated to ms; becomes `occurred_at` and the `event_id` timestamp);
- `X-Act-As-User: <uuid>` and optional `X-Act-As-Group: <uuid>` make the request act as that student (events carry the student as actor);
- simulated time must be monotone per attempt (409 `non_monotonic_time`); an attempt started with simulated time only accepts simulated requests and vice versa (409 `clock_mode_mismatch`); the real-time sweeper never touches simulated attempts (they time out lazily on the next simulated request).

When the flag is off, any of these headers → **400 `simulation_disabled`**. When on, a non-`system` caller using them → 403. A loud `WARN` is logged at startup when enabled. The gateway strips the three headers, so replay must call the service directly on the internal network. Both modes are covered by `internal/adapters/http/handler_test.go` and `internal/app/app_test.go` (`TestSimulatedClock`).

## Configuration

| Variable | Default | Notes |
|---|---|---|
| `QUIZZES_HTTP_ADDR` | `:8080` | |
| `QUIZZES_DATABASE_URL` | — (required, secret) | role `quizzes_svc`, database `quizzes` |
| `QUIZZES_REDIS_ADDR` / `QUIZZES_REDIS_PASSWORD` | — / empty | rate limits (in-memory fallback if Redis is down) |
| `QUIZZES_KAFKA_BROKERS` | — (required) | comma-separated |
| `QUIZZES_JWKS_URL` | — (required) | e.g. `http://auth:8080/.well-known/jwks.json` |
| `QUIZZES_JWT_ISSUER` / `QUIZZES_JWT_AUDIENCE` | `examory-auth` / `examory` | |
| `QUIZZES_SIM_CLOCK` | `disabled` | `enabled` only for research replay |
| `QUIZZES_ANSWER_GRACE` | `2s` | 0–30 s |
| `QUIZZES_SWEEP_INTERVAL` | `5s` | 1 s–5 min |
| `QUIZZES_RATE_LIMIT_PER_IP` / `_PER_SUBJECT` | `600` / `300` per minute | raise for replay |
| `QUIZZES_TRUSTED_PROXIES` | none | CIDRs whose `X-Forwarded-For` is trusted (the gateway) |
| `QUIZZES_CORS_ORIGINS` | none | exact-match allow-list |
| `QUIZZES_DEV_MODE` | `false` | lets the producer auto-create topics |
| `QUIZZES_LOG_LEVEL` | `info` | |

## Run and test

```bash
make dev-env && make infra                 # once, from the repository root
cd services/quizzes
go test -race ./...                        # unit tests (domain, app with fakes, HTTP)
cd ../.. && make integration               # platform, auth, quizzes against compose infra
docker compose up -d --build               # full stack; gateway on http://127.0.0.1:8000/api/quizzes/
# gateway end-to-end (real auth + quizzes containers, Kafka):
set -a; . ./.env; set +a; cd services/quizzes && E2E_GATEWAY_URL=http://127.0.0.1:8000 \
  E2E_ADMIN_EMAIL=$AUTH_BOOTSTRAP_ADMIN_EMAIL E2E_ADMIN_PASSWORD=$AUTH_BOOTSTRAP_ADMIN_PASSWORD \
  IT_KAFKA_BROKERS=127.0.0.1:9092 go test -tags integration -run E2E -v ./integration/
```

Integration tests create a throwaway database per test (`IT_PG_ADMIN_URL`) and cover: migrations applied twice; the full flow over HTTP with tokens verified through a JWKS endpoint; events read back from Kafka with envelope, payload values, per-partition order and JSON Schema validity; 24 concurrent submits → exactly one finalization and one `quiz.submitted`; direct SQL attempts to edit a published quiz, reopen/delete a finalized attempt or add a second finalization are rejected by the schema; concurrent sweepers time out once.

## Security model

| Threat | Mitigation |
|---|---|
| Forged / tampered / replayed-as-other-type tokens | EdDSA only, `kid` lookup in the auth JWKS, `typ=at+jwt`, iss/aud/exp/nbf/iat checks with 30 s leeway, max lifetime 1 h (`platform/authn`) |
| IDOR on attempts | every attempt query includes `user_id = <token sub>`; foreign attempts are 404; group comes from the token, never from the body |
| Teacher editing another teacher's quiz | object-level check `owner_id = sub` (admins excepted); 404 for others |
| Leaking the marking key to students | student views are separate types with no correctness fields; tests assert `correct` never appears in student JSON |
| Changing a quiz after students saw it | publish makes content immutable; triggers reject UPDATE/DELETE of published quizzes and their questions/options/variants |
| Double submission / double `quiz.submitted` | row lock + state check + `attempt_finalizations` PK; idempotency key unique per student; race tests in unit and integration suites |
| Fabricated timing | all times are server-side; answering an unshown question is refused; first-shown time is write-once (trigger) |
| Time manipulation via headers | simulation headers rejected unless explicitly enabled; even then only for `system` callers; stripped at the gateway |
| Lost or duplicated events | transactional outbox, at-least-once relay with ordered per-key delivery; duplicates keep `event_id` (consumers dedup, NFR-12) |
| Injection | pgx parameterised statements only; ids validated as UUIDs before use |
| Oversized / malformed input, DoS | body caps, strict JSON, bounded collections, pagination ≤ 100, per-IP and per-subject Redis rate limits, server timeouts, gateway limits |
| Error detail leakage | problems carry a code and generic text; internal errors are logged with the request id only |
| Personal data in events/logs | events carry ids only (NFR-9); logs never include tokens or answers (redacting logger as a backstop) |
| Container breakout / tampering | distroless static nonroot image, read-only rootfs, all capabilities dropped, `no-new-privileges` |

## Dependencies

| Module | Why |
|---|---|
| `services/platform` | shared kit (config, logging, HTTP, JWT verification, outbox, migrator, rate limiting) |
| `github.com/gin-gonic/gin` | project HTTP framework decision |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and pool |
| `github.com/twmb/franz-go` | Kafka producer (idempotent, acks=all) via the outbox relay; consumer in tests |
| `github.com/redis/go-redis/v9` | rate limiting |
| `github.com/santhosh-tekuri/jsonschema/v6` | contract tests only (via `platform/eventschema`) |

## Known limitations

- Course membership is verified through `COURSES_INTERNAL_URL` with `COURSES_INTERNAL_TOKEN`. Authoring requires an assigned teacher; assignments and attempts also require current group enrollment. Membership outages return 503. Students read and submit only their own in-scope attempts.
- `course_id` is an opaque string; there is no `courses` service yet.
- One variant per quiz is generated; there is no API to define additional variants, and no question bank shared across quizzes (questions belong to one quiz).
- Only choice questions; all-or-nothing scoring; no partial credit, no manual grading, no release workflow beyond `show_score` (FR-15 is partial).
- Access tokens remain valid until expiry (≤ 10 min) after logout or deactivation; quizzes has no revocation list.
- Retention/expiry of attempts and answers (NFR-10) and audit logging of teacher actions in quizzes (NFR-16) are not implemented.
- The answer-hash canonical form and the `reason` field of `quiz.submitted` are proposals that need the events.md review (see events.md §6 draft).
