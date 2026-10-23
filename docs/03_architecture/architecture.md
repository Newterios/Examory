# Examory — System Architecture

Status: draft v0.2 (23 October 2026, after supervisor review). Owner: Kaiyrbekuly Aitbek.

**Document precedence.** When documents disagree, the order of truth is: `events.md` (event contract) → `technical_specification.md` (requirements) → this document (architecture) → `proposal.md` (scope). A conflict is fixed by changing the lower-ranked document in the same pull request.

## 1. Principles

1. **Event-driven.** Every meaningful learning action is published to Apache Kafka as an immutable event. Platform services write their own state to their own database and emit events; AI modules only consume events and publish risk signals back.
2. **Microservices with a single entry point.** Clients talk only to the API gateway (Nginx). Services communicate synchronously through REST/gRPC only when a request needs an immediate answer; everything else goes through Kafka.
3. **Human-in-the-loop.** No AI module changes a grade or blocks a student. AI modules produce `risk.*` events with a score and an explanation; the teacher sees them in the dashboard and makes the decision.
4. **Database per service.** No service reads another service's tables. Shared data is obtained through APIs or events.
5. **Privacy by design.** Video, webcam frames and exam-agent telemetry are collected only during an exam session, only with explicit student consent, and have a retention period.

## 2. Context

Editable diagrams: [context.puml](../04_uml/context.puml) (system context, C4) and [component.puml](../04_uml/component.puml) (component diagram, draft); PNG exports next to them. Technology decisions D1–D6 are justified in [technology_comparison.md](../07_analysis/technology_comparison.md).

![System context](../04_uml/context.png)

![Components](../04_uml/component.png)

```
Students, Teachers, Administrators
          │ HTTPS
          ▼
   ┌──────────────┐        ┌──────────────────────┐
   │ Next.js web  │        │ Exam client agent    │  (Windows, Go)
   └──────┬───────┘        └──────────┬───────────┘
          │                           │
          ▼                           ▼
   ┌──────────────────── Nginx API gateway ───────────────────┐
   │ auth │ users │ courses │ assignments │ quizzes │ grading  │
   │ notifications │ files │ exam-sessions │ analytics-api     │
   └───────────────────────────┬──────────────────────────────┘
                               │ events
                               ▼
                         Apache Kafka
                 ┌─────────────┼───────────────┐
                 ▼             ▼               ▼
         ML integrity    ML anomaly      Analytics module
         (Python)        (Python/Rust)   (Node.js + pandas)
                 └─────────────┴───────────────┘
                               │ risk.* / analytics.* events
                               ▼
                    grading / dashboards (teacher)
```

## 3. Services

### 3.1 Platform core (Go / Gin) — Kaiyrbekuly A.

| Service | Responsibility | Store | Publishes |
|---------|----------------|-------|-----------|
| `auth` | Registration, login, JWT access/refresh tokens (refresh in Redis), RBAC roles (student, teacher, admin, …) | PostgreSQL, Redis | `user.logged_in`, `user.logged_out` |
| `users` | Profiles, groups, enrollment of students into groups | PostgreSQL | `user.created`, `group.updated` |
| `courses` | Courses, modules, learning materials, enrollment of groups into courses | PostgreSQL | `course.*`, `material.viewed` |
| `assignments` | Assignments, submissions (text, files, code), deadlines, versions | PostgreSQL + MinIO | `assignment.published`, `submission.created`, `submission.updated` |
| `quizzes` | Question bank, test variants, attempts, per-question answers with timing | PostgreSQL | `quiz.started`, `quiz.answered`, `quiz.submitted` |
| `grading` | Rubrics, grades, weighted grade engine, teacher decisions on risk signals | PostgreSQL | `grade.assigned`, `risk.reviewed` |
| `notifications` | E-mail / in-app notifications, consumer of many topics | MongoDB | — |
| `files` | Upload/download through MinIO, presigned URLs, antivirus hook | MinIO | `file.uploaded` |
| `exam-sessions` | Exam session lifecycle, student consent, link between quiz attempt, webcam stream and client agent telemetry | PostgreSQL + MinIO (video) | `exam.started`, `exam.ended`, `exam.agent_signal` |
| `event-collector` | Thin HTTP/gRPC endpoint for the web client and the exam agent to push client-side events (focus lost, tab switched, paste) | — | `client.*` |

### 3.2 Exam session client agent (Go, Windows) — Kaiyrbekuly A.

