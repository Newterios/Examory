# EdNet pipeline (real behaviour for the cheating-detector study)

Two outputs for the paper *Feature injection inflates the performance of quiz-cheating detectors*:

1. **Unlabelled real attempts**: EdNet evaluation users' TOEIC Part-5 sittings converted to the
   Examory quiz event CSV (`docs/03_architecture/events.md`), so the frozen detector pipeline
   (`ml/anomaly/`) can score real behaviour unchanged.
2. **Calibration** of the latent learner model used by the Go simulator: `calibration.json`
   (schema of `sim/params/default.json`) and `calibration_report.md` (estimates, uncertainty and
   posterior-predictive fit diagnostics).

## Provenance and licence

- EdNet-KT2 (`EdNet-KT2.zip`, SHA-256 `bb5dec5708b6af913d8b99997e1a74699fced4555288862a80925bab3b5abff5`)
  and the content tables (`contents.zip`, SHA-256 `aa910a0436d9dbac0ba232f55e27b37ad8d39da285fcf15f1c1eb5d064be98e7`)
  from the official repository <https://github.com/riiid/ednet> (Riiid, Santa TOEIC tutor).
- Licence: **CC BY-NC 4.0**, research use. Raw and derived per-user data stay under
  `ml/datasets/raw/` (git-ignored). Committed files contain only aggregate item/population
  parameters and summary statistics (no user-level rows).
- Cite: Choi et al., "EdNet: A Large-Scale Hierarchical Dataset in Education", AIED 2020.

## Reproduce

From the repository root (Python 3.14 venv, numpy/pandas/scipy only; files under
`ml/datasets/raw/ednet/` must already exist):

```bash
# 1. one pass over the zip (12 workers, ~85 s, ~4 GB peak RSS)
.venv/bin/python -m ml.ednet.extract
# 2. evaluation users -> event CSV (~20 s) and the frozen detector pipeline on it
.venv/bin/python -m ml.ednet.convert --output ml/datasets/raw/ednet/derived/ednet_eval.csv
.venv/bin/python -m ml.anomaly.split --csv ml/datasets/raw/ednet/derived/ednet_eval.csv \
    --seed 42 --output ml/datasets/raw/ednet/derived/eval_run/split.json
.venv/bin/python -m ml.anomaly.train --csv ml/datasets/raw/ednet/derived/ednet_eval.csv \
    --manifest ml/datasets/raw/ednet/derived/eval_run/split.json \
    --output ml/datasets/raw/ednet/derived/eval_run
# 3. calibration users -> ml/ednet/calibration.json + calibration_report.md
.venv/bin/python -m ml.ednet.calibrate
# tests (synthetic fixture, no real data needed) and lint
.venv/bin/python -m unittest discover -s ml/tests
.venv/bin/ruff check ml
```

`--max-users N` on `extract` reads only the first N user files (quick runs). Everything is
deterministic: user splits and samples use SHA-256 of salted user ids, event ids are
deterministic UUIDv7, the calibration uses a fixed seed (`--seed`).

## Scripts

| file | role |
|---|---|
| `common.py` | paths, code tables, salted hash split (50 % calibration / 50 % evaluation users; calibration users split again 80 % fit / 20 % holdout), exclusion mask, change-event rule, pseudo-attempt windows |
| `extract.py` | streams `KT2/u*.csv` from the zip once, keeps single-question Part-5 bundles (all 5,703 Part-5 bundles are single-question - verified, none dropped), drops and counts malformed bundles, caches `derived/part5_responses.npz` (+ `.meta.json`) |
| `convert.py` | pseudo-attempts of evaluation users -> event CSV accepted by `ml.anomaly.features.read_attempts`; writes `<csv>.meta.json` |
| `models.py` | estimators: zero-inflated beta-binomial MLE, joint-MAP 3PL with pinned latent scale + parametric-bootstrap bias correction, lognormal RT model, noise-corrected SDs |
| `simulate.py` | minimal Python re-implementation of the honest learner model, used for the fit diagnostics (its docstring states the exact generative rules the parameters assume) |
| `calibrate.py` | all estimates, `calibration.json`, `calibration_report.md` |

### Key definitions

