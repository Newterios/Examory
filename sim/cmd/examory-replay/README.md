# Replaying a simulator world

On stacks with the courses service, replay creates a course as the teacher and enrolls each simulated group as the administrator. The earlier M4 gateway returns HTTP 404 for the course route and uses a run-specific course label. Session failures, authorization errors and service outages stop the replay.

Run the command from `sim/` with the local Examory research stack and the bootstrap administrator's email and password in `E2E_ADMIN_EMAIL` and `E2E_ADMIN_PASSWORD`. The default origins are `http://127.0.0.1:8000` for the gateway and `http://127.0.0.1:8001` for quizzes. Set `E2E_GATEWAY_URL` or `E2E_QUIZZES_URL` to use different loopback ports. The command accepts only HTTP loopback origins and disables redirects and proxies.

```sh
go run ./cmd/examory-replay --mode parity --world /tmp/parity.csv \
  --keys /tmp/parity-keys.json --rate 20 --out /tmp/parity-replay.json

go run ./cmd/examory-replay --mode load --world /tmp/load.csv \
  --keys /tmp/load-keys.json --rate 20 --duration 15m --out /tmp/load-replay.json

go test -race ./cmd/examory-replay
```

Use a single fixed roster of 1–100 four-option questions, one source course, quiz and variant, up to 20 attempts per student, and attempts shorter than or equal to four hours. Generate the keys with the companion `examory-replay-keys` command using the same simulator settings. The keys JSON maps each original question ID to its correct option index, with indices starting at zero. The tool validates the complete source with the simulator's event validator, recomputes each final score and rejects truncated rosters. See [the replay analysis guide](../../../ml/replay/README.md) for world generation, the offline score comparison and load reporting.

Each invocation provisions a fresh course identifier, group memberships, synthetic students, a teacher, a replay system user and a single published quiz through the gateway. The course identifier namespaces the quiz within the local benchmark. It logs in with actual credentials and renews short-lived access tokens through the same login endpoint. Only the authenticated system user calls the direct research quiz API with the simulation headers. Platform IDs come from create responses. In particular, option IDs come from the authored quiz and continue to identify the same options when a student's display order is shuffled.

Administrative auth writes are spaced 250 ms apart (at most 240/minute), below both the gateway's 10/s limit and auth's 300/minute per-IP limit. During provisioning an explicit HTTP 429 is rejected before the write handler runs. The client honors `Retry-After` in seconds or HTTP-date form. Without a valid header, it waits 1, 2, 4, 8 and 16 seconds across successive rejections. Each operation allows up to five retries and two minutes of waiting. A larger server-requested delay ends the run. Transport failures, other HTTP errors and measured quiz operations are recorded without retries.

### Timing and throughput

Source submission time minus `duration_ms` determines attempt start. The first answer to a question minus `time_since_shown_ms` determines its first display. The command sends every distinct saved answer and revision, preserving the simulator's UTC timestamps to the millisecond. It checks the platform's returned timestamps, revision flags and final scores. There is no wall-clock shift. Simulation-mode attempts are exempt from the wall-clock timeout sweeper.

The scheduler gives a slot to each operation that emits `quiz.started`, `quiz.answered` or `quiz.submitted`. At 20/s the slots are 50 ms apart. A question display costs an additional HTTP request and shares the next event slot. Setup and login are excluded from the event count. Parity dispatches in virtual-time order, one operation at a time. Load schedules events against an absolute wall-clock timeline. Requests for one student execute in order, with at most eight concurrent requests across students. If the local stack falls behind, the manifest records the actual dispatch and acknowledgement times. Compare successful event-bearing dispatches to the scheduled slots and cross-check emitted events in the database.

A 15-minute load run uses exactly the first 18,000 event-bearing operations in the world's virtual-time order. The input may contain additional events. The prefix can end mid-attempt, and such attempts are marked `partial`. A world smaller than the requested event count fails preflight. A failed API operation is recorded with its HTTP status or transport error, subsequent operations on that attempt are marked skipped, independent attempts continue, and the process exits unsuccessfully. There are no automatic retries of ambiguous writes.

### Manifest

Provide a new output filename for each invocation. The file is created with mode 0600 on POSIX, and is filled on completion, caught cancellation or ordinary failure. A forced process kill can leave an empty file. It records the SHA-256 digests of the original CSV and key file, run ID, mappings for courses, groups, users, quizzes, variants, questions and attempts, and UTC wall-clock start and end of measured replay. Each question maps all four original option indices to live IDs and hashes. For a single-choice platform answer the hash is SHA-256 of the trimmed lowercase option UUID. Each attempt records the live identifiers and wall times around submission. An entry in `actions` records the operation type, intended slot, actual dispatch and HTTP completion. A transport error has status zero. The manifest contains synthetic IDs and omits account credentials. Raw manifests belong outside the repository.

Use the offline analyser for feature and score parity. Use live database receipts for submit-to-publication latency. HTTP response time alone measures the quiz endpoint and ends before the risk pipeline has necessarily produced a signal.
