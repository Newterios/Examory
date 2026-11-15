# Feature-injection inflation experiment

Harness and statistical analysis for *"Feature injection can inflate the performance of
quiz-cheating detectors: evidence from behaviour-level simulation"*.

The detector pipeline `ml/anomaly/{features,split,train,evaluate}.py` is **frozen**: this
package imports it (`train.train`, `features.build_features`, `train.score_model`,
`train.choose_threshold`, `train.matrix`, `split.split_groups`) and never modifies it.
Every stage JSON records the git HEAD, the HEAD and working-tree git blob hashes of
`ml/anomaly/*.py`, the SHA-256 of every generated CSV, simulator manifest, parameter
and mechanism file, the simulator binary and source tree, library versions and wall time.

## Layout

| File | Purpose |
|---|---|
| `common.py` | constants (conditions, slots, seeds), provenance, JSON I/O |
| `data.py` | generate/reuse datasets (legacy generator, `examory-sim`), hash checks, truth join, pairing checks |
| `pipeline.py` | cohort split, frozen fit, scoring, `.npz` score cache, robust detectors, per-subset evaluation |
| `metrics.py` | recall/precision/F1/FPR at a threshold, ROC-AUC, McClish standardized pAUC, AP, cohort bootstrap |
| `stats.py` | seed-bootstrap CIs, paired/unpaired tests, Holm, Spearman, Latin hypercube |
| `run.py` | stage CLI |
| `ednet.py` | EdNet stage (P1 transfer, P2 refit, realism) |
| `analyze.py` | aggregation, Markdown tables, figures |
| `supervised.py` | post hoc stage `supervised`: supervised classifiers on the frozen feature table |
| `similarity.py` | post hoc stage `similarity`: answer-similarity detector AnswerSim |
| `posthoc.py` | aggregation, tables and figure F7 of the two post hoc stages |
| `review.py` | stages added after independent review: `fi-dose`, `protocol`, `baseline`, sensitivity v2 helpers |
| `review_analysis.py` | their aggregation, tables, figures F5 (v2) and F8, sub-populations, prevalence diagnostic, p-value changes |
| `compact.py` | compact committed stage format and its inverse |
| `results_defaults/` | appendix: uncalibrated-defaults run of 14 Nov 2026 |
| `results/` | committed outputs: `stages/*.json`, `summary.json`, `tables/*.md`, `figures/*.{png,pdf}` |

Generated data, model pickles and per-attempt score caches (`scores.npz`) live under
`ml/datasets/raw/exp/` (git-ignored). No CSV is written to the repository.

## Reproduce

From the repository root (Python 3.14 venv, `ml/requirements.txt` + scipy/matplotlib):

```bash
# simulator (contract build; the harness also accepts --sim-binary PATH)
(cd sim && go build -o bin/examory-sim ./cmd/examory-sim)

PY=.venv/bin/python
$PY -m ml.experiments.inflation.run main               # 20 seeds × {LEGACY, SIM-FI, SIM-BL}
$PY -m ml.experiments.inflation.run transfer
$PY -m ml.experiments.inflation.run robust-detectors
$PY -m ml.experiments.inflation.run dose               # 10 seeds × 5 intensities
$PY -m ml.experiments.inflation.run sensitivity        # 40 LHS configs × 3 seeds
$PY -m ml.experiments.inflation.run prevalence         # anomaly-rate 0.0167
$PY -m ml.experiments.inflation.run ednet --ednet-csv ml/datasets/raw/ednet/derived/<file>.csv
$PY -m ml.experiments.inflation.analyze                # summary.json, tables, figures

# post hoc, exploratory (amendment of 15 Nov 2026); rerun analyze afterwards
$PY -m ml.experiments.inflation.run supervised         # 20 seeds × 6 train→test cells
$PY -m ml.experiments.inflation.run similarity         # 20 seeds × {LEGACY, SIM-FI, SIM-BL, SIM-HONEST}

# after independent review (amendment of 15 Nov 2026); rerun analyze afterwards
$PY -m ml.experiments.inflation.run fi-dose            # 10 seeds × (4 λ + 5 intensities)
$PY -m ml.experiments.inflation.run protocol           # 20 seeds × 3 conditions × 3 protocol variants
$PY -m ml.experiments.inflation.run sensitivity        # v2: (60 configurations + default) × 10 seeds
$PY -m ml.experiments.inflation.run baseline           # 20 seeds × 4 honest-baseline variants
                                                       # (closing_* use --sim-binary-next, default sim/bin/examory-sim-next)
```

