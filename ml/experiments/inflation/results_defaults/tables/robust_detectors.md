> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# Appendix: other unsupervised detectors

LOF (novelty), One-class SVM (RBF, nu = 0.05, standardized), robust Mahalanobis (MinCovDet) on the frozen model matrix, with and without peer features; same protocol as the main experiment.

## Recall (%) at the validation-frozen 5 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 67.7 [65.9, 69.4] | 59.7 [57.7, 61.8] | 31.6 [29.3, 34.0] |
| lof | final | 67.7 [65.9, 69.4] | 59.7 [57.7, 61.8] | 31.6 [29.3, 34.0] |
| lof | S1 rapid_response↔preknowledge | 47.9 [43.9, 51.6] | 27.5 [24.2, 30.7] | 29.2 [26.3, 32.1] |
| lof | S2 excessive_changes↔lookup | 97.1 [96.2, 98.0] | 99.8 [99.5, 100.0] | 36.6 [31.8, 41.8] |
| lof | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 43.4 [38.9, 48.3] |
| lof | S4 score_jump↔proxy | 15.8 [12.9, 18.9] | 10.0 [7.5, 12.8] | 12.2 [8.8, 15.7] |
| lof_no_peers | all | 46.1 [43.9, 48.0] | 41.3 [39.2, 43.5] | 25.1 [23.6, 26.8] |
| lof_no_peers | final | 46.1 [43.9, 48.0] | 41.3 [39.2, 43.5] | 25.1 [23.6, 26.8] |
| lof_no_peers | S1 rapid_response↔preknowledge | 47.1 [43.5, 50.4] | 27.7 [24.0, 31.5] | 30.8 [28.3, 33.4] |
| lof_no_peers | S2 excessive_changes↔lookup | 97.2 [96.2, 98.3] | 99.8 [99.5, 100.0] | 37.3 [32.7, 42.3] |
| lof_no_peers | S3 collusion↔copy_peer | 10.4 [8.1, 12.6] | 19.5 [15.3, 24.2] | 11.7 [9.5, 13.9] |
| lof_no_peers | S4 score_jump↔proxy | 14.7 [11.5, 17.9] | 10.4 [7.7, 13.4] | 13.4 [9.7, 17.3] |
| ocsvm | all | 68.8 [66.9, 70.8] | 60.7 [58.9, 62.4] | 35.0 [32.9, 37.3] |
| ocsvm | final | 68.8 [66.9, 70.8] | 60.7 [58.9, 62.4] | 35.0 [32.9, 37.3] |
| ocsvm | S1 rapid_response↔preknowledge | 49.3 [45.9, 52.7] | 29.4 [27.0, 31.8] | 32.2 [29.4, 35.0] |
| ocsvm | S2 excessive_changes↔lookup | 97.3 [96.2, 98.4] | 99.8 [99.5, 100.0] | 36.8 [33.5, 40.4] |
| ocsvm | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 50.0 [45.2, 55.1] |
| ocsvm | S4 score_jump↔proxy | 20.1 [17.0, 23.3] | 11.6 [8.9, 14.5] | 18.2 [14.5, 22.3] |
| ocsvm_no_peers | all | 49.4 [47.5, 51.2] | 43.9 [41.9, 45.9] | 26.3 [25.0, 27.8] |
| ocsvm_no_peers | final | 49.4 [47.5, 51.2] | 43.9 [41.9, 45.9] | 26.3 [25.0, 27.8] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 52.3 [49.0, 55.5] | 31.9 [29.0, 34.9] | 32.5 [30.0, 35.1] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 97.8 [96.9, 98.7] | 99.8 [99.5, 100.0] | 39.6 [35.3, 44.4] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 12.0 [9.1, 15.0] | 22.6 [19.3, 26.3] | 7.9 [5.8, 10.2] |
| ocsvm_no_peers | S4 score_jump↔proxy | 21.1 [17.0, 25.3] | 12.5 [9.9, 15.6] | 19.2 [15.1, 23.6] |
| mcd | all | 23.2 [19.1, 26.7] | 45.1 [39.1, 50.7] | 16.0 [13.9, 18.0] |
| mcd | final | 23.2 [19.1, 26.7] | 45.1 [39.1, 50.7] | 16.0 [13.9, 18.0] |
| mcd | S1 rapid_response↔preknowledge | 4.4 [3.1, 5.8] | 15.6 [11.0, 20.2] | 4.5 [3.0, 6.2] |
| mcd | S2 excessive_changes↔lookup | 4.6 [3.1, 6.3] | 83.9 [68.1, 97.3] | 26.3 [18.5, 33.8] |
| mcd | S3 collusion↔copy_peer | 85.3 [70.0, 100.0] | 76.0 [56.6, 91.1] | 31.5 [22.9, 40.1] |
| mcd | S4 score_jump↔proxy | 4.5 [2.3, 6.7] | 4.1 [2.4, 6.0] | 3.6 [1.7, 5.9] |
| mcd_no_peers | all | 6.1 [4.3, 8.4] | 30.6 [28.9, 32.4] | 10.2 [8.8, 11.7] |
| mcd_no_peers | final | 6.1 [4.3, 8.4] | 30.6 [28.9, 32.4] | 10.2 [8.8, 11.7] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 5.3 [4.0, 6.5] | 12.5 [8.6, 16.5] | 2.8 [1.3, 4.6] |
| mcd_no_peers | S2 excessive_changes↔lookup | 8.3 [2.2, 17.4] | 100.0 [100.0, 100.0] | 31.8 [27.2, 36.3] |
| mcd_no_peers | S3 collusion↔copy_peer | 6.1 [4.1, 8.1] | 0.8 [0.0, 1.8] | 4.0 [2.4, 5.6] |
| mcd_no_peers | S4 score_jump↔proxy | 5.6 [3.9, 7.4] | 4.0 [2.4, 5.9] | 2.2 [0.7, 3.9] |

