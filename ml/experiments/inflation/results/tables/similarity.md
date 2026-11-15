# Post hoc: classical baselines AnswerSim and response-time person-fit (exploratory)

**Added after the main results had been seen (amendment of 15 Nov 2026); exploratory.** For two attempts of different students in the same cohort and quiz sharing ≥ 10 items: z = (matches − Σ p_k) / √Σ p_k(1 − p_k), p_k = Σ_o f_ko² from the option frequencies of all train-cohort attempts (no labels); attempt score = max over partners. Secondary: the same sum over items where the scored attempt's final answer is wrong (key recovered from the submitted scores and verified on every attempt). Submission times are not used. RT person-fit (added after independent review, 15 Nov 2026; pre-knowledge baseline): ln t_ik = β_k − τ_i + ε_ik fitted on all train-cohort attempts by alternating means (no labels), σ_k = SD of item k's residuals; for any attempt τ_i = mean_k(β_k − ln t_ik), z_ik = (ln t_ik − (β_k − τ_i))/σ_k, score l_t = Σ_k z_ik² (Marianti et al., 2014); t_ik = `time_since_shown_ms` of the item's first answer event (first-answer latency; answer changes and follow-up events after the student moved to other items are not part of t_ik). Same protocol as the main stage: threshold = max validation recall s.t. validation FPR ≤ 5 %, metrics on test cohorts; mean over seeds [95 % CI]; recall and FPR in %.

## Item sharing (`--item-sampling` simulator default (fixed))

| World | min |S| (best partner) | median |S| | min eligible partners | median eligible partners | % attempts with a partner (min over seeds) | % without wrong-answer partner (incorrect variant) | answer key verified |
|---|---:|---:|---:|---:|---:|---:|---:|
| LEGACY | 20 | 20 | 57 | 57 | 100.0 | 2.75 | 20/20 |
| SIM-FI | 20 | 20 | 57 | 57 | 100.0 | 0.14 | 20/20 |
| SIM-BL | 20 | 20 | 57 | 57 | 100.0 | 0.42 | 20/20 |
| SIM-HONEST | 20 | 20 | 57 | 57 | 100.0 | 0.10 | 20/20 |

