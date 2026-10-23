<!-- docs/02_requirements/technical_specification.md -->
# Examory Technical Specification

Version: 1.1 | Date: 2026-10-23 | Owner: Shadiyarbek Syrym, SE-2424, AITU

Change log: v1.1 — after supervisor review of 23 Oct 2026: FR-52 moved to *should* with a fixture, FR-57 and FR-58 added (students without Windows; agent allowlist of checks), AC-10 adjusted, §6.1 lists the agent checks. v1.0 — first complete version.

Project: Development of an AI-Powered LMS Platform for Academic Integrity, Personalized Learning Analytics, and Anomaly Detection.

## 1. Purpose and scope

Examory supports university teaching, assessment, and timely learning support. Its LMS records learning activity as Apache Kafka events; connected services return explainable risk signals and personalized analytics. A risk signal is evidence for review, never a finding of misconduct. Teachers retain authority over academic decisions and published grades.

The MVP covers courses, assignments, quizzes, grading, notifications, files, academic integrity, personalized analytics, anomaly detection, and consent-based proctoring. It includes student and teacher dashboards, rubric-based grade suggestions, and possible quiz variant leakage alerts. This specification defines observable behavior, permissions, data contracts, and acceptance conditions. ML algorithms, model training, inference internals, core service implementation, and Windows client agent internals belong to other team members and are outside this document.

The fixed integration environment is a Next.js Web client and Windows client agent, Nginx API gateway, Go services `auth`, `users`, `courses`, `assignments`, `quizzes`, `grading`, `notifications`, `files`, `exam-sessions`, and `event-collector`, Apache Kafka, MinIO, PostgreSQL, MongoDB, and Redis. The personalized analytics module uses Node.js/Express, with Python/pandas for calculations and MongoDB/PostgreSQL for aggregates; dashboards use Chart.js/Recharts. These are integration constraints, not a redesign of the core.

## 2. Users and roles

Permissions are enforced by the backend for every object and action. A teacher is limited to assigned courses and groups; a student is limited to enrolled courses and their own records. An admin manages institutional configuration but receives no academic decision authority solely from that role. A separately assigned teacher role is required for grading or risk review. ML Services are service identities, not interactive users.

| Capability | Student | Teacher | Admin | ML Services |
|---|---|---|---|---|
| View published courses and materials | Enrolled courses | Assigned courses | Administrative metadata | Minimum authorized reference data |
| Manage course content and assessment settings | No | Assigned courses | Assign ownership and access | No |
| Manage users, roles, groups, enrollment | Own profile only | Read assigned rosters | Yes | No |
| Submit assignments and take quizzes | Own work | Preview authored content | No | No |
| Read submissions, answers, and grades | Own released records | Assigned courses | No by default | Minimum processing scope |
| Create rubrics and publish grades | No | Assigned courses | No | Suggestions only |
| View personal learning analytics | Own | Assigned students | Service health only | Produce authorized outputs |
| View group analytics and variant alerts | No | Assigned groups | No by default | Produce authorized outputs |
| Inspect raw risk evidence | No by default | Assigned cases with explicit access | No by default | Required processing only |
| Confirm or dismiss risk signals | No | Assigned cases | No | No |
| Give or withdraw proctoring consent | Self only | View consent status | Manage policy version | No |
| Manage retention, audit, and service health | No | Read own review history | Yes; sensitive content excluded | Own operational status |

Released review outcomes may be visible to the affected student without exposing other students, restricted source material, or raw proctoring evidence.

## 3. Functional requirements

`must` is required for MVP acceptance; `should` is a planned improvement; `could` is optional. Every row describes an externally testable capability.

### 3.1 LMS core

