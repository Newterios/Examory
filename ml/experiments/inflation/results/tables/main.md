# Main experiment: detector performance by condition

Design {'students': 2000, 'attempts': 3, 'questions': 20, 'anomaly_rate': 0.12}; seeds 1001–1020 (n = 20); simulator params `ml/ednet/calibration.json` (ednet_calibration).

## Recall (%) at the validation-frozen 5 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 48.6 [46.7, 50.5] | 53.6 [51.9, 55.3] | 27.8 [25.8, 29.7] |
| iforest | final | 48.6 [46.7, 50.5] | 53.6 [51.9, 55.3] | 27.8 [25.8, 29.7] |
| iforest | S1 rapid_response↔preknowledge | 38.6 [35.0, 42.1] | 35.4 [32.5, 38.4] | 24.2 [22.1, 26.4] |
| iforest | S2 excessive_changes↔lookup | 50.0 [46.6, 53.4] | 68.0 [63.5, 72.4] | 14.4 [11.7, 17.2] |
| iforest | S3 collusion↔copy_peer | 88.8 [84.9, 92.5] | 100.0 [100.0, 100.0] | 61.0 [56.5, 65.4] |
| iforest | S4 score_jump↔proxy | 6.6 [4.1, 9.1] | 5.1 [3.1, 7.2] | 9.5 [7.8, 11.3] |
| iforest_no_peers | all | 40.6 [38.2, 42.9] | 46.3 [43.9, 48.5] | 22.9 [21.0, 24.9] |
| iforest_no_peers | final | 40.6 [38.2, 42.9] | 46.3 [43.9, 48.5] | 22.9 [21.0, 24.9] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 57.9 [54.0, 61.8] | 55.6 [52.5, 58.5] | 29.8 [27.4, 32.3] |
| iforest_no_peers | S2 excessive_changes↔lookup | 74.1 [70.7, 77.4] | 91.1 [88.9, 93.2] | 23.2 [20.7, 26.0] |
| iforest_no_peers | S3 collusion↔copy_peer | 1.4 [0.5, 2.4] | 11.0 [7.7, 14.5] | 21.5 [18.7, 24.3] |
| iforest_no_peers | S4 score_jump↔proxy | 6.3 [4.4, 8.3] | 5.6 [3.6, 8.1] | 8.3 [5.8, 11.1] |
| quantile_rules | all | 68.6 [66.8, 70.4] | 61.9 [59.8, 64.0] | 36.2 [33.0, 39.3] |
| quantile_rules | final | 68.6 [66.8, 70.4] | 61.9 [59.8, 64.0] | 36.2 [33.0, 39.3] |
| quantile_rules | S1 rapid_response↔preknowledge | 49.7 [46.1, 53.2] | 34.1 [29.9, 38.2] | 34.3 [29.9, 38.6] |
| quantile_rules | S2 excessive_changes↔lookup | 99.0 [98.5, 99.6] | 97.8 [96.7, 98.8] | 28.0 [23.1, 33.0] |
| quantile_rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 65.5 [60.1, 70.9] |
| quantile_rules | S4 score_jump↔proxy | 14.0 [10.5, 17.6] | 12.3 [8.2, 17.4] | 10.7 [7.3, 14.2] |

## Realized test FPR (%) at the validation-frozen 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 5.1 [4.7, 5.5] | 5.1 [4.6, 5.7] | 5.0 [4.4, 5.6] |
| iforest | final | 4.4 [3.7, 5.1] | 4.8 [4.1, 5.5] | 4.6 [3.9, 5.2] |
| iforest_no_peers | all | 4.9 [4.5, 5.3] | 5.3 [4.8, 6.0] | 5.3 [4.7, 6.0] |
| iforest_no_peers | final | 3.7 [3.2, 4.2] | 4.3 [3.7, 4.9] | 4.4 [3.8, 5.0] |
| quantile_rules | all | 4.2 [3.9, 4.6] | 5.3 [4.7, 6.0] | 5.2 [4.5, 5.8] |
| quantile_rules | final | 4.3 [3.7, 4.8] | 5.9 [5.3, 6.6] | 5.9 [5.2, 6.7] |

