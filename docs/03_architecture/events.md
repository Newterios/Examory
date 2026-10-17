# Examory — Learning Activity Event Contract

Status: draft v0.1 (stage 1, October 2026). Owner: Kaiyrbekuly Aitbek. Consumers: analytics module (Shadiyarbek S.), ML modules (Akhanov Y.).

This document is the shared contract between the platform core and the AI modules. Changes go through a pull request reviewed by all three team members.

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
| `event_id` | UUID v7 | Unique, time-ordered, used for idempotent consumers |
| `event_type` | string | `<domain>.<action>`, lower snake case |
| `version` | int | Schema version of the payload |
| `occurred_at` | RFC 3339 UTC | Time of the action, not of publishing |
| `producer` | string | Service name |
| `actor` | object | Who performed the action; `role` ∈ student, teacher, admin, system |
| `context` | object | Optional course / group / session identifiers for partitioning and joins |
| `payload` | object | Type-specific, documented below |

**Partition key:** `actor.user_id` for student actions (keeps one student's events ordered), `context.course_id` for course-level events.

## 2. Topics

| Topic | Event types | Retention |
|-------|-------------|-----------|
| `lms.users` | `user.created`, `user.logged_in`, `user.logged_out`, `group.updated` | 30 d |
| `lms.courses` | `course.created`, `course.updated`, `material.viewed`, `assignment.published` | 90 d |
| `lms.submissions` | `submission.created`, `submission.updated`, `file.uploaded` | 365 d |
| `lms.quizzes` | `quiz.started`, `quiz.answered`, `quiz.submitted` | 365 d |
| `lms.grades` | `grade.assigned`, `risk.reviewed` | 365 d |
| `lms.exams` | `exam.started`, `exam.ended`, `exam.agent_signal`, `client.focus_lost`, `client.tab_switched`, `client.paste` | 90 d |
| `ai.risk` | `risk.plagiarism`, `risk.ai_generated`, `risk.behavior_anomaly`, `risk.proctoring`, `risk.variant_leak` | 365 d |
| `ai.analytics` | `analytics.recommendation`, `analytics.group_risk`, `analytics.grade_suggestion` | 90 d |

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

### `quiz.started` (v1)

```json
{ "attempt_id": "att_88", "quiz_id": "q_7", "variant_id": "v_3", "question_count": 20, "time_limit_sec": 1800, "exam_session_id": "s_77 | null" }
```

### `quiz.answered` (v1)

One event per answer or answer change.

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

### `quiz.submitted` (v1)

```json
{ "attempt_id": "att_88", "quiz_id": "q_7", "variant_id": "v_3", "score": 17, "max_score": 20, "duration_ms": 1012000, "answers_changed": 3 }
```

### `grade.assigned` (v1)

```json
{ "target_type": "submission | attempt", "target_id": "sub_501", "grade": 85, "max_grade": 100, "rubric_scores": [ { "criterion_id": "r_1", "score": 20 } ], "graded_by": "u_7" }
```

### `material.viewed` (v1)

```json
{ "material_id": "m_12", "module_id": "mod_3", "view_duration_ms": 94000, "completed": true }
```

### `exam.started` / `exam.ended` (v1)

```json
{ "exam_session_id": "s_77", "attempt_id": "att_88", "consent": { "webcam": true, "agent": true, "given_at": "..." }, "video_ref": "minio://exams/s_77/video.mp4 | null" }
```

### `exam.agent_signal` (v1)

Produced by the exam client agent via `event-collector`.

```json
{
  "exam_session_id": "s_77",
  "signal": "suspicious_process | virtual_camera | remote_access | virtual_machine | screen_recorder",
  "details": { "process_name": "obs64.exe" },
  "confidence": 0.9
}
```

### `client.focus_lost` / `client.tab_switched` / `client.paste` (v1)

```json
{ "attempt_id": "att_88", "duration_ms": 12000, "chars": 540 }
```

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
  "explanation": "78% of 5-gram shingles match submission sub_377 (same group)",
  "evidence": { }
}
```

`evidence` is type-specific:

| Event | `evidence` |
|-------|------------|
| `risk.plagiarism` | `matches: [{ submission_id, similarity, overlap_spans }]` |
| `risk.ai_generated` | `features: { perplexity, burstiness, ... }` |
| `risk.behavior_anomaly` | `features: { mean_answer_time_ms, z_score, synchronous_with: [attempt_id] }` |
| `risk.proctoring` | `segments: [{ from_ms, to_ms, kind: "absent | gaze_away | virtual_camera | deepfake" }]` |
| `risk.variant_leak` | `variant_id, pass_rate_before, pass_rate_after, window` |

### `risk.reviewed` (v1), produced by `grading`

```json
{ "risk_id": "risk_301", "decision": "confirmed | dismissed", "reviewed_by": "u_7", "comment": "..." }
```

Reviewed decisions are the labelled dataset for measuring false positives.

### `analytics.*` (v1)

```json
{ "student_id": "u_123", "course_id": "c_10", "kind": "recommendation | group_risk | grade_suggestion", "items": [ ], "explanation": "..." }
```

## 5. Versioning rules

- Adding an optional field: same `version`.
- Removing or renaming a field, changing a type: `version + 1`; consumers must support the previous version for 30 days.
- Each payload will have a JSON Schema in `docs/03_architecture/schemas/` (to be added in stage 2).
