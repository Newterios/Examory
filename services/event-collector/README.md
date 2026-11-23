# Event collector

POST /api/event-collector/v1/events with a student JWT and application/json. The browser supplies one of the following payloads, each with a platform attempt UUID.

| Event | Additional field |
|---|---|
| client.focus_lost | duration_ms, 0 to 86,400,000 |
| client.tab_switched | duration_ms, 0 to 86,400,000 |
| client.paste | chars, 0 to 10,000,000 |

Unknown fields, including clipboard text, fail validation. The maximum request body is 2 KiB. Redis enforces 60 requests per student per minute. If Redis is unavailable, each collector process enforces the same limit locally until shared accounting recovers. The collector forwards only the bearer token to the private quizzes service. Its ownership-filtered attempt endpoint supplies the current status and deadline. Only an in-progress attempt before its deadline is accepted. A supplied exam_session_id must match the server's session. Until quizzes supplies a session, omit it or send null.

Accepted events get a server timestamp and UUIDv7, undergo v1 JSON Schema validation and are sent to Kafka on lms.exams with attempt_id as the key. HTTP 202 includes the event ID after broker acknowledgement. An unknown or closed attempt returns 404, rate limiting returns 429, and an unconfirmed upstream request returns 503. Browser events are best effort. A transport failure after broker acknowledgement can produce a duplicate on retry. A submission racing an authorized client observation can close the attempt immediately after the server's check.

Run `make dev-env` and `docker compose up -d --build event-collector`. From services/event-collector run `go test -race ./...`. Refresh embedded schemas with `make sync-schemas`.