## Recall (%) at the validation-frozen 1 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 21.6 [19.9, 23.2] | 32.9 [29.5, 36.3] | 14.2 [12.3, 16.0] |
| iforest | final | 21.6 [19.9, 23.2] | 32.9 [29.5, 36.3] | 14.2 [12.3, 16.0] |
| iforest | S1 rapid_response↔preknowledge | 17.2 [14.5, 20.0] | 15.7 [12.9, 18.6] | 10.7 [8.5, 13.0] |
| iforest | S2 excessive_changes↔lookup | 29.5 [26.3, 32.8] | 39.8 [34.3, 45.4] | 4.0 [2.4, 5.6] |
| iforest | S3 collusion↔copy_peer | 32.6 [28.4, 36.7] | 74.6 [66.4, 82.1] | 38.2 [33.1, 43.3] |
| iforest | S4 score_jump↔proxy | 1.5 [0.4, 2.9] | 1.1 [0.2, 2.1] | 3.0 [1.8, 4.4] |
| iforest_no_peers | all | 24.6 [22.2, 27.1] | 24.4 [21.7, 26.9] | 7.3 [6.1, 8.5] |
| iforest_no_peers | final | 24.6 [22.2, 27.1] | 24.4 [21.7, 26.9] | 7.3 [6.1, 8.5] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 35.8 [31.9, 39.6] | 27.2 [23.5, 30.8] | 13.2 [10.7, 15.9] |
| iforest_no_peers | S2 excessive_changes↔lookup | 45.7 [41.3, 50.2] | 54.3 [49.1, 59.2] | 5.9 [4.4, 7.5] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.2 [0.0, 0.5] | 2.7 [1.3, 4.3] | 3.8 [2.3, 5.4] |
| iforest_no_peers | S4 score_jump↔proxy | 2.2 [0.9, 3.5] | 1.9 [0.9, 2.9] | 1.9 [0.8, 3.2] |
| quantile_rules | all | 56.4 [49.4, 60.9] | 48.2 [42.2, 52.3] | 18.1 [15.2, 20.6] |
| quantile_rules | final | 56.4 [49.4, 60.9] | 48.2 [42.2, 52.3] | 18.1 [15.2, 20.6] |
| quantile_rules | S1 rapid_response↔preknowledge | 29.0 [24.1, 33.1] | 13.8 [10.8, 16.9] | 10.3 [7.6, 13.1] |
| quantile_rules | S2 excessive_changes↔lookup | 91.3 [80.9, 97.1] | 83.9 [73.6, 90.4] | 7.8 [5.8, 9.8] |
| quantile_rules | S3 collusion↔copy_peer | 95.0 [85.0, 100.0] | 95.0 [85.0, 100.0] | 52.5 [44.7, 59.2] |
| quantile_rules | S4 score_jump↔proxy | 3.1 [1.7, 4.8] | 1.7 [0.7, 2.7] | 1.9 [0.8, 3.2] |

## Realized test FPR (%) at the 1 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.9 [0.8, 1.0] | 1.2 [0.9, 1.5] | 1.0 [0.8, 1.3] |
| iforest | final | 0.7 [0.6, 0.9] | 1.0 [0.7, 1.3] | 0.7 [0.4, 0.9] |
| iforest_no_peers | all | 1.1 [0.8, 1.3] | 1.2 [0.9, 1.4] | 1.1 [0.8, 1.3] |
| iforest_no_peers | final | 0.7 [0.4, 1.0] | 1.1 [0.8, 1.5] | 1.0 [0.7, 1.3] |
| quantile_rules | all | 0.7 [0.5, 0.8] | 0.9 [0.7, 1.2] | 1.0 [0.7, 1.2] |
| quantile_rules | final | 0.7 [0.4, 0.9] | 0.9 [0.7, 1.2] | 1.1 [0.7, 1.6] |

