# Stage report: pre-defense-1 (probationary period)

Period: 7–24 October 2026. Repository: [Newterios/Examory](https://github.com/Newterios/Examory). Task tracker: [YouTrack, project LMS](https://examory.youtrack.cloud/projects/LMS). Tag: [`pre-defense-1`](https://github.com/Newterios/Examory/tree/pre-defense-1).

Every task has a YouTrack issue with owner, priority, due date, expected outcome and acceptance criteria, and a pull request reviewed by another team member before merging into `main`. Individual contribution is traceable through commit authorship: 26 pull requests merged, 27 feature commits by three authors.

## 1. Deliverables by component of the probationary-period requirements

| # | Component | Deliverable | Owner | Tasks, PRs |
|---|-----------|-------------|-------|------------|
| 1 | Project proposal | [proposal.md](../01_proposal/proposal.md): problem, users, aim, 8 objectives, outcomes, scope and out of scope, complexity, stage plan | Kaiyrbekuly | LMS-22, [#13](https://github.com/Newterios/Examory/pull/13) |
| 2 | Evidence of relevance | [user_research.md](../07_analysis/user_research.md): 7 interviews (3 teachers, 4 students), 9 findings mapped to requirement IDs, 5 assumptions checked, impact-on-requirements table | all three | LMS-25, [#19](https://github.com/Newterios/Examory/pull/19), [#22](https://github.com/Newterios/Examory/pull/22), [#23](https://github.com/Newterios/Examory/pull/23), [#25](https://github.com/Newterios/Examory/pull/25), [#26](https://github.com/Newterios/Examory/pull/26) |
| 3 | Literature review | [plagiarism_detection.md](../05_literature/plagiarism_detection.md), [anomaly_detection.md](../05_literature/anomaly_detection.md), [publications_table.md](../05_literature/publications_table.md), [sources.bib](../05_literature/sources.bib): 29 sources, 25 scientific, comparison tables, MVP decisions | Akhanov | LMS-8, 9, 24, [#4](https://github.com/Newterios/Examory/pull/4), [#5](https://github.com/Newterios/Examory/pull/5), [#21](https://github.com/Newterios/Examory/pull/21) |
| 4 | Existing system comparison | [existing_systems.md](../07_analysis/existing_systems.md): Moodle, Canvas, Turnitin, Proctorio, MOSS/JPlag × 10 criteria, findings, sources | Kaiyrbekuly | LMS-26, [#14](https://github.com/Newterios/Examory/pull/14) |
| 5 | Technology comparison | [technology_comparison.md](../07_analysis/technology_comparison.md): 7 decisions (D1–D6, D3a) with alternatives, criteria and requirement links | Kaiyrbekuly | LMS-27, [#15](https://github.com/Newterios/Examory/pull/15), revised in [#24](https://github.com/Newterios/Examory/pull/24) |
| 6 | Requirements specification | [technical_specification.md](../02_requirements/technical_specification.md) v1.1: 58 FR, 16 NFR, 12 acceptance criteria; [glossary.md](../02_requirements/glossary.md); [user_stories.md](../02_requirements/user_stories.md) | Shadiyarbek | LMS-5, 20, [#1](https://github.com/Newterios/Examory/pull/1), [#12](https://github.com/Newterios/Examory/pull/12) |
| 7 | Architecture and diagrams | [architecture.md](../03_architecture/architecture.md) v0.2, [events.md](../03_architecture/events.md) v0.2; C4 context and component, use case, data model, 3 sequence diagrams as PlantUML + PNG in [docs/04_uml](../04_uml) | Kaiyrbekuly (architecture, contract, C4), Shadiyarbek (UML) | LMS-2, 3, 6, 21, 28, [#2](https://github.com/Newterios/Examory/pull/2), [#10](https://github.com/Newterios/Examory/pull/10), [#16](https://github.com/Newterios/Examory/pull/16) |
| 8 | UX / prototype | Figma mockups of 4 screens + mobile and states ([prototypes](../04_uml/prototypes/)), [dashboards_spec.md](../04_uml/prototypes/dashboards_spec.md); runnable ML baselines in [ml/](../../ml); reference prototype EduLMS | Shadiyarbek (UX), Akhanov (ML), Kaiyrbekuly (EduLMS) | LMS-7, 23, 17, 18, 19, [#3](https://github.com/Newterios/Examory/pull/3), [#20](https://github.com/Newterios/Examory/pull/20), [#7](https://github.com/Newterios/Examory/pull/7), [#8](https://github.com/Newterios/Examory/pull/8), [#9](https://github.com/Newterios/Examory/pull/9) |
| 9 | Verification and evaluation plan | [verification_plan.md](../08_verification/verification_plan.md): research question and hypotheses H1–H3, test levels, metrics per objective, conditions, baselines; [datasets.md](../../ml/notebooks/datasets.md), [baseline_plan.md](../../ml/notebooks/baseline_plan.md) | Kaiyrbekuly, Akhanov | LMS-10, 29, [#6](https://github.com/Newterios/Examory/pull/6), [#17](https://github.com/Newterios/Examory/pull/17) |
| 10 | Project management and risks | [project_management.md](../09_project_management/project_management.md): tools, responsibilities, milestones M1–M9, M4 scenario, 10 risks, dependencies; YouTrack board; CI | Kaiyrbekuly | LMS-1, 4, 30, [#18](https://github.com/Newterios/Examory/pull/18) |
| 11 | Supervisor feedback | [10_supervisor_feedback.md](../10_supervisor_feedback.md): 5 review points of 23 Oct 2026 with decisions, applied revisions and follow-up tasks LMS-34…38 | Kaiyrbekuly | LMS-31, [#24](https://github.com/Newterios/Examory/pull/24) |

## 2. First measured result (synthetic data, 2 000 students, seed 42)

Isolation Forest on final quiz attempts, held-out cohorts, threshold frozen on validation at FPR ≤ 5 % (`ml/anomaly/results.md`):

| Model | Precision | Recall | F1 | FPR | PR-AUC |
|-------|----------:|-------:|---:|----:|-------:|
| Isolation Forest with peer features | 0.93 | 0.53 | 0.68 | 0.023 | 0.83 |
| Isolation Forest without peer features | 0.91 | 0.36 | 0.51 | 0.020 | 0.74 |
| Training-quantile rules | 0.97 | 0.64 | 0.77 | 0.012 | 0.90 |

Peer features make collusion detectable (recall 0.81 vs 0.00); score jumps are poorly detected by all models (recall ≤ 0.19); simple rules beat the forest on this synthetic data. These numbers describe injected synthetic anomalies, not real misconduct, and motivated the research question and the detector-independent evaluation set in the verification plan (supervisor point 4).

## 3. Individual contributions

| Member | Area | Tasks closed | Pull requests |
|--------|------|--------------|---------------|
| Kaiyrbekuly Aitbek | Team lead; proposal, architecture, event contract, C4 diagrams, system and technology comparisons, verification plan, project management, supervisor feedback, infrastructure, CI, review of every PR, submissions | LMS-1, 2, 3, 4, 11, 21, 22, 25 (I1, I4, I5, I7), 26, 27, 28, 29, 30, 31, 32 | [#10](https://github.com/Newterios/Examory/pull/10), [#11](https://github.com/Newterios/Examory/pull/11), [#13](https://github.com/Newterios/Examory/pull/13)–[#19](https://github.com/Newterios/Examory/pull/19), [#22](https://github.com/Newterios/Examory/pull/22), [#24](https://github.com/Newterios/Examory/pull/24), [#25](https://github.com/Newterios/Examory/pull/25), [#26](https://github.com/Newterios/Examory/pull/26) |
| Shadiyarbek Syrym | Requirements, UML, dashboard specification, Figma mockups, interviews | LMS-5, 6, 7, 20, 23, 25 (I2, I3) | [#1](https://github.com/Newterios/Examory/pull/1), [#2](https://github.com/Newterios/Examory/pull/2), [#3](https://github.com/Newterios/Examory/pull/3), [#12](https://github.com/Newterios/Examory/pull/12), [#20](https://github.com/Newterios/Examory/pull/20), [#23](https://github.com/Newterios/Examory/pull/23) |
| Akhanov Yerdaulet | Literature reviews, publication table, datasets and evaluation protocol, synthetic generator, two ML baselines with tests, interview | LMS-8, 9, 10, 17, 18, 19, 24, 25 (I6) | [#4](https://github.com/Newterios/Examory/pull/4)–[#9](https://github.com/Newterios/Examory/pull/9), [#21](https://github.com/Newterios/Examory/pull/21) |

Review activity: every PR reviewed by the team lead; PR #12 and PR #20 received "changes requested" and were revised before approval; the supervisor's five points led to PR #24.

## 4. Known limitations and unfinished materials

- No platform service code yet: Go services, Kafka producers and the web client start in stage 2; EduLMS serves only as an architectural reference.
- ML baselines are evaluated on synthetic data generated along the detector's own features; the PAN-PC-11 run and the detector-independent evaluation set are pending (LMS-35).
- AI-text detection and video proctoring remain at literature level; FR-52 is `should` with a fixture.
- Source of real anonymized quiz logs and the data-processing approval request are to be decided at the next weekly meeting (LMS-36).
- The resource fit of the full Compose stack on one VPS is unmeasured (LMS-34).
- Open contract items: `quiz.variant_replaced` (LMS-37) and JSON Schemas for events; Kafka schema registry decision deferred.
- Interviews cover one university; findings are indicative, not statistically representative.

## 5. Next stage (December 2026 – January 2027, milestone M4)

- M4 end-to-end scenario on staging: quiz → Kafka → anomaly model → `risk.behavior_anomaly` → teacher review → `risk.reviewed`, with the FR subset in project_management.md §3a (LMS-38; services `auth`, `users`, `courses`, `quizzes`, `grading` by Kaiyrbekuly, LMS-12…14; web client screens by Shadiyarbek).
- Analytics module skeleton with Kafka consumers and dashboard prototypes (LMS-15, 16, Shadiyarbek).
- PAN-PC-11 evaluation of the text baseline, detector-independent anomaly evaluation, research question H1 tested (LMS-35, Akhanov).
- Event contract v0.3 and JSON Schemas (LMS-37); VPS measurement (LMS-34); real-data source and approval request (LMS-36).
- Supervisor checkpoint M3 (30 Nov 2026): TS v1.1 and architecture v0.2 approval; draft thesis chapters 1–2 by M4.
