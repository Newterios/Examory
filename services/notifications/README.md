# Notifications

The student inbox stores deadlines, grade availability and review decisions. GET /api/notifications/v1/notifications takes a student bearer token, optional limit (1–100) and cursor. The response contains items and next_cursor. A notice includes id, kind, course_id, target_type, target_id and occurred_at. Deadline adds due_at. Review adds confirmed or dismissed. An empty page may carry a next cursor when enrollment has changed since the deadlines were published.

| Input | Recipient and effect |
|---|---|
| lms.courses / assignment.published | A dated assignment creates one deadline notice for its course and group. Current course enrollment is checked on read. |
| lms.grades / grade.assigned | The assessed student's own inbox shows that a grade is available. |
| lms.grades / risk.reviewed | The student sees the decision. Comments, grading details and evidence are excluded. |
| lms.quizzes / quiz.started and lms.submissions / submission.created | Establish target ownership using the originating service and student actor. Ownership and notice events may arrive in either order. |

The worker checks the topic, producer, event type and version before mapping fields. PostgreSQL commits the filtered notice or immutable owner with the consumed event ID. Redelivery does not duplicate the notice. Without a matching owner and course, a private notification is invisible. No raw event body is persisted by this service. Deadline notices are group-scoped rather than copied into every student's row.

Run `make dev-env && make infra` followed by `docker compose up -d --build notifications`. From services/notifications run `go test -race ./...`; use `make integration` from the repository root for real PostgreSQL deduplication, cross-student privacy and late-owner tests.

## Known limitations

PostgreSQL is the local storage choice for FR-18. The service creates an in-app notice when a dated assignment is published. Email, push and timed reminders are outside this implementation. Older topic partitions that have expired cannot reconstruct a previously unseen target owner; those private notifications remain undisclosed.
