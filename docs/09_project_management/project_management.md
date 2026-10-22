# Project management and risks

## 1. Tools and traceability

| Tool | Use | Link |
|------|-----|------|
| GitHub | Code, documents, diagrams; one branch and one pull request per task; `main` is the reviewed baseline; tags per submission | https://github.com/Newterios/Examory |
| YouTrack | Tasks with owner, priority, due date, expected outcome, acceptance criteria and link to the merged PR | https://examory.youtrack.cloud/projects/LMS · board https://examory.youtrack.cloud/agiles/204-1/current |
| GitHub Actions | Build and test on every PR and push; docs and compose checks | `.github/workflows/ci.yml` |
| Telegram | Daily coordination; weekly call with the supervisor | — |

Rules (from the repository README): branch `LMS-N-slug`, commit prefix `LMS-N:`, PR description references the task and says how the change was checked, another member reviews before merge, task closed only when acceptance criteria are met and the PR link is attached.

## 2. Responsibilities

| Member | Area | Stage-1 deliverables | Stage-2 tasks |
|--------|------|----------------------|---------------|
| Kaiyrbekuly Aitbek (lead) | Architecture, event contract, platform core (Go), infrastructure, CI, Windows agent, reviews, submissions | LMS-1…4, 11, 21, 22, 26…32 | LMS-12…14 core services; JSON schemas |
| Shadiyarbek Syrym | Requirements, UML, UX, personalized analytics, variant-leak detection | LMS-5…7, 20, 23, 25 | LMS-15, 16 analytics module and dashboards |
| Akhanov Yerdaulet | ML: integrity, anomaly detection, proctoring, datasets, evaluation | LMS-8…10, 17…19, 24, 25 | PAN-PC-11 evaluation, ablations, AI-text experiments |

## 3. Milestones

| # | Milestone | Date | Deliverables | Status |
|---|-----------|------|--------------|--------|
| M1 | Team, topic and supervision plan approved | 14 Oct 2026 | Justification signed by the supervisor | Done |
| M2 | Probationary-period submission | 24 Oct 2026 | Tag `prob-period-1`, consolidated report, form | In progress |
| M3 | Technical specification and architecture approved by the supervisor | 30 Nov 2026 | Supervisor feedback recorded, v1.0 of TS and events.md | Planned |
| M4 | MVP demonstration | 31 Jan 2027 | Core services + Kafka running; dashboard prototype; baselines evaluated on open data; draft chapters 1–2 | Planned |
| M5 | Pre-defense 1 | per School schedule (Feb–Mar 2027) | Tag `pre-defense-1`, integrated AI modules | Planned |
| M6 | Integrated version | 31 Mar 2027 | Recommendations, exam sessions, client agent, proctoring pilot | Planned |
| M7 | Evaluation complete | 30 Apr 2027 | Pilot course, false-positive evaluation, full thesis text, plagiarism check | Planned |
| M8 | Pre-defense 2 / final pre-defense | May 2027 | Tags `pre-defense-2`, `final-pre-defense` | Planned |
| M9 | Defense | May–June 2027 | Presentation, demo | Planned |

Control: weekly supervisor meeting; monthly progress report with a working demo; checkpoint review of this plan at every milestone.

## 4. Risk register

| # | Risk | Likelihood | Impact | Mitigation | Owner |
|---|------|------------|--------|------------|-------|
| R1 | Platform core takes longer than stage 2 allows (10 services) | High | High | Prioritize `auth`, `quizzes`, `assignments`, `grading`; reuse EduLMS patterns; `notifications`/`files` minimal; defer `users` groups UI | Kaiyrbekuly |
| R2 | ML modules perform well on synthetic data but poorly on real data (stage-1 shows simple rules beat the forest on synthetic) | High | Medium | Keep rule-based baseline as fallback; pilot early with consented data; report honestly | Akhanov |
| R3 | False positives harm students (non-native writers, fast legitimate solvers) | Medium | High | Human-in-the-loop by design; per-group FPR reporting; abstain on insufficient text; AI-text indicator stays offline until evaluated | Akhanov, Shadiyarbek |
| R4 | Consent and personal-data compliance for webcam/agent not approved by the university in time | Medium | High | Consent flow and retention schedule in TS; request institutional review at M3; pilot without proctoring if not approved | Kaiyrbekuly |
| R5 | Open datasets unavailable or licence-restricted (PAN-PC-11 download, BigCloneBench sign-in) | Medium | Medium | Scripts with checksum; alternative corpora listed in datasets.md; synthetic text pairs as fallback | Akhanov |
| R6 | Windows client agent flagged by antivirus or unstable across machines | Medium | Medium | Signed binary, read-only monitoring, explicit allowlist of checks; agent optional per exam | Kaiyrbekuly |
| R7 | Single VPS cannot run Kafka + 4 stores + services | Medium | Medium | KRaft single node, resource limits; fallback to NATS JetStream (D2) | Kaiyrbekuly |
| R8 | Team member overload (course load, GPA conditions of the School) | Medium | High | Weekly task review on the board; tasks sized to ≤ 1 week; redistribute via YouTrack | All |
| R9 | Supervisor feedback arrives late for a checkpoint | Low | Medium | Send materials ≥ 5 days before each checkpoint; record feedback in `docs/10_supervisor_feedback.md` | Kaiyrbekuly |
| R10 | Scope creep (deepfake detection, mobile apps) | Medium | Medium | Out-of-scope list in the proposal; new features only through a task with an owner and a milestone | Kaiyrbekuly |

## 5. Dependencies

| Dependency | Needed for | Status |
|------------|------------|--------|
| Supervisor review of TS and architecture | M3 | Requested (LMS-31) |
| University decision on consent and retention policy | Exam sessions pilot (M6) | To request at M3 |
| PAN-PC-11 and BigCloneBench access | Integrity evaluation (M4) | Download scripts ready; corpora not fetched yet |
| Figma mockups | UX section, frontend work (M4) | LMS-23 in progress |
| Teachers for the pilot course | M7 | To agree with the School in stage 3 |
| VPS for staging | M4 | Existing server (aitbek.tech) to be re-provisioned |

## 6. Change management

Changes to the event contract or the technical specification go through a PR reviewed by all three members; the changelog in `events.md` §6 and the version line of the TS record them. Scope changes require a YouTrack task approved in the weekly meeting and, if they affect milestones, a note to the supervisor.