`--sim-binary` (default `sim/bin/examory-sim`) selects the simulator for every stage; `--sim-binary-next`
is used only by the `baseline` closing-time variants (simulator ≥ 0.3.0).

Simulator parameters: `ml/ednet/calibration.json` when it exists, else
`sim/params/default.json`; the choice is recorded as `params.source` in each stage JSON
(`ednet_calibration` / `sim_defaults` / `explicit`). Without a simulator binary only
`--conditions LEGACY` runs. `--from-cache` re-evaluates the cached `.npz` scores without
refitting (the CSV hash must still match). `--workers` (default 10) parallelises over seeds.

Tests: `.venv/bin/python -m unittest ml.tests.test_inflation` (≈20 s; uses a stub
simulator that copies legacy CSVs into the simulator file contract).

Runtimes measured on the development machine (14 cores, 10 workers) are listed in
`results/summary.json` → `stages.*.wall_seconds`.

## Conditions and design

* **LEGACY** – `ml/datasets/generate_quiz_behavior.py` (feature-aligned injection).
* **SIM-FI** – simulator world `fi`: the legacy recipes applied to the cheaters' honest attempts.
* **SIM-BL** – simulator world `bl`: cheating simulated as behaviour (preknowledge, lookup, copy_peer, proxy).

SIM-FI and SIM-BL of one seed are paired counterfactuals: same honest attempts, same
cheaters, same slots. The harness asserts identical attempt IDs, cheater sets, slot
assignment and cohort partition. Main design: `--students 2000 --attempts 3
--questions 20 --anomaly-rate 0.12`, seeds 1001–1020; the generator seed is also the
cohort-split seed. Slots: S1 rapid_response↔preknowledge, S2 excessive_changes↔lookup,
S3 collusion↔copy_peer, S4 score_jump↔proxy.

## Analysis plan

**What was fixed when.** The protocol, the metrics, the primary endpoint (SIM-FI − SIM-BL
recall per slot × detector), its tests and Holm correction, and the H1 decision rule were fixed
before the multi-seed runs. A smoke run of `main` on a pre-release simulator build preceded the
first amendment below. The seven amendments of 14 Nov 2026 were made after the first full
multi-seed run with uncalibrated parameters (`results_defaults/`) and before the calibrated rerun
(`results/`); they added secondary analyses and output formats and did not change any metric, test,
correction or decision rule. Plan, simulator, calibration and results were committed together, so
the repository history does not independently date these steps.

**Protocol per seed and condition.** Cohorts split 60/20/20 (`split_groups`). Reference
(timing quantiles, answer frequencies) and models fitted on normal attempts of training
cohorts only (`train.train`). Thresholds chosen on validation cohorts only by
`choose_threshold` at target FPR 5 % (primary) and 1 %. All metrics on test cohorts only.

**Metrics.** For each detector (`iforest`, `iforest_no_peers`, `quantile_rules`) on (a) all
test attempts, (b) final attempts, (c) each slot's cheaters vs all normal test attempts:
recall, precision, F1, realized FPR, alerts per 100 at the frozen 5 % threshold (and recall/FPR
at 1 %); ROC-AUC; standardized partial AUC on FPR ∈ [0, 0.05] (McClish: 0.5 = chance,
1 = perfect; a ROC with TPR = 0 throughout the region gives 0.487); average precision.
SIM-BL breakdowns by truth fields: copy_peer by `role` (source/copier) and by the pair-level
`sync_submit` flag (sync vs non-sync submission; any truth column containing "sync"), preknowledge and
lookup by `mech_flag` (naive/disguising; verify/lookup_then_answer).

