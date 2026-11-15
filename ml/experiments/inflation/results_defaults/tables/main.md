> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# Main experiment: detector performance by condition

Design {'students': 2000, 'attempts': 3, 'questions': 20, 'anomaly_rate': 0.12}; seeds 1001–1020 (n = 20); simulator params `sim/params/default.json` (sim_defaults).

## Recall (%) at the validation-frozen 5 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 48.6 [46.7, 50.5] | 55.6 [52.8, 58.1] | 22.1 [20.0, 24.2] |
| iforest | final | 48.6 [46.7, 50.5] | 55.6 [52.8, 58.1] | 22.1 [20.0, 24.2] |
| iforest | S1 rapid_response↔preknowledge | 38.6 [35.0, 42.1] | 34.9 [31.4, 38.1] | 18.6 [16.4, 20.8] |
| iforest | S2 excessive_changes↔lookup | 50.0 [46.6, 53.4] | 80.0 [75.1, 84.4] | 13.6 [10.3, 17.1] |
| iforest | S3 collusion↔copy_peer | 88.8 [84.9, 92.5] | 94.1 [90.2, 97.4] | 46.1 [40.2, 52.0] |
| iforest | S4 score_jump↔proxy | 6.6 [4.1, 9.1] | 6.9 [4.8, 9.3] | 8.7 [6.1, 11.4] |
| iforest_no_peers | all | 40.6 [38.2, 42.9] | 43.1 [41.3, 45.0] | 13.4 [11.9, 15.2] |
| iforest_no_peers | final | 40.6 [38.2, 42.9] | 43.1 [41.3, 45.0] | 13.4 [11.9, 15.2] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 57.9 [54.0, 61.8] | 46.4 [43.8, 49.0] | 19.4 [17.4, 21.7] |
| iforest_no_peers | S2 excessive_changes↔lookup | 74.1 [70.7, 77.4] | 95.5 [93.8, 97.3] | 19.1 [15.9, 22.7] |
| iforest_no_peers | S3 collusion↔copy_peer | 1.4 [0.5, 2.4] | 7.0 [4.0, 10.0] | 3.6 [2.0, 5.4] |
| iforest_no_peers | S4 score_jump↔proxy | 6.3 [4.4, 8.3] | 5.0 [3.4, 6.7] | 6.3 [3.9, 8.8] |
| quantile_rules | all | 68.6 [66.8, 70.4] | 61.2 [59.5, 62.9] | 32.6 [29.8, 35.5] |
| quantile_rules | final | 68.6 [66.8, 70.4] | 61.2 [59.5, 62.9] | 32.6 [29.8, 35.5] |
| quantile_rules | S1 rapid_response↔preknowledge | 49.7 [46.1, 53.2] | 29.5 [26.5, 32.7] | 30.2 [26.6, 33.5] |
| quantile_rules | S2 excessive_changes↔lookup | 99.0 [98.5, 99.6] | 99.8 [99.5, 100.0] | 25.6 [21.1, 30.3] |
| quantile_rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 54.8 [49.7, 59.6] |
| quantile_rules | S4 score_jump↔proxy | 14.0 [10.5, 17.6] | 14.0 [10.5, 17.6] | 18.0 [14.7, 21.6] |

## Realized test FPR (%) at the validation-frozen 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 5.1 [4.7, 5.5] | 5.2 [4.7, 5.7] | 5.2 [4.7, 5.8] |
| iforest | final | 4.4 [3.7, 5.1] | 4.3 [3.6, 5.0] | 4.7 [3.8, 5.6] |
| iforest_no_peers | all | 4.9 [4.5, 5.3] | 5.1 [4.6, 5.6] | 5.1 [4.5, 5.7] |
| iforest_no_peers | final | 3.7 [3.2, 4.2] | 4.3 [3.7, 4.9] | 4.4 [3.8, 5.0] |
| quantile_rules | all | 4.2 [3.9, 4.6] | 4.5 [4.0, 5.1] | 4.8 [4.3, 5.3] |
| quantile_rules | final | 4.3 [3.7, 4.8] | 4.4 [3.9, 5.0] | 4.6 [4.1, 5.2] |