## Realized test FPR (%) at the validation-frozen 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 4.5 [4.1, 4.9] | 4.9 [4.5, 5.3] | 4.9 [4.5, 5.3] |
| lof | final | 3.7 [3.1, 4.3] | 4.4 [3.9, 5.0] | 4.3 [3.8, 4.9] |
| lof_no_peers | all | 4.4 [4.0, 4.8] | 4.9 [4.5, 5.4] | 5.1 [4.7, 5.5] |
| lof_no_peers | final | 3.5 [2.9, 4.0] | 4.4 [3.8, 4.9] | 4.6 [4.0, 5.2] |
| ocsvm | all | 4.5 [4.1, 4.9] | 4.9 [4.6, 5.2] | 5.0 [4.7, 5.4] |
| ocsvm | final | 4.4 [3.8, 5.0] | 4.8 [4.3, 5.4] | 5.0 [4.4, 5.6] |
| ocsvm_no_peers | all | 4.6 [4.2, 5.1] | 5.0 [4.6, 5.3] | 5.0 [4.6, 5.3] |
| ocsvm_no_peers | final | 3.9 [3.3, 4.4] | 4.5 [3.9, 5.0] | 4.5 [3.8, 5.1] |
| mcd | all | 3.6 [2.8, 4.3] | 4.4 [3.9, 4.9] | 4.9 [4.4, 5.4] |
| mcd | final | 4.2 [3.2, 5.2] | 4.5 [3.9, 5.2] | 4.7 [3.9, 5.4] |
| mcd_no_peers | all | 3.6 [2.8, 4.5] | 4.4 [3.9, 5.0] | 4.5 [4.0, 5.0] |
| mcd_no_peers | final | 4.1 [3.0, 5.2] | 4.4 [3.8, 5.0] | 4.5 [3.7, 5.2] |