**Primary endpoint.** Inflation Δ = recall@5 %(SIM-FI) − recall@5 %(SIM-BL) per slot × detector,
paired by seed; mean over seeds with paired percentile bootstrap 95 % CI (10 000 resamples)
and two-sided paired test (Wilcoxon signed-rank until 15 Nov 2026; since then the exact sign-flip
permutation test, see the amendment of 15 Nov 2026 "after independent review"); Holm correction
across the 12 slot × detector tests.

**Secondary endpoints.** (i) LEGACY − SIM-BL recall (unpaired: independent bootstrap CI,
Mann–Whitney U, Holm over 12). (ii) The same two comparisons for standardized pAUC (each its
own Holm family of 12). (iii) Every cell: mean and 95 % percentile-bootstrap CI over seeds
(seeds are the independent replicates); the within-run cohort-bootstrap CIs (500 resamples
of test cohorts) are kept per run and summarised as their median width. (iv) Transfer
matrix. (v) Robust detectors, dose–response, sensitivity, prevalence and EdNet are
descriptive/exploratory; Spearman correlations in the sensitivity analysis are uncorrected.

**H1 (verification plan §0), tested on SIM-BL.** Gain = recall@5 %(iforest) −
recall@5 %(quantile_rules) on S3 (copy_peer) and S4 (proxy), paired by seed (bootstrap CI,
paired test as for the primary endpoint, Holm over the 2 slots). Decision on the across-seed mean gain per slot: ≥ 15 pp
supported; < 5 pp rules adopted; otherwise inconclusive. H1 supported only if both slots are
supported; "rules adopted" if both gains are < 5 pp; anything else is mixed/inconclusive.
The outcome is reported whatever it is; the same computation is shown for LEGACY and SIM-FI.

**Amendment log.** After a smoke run of `main` on a pre-release simulator build, and at the
simulator team's request (new mechanism parameter), two items were added: the `sync_submit`
breakdown of copy_peer and the `copy_peer.sync_submit_prob` sensitivity dimension. No metric,
threshold, test, correction or decision rule was changed.

Amendments of 14 Nov 2026 (coordinator request, before the calibrated rerun; no metric, test,
correction or decision rule changed):
1. The uncalibrated run (`sim/params/default.json`) is kept as an appendix in `results_defaults/`
   (compact `summary.json`, `tables/*.md`, F1); `results/` now holds the EdNet-calibrated run
   (`params.source = ednet_calibration`).
2. Disk: every stage deletes each generated dataset directory and model pickle as soon as its scores
   are cached (`--keep-data` opts out); only in-flight workers' data are on disk.
3. Committed size: `results/stages/*.json` are compact (6 significant digits) and columnar per-seed
   scalar metrics (`compact.py`); ROC curves only as mean and 2.5/97.5 % band over seeds on the fixed
   FPR grid (main stage); full per-run output goes to `ml/datasets/raw/exp/stages_full/`. `analyze`
   regenerates all tables and figures from `results/stages/` alone. PNGs are saved as 256-colour palettes.
4. EdNet: SIM source = honest world with `--item-sampling per-attempt` and the calibrated params
   (`SIM-calibrated`), plus the uncalibrated defaults (`SIM-defaults`) side by side with LEGACY in P1 and
   realism. Primary C2ST on behavioural BASE_FEATURES (without `history_missing`, `z_score`, which encode
   attempts per user); all-BASE_FEATURES C2ST kept as a secondary row. P1/P2 strata add quartiles of
   `fast_answer_fraction`; all strata use EdNet's own-reference features; a value equal to a cut point now
   falls in the lower quartile (Q1 = values ≤ 25th percentile), so tied low values no longer leave Q1 empty;
   strata are stored at the 5 % operating point only.
