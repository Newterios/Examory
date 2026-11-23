# Assignments service

A teacher assigned to a course publishes an assignment for an enrolled group. Students in that group upload one to four text files through the files service and submit their file IDs. The submission response is a receipt with the server timestamp, attempt number, late status, sizes and SHA-256 digests.

| Method and path under `/api/assignments/v1` | Purpose |
|---|---|
| POST `/assignments` | Publish an assignment as a teacher |
| GET `/assignments`, GET `/assignments/{id}` | Read assignments in the caller's current scope |
| POST `/assignments/{id}/submissions` | Submit `{"file_ids":["<completed file UUID>"]}` with an `Idempotency-Key` |
| GET `/assignments/{id}/submissions` | Read receipts; students see their own, teachers see those in scope |

Example creation payload:

```json
{"course_id":"<course UUID>","group_id":"<group UUID>","title":"Source analysis","instructions":"Upload your UTF-8 essay.","deadline_at":"2026-12-15T13:00:00Z","max_attempts":2,"max_bytes":65536,"accepted_types":["text/plain","text/markdown"],"kind":"text","language":"en"}
```

The allowed maximum is 2 MiB including one LF separator between files; accepted work is concatenated in request order without rewriting Unicode or line endings. A submission after the deadline is accepted and marked late by the server clock. The stable ID derives from the assignment, student and idempotency key. An identical retry returns the original receipt; a changed request using that key returns 409. Concurrent requests share the student's per-assignment attempt limit. A completed file must belong to that student and match the assignment limits. If the files or courses service is unavailable the write fails and the client can retry.

Assignments writes `minio://submissions/{submission_id}/extracted.txt` through the private files endpoint and records the receipt and `submission.created` outbox entry together. A database failure after object creation can leave an unreferenced immutable object. Retrying with the same content reuses it. Kafka delivery and plagiarism scoring run after the receipt commits. The event contains a reference and digests; work text remains in private object storage.

Configure `ASSIGNMENTS_DATABASE_URL`, `ASSIGNMENTS_JWKS_URL`, `ASSIGNMENTS_KAFKA_BROKERS`, `COURSES_INTERNAL_URL`, `COURSES_INTERNAL_TOKEN`, `FILES_INTERNAL_URL`, and `FILES_INTERNAL_TOKEN`. From this directory run `go test ./...`; to exercise real PostgreSQL receipt concurrency, set `IT_PG_ADMIN_URL` and run `go test -tags integration -count=1 -run TestReceiptAndOutboxInPostgres ./internal/service`. This test uses an in-process files HTTP double and validates the emitted event against the repository schema.
