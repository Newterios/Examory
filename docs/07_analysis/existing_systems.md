# Existing system comparison

Purpose: show what current LMS platforms and integrity/proctoring tools already offer against a single set of criteria, and where Examory adds value. Claims are taken from the vendors' own documentation (links in the last row); where documentation is silent, the cell says "not documented". Checked on 22 October 2026 (Moodle and Turnitin pages verified in a browser; they reject automated requests).

## Systems compared

| # | System | Type | Licence / deployment |
|---|--------|------|----------------------|
| S1 | **Moodle** | Open-source LMS | GPL, self-hosted or MoodleCloud |
| S2 | **Canvas LMS** (Instructure) | LMS | Open-source core (AGPL) + commercial hosting |
| S3 | **Turnitin** (Feedback Studio / Originality) | Similarity and AI-writing detection service | Commercial SaaS, integrated into LMS via LTI |
| S4 | **Proctorio** | Online proctoring | Commercial SaaS, browser extension, LTI integration |
| S5 | **MOSS** (Stanford) / **JPlag** | Source-code similarity tools | Free academic service (MOSS) / open source (JPlag, GPL) |

Why these five: S1 and S2 are the two most widely deployed LMS platforms in universities and represent the "core LMS" baseline; S3 and S4 are the de-facto commercial answers to text integrity and proctoring that universities bolt onto S1/S2; S5 represents the code-similarity tools used in programming courses. Together they cover every function Examory proposes, which lets the comparison show what is missing when they are used side by side.

## Criteria

| # | Criterion | Why it matters for Examory |
|---|-----------|----------------------------|
| C1 | Course, assignment and quiz management | Baseline LMS functionality that Examory must also provide |
| C2 | Text similarity detection with evidence (matched sources and spans) | FR-21…FR-24 |
| C3 | Source-code similarity detection | Programming courses are the pilot context |
| C4 | AI-generated-text indicators, with stated limitations | FR-23; known false-positive problem for non-native writers |
| C5 | Learning analytics: weak topics, recommendations, at-risk students | FR-29…FR-37 |
| C6 | Behavioural anomaly detection in quizzes (timing, synchrony, score jumps, variant leaks) | FR-41…FR-45 |
| C7 | Online proctoring: webcam, environment and client-side signals, with consent | FR-47…FR-55 |
| C8 | Explainability and human-in-the-loop review (teacher confirms/dismisses with evidence) | FR-25, FR-26, FR-28 |
| C9 | Open event/data model for combining signals across modules | events.md, NFR-12 |
| C10 | Self-hosting and data residency control | Constraint 6.1 (Kazakhstan personal-data law) |

## Comparison

| Criterion | S1 Moodle | S2 Canvas | S3 Turnitin | S4 Proctorio | S5 MOSS / JPlag | **Examory (proposed)** |
|-----------|-----------|-----------|-------------|--------------|-----------------|------------------------|
| C1 Course / assignment / quiz | Yes, full | Yes, full | No (plugs into an LMS) | No (plugs into an LMS) | No | Yes (core services) |
| C2 Text similarity with evidence | Only via plugins (Turnitin, Compilatio, Ouriginal) | Only via LTI plugins | Yes: similarity report with matched sources and highlighted text | No | No | Yes: MinHash/LSH with exact span evidence (`risk.plagiarism.evidence.matches`) |
| C3 Code similarity | Via external tools | Via external tools | Limited (code treated as text) | No | Yes: token/winnowing-based, per language | Yes: token normalization + winnowing (stage 2) |
| C4 AI-text indicators | No | No | Yes ("AI writing" indicator); vendor states it should not be the sole basis for action | No | No | Signal only, offline-evaluated, with per-language false-positive reporting |
| C5 Learning analytics | Partial: "Students at risk of dropping out" model, course reports | Partial: New Analytics (grades, participation) | No | No | No | Yes: weak topics, recommendations, group risk, grade suggestion |
| C6 Quiz behaviour anomalies | No (quiz logs exist, no analysis) | No | No | Partial: during proctored sessions only | No | Yes: event-level features, Isolation Forest, variant-leak detection |
| C7 Proctoring with consent | Only via Safe Exam Browser or third parties | Only via third parties | No | Yes: webcam, screen, environment; automated "suspicion" levels | No | Yes: webcam presence + Windows agent, consent recorded in events, no automatic verdict |
| C8 Explainable, human-in-the-loop | Depends on plugin | Depends on plugin | Report is shown to the instructor; AI score is a percentage without evidence | Suspicion score with timeline; instructor reviews | Match percentages with aligned code | Every signal carries `explanation` and `evidence`; teacher decision event `risk.reviewed` is logged |
| C9 Open event model across modules | Logstore API, no streaming | Canvas Data / Live Events (paid tiers) | Closed | Closed | N/A | Kafka topics and a published contract (`events.md`) |
| C10 Self-hosting / data residency | Yes | Yes (open-source core), hosted by default | No (vendor cloud) | No (vendor cloud) | MOSS: no (Stanford server); JPlag: yes | Yes: Docker Compose on institutional servers |