| ID | Priority | Requirement |
|---|---|---|
| FR-1 | must | `auth` shall authenticate users, enforce role access, and invalidate a signed-out or revoked session. |
| FR-2 | must | `users` shall let an admin create, deactivate, and assign roles to users without deleting their academic history. |
| FR-3 | must | An admin shall manage `Group` membership, course enrollment, and teacher assignments; access shall follow current membership. |
| FR-4 | must | `courses` shall let an assigned teacher create, update, publish, and archive a `Course` and its ordered `Module` records. |
| FR-5 | must | A teacher shall publish `Material` records and attached files; enrolled students shall see only released content. |
| FR-6 | must | Material interaction shall produce `material.viewed` with `material_id`, `module_id`, `view_duration_ms`, and `completed`; repeated views shall not multiply completion. |
| FR-7 | must | `assignments` shall let a teacher configure an `Assignment` with instructions, deadline, accepted file types, size limit, and allowed attempts. |
| FR-8 | must | `files` shall validate authorized uploads and issue access-controlled file references; students shall receive a clear error for a rejected upload. |
| FR-9 | must | `assignments` shall accept a student's valid file references, persist a `Submission`, return a receipt, and publish `submission.created` without waiting for ML results. |
| FR-10 | must | The receipt and teacher view shall show submission time, attempt number, and late status using the server timestamp and configured deadline. |
| FR-11 | must | `quizzes` shall let teachers maintain a `Question` bank, question-to-topic mapping, scoring rules, and `Variant` question sets within a `Quiz`. |
| FR-12 | must | `quizzes` shall create an authorized `Attempt` with a fixed `variant_id`, server-controlled time limit, and `quiz.started`. |
| FR-13 | must | `quizzes` shall save each `Answer` and accepted answer change, acknowledge persistence, and publish `quiz.answered` with the brief's timing and change fields. |
| FR-14 | must | `quizzes` shall finalize an attempt once on submit or timeout and publish `quiz.submitted` with `score`, `max_score`, and `duration_ms`; retries shall not create another final attempt. |
| FR-15 | must | A student shall see their released quiz result, grades, and feedback; unreleased answers and marking keys shall remain inaccessible. |
| FR-16 | must | A teacher shall create a versioned `Rubric` with criterion identifiers, descriptions, and score bounds and associate it with an assessment. |
| FR-17 | must | `grading` shall let a teacher record or revise a `Grade`, validate criterion and total bounds, retain revision history, and publish `grade.assigned`. |
| FR-18 | must | `notifications` shall show authorized in-app deadline, published-grade, and review-outcome notifications without exposing restricted evidence. |
| FR-19 | must | All brief-listed learning, risk, and analytics events shall preserve the common event envelope and stable target references; invalid or unsupported events shall be reported without corrupting visible records. |
| FR-20 | should | A teacher shall export an access-scoped gradebook with course, student, assessment, and grade identifiers. |

### 3.2 Academic integrity

| ID | Priority | Requirement |
|---|---|---|
| FR-21 | must | Authorized ML integrity processing shall receive `submission.created` and access only the referenced text or code needed for that submission. |
| FR-22 | must | `grading` shall consume `risk.plagiarism` and persist the supplied risk identifier, target, score, level, explanation, and evidence beside the corresponding work. |
| FR-23 | should | Supported AI-generated-text indications shall appear as distinct explained `risk.ai_generated` signals. |
| FR-24 | must | Teachers shall inspect risk category, severity, explanation, relevant excerpts or evidence links, processing time, and review state for their assigned cases. |
| FR-25 | must | A teacher shall confirm or dismiss a signal with a reason; `grading` shall record reviewer and timestamp and publish `risk.reviewed`. |
| FR-26 | must | Neither receipt nor confirmation of a risk signal shall automatically change a grade, impose a penalty, or declare misconduct; any academic action requires a separate teacher decision. |
| FR-27 | must | Students shall see released review outcomes concerning their work and be able to submit a clarification for teacher consideration without viewing another student's evidence. |
| FR-28 | must | Processing pending, failed, unavailable, and completed-with-no-signal states shall be distinguishable; missing ML output shall never be displayed as proof of integrity. |

### 3.3 Personalized analytics

