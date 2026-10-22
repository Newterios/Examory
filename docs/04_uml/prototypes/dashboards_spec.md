<!-- docs/04_uml/prototypes/dashboards_spec.md -->
# Examory Dashboard Specifications

## 1. Purpose and scope

This document specifies two English-language screens for implementation in Figma: the Student Dashboard and Teacher Dashboard. It covers content, event provenance, interactions, states, and access rules. It specifies observable analytics outputs without describing ML model or LMS core internals.

All displayed data is limited to the signed-in user's permitted courses and groups. Web clients read authorized service APIs; they do not subscribe directly to Kafka. The analytics service consumes `lms.quizzes`, `lms.submissions`, `lms.grades`, and `lms.courses`. `grading` supplies persisted risk signals and review outcomes. Event names below are exact contracts from the brief.

## 2. Shared layout and components

| Element | Specification |
|---|---|
| Desktop frame | 1440 px wide; 240 px navigation; 24 px page padding and gaps; 12-column content grid. Header contains page title, course selector, period selector, and freshness timestamp. |
| Student layout | First row: progress and upcoming deadlines. Second row: weak topics and recommended materials. Third row: grades and exam session status. |
| Teacher layout | First row: group performance. Second row: students needing learning support and work awaiting risk review. Third row: rubric grade suggestion and variant comparison. |
| Mobile frame | 360 px wide; 16 px padding; one column; navigation drawer; same block order. Replace wide tables with labeled cards; retain all actions and status text. |
| Filters | Student: own enrolled course and period. Teacher: assigned course, permitted group, period, and review status. Default period: previous 30 days through now. Define intervals as start-inclusive, end-exclusive. Show active filters and a reset action. |
| Time | Store event times in UTC. Display date, time, and explicit timezone; use `Asia/Almaty` (Astana) as default. Deadlines use authoritative service values. Analytics periods use `occurred_at`; submission lateness uses `submitted_at` against `deadline_at`. |
| Components | Navigation item, page header, filter select, metric card, progress bar, chart with table alternative, topic row, material card, deadline row, risk badge, evidence drawer, rubric table, review dialog, toast, skeleton, empty state, and error banner. |
| Figma variants | Default, hover, focus, disabled, loading, empty, error, stale, selected, and success where applicable. Risk badge variants: pending review, confirmed, dismissed; visual severity remains separate from review status. |
| Accessibility | Keyboard operation and visible focus; meaningful labels; 4.5:1 normal-text contrast; no meaning conveyed by color alone; chart summaries and data tables; status updates announced without moving focus. |

The header shows both the last successful refresh and data coverage end. A delayed event stream produces a stale banner; a completed action is not represented as missing simply because its projection is late. Deduplicate events by `event_id`, and use the latest authoritative entity state when updates conflict. Filter changes preserve focus and do not expose data from the previous permission scope.

## 3. Screen A: Student Dashboard

**User goal:** understand personal progress, identify areas to study, and complete the next learning task. Show only the student's own results. Course-wide misconduct signals and other students' identities are excluded.