## Findings

1. **No single system covers the three modules.** A university that wants integrity, analytics and proctoring today combines an LMS (S1/S2) with two or three vendor services (S3, S4, S5). Each has its own data model and opaque score; signals cannot be correlated, and the student's earlier work is not available to the integrity check.
2. **Behavioural anomaly detection outside proctored sessions is absent.** S1 and S2 store quiz attempt logs but do not analyse timing or synchrony; S4 analyses behaviour only inside its own proctored session.
3. **Explainability is uneven.** S3 shows matched text for similarity but only a percentage for AI writing; S4 produces suspicion levels whose logic is not disclosed. Examory's requirement that every risk event carries evidence and an explanation, and that only a teacher decision changes anything, is the main design difference.
4. **Data residency.** S3 and S4 process student text and video in the vendor's cloud; this conflicts with the local-storage expectation under Kazakhstan's personal-data law noted in the technical specification. Examory is self-hosted by design.
5. **Where Examory does not compete.** S1/S2 offer far broader LMS functionality (forums, SCORM, competencies, mobile apps) and mature ecosystems; S3 has a similarity corpus of billions of documents that Examory cannot match. Examory's integrity module therefore targets intra-course and intra-institution reuse plus code, not web-scale source retrieval.

## Sources

- Moodle: Plagiarism plugins, https://docs.moodle.org/en/Plagiarism_prevention ; Analytics and "Students at risk" model, https://docs.moodle.org/en/Analytics ; Safe Exam Browser, https://docs.moodle.org/en/Safe_Exam_Browser ; Logs and log stores, https://docs.moodle.org/en/Logs
- Canvas LMS: open-source repository, https://github.com/instructure/canvas-lms ; New Analytics, https://community.canvaslms.com/t5/Canvas-Basics-Guide/What-is-New-Analytics/ta-p/73 ; Live Events / Canvas Data, https://canvas.instructure.com/doc/api/file.data_service_introduction.html
- Turnitin: Similarity Report, https://guides.turnitin.com/hc/en-us/articles/23435833938701-Understanding-the-similarity-score ; AI writing detection and its limitations, https://www.turnitin.com/solutions/topics/ai-writing/
- Proctorio: product description and settings, https://proctorio.com/products/online-proctoring ; company site, https://proctorio.com/
- MOSS: https://theory.stanford.edu/~aiken/moss/ (service page, intermittently unreachable on 22 Oct 2026; the method is described in Schleimer et al. 2003, source 5 of the plagiarism review) ; JPlag: https://github.com/jplag/JPlag

Liang et al. (2023) on detector bias against non-native English writers is cited in `docs/05_literature/plagiarism_detection.md` (source 14) and supports the limitation noted for C4.
