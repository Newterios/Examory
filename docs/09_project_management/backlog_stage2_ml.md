# Stage 2 backlog: ML modules and supporting services

Owner: Akhanov Yerdaulet. Prepared by the team lead on 22 November 2026 against `main` and the open pull request #31 (`LMS-38-m4-quiz-core`). Milestones and the M4 scenario are defined in [project_management.md](project_management.md) §3 and §3a; hypotheses H1–H3 in [verification_plan.md](../08_verification/verification_plan.md) §0.

The backlog is deliberately larger than what one person finishes before M4. Tasks are taken in the order of §2; whatever is not started stays here for stage 3. The cards carry temporary numbers `E-NN` until YouTrack IDs are assigned; a branch is pushed only under its `LMS-NN-slug` name.

## 1. Conventions

- One task, one branch, one pull request; commit messages start with the task ID; review by the team lead.
- **Base branch.** A card marked "base: PR #31" needs the platform code of pull request #31 and is branched from `LMS-38-m4-quiz-core`; its pull request is opened as a draft against that branch and retargeted to `main` once #31 is merged. All other cards branch from `main`.
- **Frozen paths.** `docs/11_paper/`, `ml/experiments/inflation/results/` and `ml/anomaly/` (pipeline v1, which the article rests on) are not modified. `services/platform/` and `services/auth/` are changed only by their owner.
- **Shared files.** In `docker-compose.yml`, `gateway/nginx.conf`, `Makefile` and `.github/workflows/ci.yml` a task only adds the block of its own new service or target, as a separate last commit.
- **Event contract.** Changes to `events.md` and the JSON Schemas are additive, marked "proposal, pending review" in the changelog, and reviewed by all three members; payload versions are not raised.
- **Data.** Corpora, model weights and video are never committed, only download scripts. A results file contains only numbers that were actually computed.
- **Evaluation.** Thresholds are chosen on validation and frozen before the test; synthetic results are labelled as synthetic; intervals are cohort-level bootstrap.
- **Size.** S: under a day. M: two to three days. L: about a week.

## 2. Order and work streams

Order by value: E-00 → E-01 → E-04 → E-08 → E-09 → E-05 → E-02 → E-11 → E-06 → E-03 → E-07 → E-10 → E-13, then any of E-12 and E-14…E-23.

The first seven are what M4 needs. E-08 and E-09 are on the critical path of the M4 scenario: if there is no branch with passing tests by 15 December 2026, they return to the team lead.

Tasks in different streams do not touch the same files; inside a stream they are done in sequence.

| Stream | Directory | Base | Tasks in order |
|--------|-----------|------|----------------|
| A. Text and code reuse, offline | `ml/integrity/` | `main` | E-01 → E-03 → E-18 |
| B. Anomaly scoring service | `ml/service/` | PR #31 | E-04 → E-05 → E-07 → E-14 → E-15 |
| C. New Go services | `services/<new>/` | PR #31 | E-08 → E-13 → E-21 → E-22 |
| D. `grading` and access scope | `services/grading/`, `services/quizzes/` | PR #31 | E-09 → E-10 (needs E-08) → E-12 (needs E-11) |
| E. Integrity service and replay | `ml/integrity_service/`, `sim/cmd/` | PR #31 | E-11 → E-06 |
| F. Research and thesis text | `docs/12_thesis/`, new files under `ml/` | `main` | E-02 → E-16 → E-20 → E-17 → E-23 → E-19 |

## 3. Task cards

### E-00. Review of pull request #31 and of the `risk.*` part of the contract

Size S. Due 27 November 2026 (before M3).

- **Why.** Every pull request is reviewed by a second member before merging, and `events.md` v0.3-draft has to be seen by all three before M3. Tasks E-04…E-15 build on this pull request.
- **Read.** On the branch: `ml/service/README.md`, `scoring.py`, `watermark.py`, `processor.py`; `docs/03_architecture/schemas/risk.behavior_anomaly.v1.schema.json`; `events.md` §4 and the v0.3-draft changelog entry.
- **Do.** Run `make m4 && make e2e` locally. Review: (1) whether the features and thresholds of the service match `ml/anomaly/`; (2) what the ±5 s watermark rule does with a late peer; (3) whether `evidence` is enough for a teacher (FR-42); (4) remarks on the `risk.behavior_anomaly.v1` schema; (5) record that the service publishes the Isolation Forest although H1 adopted the rules (fixed in E-04).
- **Done when.** The pull request has the review with concrete remarks or an approval, and the output of `make e2e` is attached.

### E-01. MinHash baseline on PAN-PC-11, test of H3

Size S (little work, long machine time).