| Block name | Data displayed | Events and supporting reads | User actions |
|---|---|---|---|
| Course progress | Course title; completed/eligible material count; percentage; progress by Module; last activity. Show `No materials` when denominator is zero. | `material.viewed` with `completed`; `courses` supplies enrolled Course, Module, Material, and published/eligibility status. | Change course; open a Module; resume a Material; inspect the completion list. |
| Weak topics | Topic name; mastery percentage; number of distinct scored questions; evidence period; explanation; insufficient-data state. Timing alone never means an incorrect answer. | `quiz.answered` indicates activity and question identity; `quiz.submitted` indicates completion. `quizzes` supplies authorized Question-to-topic mapping and authoritative scored Answer results. | Open topic details; see permitted feedback; open related Material. Hide unreleased answers and solutions. |
| Recommended materials | Ranked Material cards, associated topic, explanation, and recommendation timestamp. If no recommendation exists, show a neutral empty state. | `analytics.recommendation`; `material.viewed` updates completion. Resolve valid, published Material titles and links through `courses`. | Open a Material; view why it is recommended; refresh recommendations. |
| Upcoming deadlines | Assignment title, due time, remaining time, current Submission state, overdue label, and submission availability. Sort actionable deadlines ascending. | `submission.created` updates activity. `assignments` supplies authoritative upcoming Assignment schedules, effective deadlines, availability, and current Submission state; a past event cannot enumerate future tasks. | Open Assignment; upload through `files` and submit through `assignments`. |
| Grades and feedback | Published Grade, maximum Grade, percentage, rubric criterion scores, teacher feedback, and publication time. Label quiz score separately from final Grade. | `grade.assigned`; `quiz.submitted`; authorized `grading` reads supply published Grade and Rubric details; `quizzes` supplies permitted Quiz links and availability. | Open feedback; inspect rubric; filter grade history; open an available Quiz. A suggested grade is not presented as final. |
| Exam session | Scheduled exam, readiness status, consent state, current ExamSession state, and camera/agent status when applicable. | `exam.started`, `exam.agent_signal`, and `exam.ended` provide lifecycle updates. `exam-sessions` supplies authorized schedules, consent, and current session state; `quizzes` supplies permitted Quiz availability. | Read consent terms; grant or decline consent; run readiness checks; enter an eligible session; start/resume an available Quiz; end session; contact teacher for an alternative assessment arrangement. |

### Student interactions and states

- Progress details list eligible published Material and its completion status; an ordinary view with `completed=false` does not complete it.
- Weak-topic details show aggregate evidence only when assessment feedback is released. Before release, show `Feedback available after release` without exposing Question content, solutions, or scoring evidence.
- Deadlines show overdue work separately from upcoming work. A successful submission shows its server timestamp; a pending analytics refresh does not invite an unnecessary second submission.
- Consent has separate video and client-agent disclosures with no preselected agreement. Declining blocks the proctored start and presents the institutional alternative-assessment route; it does not assign a penalty.
- During an active exam, show visible capture status and the approved stop/withdrawal control. Withdrawal stops collection and follows the configured exam-session policy; do not imply a completed examination or a failed Grade automatically.
- First-time users see `No activity yet` with links to available course tasks. API failures show retry actions; failed retrieval must not display a misleading zero score.

## 4. Screen B: Teacher Dashboard

**User goal:** assess group learning, plan support, review risk evidence, and assign defensible grades. Keep learning-support indicators visually and semantically separate from suspected academic misconduct.

| Block name | Data displayed | Events and supporting reads | User actions |
|---|---|---|---|
| Group performance | Mean normalized quiz score, scored-student count/roster count, material completion, submission lateness, score distribution, and trend for the selected period. Chart tooltips show denominators. | `material.viewed`, `submission.created`, `quiz.submitted`, `grade.assigned`; `courses` and `users` supply authorized roster and course catalog; `quizzes` and `grading` validate scores. | Filter Course/Group/period; inspect table alternative; open a permitted student learning profile. |
| Students needing learning support | Student identity; learning-risk reason; quiz average and sample count; overdue required tasks; last activity; suggested materials. Use `Learning support` labels, never misconduct labels. | `analytics.group_risk`, `analytics.recommendation`; contributing `material.viewed`, `submission.created`, `quiz.submitted`, and `grade.assigned`. Read schedules and roster through scoped APIs. | Inspect supporting results; open recommended Material; prepare a support action. Insufficient evidence remains distinct from no risk. |
| Work awaiting risk review | Submission/Attempt/ExamSession reference, student, source, signal level/score where provided, explanation, evidence availability, and review status. Group-level variant signals link to the variant block. | `risk.plagiarism`, `risk.behavior_anomaly`, `risk.proctoring`, `risk.variant_leak`, and `risk.reviewed`; `grading` supplies persisted RiskSignal and review state. | Open evidence; use `Confirm` or `Dismiss`; enter mandatory rationale; submit review; inspect review history. |
| Rubric grade suggestion | Selected work and Rubric; criterion descriptions, score limits, suggested criterion scores, suggested total, and explanation. Show current teacher Grade separately. | `analytics.grade_suggestion`; `submission.created`, `grade.assigned`; authorized `assignments` and `grading` reads resolve work, Rubric, and current Grade. | Inspect explanation; edit proposed criterion scores; explicitly assign Grade; compare with existing Grade. No automatic acceptance or publication. |
| Variant comparison | Quiz and Variant; recent/baseline pass rates; rate difference in percentage points; distinct-student sample sizes; periods; configured pass threshold; comparability limitations; signal explanation. | `quiz.started`, `quiz.submitted`, and `risk.variant_leak`; `quizzes` supplies Variant, scoring/pass policy, eligible attempts, and assessment comparability data. | Select Variant; inspect cohort table and eligible evidence; open signal review; confirm or dismiss with rationale. |