5. Robust detectors: cells are marked "† k/n" (unreliable) when the validation FPR at the 5 % target was
   < 2.5 % in k of n seeds (degenerate score distribution, e.g. singular MCD covariance).
6. Main stage: a descriptive "SIM-BL copy_peer, non-sync pairs (≈ sync_submit_prob = 0)" block and a
   manipulation-check table (mean paired difference cheater − own honest counterfactual in score points,
   duration, answer changes and median first-answer time) for the fi and bl worlds.
7. Figure labels: "Legacy generator", "Feature-aligned injection (simulated normals)", "Behaviour-level
   cheating"; detectors "Rules" (quantile_rules), "IF" (iforest_no_peers), "IF + peers" (iforest).

Amendment of 15 Nov 2026 (post hoc, coordinator request): the stages `supervised` and `similarity`
below were added **after the main results had been seen**. They are exploratory: their results do not
enter the primary endpoint, H1 or any Holm family, and their p-values are unadjusted. Their definitions
were fixed in the request before they were run and were not tuned afterwards: (a) `supervised` — the
frozen 15-/12-feature table, HistGradientBoostingClassifier (defaults, `random_state = seed`; primary) and
LogisticRegression on standardized features (defaults), no hyper-parameter search, threshold by
`choose_threshold` on the training world's validation cohorts, cells train ∈ {SIM-FI, SIM-BL, LEGACY} ×
test ∈ {SIM-FI, SIM-BL}, and the three contrasts listed below; (b) `similarity` — the AnswerSim statistic
and its incorrect-answers variant as defined below, plugged into the main protocol. Choices made while
implementing them (before the 20-seed runs; a 2-seed smoke run preceded them) are listed in the stage
descriptions (pairs of the same student excluded; answer key recovered from scores; no-partner score;
target-world features built with the training world's reference). After the smoke run only presentation
changed (LEGACY → SIM-BL left out of figure F7 because its realized FPR is not near 5 %). No existing
stage, metric, test, decision rule, stage file, table or figure was changed; `summary.json` only gained the
keys `supervised` and `similarity` (and its provenance fields were refreshed).

Amendment of 15 Nov 2026, **after independent review of the draft** (coordinator request; all items are
post hoc and exploratory unless they only change how existing results are tested or printed):
- A. Stage `fi-dose`: injection magnitude λ ∈ {0.25, 0.5, 0.75, 1} multiplying `fi.rapid_item_prob`,
  `fi.changes_prob_low/high`, `fi.collusion_copy_prob`, `fi.score_jump_low/high` (generated mechanisms
  file, no simulator change), crossed with the dose stage's behaviour-level intensity grid, seeds
  1001–1010. Finding recorded first: `--intensity` acts only on the bl world (`world.go: s.intensity` is
  used in `buildBL` only); the dose stage's SIM-FI CSVs are byte-identical across intensities.
- B. Sub-population recall from the main stage (pre-knowledge naive vs disguising, copy_peer sync vs
  non-sync) and the count of cheating with no observable change (`tables/subpopulations.md`).
- C. Stage `protocol`: label-free threshold, contaminated fitting, final-attempt evaluation.
- D. Sensitivity analysis v2 replaces v1 in `tables/sensitivity.md` and figure F5; v1's stage file is kept
  as `stages/sensitivity_v1.json` (its table/figure are no longer generated). This changes the numbers
  of that table and figure by design (new design), not by a change of method for the same design.
- E. Stage `baseline`: exam-like revision rate, per-attempt item sampling, fixed closing time
  (simulator 0.3.0, `--sim-binary-next`).
- F. Response-time person-fit l_t added to the classical-baselines stage `similarity`
  (AnswerSim numbers unchanged).
- G. (1) Every paired p-value is now the exact two-sided sign-flip permutation test on the per-seed
  differences (all 2^n sign patterns, n ≤ 20), replacing Wilcoxon (SciPy chose exact or asymptotic
  variants, giving p = 0.0008 for a 68-pp effect and p < 0.0001 for a 26-pp effect); Holm is kept for the
  primary family of 12. **This changes p-values (and Holm p-values) in the committed tables
  `inflation.md`, `robust_inflation.md`, `h1.md` and `supervised.md`** (and the `copy_peer_sync` test stored in
  `summary.json`);
  `tables/pvalue_changes.md` lists every cell that crossed 0.05. Estimates and CIs are unchanged.
  (2) Monte Carlo standard error columns (SD of per-seed values / √n) in `main.md`, `inflation.md`
  (and `robust_detectors.md`, `robust_inflation.md`, which use the same printers). (3) `robust_inflation.md`
  said "family of 12" while Holm was applied over the 24 tests (6 detectors × 4 slots) of each family;
  header and computation now agree (24 there, 12 in `inflation.md`, printed per block). (4) `ednet.md`
  labels the across-seed interval and the within-run cohort/user-bootstrap interval for flag rates and
  both C2ST feature sets. (5) Title "inflates" → "can inflate"; the limitation on the C2ST features is
  corrected to match amendment 4.
- H. Diagnostic of Rules recall on copy_peer at low prevalence (`tables/prevalence_diagnostic.md`).
- Follow-up (same day, after review of round 2): (1) the `protocol` table also reports the H1 contrast
  recall(IF + peers) − recall(Rules) on SIM-BL (S3, S4, all) under each protocol variant, with paired CI and
  exact sign-flip p (the main-protocol rows repeat `h1.md`), and the share of train-cohort attempts that are
  cheaters (what contaminated fitting presents as normal); (2) the three protocol variants are repeated at the
  low prevalence of the `prevalence` stage (`run protocol --anomaly-rate 0.0167 --output
  results/stages/protocol_prevalence.json`, `tables/protocol_prevalence.md`; its main-protocol rows reproduce
  `prevalence.json`, checked); (3) `ednet.md` adds the per-seed minimum – maximum of the flag rate next to the
  across-seed mean, and explains why `quantile_rules` and `quantile_rules_no_peers` coincide on some seeds
  (EdNet attempts have no synchronous peers, so the peer columns never set the rule score; checked on seed 1001:
  identical thresholds and identical flag vectors, i.e. not a bug; no number changed). The full (non-compact)
  copy of a stage written with `--output` is now named after the output file in `stages_full/`.

**Transfer.** Fit (reference + model + threshold) on condition X, apply to test cohorts of Y
for (LEGACY→SIM-BL), (SIM-FI→SIM-BL), (SIM-BL→SIM-FI), (LEGACY→SIM-FI) and the diagonals;
report realized FPR and recall ("what a team that validated on X would see on Y").

**Robust detectors (appendix).** LOF (novelty, default k = 20), One-class SVM (RBF,
gamma = scale, nu = 0.05) and robust Mahalanobis distance (MinCovDet, random_state 42) on the
frozen model matrix (`train.matrix`), standardized with training statistics, constant and
exactly duplicated columns removed, fitted on normal training attempts, each with and without
peer features; thresholds by `choose_threshold` on validation.

**Dose.** SIM-BL at `--intensity` ∈ {0.1, 0.25, 0.5, 0.75, 1.0}, seeds 1001–1010.
**Sensitivity.** 40-point Latin hypercube (RNG seed 20261114) over the 10 mechanism
parameters listed in `run.SENSITIVITY_PARAMETERS` (incl. `copy_peer.sync_submit_prob` ∈ [0, 1]) (medians of log-normal durations sampled
log-uniformly; `speedup_low = 0.4 × speedup_high`), seeds 1001–1003; per configuration Δ =
mean over seeds of the paired difference; reported: distribution, share Δ > 0 and > 10 pp,
Spearman ρ per parameter. **Prevalence.** main protocol at anomaly-rate 0.0167.

**EdNet (unlabelled real data).** Assumption: cheating is negligible in an ungraded
self-study tutor, so the flag rate estimates the realized FPR (not verifiable). P1: detectors
and thresholds from LEGACY (threshold with LEGACY validation labels) and from the SIM honest
world (`--item-sampling per-attempt`, all-zero labels) applied to EdNet test cohorts; P2:
refit on EdNet train cohorts, threshold on EdNet validation (all-zero labels). Flag rate with
cohort-bootstrap CI, per seed and mean over seeds; strata by quartiles (EdNet-train cut points)
of score and median answer time and by any answer change. Peer features are not meaningful on
EdNet: `iforest` and `quantile_rules` (both use peer features in the frozen pipeline) are
reported but flagged; `quantile_rules_no_peers` (frozen `quantile_scores` on BASE_FEATURES) is
added as the second peer-free detector. Realism: HistGradientBoosting C2ST real-vs-synthetic on
BASE_FEATURES, 5-fold StratifiedGroupKFold by user, AUC with user-bootstrap CI, per-feature KS
and Wasserstein/SD(EdNet). EdNet is subsampled to whole cohorts (≤ 30 000 attempts, fixed seed)
when larger; C2ST uses ≤ 6 000 attempts per class.

**Supervised (post hoc, exploratory).** Question: how much of the cheating is recoverable from the
frozen features when labels are available (an information ceiling for this feature set), and what a
supervised detector trained on feature-injected labels achieves on behaviour-level cheating. Feature
table exactly as in the transfer stage: `build_features` with the *training* world's normal-train
reference (via `train.train`), peers computed within the target world; model matrix `train.matrix`
(log transform, missing indicators, medians from the training world's train-cohort rows) on all 15
features (`hgb`, `logreg`) or the 12 attempt features (`*_no_peers`), mirroring "IF + peers" vs "IF".
Labels: attempt-level any-cheating vs none from the training world's `truth.csv` (`is_anomaly`, checked
against the CSV label; LEGACY: the CSV label), all train-cohort attempts. Thresholds: `choose_threshold`
(5 % and 1 %) on the training world's validation cohorts; metrics: `evaluate_table` on the target world's
test cohorts (recall per type, realized FPR, ROC-AUC, standardized pAUC, sync/non-sync copy_peer split).
Cells: SIM-FI→SIM-FI, SIM-FI→SIM-BL, SIM-BL→SIM-FI, SIM-BL→SIM-BL, LEGACY→SIM-FI, LEGACY→SIM-BL (LEGACY
uses the same code path; its realized FPR on SIM targets is far above 5 %, as in the transfer stage, so
its recall is not at a 5 % operating point). Contrasts, paired by seed (bootstrap CI, sign-flip test,
unadjusted), overall and per slot, for every model (primary: `hgb` with 15 features): (1) recall(FI→FI) −
recall(FI→BL); (2) recall(BL→BL) − recall(FI→BL); (3) recall(BL→BL) − recall of the best unsupervised
detector of the main stage on SIM-BL (the one of Rules / IF / IF + peers with the highest mean recall on
that subset; same seeds and data, verified by CSV hash). Outputs: `tables/supervised.md`,
`figures/F7_supervised.{png,pdf}`, `summary.json → supervised`.

**Similarity (post hoc, exploratory).** Question: is behaviour-level copying detectable by a classical
answer-similarity statistic that does not use submission-time synchrony? AnswerSim: for attempts i, j of
*different students* in the same cohort on the same quiz/variant sharing an item set S with |S| ≥ 10,
m_k = 1 if both final selected options on item k are identical; p_k = Σ_o f_ko², f_ko the relative
frequency of option o on item k among all train-cohort attempts (no labels);
z_ij = (Σ m_k − Σ p_k) / √(Σ p_k(1 − p_k)); attempt score = max_j z_ij; attempts with no eligible partner
score −10⁹ (below every attainable z). Secondary variant `answersim_incorrect`: the same sums over the
items of S on which attempt i's final answer is incorrect. The event log has no answer key; it is
recovered per item from the submitted scores (the option whose choosers have the highest mean score) and
accepted only if it reproduces the score of every attempt exactly (it did in all 80 runs). Threshold,
metrics and breakdowns as in the main stage, for LEGACY, SIM-FI, SIM-BL and the honest world (FPR only).
Item sharing: the main configuration uses the simulator's default `--item-sampling fixed`, so all
attempts share the same 20 items (|S| = 20, 57 eligible partners per attempt). The statistic is then a
monotone function of the match count and takes few distinct values, so the realized operating point lies
below 5 % FPR. Outputs: `tables/similarity.md`, `summary.json → similarity`.

**Fi-dose (after review).** For each λ the harness writes `fi_dose_mechanisms/lambda_<λ>.json` (λ × the six
listed fi parameters, capped at 1) and generates worlds honest+fi with it; λ = 1 writes a file identical in
content to `sim/params/mechanisms.json` and reproduces the main stage's SIM-FI CSV (hash-checked). SIM-BL
at the dose stage's intensities is regenerated (worlds honest+bl, identical bl CSVs to the dose stage,
checked). Output per detector × slot: recall_FI(λ), recall_BL(x) and the 4 × 5 grid Δ = recall_FI(λ) −
recall_BL(x), paired by seed (bootstrap CI; no tests); figure F8 (heat-map per slot × detector with the
Δ = 0 contour). Common dose scale per grid point: mean score gain of cheaters vs their honest
counterfactual (manipulation check) and mean `truth.csv:items_affected`. Collusion keeps its 6
forced-wrong items and the 1-s submit window at every λ, so λ does not dose that recipe's
synchrony signal. `--intensity x` replaces the per-mechanism defaults (0.65 / 0.70 / 0.80, proxy scale 1), so
x = 1 is a stronger bl dose than the main stage; the table and F8 therefore add the main stage's SIM-BL
(no `--intensity`, same seeds) as column `x = default`, and the main contrast is (λ = 1, x = default).