- **Why.** M4 requires baselines evaluated on open data. Every cell of `ml/integrity/results.md` is still `pending`.
- **Read.** `ml/integrity/README.md`, `ml/integrity/evaluate.py`, `ml/datasets/download_pan.py`; verification plan §0 (H3) and §2 (O4).
- **Do.** Download and extract the corpus (1.7 GB, needs 7-Zip). Main run with `--max-documents 120 --seed 42`; then enlarge the subset until the test partition has at least five positive families and repeat on seeds 42–46. Ablations from the README: `--mode whole`, `--num-perm 256`. Write `ml/integrity/results.md` (generated by the script) and add an "Interpretation" section: whether the H3 criterion holds (pair precision ≥ 0.90 and recall ≥ 0.85 on reuse of at least 50 tokens, candidate recall ≥ 0.95), the breakdown by obfuscation type, where the method fails. Add "Status of H3" to the verification plan §0 and correct §6. Update the outdated `ml/README.md`.
- **Done when.** No `pending` cell remains; commit, seed, subset size and the number of excluded large families are stated; the H3 verdict is stated as measured, including a failure. A defect found on the real corpus is fixed together with a test in `ml/tests/test_integrity.py`.
- **Check.** `python -m unittest discover -s ml/tests -v`; `python -m ruff check ml`.

### E-02. Draft of the ML sections of thesis chapters 1 and 2

Size M.

- **Why.** M4 requires draft chapters 1–2; the material exists in the repository and has to become thesis text.
- **Read.** `docs/05_literature/`, `ml/notebooks/datasets.md`, `ml/notebooks/baseline_plan.md`, `ml/anomaly/results.md`, `ml/integrity/results.md` (after E-01), `docs/06_reports/article-experiment-2026-11.md`, verification plan §0 and §3b.
- **Do.** `docs/12_thesis/chapter1_ml_review.md`: methods for text and code reuse, AI-text indicators, behavioural anomalies and proctoring, ending with what the project adopts and why. `docs/12_thesis/chapter2_ml_methods.md`: research question, H1–H3, data (synthetic generator, behaviour simulator, EdNet, PAN), evaluation protocol, baselines. Only sources from `sources.bib`; every number refers to its results file.
- **Not in scope.** The article in `docs/11_paper/` is not copied paragraph by paragraph: the thesis goes through a similarity check and the article is published separately. Restate and cite.
- **Done when.** Every citation resolves in `sources.bib` and no number lacks a source.

### E-03. Code similarity baseline (winnowing)

Size M.

- **Why.** Objective O4 "Integrity: code" of the verification plan has not been started; FR-21 and FR-22 need it for code in stage 3.
- **Read.** `ml/integrity/minhash.py`, `evaluate.py`, `to_risk_event.py`; the code section of `docs/05_literature/plagiarism_detection.md`; `ml/notebooks/datasets.md`; `risk.plagiarism` in `events.md`.
- **Do.** `ml/integrity/code/`: a lexer that normalizes identifiers and literals (Java for BigCloneBench, behind an interface so that Python can be added), k-grams with winnowing, fingerprint comparison, alignment of matched fragments into `overlap_spans` in code points of the original text. Comparison baseline: exact token-sequence match. Evaluation per clone type (T1, T2, T3) on a BigCloneBench subset; if the download needs a sign-in that cannot be obtained, synthetic pairs from an open corpus (renaming, method reordering, dead code), stated plainly in the results.
- **Done when.** Precision, recall and FPR per clone type at a validation-chosen threshold; a `risk.plagiarism` event for a pair of code files has the shape of `events.md`; tests in `ml/tests/`.
- **Check.** `python -m unittest discover -s ml/tests -v`; `python -m ruff check ml`.

### E-04. The anomaly service publishes the adopted detector (rules)

Size M. Base: PR #31.

- **Why.** H1 adopted the quantile rules, while `ml/service/scoring.py` is fixed to `MODEL_KEY = "iforest"`. The M4 demonstration has to show the detector the project reports as adopted.
- **Read.** `ml/service/scoring.py`, `artifact.py`, `README.md`; `ml/anomaly/train.py` (`score_model`, `quantile_rules`), `ml/anomaly/to_risk_event.py`; verification plan §0 (status of H1 and the caveat of 15 November).
- **Do.** The published model is set by configuration (`ANOMALY_PUBLISH_MODEL`, default `quantile_rules`); the other model runs in shadow mode, its score is stored in the service database and not sent to Kafka. `model.name` and `explanation` describe the detector that fired; for the rules the explanation lists the features outside their bounds together with the bounds. `risk_id` stays deterministic: check that a change of model does not create a second signal for an attempt already flagged, and document the behaviour. The README explains the choice with a reference to the H1 tables.
- **Done when.** Unit tests cover both modes. `TestM4Scenario` passes with the rules (two crafted attempts flagged, none of eight ordinary ones); if the rules behave differently, that is a finding to report with numbers, and the test is not adjusted to the result.
- **Check.** `make py-test`; `make m4 && make e2e`; `python -m ruff check ml`.