## Recall (%) at the validation-frozen 1 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 57.6 [55.9, 59.3] | 50.5 [48.6, 52.4] | 13.1 [10.9, 15.5] |
| lof | final | 57.6 [55.9, 59.3] | 50.5 [48.6, 52.4] | 13.1 [10.9, 15.5] |
| lof | S1 rapid_response↔preknowledge | 28.9 [26.0, 31.9] | 6.0 [4.2, 8.1] | 10.2 [7.7, 12.8] |
| lof | S2 excessive_changes↔lookup | 90.7 [89.2, 92.2] | 99.1 [98.1, 100.0] | 16.0 [12.8, 19.4] |
| lof | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 21.5 [17.8, 25.6] |
| lof | S4 score_jump↔proxy | 4.6 [3.1, 6.2] | 0.9 [0.2, 1.8] | 3.5 [1.4, 5.8] |
| lof_no_peers | all | 34.7 [32.8, 36.6] | 29.8 [27.6, 32.1] | 9.4 [7.9, 11.1] |
| lof_no_peers | final | 34.7 [32.8, 36.6] | 29.8 [27.6, 32.1] | 9.4 [7.9, 11.1] |
| lof_no_peers | S1 rapid_response↔preknowledge | 29.2 [26.5, 32.2] | 8.2 [5.7, 10.7] | 10.7 [8.3, 13.2] |
| lof_no_peers | S2 excessive_changes↔lookup | 91.5 [89.7, 93.3] | 99.4 [98.8, 99.9] | 17.2 [14.0, 20.5] |
| lof_no_peers | S3 collusion↔copy_peer | 2.0 [0.6, 3.8] | 5.7 [3.4, 8.3] | 3.5 [2.2, 4.8] |
| lof_no_peers | S4 score_jump↔proxy | 4.8 [3.3, 6.6] | 2.7 [1.1, 4.5] | 4.0 [1.6, 6.6] |
| ocsvm | all | 58.8 [56.8, 60.7] | 53.2 [51.4, 55.0] | 16.6 [14.5, 18.9] |
| ocsvm | final | 58.8 [56.8, 60.7] | 53.2 [51.4, 55.0] | 16.6 [14.5, 18.9] |
| ocsvm | S1 rapid_response↔preknowledge | 33.1 [30.0, 36.1] | 12.9 [10.7, 15.3] | 11.5 [9.6, 13.3] |
| ocsvm | S2 excessive_changes↔lookup | 89.1 [87.5, 90.6] | 99.6 [99.1, 100.0] | 16.6 [13.2, 20.4] |
| ocsvm | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 33.1 [27.9, 38.7] |
| ocsvm | S4 score_jump↔proxy | 5.6 [3.7, 7.6] | 1.5 [0.4, 2.8] | 4.8 [2.5, 7.4] |
| ocsvm_no_peers | all | 38.2 [36.4, 39.9] | 32.7 [30.8, 34.6] | 10.2 [8.6, 11.7] |
| ocsvm_no_peers | final | 38.2 [36.4, 39.9] | 32.7 [30.8, 34.6] | 10.2 [8.6, 11.7] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 36.1 [33.1, 39.0] | 14.5 [12.1, 17.2] | 12.5 [9.6, 15.3] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 92.5 [90.9, 94.0] | 99.6 [99.1, 100.0] | 18.2 [14.9, 21.7] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 3.7 [1.8, 5.9] | 8.4 [5.7, 11.5] | 2.2 [1.0, 3.9] |
| ocsvm_no_peers | S4 score_jump↔proxy | 7.2 [5.3, 9.1] | 2.5 [1.1, 4.0] | 5.4 [3.0, 8.1] |
| mcd | all | 21.1 [17.4, 24.3] | 37.6 [31.6, 43.5] | 7.9 [6.5, 9.3] |
| mcd | final | 21.1 [17.4, 24.3] | 37.6 [31.6, 43.5] | 7.9 [6.5, 9.3] |
| mcd | S1 rapid_response↔preknowledge | 2.3 [1.2, 3.6] | 2.9 [1.4, 4.7] | 0.3 [0.0, 0.7] |
| mcd | S2 excessive_changes↔lookup | 2.9 [1.5, 4.3] | 75.1 [55.3, 90.3] | 16.0 [10.9, 21.1] |
| mcd | S3 collusion↔copy_peer | 84.0 [67.7, 98.6] | 75.0 [55.3, 90.3] | 15.9 [9.0, 23.6] |
| mcd | S4 score_jump↔proxy | 1.5 [0.2, 3.0] | 0.8 [0.2, 1.7] | 0.2 [0.0, 0.8] |
| mcd_no_peers | all | 2.7 [2.0, 3.5] | 25.8 [24.4, 27.4] | 4.3 [3.3, 5.3] |
| mcd_no_peers | final | 2.7 [2.0, 3.5] | 25.8 [24.4, 27.4] | 4.3 [3.3, 5.3] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 3.7 [2.5, 5.0] | 1.8 [0.6, 3.2] | 0.1 [0.0, 0.3] |
| mcd_no_peers | S2 excessive_changes↔lookup | 3.0 [1.1, 5.6] | 98.8 [97.3, 99.9] | 16.1 [12.1, 20.3] |
| mcd_no_peers | S3 collusion↔copy_peer | 1.8 [0.8, 2.9] | 0.0 [0.0, 0.0] | 0.7 [0.1, 1.4] |
| mcd_no_peers | S4 score_jump↔proxy | 1.6 [0.5, 2.8] | 0.6 [0.0, 1.3] | 0.2 [0.0, 0.8] |

## Realized test FPR (%) at the 1 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.7 [0.6, 0.9] | 0.8 [0.6, 1.0] | 1.0 [0.8, 1.2] |
| lof | final | 0.7 [0.5, 0.9] | 0.6 [0.4, 0.9] | 0.7 [0.4, 1.0] |
| lof_no_peers | all | 0.7 [0.6, 0.8] | 0.9 [0.7, 1.1] | 0.9 [0.7, 1.2] |
| lof_no_peers | final | 0.6 [0.4, 0.8] | 0.7 [0.4, 0.9] | 0.8 [0.5, 1.1] |
| ocsvm | all | 0.9 [0.7, 1.1] | 0.8 [0.6, 1.0] | 0.9 [0.7, 1.1] |
| ocsvm | final | 0.9 [0.7, 1.1] | 0.7 [0.5, 0.9] | 0.9 [0.6, 1.1] |
| ocsvm_no_peers | all | 0.9 [0.7, 1.1] | 0.9 [0.7, 1.2] | 1.0 [0.8, 1.2] |
| ocsvm_no_peers | final | 0.7 [0.5, 1.0] | 1.0 [0.7, 1.3] | 1.0 [0.8, 1.3] |
| mcd | all | 0.7 [0.4, 0.9] | 0.6 [0.3, 0.8] | 0.8 [0.6, 1.0] |
| mcd | final | 1.0 [0.6, 1.5] | 0.7 [0.4, 1.0] | 0.8 [0.5, 1.0] |
| mcd_no_peers | all | 0.5 [0.4, 0.7] | 0.6 [0.4, 0.8] | 0.8 [0.6, 1.0] |
| mcd_no_peers | final | 0.5 [0.3, 0.9] | 0.5 [0.3, 0.7] | 0.8 [0.6, 1.0] |