## Recall (%) at the validation-frozen 5 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| AnswerSim | all | 16.1 [12.4, 19.7] | 26.5 [24.3, 28.7] | 29.1 [24.8, 33.5] |
| AnswerSim | S1 rapid_response↔preknowledge | 3.3 [1.8, 5.1] | 2.9 [1.9, 4.0] | 30.2 [25.6, 34.7] |
| AnswerSim | S2 excessive_changes↔lookup | 4.2 [2.4, 6.1] | 3.1 [1.5, 5.1] | 22.1 [17.3, 28.1] |
| AnswerSim | S3 collusion↔copy_peer | 38.1 [28.7, 47.6] | 94.8 [91.4, 97.6] | 47.0 [39.8, 54.6] |
| AnswerSim | S4 score_jump↔proxy | 30.0 [21.9, 38.4] | 17.4 [12.8, 22.4] | 11.2 [8.1, 14.4] |
| AnswerSim | copy_peer, non-sync pairs | NA | NA | 46.7 [38.1, 55.6] |
| AnswerSim | copy_peer, sync pairs | NA | NA | 48.3 [38.8, 57.9] |
| AnswerSim | preknowledge, naive | NA | NA | 30.7 [25.9, 35.1] |
| AnswerSim | preknowledge, disguising | NA | NA | 29.9 [23.7, 36.0] |
| AnswerSim (incorrect only) | all | 23.0 [21.5, 24.7] | 27.4 [25.2, 29.5] | 17.5 [15.8, 19.1] |
| AnswerSim (incorrect only) | S1 rapid_response↔preknowledge | 4.2 [3.2, 5.1] | 4.6 [2.9, 6.6] | 6.9 [5.3, 8.6] |
| AnswerSim (incorrect only) | S2 excessive_changes↔lookup | 4.7 [3.1, 6.6] | 4.6 [3.4, 5.7] | 6.9 [4.7, 9.3] |
| AnswerSim (incorrect only) | S3 collusion↔copy_peer | 84.3 [80.4, 88.1] | 99.9 [99.6, 100.0] | 50.4 [46.6, 54.4] |
| AnswerSim (incorrect only) | S4 score_jump↔proxy | 2.1 [0.8, 3.7] | 10.0 [7.2, 13.1] | 10.1 [6.5, 14.0] |
| AnswerSim (incorrect only) | copy_peer, non-sync pairs | NA | NA | 47.9 [42.5, 53.7] |
| AnswerSim (incorrect only) | copy_peer, sync pairs | NA | NA | 52.6 [46.5, 58.5] |
| AnswerSim (incorrect only) | preknowledge, naive | NA | NA | 6.3 [4.3, 8.6] |
| AnswerSim (incorrect only) | preknowledge, disguising | NA | NA | 7.5 [5.0, 10.6] |
| RT person-fit l_t | all | 25.4 [23.6, 27.4] | 29.9 [26.2, 33.5] | 17.4 [15.2, 19.8] |
| RT person-fit l_t | S1 rapid_response↔preknowledge | 40.7 [37.7, 43.8] | 38.6 [33.4, 43.8] | 9.4 [6.8, 12.3] |
| RT person-fit l_t | S2 excessive_changes↔lookup | 37.1 [33.7, 40.4] | 55.6 [48.6, 62.1] | 15.8 [12.1, 19.8] |
| RT person-fit l_t | S3 collusion↔copy_peer | 3.9 [2.3, 5.6] | 6.2 [4.1, 8.5] | 44.0 [41.6, 46.4] |
| RT person-fit l_t | S4 score_jump↔proxy | 3.8 [2.3, 5.3] | 5.1 [3.2, 7.1] | 0.2 [0.0, 0.6] |
| RT person-fit l_t | copy_peer, non-sync pairs | NA | NA | 43.2 [39.7, 46.7] |
| RT person-fit l_t | copy_peer, sync pairs | NA | NA | 44.5 [41.9, 47.1] |
| RT person-fit l_t | preknowledge, naive | NA | NA | 16.9 [12.2, 22.2] |
| RT person-fit l_t | preknowledge, disguising | NA | NA | 2.0 [0.8, 3.4] |