## Recall (%) at the validation-frozen 1 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 21.6 [19.9, 23.2] | 28.9 [24.6, 33.2] | 8.1 [6.5, 10.0] |
| iforest | final | 21.6 [19.9, 23.2] | 28.9 [24.6, 33.2] | 8.1 [6.5, 10.0] |
| iforest | S1 rapid_response↔preknowledge | 17.2 [14.5, 20.0] | 15.5 [12.5, 18.5] | 7.6 [6.5, 8.8] |
| iforest | S2 excessive_changes↔lookup | 29.5 [26.3, 32.8] | 45.7 [39.6, 51.8] | 3.8 [1.9, 6.1] |
| iforest | S3 collusion↔copy_peer | 32.6 [28.4, 36.7] | 50.0 [40.6, 59.7] | 17.6 [13.0, 22.7] |
| iforest | S4 score_jump↔proxy | 1.5 [0.4, 2.9] | 1.4 [0.5, 2.4] | 2.1 [0.8, 3.7] |
| iforest_no_peers | all | 24.6 [22.2, 27.1] | 22.0 [19.5, 24.4] | 4.0 [3.3, 4.7] |
| iforest_no_peers | final | 24.6 [22.2, 27.1] | 22.0 [19.5, 24.4] | 4.0 [3.3, 4.7] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 35.8 [31.9, 39.6] | 20.6 [17.4, 23.8] | 7.7 [6.5, 8.8] |
| iforest_no_peers | S2 excessive_changes↔lookup | 45.7 [41.3, 50.2] | 56.1 [50.4, 62.0] | 3.5 [2.0, 5.0] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.2 [0.0, 0.5] | 1.2 [0.3, 2.3] | 1.1 [0.4, 1.8] |
| iforest_no_peers | S4 score_jump↔proxy | 2.2 [0.9, 3.5] | 0.6 [0.0, 1.3] | 1.9 [0.6, 3.5] |
| quantile_rules | all | 56.4 [49.4, 60.9] | 50.4 [44.3, 54.5] | 15.2 [12.6, 17.7] |
| quantile_rules | final | 56.4 [49.4, 60.9] | 50.4 [44.3, 54.5] | 15.2 [12.6, 17.7] |
| quantile_rules | S1 rapid_response↔preknowledge | 29.0 [24.1, 33.1] | 12.0 [8.9, 15.1] | 7.8 [5.8, 9.7] |
| quantile_rules | S2 excessive_changes↔lookup | 91.3 [80.9, 97.1] | 94.8 [84.7, 100.0] | 8.4 [5.7, 11.4] |
| quantile_rules | S3 collusion↔copy_peer | 95.0 [85.0, 100.0] | 95.0 [85.0, 100.0] | 42.0 [35.6, 47.9] |
| quantile_rules | S4 score_jump↔proxy | 3.1 [1.6, 4.8] | 2.1 [0.8, 3.6] | 4.0 [2.2, 6.0] |

## Realized test FPR (%) at the 1 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.9 [0.8, 1.0] | 1.1 [0.9, 1.5] | 1.2 [0.9, 1.5] |
| iforest | final | 0.7 [0.6, 0.9] | 1.0 [0.7, 1.3] | 1.0 [0.7, 1.3] |
| iforest_no_peers | all | 1.1 [0.8, 1.3] | 1.3 [1.0, 1.6] | 1.2 [0.9, 1.5] |
| iforest_no_peers | final | 0.7 [0.4, 1.0] | 1.1 [0.7, 1.5] | 1.1 [0.7, 1.5] |
| quantile_rules | all | 0.7 [0.5, 0.8] | 0.9 [0.7, 1.2] | 0.9 [0.6, 1.1] |
| quantile_rules | final | 0.7 [0.4, 0.9] | 0.9 [0.6, 1.3] | 0.9 [0.6, 1.3] |

