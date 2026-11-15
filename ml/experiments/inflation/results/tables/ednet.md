# EdNet (real, unlabelled) — flag rates and realism

**Assumption.** EdNet has no cheating labels. Flag rate is interpreted as realized FPR under the assumption that cheating is negligible in an ungraded self-study tutor (EdNet/Santa). This assumption is not verifiable from the data.

**Peer features.** iforest and quantile_rules include peer features (synchrony/agreement within 5 s). EdNet learners do not sit a common timed quiz, so these features are not meaningful on EdNet; their rows are reported but flagged. quantile_rules_no_peers is a harness-constructed detector (frozen train.quantile_scores on BASE_FEATURES), not part of the frozen pipeline.

EdNet input: `ml/datasets/raw/ednet/derived/ednet_eval.csv` (SHA-256 `0c30a770eb3f2f4f…`); subsample: {'source': 'ml/datasets/raw/ednet/derived/ednet_eval.csv', 'source_sha256': '0c30a770eb3f2f4f1ac833a89958ab367a807a54650df1a5af0a3be2037213f9', 'source_attempts': 23042, 'source_cohorts': 534, 'max_attempts': 30000, 'subsample_seed': 20261114, 'subsampled': False, 'attempts': 23042, 'cohorts': 534}. Simulator params: `ml/ednet/calibration.json` (ednet_calibration).

## Flag rate on EdNet test cohorts (%)

Two different intervals: the across-seed interval is the 95 % percentile bootstrap of the mean flag rate over the 10 seeds (between-replicate variation); the cohort-bootstrap interval is the within-run 95 % interval (500 resamples of EdNet test cohorts) of seed 1001 only (sampling variation of one deployment).

| Fitted on | Detector | nominal FPR | flag rate: mean over seeds [across-seed 95 % CI] | flag rate: cohort-bootstrap 95 % CI (seed 1001) | flag |
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
| P1:SIM-defaults | iforest | 5 % | 27.4 [25.4, 29.2] | [25.6, 28.5] | peer features |
| P1:SIM-defaults | iforest | 1 % | 9.4 [8.0, 10.8] | [8.9, 11.2] | peer features |
| P1:SIM-defaults | iforest_no_peers | 5 % | 30.4 [28.5, 32.3] | [29.6, 32.6] |  |
| P1:SIM-defaults | iforest_no_peers | 1 % | 11.5 [9.7, 13.4] | [13.1, 15.5] |  |
| P1:SIM-defaults | quantile_rules | 5 % | 36.4 [34.7, 38.0] | [38.7, 42.3] | peer features |
| P1:SIM-defaults | quantile_rules | 1 % | 21.7 [19.7, 23.7] | [24.7, 27.8] | peer features |
| P1:SIM-defaults | quantile_rules_no_peers | 5 % | 36.8 [35.1, 38.4] | [39.4, 43.0] |  |
| P1:SIM-defaults | quantile_rules_no_peers | 1 % | 22.2 [20.7, 23.8] | [24.7, 27.8] |  |
| P1:SIM-calibrated | iforest | 5 % | 5.3 [4.7, 5.9] | [5.3, 7.0] | peer features |
| P1:SIM-calibrated | iforest | 1 % | 1.0 [0.7, 1.3] | [1.0, 1.8] | peer features |
| P1:SIM-calibrated | iforest_no_peers | 5 % | 11.4 [10.6, 12.2] | [12.5, 14.8] |  |
| P1:SIM-calibrated | iforest_no_peers | 1 % | 3.4 [2.9, 4.0] | [3.2, 4.5] |  |
| P1:SIM-calibrated | quantile_rules | 5 % | 21.0 [19.8, 22.5] | [24.8, 28.1] | peer features |
| P1:SIM-calibrated | quantile_rules | 1 % | 9.9 [7.5, 11.5] | [11.0, 13.1] | peer features |
| P1:SIM-calibrated | quantile_rules_no_peers | 5 % | 22.0 [20.7, 23.6] | [26.4, 29.5] |  |
| P1:SIM-calibrated | quantile_rules_no_peers | 1 % | 11.7 [10.5, 12.9] | [14.0, 16.3] |  |

## Who is flagged (5 % threshold): flag rate (%) by stratum

Strata use EdNet's own-reference features for every row (so P1 and P2 strata are the same attempts). Quartiles of score, median answer time and fast-answer fraction use cut points from EdNet training cohorts; a quartile can be empty or merged when the cut points tie.