Runs only during an exam session after the student's consent. Checks running processes, virtual cameras, remote-access tools, virtual machines and screen recorders through WMI and sends `exam.agent_signal` events. It never blocks anything on the student's machine; it only reports.

### 3.3 Personalized analytics module (Node.js / Express, Python) — Shadiyarbek S.

Consumes `quiz.*`, `submission.*`, `grade.*`, `material.viewed`. Aggregates per student and per group in MongoDB/PostgreSQL. Produces:

- weak topics and material recommendations for the student dashboard (`analytics.recommendation`);
- group performance and at-risk students for the teacher dashboard (`analytics.group_risk`);
- recommended grade by rubric criteria with explanation and a summary of the student's previous work (`analytics.grade_suggestion`);
- test-variant leak detection by abnormal growth of pass rate per variant (`risk.variant_leak`). The signal is a candidate for teacher review only (FR-43–FR-45). Replacing the variant from the question bank is an explicit teacher action in `quizzes` after the signal is confirmed; it will publish `quiz.variant_replaced` with the confirming `risk_id` (events.md v0.3, LMS-37) and is recorded in the audit log.

### 3.4 ML modules (Python, Rust) — Akhanov Y.

| Module | Input events | Method | Output |
|--------|--------------|--------|--------|
| Plagiarism (text and code) | `submission.created` | shingles / MinHash, sentence embeddings, token-based code similarity | `risk.plagiarism` with matched submissions and similarity score |
| AI-generated text indicators | `submission.created` | perplexity / stylometric features, classifier | `risk.ai_generated` (signal only) |
| Behavioral anomalies in quizzes | `quiz.answered`, `quiz.submitted`, `client.*` | Isolation Forest, autoencoders on answer timing, answer patterns, score jumps, synchronous submissions | `risk.behavior_anomaly` |
| Proctoring (*should*, FR-52) | webcam frames from `exam-sessions`, `exam.agent_signal` | OpenCV, MediaPipe (face presence; gaze proxy exploratory). Deepfake detection is **out of scope** (proposal §6); only agent-reported virtual-camera signals are used | `risk.proctoring`; a fixture is used for acceptance while the model is not deployed |
| Inference service | — | ONNX Runtime served by Rust (Axum) for low-latency scoring | HTTP API used by the modules above |

Each module is evaluated on synthetic and open datasets with precision, recall and false-positive rate before integration.

## 4. Data flow example: assignment submission

1. Student uploads a file in the web client → `files` stores it in MinIO → `file.uploaded`.
2. Student submits → `assignments` saves the submission → `submission.created` with file references and `text_ref`, a MinIO pointer to the extracted text (the text itself is never embedded in the event).
3. ML integrity consumes the event, compares with the corpus, publishes `risk.plagiarism` (score, matched submission IDs, explanation).
4. Analytics consumes the event, updates student progress, publishes `analytics.grade_suggestion` when the teacher opens grading.
5. `grading` stores risk signals next to the submission; the teacher sees them, decides, and `risk.reviewed` is published with the decision (confirmed / dismissed). Reviewed decisions become labelled data for evaluating false positives.

## 5. Infrastructure

- Docker Compose for local development (`docker-compose.yml` in repository root), one container per service.
- Nginx as API gateway and TLS termination.
- GitHub Actions: build and test on every pull request; image build on tags.
- Production target: a single VPS (4 vCPU, 8 GB) with Docker Compose for the MVP; Kubernetes is out of scope. Memory and CPU of the full stack are measured before M4 (LMS-34); if it does not fit, MongoDB folds into PostgreSQL JSONB first, then Kafka is replaced by NATS JetStream (technology_comparison.md D2, D3).

## 6. Decisions and open questions

| Question | Status |
|----------|--------|
| gRPC vs REST between services | Decided: REST/JSON at the gateway, gRPC between core services (technology_comparison.md D4) |
| Where extracted submission text is stored | Decided: MinIO, referenced by `text_ref` in `submission.created` (events.md §3) |
| Retention of exam recordings and agent telemetry | Decided as a proposal: 7 days after `exam.ended`, pending university approval (TS §6.2) |
| Automatic variant replacement | Decided: never automatic; teacher action with `quiz.variant_replaced` (LMS-37) |
| ML serving in Rust/ONNX | Deferred to *could*: Python serving for the MVP, Rust only if measurements require it (technology_comparison.md D5, LMS-34) |
| Schema registry for Kafka events | Open: JSON Schema files in `docs/03_architecture/schemas/` first; registry revisited when a third producer appears |
| Resource fit of the full stack on one VPS | Open until measured (LMS-34, before M4) |