### E-05. Metrics and processing states of the anomaly service (FR-28)

Size M. Base: PR #31.

- **Why.** Pending, scored without a signal, not scorable and failed are visible only in the service database. FR-28 requires these states to be distinguishable, and the VPS measurement (LMS-34) needs latency figures.
- **Read.** Known limitations in `ml/service/README.md`, `store.py`, `processor.py`; the FR-28 row of `services/grading/README.md`; decision D5 in `technology_comparison.md` (trigger figures: p95 scoring latency above 2 s, CPU above 50 % of one vCPU at 20 events/s).
- **Do.** (1) Prometheus `/metrics` on the health port: events accepted and rejected, attempts pending, scored, flagged, not scorable by reason, a histogram of scoring time, a histogram from `quiz.submitted` to the published signal, watermark lag. (2) An internal read-only `GET /attempts/{attempt_id}/status`, not published to the host. (3) In the pull request, a short proposal on how `grading` should obtain the state: a call to this API or a new event. Changing `grading` or the contract is not part of this task.
- **Done when.** Metrics are covered by tests, the README has a metrics table, and neither logs nor metrics contain student identifiers or answers.
- **Check.** `make py-test`; `curl` to `/metrics` inside the container after `make m4`.

### E-06. Replay of simulator worlds through the platform: feature parity and load

Size L. Base: PR #31.

- **Why.** The verification plan §3b states that features are computed by the same pipeline as for real data. This needs a test, and the same tool yields the figures for LMS-34.
- **Read.** `sim/README.md`, `sim/internal/events/`, `docker-compose.research.yml`, the simulation clock section of `services/quizzes/README.md`, `services/grading/integration/m4_e2e_test.go` as a pattern, `ml/experiments/inflation/pipeline.py`.
- **Do.** `sim/cmd/examory-replay` (Go): plays a simulator world through the `quizzes` API in research mode (`X-Sim-Time`) at a given event rate. **Parity** mode compares the service score of every attempt with the offline score of the same world under the same artifact and fails above a tolerance. **Load** mode runs 20 events/s for 15 minutes and reports p50, p95 and p99 of the time from submission to signal, with CPU and memory of the service container.
- **Done when.** The parity test passes on a small world, or the discrepancy is found and described; `ml/service/benchmarks.md` holds the load figures and the machine they were measured on; `replay-parity` and `replay-load` targets are added to the `Makefile`.

### E-07. Evidence for the teacher: per-question timing (FR-42)

Size S–M. Base: PR #31.

- **Why.** FR-42 is partly covered: `evidence` has no per-question timing, so a teacher cannot see which answers were unusually fast.
- **Read.** `risk.behavior_anomaly` in `events.md` (feature keys may be added without a version bump), its JSON Schema, `ml/anomaly/to_risk_event.py`, `ml/service/scoring.py`.
- **Do.** An additive key in `evidence.features` (for example `per_question`: question index, answer time, reference median, faster-than-threshold flag, changed-answer flag), built in `ml/service` as a wrapper around the frozen output. Schema and `events.md` updated additively as a proposal; run `make sync-schemas`, because `grading` embeds a copy of the schema and a test fails on drift.
- **Done when.** A new event validates against the updated schema, an old event without the key still validates, tests added.

### E-08. Minimal `courses` service (Go)

Size L. Base: PR #31. Critical path of M4.

- **Why.** The M4 scenario needs FR-4 and FR-5 (one course with modules and materials), and without `courses` a teacher is not bound to a course: at present any teacher sees every signal.
- **Read.** `services/README.md`, `services/platform/README.md`, the whole of `services/quizzes/` as the pattern (hexagonal layout, outbox, migrations, integration tests, Dockerfile); FR-3…FR-6 of the technical specification; topic `lms.courses` in `events.md`; `docs/04_uml/data_model.puml`.
- **Do.** `services/courses/` with its own `go.mod`: course, ordered modules, materials (text and link; files in stage 3), states draft / published / archived, assignment of teachers to a course, enrolment of groups. A student sees only published content of the courses of their group (group from the token). Events `course.created`, `course.updated`, `material.viewed` (FR-6: a repeated view does not multiply completion) through the outbox, with JSON Schemas. An internal endpoint answers "teacher X is assigned to course Y" and "group G is enrolled in course Y" for `quizzes` and `grading`.
- **Done when.** Unit tests and integration tests against real PostgreSQL and Kafka; negative access tests (another course, unpublished material, a student creating a course); `go vet`, `gofmt` and `govulncheck` clean; a README in the format of the other services with an FR coverage table and known limitations.
- **Check.** `make check`; `make infra && make integration`.

