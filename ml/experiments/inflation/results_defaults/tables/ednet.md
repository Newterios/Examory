> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# EdNet (real, unlabelled) — flag rates and realism

**Assumption.** EdNet has no cheating labels. Flag rate is interpreted as realized FPR under the assumption that cheating is negligible in an ungraded self-study tutor (EdNet/Santa). This assumption is not verifiable from the data.

**Peer features.** iforest and quantile_rules include peer features (synchrony/agreement within 5 s). EdNet learners do not sit a common timed quiz, so these features are not meaningful on EdNet; their rows are reported but flagged. quantile_rules_no_peers is a harness-constructed detector (frozen train.quantile_scores on BASE_FEATURES), not part of the frozen pipeline.

EdNet input: `ml/datasets/raw/ednet/derived/ednet_eval.csv` (SHA-256 `0c30a770eb3f2f4f…`); subsample: {'source': 'ml/datasets/raw/ednet/derived/ednet_eval.csv', 'source_sha256': '0c30a770eb3f2f4f1ac833a89958ab367a807a54650df1a5af0a3be2037213f9', 'source_attempts': 23042, 'source_cohorts': 534, 'max_attempts': 30000, 'subsample_seed': 20261114, 'subsampled': False, 'attempts': 23042, 'cohorts': 534}. Simulator params: `sim/params/default.json` (sim_defaults).

## Flag rate on EdNet test cohorts (%)

| Fitted on | Detector | nominal FPR | flag rate, mean over seeds [CI] | cohort-bootstrap CI (first seed) | flag |
|---|---|---:|---:|---:|---|
| P2_refit | iforest | 5 % | 5.0 [4.7, 5.3] | [4.1, 5.7] | peer features |
| P2_refit | iforest | 1 % | 1.0 [0.8, 1.1] | [0.9, 1.8] | peer features |
| P2_refit | iforest_no_peers | 5 % | 4.9 [4.6, 5.2] | [4.2, 5.8] |  |
| P2_refit | iforest_no_peers | 1 % | 1.0 [0.9, 1.2] | [0.9, 1.7] |  |
| P2_refit | quantile_rules | 5 % | 5.2 [4.8, 5.5] | [4.6, 6.1] | peer features |
| P2_refit | quantile_rules | 1 % | 0.9 [0.8, 1.0] | [0.5, 1.1] | peer features |
| P2_refit | quantile_rules_no_peers | 5 % | 5.2 [4.8, 5.5] | [4.6, 6.1] |  |
| P2_refit | quantile_rules_no_peers | 1 % | 0.9 [0.8, 1.0] | [0.5, 1.1] |  |
| P1:LEGACY | iforest | 5 % | 12.9 [11.5, 14.2] | [8.1, 10.1] | peer features |
| P1:LEGACY | iforest | 1 % | 4.2 [3.7, 4.8] | [2.0, 3.2] | peer features |
| P1:LEGACY | iforest_no_peers | 5 % | 31.5 [29.7, 33.6] | [31.5, 35.0] |  |
| P1:LEGACY | iforest_no_peers | 1 % | 12.7 [10.9, 14.6] | [12.0, 14.4] |  |
| P1:LEGACY | quantile_rules | 5 % | 35.7 [34.3, 37.2] | [33.6, 37.0] | peer features |
| P1:LEGACY | quantile_rules | 1 % | 17.8 [13.3, 21.0] | [18.3, 20.9] | peer features |
| P1:LEGACY | quantile_rules_no_peers | 5 % | 39.1 [37.6, 40.6] | [33.6, 37.0] |  |
| P1:LEGACY | quantile_rules_no_peers | 1 % | 21.9 [20.0, 23.6] | [18.3, 20.9] |  |
| P1:SIM-HONEST | iforest | 5 % | 27.4 [25.4, 29.2] | [25.6, 28.5] | peer features |
| P1:SIM-HONEST | iforest | 1 % | 9.4 [8.0, 10.8] | [8.9, 11.2] | peer features |
| P1:SIM-HONEST | iforest_no_peers | 5 % | 30.4 [28.5, 32.3] | [29.6, 32.6] |  |
| P1:SIM-HONEST | iforest_no_peers | 1 % | 11.5 [9.7, 13.4] | [13.1, 15.5] |  |
| P1:SIM-HONEST | quantile_rules | 5 % | 36.4 [34.7, 38.0] | [38.7, 42.3] | peer features |
| P1:SIM-HONEST | quantile_rules | 1 % | 21.7 [19.7, 23.7] | [24.7, 27.8] | peer features |
| P1:SIM-HONEST | quantile_rules_no_peers | 5 % | 36.8 [35.1, 38.4] | [39.4, 43.0] |  |
| P1:SIM-HONEST | quantile_rules_no_peers | 1 % | 22.2 [20.7, 23.8] | [24.7, 27.8] |  |

## Who is flagged (5 % threshold): flag rate (%) by stratum

Quartiles of score and median answer time use cut points from EdNet training cohorts.

