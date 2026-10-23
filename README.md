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

## Problem statement

Learning management systems record what students submit but not how they work. Integrity checks run late and only on text, learning analytics stop at grades and completion, and anomalies in online quizzes (unusually fast answers, synchronized submissions, leaked variants) leave traces that nobody analyses. Commercial add-ons for similarity and proctoring are closed, opaque and cannot be combined. Full proposal: [docs/01_proposal/proposal.md](docs/01_proposal/proposal.md).

**Target users:** students, teachers and academic administrators of a university; pilot context is the School of Software Engineering at AITU.

**Expected outcomes:** a deployable event-driven LMS with three explainable AI modules; measured precision, recall and false-positive rate of the integrity and anomaly modules; a pilot course scenario from submission to teacher decision with an audit trail; thesis documentation.

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

## Documentation map

| Component | Document |
|-----------|----------|
| Project proposal and scope | [docs/01_proposal/proposal.md](docs/01_proposal/proposal.md) |
| Evidence of relevance (user interviews) | [docs/07_analysis/user_research.md](docs/07_analysis/user_research.md) |
| Literature review | [docs/05_literature/](docs/05_literature/) |
| Existing system comparison | [docs/07_analysis/existing_systems.md](docs/07_analysis/existing_systems.md) |
| Technology comparison | [docs/07_analysis/technology_comparison.md](docs/07_analysis/technology_comparison.md) |
| Requirements specification | [docs/02_requirements/](docs/02_requirements/) |
| Architecture and diagrams | [docs/03_architecture/](docs/03_architecture/), [docs/04_uml/](docs/04_uml/) |
| UX / prototype | [docs/04_uml/prototypes/](docs/04_uml/prototypes/), ML baselines in [ml/](ml/), reference prototype EduLMS: https://github.com/Newterios/educationalLMS |
| Verification and evaluation plan | [docs/08_verification/verification_plan.md](docs/08_verification/verification_plan.md) |
| Project management and risks | [docs/09_project_management/project_management.md](docs/09_project_management/project_management.md) |
| Supervisor feedback | [docs/10_supervisor_feedback.md](docs/10_supervisor_feedback.md) |
| Stage reports | [docs/06_reports/pre-defense-1.md](docs/06_reports/pre-defense-1.md) |

## Repository structure

```
docs/
  01_justification/   supervisor's justification document (RU)
  01_proposal/        project proposal and scope
  02_requirements/    technical specification, glossary, user stories
  03_architecture/    architecture description, event contract
  04_uml/             C4 context and component, use case, data model, sequence diagrams (PlantUML + PNG), UX prototypes
  05_literature/      literature reviews, publication table, sources.bib
  06_reports/         stage reports (pre-defense-1, ...)
  07_analysis/        user research, existing system and technology comparisons
  08_verification/    verification and evaluation plan
  09_project_management/  milestones, responsibilities, risks
  10_supervisor_feedback.md
services/             Go microservices of the platform core (one folder per service)
analytics/            personalized learning analytics module
ml/                   ML experiments, notebooks, model training and inference services
.github/workflows/    CI
docker-compose.yml    local infrastructure (PostgreSQL, MongoDB, Redis, Kafka, MinIO)
```

## Project status

| Stage | Period | Deliverables | Status |
|-------|--------|--------------|--------|
| 1. Analysis and design | Oct – Nov 2026 | Proposal, user research, literature review, system and technology comparisons, technical specification, architecture, diagrams, UX mockups, verification plan, project management, supervisor review | **submitted 24 Oct 2026 (tag `pre-defense-1`)**; supervisor approval checkpoint M3 on 30 Nov |
| 2. MVP | Dec 2026 – Jan 2027 | Platform core and event bus, analytics dashboard prototypes, baseline plagiarism and anomaly models, draft chapters 1–2 | planned |
| 3. Integration | Feb – Mar 2027 | AI modules integrated with the platform, recommendations, exam client agent, proctoring, test-variant leak detection, experiments | planned |
| 4. Evaluation | Apr 2027 | Testing, false-positive evaluation, pilot course scenario, full thesis text | planned |
| 5. Defense | May 2027 | Pre-defense, corrections, presentation and demo | planned |

Detailed stage reports: [docs/06_reports](docs/06_reports).

## Task tracking

Tasks are tracked in YouTrack, project key `LMS`. Stage-1 tasks are listed in `scripts/youtrack_seed.py` and in [docs/06_reports/pre-defense-1.md](docs/06_reports/pre-defense-1.md). Every branch and commit references a task ID, e.g. `LMS-6: add use case diagram`.


YouTrack: project [LMS](https://examory.youtrack.cloud/projects/LMS) · [Kanban board](https://examory.youtrack.cloud/agiles/204-1/current) · [issues](https://examory.youtrack.cloud/issues?q=project:%20LMS)

## Workflow

- One task = one branch (`LMS-12-event-schema`) = one pull request into `main`.
- Commit messages start with the task ID.
- Every pull request is reviewed by another team member before merging.
- Diagrams are stored as PlantUML source plus exported PNG; documents are written in Markdown, DOCX/PDF are exported only for submission.

## How to run and test

The platform core is under development (stage 2). What runs today:

```bash
# local infrastructure: PostgreSQL, MongoDB, Redis, Kafka, Kafka UI (localhost:8080), MinIO
docker compose up -d

# ML baselines (Python 3.12)
python -m pip install -r ml/requirements.txt
python ml/datasets/generate_quiz_behavior.py --students 2000 --seed 42 --output ml/datasets/raw/quiz.csv
python -m ml.anomaly.split --csv ml/datasets/raw/quiz.csv --seed 42
python -m ml.anomaly.train --csv ml/datasets/raw/quiz.csv
python -m ml.anomaly.evaluate --csv ml/datasets/raw/quiz.csv --bootstrap 1000
python -m unittest discover -s ml/tests -v
```

Details: [ml/anomaly/README.md](ml/anomaly/README.md), [ml/integrity/README.md](ml/integrity/README.md). Service-specific instructions will be added to each service's README as they appear.

## Reference prototype

EduLMS (https://github.com/Newterios/educationalLMS) is a pre-diploma LMS prototype by Kaiyrbekuly A. (Go microservices, Nginx gateway, JWT with Redis, RBAC, Kafka, MinIO, CI/CD). It serves as the architectural reference for Examory; the diploma platform is reimplemented from scratch around the event-driven design in `docs/03_architecture/`.
