# auth service

Authentication, sessions and account administration for Examory (FR-1, FR-2 minimal, NFR-5, NFR-7, NFR-16).

**For M4 the minimal responsibilities of the planned `users` service are folded into `auth`:** admin-provisioned accounts, roles, account deactivation, groups and group membership. There is no self-registration. When `users` is split out (stage 3), the `users`, `groups`, `group_members` tables and the `user.created` / `group.updated` events move with it; the token claims stay the same.

## Architecture

```
cmd/auth/main.go                 composition root (config, wiring, lifecycle, `auth healthcheck`)
internal/domain                  entities, validation, password policy, lockout policy, domain errors (no framework imports)
internal/password                Argon2id hasher (PHC format)
internal/app                     use cases (login, refresh, logout, me, admin) on top of ports
internal/app/apptest             in-memory fakes of the ports (tests only)
internal/adapters/postgres       app.Store with pgx; embedded migrations
internal/adapters/redis          refresh-token store and login lockout
internal/adapters/http           Gin router, DTOs, error mapping
integration/                     end-to-end test against real Postgres/Redis/Kafka (build tag `integration`)
```

Every state change and its event are written in one Postgres transaction (`outbox` table, `services/platform/outbox`); the relay publishes to Kafka `lms.users`.

## API

All errors are RFC 9457 problems (`application/problem+json`) with a stable `code`. Request bodies are strict JSON (unknown fields rejected, one object, `Content-Type: application/json`, 4 KiB cap).

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/v1/auth/login` | — | `{email, password}` → session (below) + refresh cookie. 401 `invalid_credentials`, 429 `account_locked`/`rate_limited` |
| POST | `/v1/auth/refresh` | refresh cookie | rotates the cookie, returns a new session. Replay of a rotated cookie → 401 and the whole session family is revoked |
| POST | `/v1/auth/logout` | refresh cookie | 204; revokes the session family, clears the cookie. Idempotent |
| GET | `/.well-known/jwks.json` | — | Ed25519 public keys (`Cache-Control: public, max-age=300`) |
| GET | `/v1/me` | bearer | the caller's account |
| POST | `/v1/users` | admin | `{email, display_name, role, password}` → 201 user |
| GET | `/v1/users?limit=&cursor=` | admin | `{items: [user], next_cursor}` (limit ≤ 100, keyset on id) |
| GET | `/v1/users/{id}` | admin | user |
| PATCH | `/v1/users/{id}` | admin | `{role?, status?}` (`active`/`deactivated`). Admins cannot demote/deactivate themselves (409 `self_modification`); the last active admin cannot be removed (409 `last_admin`) |
| POST | `/v1/groups` | admin | `{name, course_id?}` → 201 group |
| GET | `/v1/groups?limit=&cursor=` | admin | `{items: [group], next_cursor}` |
| GET | `/v1/groups/{id}` | admin | group with `members` |
| PUT | `/v1/groups/{id}/members/{userId}` | admin | add an active student (moves them out of a previous group); 409 `not_student` otherwise |
| DELETE | `/v1/groups/{id}/members/{userId}` | admin | 204; 404 `not_member` |
| GET | `/healthz`, `/readyz` | — | liveness; readiness (Postgres, Redis). Not routed by the gateway |

Shapes:

```jsonc
// session (login, refresh)
{"access_token":"<JWT>","token_type":"Bearer","expires_in":600,
 "user":{"id":"<uuidv7>","email":"s@x.kz","display_name":"…","role":"student","status":"active","group_id":"<uuid>|null","created_at":"…"}}