| Fitted on | Detector | any_change:False | any_change:True | fast_answer_fraction:Q1 | fast_answer_fraction:Q2 | fast_answer_fraction:Q3 | fast_answer_fraction:Q4 | median_answer_time_ms:Q1 | median_answer_time_ms:Q2 | median_answer_time_ms:Q3 | median_answer_time_ms:Q4 | score_percent:Q1 | score_percent:Q2 | score_percent:Q3 | score_percent:Q4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| P2_refit | iforest_no_peers | 5.0 [4.5, 5.5] | 4.9 [4.6, 5.3] | 3.4 [3.1, 3.7] | 1.2 [1.0, 1.4] | 1.0 [0.9, 1.2] | 15.0 [14.0, 16.2] | 12.6 [11.8, 13.5] | 0.6 [0.5, 0.7] | 0.5 [0.4, 0.7] | 5.8 [5.3, 6.3] | 6.8 [6.3, 7.3] | 3.2 [2.9, 3.5] | 3.1 [2.8, 3.4] | 6.8 [6.2, 7.6] |
| P2_refit | quantile_rules_no_peers | 5.0 [4.2, 5.9] | 5.2 [4.9, 5.5] | 5.2 [4.8, 5.5] | 3.1 [2.8, 3.5] | 2.9 [2.5, 3.4] | 9.5 [8.2, 10.8] | 7.6 [6.5, 8.7] | 2.5 [2.2, 2.8] | 2.5 [2.2, 2.8] | 8.0 [7.4, 8.6] | 7.3 [6.6, 7.9] | 3.3 [2.9, 3.7] | 3.9 [3.5, 4.3] | 6.1 [5.1, 7.8] |
| P1:LEGACY | iforest_no_peers | 44.7 [40.6, 50.1] | 29.8 [28.1, 31.5] | 14.8 [12.7, 17.3] | 13.8 [12.3, 15.4] | 27.9 [25.3, 30.5] | 78.0 [76.2, 79.8] | 76.7 [74.2, 79.0] | 24.3 [21.6, 26.8] | 10.2 [8.6, 12.0] | 14.5 [12.5, 16.9] | 38.2 [35.8, 40.7] | 25.9 [23.8, 28.0] | 25.8 [24.0, 28.2] | 36.2 [33.9, 38.4] |
| P1:LEGACY | quantile_rules_no_peers | 28.6 [26.8, 30.7] | 40.5 [39.1, 41.9] | 30.9 [29.0, 32.9] | 28.7 [27.1, 30.2] | 33.0 [31.4, 34.8] | 68.0 [66.6, 69.4] | 69.7 [68.4, 71.4] | 30.4 [28.5, 32.1] | 19.9 [18.5, 21.5] | 35.7 [33.8, 37.7] | 41.1 [39.3, 42.8] | 37.1 [35.7, 38.6] | 37.5 [36.1, 39.0] | 40.9 [39.5, 42.0] |
| P1:SIM-defaults | iforest_no_peers | 9.1 [8.4, 10.0] | 33.3 [31.3, 35.2] | 12.1 [10.6, 13.5] | 17.3 [15.7, 19.0] | 29.4 [26.4, 32.1] | 72.1 [70.8, 73.4] | 70.4 [68.6, 72.4] | 25.5 [22.8, 28.3] | 12.4 [10.8, 14.4] | 13.0 [11.6, 14.4] | 32.8 [31.0, 34.5] | 27.0 [25.2, 28.9] | 27.7 [25.7, 29.8] | 35.9 [33.4, 38.6] |
| P1:SIM-defaults | quantile_rules_no_peers | 23.3 [21.8, 24.9] | 38.6 [36.8, 40.3] | 32.6 [30.3, 34.7] | 30.3 [28.7, 32.0] | 31.0 [29.0, 32.8] | 55.7 [54.0, 57.4] | 56.2 [54.1, 58.2] | 27.3 [25.6, 28.8] | 22.8 [21.0, 24.4] | 40.5 [37.7, 43.2] | 39.6 [37.7, 41.5] | 35.5 [33.6, 37.3] | 35.0 [33.0, 36.9] | 35.8 [34.5, 37.1] |
| P1:SIM-calibrated | iforest_no_peers | 13.6 [12.2, 15.2] | 11.1 [10.3, 11.9] | 6.9 [6.3, 7.6] | 3.5 [3.2, 3.9] | 4.6 [4.1, 5.2] | 32.9 [31.0, 35.0] | 30.1 [28.0, 32.3] | 2.7 [2.4, 3.1] | 1.6 [1.3, 2.0] | 10.8 [10.0, 11.5] | 13.1 [12.3, 14.0] | 8.4 [7.9, 9.0] | 8.9 [7.8, 10.0] | 17.0 [15.3, 18.8] |
| P1:SIM-calibrated | quantile_rules_no_peers | 19.0 [17.2, 20.9] | 22.4 [21.1, 24.0] | 16.8 [15.6, 18.2] | 14.5 [13.5, 15.6] | 18.7 [16.9, 20.5] | 40.8 [38.8, 42.9] | 41.2 [38.8, 43.4] | 14.7 [13.3, 16.2] | 9.3 [8.2, 10.6] | 22.6 [21.1, 24.3] | 23.2 [21.9, 24.7] | 20.1 [18.7, 21.7] | 20.7 [19.2, 22.5] | 25.0 [22.6, 27.2] |

## Realism: classifier two-sample test (real EdNet vs synthetic normal attempts)

AUC 0.5 = indistinguishable. HistGradientBoosting, 5-fold StratifiedGroupKFold by user. Primary: behavioural features only (mean_answer_time_ms, median_answer_time_ms, p10_answer_time_ms, answer_time_variance_ms2, fast_answer_fraction, final_answer_time_ms, duration_ms, changes_count, changes_fraction, score_percent); secondary: all BASE_FEATURES incl. history_missing and z_score, which reflect the attempts-per-user design. SIM sources are the honest world with --item-sampling per-attempt.