| ID | Priority | Requirement |
|---|---|---|
| FR-29 | must | The personalized analytics module shall consume `lms.quizzes`, `lms.submissions`, `lms.grades`, and `lms.courses`, using the brief's event types and access-scoped reference data. |
| FR-30 | must | A student dashboard shall show completed versus eligible published materials per course and module, based on `material.viewed` and the current material catalog. |
| FR-31 | must | The dashboard shall show score trends using `quiz.submitted` and `grade.assigned`, normalize scores only for positive maxima, and distinguish missing results from zero. |
| FR-32 | must | Weak-topic summaries shall combine authoritative scored answers from `quizzes` with question-to-topic mappings; `quiz.answered` timing alone shall not establish mastery or correctness. |
| FR-33 | must | The module shall publish `analytics.recommendation`; each recommendation shall identify accessible learning items and explain the related topic or observed learning need. |
| FR-34 | must | The student dashboard shall show upcoming and overdue assignment deadlines from current `assignments` data, reconciled with accepted submissions. |
| FR-35 | must | A teacher dashboard shall show group performance, completion, missing work, and learning-support candidates scoped to the selected course, group, and time period. |
| FR-36 | must | The module shall publish `analytics.group_risk` with reasons and supporting metrics; a learning-support designation shall be visually separate from academic-integrity signals. |
| FR-37 | must | The module shall publish `analytics.grade_suggestion` for an eligible work item, with rubric version, criterion scores, suggested total, and an explanation; insufficient evidence shall produce no suggestion. |
| FR-38 | must | A teacher shall inspect, edit, accept, or reject a grade suggestion; only an explicit save through `grading` shall publish a final `grade.assigned`. |
| FR-39 | must | Analytics shall deduplicate by `event_id`, tolerate delayed delivery, and reconcile authoritative revisions without double-counting submissions, attempts, or grade history. |
| FR-40 | could | A student shall dismiss a recommendation or mark it helpful without changing their assessment result. |

### 3.4 Anomaly detection

| ID | Priority | Requirement |
|---|---|---|
| FR-41 | must | ML anomaly processing shall consume `quiz.started`, `quiz.answered`, and `quiz.submitted` and return explainable `risk.behavior_anomaly` signals for relevant attempts. |
| FR-42 | must | Teachers shall see the observations underlying unusually fast answers, synchronized submissions, or abrupt performance changes when supplied by ML, without being shown an automatic misconduct verdict. |
| FR-43 | must | Personalized analytics shall compare pass rates for the same `variant_id` across comparable time windows and publish `risk.variant_leak` candidates with counts, rates, and an explanation. |
| FR-44 | must | Variant leakage analysis shall suppress a conclusion when samples are insufficient or quiz, variant, pass threshold, or scoring conditions are not comparable. |
| FR-45 | must | `grading` shall expose behavior and variant leakage signals in the teacher review workflow and publish `risk.reviewed` on a teacher's confirm or dismiss action. |
| FR-46 | should | Teachers shall filter anomaly cases by event category, course, group, time window, severity, and review state. |

### 3.5 Proctoring and exam sessions

| ID | Priority | Requirement |
|---|---|---|
| FR-47 | must | A teacher shall configure an `ExamSession` through `exam-sessions` for an authorized quiz, participant, time window, and declared monitoring policy. |
| FR-48 | must | Before capture, students shall receive a notice describing purpose, webcam and client agent data, access, retention, withdrawal, and the alternative assessment contact. |
| FR-49 | must | `exam-sessions` shall record affirmative consent, policy version, permitted capture scopes, and time before starting monitoring or publishing `exam.started`; refusal shall prevent capture. |
| FR-50 | must | A pre-exam readiness check shall show webcam permission and Windows client agent connection status; a failure shall produce an actionable message and no hidden capture. |
| FR-51 | must | While consent remains valid and the session is active, webcam evidence and allowlisted client agent observations shall be associated with that session; `event-collector` shall publish `exam.agent_signal`. |
| FR-52 | should | ML proctoring shall consume authorized session observations and return `risk.proctoring`; `grading` shall store the signal for teacher review with explanation and access-controlled evidence. Until the model is deployed and the university approves video capture, acceptance uses a fixture `risk.proctoring` event; consent and session lifecycle (FR-47–FR-51, FR-53–FR-55) remain `must` and are verified with test accounts. |
| FR-53 | must | Web and client agent shall display monitoring status and allow withdrawal; withdrawal shall stop capture and end the monitored session without automatically assigning a failing grade. |
| FR-54 | must | Normal completion, timeout, or withdrawal shall finalize the session once, stop webcam and client agent capture, and publish `exam.ended`; closed sessions shall reject new capture. |
| FR-55 | must | Lost connectivity or unavailable proctoring shall be shown as an interruption requiring review; it shall not itself establish cheating, and already saved quiz answers shall remain saved. |
| FR-56 | should | Teachers shall inspect a chronological session evidence timeline, including interruptions and risk reviews, within the applicable retention period. |
| FR-57 | must | A student whose device cannot run the Windows client agent shall be offered an alternative path configured per exam by the teacher: webcam-only session, an in-person or supervised session, or an alternative assessment; the chosen path shall be recorded in the exam session and shall not penalize the student. |
| FR-58 | must | The client agent shall perform only the checks listed in §6.1 (allowlist); the consent notice shall show this exact list; the agent shall run only while the exam session is active, shall not read files or keystrokes, and shall remove itself completely after the session. |