## Standardized partial AUC, FPR ∈ [0, 0.05]

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| AnswerSim | all | 0.558 [0.548, 0.568] | 0.615 [0.605, 0.625] | 0.621 [0.608, 0.635] |
| AnswerSim | S1 rapid_response↔preknowledge | 0.502 [0.498, 0.506] | 0.502 [0.498, 0.507] | 0.625 [0.613, 0.637] |
| AnswerSim | S2 excessive_changes↔lookup | 0.507 [0.501, 0.514] | 0.501 [0.497, 0.506] | 0.593 [0.574, 0.614] |
| AnswerSim | S3 collusion↔copy_peer | 0.654 [0.622, 0.690] | 0.954 [0.935, 0.969] | 0.699 [0.673, 0.728] |
| AnswerSim | S4 score_jump↔proxy | 0.615 [0.591, 0.637] | 0.559 [0.545, 0.574] | 0.541 [0.530, 0.552] |
| AnswerSim | copy_peer, non-sync pairs | NA | NA | 0.701 [0.669, 0.733] |
| AnswerSim | copy_peer, sync pairs | NA | NA | 0.700 [0.661, 0.740] |
| AnswerSim | preknowledge, naive | NA | NA | 0.630 [0.613, 0.650] |
| AnswerSim | preknowledge, disguising | NA | NA | 0.621 [0.607, 0.636] |
| AnswerSim (incorrect only) | all | 0.587 [0.579, 0.596] | 0.614 [0.604, 0.623] | 0.543 [0.537, 0.550] |
| AnswerSim (incorrect only) | S1 rapid_response↔preknowledge | 0.501 [0.497, 0.505] | 0.498 [0.495, 0.501] | 0.503 [0.500, 0.507] |
| AnswerSim (incorrect only) | S2 excessive_changes↔lookup | 0.500 [0.495, 0.505] | 0.500 [0.496, 0.503] | 0.499 [0.495, 0.503] |
| AnswerSim (incorrect only) | S3 collusion↔copy_peer | 0.871 [0.851, 0.892] | 0.990 [0.984, 0.994] | 0.678 [0.658, 0.696] |
| AnswerSim (incorrect only) | S4 score_jump↔proxy | 0.493 [0.489, 0.499] | 0.512 [0.505, 0.519] | 0.507 [0.500, 0.514] |
| AnswerSim (incorrect only) | copy_peer, non-sync pairs | NA | NA | 0.663 [0.637, 0.690] |
| AnswerSim (incorrect only) | copy_peer, sync pairs | NA | NA | 0.693 [0.663, 0.721] |
| AnswerSim (incorrect only) | preknowledge, naive | NA | NA | 0.501 [0.496, 0.506] |
| AnswerSim (incorrect only) | preknowledge, disguising | NA | NA | 0.506 [0.501, 0.511] |
| RT person-fit l_t | all | 0.580 [0.574, 0.587] | 0.546 [0.538, 0.554] | 0.532 [0.528, 0.537] |
| RT person-fit l_t | S1 rapid_response↔preknowledge | 0.642 [0.629, 0.655] | 0.561 [0.549, 0.574] | 0.502 [0.499, 0.505] |
| RT person-fit l_t | S2 excessive_changes↔lookup | 0.620 [0.608, 0.630] | 0.598 [0.582, 0.615] | 0.523 [0.516, 0.531] |
| RT person-fit l_t | S3 collusion↔copy_peer | 0.497 [0.492, 0.503] | 0.499 [0.495, 0.503] | 0.623 [0.610, 0.635] |
| RT person-fit l_t | S4 score_jump↔proxy | 0.499 [0.494, 0.504] | 0.501 [0.495, 0.507] | 0.487 [0.487, 0.487] |
| RT person-fit l_t | copy_peer, non-sync pairs | NA | NA | 0.616 [0.601, 0.630] |
| RT person-fit l_t | copy_peer, sync pairs | NA | NA | 0.630 [0.615, 0.647] |
| RT person-fit l_t | preknowledge, naive | NA | NA | 0.513 [0.508, 0.518] |
| RT person-fit l_t | preknowledge, disguising | NA | NA | 0.491 [0.489, 0.493] |

