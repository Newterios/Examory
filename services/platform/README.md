# platform — shared Go kit for Examory services

Module `github.com/Newterios/Examory/services/platform`. Services depend on it through `replace github.com/Newterios/Examory/services/platform => ../platform` (the repository does not use `go.work`). Go 1.26 (`toolchain go1.26.11`).

The public API below is deliberately small. Treat it as stable: additive changes are fine, breaking changes need a pull request touching every service. `auth` and `quizzes` are the reference users; `grading` and the anomaly consumer should be built the same way (see "Building a consumer service").

## Packages

| Package | Purpose | Main API |
|---|---|---|
| `clock` | testable time | `Clock{Now()}`, `System{}`, `NewFake(t)` (`Set`, `Advance`), `Fixed` |
| `ids` | UUIDv7 (RFC 9562) | `NewV7()`, `NewV7At(t)` (timestamp = t at ms), `Valid`, `Canonical`, `V7Time` |
| `config` | env config with accumulated validation; secrets never echoed | `New(os.LookupEnv)`, `String/Required/Secret/OptionalSecret/Enum/Bool/Int/Duration/List/URL/Check`, `Err()` |
| `logging` | JSON `slog`, `service` + `request_id` attributes, key-based redaction (password, token, secret, cookie, authorization, answer, dsn, seed, private, credential, selected) | `New(w, service, level)`, `Discard()`, `WithRequestID`, `RequestID` |
| `httpx` | Gin engine + middleware, server, problems, strict JSON, health | `NewEngine(EngineConfig)`, `Serve(ctx, ServerConfig, h, log)`, `RegisterHealth`, `Probe`, `BindJSON/DecodeJSON`, `Page`, `PathID`, `WriteProblem/Fail/Invalid/Internal`, `Authenticate(verifier)`, `RequireRoles(...)`, `Principal(c)`, `RateLimit(l, log, name, limit, window, ByIP|BySubject)`, `RequestID`, `Recovery`, `SecurityHeaders`, `BodyLimit`, `CORS`, `AccessLog` |
| `authn` | principal, roles, access-token claims, Ed25519 JWK/JWKS, signing (issuer) and verification (resource services) | `Principal`, `Role*`, `WithPrincipal/FromContext`, `NewVerifier(src, VerifierConfig)`, `NewJWKSClient(url, JWKSOptions)`, `StaticKeys`, `NewSigner`, `PublicJWK`, `Thumbprint`, `ParseJWKSet` |
| `ratelimit` | fixed-window limits | `Limiter`, `NewRedis(client, prefix)`, `NewMemory(clock)`, `Fallback{Primary, Secondary, Logger}` |
| `events` | envelope of events.md §1, topics, typed payloads | `New(type, version, producer, actor, ctx, payload, at)`, `Envelope.Validate`, `Parse`, `DecodePayload`, `Time`, `Ptr`, `Topic*`, `Type*`, `QuizStarted/QuizAnswered/QuizSubmitted/User*/GroupUpdated` |
| `db` | pgx pool with server-side timeouts, tx helper, SQLSTATE helpers | `Open(ctx, Config)`, `InTx(ctx, pool, fn)`, `ErrorCode`, `IsUniqueViolation`, `Code*` |
| `migrate` | embedded SQL migrations under a Postgres advisory lock, checksum-verified | `Up(ctx, pool, log, sets...)`, `Set{Name, FS}` |
| `outbox` | transactional outbox + relay to Kafka | `Migrations()`, `Enqueue(ctx, tx, topic, key, env)`, `NewRelay(pool, publisher, log, RelayConfig)`, `Relay.Run(ctx)` |
| `consumer` | Kafka consumer group with idempotent processing by `event_id` (NFR-12) | `Migrations()`, `New(Config, pool, handler, log)`, `Consumer.Run(ctx)`, `HandleOnce(ctx, pool, name, env, handler)`, `ErrPermanent` |
| `kafkax` | franz-go wrappers | `NewProducer(Config)` (idempotent, acks=all), `Publisher`, `Message`, `NewGroupClient` |
| `redisx` | Redis client with timeouts | `New(Config)`, `Ping(client)` |
| `eventschema` | **tests only**: validate serialized events against `docs/03_architecture/schemas` | `FindDir()`, `Load(dir)`, `Validator.Validate(raw)` |
| `testpg` | **tests only**: throwaway database per test from `IT_PG_ADMIN_URL` | `New(t)` |

## Guarantees and defaults