## 4. Event and data contract boundaries

Every event uses `event_id`, `event_type`, `version`, `occurred_at`, `producer`, `actor {user_id, role}`, `context {course_id, group_id, session_id}`, and `payload`. Store timestamps in UTC; display deadlines and time windows in the selected institutional timezone, initially `Asia/Almaty` (Astana). Context identifiers are optional when inapplicable. Event types and Kafka topics are different identifiers.

| Event | Payload defined in `docs/03_architecture/events.md` |
|---|---|
| `material.viewed` | `material_id`, `module_id`, `view_duration_ms`, `completed` |
| `submission.created` | `submission_id`, `assignment_id`, `attempt`, `kind`, `language`, `text_ref`, `files[{file_id, name, size, sha256}]`, `submitted_at`, `deadline_at` |
| `quiz.started` | `attempt_id`, `quiz_id`, `variant_id`, `question_count`, `time_limit_sec`, `exam_session_id` (`null` when inapplicable) |
| `quiz.answered` | `attempt_id`, `question_id`, `question_index`, `answer_hash`, `is_change`, `time_since_shown_ms`, `time_since_attempt_start_ms` |
| `quiz.submitted` | `attempt_id`, `quiz_id`, `variant_id`, `score`, `max_score`, `duration_ms`, `answers_changed` |
| `grade.assigned` | `target_type` (`submission` or `attempt`), `target_id`, `grade`, `max_grade`, optional `rubric_id`, optional `rubric_version`, `rubric_scores[{criterion_id, score}]`, `graded_by` |
| ML `risk.*`, including `risk.variant_leak` | `risk_id`, `target_type` (`submission`, `attempt`, `exam_session`, or `variant`), `target_id`, `score`, `level`, `model{name, version}`, `explanation`, `evidence` |
| `analytics.recommendation`, `analytics.group_risk`, `analytics.grade_suggestion` | `student_id`, `course_id`, `kind`, `items[]`, `explanation` |
| `risk.reviewed` | `risk_id`, `target_type`, `target_id`, `decision` (`confirmed` or `dismissed`), `reviewed_by`, `comment`. This review event is distinct from the ML risk payload above. |
| `exam.started` | `exam_session_id`, `attempt_id`, `consent{webcam, agent, consent_version, given_at}` |
| `exam.ended` | `exam_session_id`, `attempt_id`, `reason` (`completed`, `timeout`, `withdrawn`, or `interrupted`), `ended_at`, `video_ref` |
| `exam.agent_signal` | `exam_session_id`, `signal`, `details`, `confidence` |

| Kafka topic | Event types |
|---|---|
| `lms.users` | `user.created`, `user.logged_in`, `user.logged_out`, `group.updated` |
| `lms.courses` | `course.created`, `course.updated`, `material.viewed`, `assignment.published` |
| `lms.submissions` | `submission.created`, `submission.updated`, `file.uploaded` |
| `lms.quizzes` | `quiz.started`, `quiz.answered`, `quiz.submitted` |
| `lms.grades` | `grade.assigned`, `risk.reviewed` |
| `lms.exams` | `exam.started`, `exam.ended`, `exam.agent_signal`, `client.focus_lost`, `client.tab_switched`, `client.paste` |
| `ai.risk` | `risk.plagiarism`, `risk.ai_generated`, `risk.behavior_anomaly`, `risk.proctoring`, `risk.variant_leak` |
| `ai.analytics` | `analytics.recommendation`, `analytics.group_risk`, `analytics.grade_suggestion` |

