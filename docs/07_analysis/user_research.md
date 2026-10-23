# User research: interviews with teachers and students

Status: five interviews completed on 23 October 2026 (LMS-25): one teacher (I2) and four students (I1, I3, I4, I5). Summary: 9 findings, 5 of 5 initial assumptions checked (4 confirmed, 1 partly). This document separates **verified findings** (said by interviewees) from **assumptions** (ours, not yet confirmed).

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
| I2 | Teacher, Computer Organization and Architecture 1; practical and laboratory assessments, weekly quizzes, two attestations and a Moodle final exam | Not provided | Syrym | **Assessment:** students assemble and disassemble PCs, work with BIOS/UEFI, install Windows, use the command line and Cisco Packet Tracer, and defend laboratory work; two attestations contribute 30% each and the final Moodle quiz 40%. **Integrity review:** checks reports manually in Moodle, compares similar screenshots, steps and even shared errors, and asks students to explain their work; a routine review takes about 5–10 minutes, longer when doubts arise. **Recent case:** near-identical Windows installation reports led to oral explanations that revealed different levels of understanding; highlighted matching passages would save manual comparison. **Quiz anomalies:** an unexpected rise from roughly 50–60 to 95 is a reason to investigate, not proof of cheating; wants to compare answers and submission times with past quiz and practical results. **Weekly view:** weak topics such as BIOS/UEFI, RAID and OSI/TCP/IP, changing grades, missed labs and attendance against the course's 70% minimum. **Risk review:** needs specific matching passages and context; common lab instructions may produce legitimate similarities, and the system must never assign zero or accuse a student automatically. **Proctoring:** potentially useful for a remote final exam, not routine labs; requires clear data rules, university approval, student awareness of what is recorded and who can view it; prefers oral lab defense for practical work. |
| I3 | Student, Astana IT University; uses Moodle | Not provided | Syrym | **Progress view:** says Moodle currently provides enough information and reports no missing progress information. **Recommendations:** would find course-material recommendations convenient because materials could be kept in one place instead of searched for manually. **Integrity checks:** has a mixed view of AI-assisted work; considers the idea to be the student's while AI can help find information and write up or implement it, and does not regard that use as inherently dishonest. Did not specify a process for contesting a similarity or AI-text signal. **Unfair quiz behaviour:** has not encountered an example. **Proctoring:** would consent to webcam monitoring for an exam as a precaution for honest assessment, but wants to know who can view the video; accepts access limited to the examiner and would be uncomfortable with public exposure. Did not state a retention period. |
| I4 | Student, Software Engineering, AITU (Moodle user) | 23 Oct 2026 | Kaiyrbekuly A. (written answers) | **Progress view:** satisfied with Moodle, reports nothing missing. **Recommendations:** yes, convenient to have course materials gathered in one place instead of searching manually. **Integrity checks:** mixed view: considers AI a tool for automation and search; if the idea is the student's own and AI only writes it up, that is not dishonest in his view; plagiarism checks are a different matter. **Unfair quiz behaviour:** has not encountered any. **Proctoring:** would agree under any conditions since examiners need assurance the test is honest; wants to know who sees the video and would object to public exposure; accepts examiner-only access. |
| I5 | Student, Software Engineering, AITU (university LMS user) | 23 Oct 2026 | Kaiyrbekuly A. (voice message, transcribed) | **Progress view:** main gap is transparency of grading: unclear how the final grade is composed and what weight each assignment has; syllabi in the LMS are often outdated versions that differ from what teachers require in class; wants an up-to-date dashboard with a clear point breakdown per assignment. **Recommendations:** would definitely use them if precise: not "repeat topic X" but a link to the exact lecture fragment where the gap is, based on own past mistakes; annoyed by abstract advice ("read the whole module"), recommendations of things not in the test, and intrusive pop-ups. **Integrity checks:** fine with plagiarism and AI checks as long as rules are the same for everyone and the system is honest: a detailed report instead of abstract percentages; AI detectors often misfire on ordinary academic language, so a human must make the final decision; would like to pre-check a work before final submission. **Unfair quiz behaviour seen:** classic cheat sheets, phones outside the frame, micro-earpieces, cooperation on a shared call or chat, and browser extensions with neural networks that parse the question and overlay answers on the test. **Proctoring:** would agree under clear and reasonable conditions: the program must not access personal files and must not break on small connection drops; wants to know in advance (1) what exactly counts as a violation, e.g. leaning back or looking aside; (2) who has access: only proctors, and the committee in case of a dispute; (3) retention: on the university server, one month after the session for appeals, then deleted. |

## 4. Findings