## Precision (%) at the 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 56.5 [54.7, 58.4] | 58.7 [56.4, 61.1] | 43.3 [40.3, 46.3] |
| iforest | final | 86.1 [84.2, 88.0] | 86.1 [84.2, 88.0] | 76.9 [74.1, 79.7] |
| iforest | S1 rapid_response↔preknowledge | 26.5 [24.4, 28.6] | 24.9 [23.1, 26.5] | 19.2 [17.3, 21.2] |
| iforest | S2 excessive_changes↔lookup | 25.3 [23.2, 27.7] | 31.4 [29.1, 33.7] | 9.0 [7.3, 10.6] |
| iforest | S3 collusion↔copy_peer | 35.8 [33.0, 39.1] | 37.9 [34.3, 41.6] | 28.0 [24.6, 31.5] |
| iforest | S4 score_jump↔proxy | 2.7 [1.6, 3.7] | 2.0 [1.2, 2.9] | 4.1 [3.3, 4.9] |
| iforest_no_peers | all | 53.0 [50.8, 55.4] | 54.1 [51.5, 56.7] | 36.9 [34.1, 39.7] |
| iforest_no_peers | final | 85.8 [84.0, 87.6] | 85.5 [83.8, 87.1] | 74.2 [71.3, 77.0] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 36.2 [34.0, 38.5] | 33.6 [31.2, 36.1] | 21.6 [19.2, 24.1] |
| iforest_no_peers | S2 excessive_changes↔lookup | 34.5 [31.8, 37.5] | 37.4 [34.5, 40.2] | 13.3 [11.6, 14.9] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.9 [0.3, 1.6] | 5.5 [4.1, 6.9] | 11.5 [9.6, 13.3] |
| iforest_no_peers | S4 score_jump↔proxy | 2.7 [1.8, 3.5] | 2.3 [1.5, 3.2] | 3.1 [2.2, 4.1] |
| quantile_rules | all | 68.8 [66.7, 70.7] | 61.5 [58.7, 64.3] | 48.7 [46.0, 51.4] |
| quantile_rules | final | 89.9 [88.6, 91.2] | 85.2 [83.7, 86.6] | 76.9 [74.5, 79.3] |
| quantile_rules | S1 rapid_response↔preknowledge | 36.0 [33.4, 38.5] | 23.5 [21.4, 25.7] | 24.2 [21.5, 26.9] |
| quantile_rules | S2 excessive_changes↔lookup | 44.8 [42.2, 47.5] | 39.4 [35.9, 43.2] | 15.9 [13.1, 18.7] |
| quantile_rules | S3 collusion↔copy_peer | 43.2 [40.2, 46.4] | 37.3 [34.1, 40.7] | 28.4 [25.7, 31.2] |
| quantile_rules | S4 score_jump↔proxy | 6.5 [4.9, 8.0] | 4.8 [3.4, 6.4] | 4.4 [2.9, 6.1] |

## Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.658 [0.652, 0.665] | 0.700 [0.691, 0.709] | 0.589 [0.580, 0.597] |
| iforest | final | 0.673 [0.659, 0.686] | 0.710 [0.698, 0.722] | 0.595 [0.585, 0.605] |
| iforest | S1 rapid_response↔preknowledge | 0.623 [0.610, 0.636] | 0.608 [0.596, 0.620] | 0.572 [0.561, 0.582] |
| iforest | S2 excessive_changes↔lookup | 0.684 [0.669, 0.699] | 0.751 [0.732, 0.770] | 0.529 [0.520, 0.540] |
| iforest | S3 collusion↔copy_peer | 0.781 [0.757, 0.805] | 0.926 [0.913, 0.937] | 0.730 [0.710, 0.750] |
| iforest | S4 score_jump↔proxy | 0.505 [0.497, 0.515] | 0.502 [0.495, 0.509] | 0.517 [0.510, 0.523] |
| iforest_no_peers | all | 0.648 [0.638, 0.657] | 0.656 [0.648, 0.665] | 0.555 [0.549, 0.562] |
| iforest_no_peers | final | 0.664 [0.653, 0.677] | 0.667 [0.657, 0.676] | 0.565 [0.557, 0.573] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.719 [0.703, 0.733] | 0.681 [0.670, 0.693] | 0.588 [0.577, 0.600] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.784 [0.770, 0.797] | 0.849 [0.836, 0.863] | 0.550 [0.543, 0.557] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.490 [0.488, 0.492] | 0.513 [0.506, 0.522] | 0.544 [0.533, 0.555] |
| iforest_no_peers | S4 score_jump↔proxy | 0.506 [0.500, 0.513] | 0.504 [0.497, 0.512] | 0.509 [0.501, 0.518] |
| quantile_rules | all | 0.805 [0.796, 0.814] | 0.758 [0.748, 0.767] | 0.616 [0.606, 0.626] |
| quantile_rules | final | 0.807 [0.796, 0.817] | 0.755 [0.743, 0.766] | 0.612 [0.601, 0.623] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.691 [0.675, 0.707] | 0.593 [0.579, 0.607] | 0.589 [0.576, 0.603] |
| quantile_rules | S2 excessive_changes↔lookup | 0.971 [0.961, 0.979] | 0.939 [0.930, 0.949] | 0.571 [0.556, 0.586] |
| quantile_rules | S3 collusion↔copy_peer | 0.980 [0.972, 0.986] | 0.974 [0.969, 0.980] | 0.777 [0.754, 0.801] |
| quantile_rules | S4 score_jump↔proxy | 0.531 [0.521, 0.542] | 0.523 [0.510, 0.538] | 0.513 [0.503, 0.524] |