## Precision (%) at the 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 67.2 [65.3, 69.1] | 62.2 [60.1, 64.2] | 46.4 [44.1, 48.8] |
| lof | final | 91.2 [89.7, 92.5] | 88.1 [86.6, 89.4] | 79.8 [77.2, 82.1] |
| lof | S1 rapid_response↔preknowledge | 33.8 [30.9, 36.5] | 20.8 [19.1, 22.5] | 22.2 [20.0, 24.6] |
| lof | S2 excessive_changes↔lookup | 42.8 [40.2, 45.5] | 41.4 [38.3, 44.6] | 20.2 [17.9, 22.6] |
| lof | S3 collusion↔copy_peer | 41.7 [38.9, 44.6] | 38.7 [36.1, 41.2] | 21.5 [18.9, 24.1] |
| lof | S4 score_jump↔proxy | 6.9 [5.7, 8.2] | 4.2 [3.2, 5.1] | 5.0 [3.7, 6.4] |
| lof_no_peers | all | 58.7 [56.3, 61.1] | 53.0 [50.8, 55.2] | 39.9 [37.8, 42.1] |
| lof_no_peers | final | 88.1 [86.4, 90.0] | 83.9 [82.1, 85.8] | 75.1 [72.5, 77.5] |
| lof_no_peers | S1 rapid_response↔preknowledge | 34.0 [31.0, 36.8] | 20.8 [18.8, 22.9] | 22.5 [20.4, 24.7] |
| lof_no_peers | S2 excessive_changes↔lookup | 43.3 [40.7, 46.1] | 41.1 [38.2, 44.2] | 19.8 [17.6, 22.1] |
| lof_no_peers | S3 collusion↔copy_peer | 7.1 [5.5, 8.9] | 10.4 [8.5, 12.3] | 6.6 [5.2, 8.0] |
| lof_no_peers | S4 score_jump↔proxy | 6.5 [5.1, 7.8] | 4.1 [3.1, 5.1] | 5.3 [3.9, 6.7] |
| ocsvm | all | 67.7 [65.6, 69.8] | 62.4 [60.6, 64.2] | 48.3 [45.9, 50.5] |
| ocsvm | final | 89.7 [88.3, 91.1] | 87.3 [85.8, 88.7] | 79.3 [77.2, 81.5] |
| ocsvm | S1 rapid_response↔preknowledge | 34.6 [32.1, 37.5] | 22.1 [20.6, 23.9] | 23.3 [21.4, 25.3] |
| ocsvm | S2 excessive_changes↔lookup | 43.0 [40.2, 46.1] | 41.1 [38.7, 43.4] | 20.0 [18.1, 21.8] |
| ocsvm | S3 collusion↔copy_peer | 41.8 [39.1, 44.7] | 38.5 [35.6, 41.3] | 23.3 [20.8, 25.9] |
| ocsvm | S4 score_jump↔proxy | 8.6 [7.5, 9.7] | 4.7 [3.6, 5.7] | 7.2 [5.8, 8.6] |
| ocsvm_no_peers | all | 59.7 [57.4, 61.8] | 54.3 [52.0, 56.4] | 41.6 [39.8, 43.4] |
| ocsvm_no_peers | final | 87.8 [86.4, 89.3] | 84.3 [82.7, 86.1] | 76.7 [74.8, 78.8] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 35.6 [33.0, 38.1] | 23.3 [21.5, 25.2] | 23.8 [21.9, 25.6] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 42.7 [39.9, 45.4] | 40.9 [38.2, 43.5] | 21.1 [19.3, 23.0] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 8.0 [6.0, 10.2] | 12.1 [10.5, 13.7] | 4.6 [3.3, 6.2] |
| ocsvm_no_peers | S4 score_jump↔proxy | 8.7 [7.1, 10.2] | 5.1 [4.1, 6.3] | 7.6 [6.0, 9.2] |
| mcd | all | 47.2 [41.0, 53.4] | 57.1 [53.3, 61.0] | 30.6 [27.5, 33.8] |
| mcd | final | 74.0 [67.5, 79.2] | 83.5 [80.2, 86.4] | 64.8 [60.2, 69.7] |
| mcd | S1 rapid_response↔preknowledge | 6.8 [4.2, 10.4] | 13.9 [9.9, 18.2] | 4.0 [2.7, 5.6] |
| mcd | S2 excessive_changes↔lookup | 5.8 [3.1, 9.0] | 38.0 [30.6, 44.9] | 15.0 [10.3, 19.6] |
| mcd | S3 collusion↔copy_peer | 40.4 [31.7, 48.6] | 30.7 [23.0, 37.6] | 15.9 [11.8, 20.3] |
| mcd | S4 score_jump↔proxy | 3.1 [1.2, 5.8] | 2.0 [1.2, 2.8] | 1.4 [0.6, 2.2] |
| mcd_no_peers | all | 20.1 [15.6, 25.2] | 49.0 [45.7, 52.5] | 23.4 [21.0, 26.0] |
| mcd_no_peers | final | 46.3 [38.4, 54.4] | 79.5 [77.1, 81.7] | 55.8 [51.1, 60.5] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 9.0 [5.3, 13.6] | 11.0 [8.2, 13.9] | 2.6 [1.3, 4.0] |
| mcd_no_peers | S2 excessive_changes↔lookup | 5.0 [2.0, 8.8] | 44.6 [40.7, 48.5] | 19.6 [16.9, 22.3] |
| mcd_no_peers | S3 collusion↔copy_peer | 4.4 [2.9, 5.9] | 0.4 [0.0, 0.9] | 2.4 [1.5, 3.3] |
| mcd_no_peers | S4 score_jump↔proxy | 5.1 [2.6, 8.1] | 1.9 [1.2, 2.6] | 0.9 [0.3, 1.5] |

## Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.809 [0.801, 0.817] | 0.768 [0.758, 0.778] | 0.596 [0.588, 0.605] |
| lof | final | 0.817 [0.808, 0.824] | 0.774 [0.763, 0.784] | 0.609 [0.598, 0.621] |
| lof | S1 rapid_response↔preknowledge | 0.687 [0.671, 0.702] | 0.570 [0.560, 0.580] | 0.583 [0.571, 0.596] |
| lof | S2 excessive_changes↔lookup | 0.968 [0.961, 0.974] | 0.997 [0.995, 0.999] | 0.614 [0.600, 0.631] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.648 [0.631, 0.667] |
| lof | S4 score_jump↔proxy | 0.539 [0.530, 0.549] | 0.512 [0.505, 0.521] | 0.524 [0.512, 0.537] |
| lof_no_peers | all | 0.696 [0.685, 0.706] | 0.665 [0.655, 0.675] | 0.570 [0.564, 0.577] |
| lof_no_peers | final | 0.707 [0.695, 0.718] | 0.673 [0.662, 0.683] | 0.580 [0.571, 0.589] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.686 [0.670, 0.702] | 0.573 [0.561, 0.584] | 0.588 [0.577, 0.600] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.972 [0.966, 0.978] | 0.998 [0.996, 0.999] | 0.619 [0.604, 0.635] |
| lof_no_peers | S3 collusion↔copy_peer | 0.521 [0.511, 0.530] | 0.544 [0.529, 0.560] | 0.522 [0.514, 0.531] |
| lof_no_peers | S4 score_jump↔proxy | 0.538 [0.528, 0.547] | 0.515 [0.507, 0.525] | 0.525 [0.512, 0.540] |
| ocsvm | all | 0.813 [0.805, 0.821] | 0.778 [0.769, 0.786] | 0.613 [0.603, 0.623] |
| ocsvm | final | 0.814 [0.805, 0.823] | 0.779 [0.769, 0.789] | 0.618 [0.605, 0.632] |
| ocsvm | S1 rapid_response↔preknowledge | 0.696 [0.681, 0.711] | 0.594 [0.584, 0.603] | 0.595 [0.583, 0.608] |
| ocsvm | S2 excessive_changes↔lookup | 0.962 [0.957, 0.968] | 0.997 [0.995, 0.999] | 0.615 [0.602, 0.630] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.688 [0.668, 0.710] |
| ocsvm | S4 score_jump↔proxy | 0.549 [0.539, 0.559] | 0.520 [0.512, 0.527] | 0.544 [0.529, 0.559] |
| ocsvm_no_peers | all | 0.709 [0.699, 0.718] | 0.679 [0.669, 0.688] | 0.575 [0.570, 0.580] |
| ocsvm_no_peers | final | 0.717 [0.706, 0.727] | 0.682 [0.672, 0.691] | 0.579 [0.574, 0.586] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.712 [0.694, 0.728] | 0.601 [0.590, 0.613] | 0.597 [0.587, 0.608] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.972 [0.966, 0.978] | 0.997 [0.995, 0.999] | 0.627 [0.613, 0.641] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.526 [0.516, 0.536] | 0.560 [0.548, 0.573] | 0.511 [0.503, 0.519] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.555 [0.544, 0.567] | 0.519 [0.511, 0.527] | 0.542 [0.528, 0.559] |
| mcd | all | 0.603 [0.583, 0.620] | 0.700 [0.670, 0.728] | 0.548 [0.540, 0.556] |
| mcd | final | 0.603 [0.583, 0.621] | 0.698 [0.667, 0.727] | 0.548 [0.540, 0.557] |
| mcd | S1 rapid_response↔preknowledge | 0.505 [0.500, 0.511] | 0.536 [0.519, 0.554] | 0.499 [0.494, 0.504] |
| mcd | S2 excessive_changes↔lookup | 0.508 [0.501, 0.517] | 0.895 [0.806, 0.971] | 0.593 [0.560, 0.625] |
| mcd | S3 collusion↔copy_peer | 0.925 [0.848, 1.000] | 0.874 [0.773, 0.952] | 0.611 [0.572, 0.654] |
| mcd | S4 score_jump↔proxy | 0.503 [0.495, 0.511] | 0.500 [0.495, 0.505] | 0.496 [0.491, 0.501] |
| mcd_no_peers | all | 0.508 [0.504, 0.514] | 0.631 [0.623, 0.639] | 0.525 [0.520, 0.530] |
| mcd_no_peers | final | 0.513 [0.506, 0.521] | 0.632 [0.624, 0.640] | 0.526 [0.521, 0.532] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.511 [0.505, 0.517] | 0.522 [0.511, 0.533] | 0.494 [0.490, 0.499] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.507 [0.494, 0.527] | 0.999 [0.998, 1.000] | 0.612 [0.594, 0.631] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.508 [0.502, 0.514] | 0.489 [0.487, 0.491] | 0.498 [0.494, 0.502] |
| mcd_no_peers | S4 score_jump↔proxy | 0.509 [0.501, 0.517] | 0.497 [0.493, 0.501] | 0.494 [0.489, 0.499] |

