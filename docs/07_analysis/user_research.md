# User research: interviews with teachers and students

Status: interviews in progress, 23 October 2026 (LMS-25). This document separates **verified findings** (said by interviewees) from **assumptions** (ours, not yet confirmed).

## 1. Purpose and method

Short semi-structured interviews (20–30 min, voice or in person) with teachers and students of Astana IT University. Participants are informed that notes are taken, no recording is made, and only role and experience are published. Each team member conducts at least one teacher and one student interview.

## 2. Interview guide

**Teachers**
1. Which courses do you teach, and what kinds of assessment do you use (assignments, code, quizzes, projects)?
2. How do you currently check submissions for copying or AI-generated text? What tools, how long does it take, what do you do with the result?
3. Describe a recent case where you suspected misconduct. What evidence did you have, and what would have helped?
4. In quizzes: have you seen leaked variants, synchronized answers or unexpected score jumps? How did you notice?
5. Which information about a group would you want to see weekly to intervene early (weak topics, at-risk students)?
6. If a system flagged a submission as risky, what would you need to see before acting? What must it never do automatically?
7. Attitude to webcam proctoring and a client agent: under which conditions would you use them?

**Students**
1. Which LMS do you use, and what is missing in your view of your own progress?
2. Would you use recommendations of what to study before a quiz? What would make them useful or annoying?
3. How do you feel about similarity or AI-text checks on your work? What would make the process fair?
4. Online quizzes: what behaviour of other students have you observed that you consider unfair?
5. Webcam monitoring during exams: what consent, transparency and data-retention conditions would you require?

## 3. Interviews conducted

| # | Role, experience | Date | Interviewer | Key statements (own words of the interviewee, summarized) |
|---|------------------|------|-------------|------------------------------------------------------------|
| I1 | Student, 3rd year, Software Engineering (uses Moodle) | 23 Oct 2026 | Kaiyrbekuly A. | **Progress view:** grades are scattered per assignment; wants one picture of points earned, points still available and weak topics. **Recommendations:** would use them if concrete ("topic X, two materials, 20-minute tasks") and based on own mistakes; too many, repetitive or last-minute notifications would annoy. **Integrity checks:** plagiarism checks are fair; AI detectors often err, so a result must not be a verdict; fairness means showing what was found, teacher verifies, student can explain or prove authorship (drafts, edit history). **Unfair behaviour seen in online quizzes:** answers shared in group chats during the test, second device, solving in pairs, AI open in another tab. **Proctoring:** would consent if told in advance what is recorded and analysed; agent must run only during the exam and uninstall cleanly; wants to know retention period (until appeals close, then delete), who sees the recording (teacher or committee only), and that a human checks a suspicion before any zero. **Separate wish:** a queue system for lab defenses and consultations (sign up, see your place and expected time), currently done in chats and corridors. |
| I2 | | | | |
| I3 | | | | |
| I4 | | | | |
| I5 | | | | |

## 4. Findings

| # | Finding | Source (I#) | Verified / assumption | Affected requirements |
|---|---------|-------------|-----------------------|-----------------------|
| F1 | Students need a single progress view: points earned, points still available, weak topics by topic | I1 | verified (1 student) | FR-30, FR-31, FR-32 confirmed; add "points still available" to the student dashboard spec |
| F2 | Recommendations are accepted only if concrete, tied to own mistakes, few and timely | I1 | verified (1 student) | FR-33: each recommendation names topic, 1–2 materials and a time estimate; FR-40 dismiss/helpful; add frequency limit to NFR list |
| F3 | Plagiarism checks are seen as fair; AI-text detectors are distrusted; a result must not be a verdict and the student must be able to respond | I1 | verified (1 student) | FR-26, FR-27, FR-28 confirmed; FR-23 stays "should" and signal-only |
| F4 | Observed quiz cheating: group-chat answer sharing, second device, pair solving, AI in another tab | I1 | verified (1 student) | FR-41, FR-43 (synchronous submissions, answer agreement); client.tab_switched / client.paste signals; motivates the time-window collusion heuristic |
| F5 | Proctoring is acceptable only with advance disclosure, exam-only agent, clean uninstall, stated retention (until appeals close), restricted viewers, human check before any sanction | I1 | verified (1 student) | FR-48, FR-49, FR-53, FR-55, NFR-8, NFR-10 confirmed; add explicit "agent uninstalls after the session" to the agent requirements |
| F6 | Students want a fair queue for lab defenses and consultations with a Telegram interface | I1 | verified need, out of project scope | Recorded in backlog (LMS-33) for after the diploma; not added to scope |

Assumptions made before interviews, to be confirmed or rejected:

| # | Assumption | Status after interviews |
|---|------------|-------------------------|
| A1 | Teachers check similarity manually or not at all because licensed tools are not available for every course | |
| A2 | Teachers want an explanation and the matched source, not a percentage | |
| A3 | Students accept monitoring during exams only with explicit consent, visible status and a short retention period | Confirmed by I1 (adds: exam-only agent, clean uninstall, restricted viewers) |
| A4 | Quiz variant leaks are noticed late, by a sudden rise in scores | |
| A5 | Weekly at-risk lists would be used if they come with a reason and a suggested action | Not yet covered (teacher interviews pending) |

## 5. Impact on requirements

Changes to make in the technical specification after the interviews (tracked as a follow-up task):

| Change | Caused by |
|--------|-----------|
| FR-30: student progress shows points earned and points still available per course, not only completion | F1 |
| FR-33: a recommendation names the topic, at most two materials and an estimated time; at most N active recommendations per course (N configurable) | F2 |
| New agent requirement: the client agent runs only during the exam session and removes itself completely afterwards; this is stated on the consent screen | F5 |
| Consent screen: retention period and the list of who can view recordings are shown explicitly | F5 |
| Backlog (out of scope): lab-defense queue with Telegram bot | F6 |
