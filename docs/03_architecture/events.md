# Examory — Learning Activity Event Contract

Status: draft **v0.2** (stage 1, October 2026). Owner: Kaiyrbekuly Aitbek. Consumers: analytics module (Shadiyarbek S.), ML modules (Akhanov Y.).

This document is the shared contract between the platform core and the AI modules. Changes go through a pull request reviewed by all three team members. Section 6 lists what changed between versions.

## 1. Envelope

Every message in Kafka is a JSON object with a common envelope and a type-specific `payload`.

```json
{
  "event_id": "018f3a2e-7c1b-7d6e-9a3f-4d2c1b0a9e8f",
  "event_type": "quiz.answered",
  "version": 1,
  "occurred_at": "2026-11-03T10:15:42.317Z",
  "producer": "quizzes",
  "actor": { "user_id": "u_123", "role": "student" },
  "context": { "course_id": "c_10", "group_id": "g_5", "session_id": "s_77" },
  "payload": { }
}
```

| Field | Type | Notes |
|-------|------|-------|
| `event_id` | UUID v7 | Unique, time-ordered; its timestamp component equals `occurred_at` at millisecond precision. Consumers deduplicate by this field. |
| `event_type` | string | `<domain>.<action>`, lower snake case |
| `version` | int | Schema version of the payload |
| `occurred_at` | RFC 3339 UTC, millisecond precision | Time of the action, not of publishing. Storage is always UTC; the web client displays times in `Asia/Almaty` by default. |
| `producer` | string | Service name |
| `actor` | object | Who performed the action; `role` ∈ `student`, `teacher`, `admin`, `system` |
| `context` | object | `course_id`, `group_id`, `session_id`; any of them may be `null` when not applicable. `session_id` is the exam-session context, **not** a join key for quiz attempts (use `payload.attempt_id`). |
| `payload` | object | Type-specific, documented below |

