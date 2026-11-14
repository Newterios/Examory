# Feature-injection inflation experiment

Harness and statistical analysis for *"Feature injection inflates the performance of
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
```

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

## Pre-specified analysis plan (written before any result was inspected)

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
and two-sided Wilcoxon signed-rank test; Holm correction across the 12 slot × detector tests.

**Secondary endpoints.** (i) LEGACY − SIM-BL recall (unpaired: independent bootstrap CI,
Mann–Whitney U, Holm over 12). (ii) The same two comparisons for standardized pAUC (each its
own Holm family of 12). (iii) Every cell: mean and 95 % percentile-bootstrap CI over seeds
(seeds are the independent replicates); the within-run cohort-bootstrap CIs (500 resamples
of test cohorts) are kept per run and summarised as their median width. (iv) Transfer
matrix. (v) Robust detectors, dose–response, sensitivity, prevalence and EdNet are
descriptive/exploratory; Spearman correlations in the sensitivity analysis are uncorrected.

**H1 (verification plan §0), tested on SIM-BL.** Gain = recall@5 %(iforest) −
recall@5 %(quantile_rules) on S3 (copy_peer) and S4 (proxy), paired by seed (bootstrap CI,
Wilcoxon, Holm over the 2 slots). Decision on the across-seed mean gain per slot: ≥ 15 pp
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
* The C2ST uses `history_missing`/`z_score`, which partly reflect the attempts-per-user design
  rather than behaviour.
* The simulator lives in an untracked directory (`sim/`); its source tree hash and binary hash
  are recorded instead of a git revision.