## ROC-AUC

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.838 [0.829, 0.845] | 0.844 [0.834, 0.854] | 0.719 [0.705, 0.733] |
| iforest | final | 0.858 [0.848, 0.867] | 0.856 [0.846, 0.866] | 0.739 [0.725, 0.754] |
| iforest | S1 rapid_response↔preknowledge | 0.809 [0.792, 0.825] | 0.817 [0.805, 0.830] | 0.735 [0.721, 0.749] |
| iforest | S2 excessive_changes↔lookup | 0.909 [0.901, 0.918] | 0.955 [0.951, 0.960] | 0.699 [0.674, 0.724] |
| iforest | S3 collusion↔copy_peer | 0.977 [0.973, 0.980] | 0.993 [0.992, 0.994] | 0.851 [0.829, 0.871] |
| iforest | S4 score_jump↔proxy | 0.573 [0.544, 0.603] | 0.518 [0.490, 0.548] | 0.525 [0.498, 0.550] |
| iforest_no_peers | all | 0.744 [0.734, 0.754] | 0.817 [0.805, 0.830] | 0.712 [0.697, 0.726] |
| iforest_no_peers | final | 0.780 [0.770, 0.789] | 0.839 [0.828, 0.851] | 0.745 [0.730, 0.759] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.867 [0.853, 0.880] | 0.876 [0.863, 0.888] | 0.778 [0.760, 0.794] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.958 [0.953, 0.962] | 0.982 [0.980, 0.984] | 0.761 [0.741, 0.781] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.430 [0.398, 0.461] | 0.739 [0.716, 0.762] | 0.664 [0.644, 0.685] |
| iforest_no_peers | S4 score_jump↔proxy | 0.593 [0.568, 0.618] | 0.543 [0.513, 0.573] | 0.554 [0.526, 0.584] |
| quantile_rules | all | 0.909 [0.902, 0.915] | 0.879 [0.871, 0.887] | 0.796 [0.785, 0.807] |
| quantile_rules | final | 0.907 [0.900, 0.914] | 0.872 [0.863, 0.881] | 0.783 [0.771, 0.796] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.851 [0.837, 0.864] | 0.831 [0.817, 0.845] | 0.827 [0.812, 0.842] |
| quantile_rules | S2 excessive_changes↔lookup | 0.997 [0.996, 0.998] | 0.992 [0.991, 0.994] | 0.799 [0.779, 0.819] |
| quantile_rules | S3 collusion↔copy_peer | 0.998 [0.997, 0.999] | 0.997 [0.997, 0.998] | 0.871 [0.854, 0.888] |
| quantile_rules | S4 score_jump↔proxy | 0.761 [0.738, 0.782] | 0.635 [0.614, 0.658] | 0.618 [0.595, 0.642] |