- **HTTP server**: `ReadHeaderTimeout` 5 s, `ReadTimeout` 15 s, `WriteTimeout` 30 s, `IdleTimeout` 120 s, `MaxHeaderBytes` 16 KiB, graceful shutdown 20 s. Gin in release mode; trusted proxies explicit (none by default, so `ClientIP` is the TCP peer); no automatic path redirects; unknown routes/methods return problems.
- **Middleware order** (`NewEngine`): request id (accepts a well-formed gateway `X-Request-ID`, else UUIDv7) → panic recovery (500 problem, stack only in logs) → security headers (`nosniff`, `DENY`, CSP `default-src 'none'`, `no-referrer`, CORP/COOP, `Cache-Control: no-store` default, optional HSTS) → access log (route template, never the query string) → CORS (exact allow-list) → body limit.
- **Problems** (RFC 9457): `{type: "urn:examory:problem:<code>", title, status, detail, code, request_id, errors?}`, media type `application/problem+json`. `Internal` never returns error text.
- **Strict JSON**: `Content-Type: application/json` (charset utf-8 only), size cap, valid UTF-8, exactly one top-level object, `DisallowUnknownFields`. Note Go's decoder matches field names case-insensitively and keeps the last duplicate key.
- **JWT verification**: `alg` must be `EdDSA` (no `none`, no HMAC confusion), header `typ` must be `at+jwt`, `kid` required and resolved via JWKS; `iss`, `aud`, `exp` (required), `nbf`, `iat` (required, not in the future) with 30 s leeway (max 2 m); lifetime ≤ 1 h; `sub`, `jti`, `group_id` must be UUIDs; role must be known; tokens > 4 KiB rejected.
- **JWKS client**: cache 5 min; unknown `kid` triggers at most one refresh per 30 s (forged kids cannot hammer auth); a failed refresh keeps serving cached keys; 64 KiB body cap; 5 s timeout; only Ed25519 signing keys accepted. Rotation: auth publishes old and new public keys; verifiers pick by `kid`.
- **Events**: `occurred_at` is RFC 3339 UTC with milliseconds; `event_id` is a UUIDv7 whose timestamp equals `occurred_at` (enforced by `Envelope.Validate`, which `outbox.Enqueue` calls).
- **Outbox relay**: at-least-once (a row is marked only after the broker ack; a crash in between re-publishes identical bytes, same `event_id`); one active relay per database (advisory lock held on the connection that also runs the relay queries); rows of one partition key are sent strictly in id order: the n-th pending row of a key is sent only after the (n−1)-th was acknowledged, and after a failure nothing later of that key is sent; exponential backoff with jitter (200 ms → 30 s); publish timeout 10 s per wave; published rows deleted after 72 h; `Run` returns on context cancel after the in-flight wave. Ordering by id equals commit order for one key only if producers serialise writes per key (lock the aggregate row first, as `quizzes` does).
- **Consumer**: auto-commit disabled; offsets committed after the handler's transaction commits; `(consumer, event_id)` inserted into `processed_events` in the same transaction, so duplicates are skipped; failing handlers are retried with backoff (`MaxAttempts` 10), `ErrPermanent` or exhausted retries are logged and skipped (no DLQ yet); envelopes that fail validation are logged and skipped.
- **Migrator**: `pg_advisory_lock` so concurrent replicas are safe; each file in its own transaction; SHA-256 recorded; a modified applied migration aborts startup.
- **DB pool**: `statement_timeout` and `lock_timeout` 5 s, `idle_in_transaction_session_timeout` 15 s, `timezone=UTC`, max 10 connections by default; DSNs are never included in errors.
- **Rate limiting**: Redis fixed window (atomic Lua `INCR`+`PEXPIRE`); `Fallback` switches to in-memory limits when Redis errors (limits stay on, per instance). `httpx.RateLimit` allows the request if the limiter itself errors (use `Fallback` to avoid that).

## Building a consumer service (e.g. `grading`, anomaly scoring)

```go
pool, _ := db.Open(ctx, db.Config{URL: cfg.DSN, ApplicationName: "grading"})
_ = migrate.Up(ctx, pool, logger, outbox.Migrations(), consumer.Migrations(), gradingMigrations)
c, _ := consumer.New(consumer.Config{
    Kafka:  kafkax.Config{Brokers: cfg.Brokers, ClientID: "grading"},
    Group:  "grading", Topics: []string{events.TopicRisk},
}, pool, func(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
    var p RiskPayload
    if err := env.DecodePayload(&p, false); err != nil { return consumer.ErrPermanent }
    // write the effect with tx; publish follow-up events with outbox.Enqueue(ctx, tx, ...)
    return nil
}, logger)
go c.Run(ctx)
```

Order consumers by `occurred_at` and `event_id` across attempts (events.md §1); within one attempt Kafka order is guaranteed for `lms.quizzes` (partition key = student id).

## Tests

```bash
go test -race ./...                                    # unit tests
make infra && make integration                         # from the repo root (migrator, relay, consumer)
IT_DOCKER_KAFKA=examory-kafka-1 make integration       # also runs the docker-pause Kafka outage test
```

Integration tests (`//go:build integration`) cover: concurrent migrators, checksum mismatch detection, relay during a broker outage (nothing marked, nothing lost), recovery after restart with per-key order, crash-between-ack-and-mark duplicates with identical `event_id`, a real `docker pause` of Kafka mid-stream, and `HandleOnce` under 8 concurrent deliveries (exactly one effect; failure rolls back the dedup row).

## Dependencies

gin (HTTP framework decision), pgx/v5 (Postgres), franz-go (Kafka), go-redis/v9 (Redis), golang-jwt/v5 (JWT parsing with an algorithm allow-list), santhosh-tekuri/jsonschema/v6 (contract tests only), alicebob/miniredis (tests only). No UUID library: UUIDv7 is 60 lines in `ids`.

## Known limitations

- No metrics endpoint and no tracing; health/readiness only. Consumer lag and outbox backlog are not exported yet (NFR-11).
- Consumer has no dead-letter topic; permanently failing events are logged and skipped.
- The relay is single-active per database (by design for ordering); throughput is bounded by one connection and per-key waves.
- `JWKSClient` holds its lock during a fetch (fetches are rare and rate-limited).
- TLS to Postgres/Redis/Kafka is not configured for local development (NFR-4 for staging).