Consumers resolve missing quiz, course, and topic metadata through authorized source-service reads. The `items[]` structure by `kind` and `student_id = null` for group-scoped outputs are defined in `docs/03_architecture/events.md` §4.

A `Grade` targets exactly one `Submission` or `Attempt`. A `RiskSignal` targets exactly one `Submission`, `Attempt`, `ExamSession`, or `Variant`; variant signals describe a population pattern, not an accusation against every participant. Course and group permissions apply to all linked evidence. Rubric versions and assessment scoring definitions remain traceable after edits. The class diagram is a logical cross-service model, not a shared-database design.

## 5. Non-functional requirements

All numeric targets below are proposed MVP targets measured on a documented staging environment with 100 concurrent authenticated users, 20 accepted events/second, and at least 10,000 historical learning events. They are not measured results.

| ID | Category | Requirement and verification |
|---|---|---|
| NFR-1 | Performance | Ordinary non-upload API operations shall have p95 response time <=500 ms under the stated load; verify with a 15-minute mixed LMS workload. |
| NFR-2 | Performance | Dashboard data shall load within 2 seconds at p95; accepted learning events shall appear in aggregates within 60 seconds at p95 under normal operation. Measure event-to-visible latency. |
| NFR-3 | Performance | A valid received `risk.*` or `analytics.*` result shall become visible to its authorized user within 10 seconds at p95; ML inference time is measured separately and is not assumed here. |
| NFR-4 | Security | All external and service data transport shall be encrypted; stored sensitive data and backups shall use encryption and controlled keys. Verify configuration and unauthorized connection rejection. |
| NFR-5 | Security | All object reads and writes shall enforce role, enrollment, ownership, and course scope; negative access tests shall cover other students' work, evidence, and administrative actions. |
| NFR-6 | Security | Uploads shall enforce configured type and size allowlists; evidence URLs shall expire within 10 minutes and shall not grant access outside the intended recipient scope. Verify rejected inputs and expired links. |
| NFR-7 | Security | Credentials shall not appear in source, events, exports, or logs; authentication shall resist repeated failed attempts through rate limiting. Inspect representative logs and abuse tests. |
| NFR-8 | Privacy | Capture shall start only after recorded consent and stop within 5 seconds of withdrawal or ending under normal connectivity; an offline agent shall stop locally on withdrawal or authorization expiry. |
| NFR-9 | Privacy | Events shall carry only necessary identifiers and references; raw webcam data shall not be embedded in general Kafka payloads or dashboard responses. Verify schema samples and access boundaries. |
| NFR-10 | Privacy | Configured expiry shall remove eligible records and references from live stores within 24 hours; backup copies shall expire within 30 days, and restored data shall reapply deletion records. |
| NFR-11 | Availability | The staging service shall target 99% monthly availability excluding announced maintenance; monitor authenticated course access and submission receipt independently of ML availability. Admins shall see service health, consumer lag, and last successful processing time for each analytics input topic. |
| NFR-12 | Resilience | Duplicate delivery, delayed events, or unavailable ML shall not lose acknowledged work or duplicate visible records; verify replay and recovery scenarios. Pending processing shall remain visible. |
| NFR-13 | Recovery | Backup and restore shall meet a proposed RPO of 24 hours and RTO of 4 hours for a full storage disaster; verify one documented staging restoration. Ordinary process restarts shall preserve acknowledged writes. |
| NFR-14 | Usability | Core student and teacher workflows shall support keyboard navigation, visible focus, labeled controls, text alternatives to charts, and status indicators that do not rely only on color; verify manually at 200% zoom. |
| NFR-15 | Usability | Dashboards shall remain usable at 360 px and 1440 px widths, show explicit loading/empty/error/stale states, and display the timezone for deadlines; verify both layouts. |
| NFR-16 | Auditability | Grade changes, role changes, consent, evidence access, and risk reviews shall record actor, target, action, timestamp, and outcome; only authorized audit users may read these records. |

## 6. Constraints and assumptions

### 6.1 Consent, human review, and personal data