### Teacher interactions and states

- Selecting work opens an evidence drawer without losing filters. Evidence links are authorized and time-limited; expired or deleted evidence is labeled unavailable rather than silently removed from the explanation.
- The risk review dialog states that the signal is an indicator requiring judgment. Show `Confirm signal` and `Dismiss signal`, a required rationale, and target details. Disable submission while a request is pending; preserve entered text on failure.
- A successful review through `grading` produces `risk.reviewed`, updates review status, and displays the reviewer and review time. If another teacher already reviewed it, refresh the current state and require an explicit new decision. Confirmation does not automatically assign Grade, impose a sanction, or close an exam.
- A rubric suggestion is advisory. The teacher reviews each criterion and explicitly saves through `grading`; only successful assignment is reflected by `grade.assigned`. Block scores outside rubric limits. If the work or Rubric no longer matches the suggestion, mark it stale and request an updated suggestion.
- A variant signal is a group-level hypothesis, not an accusation against every member. With insufficient samples or incompatible scoring/exposure conditions, show `Insufficient comparable evidence` and the missing inputs.
- Empty review queues show `No pending signals`. Missing analytics shows `Not available yet`; loading, zero values, and absence of evidence are separate states.

## 5. Metric definitions and evidence rules

| Metric | Definition and boundary conditions |
|---|---|
| Material progress | Distinct eligible published Material with an authorized completion record divided by all eligible published Material in the Course. Count each Material once; use authoritative completion state if later corrected. No denominator means not applicable. |
| Normalized quiz result | `100 × score / max_score` from a valid completed Attempt, with `max_score > 0` and score inside the permitted range. Invalid values are excluded with a data-quality notice. |
| Group quiz average | Use the latest eligible completed Attempt per student per Quiz inside the selected period. Average those quiz percentages per student, then average across students with data; show both scored-student count and total roster size. Do not mix raw points or ungraded work into this metric. |
| Grade percentage | `100 × grade / max_grade` for a published valid Grade with a positive maximum. Keep this metric separate from quiz results unless the course explicitly defines a combined grading policy. |
| Topic mastery | For the latest eligible scored response to each distinct Question in a topic, sum awarded points and divide by available points, then multiply by 100. Topic mapping and scored responses come from `quizzes`, not `quiz.answered`. Missing scoring or zero available points means unavailable. |
| Weak topic | Proposed configurable default: mastery below 60% across at least three distinct scored Question items. Fewer items means insufficient evidence, not low mastery. |
| Submission lateness | A Submission is late when `submitted_at > deadline_at`; equality is on time. Respect student-specific deadline extensions supplied by `assignments`. Display the actual deadline used. |
| Overdue task | An available, required Assignment has passed its effective deadline and has no accepted Submission. Derive missing work from scoped task and submission reads; absence of an event alone is insufficient. |
| Learning-risk candidate | Proposed configurable default: quiz average below 60% across at least three distinct quizzes, or at least two overdue required assignments. Explain which condition applies. This is a learning-support classification with no misconduct consequence. |
| Variant pass rate | Percentage of eligible students whose normalized Quiz result meets that Quiz's configured pass threshold. Use latest eligible completed Attempt per student per Quiz within each comparison window, grouped by `variant_id`. |
| Variant comparison windows | Recent window: previous 7 days. Baseline: the preceding 30 days. Windows are contiguous and non-overlapping. Compare the same Quiz and Variant under compatible scoring, pass threshold, access conditions, and assessment scope. |
| Variant review candidate | Proposed configurable default: recent pass rate exceeds baseline by at least 20 percentage points, with at least 10 distinct students in each window. Exclude recent-window students from baseline membership to avoid overlapping cohorts; report exclusions and final sample sizes. Failing these checks yields insufficient evidence, not a cleared or confirmed leak. |