## Precision (%) at the 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 56.5 [54.7, 58.4] | 59.2 [56.7, 61.8] | 36.4 [33.7, 39.0] |
| iforest | final | 86.1 [84.2, 88.0] | 87.6 [85.6, 89.6] | 72.3 [68.3, 76.3] |
| iforest | S1 rapid_response↔preknowledge | 26.5 [24.4, 28.6] | 24.4 [22.2, 26.5] | 14.8 [12.8, 16.9] |
| iforest | S2 excessive_changes↔lookup | 25.3 [23.2, 27.7] | 34.9 [32.1, 37.7] | 8.0 [6.4, 9.6] |
| iforest | S3 collusion↔copy_peer | 35.8 [33.0, 39.1] | 36.3 [32.7, 39.8] | 21.6 [18.5, 24.9] |
| iforest | S4 score_jump↔proxy | 2.7 [1.6, 3.7] | 2.8 [1.9, 3.8] | 3.2 [2.4, 4.0] |
| iforest_no_peers | all | 53.0 [50.8, 55.4] | 53.5 [50.9, 56.0] | 26.2 [23.9, 28.5] |
| iforest_no_peers | final | 85.8 [84.0, 87.6] | 84.7 [82.8, 86.3] | 62.5 [59.7, 65.3] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 36.2 [34.0, 38.5] | 30.6 [28.4, 32.9] | 15.5 [14.1, 17.0] |
| iforest_no_peers | S2 excessive_changes↔lookup | 34.5 [31.8, 37.5] | 39.6 [36.3, 42.7] | 11.4 [9.6, 13.2] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.9 [0.3, 1.6] | 3.5 [2.1, 5.0] | 2.0 [1.1, 3.0] |
| iforest_no_peers | S4 score_jump↔proxy | 2.7 [1.8, 3.5] | 2.0 [1.3, 2.7] | 2.4 [1.5, 3.3] |
| quantile_rules | all | 68.8 [66.7, 70.7] | 65.0 [62.0, 68.0] | 47.8 [45.1, 50.3] |
| quantile_rules | final | 89.9 [88.6, 91.2] | 88.3 [86.9, 89.8] | 79.1 [76.7, 81.4] |
| quantile_rules | S1 rapid_response↔preknowledge | 36.0 [33.4, 38.5] | 24.3 [21.8, 26.8] | 23.0 [20.6, 25.4] |
| quantile_rules | S2 excessive_changes↔lookup | 44.8 [42.2, 47.5] | 43.9 [40.0, 48.0] | 15.3 [13.0, 17.4] |
| quantile_rules | S3 collusion↔copy_peer | 43.2 [40.2, 46.4] | 41.3 [37.4, 45.4] | 26.2 [23.4, 29.1] |
| quantile_rules | S4 score_jump↔proxy | 6.5 [4.9, 8.0] | 6.2 [4.7, 7.6] | 7.6 [6.2, 9.2] |

## Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.658 [0.652, 0.665] | 0.690 [0.676, 0.704] | 0.553 [0.546, 0.561] |
| iforest | final | 0.673 [0.659, 0.686] | 0.708 [0.688, 0.727] | 0.562 [0.551, 0.573] |
| iforest | S1 rapid_response↔preknowledge | 0.623 [0.610, 0.636] | 0.605 [0.593, 0.617] | 0.546 [0.538, 0.555] |
| iforest | S2 excessive_changes↔lookup | 0.684 [0.669, 0.699] | 0.789 [0.765, 0.813] | 0.522 [0.513, 0.532] |
| iforest | S3 collusion↔copy_peer | 0.781 [0.757, 0.805] | 0.837 [0.812, 0.864] | 0.628 [0.607, 0.651] |
| iforest | S4 score_jump↔proxy | 0.505 [0.497, 0.515] | 0.505 [0.499, 0.512] | 0.509 [0.502, 0.517] |
| iforest_no_peers | all | 0.648 [0.638, 0.657] | 0.643 [0.634, 0.652] | 0.525 [0.521, 0.529] |
| iforest_no_peers | final | 0.664 [0.653, 0.677] | 0.653 [0.641, 0.665] | 0.531 [0.525, 0.536] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.719 [0.703, 0.733] | 0.644 [0.631, 0.656] | 0.549 [0.541, 0.556] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.784 [0.770, 0.797] | 0.860 [0.847, 0.872] | 0.535 [0.526, 0.543] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.490 [0.488, 0.492] | 0.504 [0.496, 0.512] | 0.496 [0.491, 0.501] |
| iforest_no_peers | S4 score_jump↔proxy | 0.506 [0.500, 0.513] | 0.499 [0.494, 0.504] | 0.503 [0.497, 0.510] |
| quantile_rules | all | 0.805 [0.796, 0.814] | 0.771 [0.762, 0.779] | 0.608 [0.597, 0.619] |
| quantile_rules | final | 0.807 [0.796, 0.817] | 0.771 [0.761, 0.780] | 0.611 [0.596, 0.626] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.691 [0.675, 0.707] | 0.592 [0.581, 0.604] | 0.585 [0.574, 0.595] |
| quantile_rules | S2 excessive_changes↔lookup | 0.971 [0.961, 0.979] | 0.980 [0.974, 0.985] | 0.569 [0.555, 0.583] |
| quantile_rules | S3 collusion↔copy_peer | 0.980 [0.972, 0.986] | 0.981 [0.975, 0.986] | 0.732 [0.707, 0.757] |
| quantile_rules | S4 score_jump↔proxy | 0.531 [0.521, 0.542] | 0.531 [0.520, 0.542] | 0.543 [0.528, 0.559] |

## ROC-AUC

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.838 [0.829, 0.845] | 0.844 [0.835, 0.852] | 0.677 [0.661, 0.691] |
| iforest | final | 0.858 [0.848, 0.867] | 0.861 [0.852, 0.870] | 0.708 [0.694, 0.721] |
| iforest | S1 rapid_response↔preknowledge | 0.809 [0.792, 0.825] | 0.800 [0.788, 0.813] | 0.675 [0.653, 0.695] |
| iforest | S2 excessive_changes↔lookup | 0.909 [0.901, 0.918] | 0.971 [0.967, 0.975] | 0.690 [0.668, 0.713] |
| iforest | S3 collusion↔copy_peer | 0.977 [0.973, 0.980] | 0.983 [0.981, 0.986] | 0.750 [0.716, 0.781] |
| iforest | S4 score_jump↔proxy | 0.573 [0.544, 0.603] | 0.541 [0.518, 0.566] | 0.555 [0.531, 0.580] |
| iforest_no_peers | all | 0.744 [0.734, 0.754] | 0.790 [0.777, 0.803] | 0.644 [0.630, 0.657] |
| iforest_no_peers | final | 0.780 [0.770, 0.789] | 0.816 [0.805, 0.828] | 0.687 [0.674, 0.699] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.867 [0.853, 0.880] | 0.855 [0.847, 0.862] | 0.710 [0.690, 0.727] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.958 [0.953, 0.962] | 0.986 [0.984, 0.987] | 0.743 [0.718, 0.766] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.430 [0.398, 0.461] | 0.618 [0.582, 0.658] | 0.490 [0.461, 0.518] |
| iforest_no_peers | S4 score_jump↔proxy | 0.593 [0.568, 0.618] | 0.581 [0.558, 0.606] | 0.562 [0.533, 0.588] |
| quantile_rules | all | 0.909 [0.902, 0.915] | 0.876 [0.869, 0.882] | 0.765 [0.755, 0.775] |
| quantile_rules | final | 0.907 [0.900, 0.914] | 0.873 [0.867, 0.879] | 0.759 [0.748, 0.769] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.851 [0.837, 0.864] | 0.796 [0.786, 0.806] | 0.790 [0.775, 0.805] |
| quantile_rules | S2 excessive_changes↔lookup | 0.997 [0.996, 0.998] | 0.998 [0.997, 0.998] | 0.769 [0.748, 0.788] |
| quantile_rules | S3 collusion↔copy_peer | 0.998 [0.997, 0.999] | 0.998 [0.998, 0.999] | 0.786 [0.753, 0.816] |
| quantile_rules | S4 score_jump↔proxy | 0.761 [0.738, 0.782] | 0.683 [0.662, 0.704] | 0.669 [0.642, 0.695] |