Proctoring consent is specific to webcam and client agent monitoring, records the notice version, and is obtained before collection. Refusal or withdrawal presents the institutional alternative assessment or teacher contact workflow; it does not automatically fail the student or guarantee that an alternative has already been granted. The MVP assumes adult participants; any real deployment involving minors must establish the appropriate consent process before collection. Collection is limited to the disclosed webcam evidence and allowlisted exam-related observations; unrelated personal files, passwords, and unrestricted desktop collection are excluded.

**Client agent allowlist of checks (FR-58).** The agent reports only: (1) names of running processes matched against a published list of remote-access tools, screen recorders and known cheating helpers; (2) presence of virtual-camera drivers or devices; (3) indicators that the OS runs inside a virtual machine; (4) active remote-desktop sessions; (5) active screen-capture sessions. It does not capture screenshots, keystrokes, file contents, browser history or network traffic. The list is shown verbatim on the consent screen and versioned with the consent notice.

The university must establish the permitted purposes, access policy, consent process, storage arrangements, and data-subject request procedure against Kazakhstan's Law No. 94-V, *On Personal Data and their Protection*, before real student data is used. Kazakhstan-hosted storage is the project deployment assumption; any cross-border transfer requires separate institutional review. These are project constraints, not a legal compliance certification. Reference: [Adilet, Law No. 94-V](https://old.adilet.zan.kz/rus/docs/Z1300000094), checked 2026-10-18.

Signals, learning-support labels, AI-text indications, and grade suggestions are provisional. A teacher reviews the explanation, relevant context, and student clarification. Confirming a signal records that review; academic sanctions and grade changes require a separate authorized action. No detector accuracy, perfect cheating detection, or causal explanation of learning outcomes is promised.

### 6.2 Proposed retention schedule

These durations are configurable project assumptions, not statutory periods. The university must approve the schedule before production. Deletion also applies to derived copies, evidence links, and downstream aggregates where they identify an individual; only irreversibly anonymous statistics may remain.

| Data category | Proposed retention | End-of-period action |
|---|---|---|
| Raw webcam and client agent evidence | 7 days after `exam.ended` | Delete evidence and invalidate links; retain a non-content expiry marker. |
| Derived risk signals and pseudonymous learning events/analytics | 180 days after course end | Delete or irreversibly anonymize; pseudonymization alone is not anonymization. |
| Submissions, answers, grades, associated consent and audit records | 1 year after course end | Delete or archive only under a separately documented institutional obligation. |
| Backup copies of expired records | At most 30 further days | Expire backups and reapply deletion records on restore. |

An active appeal may justify a specifically authorized hold with an owner, reason, scope, and expiry date. A hold shall not silently extend retention for unrelated records. Admins manage expiry and requests without receiving default permission to inspect evidence or make academic decisions.

### 6.3 Metric and contract assumptions

- Course progress is completed eligible published materials divided by all eligible published materials. No materials means “No materials,” not 0% mastery.
- Quiz and grade percentages use `100 * score / max_score` or `100 * grade / max_grade` only where the denominator is positive. Quiz scores and later grades are separate views of the same assessment, not additive points.
- A weak topic initially means <60% of available scored points over at least 3 distinct scored questions in that topic. Missing topic mapping or scored answers means insufficient data.
- A learning-support candidate initially means a mean normalized quiz score <60% across at least 3 distinct quizzes, or at least 2 overdue required assignments. This designation is separate from misconduct risk.
- Variant comparison uses the latest eligible completed attempt per student per window, a recent 7-day window and the preceding 30-day baseline, the configured quiz pass threshold, and at least 10 distinct students per window. A rise of at least 20 percentage points creates a review candidate. Recent-window students are excluded from baseline membership to avoid overlapping cohorts; counts are recomputed. Quiz/variant/scoring versions must match. Thresholds are configurable and are not a validated detector.
- Deadline catalogs, rosters, rubric definitions, scored answers, and question-topic mappings come from scoped source-service reads because the listed events do not contain all required fields. No additional canonical event names are invented.
- The `quiz.answered` counting conventions are fixed in `docs/03_architecture/events.md` §3: `question_index` is zero-based, `time_since_shown_ms` is cumulative since first display, and `quiz.submitted.answers_changed` counts events with `is_change = true`.
- Uploads initially allow PDF, TXT, and ZIP up to 20 MB; exact allowed code archive content is configured per assignment. Dates use UTC storage and initial `Asia/Almaty` (Astana) display. Measurement targets and retention periods are proposed defaults.