Period selectors affect learning metrics. Variant comparison always displays its explicit paired windows and evaluation time; it must not silently reuse the dashboard's single-period denominator. A sharp improvement can have legitimate causes, so comparison evidence includes available changes in instruction, scheduling, scoring, and cohort composition without claiming causation.

## 6. Payload and service boundaries

- The shared event envelope is `event_id`, `event_type`, `version`, `occurred_at`, `producer`, `actor {user_id, role}`, `context {course_id, group_id, session_id}`, and `payload`.
- The brief's output payload for `analytics.recommendation`, `analytics.group_risk`, `analytics.grade_suggestion`, and `risk.variant_leak` is `{student_id, course_id, kind, items[], explanation}`. Preserve these names. Do not fabricate canonical top-level fields or treat this shape as identical to the generic ML risk payload.
- The generic ML `risk.*` payload is `{risk_id, target_id, score, level, explanation, evidence}`. Display level and score only when supplied; missing fields must not become invented confidence values.
- Proposed extension requiring contract agreement: `items[]` carries the relevant display references and measures (Material references, student support entries, rubric criterion suggestions, or variant/cohort evidence). For group-scoped outputs, `student_id=null` and authorized group scope comes from `context.group_id`. A `risk.variant_leak` item carries proposed `risk_id`, `target_id=variant_id`, `score`, `level`, `explanation`, and `evidence`, with Quiz/Variant references and comparison windows; `grading` persists each item as a RiskSignal targeting Variant. These nested fields and this mapping are assumptions, not canonical additions to the brief.
- `grading` must resolve a persisted RiskSignal and review target before enabling `Confirm` or `Dismiss`. A group signal without a resolved authorized target is displayed read-only with an unavailable-review reason.
- No invented enrollment, deadline, topic, correctness, or notification events are required. Read these reference facts through the named scoped services. Analytics does not author enrollment, course availability, consent, or final Grade state.

## 7. Access, privacy, and prototype acceptance

Student filters cannot request another student's data. Teachers can view only assigned courses and groups; object-level authorization also applies to evidence links and drill-downs. Administrators do not gain a general student-evidence dashboard by default. Exam evidence is shown only to authorized reviewers and only within the approved retention period; raw webcam streams, agent telemetry, and private recordings are not copied into dashboard analytics.

For prototype review, prepare two populated desktop frames, two mobile frames, and component variants demonstrating empty data, insufficient samples, loading, failed refresh, stale suggestions, denied access, pending review, confirmed review, and dismissed review. Use clearly synthetic names and results. Verify keyboard order, filter context, denominators, timezone labels, and the distinction between an advisory suggestion and an assigned Grade.

Acceptance walkthrough: a student sees progress, weak-topic evidence, recommendations, and upcoming deadlines; a teacher filters a group, identifies a learning-support candidate, reviews risk evidence with a rationale, edits a rubric suggestion before assigning Grade, and inspects a variant comparison that exposes sample size and limitations. No walkthrough produces an automatic misconduct verdict.

Assumptions: Dashboard dimensions, metric thresholds, comparison windows, and scoped reference APIs are proposed defaults pending team contract alignment. UTC storage, `Asia/Almaty` (Astana) display, and analytics `items[]`/group-scope behavior follow `docs/03_architecture/events.md`.