**Protocol (after review).** Main configuration, 20 seeds, LEGACY / SIM-FI / SIM-BL. Per seed and condition
the standard fit (reproduces `main.json`, checked) is evaluated with (a) the main threshold rule, (b) a
label-free threshold `numpy.quantile(validation normal scores, 0.95)` (linear interpolation; 0.99 for the
1 % point), and (c) a contaminated fit: a copy of the CSV in which train-cohort cheaters carry
`is_anomaly = 0`, `anomaly_type = none`, so the frozen `train.train` fits references and models on all
train-cohort attempts; features of the original data are rebuilt with that reference; validation/test labels
are untouched and the main threshold rule applies. Each variant also reports FPR and precision against final
normal attempts only (`final_only:*`). The standard run also records feature diagnostics and, for SIM-FI /
SIM-BL, which cheater attempts are event-identical to the honest counterfactual.

**Sensitivity v2 (after review).** 60 configurations × seeds 1001–1010 over the 10 v1 behaviour parameters
plus `fi.rapid_item_prob` U(0.30, 0.90), a common multiplier on `fi.changes_prob_low/high` U(0.5, 1.25)
(probabilities capped at 1), `fi.collusion_submit_window_ms` log-uniform(500, 20000) (rounded to ms) and a
common multiplier on `fi.score_jump_low/high` U(0.5, 1.5). Latin hypercube (RNG seed 20261115) decorrelated
by within-column swaps until the maximum absolute Spearman correlation between columns is ≤ 0.10 (the achieved
value is recorded in the stage JSON and the table). The default configuration runs on the same seeds
(`config_id = -1`). Reported: SIM-FI and SIM-BL recall and Δ per configuration, the v1 distribution
summaries, the default configuration's Δ with CI, and partial rank correlation coefficients (all 14
parameters jointly; number of tests stated in the table).