## ROC-AUC

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.894 [0.889, 0.899] | 0.874 [0.865, 0.883] | 0.745 [0.732, 0.758] |
| lof | final | 0.899 [0.893, 0.904] | 0.881 [0.872, 0.889] | 0.755 [0.742, 0.768] |
| lof | S1 rapid_response↔preknowledge | 0.844 [0.827, 0.858] | 0.816 [0.798, 0.833] | 0.747 [0.728, 0.766] |
| lof | S2 excessive_changes↔lookup | 0.995 [0.993, 0.996] | 1.000 [0.999, 1.000] | 0.798 [0.772, 0.823] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.784 [0.756, 0.810] |
| lof | S4 score_jump↔proxy | 0.678 [0.655, 0.701] | 0.627 [0.612, 0.643] | 0.600 [0.575, 0.625] |
| lof_no_peers | all | 0.800 [0.788, 0.811] | 0.801 [0.788, 0.814] | 0.699 [0.688, 0.709] |
| lof_no_peers | final | 0.810 [0.799, 0.820] | 0.813 [0.801, 0.826] | 0.713 [0.701, 0.725] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.843 [0.826, 0.858] | 0.817 [0.796, 0.836] | 0.743 [0.726, 0.760] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.996 [0.994, 0.997] | 1.000 [0.999, 1.000] | 0.801 [0.777, 0.825] |
| lof_no_peers | S3 collusion↔copy_peer | 0.604 [0.566, 0.641] | 0.684 [0.652, 0.719] | 0.587 [0.560, 0.616] |
| lof_no_peers | S4 score_jump↔proxy | 0.677 [0.655, 0.698] | 0.621 [0.603, 0.641] | 0.597 [0.574, 0.622] |
| ocsvm | all | 0.894 [0.887, 0.901] | 0.839 [0.829, 0.848] | 0.756 [0.745, 0.766] |
| ocsvm | final | 0.902 [0.895, 0.909] | 0.850 [0.841, 0.858] | 0.772 [0.762, 0.782] |
| ocsvm | S1 rapid_response↔preknowledge | 0.822 [0.805, 0.837] | 0.698 [0.678, 0.718] | 0.787 [0.772, 0.802] |
| ocsvm | S2 excessive_changes↔lookup | 0.994 [0.993, 0.996] | 1.000 [0.999, 1.000] | 0.791 [0.770, 0.812] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.744 [0.709, 0.778] |
| ocsvm | S4 score_jump↔proxy | 0.732 [0.709, 0.755] | 0.664 [0.641, 0.690] | 0.645 [0.615, 0.673] |
| ocsvm_no_peers | all | 0.785 [0.776, 0.795] | 0.780 [0.767, 0.793] | 0.687 [0.676, 0.698] |
| ocsvm_no_peers | final | 0.803 [0.795, 0.810] | 0.798 [0.787, 0.810] | 0.711 [0.702, 0.721] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.829 [0.811, 0.846] | 0.705 [0.687, 0.724] | 0.776 [0.761, 0.791] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.996 [0.994, 0.997] | 1.000 [0.999, 1.000] | 0.794 [0.771, 0.815] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.526 [0.494, 0.557] | 0.741 [0.714, 0.771] | 0.475 [0.443, 0.509] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.731 [0.709, 0.754] | 0.652 [0.628, 0.679] | 0.626 [0.597, 0.653] |
| mcd | all | 0.801 [0.785, 0.817] | 0.834 [0.816, 0.850] | 0.681 [0.668, 0.694] |
| mcd | final | 0.802 [0.784, 0.820] | 0.835 [0.818, 0.851] | 0.684 [0.672, 0.696] |
| mcd | S1 rapid_response↔preknowledge | 0.749 [0.726, 0.772] | 0.814 [0.784, 0.841] | 0.643 [0.619, 0.666] |
| mcd | S2 excessive_changes↔lookup | 0.870 [0.842, 0.898] | 0.979 [0.959, 0.996] | 0.742 [0.716, 0.769] |
| mcd | S3 collusion↔copy_peer | 0.939 [0.867, 1.000] | 0.905 [0.827, 0.971] | 0.761 [0.727, 0.793] |
| mcd | S4 score_jump↔proxy | 0.605 [0.580, 0.631] | 0.547 [0.527, 0.567] | 0.559 [0.537, 0.583] |
| mcd_no_peers | all | 0.730 [0.718, 0.742] | 0.758 [0.745, 0.771] | 0.644 [0.632, 0.654] |
| mcd_no_peers | final | 0.730 [0.718, 0.743] | 0.762 [0.750, 0.774] | 0.649 [0.639, 0.659] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.761 [0.746, 0.776] | 0.805 [0.784, 0.827] | 0.621 [0.596, 0.646] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.923 [0.915, 0.930] | 1.000 [1.000, 1.000] | 0.760 [0.741, 0.781] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.555 [0.524, 0.584] | 0.563 [0.531, 0.595] | 0.628 [0.611, 0.643] |
| mcd_no_peers | S4 score_jump↔proxy | 0.609 [0.584, 0.633] | 0.552 [0.533, 0.571] | 0.536 [0.516, 0.557] |