## 7. Acceptance criteria for MVP

Acceptance uses synthetic or consented test data, two independent courses/groups, student/teacher/admin accounts, text and code submissions, sufficient quiz history for the stated windows, and at least one rubric and exam session. Contract fixtures verify integration when a dependent ML service is unavailable; a fixture-only demonstration must be reported as such and does not establish ML accuracy or full live integration.

| ID | Requirements | Pass condition |
|---|---|---|
| AC-1 | FR-1–FR-5, FR-15; NFR-5 | Admin configures users/groups/enrollment; teacher publishes a course; enrolled student sees it. Cross-course, cross-student, and admin-only-role grading/evidence requests are rejected. |
| AC-2 | FR-6–FR-10, FR-19 | Student views material and submits allowed text/code files; the correct events and one receipt appear. Invalid upload is rejected, a late submission is labeled, and duplicate delivery does not create duplicate progress or work. |
| AC-3 | FR-11–FR-14 | A timed variant attempt emits `quiz.started`, answer changes emit `quiz.answered`, and submit/timeout emits one logical `quiz.submitted`. Repeated submit does not change the finalized answers or create a second attempt. |
| AC-4 | FR-16–FR-18, FR-37–FR-38 | Teacher receives an explained rubric suggestion, edits criterion scores, saves a bounded grade, and emits `grade.assigned`. The student sees the released result and notification; receiving a suggestion alone changes no grade. |
| AC-5 | FR-21–FR-28 | Text/code integrity results reach `grading`; teacher opens evidence and confirms one signal and dismisses another with reasons. Each decision produces `risk.reviewed`; neither decision automatically penalizes the student. Clarification and pending/failed states are demonstrated. |
| AC-6 | FR-29–FR-34, FR-39 | Student dashboard shows fixture-verified progress, normalized score trend, weak topics, accessible recommendations, and current deadlines. Missing scores or topic mappings display insufficient data. Replay and delayed delivery leave the same aggregates. |
| AC-7 | FR-35–FR-36 | Teacher filters the assigned group, sees correct completion/performance counts and explained learning-support candidates from `analytics.group_risk`, and cannot access the other group's individual records. |
| AC-8 | FR-41–FR-42, FR-45 | A supplied behavior anomaly from quiz events appears beside its attempt in `grading` and follows the same teacher review workflow. No signal or service timeout is not represented as proof of integrity. |
| AC-9 | FR-43–FR-45 | A synthetic comparable variant cohort meeting sample and rate-rise thresholds produces `risk.variant_leak`; a small sample and a changed scoring version do not. Evidence shows both windows/counts and permits a teacher decision of `confirmed` or `dismissed`. |
| AC-10 | FR-47–FR-52, FR-57, FR-58 | Refused consent produces no capture and no `exam.started`. With recorded consent and readiness, the session emits `exam.started`, correlated `exam.agent_signal`, and a fixture `risk.proctoring` is reviewable in `grading` (a live model is not required for MVP acceptance). The consent screen shows the agent's allowlist; a student without Windows is offered the configured alternative path. |
| AC-11 | FR-53–FR-55; NFR-8 | Normal end, timeout, and withdrawal each stop webcam and client agent capture and produce one logical `exam.ended`; later capture is rejected. A disconnect is visible, saved answers survive, and no automatic failing grade is created. |
| AC-12 | NFR-1–NFR-4, NFR-6–NFR-7, NFR-9–NFR-16 | Recorded load, authorization, upload/link-expiry, retention, replay, restore, audit, and usability checks meet the stated targets. ML unavailability preserves submission/quiz workflows and shows pending results. |

All `must` requirements and their relevant NFR checks must pass for MVP acceptance. `should` and `could` items are tracked separately and do not silently become release gates. Delivery evidence comprises these documents, five renderable PlantUML sources with PNG exports, the two dashboard specifications, and recorded acceptance results when implementation is available. The current deliverable defines the tests; it does not claim they have been executed against an implemented platform.