**Partition key:** `actor.user_id` for student actions (keeps one student's events ordered), `context.course_id` for course-level events.

**Ordering guarantee:** within one attempt, every `quiz.answered` precedes its `quiz.submitted`. Across attempts nothing is guaranteed; consumers order by `occurred_at` and `event_id`.

## 2. Topics

| Topic | Event types | Producer | Retention |
|-------|-------------|----------|-----------|
| `lms.users` | `user.created`, `user.logged_in`, `user.logged_out`, `group.updated` | `auth`, `users` | 30 d |
| `lms.courses` | `course.created`, `course.updated`, `material.viewed`, `assignment.published` | `courses`, `assignments` | 90 d |
| `lms.submissions` | `submission.created`, `submission.updated`, `file.uploaded` | `assignments`, `files` | 365 d |
| `lms.quizzes` | `quiz.started`, `quiz.answered`, `quiz.submitted` | `quizzes` | 365 d |
| `lms.grades` | `grade.assigned`, `risk.reviewed` | `grading` | 365 d |
| `lms.exams` | `exam.started`, `exam.ended`, `exam.agent_signal`, `client.focus_lost`, `client.tab_switched`, `client.paste` | `exam-sessions`, `event-collector` | 90 d |
| `ai.risk` | `risk.plagiarism`, `risk.ai_generated`, `risk.behavior_anomaly`, `risk.proctoring`, `risk.variant_leak` | ML modules; `risk.variant_leak` by the analytics module | 365 d |
| `ai.analytics` | `analytics.recommendation`, `analytics.group_risk`, `analytics.grade_suggestion` | analytics module | 90 d |

## 3. Platform events (produced by the core)

### `submission.created` (v1)

```json
{
  "submission_id": "sub_501",
  "assignment_id": "a_42",
  "attempt": 1,
  "kind": "text | file | code",
  "language": "ru | en | kk | go | python | ...",
  "text_ref": "minio://submissions/sub_501/extracted.txt",
  "files": [ { "file_id": "f_9", "name": "report.docx", "size": 182340, "sha256": "..." } ],
  "submitted_at": "2026-11-03T10:14:00Z",
  "deadline_at": "2026-11-03T23:59:00Z"
}
```

- `text_ref` points to the extracted plain text (UTF-8, newlines preserved). All character offsets in `risk.plagiarism` evidence refer to **this exact string**, counted in Unicode code points.
- `attempt` starts at 1. A submission is late when `submitted_at > deadline_at`; equality is on time.

### `quiz.started` (v1)

```json
{ "attempt_id": "att_88", "quiz_id": "q_7", "variant_id": "v_3", "question_count": 20, "time_limit_sec": 1800, "exam_session_id": "s_77 | null" }
```

### `quiz.answered` (v1)

One event per saved answer or answer change.

```json
{
  "attempt_id": "att_88",
  "question_id": "qst_15",
  "question_index": 4,
  "answer_hash": "sha256 of normalized answer",
  "is_change": false,
  "time_since_shown_ms": 8420,
  "time_since_attempt_start_ms": 215300
}
```

Counting conventions (fixed in v0.2):

| Field | Convention |
|-------|------------|
| `question_index` | Zero-based position of the question inside the variant. |
| `answer_hash` | SHA-256 of the canonical normalized answer (trimmed, case-folded, options sorted). Contains no student, attempt, question or label identifiers, so equal answers to the same `question_id` have equal hashes across students. Comparable only within the same `question_id`. |
| `is_change` | `false` for the first saved answer to a question in this attempt, `true` for every later change. |
| `time_since_shown_ms` | Milliseconds since the question was **first** displayed in this attempt, cumulative; on a change it includes the time of earlier answers. First-answer time is therefore the value of the event with `is_change = false`. |
| `time_since_attempt_start_ms` | Milliseconds since `quiz.started`, cumulative. |

### `quiz.submitted` (v1)

```json
{ "attempt_id": "att_88", "quiz_id": "q_7", "variant_id": "v_3", "score": 17, "max_score": 20, "duration_ms": 1012000, "answers_changed": 3 }
```

- Published exactly once per attempt, on submit or on server-side timeout; the same `attempt_id` never gets a second `quiz.submitted`.
- `score` and `max_score` are raw points; consumers compare attempts as `100 * score / max_score` with `max_score > 0`.
- `answers_changed` is the **number of change events** (`quiz.answered` with `is_change = true`), not the number of distinct changed questions.

### `grade.assigned` (v1)

```json
{ "target_type": "submission | attempt", "target_id": "sub_501", "grade": 85, "max_grade": 100, "rubric_id": "rub_3", "rubric_version": 2, "rubric_scores": [ { "criterion_id": "r_1", "score": 20 } ], "graded_by": "u_7" }
```

`rubric_id` and `rubric_version` are optional (added in v0.2) and identify the immutable rubric snapshot the grade was given against.

### `material.viewed` (v1)

```json
{ "material_id": "m_12", "module_id": "mod_3", "view_duration_ms": 94000, "completed": true }
```

Repeated views of a completed material publish `completed: true` again; consumers must not count completion twice.

### `exam.started` (v1)

```json
{ "exam_session_id": "s_77", "attempt_id": "att_88", "consent": { "webcam": true, "agent": true, "consent_version": "2026-10", "given_at": "2026-11-03T09:58:00Z" } }
```

Published only after affirmative consent is recorded. Refused consent produces no `exam.started` and no capture.

### `exam.ended` (v1)

```json
{ "exam_session_id": "s_77", "attempt_id": "att_88", "reason": "completed | timeout | withdrawn | interrupted", "ended_at": "2026-11-03T10:45:12Z", "video_ref": "minio://exams/s_77/video.mp4 | null" }
```

- `withdrawn`: the student withdrew consent; capture stopped, saved answers remain valid.
- `interrupted`: connectivity or capture failure; this is an interruption requiring review, never evidence of misconduct.
- `video_ref` is `null` when no webcam recording exists.

### `exam.agent_signal` (v1)

Produced by the exam client agent via `event-collector`, only while an exam session is active and consented.

```json
{
  "exam_session_id": "s_77",
  "signal": "suspicious_process | virtual_camera | remote_access | virtual_machine | screen_recorder",
  "details": { "process_name": "obs64.exe" },
  "confidence": 0.9
}
```

### `client.focus_lost` (v1)

```json
{ "attempt_id": "att_88", "exam_session_id": "s_77 | null", "duration_ms": 12000 }
```

### `client.tab_switched` (v1)

```json
{ "attempt_id": "att_88", "exam_session_id": "s_77 | null", "duration_ms": 4000 }
```

### `client.paste` (v1)

```json
{ "attempt_id": "att_88", "exam_session_id": "s_77 | null", "chars": 540 }
```

Client-side events are best-effort signals from the browser. They can be absent even when the behaviour occurred and must never be the only basis for a risk signal.

## 4. AI events (produced by the modules)

All `risk.*` events share this payload shape so the `grading` service and dashboards can render them uniformly:

```json
{
  "risk_id": "risk_301",
  "target_type": "submission | attempt | exam_session | variant",
  "target_id": "sub_501",
  "score": 0.82,
  "level": "low | medium | high",
  "model": { "name": "minhash-v1", "version": "0.1.0" },
  "explanation": "78% of 5-token shingles match submission sub_377 (same group)",
  "evidence": { }
}
```

- `score` is a **relative risk ranking in [0, 1]**, not a calibrated probability of misconduct. Each model documents how it maps its raw output into this range.
- `level` cutoffs are chosen by each model on validation data and frozen with the model version; `grading` never recomputes them.
- `explanation` is human-readable text for the teacher. Absence of a risk event is not evidence of integrity.

### `evidence` by event type

#### `risk.plagiarism`

```json
{
  "matches": [
    {
      "submission_id": "sub_377",
      "similarity": 0.78,
      "overlap_spans": [
        { "target": [1117, 3000], "source": [389, 2272], "token_count": 250 }
      ]
    }
  ]
}
```

`target` and `source` are half-open `[start, end)` code-point offsets into the extracted text (`text_ref`) of the target submission and of the matched source submission respectively; `token_count` is the number of normalized tokens in the span. `similarity` is the maximum verified local-window Jaccard, not whole-document overlap.

#### `risk.ai_generated`

```json
{ "features": { "perplexity": 11.4, "burstiness": 0.31, "model_ref": "gpt2-medium" } }
```

Signal only; never a verdict. Published only after an offline evaluation with documented false-positive rates by language.

#### `risk.behavior_anomaly`

```json
{
  "features": {
    "mean_answer_time_ms": 3120,
    "z_score": -2.4,
    "synchronous_with": [ "att_91", "att_93" ],
    "changes_fraction": 0.45
  }
}
```

- `target_type` is `attempt`, `target_id` the `attempt_id`.
- `z_score` is the standardized delta of the attempt's percentage score against the student's earlier comparable attempts (same quiz and variant), `null` when there is no history.
- `synchronous_with` lists peer `attempt_id` values submitted within ±5 s in the same quiz, variant and cohort.
- Additional feature keys may be added without a version bump; consumers ignore unknown keys.

#### `risk.proctoring`

```json
{ "segments": [ { "from_ms": 120000, "to_ms": 143500, "kind": "absent | multiple_faces | gaze_away | virtual_camera | deepfake" } ] }
```

- `target_type` is `exam_session`. `from_ms` / `to_ms` are relative to the start of the recording referenced by `exam.ended.video_ref`.

#### `risk.variant_leak`

Published by the **analytics module** (Shadiyarbek S.) to `ai.risk`.

```json
{
  "quiz_id": "q_7",
  "variant_id": "v_3",
  "recent_window": { "from": "2026-11-01T00:00:00Z", "to": "2026-11-08T00:00:00Z", "students": 14, "pass_rate": 0.86 },
  "baseline_window": { "from": "2026-10-02T00:00:00Z", "to": "2026-11-01T00:00:00Z", "students": 31, "pass_rate": 0.52 },
  "pass_threshold": 0.6,
  "scoring_version": 3
}
```

- `target_type` is `variant`, `target_id` the `variant_id`. This is a population-level hypothesis, never an accusation against individual students.
- Published only when both windows have at least 10 distinct students and the same quiz, variant, scoring version and pass threshold. Insufficient data produces no event.

### `risk.reviewed` (v1), produced by `grading`

```json
{ "risk_id": "risk_301", "target_type": "submission", "target_id": "sub_501", "decision": "confirmed | dismissed", "reviewed_by": "u_7", "comment": "..." }
```

- `target_type` and `target_id` are copied from the reviewed risk event (added in v0.2 so consumers need no join to find the target).
- A review never changes a grade by itself; grading actions are separate `grade.assigned` events.
- Reviewed decisions are the labelled dataset for measuring false positives. Unreviewed risks are unknown, not negatives.

### `analytics.*` (v1)

Common shape:

```json
{ "student_id": "u_123 | null", "course_id": "c_10", "kind": "recommendation | group_risk | grade_suggestion", "items": [ ], "explanation": "..." }
```

`student_id` is `null` for group-scoped outputs; the group then comes from `context.group_id`. `items[]` depends on `kind`:

| `kind` | `items[]` element |
|--------|-------------------|
| `recommendation` | `{ "material_id": "m_12", "topic_id": "t_4", "reason": "mastery 42% over 5 scored questions" }` |
| `group_risk` | `{ "student_id": "u_123", "reason": "quiz_average_below_60 \| overdue_assignments", "quiz_average": 48.5, "scored_quizzes": 4, "overdue_required": 2 }` |
| `grade_suggestion` | `{ "target_type": "submission", "target_id": "sub_501", "rubric_id": "rub_3", "rubric_version": 2, "criteria": [ { "criterion_id": "r_1", "score": 18, "max_score": 20, "reason": "..." } ], "suggested_total": 85 }` |

A `grade_suggestion` is advisory; only an explicit teacher save through `grading` produces `grade.assigned`.

## 5. Versioning rules

- Adding an optional field: same `version`.
- Removing or renaming a field, changing a type or a counting convention: `version + 1`; consumers must support the previous version for 30 days.
- Each payload will have a JSON Schema in `docs/03_architecture/schemas/` (to be added in stage 2).

## 6. Changelog

**v0.2 (October 2026)** — clarifications raised by the technical specification (LMS-5), dataset plan (LMS-10) and the synthetic generator (LMS-19); all additive, payload versions stay at 1:

- Envelope: `event_id` timestamp equals `occurred_at`; `context.session_id` is not an attempt join key; ordering guarantee within an attempt; UTC storage with `Asia/Almaty` display.
- `quiz.answered`: counting conventions for `question_index`, `time_since_shown_ms`, `time_since_attempt_start_ms`, `answer_hash`, `is_change`.
- `quiz.submitted`: exactly-once semantics, raw points, `answers_changed` counts change events.
- `grade.assigned`: optional `rubric_id`, `rubric_version`.
- `exam.started` and `exam.ended` split into separate payloads; `exam.ended` gains `reason` and `ended_at`; consent gains `consent_version`.
- `client.*` events get separate payloads with optional `exam_session_id`.
- `risk.*`: `score` defined as a relative ranking; `overlap_spans` format; `z_score` and `synchronous_with` definitions; proctoring segment timing; full `risk.variant_leak` payload and its producer.
- `risk.reviewed`: `target_type`, `target_id` added.
- `analytics.*`: `items[]` structure per `kind`; `student_id` null for group scope.

**v0.1 (October 2026)** — initial draft.