## Average precision

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.764 [0.754, 0.775] | 0.706 [0.690, 0.723] | 0.376 [0.354, 0.399] |
| lof | final | 0.885 [0.879, 0.891] | 0.855 [0.845, 0.865] | 0.672 [0.654, 0.691] |
| lof | S1 rapid_response↔preknowledge | 0.430 [0.392, 0.465] | 0.188 [0.171, 0.205] | 0.189 [0.167, 0.211] |
| lof | S2 excessive_changes↔lookup | 0.939 [0.927, 0.950] | 0.993 [0.989, 0.997] | 0.222 [0.191, 0.256] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.262 [0.225, 0.304] |
| lof | S4 score_jump↔proxy | 0.063 [0.054, 0.073] | 0.040 [0.035, 0.047] | 0.053 [0.040, 0.069] |
| lof_no_peers | all | 0.574 [0.556, 0.592] | 0.534 [0.515, 0.553] | 0.308 [0.291, 0.325] |
| lof_no_peers | final | 0.778 [0.768, 0.788] | 0.762 [0.749, 0.775] | 0.618 [0.602, 0.634] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.431 [0.395, 0.465] | 0.193 [0.174, 0.212] | 0.196 [0.176, 0.217] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.949 [0.936, 0.959] | 0.994 [0.990, 0.997] | 0.231 [0.200, 0.265] |
| lof_no_peers | S3 collusion↔copy_peer | 0.058 [0.048, 0.069] | 0.092 [0.076, 0.110] | 0.060 [0.051, 0.069] |
| lof_no_peers | S4 score_jump↔proxy | 0.062 [0.053, 0.073] | 0.042 [0.036, 0.049] | 0.054 [0.040, 0.070] |
| ocsvm | all | 0.769 [0.757, 0.781] | 0.700 [0.685, 0.715] | 0.415 [0.391, 0.437] |
| ocsvm | final | 0.887 [0.879, 0.895] | 0.837 [0.827, 0.847] | 0.692 [0.673, 0.710] |
| ocsvm | S1 rapid_response↔preknowledge | 0.444 [0.411, 0.477] | 0.219 [0.195, 0.243] | 0.223 [0.201, 0.246] |
| ocsvm | S2 excessive_changes↔lookup | 0.925 [0.913, 0.935] | 0.993 [0.988, 0.996] | 0.216 [0.187, 0.245] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 0.999 [0.999, 1.000] | 0.337 [0.291, 0.386] |
| ocsvm | S4 score_jump↔proxy | 0.075 [0.064, 0.087] | 0.052 [0.045, 0.062] | 0.068 [0.053, 0.086] |
| ocsvm_no_peers | all | 0.590 [0.574, 0.604] | 0.546 [0.527, 0.564] | 0.311 [0.297, 0.324] |
| ocsvm_no_peers | final | 0.783 [0.776, 0.790] | 0.757 [0.746, 0.770] | 0.615 [0.602, 0.627] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.482 [0.443, 0.517] | 0.229 [0.204, 0.256] | 0.219 [0.200, 0.238] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.944 [0.931, 0.956] | 0.992 [0.988, 0.996] | 0.229 [0.201, 0.258] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.058 [0.047, 0.069] | 0.116 [0.097, 0.137] | 0.040 [0.033, 0.047] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.085 [0.072, 0.099] | 0.049 [0.043, 0.055] | 0.062 [0.050, 0.077] |
| mcd | all | 0.445 [0.407, 0.480] | 0.592 [0.541, 0.641] | 0.263 [0.245, 0.280] |
| mcd | final | 0.694 [0.663, 0.725] | 0.780 [0.748, 0.810] | 0.547 [0.526, 0.568] |
| mcd | S1 rapid_response↔preknowledge | 0.111 [0.093, 0.129] | 0.153 [0.125, 0.182] | 0.068 [0.062, 0.073] |
| mcd | S2 excessive_changes↔lookup | 0.155 [0.121, 0.190] | 0.804 [0.641, 0.937] | 0.208 [0.155, 0.261] |
| mcd | S3 collusion↔copy_peer | 0.858 [0.711, 1.000] | 0.760 [0.568, 0.905] | 0.227 [0.155, 0.305] |
| mcd | S4 score_jump↔proxy | 0.039 [0.030, 0.050] | 0.027 [0.024, 0.030] | 0.027 [0.025, 0.028] |
| mcd_no_peers | all | 0.254 [0.238, 0.271] | 0.456 [0.437, 0.475] | 0.200 [0.185, 0.216] |
| mcd_no_peers | final | 0.561 [0.540, 0.582] | 0.686 [0.669, 0.703] | 0.485 [0.464, 0.506] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.130 [0.116, 0.144] | 0.129 [0.111, 0.149] | 0.061 [0.056, 0.067] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.199 [0.174, 0.232] | 0.999 [0.997, 1.000] | 0.218 [0.180, 0.258] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.046 [0.039, 0.053] | 0.035 [0.030, 0.039] | 0.043 [0.039, 0.048] |
| mcd_no_peers | S4 score_jump↔proxy | 0.041 [0.033, 0.049] | 0.027 [0.024, 0.030] | 0.025 [0.023, 0.027] |

