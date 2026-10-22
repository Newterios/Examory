# Verification and evaluation plan

Purpose: define how the achievement of the project aim will be demonstrated: which tests are run at which level, which outcome metrics are measured for each module, under which conditions, and against which baseline. Acceptance criteria AC-1…AC-12 and requirement IDs refer to `docs/02_requirements/technical_specification.md`; ML protocol details are in `ml/notebooks/datasets.md` and `ml/notebooks/baseline_plan.md`.

## 1. Test levels

| Level | Scope | Tooling | Runs | Gate |
|-------|-------|---------|------|------|
| Unit | Go packages per service; Python modules in `ml/` | `go test`, `unittest`/`pytest` | Every PR (CI) | PR cannot merge on failure |
| Contract | Every produced event validated against the JSON Schema of `events.md` (schemas added in stage 2) | schema validation in producer tests; consumer fixtures | Every PR | Mandatory for any change to `events.md` |
| Integration | Service + its database + Kafka in Docker Compose; producer → topic → consumer | `docker compose` test profile, Go integration tests, `testcontainers` | Nightly and before each tag | Required for AC-2, AC-3, AC-5 |
| End-to-end scenario | Acceptance walkthroughs AC-1…AC-11 on a staging deployment with synthetic users | scripted API scenarios + manual UI checklist | Before each pre-defense | All `must` requirements pass |
| ML offline evaluation | Integrity and anomaly models on synthetic and open datasets | `ml/anomaly/evaluate.py`, `ml/integrity/evaluate.py` | On every model change; results committed to `results.md` | Thresholds frozen on validation before test |
| Non-functional | Load, security, privacy, usability checks of NFR-1…NFR-16 | k6 or Locust (load), OWASP ZAP baseline scan, manual accessibility checklist | Stage 4 | AC-12 |
| Pilot | One consented course scenario with real teachers reviewing signals | review decisions logged as `risk.reviewed` | Stage 4 | Alert precision reported; recall estimated by sampled audit |

## 2. Outcome metrics per objective

| Objective (proposal §4) | Metric | Measurement condition | Baseline for comparison | Target (provisional) |
|-------------------------|--------|-----------------------|-------------------------|----------------------|
| O3 LMS core | All `must` FR-1…FR-20 pass their acceptance tests; p95 API latency | Staging VPS, 100 concurrent users, 20 events/s, ≥ 10 000 historical events (TS §5) | — | 100 % of must-FR; p95 ≤ 500 ms (NFR-1) |
| O4 Integrity: text reuse | Pair precision, recall, F1; LSH candidate recall; span-level overlap | PAN-PC-11 subset, family-level 60/20/20 split, thresholds frozen on validation | Exhaustive exact Jaccard; TF-IDF + cosine | Literal/light-edit reuse ≥ 50 tokens: P ≥ 0.90, R ≥ 0.85; candidate recall ≥ 0.95 |
| O4 Integrity: code | Clone-type precision/recall on a BigCloneBench subset | Token-normalized winnowing, one language | JPlag-style token match | Reported per clone type; no target before first run |
| O4 AI-text indicators | FPR by language group; AUROC | HC3 English subset + locally collected consented essays | Perplexity threshold | Not deployed unless FPR for non-native writers is reported and acceptable to the supervisor |
| O5 Anomaly detection | Precision, recall, F1, FPR at validation-chosen threshold; PR-AUC; per-type recall; collusion pair precision | Synthetic generator, 2 000 students, cohort-level split, seeds 42–46, prevalence 1 %, 5 %, 12 % | Training-quantile rules; Isolation Forest without peer features | R ≥ 0.70 at FPR ≤ 0.05 on 12 % prevalence (stage-1 result: R 0.53, P 0.93, FPR 0.023 on final attempts) |
| O5 Variant-leak detection | Detection of injected pass-rate jumps; false alerts on stable variants | Synthetic cohorts with ≥ 10 students per window | Fixed-threshold rule (20 pp rise) | No synthetic jump ≥ 20 pp missed; 0 alerts on stable variants |
| O6 Analytics | Agreement of weak-topic and at-risk labels with teacher judgement; dashboard latency | Pilot course, teacher rates 20 sampled recommendations | No-recommendation baseline (completion only) | ≥ 70 % of recommendations rated "useful or neutral"; event-to-dashboard ≤ 60 s (NFR-2) |
| O7 Exam sessions | Consent before capture; capture stops ≤ 5 s after withdrawal; agent signal delivery | Staging exam with test accounts | — | 100 % of AC-10, AC-11 steps; NFR-8 |
| O8 Overall | Alert precision in pilot; audit-sampled recall; teacher time per review | Pilot course with ≥ 2 teachers | Manual review without signals | Reported with confidence intervals; no accuracy claim beyond the pilot |

All ML numbers are reported with cohort-level bootstrap 95 % intervals and a frozen threshold; synthetic results are labelled as such and never presented as real-misconduct detection rates.

## 3. Measurement conditions

- **Environment:** staging deployment from `docker-compose.yml` on one VPS (4 vCPU, 8 GB RAM target); versions pinned in `ml/requirements.txt` and service `go.mod` files; seeds and split manifests committed with results.
- **Data:** synthetic events from `ml/datasets/generate_quiz_behavior.py`; open corpora downloaded by scripts under `ml/datasets/` (never committed); pilot data only from consented participants, pseudonymized, retained per TS §6.2.
- **Procedure:** thresholds and hyper-parameters chosen on validation only; test evaluated once per model version; every result links to the commit and the YouTrack task.

## 4. Baseline solutions

| Module | Baseline | Why this baseline |
|--------|----------|-------------------|
| Text reuse | TF-IDF + cosine; exhaustive Jaccard | Cheapest lexical method; isolates retrieval loss of LSH |
| Quiz anomalies | Two-sided training-quantile rules per feature | Simplest explainable detector; stage-1 result shows it is competitive on synthetic data, so the forest must beat it on real data to be justified |
| Analytics | Completion-only dashboard without recommendations | Represents what Moodle/Canvas offer today |
| Proctoring | Presence-only check (face detected / not detected) | Minimum signal before any gaze or deepfake indicator |

## 5. Verification of mandatory requirements

Every `must` requirement maps to one acceptance test (TS §7, AC-1…AC-12). The acceptance protocol is: two independent courses and groups, student/teacher/admin accounts, text and code submissions, quiz history sufficient for the 7-day/30-day windows, one rubric and one exam session; fixture-based ML outputs when a module is unavailable, reported as fixtures. A requirement is "verified" only when its test has been executed on a tagged version and the result is recorded in `docs/06_reports/`.

## 6. Stage-1 status of this plan

- Executed: unit tests for `ml/anomaly` and `ml/integrity`; offline evaluation of the anomaly baseline on synthetic data (`ml/anomaly/results.md`); CI builds docs and compose validity.
- Not executed yet: PAN-PC-11 run; any integration or end-to-end test (no platform code); load, security, usability checks; pilot.
