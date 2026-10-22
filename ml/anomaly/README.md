<!-- ml/anomaly/README.md -->
# LMS-18: behavior baseline

Run from the repository root with Python 3.12. Direct dependencies are pinned;
pip installs their normal transitive dependencies. No extra Python packages are
required. Both `python -m ml.anomaly.train` and direct script invocation work.

```bash
python -m pip install -r ml/requirements.txt
python ml/datasets/generate_quiz_behavior.py --students 2000 --seed 42 --output ml/datasets/raw/quiz.csv
python -m ml.anomaly.split --csv ml/datasets/raw/quiz.csv --seed 42
python -m ml.anomaly.train --csv ml/datasets/raw/quiz.csv
python -m ml.anomaly.evaluate --csv ml/datasets/raw/quiz.csv --bootstrap 1000
python -m ml.anomaly.to_risk_event --csv ml/datasets/raw/quiz.csv
python -m unittest discover -s ml/tests -v
```

To reproduce the plan's calendar origin, append
`--attempts 3 --questions 20 --anomaly-rate 0.12 --start-at 2026-12-01T09:00:00Z`
to the generator command. Absolute dates are never predictors. Repeat generator
seeds 43–46 and rates 0, 0.01, 0.05 in separate input/artifact directories, using
`--output` for split/train/evaluate and `--manifest`/`--model` to select them.
Keep model random_state=42. Do not pool repeated simulations as independent
real students; compare methods on the same cohort manifests.

Optional feature export:

```bash
python -m ml.anomaly.features --csv ml/datasets/raw/quiz.csv --manifest ml/anomaly/artifacts/split.json --output ml/anomaly/artifacts/features.csv --reference-output ml/anomaly/artifacts/reference.json
```

`split.json` records whole group IDs, attempt IDs, seed, counts and CSV SHA-256.
At least five cohorts are needed. A student crossing cohorts is rejected.
The 60/20/20 proportions apply to cohort counts, with largest-remainder rounding.

`train` fits normal-only timing references, Laplace-smoothed item answer
frequencies, imputation and three baselines: the required Isolation Forest,
the same estimator without peer features, and two-sided training-quantile rules.
Count/time features use log1p; every numeric feature also has a missing indicator.
The model matrix has an explicit allowlist: labels, IDs and dates are excluded.
The fixed forest has 200 trees, max_samples=min(256,n_train), contamination=auto,
random_state=42. Artifacts include the model, reference distributions, normal
training score CDF, environment and frozen validation thresholds.

First-answer timing uses the initial event per question; revisions do not inflate
the mean. Variance uses ddof=0. Fast fraction means first times below the item's
normal-train p10 (at least 30 observations), otherwise global normal-train p10.
Final response time is cumulative time on the last answered question.
Changes fraction = revision events / all answer events. Percentage score is
100*score/max_score. The standardized delta uses the mean of strictly earlier
comparable attempts and denominator max(prior population SD, training delta SD,
5 percentage points); absent history stays missing and has a separate indicator.

Peer comparison uses different students in the same quiz, variant and cohort,
within ±5 seconds, including normal peers. Hashes are aligned by question_id.
Agreement is the maximum common-item match fraction; rarity agreement is the
maximum mean matched-answer surprisal under normal-train answer frequencies.
The CSV has no answer key: this is not specifically wrong-answer agreement.
Inference must wait for a five-second event-time watermark and supply complete
cohort history. The CSV batch converter assumes that watermark has passed.

Predictions use raw reversed score_samples >= the validation threshold. Select
maximum validation recall subject to FPR<=5%, breaking ties by fewer false
positives. A separate 1% point requires at least 100 validation negatives to be
reported as supported (this is resolution, not a statistical guarantee).
Medium starts at the 5% threshold; high starts at max(5%,1% thresholds).
Equal cutoffs can produce no medium cases. Severity uses raw scores because the
empirical CDF can saturate at 1. The event score is a rank, not a probability.
Explanation lists marginal outliers, not causal forest feature attribution.

Evaluation freezes all choices before test: all attempts, final attempts and
each injection type versus the same normal negatives; trapezoidal PR-AUC and
average precision; cohort bootstrap 95% intervals; and the achieved test FPR at
validation-selected operating points. Test FPR is not guaranteed to meet the
validation target. Undefined metrics are NA. `evaluation.json` retains PR curves.
Latent fast/careful profile and actual collusion partner IDs are not exported by
the generator, so true profile FPR and pair-level metrics cannot be recovered.
Observed fast/revision proxies are explicitly labelled as proxies.

This baseline cannot establish cheating. Synthetic injection labels do not
necessarily produce observable anomalies. Before real use, independently review
both flagged and randomly sampled unflagged attempts. Join risk.reviewed by
risk_id; confirmed/dismissed label the reviewed risk only. Never treat unreviewed
attempts as negatives. Raw inputs and artifacts are ignored by ml/.gitignore;
load pickle files only from trusted locally generated runs.