## Average precision

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | all | 0.527 [0.513, 0.541] | 0.591 [0.573, 0.610] | 0.350 [0.326, 0.372] |
| iforest | final | 0.792 [0.778, 0.807] | 0.812 [0.799, 0.824] | 0.643 [0.620, 0.665] |
| iforest | S1 rapid_response↔preknowledge | 0.270 [0.245, 0.294] | 0.251 [0.232, 0.271] | 0.175 [0.157, 0.194] |
| iforest | S2 excessive_changes↔lookup | 0.375 [0.342, 0.411] | 0.504 [0.470, 0.541] | 0.087 [0.072, 0.103] |
| iforest | S3 collusion↔copy_peer | 0.488 [0.442, 0.538] | 0.744 [0.701, 0.786] | 0.424 [0.372, 0.473] |
| iforest | S4 score_jump↔proxy | 0.034 [0.027, 0.042] | 0.027 [0.024, 0.029] | 0.042 [0.033, 0.052] |
| iforest_no_peers | all | 0.472 [0.455, 0.489] | 0.516 [0.499, 0.534] | 0.294 [0.276, 0.311] |
| iforest_no_peers | final | 0.735 [0.724, 0.746] | 0.776 [0.766, 0.788] | 0.626 [0.604, 0.647] |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.476 [0.441, 0.509] | 0.404 [0.377, 0.431] | 0.221 [0.199, 0.242] |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.581 [0.549, 0.613] | 0.675 [0.637, 0.709] | 0.116 [0.101, 0.131] |
| iforest_no_peers | S3 collusion↔copy_peer | 0.028 [0.024, 0.032] | 0.069 [0.061, 0.077] | 0.089 [0.074, 0.104] |
| iforest_no_peers | S4 score_jump↔proxy | 0.038 [0.031, 0.046] | 0.031 [0.026, 0.039] | 0.038 [0.030, 0.048] |
| quantile_rules | all | 0.754 [0.739, 0.769] | 0.680 [0.664, 0.695] | 0.436 [0.410, 0.461] |
| quantile_rules | final | 0.883 [0.874, 0.892] | 0.840 [0.828, 0.851] | 0.696 [0.674, 0.717] |
| quantile_rules | S1 rapid_response↔preknowledge | 0.405 [0.368, 0.440] | 0.236 [0.212, 0.261] | 0.218 [0.196, 0.239] |
| quantile_rules | S2 excessive_changes↔lookup | 0.883 [0.845, 0.917] | 0.804 [0.776, 0.833] | 0.143 [0.116, 0.171] |
| quantile_rules | S3 collusion↔copy_peer | 0.897 [0.867, 0.925] | 0.858 [0.825, 0.889] | 0.492 [0.438, 0.546] |
| quantile_rules | S4 score_jump↔proxy | 0.071 [0.062, 0.079] | 0.046 [0.039, 0.056] | 0.041 [0.034, 0.049] |

Cells: mean over seeds [95 % percentile-bootstrap CI over seeds]. Slot rows compare that slot's cheaters with all normal test attempts (LEGACY/SIM-FI type ↔ SIM-BL mechanism).

## Median width (pp) of the within-run cohort-bootstrap 95 % CI of recall@5 %

What a single-dataset study would report as its uncertainty.

| Detector | Slot | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| iforest | S1 | 27.0 | 27.0 | 25.2 |
| iforest | S2 | 30.2 | 29.2 | 21.2 |
| iforest | S3 | 19.5 | 0.0 | 39.8 |
| iforest | S4 | 19.4 | 13.1 | 21.5 |
| iforest_no_peers | S1 | 26.7 | 25.2 | 24.8 |
| iforest_no_peers | S2 | 29.7 | 17.3 | 25.7 |
| iforest_no_peers | S3 | 0.0 | 22.9 | 24.2 |
| iforest_no_peers | S4 | 16.5 | 16.6 | 18.9 |
| quantile_rules | S1 | 28.1 | 25.6 | 24.1 |
| quantile_rules | S2 | 0.0 | 7.7 | 27.8 |
| quantile_rules | S3 | 0.0 | 0.0 | 43.1 |
| quantile_rules | S4 | 27.2 | 22.0 | 24.3 |

## SIM-BL, copy_peer with sync_submit_prob = 0 (subset: non-synchronously submitting pairs)

**Subset analysis, not a separate simulation:** the non-sync column restricts SIM-BL copy_peer cheaters to pairs that did not submit within ~2 s (truth.csv `sync_submit = 0`), against all normal test attempts. Last column: SIM-FI collusion − SIM-BL non-sync copy_peer, paired by seed (bootstrap CI; descriptive, outside the Holm family). Recall in %, pAUC standardized.

