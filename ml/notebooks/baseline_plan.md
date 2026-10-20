<!-- ml/notebooks/baseline_plan.md -->
# Stage-two baseline experiments

Examory LMS-17/LMS-18; planned for December 2026–January 2027. These are preregistered experiment steps and provisional acceptance targets, **not measured results**. Dataset selection and leakage controls are specified in `datasets.md`.

Event fields are aligned to `docs/03_architecture/events.md`, draft v0.1 at main commit `e60b7ac`, checked on 20 October 2026.

## 1. Freeze data and reproducibility

Record source URLs, dataset release/checksum, terms, code revision, Python and library versions, hardware, seeds and grouped split manifests. Keep raw corpora, generated CSVs and weights outside Git. Use identical held-out cases for method comparisons. Thresholds are selected on validation only.

Generate behavior data with:

    python ml/datasets/generate_quiz_behavior.py --students 2000 --attempts 3 --questions 20 --anomaly-rate 0.12 --seed 42 --start-at 2026-12-01T09:00:00Z --output C:/Temp/examory/quiz_behavior.csv

Repeat seeds 43–46 and rates 0, 0.01 and 0.05. The default 300-student run is a smoke test, not a sufficiently precise fairness evaluation. Validate UUID v7 event IDs and their millisecond timestamp agreement with `occurred_at`, JSON payload fields, chronological order, attempt joins, hash comparability, revision counts, `0 <= score <= max_score`, positive `max_score` and submission counts before feature extraction.

## 2. LMS-17: MinHash baseline for text reuse

1. Build a PAN subset with source/suspicious documents and annotated spans. Keep related source/copy families together across splits. Add documented legitimate quotations and assignment boilerplate as hard negatives. If such additions lack independent annotation, report them as a separate constructed set.
2. Apply Unicode normalization and a fixed word tokenizer, preserve an offset map to original text, and normalize case for English. Exclude teacher-provided templates through an explicit allowlist. Do not silently remove all citations: attribution is part of human interpretation.
3. Extract sets of five-token shingles. Compare whole-document indexing with overlapping 200-token windows at stride 100, retaining a final short window. Windows help recover short borrowed passages inside a larger work. For fewer than five tokens, abstain from this detector and record coverage.
4. Use 128 seeded MinHash permutations and LSH with 32 bands of four rows. Candidate probability at Jaccard similarity `s` is `1-(1-s^4)^32`; this is probabilistic retrieval, not a hard similarity threshold. Compare 256 permutations/64×4 as a resource–recall ablation.
5. For retrieved pairs, calculate **exact** shingle Jaccard and reconstruct matching token runs from original offsets. Merge overlapping window hits. Define document-pair similarity as the maximum verified window Jaccard for this baseline, clearly distinguishing local overlap from whole-document similarity.
6. Search verification thresholds `{0.50,0.65,0.80}` and minimum contiguous evidence spans `{20,50}` tokens on validation. Freeze one configuration. Measure both LSH candidate recall and final precision/recall/F1, with all missed candidates counted as misses.
7. Compare with exhaustive exact Jaccard on a small subset to isolate retrieval loss, and TF-IDF/cosine as a lexical reference. Report literal copying, light lexical edits, heavy paraphrases and translated reuse separately. Do not expect MinHash to solve semantic paraphrase detection.
8. Inspect short common answers, quotations and same-topic independent work. A high similarity score prompts review; it does not prove plagiarism.

Input content is obtained from `submission.created` using the brief's `submission_id`, `assignment_id`, `kind`, `language`, `text_ref` and `files`. The reported result is `risk.plagiarism`; `evidence.matches` contains `submission_id`, `similarity` and `overlap_spans`. The exact inner representation of `overlap_spans` is unspecified in the brief: the experiment stores documented half-open original-text character offsets for both documents, pending schema alignment. Do not treat a MinHash estimate as a probability of misconduct.

Code similarity is a separate exploratory ablation: tokenize one selected language, suppress comments/formatting and provided starter code, then compare normalized token shingles or winnowing on BigCloneBench. Report clone types and parsing coverage; never pool code clone results with text plagiarism metrics.

## 3. LMS-18: Isolation Forest behavior baseline