// user (/v1/me, /v1/users/…)
{"id":"…","email":"…","display_name":"…","role":"student|teacher|admin|system","status":"active|deactivated","group_id":null,"created_at":"…"}
// group
{"id":"…","name":"SE-2424","course_id":"c_10|null","member_count":1,"created_at":"…","members":[user]}   // members only on GET /v1/groups/{id}
```

Access token (JWT, `alg=EdDSA`, header `kid`, `typ=at+jwt`, TTL 10 min): `iss` (`examory-auth`), `aud` (`examory`), `sub` (user id), `role`, `group_id` (string or `null`), `jti`, `iat`, `nbf`, `exp`. Resource services verify it with `platform/authn` against the JWKS. Role and group changes take effect at the next refresh (≤ 10 min).

Accounts for the research simulator: an admin creates a user with `role: "system"` and logs in with it like any other user.

## Events (topic `lms.users`, schemas in `docs/03_architecture/schemas/`)

| Event | When | Partition key | Actor |
|---|---|---|---|
| `user.created` | admin creates a user; bootstrap admin | `payload.user_id` | creating admin (bootstrap: the new admin itself) |
| `user.logged_in` | successful login | `payload.user_id` | the user |
| `user.logged_out` | `reason`: `logout`, `token_reuse` (replay detected), `deactivated` (admin) | `payload.user_id` | the user (admin for `deactivated`) |
| `group.updated` | `change`: `created`, `member_added`, `member_removed` | `payload.group_id` | admin |

Payloads carry identifiers only — no e-mail or names (NFR-9). `context.group_id`/`course_id` are the user's group (or the changed group) and its course, otherwise `null`. Refresh-token rotation itself emits no event.

## Configuration

| Variable | Default | Notes |
|---|---|---|
| `AUTH_HTTP_ADDR` | `:8080` | |
| `AUTH_DATABASE_URL` | required (secret) | role `auth_svc` on database `auth` |
| `AUTH_REDIS_ADDR` / `AUTH_REDIS_PASSWORD` | required / optional | keys prefixed `auth:` |
| `AUTH_KAFKA_BROKERS` | required | comma-separated |
| `AUTH_JWT_ISSUER` / `AUTH_JWT_AUDIENCE` | `examory-auth` / `examory` | must match the resource services |
| `AUTH_JWT_SIGNING_KEY` | required unless dev mode (secret) | base64 of a 32-byte Ed25519 seed (`openssl rand -base64 32`) |
| `AUTH_JWT_KID` | RFC 7638 thumbprint | key id of the signing key |
| `AUTH_JWT_EXTRA_PUBLIC_KEYS` | — | `kid=base64url(pub),…` served in the JWKS for rotation |
| `AUTH_DEV_MODE` | `false` | allows an ephemeral signing key (logged as a loud warning), Kafka topic auto-creation, no HSTS |
| `AUTH_BOOTSTRAP_ADMIN_EMAIL` / `_PASSWORD` | — | creates the first admin if no active admin exists; remove the password afterwards |
| `AUTH_TRUSTED_PROXIES` | none | CIDRs whose `X-Forwarded-For` is trusted (the gateway) |
| `AUTH_CORS_ORIGINS` | none | exact origins; also accepted by the CSRF Origin check |
| `AUTH_COOKIE_SECURE` | `true` | set `false` only for plain-HTTP development on a non-localhost host |
| `AUTH_ACCESS_TTL` | `10m` | 1m–1h |
| `AUTH_REFRESH_TTL` | `168h` | lifetime of each refresh token |
| `AUTH_SESSION_MAX_AGE` | `720h` | absolute lifetime of a session family regardless of rotation |
| `AUTH_HASH_CONCURRENCY` | `4` | max simultaneous Argon2id computations (19 MiB each) |
| `AUTH_LOG_LEVEL` | `info` | |

**Key rotation:** generate a new seed; deploy with the new `AUTH_JWT_SIGNING_KEY` and the old public key in `AUTH_JWT_EXTRA_PUBLIC_KEYS`; remove the old key after 10 minutes (access-token TTL) plus the JWKS cache time (5 min).

## Run and test

```bash
make dev-env && make infra                                  # repository root
cd services/auth
GOTOOLCHAIN=local go test -race ./...                       # unit + HTTP tests (fakes, miniredis)
# integration (fresh database per run; needs the compose infra)
set -a; . ../../.env; set +a
IT_PG_ADMIN_URL="postgres://examory:$POSTGRES_PASSWORD@127.0.0.1:5432/postgres?sslmode=disable" \
IT_REDIS_ADDR=127.0.0.1:6379 IT_REDIS_PASSWORD="$REDIS_PASSWORD" IT_KAFKA_BROKERS=127.0.0.1:9092 \
  go test -race -count=1 -tags integration ./integration/