## ROC-AUC

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| AnswerSim | all | 0.674 [0.664, 0.685] | 0.660 [0.651, 0.670] | 0.855 [0.845, 0.866] |
| AnswerSim | S1 rapid_response↔preknowledge | 0.536 [0.518, 0.552] | 0.507 [0.492, 0.522] | 0.873 [0.865, 0.882] |
| AnswerSim | S2 excessive_changes↔lookup | 0.546 [0.530, 0.564] | 0.522 [0.501, 0.543] | 0.845 [0.828, 0.861] |
| AnswerSim | S3 collusion↔copy_peer | 0.905 [0.889, 0.921] | 0.994 [0.991, 0.997] | 0.905 [0.889, 0.920] |
| AnswerSim | S4 score_jump↔proxy | 0.842 [0.822, 0.861] | 0.734 [0.709, 0.758] | 0.754 [0.731, 0.778] |
| AnswerSim | copy_peer, non-sync pairs | NA | NA | 0.908 [0.890, 0.926] |
| AnswerSim | copy_peer, sync pairs | NA | NA | 0.900 [0.869, 0.928] |
| AnswerSim | preknowledge, naive | NA | NA | 0.874 [0.861, 0.888] |
| AnswerSim | preknowledge, disguising | NA | NA | 0.874 [0.860, 0.887] |
| AnswerSim (incorrect only) | all | 0.608 [0.593, 0.624] | 0.637 [0.625, 0.648] | 0.669 [0.655, 0.681] |
| AnswerSim (incorrect only) | S1 rapid_response↔preknowledge | 0.507 [0.489, 0.526] | 0.498 [0.485, 0.511] | 0.619 [0.594, 0.642] |
| AnswerSim (incorrect only) | S2 excessive_changes↔lookup | 0.525 [0.506, 0.544] | 0.517 [0.497, 0.537] | 0.606 [0.590, 0.622] |
| AnswerSim (incorrect only) | S3 collusion↔copy_peer | 0.973 [0.967, 0.979] | 0.999 [0.998, 0.999] | 0.852 [0.834, 0.870] |
| AnswerSim (incorrect only) | S4 score_jump↔proxy | 0.422 [0.381, 0.463] | 0.609 [0.580, 0.634] | 0.611 [0.583, 0.638] |
| AnswerSim (incorrect only) | copy_peer, non-sync pairs | NA | NA | 0.861 [0.842, 0.881] |
| AnswerSim (incorrect only) | copy_peer, sync pairs | NA | NA | 0.844 [0.816, 0.872] |
| AnswerSim (incorrect only) | preknowledge, naive | NA | NA | 0.629 [0.596, 0.660] |
| AnswerSim (incorrect only) | preknowledge, disguising | NA | NA | 0.611 [0.578, 0.642] |
| RT person-fit l_t | all | 0.705 [0.693, 0.717] | 0.769 [0.759, 0.778] | 0.642 [0.634, 0.651] |
| RT person-fit l_t | S1 rapid_response↔preknowledge | 0.840 [0.829, 0.851] | 0.909 [0.902, 0.915] | 0.655 [0.640, 0.671] |
| RT person-fit l_t | S2 excessive_changes↔lookup | 0.831 [0.814, 0.845] | 0.941 [0.936, 0.946] | 0.662 [0.648, 0.676] |
| RT person-fit l_t | S3 collusion↔copy_peer | 0.500 [0.476, 0.523] | 0.550 [0.521, 0.578] | 0.726 [0.710, 0.743] |
| RT person-fit l_t | S4 score_jump↔proxy | 0.502 [0.477, 0.527] | 0.504 [0.486, 0.523] | 0.462 [0.444, 0.481] |
| RT person-fit l_t | copy_peer, non-sync pairs | NA | NA | 0.723 [0.697, 0.749] |
| RT person-fit l_t | copy_peer, sync pairs | NA | NA | 0.729 [0.710, 0.747] |
| RT person-fit l_t | preknowledge, naive | NA | NA | 0.847 [0.831, 0.862] |
| RT person-fit l_t | preknowledge, disguising | NA | NA | 0.474 [0.452, 0.498] |

## Operating point: validation and realized test FPR (%)

The score is discrete (with one shared item set the null terms are the same for every pair, so z is a monotone function of the match count); the largest threshold step with validation FPR ≤ 5 % can lie well below 5 %. SIM-HONEST: honest world (no cheaters), threshold from its own validation cohorts.

