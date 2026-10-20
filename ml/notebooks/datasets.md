<!-- ml/notebooks/datasets.md -->
# Dataset selection and evaluation protocol

Examory, LMS-10/LMS-19. Sources checked on 20 October 2026. Dataset sizes below describe published releases, not files already downloaded. Keep raw data, generated CSVs and model weights outside version control.

Event fields are aligned to `docs/03_architecture/events.md`, draft v0.1 at main commit `e60b7ac`, checked on 20 October 2026.

## Candidate datasets

| Name | Task | Size | License / access | Link | How we use it |
|---|---|---|---|---|---|
| PAN-PC-11 | Source retrieval and text reuse localization | Approximately 27,000 documents; download about 1.7 GB | Official record permits free research use; a standard license identifier is not visible in that record. Retain the downloaded terms and do not assume unrestricted redistribution. | [Official dataset](https://zenodo.org/records/3250095); [organizer catalogue and size](https://pan.webis.de/data.html) | Evaluate MinHash retrieval and recovered spans against inserted plagiarism annotations; separate literal, obfuscated and translated cases. Gutenberg-derived prose is not representative of student coursework. |
| BigCloneBench with IJaDataset | Java clone detection | The 2016 release reports over 8 million validated clone pairs in a repository of 2.3 million Java files; later versions differ | Benchmark database: CC BY-NC 4.0. Source files retain individual project licenses. Downloads may require sign-in. | [Maintainers and licenses](https://github.com/clonebench/BigCloneBench); [release paper](https://clones.usask.ca/pubfiles/articles/SvajlenkoBigCloneEvalICSME2016.pdf) | Evaluate normalized tokens/winnowing by clone type on a manageable subset. Record exact release and sampling manifest. A functional clone is not evidence of unauthorized copying; unlabelled pairs are not automatically negatives. |
| HC3, English and Chinese | Human versus ChatGPT text classification | About 24.3k English and 12.9k Chinese **question records**, each containing lists of human and generated answers | Dataset cards specify CC BY-SA 4.0; creators require stricter upstream licenses to prevail and identify unresolved source terms | [English card](https://huggingface.co/datasets/Hello-SimpleAI/HC3), [Chinese card](https://huggingface.co/datasets/Hello-SimpleAI/HC3-Chinese), [source-specific terms](https://github.com/Hello-SimpleAI/chatgpt-comparison-detection) | Research-only indicator experiments, initially English subsets with clear terms. Split by question and source; never place answers to the same question in different partitions. This older ChatGPT corpus cannot validate detection of current models or Kazakh/Russian coursework. |
| EdNet | Educational response timing and interaction sequences | Maintainer README: 131,441,538 interactions from 784,309 learners; counts differ slightly from the paper's abstract | CC BY-NC 4.0, research purposes | [Official repository, schema and license](https://github.com/riiid/ednet) | Explore realistic timing distributions and preprocessing. It is a tutoring dataset, lacks validated cheating labels and does not implement Examory's event contract. Do not invent `risk.reviewed` outcomes from performance. |
| Examory synthetic quiz behavior | Attempt-level anomaly detection and event validation | Default: 300 students × 3 attempts × 20 questions = 900 submissions and at least 18,000 answer events; changes add rows | Locally generated; no third-party records. Redistribution license remains the project's decision. | `ml/datasets/generate_quiz_behavior.py` | Reproducible controlled baseline with known injected anomalies; stress-test false positives on ordinary fast, slow and revising students. |

No verified public dataset identified in this review combines Examory's exact events, repeated-attempt history, synchronized peers and independently reviewed misconduct labels. Public educational logs do exist, as EdNet demonstrates. Therefore use synthetic labels for controlled experiments and a separately approved local pilot for external validation. Do not report synthetic detection accuracy as evidence that cheating can be reliably identified.

## Synthetic data generator plan

### Event representation

CSV columns are `event_id,event_type,version,occurred_at,producer,actor,context,payload,is_anomaly,anomaly_type`. The `actor`, `context` and `payload` cells contain JSON, preserving the event contract's nested names rather than inventing flattened aliases.

- `actor`: `user_id`, `role`; `context`: `course_id`, `group_id`, `session_id`.
- `quiz.answered` payload: `attempt_id`, `question_id`, `question_index`, `answer_hash`, `is_change`, `time_since_shown_ms`, `time_since_attempt_start_ms`.
- `quiz.submitted` payload: `attempt_id`, `quiz_id`, `variant_id`, `score`, `max_score`, `duration_ms`, `answers_changed`.

`payload.attempt_id` identifies one attempt; `context.session_id` identifies its exam/session context and is a distinct identifier, never the aggregation key for attempts. Join answers to submissions by `attempt_id` to obtain `quiz_id` and `variant_id`. The generator reuses one quiz, variant and item key across attempts. Its `score` is the number of correct final answers and `max_score` is the question count; derive percentage scores as `100 * score / max_score` before comparing attempts.

The draft does not specify every counting convention. For this synthetic dataset, `question_index` is zero-based; `time_since_shown_ms` is cumulative since the current question was displayed, including revisions; `time_since_attempt_start_ms` is cumulative since the attempt began; and `answers_changed` counts revision events, not distinct revised questions. Record these conventions when comparing against later production data.

The generator writes version 1, RFC 3339 UTC timestamps and deterministic UUID v7 event IDs whose timestamp component equals the millisecond timestamp in `occurred_at`. Answers precede their submission. `answer_hash` is SHA-256 of the canonical normalized option representation, without student, attempt, item or label identifiers. The same normalized answer has the same hash across users; equality must still be compared within aligned questions. Hashes are not a privacy guarantee for four-option questions. `is_anomaly` and `anomaly_type` are offline annotation columns, never payload fields or model inputs.

### Normal profiles and parameters

Generate stable learner ability, item difficulty and individual speed, plus stochastic variation between attempts. Use overlapping lognormal response times; correctness is stochastic and revisions can help or hurt. Scores derive from final answers. Ordinary students may answer quickly, improve, share common answers and submit near classmates. These are necessary hard negatives.

| Normal profile | Student share | Median time before individual scaling | Lognormal sigma | Revision probability per question |
|---|---:|---:|---:|---:|
| Fast | 25% | 18 seconds | 0.65 | 0.08 |
| Careful/revising | 35% | 38 seconds | 0.65 | 0.25 |
| Deliberate | 40% | 28 seconds | 0.65 | 0.15 |

Multiply times by a stable per-student lognormal factor with log-mean 0 and log-sigma 0.15. Normal times have a 1-second floor. A selected normal revision sequence contains 1–2 changes. Draw base ability from Beta(7,3), clipped to [0.25,0.93], item difficulty from Uniform(-0.15,0.15), and add 0.015 ability per attempt. Reuse the same quiz, variant and item key; this is a controlled repeated-assessment model, not a realistic model of memorization.

Defaults: `--students 300 --attempts 3 --questions 20 --seed 42 --anomaly-rate 0.12`. The default anomaly mixture is `rapid_response=0.35,excessive_changes=0.25,collusion=0.25,score_jump=0.15`, normalized over nonnegative weights. Seed and fixed `--start-at` make output reproducible. Use `--output` to select an untracked CSV path.

Only final attempts receive injections, so preceding attempts provide clean score history. The requested count is rounded from `students × attempts × anomaly_rate`, bounded by the number of final attempts; consequently `anomaly_rate <= 1/attempts`. An attempt receives one anomaly type. Collusion needs pairs within the same cohort, so rounding can redistribute an odd slot or reduce the realized count for a collusion-only request. Always record the printed realized count and mixture.

| Injection | Default share of injected attempts | Mechanism | What it tests |
|---|---:|---|---|
| `rapid_response` | 35% | Scale 65% of question durations by Uniform(0.18,0.55), with a 0.5-second floor | Unusually short per-item times and low total duration; speed alone is not misconduct |
| `excessive_changes` | 25% | Draw revision probability from Uniform(0.45,0.75), then make 1–4 changes on selected questions | Abnormal change rate and temporal patterns, with careful normal learners as hard negatives |
| `collusion` | 25% | Paired students in a cohort share answers, with forced shared rare wrong options on about 30% of questions; submissions differ by at most one second | Joint evidence from timing and item-conditioned agreement; common correct answers alone are weak evidence |
| `score_jump` | 15% | Add Uniform(0.18,0.32) to final-attempt latent ability; random outcomes need not produce an observed score increase every time | Unusual improvement against prior history; legitimate improvement must remain possible |

All rows in an injected attempt carry `is_anomaly=1`, including unaffected questions; other rows use `0` and `none`. This is **attempt-level injection membership**, not proof that every action is abnormal. No synthetic `risk.*`, focus, paste or video events are invented. The prototype does not model item randomization, adaptive tests, skipped items or realistic learning trajectories.

Run a default smoke dataset, then a larger evaluation dataset with 2,000 students, three attempts and at least five seeds. Repeat at 1%, 5% and 12% anomaly prevalence, with single-type injections and normal-only data. Change timing/ability conditions for a distribution-shift test; do not tune every evaluation scenario back to the generator.

## Evaluation protocol

### Units and splitting

1. Aggregate answer rows into one record per `payload.attempt_id` and join the corresponding submission by the same identifier. Deduplicate by `event_id` before computing features. Keep `context.session_id` as exam/session context only. Compute first-answer and final-answer times separately: averaging all cumulative revision timestamps would overweight revising students.
2. Use 60%/20%/20% train/validation/test partitions of whole `context.group_id` cohorts. Keep all attempts of a student, both members of a collusion pair and their previous history in one partition. Shuffle cohort allocation with a recorded seed; never split CSV rows at random.
3. Learn transforms, timing reference distributions and Isolation Forest only from training cohorts. Primary experiment uses known normal training attempts; a secondary experiment trains with 1%/5% unknown contamination. Synthetic labels are used for experimental selection/evaluation only.
4. Choose thresholds on validation, freeze them, then evaluate test once. Assess final attempts separately as well as all attempts because the generator injects only in the final round. Report each denominator and its realized prevalence.
5. Compute score deltas from `100 * score / max_score`, with positive `max_score`, using only earlier comparable submissions for that student and quiz/variant. Treat missing history explicitly. Compute synchronization using only already submitted peers or a declared fixed post-submission window; use the same window at evaluation and inference.
6. For text, keep a source document and every derived copy in one partition. For code, group by originating project and connected clone families; if this creates giant components, hold out complete functionalities/projects and report coverage. For HC3, group by question and source and add domain-held-out evaluation. Fit vocabulary, thresholds and supervised classifiers on training data only.

### Metrics

Report `precision=TP/(TP+FP)`, `recall=TP/(TP+FN)`, `F1=2PR/(P+R)` and `FPR=FP/(FP+TN)`. Mark undefined denominators as unavailable and publish counts. Do not call `FP/(TP+FP)` the FPR; that is the false discovery proportion.

Report attempt-level micro scores and per-anomaly recall/F1 against the same ordinary-negative population; additionally report collusion pair precision/recall. Show precision–recall curves, PR-AUC, recall at 1% and 5% FPR, subgroup FPR, and alert count per 100 attempts. Bootstrap confidence intervals by cohort, not by correlated event row. Include runtime, memory, generator seed, dependency versions and split manifest.

For MinHash, report both candidate-retrieval recall and end-to-end pair/span performance: missed candidates count as false negatives. Score source spans separately from suspicious spans using the dataset's annotation convention. Semantic similarity labels and clone labels do not establish misconduct.

### Teacher review as labels

Join `risk.reviewed.payload.risk_id` to the original risk record's `risk_id`. The contract defines review payload fields `risk_id`, `decision`, `reviewed_by` and `comment`; `decision` is `confirmed` or `dismissed`. Use `confirmed` as positive and `dismissed` as negative for the specifically reviewed risk type. Retain the review event's `occurred_at`, reviewer, comment, original model version and decision scope. A behavioral risk uses `target_type="attempt"` and `target_id=attempt_id`; its `evidence.features.synchronous_with` contains peer `attempt_id` values. Pending, conflicting or insufficient-evidence decisions remain unknown; adjudicate disagreements.

Reviewing only alerts estimates precision or dismissal rate among reviewed alerts. It cannot estimate population recall or FPR. Independently audit a random sample of unflagged attempts and a sample of alerts, record sampling probabilities, and use weighted population estimates where appropriate. Do not label every unflagged attempt negative.

Preserve a future, adjudicated cohort as the final holdout. Review decisions are imperfect human labels, not unquestionable ground truth: use independent second review on a subset, track disagreements and appeals, and exclude unresolved cases. Report false positives for non-native English writers and relevant testing accommodations only where those attributes are voluntarily and lawfully collected; do not infer them from a name, face or writing style.
