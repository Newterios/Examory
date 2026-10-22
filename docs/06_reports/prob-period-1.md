# Stage report: prob-period-1

Period: October – November 2026. Repository: [Newterios/Examory](https://github.com/Newterios/Examory). Task tracker: [YouTrack, project LMS](https://examory.youtrack.cloud/projects/LMS). Tag: [`prob-period-1`](https://github.com/Newterios/Examory/tree/prob-period-1) (all materials below are in `main` at this tag).

Every task below has a YouTrack issue with the same ID and a pull request reviewed by another team member before merging into `main`. Individual contribution is traceable through commit authorship.

## Completed

### Planning and architecture (Kaiyrbekuly Aitbek)

- Team roles, supervision plan and calendar plan agreed with the supervisor ([docs/01_justification](../01_justification)).
- LMS-1 — Shared repository, folder structure, README, CI (GitHub Actions), YouTrack project and Kanban board.
- LMS-2 — Microservice architecture: principles, context, ten core services with responsibilities and stores, AI modules, data flow, infrastructure ([architecture.md](../03_architecture/architecture.md)).
- LMS-3 — Learning-activity event contract for Kafka: envelope, eight topics, payload of every platform and AI event ([events.md](../03_architecture/events.md)).
- LMS-4 — Local infrastructure in Docker Compose: PostgreSQL, MongoDB, Redis, Kafka (KRaft), Kafka UI, MinIO ([docker-compose.yml](../../docker-compose.yml)).
- LMS-21 — Event contract v0.2: counting conventions, evidence formats, `risk.variant_leak` and `analytics.*` item structures raised by the reviews below ([PR #10](https://github.com/Newterios/Examory/pull/10)).

### Requirements, modeling and interfaces (Shadiyarbek Syrym)

- LMS-5 — Technical specification: 56 functional requirements (FR-1…FR-56) across five modules, 16 non-functional requirements, consent and retention constraints, 12 MVP acceptance criteria; glossary; user stories for student, teacher and admin ([PR #1](https://github.com/Newterios/Examory/pull/1), [docs/02_requirements](../02_requirements)).
- LMS-6 — UML: use case diagram, conceptual data model (16 entities), three sequence diagrams (submission with integrity review, quiz attempt with anomaly review, exam session with proctoring), PlantUML sources with PNG exports ([PR #2](https://github.com/Newterios/Examory/pull/2), [docs/04_uml](../04_uml)).
- LMS-7 — Dashboard specification for Figma: student and teacher screens, blocks with data provenance per event, interactions, states, metric definitions, access rules ([PR #3](https://github.com/Newterios/Examory/pull/3), [dashboards_spec.md](../04_uml/prototypes/dashboards_spec.md)).
- LMS-20 — Technical specification §4, dashboard specification and the three sequence diagrams aligned with event contract v0.2; PNGs regenerated ([PR #12](https://github.com/Newterios/Examory/pull/12)).

### Machine learning (Akhanov Yerdaulet)

- LMS-8 — Literature review on plagiarism and AI-text detection: MinHash/LSH, TF-IDF, sentence embeddings, winnowing/MOSS, JPlag, AST, CodeBERT, perplexity/DetectGPT, watermarking, stylometry, non-native-writer bias; comparison table, MVP decision, 14 sources ([PR #4](https://github.com/Newterios/Examory/pull/4), [plagiarism_detection.md](../05_literature/plagiarism_detection.md)).
- LMS-9 — Literature review on behavioral anomaly detection and proctoring: Isolation Forest, One-Class SVM, LOF, autoencoders, answer-copying statistics, MediaPipe, virtual cameras, deepfake detection, ethical and legal constraints (Kazakhstan Law 94-V, EU AI Act); 15 sources ([PR #5](https://github.com/Newterios/Examory/pull/5), [anomaly_detection.md](../05_literature/anomaly_detection.md)).
- LMS-10 — Dataset selection (PAN-PC-11, BigCloneBench, HC3, EdNet, synthetic) and evaluation protocol: cohort-level splits, metrics, teacher reviews as labels ([PR #6](https://github.com/Newterios/Examory/pull/6), [datasets.md](../../ml/notebooks/datasets.md), [baseline_plan.md](../../ml/notebooks/baseline_plan.md)).
- LMS-19 — Reproducible synthetic quiz-behavior generator emitting `quiz.answered` / `quiz.submitted` events with injected anomalies (rapid response, excessive changes, collusion, score jump) ([PR #7](https://github.com/Newterios/Examory/pull/7), [generate_quiz_behavior.py](../../ml/datasets/generate_quiz_behavior.py)).
- LMS-18 — Behavioral anomaly baseline (stage-2 task delivered early): feature extraction, cohort-disjoint split, Isolation Forest with and without peer features, quantile-rule comparator, validation-frozen thresholds, held-out evaluation with bootstrap intervals, conversion to `risk.behavior_anomaly` ([PR #8](https://github.com/Newterios/Examory/pull/8), [ml/anomaly](../../ml/anomaly)).
- LMS-17 — Text reuse baseline (stage-2 task delivered early): five-token shingles, 128-permutation MinHash, 32×4 LSH, windowed retrieval with exact verification and character-offset spans, PAN-PC-11 evaluation harness, conversion to `risk.plagiarism` ([PR #9](https://github.com/Newterios/Examory/pull/9), [ml/integrity](../../ml/integrity)).

### First measured result (synthetic data, 2 000 students, seed 42)

Isolation Forest on final attempts, held-out cohorts, threshold frozen on validation at FPR ≤ 5%:

| Model | Precision | Recall | F1 | FPR | PR-AUC |
|-------|----------:|-------:|---:|----:|-------:|
| Isolation Forest (with peer features) | 0.93 | 0.53 | 0.68 | 0.023 | 0.83 |
| Isolation Forest (no peer features) | 0.91 | 0.36 | 0.51 | 0.020 | 0.74 |
| Training-quantile rules | 0.97 | 0.64 | 0.77 | 0.012 | 0.90 |

Observations: peer features are what make collusion detectable (recall 0.81 vs 0.00 without them); score jumps are poorly detected by all models (recall ≤ 0.19); on this synthetic data simple quantile rules outperform the forest, which is expected for injected anomalies and will be re-examined on real data. These numbers describe injected synthetic anomalies, not real misconduct.

## Individual contributions

| Member | Area | Tasks | Pull requests |
|--------|------|-------|---------------|
| Kaiyrbekuly Aitbek | Team lead; architecture, event contract, infrastructure, CI, review of all PRs | LMS-1, LMS-2, LMS-3, LMS-4, LMS-11, LMS-21 | [#10](https://github.com/Newterios/Examory/pull/10), direct commits to `main` for the initial scaffold ([history](https://github.com/Newterios/Examory/commits/main?author=Newterios)) |
| Shadiyarbek Syrym | Requirements, UML, dashboard specification, analytics | LMS-5, LMS-6, LMS-7, LMS-20 | [#1](https://github.com/Newterios/Examory/pull/1), [#2](https://github.com/Newterios/Examory/pull/2), [#3](https://github.com/Newterios/Examory/pull/3), [#12](https://github.com/Newterios/Examory/pull/12) |
| Akhanov Yerdaulet | ML: integrity and anomaly detection, datasets, baselines | LMS-8, LMS-9, LMS-10, LMS-17, LMS-18, LMS-19 | [#4](https://github.com/Newterios/Examory/pull/4), [#5](https://github.com/Newterios/Examory/pull/5), [#6](https://github.com/Newterios/Examory/pull/6), [#7](https://github.com/Newterios/Examory/pull/7), [#8](https://github.com/Newterios/Examory/pull/8), [#9](https://github.com/Newterios/Examory/pull/9) |

## Known limitations and unfinished materials

- No platform service code yet: the core (Go services, Kafka producers, web client) starts in stage 2; EduLMS is used only as an architectural reference.
- ML baselines are evaluated on synthetic data only; the PAN-PC-11 run for the text baseline has not been executed yet (corpus download pending), so `ml/integrity/results.md` is still a template.
- AI-text detection and proctoring remain literature-level; no models.
- The technical specification, literature reviews and architecture are drafts awaiting supervisor feedback.
- Open architecture decisions (gRPC vs REST, schema registry, retention of exam recordings) are documented in architecture.md §6 and not yet decided.

## Next stage (December 2026 – January 2027)

- LMS-12…14 — Platform core: `auth` (JWT, refresh tokens in Redis, RBAC), `users`, `courses`, `assignments`, `quizzes` with Kafka producers per events.md v0.2 (Kaiyrbekuly).
- LMS-15, LMS-16 — Analytics module skeleton with Kafka consumers, metric aggregation, dashboard prototypes in Chart.js (Shadiyarbek).
- PAN-PC-11 evaluation of the text baseline, contamination and prevalence ablations of the anomaly baseline, synthetic-data stress tests (Akhanov).
- MVP demonstration to the supervisor; draft of thesis chapters 1–2.
