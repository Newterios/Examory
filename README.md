# Examory

**AI-Powered LMS Platform for Academic Integrity, Personalized Learning Analytics, and Anomaly Detection**

Разработка интеллектуальной LMS-платформы на основе ИИ для обеспечения академической добросовестности, персонализированной аналитики и выявления аномалий

Академиялық адалдықты қамтамасыз етуге, дербестендірілген оқу аналитикасына және аномалияларды анықтауға арналған жасанды интеллект негізіндегі зияткерлік LMS-платформасын әзірлеу

Diploma project, Astana IT University, School of Software Engineering, educational program **Software Engineering**, 2026–2027.

## Team

| # | Member | Group | Role and area of responsibility | GitHub |
|---|--------|-------|---------------------------------|--------|
| 1 | Kaiyrbekuly Aitbek | SE-2424 | Team lead. Platform architecture, microservices, event streaming, exam session client agent, CI/CD and deployment | [@Newterios](https://github.com/Newterios) |
| 2 | Shadiyarbek Syrym | SE-2424 | Requirements and UML modeling, UI prototypes, personalized learning analytics, student and teacher dashboards, test-variant leak detection | [@SyrymSh](https://github.com/SyrymSh) |
| 3 | Akhanov Yerdaulet | SE-2424 | ML modules: plagiarism and AI-generated text detection, behavioral anomaly detection, online proctoring, inference service | [@SuperFlyTK](https://github.com/SuperFlyTK) |

**Supervisor:** Sakhipov A.A., PhD, Assistant Professor, School of Software Engineering.

## Project goal

Examory is a learning management system in which every learning action of a student (submissions, quiz answers, logins, exam sessions) is collected as a stream of events. Three AI modules consume this stream:

1. **Academic integrity** — similarity of submitted texts and code, indicators of AI-generated text, online proctoring signals.
2. **Personalized learning analytics** — weak topics, material recommendations, group risk dashboards for teachers, rubric-based recommended grades.
3. **Anomaly detection** — unusual answer timing and patterns, synchronous submissions, sudden score jumps, suspicious processes during exams.

All AI outputs are **risk signals, not verdicts**. The final decision on any violation remains with the teacher (human-in-the-loop).

## Architecture overview

Event-driven microservice architecture. See [docs/03_architecture/architecture.md](docs/03_architecture/architecture.md) and the event contract in [docs/03_architecture/events.md](docs/03_architecture/events.md).

| Layer | Technology |
|-------|------------|
| Platform services | Go (Gin), REST + gRPC between services |
| Web client | Next.js, TypeScript |
| Analytics module | Node.js / Express, Chart.js / Recharts, Python (pandas) |
| ML services | Python (scikit-learn, PyTorch, OpenCV, MediaPipe), ONNX Runtime, Rust (Axum) inference |
| Exam session client agent | Go, Windows API (WMI) |
| Data | PostgreSQL, MongoDB, Redis |
| Event bus | Apache Kafka |
| Object storage | MinIO (files, exam video recordings) |
| Infrastructure | Docker, Nginx (API gateway), GitHub Actions |

## Repository structure

```
docs/
  01_justification/   supervisor's justification document (RU)
  02_requirements/    technical specification, glossary, functional requirements
  03_architecture/    architecture description, event schema, ADRs
  04_uml/             use case, data model and sequence diagrams (PlantUML + PNG)
  05_literature/      literature review and sources
  06_reports/         stage reports (prob-period-1, ...)
services/             Go microservices of the platform core (one folder per service)
analytics/            personalized learning analytics module
ml/                   ML experiments, notebooks, model training and inference services
.github/workflows/    CI
docker-compose.yml    local infrastructure (PostgreSQL, MongoDB, Redis, Kafka, MinIO)
```

## Project status

| Stage | Period | Deliverables | Status |
|-------|--------|--------------|--------|
| 1. Analysis and design | Oct – Nov 2026 | Domain analysis and literature review, technical specification, architecture, event schema, UML models, task allocation | **in progress** |
| 2. MVP | Dec 2026 – Jan 2027 | Platform core and event bus, analytics dashboard prototypes, baseline plagiarism and anomaly models, draft chapters 1–2 | planned |
| 3. Integration | Feb – Mar 2027 | AI modules integrated with the platform, recommendations, exam client agent, proctoring, test-variant leak detection, experiments | planned |
| 4. Evaluation | Apr 2027 | Testing, false-positive evaluation, pilot course scenario, full thesis text | planned |
| 5. Defense | May 2027 | Pre-defense, corrections, presentation and demo | planned |

Detailed stage reports: [docs/06_reports](docs/06_reports).

## Task tracking

Tasks are tracked in YouTrack, project key `LMS`. Stage-1 tasks are listed in `scripts/youtrack_seed.py` and in [docs/06_reports/prob-period-1.md](docs/06_reports/prob-period-1.md). Every branch and commit references a task ID, e.g. `LMS-6: add use case diagram`.


YouTrack: project [LMS](https://examory.youtrack.cloud/projects/LMS) · [Kanban board](https://examory.youtrack.cloud/agiles/204-1/current) · [issues](https://examory.youtrack.cloud/issues?q=project:%20LMS)

## Workflow

- One task = one branch (`LMS-12-event-schema`) = one pull request into `main`.
- Commit messages start with the task ID.
- Every pull request is reviewed by another team member before merging.
- Diagrams are stored as PlantUML source plus exported PNG; documents are written in Markdown, DOCX/PDF are exported only for submission.

## How to run

The platform core is under development. Local infrastructure can be started with:

```bash
docker compose up -d
```

This brings up PostgreSQL, MongoDB, Redis, Kafka and MinIO. Service-specific instructions will be added to each service's README as they appear.