### E-09. Grade entry in `grading` (FR-17)

Size M. Base: PR #31. Critical path of M4.

- **Why.** FR-17 is in the M4 subset, and `grading` has no grade path at all (signals and review only).
- **Read.** The whole of `services/grading/`; FR-15…FR-17, FR-26 and NFR-16 of the technical specification; `grade.assigned` in `events.md`.
- **Do.** A teacher records and revises the grade of a quiz attempt; bounds are validated; the revision history is append-only; `grade.assigned` is published through the outbox, with a schema. A student sees their own released grade (FR-15, minimal). **The FR-26 invariant is kept:** confirming a signal never changes a grade; the test that checks this stays and is extended.
- **Done when.** Tests for bounds, history, idempotent retry, and refusal for the student and admin roles; grade changes are audited (NFR-16).
- **Check.** `make check`; `make integration`.

### E-10. Course-scoped teacher access in `quizzes` and `grading` (NFR-5)

Size M. Base: the E-08 branch.

- **Read.** `services/courses/` (the check endpoints of E-08); the role checks in the HTTP adapters of `quizzes` and `grading`.
- **Do.** `quizzes` and `grading` ask `courses` whether the teacher is assigned to the course and the group is enrolled; when `courses` is unavailable the request is refused, not let through.
- **Done when.** All earlier tests of both services pass; negative tests show that a teacher of another course neither sees nor reviews a signal; the changes in both services are minimal.

### E-11. Text reuse service `ml/integrity_service/`

Size L. Base: PR #31.

- **Why.** The main integration of stage 3: `submission.created` → `risk.plagiarism`. It can be built before the `assignments` service exists, on event fixtures and the MinIO of the compose file.
- **Read.** The whole of `ml/service/` as the pattern (ingestion, idempotency, outbox, schema validation, pinned versions); `ml/integrity/minhash.py`, `to_risk_event.py`; `submission.created` and `risk.plagiarism` in `events.md`.
- **Do.** A consumer of `lms.submissions`; the text is read from MinIO by `text_ref` (the event carries no text); the MinHash/LSH index is kept in PostgreSQL in the service's own database; comparison within an assignment, and within a course by a parameter; the assignment template is excluded; short texts abstain; a student's own earlier attempt is not a match; the signal is raised for the later submission of a pair; `overlap_spans` in code points of the `text_ref` string; deterministic `risk_id`; publication to `ai.risk` through an outbox. New schemas `submission.created.v1` and `risk.plagiarism.v1` as a proposal.
- **Done when.** Fixture tests: a copy, a lightly edited copy, an independent text, a short text, a redelivered event, MinIO unavailable (the submission stays pending and is not lost). The image builds; the README lists limitations and a threat model in the format of `ml/service/README.md`.

### E-12. `grading` accepts `risk.plagiarism` (FR-22)

Size M. Base: PR #31. Needs E-11.

- **Read.** The whole of `services/grading/`; the `risk.plagiarism.v1` schema of E-11.
- **Do.** Generalize signal intake to the category `plagiarism` with target `submission`: storage, list, a detail view with matches and fragments, review through the same `risk.reviewed` flow; embedded schema; a filter by category.
- **Done when.** The existing behaviour-signal tests pass unchanged; new tests cover intake, deduplication, review and refusal for a student.

### E-13. `event-collector` service (Go) and client events

Size M. Base: PR #31.

- **Why.** `client.focus_lost`, `client.tab_switched` and `client.paste` are in the contract but nothing publishes them; they are inputs for anomaly detection and proctoring.
- **Do.** `services/event-collector/` after the pattern of `quizzes`: a thin HTTP endpoint with JWT; a check that the attempt belongs to the caller and is still open (through `quizzes` or a token claim, with the choice justified); rate and size limits; publication to `lms.exams` with schema validation; schemas for the three events. Clipboard content is never accepted, only `chars`.
- **Done when.** Tests include a forged `attempt_id`, another student's attempt and an exceeded limit.

### E-14. Feature pipeline v2 for live attempts

Size L. Base: PR #31.