## Average precision

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.527 [0.513, 0.541] | 0.580 [0.556, 0.602] | 0.263 [0.243, 0.283] |
| iforest | final | 0.792 [0.778, 0.807] | 0.814 [0.798, 0.828] | 0.583 [0.561, 0.604] |
| iforest | S1 rapid_response↔preknowledge | 0.270 [0.245, 0.294] | 0.245 [0.218, 0.270] | 0.126 [0.110, 0.143] |
| iforest | S2 excessive_changes↔lookup | 0.375 [0.342, 0.411] | 0.581 [0.533, 0.628] | 0.077 [0.066, 0.091] |
| iforest | S3 collusion↔copy_peer | 0.488 [0.442, 0.538] | 0.592 [0.530, 0.656] | 0.207 [0.168, 0.249] |
| iforest | S4 score_jump↔proxy | 0.034 [0.027, 0.042] | 0.028 [0.025, 0.032] | 0.033 [0.029, 0.038] |
| iforest_no_peers | all | 0.472 [0.455, 0.489] | 0.476 [0.457, 0.495] | 0.204 [0.192, 0.216] |
| iforest_no_peers | final | 0.735 [0.724, 0.746] | 0.748 [0.734, 0.761] | 0.531 [0.514, 0.547] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.476 [0.441, 0.509] | 0.326 [0.300, 0.351] | 0.136 [0.121, 0.152] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.581 [0.549, 0.613] | 0.682 [0.641, 0.716] | 0.096 [0.083, 0.109] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.028 [0.024, 0.032] | 0.049 [0.040, 0.059] | 0.033 [0.028, 0.037] |
| iforest_no_peers | S4 score_jump↔proxy | 0.038 [0.031, 0.046] | 0.029 [0.026, 0.031] | 0.031 [0.027, 0.034] |
| quantile_rules | all | 0.754 [0.739, 0.769] | 0.691 [0.674, 0.706] | 0.400 [0.376, 0.422] |
| quantile_rules | final | 0.883 [0.874, 0.892] | 0.843 [0.834, 0.852] | 0.671 [0.651, 0.690] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.405 [0.368, 0.440] | 0.211 [0.188, 0.234] | 0.193 [0.178, 0.209] |
| quantile_rules | S2 excessive_changes↔lookup | 0.883 [0.845, 0.917] | 0.901 [0.873, 0.924] | 0.135 [0.114, 0.159] |
| quantile_rules | S3 collusion↔copy_peer | 0.897 [0.867, 0.925] | 0.892 [0.863, 0.918] | 0.413 [0.355, 0.472] |
| quantile_rules | S4 score_jump↔proxy | 0.071 [0.062, 0.079] | 0.055 [0.046, 0.064] | 0.065 [0.052, 0.080] |

Cells: mean over seeds [95 % percentile-bootstrap CI over seeds]. Slot rows compare that slot's cheaters with all normal test attempts (LEGACY/SIM-FI type ↔ SIM-BL mechanism).

## Median width (pp) of the within-run cohort-bootstrap 95 % CI of recall@5 %

What a single-dataset study would report as its uncertainty.

| Detector | Slot | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | S1 | 27.0 | 25.9 | 21.0 |
| iforest | S2 | 30.2 | 25.5 | 19.4 |
| iforest | S3 | 19.5 | 8.7 | 42.9 |
| iforest | S4 | 19.4 | 15.7 | 18.7 |
| iforest_no_peers | S1 | 26.7 | 26.2 | 22.1 |
| iforest_no_peers | S2 | 29.7 | 11.4 | 22.3 |
| iforest_no_peers | S3 | 0.0 | 18.3 | 10.5 |
| iforest_no_peers | S4 | 16.5 | 14.1 | 16.0 |
| quantile_rules | S1 | 28.1 | 25.8 | 25.1 |
| quantile_rules | S2 | 0.0 | 0.0 | 25.9 |
| quantile_rules | S3 | 0.0 | 0.0 | 47.8 |
| quantile_rules | S4 | 27.2 | 26.3 | 29.0 |
