# Stage report: prob-period-1

Period: October – November 2026. Tag: `prob-period-1`.

## Completed

<!-- Update before tagging. One bullet per deliverable with a link to the file or PR. -->

- Team roles, supervision plan and calendar plan agreed with the supervisor ([docs/01_justification](../01_justification)).
- Shared repository, folder structure, CI and YouTrack board set up (LMS-1).
- Microservice architecture description ([architecture.md](../03_architecture/architecture.md), LMS-2).
- Learning-activity event contract for Kafka ([events.md](../03_architecture/events.md), LMS-3).
- Local infrastructure in Docker Compose (LMS-4).
- Functional requirements and technical specification draft (LMS-5) — _pending_.
- Use case, data model and sequence diagrams (LMS-6) — _pending_.
- Figma prototypes of student and teacher dashboards (LMS-7) — _pending_.
- Literature review on plagiarism detection (LMS-8) — _pending_.
- Literature review on behavioral anomaly detection (LMS-9) — _pending_.
- Dataset selection and baseline experiment plan (LMS-10) — _pending_.

## Individual contributions

| Member | Area | Tasks | Links |
|--------|------|-------|-------|
| Kaiyrbekuly Aitbek | Architecture, event streaming, DevOps | LMS-1, LMS-2, LMS-3, LMS-4 | |
| Shadiyarbek Syrym | Requirements, UML, UI prototypes, analytics | LMS-5, LMS-6, LMS-7 | |
| Akhanov Yerdaulet | ML: integrity and anomaly detection | LMS-8, LMS-9, LMS-10 | |

## Known limitations

- No platform code yet: the architecture and event contract are documents, the earlier EduLMS prototype is used only as a reference and is being rewritten from scratch.
- No ML models trained; only literature review and experiment plans.
- Technical specification and literature review are drafts awaiting supervisor review.
- Synthetic datasets for anomaly detection are not generated yet.
- Open architecture questions listed in architecture.md §6 are not decided.

## Next stage (December 2026 – January 2027)

- Platform core: `auth`, `users`, `courses`, `assignments`, `quizzes` services and Kafka producers (Kaiyrbekuly).
- Analytics dashboards prototype and metric aggregation over `lms.quizzes` and `lms.submissions` (Shadiyarbek).
- Baseline plagiarism (MinHash) and anomaly (Isolation Forest) models with precision / recall / false-positive evaluation on synthetic and open data (Akhanov).
- MVP demonstration to the supervisor; draft of thesis chapters 1–2.