| Detector | Metric | SIM-FI collusion (all) | SIM-BL copy_peer (all) | SIM-BL copy_peer, non-sync pairs (≈ sync_submit_prob = 0) | SIM-BL copy_peer, sync pairs | SIM-FI − non-sync |
|---|---|---:|---:|---:|---:|---:|
| Rules | recall@5 | 100.0 [100.0, 100.0] | 65.5 [60.1, 70.9] | 39.8 [32.1, 47.6] | 92.0 [87.6, 96.1] | 60.2 [52.4, 67.9] |
| Rules | pauc_std | 0.974 [0.969, 0.980] | 0.777 [0.754, 0.801] | 0.651 [0.617, 0.685] | 0.908 [0.885, 0.930] | 0.324 [0.290, 0.356] |
| IF | recall@5 | 11.0 [7.7, 14.5] | 21.5 [18.7, 24.3] | 22.1 [19.4, 25.0] | 21.5 [16.3, 26.5] | -11.1 [-15.5, -6.6] |
| IF | pauc_std | 0.513 [0.506, 0.522] | 0.544 [0.533, 0.555] | 0.548 [0.535, 0.560] | 0.543 [0.525, 0.563] | -0.034 [-0.050, -0.017] |
| IF + peers | recall@5 | 100.0 [100.0, 100.0] | 61.0 [56.5, 65.4] | 36.1 [29.5, 42.9] | 86.0 [82.5, 89.5] | 63.9 [57.1, 70.5] |
| IF + peers | pauc_std | 0.926 [0.913, 0.937] | 0.730 [0.710, 0.750] | 0.625 [0.600, 0.650] | 0.838 [0.815, 0.861] | 0.300 [0.269, 0.329] |

## Manipulation check: what each injection/mechanism changed in observable terms

Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share of cheater attempts in which at least one of the four observables differs from the counterfactual.

| World | Type | cheaters/seed | Δ score (pts) | Δ duration (s) | Δ answer changes | Δ median first-answer time (s) | share changed |
|---|---|---:|---:|---:|---:|---:|---:|
| SIM-FI | rapid_response | 252.0 | 0.00 [0.00, 0.00] | -199.84 [-205.45, -194.20] | 0.00 [0.00, 0.00] | -9.80 [-10.06, -9.51] | 1.00 [1.00, 1.00] |
| SIM-FI | excessive_changes | 180.0 | 0.00 [0.00, 0.00] | -14.62 [-15.54, -13.62] | 27.35 [27.10, 27.59] | -9.98 [-10.32, -9.68] | 1.00 [1.00, 1.00] |
| SIM-FI | collusion | 180.0 | -3.63 [-3.75, -3.51] | 17.95 [9.65, 25.97] | -4.33 [-4.53, -4.15] | 0.91 [0.67, 1.16] | 1.00 [1.00, 1.00] |
| SIM-FI | score_jump | 108.0 | 2.91 [2.81, 3.03] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.92 [0.91, 0.93] |
| SIM-BL | preknowledge | 252.0 | 4.86 [4.66, 5.07] | -92.73 [-96.45, -88.89] | -2.68 [-2.75, -2.60] | -4.00 [-4.19, -3.81] | 0.99 [0.99, 1.00] |
| SIM-BL | lookup | 180.0 | 4.46 [4.28, 4.65] | 262.43 [251.39, 274.14] | 0.61 [0.50, 0.71] | 4.50 [4.14, 4.85] | 0.99 [0.99, 1.00] |
| SIM-BL | copy_peer | 180.0 | 0.56 [0.51, 0.62] | 64.56 [58.72, 70.74] | -1.31 [-1.39, -1.22] | -1.42 [-1.67, -1.15] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=0 | 90.1 | 0.56 [0.49, 0.63] | 54.06 [45.11, 63.08] | -1.36 [-1.48, -1.23] | -1.35 [-1.73, -0.98] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=1 | 89.9 | 0.57 [0.49, 0.67] | 74.73 [66.58, 82.67] | -1.26 [-1.37, -1.16] | -1.53 [-1.85, -1.22] | 1.00 [1.00, 1.00] |
| SIM-BL | proxy | 108.0 | 2.91 [2.76, 3.06] | -54.48 [-63.61, -45.20] | -0.89 [-1.05, -0.72] | -2.14 [-2.55, -1.72] | 1.00 [1.00, 1.00] |