docker build -f auth/Dockerfile -t examory/auth:dev ..      # from services/auth: context is services/
```

## Security model

Passwords: Argon2id, m=19456 KiB, t=2, p=1 (OWASP), 16-byte random salt, 32-byte key, PHC string; constant-time comparison; parameters in stored hashes are parsed with caps (≤ 256 MiB, ≤ 10 iterations) so a tampered row cannot exhaust memory; hashes are upgraded on the next successful login when parameters change. Policy: 12–128 characters, ≤ 256 bytes, not equal to the e-mail; no composition rules.

Refresh tokens: 256-bit random, base64url, only the SHA-256 digest is stored in Redis; one atomic Lua script rotates on every use and detects reuse; families have a 30-day absolute lifetime; refresh re-reads the account from Postgres (deactivated → 401, role/group changes applied).

| Threat | Mitigation |
|---|---|
| Online password guessing / credential stuffing | per-IP limit on login (20 / 5 min, Redis with in-memory fallback) plus gateway limit; per-account exponential lockout (5 failures → 30 s, doubling to 15 min, 24 h window) keyed by a hash of the e-mail, also for unknown e-mails; while locked the password is not checked |
| User enumeration | identical 401 for unknown/deactivated/wrong password; dummy Argon2id verification for missing accounts (same cost); lockout applies to unknown e-mails |
| Offline cracking after a DB leak | Argon2id with OWASP parameters, per-password salt; refresh tokens stored only as SHA-256 |
| Hash-based DoS | 256-byte input cap, concurrency semaphore on hashing, capped stored parameters, body size caps |
| Stolen refresh token | rotation on every use; replay of an old token revokes the whole family and is audited (`auth.token_reuse`) and published (`user.logged_out` reason `token_reuse`) |
| XSS stealing tokens | refresh token only in an `HttpOnly; Secure; SameSite=Strict; Path=/v1/auth` cookie; access token short-lived (10 min) |
| CSRF | cookie only sent to `/v1/auth/*` and `SameSite=Strict`; requests with a foreign `Origin` are rejected (403); refresh body tokens are never accepted; all other APIs use bearer tokens |
| JWT forgery / algorithm confusion | EdDSA only (`alg` allow-list), required `kid` lookup, `typ=at+jwt`, issuer/audience/exp/nbf/iat checks, 30 s leeway, max lifetime check (platform/authn) |
| Privilege escalation | admin routes require role `admin` at the router and again in every use case; admins cannot demote/deactivate themselves; the last admin cannot be removed; only students can be group members (also enforced by DB triggers) |
| IDOR on ids | `/v1/me` uses only the token subject; ids in paths are validated UUIDs; admin-only object routes |
| SQL injection | static, parameterized SQL only (pgx) |
| Secrets in logs | slog redaction of sensitive keys; tokens, passwords and hashes are never logged; query strings not logged; audit details contain only ids and counters |
| Tampering with the audit trail | `audit_log` is append-only (trigger rejects UPDATE/DELETE) |
| Lost events | transactional outbox in the same transaction as the change; at-least-once relay |

## Dependencies

| Module | Why |
|---|---|
| `services/platform` | shared HTTP/auth/outbox/config kit |
| `github.com/gin-gonic/gin` | HTTP framework (project decision) |
| `github.com/jackc/pgx/v5` | PostgreSQL driver and pool |
| `github.com/redis/go-redis/v9` | refresh tokens, lockout, rate limits |
| `github.com/twmb/franz-go` | Kafka producer for the outbox relay |
| `github.com/golang-jwt/jwt/v5` | JWT signing (via platform/authn) |
| `golang.org/x/crypto` | Argon2id |
| `github.com/alicebob/miniredis/v2` | tests only: in-process Redis for adapter and rate-limit tests |
| `github.com/santhosh-tekuri/jsonschema/v6` | tests only (via platform/eventschema): event contract tests |

`klauspost/compress` and `quic-go` (transitive) are pinned to patched versions in `go.mod` because govulncheck reported reachable vulnerabilities in the versions selected by default.

## Known limitations

- Access tokens are stateless: after logout, deactivation or a role change, an already issued access token stays valid until it expires (≤ 10 min). There is no access-token denylist.
- No grace window for concurrent refreshes: two tabs refreshing with the same cookie at the same moment trigger reuse detection and both sessions end (secure default; may annoy users).
- No MFA, no password reset / change endpoint, no self-service profile updates yet.
- No TLS in local development (the gateway listens on plain HTTP); `Secure` cookies work on `localhost` in browsers. Staging must terminate TLS (NFR-4).
- The CSRF Origin check compares host names, not ports, so it works behind the gateway even when the forwarded `Host` lacks the port.
- Teacher-to-group/course assignment (FR-3) and enrollment are not modelled; groups have an optional opaque `course_id`.
- A single Redis instance with `requirepass`; Redis ACL users per service and Redis persistence guarantees are not configured. If Redis loses data, sessions are lost (users log in again) — no security impact.
- Lockout is per account key; a distributed attacker can still lock a victim's account for up to 15 minutes (accepted trade-off; per-IP limits reduce it).
- Toolchain: Go 1.26 (`toolchain go1.26.11` in go.mod). Go 1.25 is out of support and govulncheck reports reachable stdlib advisories that are fixed only in 1.26.9+; with go1.26.11 govulncheck reports no vulnerabilities.