| # | Finding | Source (I#) | Verified / assumption | Affected requirements |
|---|---------|-------------|-----------------------|-----------------------|
| F1 | Students need a single progress view: points earned, points still available, weak topics by topic; grade composition and weights must be transparent and syllabi up to date | I1, I5 (I4 reports no gap) | verified (2 of 3 students) | FR-30, FR-31, FR-32 confirmed; add "points still available" to the student dashboard spec |
| F2 | Recommendations are accepted only if concrete (exact lecture fragment, 1–2 materials), tied to own mistakes, few, timely and not intrusive | I1, I4, I5 | verified (3 students) | FR-33: each recommendation names topic, 1–2 materials and a time estimate; FR-40 dismiss/helpful; add frequency limit to NFR list |
| F3 | Plagiarism checks are seen as fair when rules are equal; AI-text detectors are distrusted (false positives on academic language); a result must be a detailed report, not a percentage, a human decides, and the student can respond; one student would like a pre-submission self-check; views on AI-assisted writing differ between students | I1, I4, I5; teacher I2 | verified (3 students, 1 teacher) | FR-26, FR-27, FR-28 confirmed; FR-23 stays "should" and signal-only |
| F4 | Observed quiz cheating: group-chat answer sharing, shared calls, second device or phone outside the frame, micro-earpieces, cheat sheets, AI in another tab and browser extensions that overlay answers | I1, I5 (I4 none); teacher I2 reports unexplained score rises | verified (2 students, 1 teacher) | FR-41, FR-43 (synchronous submissions, answer agreement); client.tab_switched / client.paste signals; motivates the time-window collusion heuristic |
| F5 | Proctoring is acceptable with advance disclosure of (a) what counts as a violation, (b) who can view recordings (proctor; committee only on dispute), (c) retention (until appeals close, about one month, then deletion); agent must not touch personal files, must tolerate short connection drops, run only during the exam and uninstall; human check before any sanction; teacher sees it as useful for remote finals, not routine labs | I1, I4, I5; teacher I2 | verified (3 students, 1 teacher) | FR-48, FR-49, FR-53, FR-55, NFR-8, NFR-10 confirmed; add explicit "agent uninstalls after the session" to the agent requirements |
| F6 | Students want a fair queue for lab defenses and consultations with a Telegram interface | I1 | verified need (1 student), out of project scope | Recorded in backlog (LMS-33) for after the diploma; not added to scope |
| F7 | Teachers compare reports manually (screenshots, steps, shared errors) and spend 5–10 minutes per routine review; highlighted matching passages with context would replace manual comparison; shared lab instructions cause legitimate similarity | I2 | verified (1 teacher) | FR-22, FR-24 confirmed; template allowlist in the integrity module (existing in ml/integrity) is a must for lab courses |
| F8 | Teachers want a weekly view of weak topics, grade changes, missed labs and attendance against the course minimum; a score rise is a reason to investigate, never proof | I2 | verified (1 teacher) | FR-35, FR-36 confirmed; add attendance/missed-labs to the teacher dashboard backlog; FR-42 wording confirmed |
| F9 | A violation definition shown before the exam (e.g. looking aside is not a violation by itself) is required for consent to be meaningful | I5 | verified (1 student) | Consent screen (FR-48) gains a short list of what is and is not treated as a signal; gaze proxy stays exploratory (events.md risk.proctoring kinds) |

Assumptions made before interviews, to be confirmed or rejected:

| # | Assumption | Status after interviews |
|---|------------|-------------------------|
| A1 | Teachers check similarity manually or not at all because licensed tools are not available for every course | Confirmed by I2 (manual comparison in Moodle, 5–10 min per report) |
| A2 | Teachers want an explanation and the matched source, not a percentage | Confirmed by I2; students I1 and I5 want the same for themselves |
| A3 | Students accept monitoring during exams only with explicit consent, visible status and a short retention period | Confirmed by I1, I4, I5 (adds: definition of violations, exam-only agent, no access to personal files, restricted viewers, ~1 month retention) |
| A4 | Quiz variant leaks are noticed late, by a sudden rise in scores | Partly confirmed by I2 (unexpected rise from 50–60 to 95 triggers investigation) |
| A5 | Weekly at-risk lists would be used if they come with a reason and a suggested action | Confirmed by I2 (weak topics, grade changes, missed labs, attendance) |

## 5. Impact on requirements

Changes to make in the technical specification after the interviews (tracked as a follow-up task):

| Change | Caused by |
|--------|-----------|
| FR-30: student progress shows points earned and points still available per course, not only completion | F1 |
| FR-33: a recommendation names the topic, at most two materials and an estimated time; at most N active recommendations per course (N configurable) | F2 |
| New agent requirement: the client agent runs only during the exam session and removes itself completely afterwards; this is stated on the consent screen | F5 |
| Consent screen: retention period and the list of who can view recordings are shown explicitly | F5 |
| Consent screen lists what is and is not treated as a violation signal | F9 |
| Teacher dashboard backlog: attendance and missed labs against the course minimum | F8 |
| Backlog (out of scope): lab-defense queue with Telegram bot | F6 |