- **Exclusions.** Sources `review` (KT2's spelling of `in_review`; 67 % repeat exposures) and
  `my_note` (re-solving bookmarked questions) are dropped, and so is every *repeat exposure*
  (a question the learner already answered earlier, any source; this removes most of
  `adaptive_offer`/`review_quiz`, which are >90 % repeats). `diagnosis` (placement test,
  0.3 % repeats) is kept.
- **Change event.** The first `respond` is the initial answer; a later `respond` is a change
  only if its option differs from the immediately preceding respond (re-taps are ignored).
- **Pseudo-attempt.** 20 consecutive responses of the learner's usable Part-5 stream, every
  previous-submit -> next-enter gap in [0, 300 s], no bundle longer than 300 s (an interruption);
  non-overlapping, cut from the start of each sitting; at most 5 per user (earliest).
  Other-part bundles answered inside a gap are invisible time (as in the brief's definition).
- **Event mapping.** `time_since_shown_ms` = respond - bundle enter; `quiz.submitted` at the last
  bundle's submit; `score` = correct final answers; all rows `is_anomaly=0`, `anomaly_type=none`
  as placeholders (EdNet has no cheating labels).

## Converted evaluation file (default run)

`ml/datasets/raw/ednet/derived/ednet_eval.csv` (+ `.meta.json`): 10,671 evaluation users (all
eligible users; `--users 20000` is an upper bound that cannot be reached at a 300 s gap), 23,042
attempts, 629,850 events (145,968 change events), 534 cohorts of 20, no windows dropped by the
validator rules. `split` (16 s, 2.2 GB RSS) and `train` (23 s, 2.0 GB RSS) run unmodified; the
whole real file fits comfortably, so no size reduction was needed. Eligible users by max gap
(evaluation half): 120 s 8,026; 300 s 10,671; 600 s 12,186; 900 s 12,822.

## Calibration summary (authoritative numbers: `calibration_report.md`)

Fit users: 117k of the 147k calibration users (20 % held out for diagnostics). IRT/RT on
1.28 M non-rapid first answers, 1,180 fitted items, 10,814 persons; 1,089 items exported after the
pool-retention rule (bias-corrected a >= 0.25 and |b| <= 4, flags `--min-a`/`--max-abs-b`; drops
70 low-a and 52 |b| > 4 items, 91 in total, fixed before any detector result on calibrated
data; `item_prior` comes from the retained pool: b 0.03 +/- 1.36). Common c = 0.125 (profile; a
parametric replicate selects 0.10). tau_sd 0.33, theta-tau corr 0.21 (disattenuated). Rapid
guesses (NT10) are rare: 0.7 % of responses, accuracy 0.38. The zero-inflated fit puts
p_zero = 0, so disengagement is Beta(0.17, 32). P(change | first wrong) 0.24, P(change | right) 0.07,
wrong_to_right 0.59. The practice gain of 0.07 has a CI that includes 0 (600 window pairs only).

Posterior-predictive fit on holdout windows:
- Fits well: pooled first-answer times (KS 0.05), per-window p10 and median times,
  rapid share, the share of windows without changes.
- Visible misfit: simulated times are too dispersed within a window (variance KS 0.24) and
  slightly too slow in the upper tail. Accuracy is about 1 point lower and more dispersed in
  simulation, because random theta-item pairing ignores the adaptive matching. Simulated window
  durations are too narrow (KS 0.18; there is no person-level pace for gaps). The change-count
  tail is too light: continuation rises with every change (0.36, 0.48, 0.60) but the model uses
  one constant.
- Diagnostics use the exported pool; non-pool slots (57 % of slots are pool items) are drawn from
  `item_prior`. The calibration loads in `sim/bin/examory-sim` (generate + validate pass, both item
  sampling modes).

## Limitations

- Santa is a **self-study tutor**, not an exam: learners see an explanation after each item,
  can stop at any time, and items are chosen adaptively. Pseudo-attempts are reconstructed
  sittings, not real quizzes; the 20 items of a window are not a fixed variant, so peer
  (collusion) features see no shared quiz content beyond `quiz_id`.
- **No cheating labels.** The converted attempts are "unlabelled real attempts"; they can
  measure false-alarm behaviour of a detector on real behaviour, never its recall.
- Timestamps are shifted by the maintainers (relative use only); occasional logging
  disorder (responds after submit) makes some bundles unusable.
- **Part 5 only** (short grammar/vocabulary items, 4 options, no audio).
- Gaps include explanation reading and the per-item submit tap; there is no end-of-quiz
  review phase, so `review` is calibrated as the last item's submit lag (see the report).
- Only ~7 % of evaluation users ever produce a 20-item sitting, so the real attempts come from
  the more persistent learners.