Cells: mean over seeds [95 % percentile-bootstrap CI over seeds]. Slot rows compare that slot's cheaters with all normal test attempts (LEGACY/SIM-FI type ↔ SIM-BL mechanism).

## Median width (pp) of the within-run cohort-bootstrap 95 % CI of recall@5 %

What a single-dataset study would report as its uncertainty.

| Detector | Slot | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | S1 | 27.5 | 22.5 | 24.2 |
| lof | S2 | 8.5 | 0.0 | 29.6 |
| lof | S3 | 0.0 | 0.0 | 42.5 |
| lof | S4 | 29.6 | 23.1 | 22.7 |
| lof_no_peers | S1 | 25.4 | 22.0 | 24.9 |
| lof_no_peers | S2 | 8.5 | 0.0 | 29.6 |
| lof_no_peers | S3 | 21.1 | 32.7 | 20.0 |
| lof_no_peers | S4 | 27.7 | 24.2 | 23.2 |
| ocsvm | S1 | 27.9 | 25.8 | 25.5 |
| ocsvm | S2 | 8.4 | 0.0 | 30.4 |
| ocsvm | S3 | 0.0 | 0.0 | 43.8 |
| ocsvm | S4 | 31.6 | 25.3 | 26.2 |
| ocsvm_no_peers | S1 | 26.0 | 24.9 | 24.1 |
| ocsvm_no_peers | S2 | 8.4 | 0.0 | 29.6 |
| ocsvm_no_peers | S3 | 23.0 | 31.5 | 16.0 |
| ocsvm_no_peers | S4 | 32.2 | 23.7 | 27.4 |
| mcd | S1 | 9.8 | 19.7 | 9.2 |
| mcd | S2 | 10.5 | 0.0 | 26.0 |
| mcd | S3 | 0.0 | 0.0 | 43.1 |
| mcd | S4 | 7.5 | 12.7 | 0.0 |
| mcd_no_peers | S1 | 10.7 | 14.9 | 3.0 |
| mcd_no_peers | S2 | 0.0 | 0.0 | 28.2 |
| mcd_no_peers | S3 | 14.9 | 0.0 | 10.7 |
| mcd_no_peers | S4 | 15.2 | 13.2 | 0.0 |