|  | LEGACY | SIM-defaults | SIM-calibrated |
|---|---:|---:|---:|
| C2ST AUC, behavioural features (primary): mean over seeds [across-seed 95 % CI] | 0.999 [0.999, 0.999] | 1.000 [1.000, 1.000] | 0.830 [0.828, 0.831] |
| C2ST AUC, behavioural features (primary): user-bootstrap 95 % CI (seed 1001) | [0.998, 1.000] | [1.000, 1.000] | [0.822, 0.843] |
| C2ST AUC, all base features (secondary): mean over seeds [across-seed 95 % CI] | 0.999 [0.999, 0.999] | 1.000 [1.000, 1.000] | 0.947 [0.946, 0.948] |
| C2ST AUC, all base features (secondary): user-bootstrap 95 % CI (seed 1001) | [0.998, 1.000] | [1.000, 1.000] | [0.944, 0.953] |
| n real / synthetic (per seed) | 3156/3155 | 3600/3600 | 3600/3600 |

## Per-feature distance to EdNet: KS statistic (mean over seeds [CI])

| Feature | LEGACY | SIM-defaults | SIM-calibrated |
|---|---:|---:|---:|
| mean_answer_time_ms | 0.48 [0.46, 0.49] | 0.41 [0.40, 0.42] | 0.12 [0.11, 0.13] |
| median_answer_time_ms | 0.37 [0.36, 0.38] | 0.33 [0.33, 0.34] | 0.07 [0.07, 0.08] |
| p10_answer_time_ms | 0.16 [0.15, 0.17] | 0.23 [0.22, 0.24] | 0.03 [0.03, 0.04] |
| answer_time_variance_ms2 | 0.59 [0.58, 0.60] | 0.50 [0.49, 0.51] | 0.22 [0.21, 0.23] |
| fast_answer_fraction | 0.14 [0.14, 0.15] | 0.06 [0.05, 0.07] | 0.05 [0.04, 0.06] |
| final_answer_time_ms | 0.23 [0.22, 0.24] | 0.15 [0.14, 0.15] | 0.03 [0.03, 0.03] |
| duration_ms | 0.28 [0.27, 0.29] | 0.46 [0.45, 0.46] | 0.19 [0.19, 0.20] |
| changes_count | 0.16 [0.15, 0.17] | 0.33 [0.32, 0.34] | 0.06 [0.06, 0.07] |
| changes_fraction | 0.16 [0.15, 0.17] | 0.33 [0.32, 0.34] | 0.06 [0.06, 0.07] |
| score_percent | 0.25 [0.20, 0.28] | 0.06 [0.06, 0.07] | 0.13 [0.12, 0.14] |
| z_score | 0.11 [0.10, 0.12] | 0.10 [0.09, 0.11] | 0.09 [0.09, 0.10] |
| history_missing | 0.08 [0.08, 0.09] | 0.13 [0.13, 0.13] | 0.13 [0.13, 0.13] |

## Per-feature distance to EdNet: Wasserstein distance / SD(EdNet) (mean over seeds [CI])

| Feature | LEGACY | SIM-defaults | SIM-calibrated |
|---|---:|---:|---:|
| mean_answer_time_ms | 1.35 [1.31, 1.38] | 1.12 [1.09, 1.16] | 0.26 [0.24, 0.27] |
| median_answer_time_ms | 0.93 [0.90, 0.96] | 0.85 [0.82, 0.88] | 0.15 [0.14, 0.16] |
| p10_answer_time_ms | 0.31 [0.29, 0.33] | 0.53 [0.51, 0.56] | 0.06 [0.05, 0.07] |
| answer_time_variance_ms2 | 1.97 [1.85, 2.10] | 1.36 [1.27, 1.45] | 0.30 [0.28, 0.32] |
| fast_answer_fraction | 0.23 [0.22, 0.23] | 0.12 [0.12, 0.13] | 0.09 [0.07, 0.10] |
| final_answer_time_ms | 0.80 [0.77, 0.84] | 0.48 [0.46, 0.50] | 0.09 [0.08, 0.10] |
| duration_ms | 0.63 [0.62, 0.65] | 0.88 [0.86, 0.89] | 0.36 [0.36, 0.37] |
| changes_count | 0.18 [0.16, 0.20] | 0.29 [0.26, 0.32] | 0.14 [0.12, 0.15] |
| changes_fraction | 0.31 [0.29, 0.32] | 0.68 [0.66, 0.69] | 0.15 [0.14, 0.15] |
| score_percent | 0.56 [0.46, 0.66] | 0.17 [0.16, 0.17] | 0.30 [0.28, 0.32] |
| z_score | 0.10 [0.09, 0.11] | 0.07 [0.06, 0.08] | 0.05 [0.04, 0.06] |
| history_missing | 0.17 [0.16, 0.18] | 0.26 [0.26, 0.27] | 0.26 [0.26, 0.27] |
