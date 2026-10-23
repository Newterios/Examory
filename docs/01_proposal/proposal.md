# Project proposal

**Project title:** Development of an AI-Powered LMS Platform for Academic Integrity, Personalized Learning Analytics, and Anomaly Detection (working name: Examory)

**Team (SE-2424):** Kaiyrbekuly Aitbek (team lead; architecture, platform core, infrastructure), Shadiyarbek Syrym (requirements, UX, personalized analytics), Akhanov Yerdaulet (machine learning: integrity, anomaly detection, proctoring)

**Supervisor:** Sakhipov A.A., PhD, Assistant Professor, School of Software Engineering, Astana IT University

**Repository:** https://github.com/Newterios/Examory · **Task tracker:** https://examory.youtrack.cloud/projects/LMS

## 1. Problem statement

Universities run assessment through learning management systems that record what students submit but not how they work. Three problems follow from this.

1. **Academic integrity is checked late and narrowly.** Text similarity tools (where licensed) run on final submissions only; code similarity is rarely checked; generative AI makes paraphrased or generated text invisible to classic similarity search. Teachers receive a percentage without an explanation and without context from the student's earlier work.
2. **Learning analytics are shallow.** Standard LMS dashboards show grades and completion, not which topics a student is weak in, which materials would help, or which students in a group are drifting toward failure early enough to intervene.
3. **Anomalies in online assessment go unnoticed.** Unusually fast answering, synchronized submissions, sudden score jumps and leaked test variants leave traces in interaction data, but no common LMS collects that data as events or analyses it.

Existing commercial add-ons (similarity checkers, proctoring services) are separate products with closed models, opaque scores and no shared data model, so a university cannot combine their signals or explain a decision to a student.

## 2. Target users

| User | Needs |
|------|-------|
| **Students** | Fair assessment; clear progress view; concrete recommendations on what to study; transparency about what is monitored and why. |
| **Teachers** | Early, explained risk signals on submissions and quizzes; group-level view of at-risk students; rubric-based grade suggestions that save time but leave the decision to them. |
| **Academic administrators** | Consistent integrity policy across courses; audit trail of decisions; control of data retention and consent. |

Primary pilot context: courses of the School of Software Engineering at Astana IT University (programming assignments, quizzes, project reports).

## 3. Aim

Design and implement an event-driven learning management platform in which learning activity is collected as a stream of events and used by three explainable AI modules (academic integrity, personalized analytics, anomaly detection) that produce risk signals and recommendations for human review, never automatic verdicts.

## 4. Objectives

1. Define the platform architecture and a shared Kafka event contract that connects the LMS core with the AI modules (done in stage 1: `docs/03_architecture/`).
2. Specify functional and non-functional requirements with acceptance criteria, including consent, privacy and human-in-the-loop constraints (done: `docs/02_requirements/`).
3. Implement the LMS core: authentication and RBAC, courses and materials, assignments and submissions, quizzes with variant banks, grading with rubrics, notifications, file storage (stage 2).
4. Implement the academic-integrity module: text and code similarity with explainable evidence spans; AI-generated-text indicators as signals only (baseline done; evaluation on PAN-PC-11 in stage 2).
5. Implement the anomaly-detection module for quiz behaviour (timing, answer changes, synchronous submissions, score jumps) and test-variant leak detection (baseline done on synthetic data).
6. Implement personalized analytics: weak topics, material recommendations, at-risk students, rubric-based grade suggestions (stage 2–3).
7. Implement consent-based exam sessions: webcam analysis and a Windows client agent that reports risk signals (stage 3).
8. Evaluate every AI module with precision, recall and false-positive rate on synthetic and open datasets, and with teacher review decisions as labels in a pilot course (stage 4).

## 5. Expected outcomes

- A deployable platform (Docker Compose) with the LMS core, event bus and three AI modules connected through the event contract.
- Measured quality of the integrity and anomaly modules: precision, recall, false-positive rate, with explicit limits (what is and is not detected).
- A pilot course scenario demonstrated end to end: submission → risk signal → teacher review → decision, with audit trail.
- Thesis documentation: requirements, architecture, diagrams, evaluation, and a user guide.
- Intermediate artifacts already delivered in stage 1: architecture and event contract v0.2, technical specification (56 FR, 16 NFR, 12 acceptance criteria), UML models, dashboard specification, literature reviews (29 sources), dataset and evaluation plan, synthetic data generator, two working ML baselines.

## 6. Scope

**In scope (diploma project):**
- LMS core for one institution: users, groups, courses, materials, assignments, quizzes, grading, notifications, files.
- Event streaming of learning activity and the AI modules listed above, with explainable outputs and teacher review workflow.
- Student and teacher dashboards; administrator functions limited to users, groups, consent policy and retention.
- Exam sessions with webcam presence analysis and a Windows client agent (process, virtual camera, remote access, VM and screen-recorder detection) during consented exams only.
- Evaluation on synthetic and open datasets plus one pilot course with consented participants.

**Out of scope:**
- Mobile applications; macOS/Linux client agents.
- Integration with university information systems (student registry, timetable) beyond CSV import.
- Legally binding misconduct decisions: the platform supplies evidence for human review only.
- Deepfake detection beyond indicator-level signals; biometric identity verification.
- Multi-tenant SaaS operation, billing, Kubernetes deployment.

## 7. Engineering complexity

Event-driven microservice architecture (Go services, Kafka, PostgreSQL, MongoDB, Redis, MinIO); three ML pipelines with different data modalities (text and code, interaction time series, video); strict requirements on false positives, explainability and personal-data handling; a native Windows agent; and evaluation methodology with cohort-level splits and teacher adjudication as labels.

## 8. Stage plan (summary)

| Stage | Period | Checkpoint |
|-------|--------|------------|
| 1. Analysis and design | Oct–Nov 2026 | This submission (tag `pre-defense-1`) |
| 2. MVP | Dec 2026 – Jan 2027 | Platform core + event bus, dashboard prototypes, baseline models evaluated on open data |
| 3. Integration | Feb–Mar 2027 | AI modules integrated, recommendations, exam sessions, client agent |
| 4. Evaluation | Apr 2027 | Pilot course, false-positive evaluation, full thesis text |
| 5. Defense | May 2027 | Pre-defense, corrections, presentation and demo |

Details: `docs/09_project_management/project_management.md`.