| Fitted on | Detector | any_change:False | any_change:True | median_answer_time_ms:Q1 | median_answer_time_ms:Q2 | median_answer_time_ms:Q3 | median_answer_time_ms:Q4 | score_percent:Q1 | score_percent:Q2 | score_percent:Q3 | score_percent:Q4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| P2_refit | iforest_no_peers | 5.0 [4.5, 5.5] | 4.9 [4.6, 5.3] | 12.6 [11.8, 13.5] | 0.6 [0.5, 0.7] | 0.5 [0.4, 0.7] | 5.8 [5.3, 6.3] | 8.4 [7.8, 8.9] | 3.6 [3.1, 4.0] | 2.9 [2.6, 3.1] | 5.4 [4.9, 5.9] |
| P2_refit | quantile_rules_no_peers | 5.0 [4.2, 5.9] | 5.2 [4.9, 5.5] | 7.6 [6.5, 8.7] | 2.5 [2.2, 2.8] | 2.5 [2.2, 2.8] | 8.0 [7.4, 8.6] | 8.4 [7.6, 9.2] | 3.9 [3.6, 4.3] | 3.6 [3.3, 4.0] | 5.2 [4.6, 6.0] |
| P1:LEGACY | iforest_no_peers | 44.7 [40.6, 50.1] | 29.8 [28.1, 31.5] | 76.7 [74.2, 79.0] | 24.3 [21.6, 26.8] | 10.1 [8.6, 12.0] | 14.5 [12.5, 16.9] | 42.4 [39.7, 45.1] | 28.3 [26.2, 30.5] | 24.9 [23.1, 26.8] | 32.3 [30.2, 34.6] |
| P1:LEGACY | quantile_rules_no_peers | 28.6 [26.8, 30.7] | 40.5 [39.1, 41.9] | 69.7 [68.4, 71.4] | 30.4 [28.6, 32.1] | 19.9 [18.5, 21.5] | 35.7 [33.8, 37.7] | 41.7 [39.5, 44.0] | 38.5 [37.2, 39.9] | 36.7 [35.4, 38.2] | 39.8 [38.5, 41.0] |
| P1:SIM-HONEST | iforest_no_peers | 9.1 [8.4, 10.0] | 33.3 [31.3, 35.2] | 70.4 [68.6, 72.4] | 25.5 [22.8, 28.3] | 12.4 [10.8, 14.4] | 13.0 [11.6, 14.4] | 34.9 [33.0, 36.7] | 28.2 [26.1, 30.2] | 26.6 [24.7, 28.4] | 32.8 [30.7, 35.0] |
| P1:SIM-HONEST | quantile_rules_no_peers | 23.3 [21.8, 24.9] | 38.6 [36.8, 40.3] | 56.2 [54.1, 58.2] | 27.3 [25.6, 28.8] | 22.8 [21.0, 24.4] | 40.5 [37.7, 43.2] | 40.3 [38.3, 42.1] | 37.2 [35.2, 38.9] | 34.7 [32.8, 36.6] | 35.7 [34.2, 37.3] |

## Realism: classifier two-sample test (real EdNet vs synthetic normal attempts)

AUC 0.5 = indistinguishable. HistGradientBoosting on BASE_FEATURES, 5-fold CV grouped by user.

| Synthetic source | C2ST AUC mean over seeds [CI] | user-bootstrap CI (first seed) | n real/synthetic |
|---|---:|---:|---:|
| LEGACY | 0.999 [0.999, 0.999] | [0.998, 1.000] | 3156/3155 |
| SIM-HONEST | 1.000 [1.000, 1.000] | [1.000, 1.000] | 3600/3600 |

## Per-feature distance to EdNet (KS statistic; Wasserstein / SD of EdNet)

| Feature | LEGACY | SIM-HONEST |
|---|---|---|
| answer_time_variance_ms2 | KS 0.59 [0.58, 0.60]; W/sd 1.97 [1.85, 2.10] | KS 0.50 [0.49, 0.51]; W/sd 1.36 [1.27, 1.45] |
| changes_count | KS 0.16 [0.15, 0.17]; W/sd 0.18 [0.16, 0.20] | KS 0.33 [0.32, 0.34]; W/sd 0.29 [0.26, 0.32] |
| changes_fraction | KS 0.16 [0.15, 0.17]; W/sd 0.31 [0.29, 0.32] | KS 0.33 [0.32, 0.34]; W/sd 0.68 [0.66, 0.69] |
| duration_ms | KS 0.28 [0.27, 0.29]; W/sd 0.63 [0.62, 0.65] | KS 0.46 [0.45, 0.46]; W/sd 0.88 [0.86, 0.89] |
| fast_answer_fraction | KS 0.14 [0.14, 0.15]; W/sd 0.23 [0.22, 0.23] | KS 0.06 [0.05, 0.07]; W/sd 0.12 [0.12, 0.13] |
| final_answer_time_ms | KS 0.23 [0.22, 0.24]; W/sd 0.80 [0.77, 0.84] | KS 0.15 [0.14, 0.15]; W/sd 0.48 [0.46, 0.50] |
| history_missing | KS 0.08 [0.08, 0.09]; W/sd 0.17 [0.16, 0.18] | KS 0.13 [0.13, 0.13]; W/sd 0.26 [0.26, 0.27] |
| mean_answer_time_ms | KS 0.48 [0.46, 0.49]; W/sd 1.35 [1.31, 1.38] | KS 0.41 [0.40, 0.42]; W/sd 1.12 [1.09, 1.16] |
| median_answer_time_ms | KS 0.37 [0.36, 0.38]; W/sd 0.93 [0.90, 0.96] | KS 0.33 [0.33, 0.34]; W/sd 0.85 [0.82, 0.88] |
| p10_answer_time_ms | KS 0.16 [0.15, 0.17]; W/sd 0.31 [0.29, 0.33] | KS 0.23 [0.22, 0.24]; W/sd 0.53 [0.51, 0.56] |
| score_percent | KS 0.25 [0.20, 0.28]; W/sd 0.56 [0.46, 0.66] | KS 0.06 [0.06, 0.07]; W/sd 0.17 [0.16, 0.17] |
| z_score | KS 0.11 [0.10, 0.12]; W/sd 0.10 [0.09, 0.11] | KS 0.10 [0.09, 0.11]; W/sd 0.07 [0.06, 0.08] |
