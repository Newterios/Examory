<!-- docs/02_requirements/user_stories.md -->
# Examory User Stories

Every story applies only to the user's authorized courses, groups, and records. Automated risks and suggested grades require human review; a risk score alone never changes a Grade or imposes a sanction.

## Student

### US-S1 — View learning progress

As a student, I want to see my Course and Module progress, so that I can plan the remaining learning activities.

- The dashboard lists only enrolled courses and shows completed and total eligible published materials.
- A processed `material.viewed` event with `completed = true` updates the corresponding completion state without counting duplicate events twice.
- Missing activity is shown as unavailable or incomplete, never as a fabricated completion.

### US-S2 — Submit assignment work

As a student, I want to upload and submit my assignment files, so that my teacher can assess the correct work.

- Supported files are uploaded through `files`; a successful submission through `assignments` returns its identifier and submitted time.
- `submission.created` identifies the Assignment, Submission, attempt number, file references, and deadline.
- A failed upload prevents submission of an incomplete file reference; late work displays its actual late status.

### US-S3 — Complete a quiz

As a student, I want to answer my assigned quiz Variant, so that my knowledge can be assessed fairly.

- Starting an authorized Attempt produces `quiz.started` with its `variant_id` and time limit.
- Saved answer actions produce `quiz.answered`; final submission produces one logical `quiz.submitted` result for the Attempt.
- The interface shows the remaining time and a submitted state; an unauthorized Variant cannot be opened.

### US-S4 — Understand weak topics

As a student, I want to see my weak topics and recommended materials, so that I can focus my revision.

- Topic findings use question-to-topic mappings and assessed outcomes, with an explanation and data freshness indicator.
- `analytics.recommendation` items open only materials available to me in the relevant Course.
- Insufficient assessed evidence displays an insufficient-data state instead of a confident weakness claim.

### US-S5 — Track deadlines and published grades

As a student, I want to see upcoming deadlines and published grades, so that I can manage my workload and understand feedback.

- Upcoming deadlines come from authorized Assignment records and show the display timezone.
- Published `grade.assigned` results show the score, maximum score, and available rubric feedback for my own work.
- Unconfirmed `analytics.grade_suggestion` values are not displayed as final grades.

### US-S6 — Start an informed exam session

As a student, I want to review webcam and Windows client agent requirements before consenting, so that I understand what the exam collects.

- The consent screen identifies the data categories, purpose, retention, and withdrawal action before capture starts.
- `exam-sessions` allows `exam.started` only after explicit consent and successful required device checks; refusal starts no capture.
- If I decline, the interface explains the institution-approved alternative or contact route without claiming that an alternative has already been granted.

### US-S7 — End capture and see exam status

As a student, I want to end my exam or withdraw consent, so that I can control continued collection and see the session outcome.

- Ending an active ExamSession produces `exam.ended` and stops webcam and client agent capture.
- Withdrawal stops capture, marks the session as ended with its reason, and explains that already collected evidence follows the disclosed retention policy.
- The interface distinguishes a submitted assessment from an interrupted exam; interruption alone creates no automatic misconduct verdict.

## Teacher

### US-T1 — Organize course learning

As a teacher, I want to manage Courses, Modules, and Materials, so that my assigned groups receive an organized learning path.

- I can create or update content in courses assigned to me and set its order and publication status.
- Students see published content only when their Group is enrolled in the Course.
- Requests to modify another teacher's unassigned Course are denied.

### US-T2 — Configure assessments

As a teacher, I want to prepare Assignments, Quizzes, Questions, Variants, and Rubrics, so that assessment expectations are explicit.

- An Assignment can define instructions, allowed files, a deadline, and a Rubric; a Quiz can define Questions, Variants, and a time limit.
- Rubric criterion limits and quiz scoring totals are validated before publication.
- A started Attempt retains its assigned Variant and assessment configuration even if a later configuration is published.

### US-T3 — Review academic integrity signals

As a teacher, I want to inspect borrowing and AI-text indicators beside submitted work, so that I can make a documented academic decision.

- `grading` presents relevant risk explanations and authorized evidence, including `risk.plagiarism`, beside the target Submission.
- Confirm and dismiss require a review note and produce `risk.reviewed` with the signal, reviewer, decision, and time.
- Either review action preserves the original signal and creates no automatic Grade change or sanction.

### US-T4 — Support students using group analytics

As a teacher, I want to see group performance and students needing learning support, so that I can target teaching assistance.

- The dashboard filters by my authorized Course and Group and summarizes published grades, progress, and weak topics.
- `analytics.group_risk` findings show their explanations and freshness and are labeled as learning-support indicators.
- Empty or insufficient data is explicit, and the view does not expose students from unassigned groups.