| Detector | World | validation FPR @5 % | test FPR @5 % | test FPR @1 % |
|---|---|---:|---:|---:|
| AnswerSim | LEGACY | 2.3 [1.7, 2.8] | 2.6 [1.8, 3.4] | 0.0 [0.0, 0.1] |
| AnswerSim | SIM-FI | 2.5 [1.9, 3.1] | 2.9 [2.2, 3.6] | 0.5 [0.4, 0.8] |
| AnswerSim | SIM-BL | 2.5 [2.0, 3.0] | 2.6 [2.1, 3.1] | 0.6 [0.3, 0.8] |
| AnswerSim | SIM-HONEST | 3.1 [2.4, 3.6] | 3.6 [2.7, 4.6] | 0.5 [0.3, 0.8] |
| AnswerSim (incorrect only) | LEGACY | 4.5 [4.3, 4.7] | 4.7 [4.2, 5.2] | 0.8 [0.6, 1.0] |
| AnswerSim (incorrect only) | SIM-FI | 4.5 [4.3, 4.7] | 4.8 [4.2, 5.3] | 0.7 [0.5, 0.9] |
| AnswerSim (incorrect only) | SIM-BL | 4.6 [4.4, 4.7] | 5.0 [4.5, 5.5] | 0.9 [0.7, 1.1] |
| AnswerSim (incorrect only) | SIM-HONEST | 5.0 [5.0, 5.0] | 5.3 [4.8, 5.8] | 1.0 [0.8, 1.2] |
| RT person-fit l_t | LEGACY | 4.8 [4.7, 4.9] | 4.5 [4.2, 4.9] | 0.9 [0.7, 1.1] |
| RT person-fit l_t | SIM-FI | 4.9 [4.9, 4.9] | 5.0 [4.4, 5.6] | 0.9 [0.6, 1.2] |
| RT person-fit l_t | SIM-BL | 4.8 [4.7, 4.9] | 4.9 [4.3, 5.5] | 1.0 [0.7, 1.3] |
| RT person-fit l_t | SIM-HONEST | 5.0 [5.0, 5.0] | 4.8 [4.3, 5.4] | 1.0 [0.8, 1.3] |

## SIM-BL: classical baselines next to the main-stage detectors (recall %, 5 % threshold)

Main-stage columns are taken from `main.json` (same seeds and data); descriptive, no test.

| Subset | AnswerSim | AnswerSim (incorrect only) | RT person-fit l_t | IF + peers (main) | IF (main) | Rules (main) |
|---|---:|---:|---:|---:|---:|---:|
| all cheating | 29.1 [24.8, 33.5] | 17.5 [15.8, 19.1] | 17.4 [15.2, 19.8] | 27.8 [25.8, 29.7] | 22.9 [21.0, 24.9] | 36.2 [33.0, 39.3] |
| S1 preknowledge (all) | 30.2 [25.6, 34.7] | 6.9 [5.3, 8.6] | 9.4 [6.8, 12.3] | 24.2 [22.1, 26.4] | 29.8 [27.4, 32.3] | 34.3 [29.9, 38.6] |
| preknowledge, naive | 30.7 [25.9, 35.1] | 6.3 [4.3, 8.6] | 16.9 [12.2, 22.2] | 38.6 [33.8, 43.3] | 51.7 [48.1, 55.4] | 41.0 [36.5, 45.8] |
| preknowledge, disguising | 29.9 [23.7, 36.0] | 7.5 [5.0, 10.6] | 2.0 [0.8, 3.4] | 10.8 [8.3, 13.6] | 9.3 [5.9, 12.9] | 28.4 [22.2, 35.0] |
| S3 copy_peer (all pairs) | 47.0 [39.8, 54.6] | 50.4 [46.6, 54.4] | 44.0 [41.6, 46.4] | 61.0 [56.5, 65.4] | 21.5 [18.7, 24.3] | 65.5 [60.1, 70.9] |
| copy_peer, non-sync pairs | 46.7 [38.1, 55.6] | 47.9 [42.5, 53.7] | 43.2 [39.7, 46.7] | 36.1 [29.5, 42.9] | 22.1 [19.4, 25.0] | 39.8 [32.1, 47.6] |
| copy_peer, sync pairs | 48.3 [38.8, 57.9] | 52.6 [46.5, 58.5] | 44.5 [41.9, 47.1] | 86.0 [82.5, 89.5] | 21.5 [16.3, 26.5] | 92.0 [87.6, 96.1] |