- **Why.** Pipeline v1 scores only fully answered quizzes with one-point questions; every other attempt becomes `not_scorable`, and a real course will have many.
- **Do.** A new module (for example `ml/anomaly_v2/`), not an edit of `ml/anomaly/`: partly answered attempts, weighted questions, timeout submission (`quiz.submitted.reason`), optional features from `client.*` (best-effort; never the only basis of a signal). Artifact version 2; the service loads both. A comparison of v1 and v2 on one simulator world: on attempts v1 can score, the results agree or the difference is explained.

### E-15. Per-quiz reference from live data and cold start

Size L. Base: PR #31.

- **Why.** The demo model is trained on synthetic data, and live quiz identifiers have no per-item timing reference. In the article experiment, detectors fitted on synthetic ordinary behaviour flag 11–39 % of real EdNet attempts at a nominal 5 %. Without a reference built from the course's own data the false-positive rate will be unacceptable.
- **Do.** A cold-start policy (below N attempts per quiz either no signal or only signals that need no reference; proposed and justified); periodic rebuilding of the reference from accumulated attempts; exclusion of attempts with a confirmed signal (`risk.reviewed`, topic `lms.grades`), which is the contamination caveat of H1; artifact versioning and rollback. Checked on the simulator: the share of ordinary attempts flagged before and after.

### E-16. False-positive rate by student group (risk R3)

Size S–M.

- **Do.** A new script and table next to `ml/experiments/inflation/` (existing results untouched): FPR of the rules and of the forest by simulator stratum, namely fast strong students, slow students, students who change answers often. The conclusion states whom the detector flags more often and by how much. The table goes into chapter 2.

### E-17. Score-jump detector

Size M.

- **Why.** Score jumps are almost undetected by every model (recall at most 0.19 on the stage-1 synthetic data).
- **Do.** A separate rule on the student's history (standardized difference against earlier comparable attempts, allowing for the expected learning gain present in the simulator model). Evaluation on the simulator with the pre-knowledge mechanism, compared with the current state. A negative result is recorded as such.

### E-18. Offline experiment with AI-text indicators (FR-23, should)

Size M.

- **Do.** HC3, English part: a perplexity threshold as the baseline against stylometric features with a simple classifier. The headline figure is the FPR on human texts, separately for non-native writers (an open corpus is found and justified; if none is suitable, that is stated). Results in `ml/integrity/ai_text/results.md`. Nothing is deployed or connected to Kafka: the contract allows `risk.ai_generated` only after an offline evaluation with FPR by language.

### E-19. Proctoring baseline: face presence

Size M.

- **Do.** `ml/proctoring/`: from a video file, segments "no face" and "several faces" (MediaPipe or OpenCV), short gaps merged, output `risk.proctoring` with `segments[{from_ms, to_ms, kind}]`; an event fixture for the acceptance of FR-52. Evaluated on clips recorded by team members with consent; video is not committed and the results contain aggregates only. Gaze and deepfake detection are out of scope.

### E-20. Preparation for real logs: Moodle converter and pseudonymization (LMS-36)

Size M.

- **Why.** Real anonymized logs are the final arbiter of H1; once the School provides them there will be no time left for a converter.
- **Do.** After the pattern of `ml/ednet/convert.py`: a converter from a Moodle quiz export (attempt and response reports, event log) to the `quiz.*` contract; a pseudonymization script (keyed HMAC of identifiers with the key outside the repository, time shift, removal of names and addresses); a document listing the fields requested from the School and the fields that are not needed. Tests on a synthetic sample export. Real data never enters the repository.

### E-21. `assignments` and `files` services (Go)

Size L + L. Base: PR #31.

- **Why.** Stage 3: FR-7…FR-10 and the source of `submission.created` for E-11.
- **Do.** Two pull requests. `files`: upload through MinIO with presigned URLs, type and size allowlists (NFR-6), links valid for at most 10 minutes, `file.uploaded`. `assignments`: an assignment with a deadline and a number of attempts, submission by file references, a receipt, late status by server time, text extraction to MinIO with `text_ref`, `submission.created` without waiting for ML. After the pattern of `quizzes`.

### E-22. `notifications` service (Go, FR-18)

Size M. Base: PR #31.

- **Do.** A consumer of `lms.grades` and `lms.courses`; in-app notifications about a deadline, a released grade and a review outcome, without exposing evidence; idempotent by `event_id`. Storage in PostgreSQL, recorded in the README as a deviation from `architecture.md`, which names MongoDB (the fallback of decision D3 folds MongoDB into PostgreSQL).

### E-23. Model cards

Size S.

- **Do.** `ml/MODEL_CARDS.md`: for every detector (rules, forest, MinHash, winnowing) its purpose, training and evaluation data, metrics with intervals, known limitations, groups with a higher FPR, and what the model does not claim.