**Baseline (after review).** Each variant is a full main-protocol run (20 seeds, SIM-FI and SIM-BL, with the
manipulation check and feature diagnostics). (a) `revision`: both `revision.change_mult_*` of the calibrated
parameters multiplied by m (≡ scaling the honest revision propensity r_i while r_i·mult < 0.95); m found by
bisection on log m so that the honest world of seed 1001 has 0.84 ± 0.005 answer changes per 20-item attempt. (The calibrated parameters give 4.66 changes per 20-item attempt on that world, not ≈ 6.7.)
(b) `per_attempt`: `--item-sampling per-attempt`. (c) `closing_p90` / `closing_p75`: simulator 0.3.0
`--close-after-ms` at the 90th / 75th percentile of the honest natural submission time after the sitting
opens (seed 1001, no window; 1 792 521 and 1 600 450 ms) and `--auto-submit-jitter-ms 2000`; the copy_peer
breakdown then also splits by `truth.csv:auto_submitted`.

**RT person-fit (after review; in `similarity`).** ln t_ik = β_k − τ_i + ε_ik fitted on all train-cohort
attempts by alternating means (no labels; τ = 0 start; stop when the largest change < 1e-12); σ_k = SD
(ddof 0) of item k's residuals; for any attempt τ_i = mean_k(β_k − ln t_ik) over its items, z_ik =
(ln t_ik − (β_k − τ_i)) / σ_k, score l_t = Σ_k z_ik² (Marianti et al., 2014). t_ik is the first-answer
latency `time_since_shown_ms` of the item's first `quiz.answered` event: the time from the item being shown
to the first selection. Later answer changes, including follow-up changes made after the student moved on
to other items (copier follow-ups), do not enter t_ik; the end of an item visit is not observable in the
event stream.