### US-T5 — Confirm a rubric-based grade

As a teacher, I want to review and adjust a rubric-based grade suggestion, so that I remain responsible for the final assessment.

- `analytics.grade_suggestion` shows suggested criterion scores, the maximum, and an explanation for the target work.
- I can accept or edit valid criterion scores; out-of-range values prevent confirmation.
- Only my explicit confirmation records the Grade and produces `grade.assigned`; the suggested value remains distinguishable in review history.

### US-T6 — Review behavior anomalies and variant leakage

As a teacher, I want to examine unusual assessment behavior and possible Variant leakage, so that I can investigate with context.

- The risk queue includes `risk.behavior_anomaly` and `risk.variant_leak`, with targets, explanations, and supporting context.
- A variant alert identifies its `variant_id`, observation period, comparison, and sample size; insufficient data is labeled accordingly.
- Confirm or dismiss produces `risk.reviewed`; a shared Variant or high score alone does not automatically mark every student as guilty.

### US-T7 — Supervise proctored exams

As a teacher, I want to inspect ExamSession status and proctoring signals, so that I can handle exam issues through human review.

- Authorized sessions show consent status, start and end times, and missing-input or interruption states.
- `risk.proctoring` is displayed with an explanation and access-controlled evidence while that evidence remains within retention.
- I can confirm or dismiss a signal with a reason; lack of telemetry alone does not automatically fail an Attempt.

## Admin

### US-A1 — Manage accounts and roles

As an admin, I want to provision accounts and assign roles, so that platform access matches institutional responsibilities.

- I can create, disable, and assign permitted roles to User accounts through the authorized administrative interface.
- Role changes and account disablement are audited with actor, target, action, and time.
- Disabled accounts cannot obtain new sessions; existing sessions are revoked according to the configured session policy.

### US-A2 — Manage group and course access

As an admin, I want to manage Group membership and Course assignments, so that users access the correct teaching context.

- I can assign students to Groups, enroll Groups in Courses, and assign teachers to Courses and Groups.
- Updated permissions apply to subsequent protected requests and prevent access outside the new scope.
- Administrative membership privileges grant no default right to assign academic grades or inspect raw proctoring evidence.

### US-A3 — Maintain consent configuration

As an admin, I want to configure the approved exam consent notice, so that students receive a consistent and auditable disclosure.

- A notice defines purposes, collected categories, retention, withdrawal behavior, and an alternative or contact route.
- Each consent record references the notice version and student, session, and acceptance time.
- Publishing a new notice preserves earlier versions and cannot fabricate or silently replace student consent.

### US-A4 — Enforce retention

As an admin, I want to configure and monitor retention, so that personal data is not kept beyond approved periods.

- Proposed defaults are raw proctoring evidence: 7 days after exam end; derived risks and pseudonymous learning events: 180 days after course end; academic records: one year after course end.
- Expired data is deleted or irreversibly anonymized by the next scheduled daily cleanup; retained risk records show when supporting evidence has expired.
- Cleanup outcomes are auditable; backup copies expire within 30 days, and restored data is subjected to retention cleanup before normal access.

### US-A5 — Monitor event ingestion

As an admin, I want to monitor event delivery and analytics freshness, so that I can identify missing or delayed updates.

- The operational view shows health, consumer lag, and the last successfully processed event time for `lms.quizzes`, `lms.submissions`, `lms.grades`, and `lms.courses`.
- Invalid envelopes and failed processing are counted with diagnostic identifiers without exposing submission text or raw webcam evidence.
- Reprocessing a duplicate `event_id` does not create a second logical Grade, risk review, or analytics activity entry.

### US-A6 — Audit privileged operations

As an admin, I want to inspect authorized access and change audit records, so that I can investigate platform administration issues.

- Audit queries record and display permitted actor, action, target, timestamp, and outcome fields for role, consent configuration, retention, and evidence-access operations.
- Ordinary administrators cannot alter audit history or use audit search to retrieve raw assessment content or proctoring evidence.
- Export is restricted to authorized administrative scope and is itself audited.

### US-A7 — Manage operational availability

As an admin, I want to monitor service health and verify recovery, so that students and teachers can continue their work reliably.

- The operational view identifies unavailable services and delayed analytics without representing stale data as current.
- A documented restore check verifies authorized account access, Course records, submitted work references, and published Grade consistency.
- Recovery preserves recorded consent and review decisions and does not restart ended webcam or client agent capture.

## Assumptions

Retention periods are proposed project defaults for institutional approval, not legal conclusions. Detailed event contracts are unavailable; review metadata, consent fields, course dates, assessment versioning, and operational health fields are proposed interface requirements. Final sanctions and alternative exam arrangements remain institutional decisions.