1. Group events by `payload.attempt_id`, joining answers to submissions for `quiz_id` and `variant_id`; retain the distinct `context.session_id` only as exam/session context. Derive mean/median/p10 first-answer time from `time_since_shown_ms`, timing dispersion, fraction of unusually rapid first answers, final-question response time, `duration_ms`, revision count/rate, percentage score `100 * score / max_score`, and previous-score delta. Use `time_since_attempt_start_ms` to check answer ordering within the attempt. The synthetic `score` counts correct final answers and `max_score` is the question count. Reference timing quantiles must be estimated on training students conditional on question/quiz where data suffice.
2. For score changes, use percentage scores from only comparable earlier quiz/variant attempts; compute a standardized delta with a variance floor and a missing-history indicator. The synthetic generator fixes the quiz and variant across attempts. The first attempt must not receive a fabricated baseline of zero. In real data, account for test difficulty and practice effects.
3. Add a separately evaluated peer feature set: count submissions within a prespecified ±5-second window and measure same-item final-answer agreement. Weight rare shared wrong answers more strongly **only if correct-answer metadata are available**. The event stream alone lacks the answer key; otherwise call the feature rarity-weighted agreement, not wrong-answer agreement. Hash equality across different questions is meaningless.
4. The ±5-second comparison requires waiting five seconds after submission. Build peer features only within the same quiz/variant/cohort/window and split, with no labels. Include synchronous normal peers as negatives. Pair rules are explainable auxiliary signals; compare IF alone against IF plus peer features/rules.
5. Exclude `is_anomaly`, `anomaly_type`, raw IDs, absolute dates, producer and generator profile from model features. Keep IDs only for grouping and evidence. Fit missing-value imputation on training; apply `log1p` to skewed nonnegative timing/count features. Preserve explicit missing indicators.
6. Fit scikit-learn `IsolationForest(n_estimators=200,max_samples=min(256,n_train),contamination="auto",random_state=42)`. Primary fit uses normal training attempts. Run contaminated-training ablations separately. Reverse `score_samples` for an anomaly ranking; `contamination="auto"` does not estimate the true anomaly rate. Parameter and score semantics follow the [official API documentation](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html).
7. Choose a score threshold on validation for empirical FPR ≤5%, then maximize recall among eligible thresholds. Report a separate 1% FPR operating point when the number of validation negatives supports it. Do not force the threshold to label exactly 12% of attempts.
8. Freeze the threshold and evaluate held-out cohorts across seeds, prevalence and anomaly types. Report final-attempt-only and all-attempt results, normal fast/careful subgroup FPR, score-jump sensitivity and pair-level collusion performance. Test legitimate fast high scores and genuine improvement explicitly.
9. Compare simple training-quantile rules, IF without peer features and IF with peer features. Defer autoencoders until data size and a clear baseline weakness justify the additional tuning. A one-question timing rule should not be represented as an IF explanation.

For `risk.behavior_anomaly`, populate `evidence.features` with `mean_answer_time_ms`, a documented `z_score` (here, standardized percentage-score delta; null when unavailable), and `synchronous_with` containing peer `attempt_id` values as specified by the contract. Explain which observed features are unusual without claiming causal feature attribution. Map anomaly scores to 0–1 using the empirical training-normal score CDF; label this a relative risk ranking, not a calibrated misconduct probability.

Both risk types retain the contract's common fields: `risk_id`, `target_type`, `target_id`, `score`, `level`, `model{name,version}`, `explanation`, `evidence`. Use `target_type="submission"` with the submission ID for plagiarism and `target_type="attempt"` with `attempt_id` for behavior anomalies. The risk `score` remains in `[0,1]`, distinct from a quiz's raw-point `score`. Freeze any low/medium/high cutoffs from validation and retain the model version.

The generator uses zero-based `question_index`, cumulative time since question display and attempt start, and `answers_changed` as the number of revision events. These counting conventions are explicit because draft v0.1 does not resolve them fully; confirm them against production data before comparison.

## 4. Expected metrics and acceptance criteria

| Experiment | Provisional target | Interpretation |
|---|---|---|
| MinHash, literal/lightly edited reuse with at least 50 contiguous copied tokens | Pair precision ≥0.90, recall ≥0.85, F1 ≥0.87; candidate recall ≥0.95 | A goal for this declared subset. Heavy paraphrase and translation are separate limitations; report measured FPR and coverage. |
| IF, all-attempt synthetic set near 12% injected prevalence | Recall ≥0.70 at FPR ≤0.05; precision approximately ≥0.65 and F1 approximately ≥0.67 | Arithmetic expectation at that prevalence, not a literature benchmark or promised result. |
| IF, 1% prevalence at recall 0.70/FPR 0.05 | Expected precision only about 0.124 | `0.70×0.01/(0.70×0.01+0.05×0.99)` illustrates why a low anomaly base rate makes most alerts false positives. |
| Per-type and subgroup assessment | Publish each confusion matrix, interval and sample size | Do not hide poor collusion recall or elevated normal fast-student FPR behind aggregate accuracy. |
| Real teacher-reviewed pilot | No accuracy target before independent labels exist | Estimate alert precision; population FPR/recall additionally require sampled unflagged reviews. |

Use cohort-level bootstrap intervals and paired comparisons across the same seeds. Record CPU latency and peak memory without asserting a performance budget before measurements. If a target fails, document the failure and error cases; do not tune on the test set.

## 5. Deliverables and review-label loop

Save experiment configuration, split manifests, aggregate metrics, error examples with permitted content, and a short model card covering scope, abstentions, limitations and subgroup results. No baseline results are claimed by this planning document.

Join `risk.reviewed` to the original risk using payload `risk_id` and use its `decision` (`confirmed`/`dismissed`) as a risk-type-specific adjudicated label. Retain `reviewed_by`, `comment`, the event's `occurred_at`, uncertainty and original model version as review provenance. Follow the independent unflagged sampling procedure in `datasets.md`; a dismissal proportion is not the FPR. Recalibrate on a later training cohort and test on a future frozen cohort. AI-text and proctoring experiments remain separate from these two baselines.