## Leakage and integrity guards

* `train.train` fits references/models on normal training cohorts; the harness re-checks the count.
* Thresholds only from validation scores; evaluation masks select test cohorts only.
* Transfer refuses to run when two conditions share cohorts but not the partition.
* Every generated directory stores `generation.json` (command + SHA-256 of all outputs); reuse
  re-hashes and fails on mismatch; simulator manifest hashes and parameter hashes are checked.
* Missing or inconsistent truth rows, paired-world mismatches and unexpected anomaly types raise.

## Known limitations

* Synthetic results are synthetic: SIM-BL encodes assumed mechanisms; the sensitivity stage
  probes, but cannot exhaust, alternative assumptions.
* Thresholds on 1 200 validation attempts are coarse at the 1 % point (≈ 12 negatives above it).
* The 20 seeds share code and parameter files; seed variation does not cover model uncertainty.
* EdNet attempts are pseudo-attempts (20 consecutive Part-5 responses), not exams; the
  no-cheating assumption cannot be checked, and peer features have no meaning there.
* The primary C2ST excludes `history_missing`/`z_score` (amendment 4); only the secondary
  all-BASE_FEATURES C2ST uses them, and it partly reflects the attempts-per-user design rather than
  behaviour.
* The simulator (`sim/`) was not yet committed when the stages ran; it is now tracked in git
  (commit 31485bd), and every stage records its source tree hash and binary hash.
* Each stage JSON records the SHA-256 of the harness files at the time the stage ran
  (`provenance.harness_sha256`). `analyze.py`, `common.py` and `compact.py` were changed after the stages
  ran (compact stage format, table and figure labels), so their committed hashes differ from those
  recorded in `results/stages/main.json`; the other harness files match. Check performed on the
  committed code (14 Nov 2026): re-running `compact` and `analyze` reproduces every committed stage
  file, table and figure byte for byte; only provenance fields of `summary.json` change.
