# Appendix: other unsupervised detectors

LOF (novelty), One-class SVM (RBF, nu = 0.05, standardized), robust Mahalanobis (MinCovDet) on the frozen model matrix, with and without peer features; same protocol as the main experiment.

**† UNRELIABLE:** in the marked k/n seeds the detector's validation FPR at the 5 % target was below 2.5 % (degenerate score distribution, e.g. a singular MCD covariance), so the frozen threshold is not a 5 % operating point.

## Recall (%) at the validation-frozen 5 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 67.7 [65.9, 69.4] | 64.4 [62.5, 66.3] | 40.9 [37.9, 43.8] |
| lof | final | 67.7 [65.9, 69.4] | 64.4 [62.5, 66.3] | 40.9 [37.9, 43.8] |
| lof | S1 rapid_response↔preknowledge | 47.9 [43.9, 51.6] | 44.0 [40.5, 47.4] | 36.2 [32.3, 40.1] |
| lof | S2 excessive_changes↔lookup | 97.1 [96.2, 98.0] | 96.8 [95.6, 98.0] | 40.9 [36.0, 45.5] |
| lof | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 71.0 [68.3, 73.9] |
| lof | S4 score_jump↔proxy | 15.8 [12.9, 18.9] | 8.0 [5.0, 11.4] | 7.8 [5.1, 11.0] |
| lof_no_peers | all | 46.1 [43.9, 48.0] | 45.7 [43.3, 48.2] | 33.8 [31.3, 36.2] |
| lof_no_peers | final | 46.1 [43.9, 48.0] | 45.7 [43.3, 48.2] | 33.8 [31.3, 36.2] |
| lof_no_peers | S1 rapid_response↔preknowledge | 47.1 [43.5, 50.4] | 43.5 [39.9, 47.2] | 34.6 [30.7, 38.3] |
| lof_no_peers | S2 excessive_changes↔lookup | 97.2 [96.2, 98.3] | 97.7 [96.6, 98.7] | 41.8 [37.2, 46.0] |
| lof_no_peers | S3 collusion↔copy_peer | 10.4 [8.1, 12.6] | 19.7 [15.5, 24.0] | 42.7 [41.1, 44.3] |
| lof_no_peers | S4 score_jump↔proxy | 14.7 [11.5, 17.9] | 7.7 [4.5, 11.5] | 6.4 [3.7, 9.5] |
| ocsvm | all | 68.8 [66.9, 70.8] | 63.9 [62.2, 65.7] | 44.4 [41.2, 47.5] |
| ocsvm | final | 68.8 [66.9, 70.8] | 63.9 [62.2, 65.7] | 44.4 [41.2, 47.5] |
| ocsvm | S1 rapid_response↔preknowledge | 49.3 [45.9, 52.7] | 40.7 [37.5, 44.0] | 43.6 [39.6, 47.7] |
| ocsvm | S2 excessive_changes↔lookup | 97.3 [96.2, 98.4] | 97.6 [96.5, 98.6] | 41.2 [35.5, 46.8] |
| ocsvm | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 73.0 [69.4, 76.7] |
| ocsvm | S4 score_jump↔proxy | 20.1 [17.0, 23.3] | 10.9 [7.4, 14.6] | 10.1 [7.4, 12.8] |
| ocsvm_no_peers | all | 49.4 [47.5, 51.2] | 48.6 [46.6, 50.7] | 37.9 [35.3, 40.4] |
| ocsvm_no_peers | final | 49.4 [47.5, 51.2] | 48.6 [46.6, 50.7] | 37.9 [35.3, 40.4] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 52.3 [49.0, 55.5] | 44.9 [42.7, 47.3] | 46.0 [42.4, 49.7] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 97.8 [96.9, 98.7] | 97.7 [96.6, 98.7] | 44.7 [39.1, 50.3] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 12.0 [9.1, 15.0] | 27.9 [23.2, 32.8] | 37.6 [34.4, 40.2] |
| ocsvm_no_peers | S4 score_jump↔proxy | 21.1 [17.0, 25.3] | 9.9 [6.4, 13.8] | 9.3 [6.1, 12.6] |
| mcd | all | 23.0 [19.0, 26.5] † 6/20 | 40.9 [34.3, 47.8] | 25.0 [21.4, 28.6] |
| mcd | final | 23.0 [19.0, 26.5] † 6/20 | 40.9 [34.3, 47.8] | 25.0 [21.4, 28.6] |
| mcd | S1 rapid_response↔preknowledge | 4.1 [2.9, 5.4] † 6/20 | 20.8 [14.5, 27.1] | 20.3 [14.1, 26.5] |
| mcd | S2 excessive_changes↔lookup | 4.7 [3.2, 6.4] † 6/20 | 48.6 [29.8, 67.6] | 10.9 [7.7, 14.1] |
| mcd | S3 collusion↔copy_peer | 85.3 [70.0, 100.0] † 6/20 | 89.8 [79.9, 97.8] | 60.7 [53.1, 67.7] |
| mcd | S4 score_jump↔proxy | 4.0 [2.1, 6.1] † 6/20 | 5.8 [3.2, 8.8] | 7.0 [5.1, 9.1] |
| mcd_no_peers | all | 6.1 [4.3, 8.4] † 4/20 | 32.0 [24.7, 39.7] | 20.3 [18.3, 22.4] |
| mcd_no_peers | final | 6.1 [4.3, 8.4] † 4/20 | 32.0 [24.7, 39.7] | 20.3 [18.3, 22.4] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 5.3 [4.0, 6.5] † 4/20 | 22.3 [17.1, 27.8] | 29.1 [25.6, 32.6] |
| mcd_no_peers | S2 excessive_changes↔lookup | 8.3 [2.2, 17.4] † 4/20 | 43.7 [24.2, 63.8] | 14.2 [12.0, 16.5] |
| mcd_no_peers | S3 collusion↔copy_peer | 6.1 [4.1, 8.1] † 4/20 | 53.2 [45.7, 60.3] | 22.3 [18.5, 26.3] |
| mcd_no_peers | S4 score_jump↔proxy | 5.6 [3.9, 7.4] † 4/20 | 7.3 [4.4, 10.4] | 8.0 [5.6, 10.6] |

## Realized test FPR (%) at the validation-frozen 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 4.5 [4.1, 4.9] | 5.1 [4.7, 5.5] | 5.1 [4.7, 5.5] |
| lof | final | 3.7 [3.1, 4.3] | 4.5 [3.9, 5.2] | 4.5 [3.8, 5.2] |
| lof_no_peers | all | 4.4 [4.0, 4.8] | 5.0 [4.5, 5.4] | 5.0 [4.5, 5.4] |
| lof_no_peers | final | 3.5 [2.9, 4.0] | 4.2 [3.6, 4.8] | 4.2 [3.7, 4.7] |
| ocsvm | all | 4.5 [4.1, 4.9] | 5.0 [4.5, 5.4] | 5.3 [4.8, 5.7] |
| ocsvm | final | 4.4 [3.8, 5.0] | 5.2 [4.6, 5.8] | 5.6 [4.9, 6.3] |
| ocsvm_no_peers | all | 4.6 [4.2, 5.1] | 5.0 [4.6, 5.5] | 5.1 [4.6, 5.6] |
| ocsvm_no_peers | final | 3.9 [3.3, 4.4] | 4.9 [4.3, 5.5] | 4.9 [4.3, 5.6] |
| mcd | all | 3.3 [2.6, 4.0] † 6/20 | 4.4 [4.1, 4.6] | 4.4 [3.9, 4.9] |
| mcd | final | 3.8 [3.0, 4.6] † 6/20 | 4.6 [4.0, 5.1] | 5.0 [4.3, 5.7] |
| mcd_no_peers | all | 3.6 [2.8, 4.5] † 4/20 | 5.0 [4.6, 5.5] | 4.4 [3.9, 5.0] |
| mcd_no_peers | final | 4.1 [3.0, 5.2] † 4/20 | 5.1 [4.3, 6.0] | 4.6 [3.6, 5.5] |

## Recall (%) at the validation-frozen 1 % FPR threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 57.6 [55.9, 59.3] | 44.7 [42.3, 47.1] | 16.0 [13.5, 18.7] |
| lof | final | 57.6 [55.9, 59.3] | 44.7 [42.3, 47.1] | 16.0 [13.5, 18.7] |
| lof | S1 rapid_response↔preknowledge | 28.9 [26.0, 31.9] | 9.5 [7.4, 11.5] | 8.2 [5.9, 10.7] |
| lof | S2 excessive_changes↔lookup | 90.7 [89.2, 92.2] | 72.0 [65.2, 78.5] | 13.0 [9.7, 16.5] |
| lof | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 42.1 [37.0, 47.5] |
| lof | S4 score_jump↔proxy | 4.6 [3.1, 6.2] | 0.4 [0.0, 1.0] | 0.4 [0.0, 1.0] |
| lof_no_peers | all | 34.7 [32.8, 36.6] | 23.4 [20.8, 25.7] | 11.2 [9.3, 13.3] |
| lof_no_peers | final | 34.7 [32.8, 36.6] | 23.4 [20.8, 25.7] | 11.2 [9.3, 13.3] |
| lof_no_peers | S1 rapid_response↔preknowledge | 29.2 [26.5, 32.2] | 10.0 [7.6, 12.3] | 7.6 [5.5, 9.9] |
| lof_no_peers | S2 excessive_changes↔lookup | 91.5 [89.7, 93.3] | 74.8 [67.9, 81.0] | 12.0 [8.7, 15.6] |
| lof_no_peers | S3 collusion↔copy_peer | 2.0 [0.6, 3.8] | 3.9 [2.2, 5.6] | 23.2 [20.1, 26.7] |
| lof_no_peers | S4 score_jump↔proxy | 4.8 [3.3, 6.6] | 0.2 [0.0, 0.7] | 0.5 [0.0, 1.2] |
| ocsvm | all | 58.8 [56.8, 60.7] | 50.8 [49.0, 52.7] | 23.1 [20.9, 25.2] |
| ocsvm | final | 58.8 [56.8, 60.7] | 50.8 [49.0, 52.7] | 23.1 [20.9, 25.2] |
| ocsvm | S1 rapid_response↔preknowledge | 33.1 [30.0, 36.1] | 18.2 [15.3, 21.2] | 16.9 [14.1, 19.6] |
| ocsvm | S2 excessive_changes↔lookup | 89.1 [87.5, 90.6] | 83.5 [79.1, 87.8] | 15.2 [12.4, 18.2] |
| ocsvm | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 55.4 [51.3, 59.9] |
| ocsvm | S4 score_jump↔proxy | 5.6 [3.7, 7.6] | 1.6 [0.3, 3.2] | 2.9 [1.4, 4.5] |
| ocsvm_no_peers | all | 38.2 [36.4, 39.9] | 31.8 [29.6, 34.1] | 15.7 [13.7, 17.8] |
| ocsvm_no_peers | final | 38.2 [36.4, 39.9] | 31.8 [29.6, 34.1] | 15.7 [13.7, 17.8] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 36.1 [33.1, 39.0] | 21.1 [18.1, 24.2] | 16.7 [14.3, 19.3] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 92.5 [90.9, 94.0] | 88.6 [85.6, 91.8] | 19.3 [15.7, 23.2] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 3.7 [1.8, 5.9] | 7.6 [4.9, 10.4] | 19.7 [16.3, 23.4] |
| ocsvm_no_peers | S4 score_jump↔proxy | 7.2 [5.3, 9.1] | 1.8 [0.6, 3.1] | 2.2 [1.0, 3.6] |
| mcd | all | 21.1 [17.3, 24.3] † 6/20 | 29.4 [24.5, 34.8] | 13.1 [11.0, 15.1] |
| mcd | final | 21.1 [17.3, 24.3] † 6/20 | 29.4 [24.5, 34.8] | 13.1 [11.0, 15.1] |
| mcd | S1 rapid_response↔preknowledge | 2.4 [1.4, 3.6] † 6/20 | 7.8 [4.3, 11.9] | 5.5 [3.1, 8.0] |
| mcd | S2 excessive_changes↔lookup | 2.6 [1.3, 4.0] † 6/20 | 34.8 [18.8, 51.6] | 2.8 [1.7, 3.9] |
| mcd | S3 collusion↔copy_peer | 84.0 [67.7, 98.6] † 6/20 | 80.4 [61.5, 95.2] | 44.6 [37.7, 51.2] |
| mcd | S4 score_jump↔proxy | 1.5 [0.2, 3.0] † 6/20 | 0.5 [0.0, 1.2] | 1.2 [0.2, 2.5] |
| mcd_no_peers | all | 2.7 [2.0, 3.5] † 4/20 | 16.2 [11.3, 21.4] | 10.4 [9.0, 11.7] |
| mcd_no_peers | final | 2.7 [2.0, 3.5] † 4/20 | 16.2 [11.3, 21.4] | 10.4 [9.0, 11.7] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 3.7 [2.5, 5.0] † 4/20 | 12.9 [10.2, 15.9] | 14.0 [11.3, 16.7] |
| mcd_no_peers | S2 excessive_changes↔lookup | 3.0 [1.1, 5.6] † 4/20 | 38.2 [20.2, 57.3] | 5.5 [3.8, 7.3] |
| mcd_no_peers | S3 collusion↔copy_peer | 1.8 [0.8, 2.9] † 4/20 | 9.6 [5.4, 14.7] | 16.3 [13.6, 19.0] |
| mcd_no_peers | S4 score_jump↔proxy | 1.6 [0.5, 2.8] † 4/20 | 1.5 [0.4, 2.7] | 2.4 [1.1, 3.8] |

## Realized test FPR (%) at the 1 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.7 [0.6, 0.9] | 1.1 [0.8, 1.4] | 1.1 [0.8, 1.4] |
| lof | final | 0.7 [0.5, 0.9] | 0.9 [0.6, 1.2] | 0.9 [0.6, 1.3] |
| lof_no_peers | all | 0.7 [0.6, 0.8] | 1.1 [0.8, 1.4] | 1.0 [0.8, 1.3] |
| lof_no_peers | final | 0.6 [0.4, 0.8] | 0.8 [0.5, 1.1] | 0.8 [0.5, 1.1] |
| ocsvm | all | 0.9 [0.7, 1.1] | 1.2 [0.9, 1.4] | 1.2 [0.9, 1.4] |
| ocsvm | final | 0.9 [0.7, 1.1] | 1.2 [0.8, 1.5] | 1.2 [0.8, 1.6] |
| ocsvm_no_peers | all | 0.9 [0.7, 1.1] | 1.2 [1.0, 1.4] | 1.2 [1.0, 1.4] |
| ocsvm_no_peers | final | 0.7 [0.5, 1.0] | 1.0 [0.7, 1.4] | 1.1 [0.7, 1.4] |
| mcd | all | 0.7 [0.4, 1.0] † 6/20 | 0.6 [0.4, 0.9] | 0.9 [0.6, 1.1] |
| mcd | final | 1.1 [0.6, 1.6] † 6/20 | 0.8 [0.4, 1.2] | 1.0 [0.6, 1.3] |
| mcd_no_peers | all | 0.5 [0.4, 0.7] † 4/20 | 1.1 [0.9, 1.3] | 1.1 [0.9, 1.3] |
| mcd_no_peers | final | 0.5 [0.3, 0.9] † 4/20 | 1.1 [0.7, 1.4] | 1.1 [0.7, 1.5] |

## Precision (%) at the 5 % threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 67.2 [65.3, 69.1] | 63.0 [60.9, 65.2] | 51.8 [49.0, 54.6] |
| lof | final | 91.2 [89.7, 92.5] | 88.7 [87.2, 90.2] | 83.3 [80.9, 85.7] |
| lof | S1 rapid_response↔preknowledge | 33.8 [30.9, 36.5] | 29.3 [26.7, 31.9] | 25.3 [22.7, 28.0] |
| lof | S2 excessive_changes↔lookup | 42.8 [40.2, 45.5] | 39.6 [37.0, 42.3] | 21.8 [18.7, 24.9] |
| lof | S3 collusion↔copy_peer | 41.7 [38.9, 44.6] | 37.8 [35.0, 40.6] | 30.2 [27.8, 32.7] |
| lof | S4 score_jump↔proxy | 6.9 [5.7, 8.2] | 3.2 [2.0, 4.4] | 3.2 [2.0, 4.4] |
| lof_no_peers | all | 58.7 [56.3, 61.1] | 55.3 [52.9, 57.7] | 47.8 [44.7, 50.8] |
| lof_no_peers | final | 88.1 [86.4, 90.0] | 85.6 [83.9, 87.3] | 81.1 [78.4, 83.8] |
| lof_no_peers | S1 rapid_response↔preknowledge | 34.0 [31.0, 36.8] | 29.4 [27.4, 31.4] | 24.9 [22.1, 27.7] |
| lof_no_peers | S2 excessive_changes↔lookup | 43.3 [40.7, 46.1] | 40.5 [37.7, 43.3] | 22.6 [19.6, 25.6] |
| lof_no_peers | S3 collusion↔copy_peer | 7.1 [5.5, 8.9] | 10.1 [8.4, 12.1] | 21.4 [18.9, 24.0] |
| lof_no_peers | S4 score_jump↔proxy | 6.5 [5.1, 7.8] | 3.1 [1.8, 4.5] | 2.5 [1.5, 3.6] |
| ocsvm | all | 67.7 [65.6, 69.8] | 63.4 [61.4, 65.6] | 53.1 [50.7, 55.4] |
| ocsvm | final | 89.7 [88.3, 91.1] | 87.2 [86.1, 88.2] | 81.1 [78.8, 83.4] |
| ocsvm | S1 rapid_response↔preknowledge | 34.6 [32.1, 37.5] | 28.2 [25.8, 30.5] | 28.3 [25.9, 30.7] |
| ocsvm | S2 excessive_changes↔lookup | 43.0 [40.2, 46.1] | 40.4 [37.8, 43.3] | 21.1 [18.3, 23.8] |
| ocsvm | S3 collusion↔copy_peer | 41.8 [39.1, 44.7] | 38.3 [35.7, 41.2] | 30.1 [27.6, 32.6] |
| ocsvm | S4 score_jump↔proxy | 8.6 [7.5, 9.7] | 4.5 [3.0, 6.0] | 3.8 [2.8, 4.9] |
| ocsvm_no_peers | all | 59.7 [57.4, 61.8] | 56.6 [54.6, 58.7] | 50.2 [47.4, 52.9] |
| ocsvm_no_peers | final | 87.8 [86.4, 89.3] | 84.4 [82.8, 86.1] | 80.5 [77.7, 83.2] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 35.6 [33.0, 38.1] | 30.1 [28.0, 32.2] | 30.4 [27.7, 33.0] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 42.7 [39.9, 45.4] | 40.2 [37.7, 42.6] | 23.3 [20.4, 26.2] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 8.0 [6.0, 10.2] | 14.1 [12.2, 16.0] | 18.9 [16.6, 21.1] |
| ocsvm_no_peers | S4 score_jump↔proxy | 8.7 [7.1, 10.2] | 3.9 [2.6, 5.3] | 3.8 [2.4, 5.3] |
| mcd | all | 49.4 [42.5, 56.1] † 6/20 | 53.7 [50.0, 57.1] | 42.6 [39.1, 46.0] |
| mcd | final | 75.3 [68.6, 80.4] † 6/20 | 81.8 [78.6, 84.6] | 72.4 [69.0, 75.9] |
| mcd | S1 rapid_response↔preknowledge | 7.4 [4.5, 11.1] † 6/20 | 16.5 [12.3, 20.7] | 16.2 [12.2, 20.2] |
| mcd | S2 excessive_changes↔lookup | 5.8 [3.2, 9.0] † 6/20 | 21.2 [13.5, 29.0] | 7.0 [5.1, 9.0] |
| mcd | S3 collusion↔copy_peer | 42.7 [33.5, 51.5] † 6/20 | 38.0 [33.9, 42.0] | 29.8 [26.0, 34.0] |
| mcd | S4 score_jump↔proxy | 3.4 [1.3, 6.2] † 6/20 | 2.4 [1.4, 3.5] | 3.5 [2.5, 4.6] |
| mcd_no_peers | all | 20.1 [15.6, 25.2] † 4/20 | 43.3 [37.6, 48.9] | 38.4 [35.6, 41.2] |
| mcd_no_peers | final | 46.3 [38.4, 54.4] † 4/20 | 74.3 [69.0, 79.4] | 71.7 [67.5, 76.0] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 9.0 [5.3, 13.6] † 4/20 | 17.2 [13.6, 20.8] | 24.0 [21.7, 26.2] |
| mcd_no_peers | S2 excessive_changes↔lookup | 5.0 [2.0, 8.8] † 4/20 | 18.2 [10.5, 26.1] | 10.2 [8.5, 12.0] |
| mcd_no_peers | S3 collusion↔copy_peer | 4.4 [2.9, 5.9] † 4/20 | 24.0 [21.1, 26.8] | 13.7 [11.3, 16.3] |
| mcd_no_peers | S4 score_jump↔proxy | 5.1 [2.6, 8.1] † 4/20 | 2.7 [1.8, 3.6] | 3.8 [2.6, 5.0] |

## Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.809 [0.801, 0.817] | 0.760 [0.750, 0.771] | 0.624 [0.613, 0.635] |
| lof | final | 0.817 [0.808, 0.824] | 0.771 [0.760, 0.783] | 0.638 [0.622, 0.655] |
| lof | S1 rapid_response↔preknowledge | 0.687 [0.671, 0.702] | 0.613 [0.599, 0.629] | 0.591 [0.576, 0.605] |
| lof | S2 excessive_changes↔lookup | 0.968 [0.961, 0.974] | 0.911 [0.901, 0.923] | 0.616 [0.598, 0.633] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 0.997 [0.995, 0.999] | 0.766 [0.752, 0.779] |
| lof | S4 score_jump↔proxy | 0.539 [0.530, 0.549] | 0.509 [0.499, 0.520] | 0.505 [0.498, 0.513] |
| lof_no_peers | all | 0.696 [0.685, 0.706] | 0.657 [0.646, 0.669] | 0.598 [0.589, 0.608] |
| lof_no_peers | final | 0.707 [0.695, 0.718] | 0.672 [0.658, 0.687] | 0.613 [0.597, 0.630] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.686 [0.670, 0.702] | 0.615 [0.600, 0.631] | 0.588 [0.575, 0.602] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.972 [0.966, 0.978] | 0.921 [0.911, 0.931] | 0.621 [0.603, 0.638] |
| lof_no_peers | S3 collusion↔copy_peer | 0.521 [0.511, 0.530] | 0.540 [0.527, 0.554] | 0.653 [0.642, 0.665] |
| lof_no_peers | S4 score_jump↔proxy | 0.538 [0.528, 0.547] | 0.507 [0.498, 0.518] | 0.504 [0.496, 0.512] |
| ocsvm | all | 0.813 [0.805, 0.821] | 0.773 [0.764, 0.781] | 0.644 [0.635, 0.654] |
| ocsvm | final | 0.814 [0.805, 0.823] | 0.771 [0.762, 0.780] | 0.644 [0.631, 0.658] |
| ocsvm | S1 rapid_response↔preknowledge | 0.696 [0.681, 0.711] | 0.626 [0.613, 0.638] | 0.621 [0.608, 0.634] |
| ocsvm | S2 excessive_changes↔lookup | 0.962 [0.957, 0.968] | 0.937 [0.928, 0.947] | 0.615 [0.599, 0.631] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.803 [0.785, 0.820] |
| ocsvm | S4 score_jump↔proxy | 0.549 [0.539, 0.559] | 0.513 [0.502, 0.525] | 0.514 [0.505, 0.522] |
| ocsvm_no_peers | all | 0.709 [0.699, 0.718] | 0.681 [0.672, 0.692] | 0.610 [0.602, 0.619] |
| ocsvm_no_peers | final | 0.717 [0.706, 0.727] | 0.684 [0.671, 0.697] | 0.614 [0.599, 0.629] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.712 [0.694, 0.728] | 0.645 [0.634, 0.656] | 0.630 [0.618, 0.643] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.972 [0.966, 0.978] | 0.946 [0.937, 0.955] | 0.634 [0.615, 0.652] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.526 [0.516, 0.536] | 0.567 [0.551, 0.584] | 0.623 [0.611, 0.635] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.555 [0.544, 0.567] | 0.512 [0.501, 0.525] | 0.510 [0.501, 0.519] |
| mcd | all | 0.603 [0.583, 0.620] † 6/20 | 0.667 [0.641, 0.696] | 0.585 [0.573, 0.597] |
| mcd | final | 0.604 [0.583, 0.622] † 6/20 | 0.666 [0.639, 0.696] | 0.581 [0.570, 0.593] |
| mcd | S1 rapid_response↔preknowledge | 0.505 [0.500, 0.510] † 6/20 | 0.561 [0.538, 0.584] | 0.553 [0.533, 0.572] |
| mcd | S2 excessive_changes↔lookup | 0.508 [0.501, 0.516] † 6/20 | 0.701 [0.617, 0.786] | 0.521 [0.512, 0.530] |
| mcd | S3 collusion↔copy_peer | 0.925 [0.848, 1.000] † 6/20 | 0.920 [0.842, 0.981] | 0.760 [0.726, 0.792] |
| mcd | S4 score_jump↔proxy | 0.504 [0.496, 0.513] † 6/20 | 0.504 [0.496, 0.514] | 0.510 [0.504, 0.516] |
| mcd_no_peers | all | 0.508 [0.504, 0.514] † 4/20 | 0.602 [0.572, 0.633] | 0.562 [0.556, 0.568] |
| mcd_no_peers | final | 0.513 [0.506, 0.521] † 4/20 | 0.607 [0.575, 0.639] | 0.565 [0.556, 0.574] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.511 [0.505, 0.517] † 4/20 | 0.575 [0.556, 0.594] | 0.591 [0.580, 0.602] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.507 [0.494, 0.527] † 4/20 | 0.691 [0.597, 0.788] | 0.538 [0.531, 0.545] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.508 [0.502, 0.514] † 4/20 | 0.622 [0.603, 0.640] | 0.581 [0.566, 0.598] |
| mcd_no_peers | S4 score_jump↔proxy | 0.509 [0.501, 0.517] † 4/20 | 0.504 [0.497, 0.511] | 0.513 [0.506, 0.520] |

## ROC-AUC

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.894 [0.889, 0.899] | 0.876 [0.867, 0.886] | 0.784 [0.770, 0.799] |
| lof | final | 0.899 [0.893, 0.904] | 0.879 [0.869, 0.890] | 0.789 [0.772, 0.805] |
| lof | S1 rapid_response↔preknowledge | 0.844 [0.827, 0.858] | 0.852 [0.839, 0.864] | 0.789 [0.770, 0.808] |
| lof | S2 excessive_changes↔lookup | 0.995 [0.993, 0.996] | 0.989 [0.987, 0.991] | 0.833 [0.814, 0.853] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.876 [0.861, 0.889] |
| lof | S4 score_jump↔proxy | 0.678 [0.655, 0.701] | 0.581 [0.552, 0.613] | 0.567 [0.542, 0.595] |
| lof_no_peers | all | 0.800 [0.788, 0.811] | 0.808 [0.796, 0.821] | 0.747 [0.732, 0.762] |
| lof_no_peers | final | 0.810 [0.799, 0.820] | 0.814 [0.800, 0.829] | 0.754 [0.736, 0.771] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.843 [0.826, 0.858] | 0.848 [0.834, 0.860] | 0.784 [0.766, 0.802] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.996 [0.994, 0.997] | 0.990 [0.988, 0.992] | 0.833 [0.813, 0.853] |
| lof_no_peers | S3 collusion↔copy_peer | 0.604 [0.566, 0.641] | 0.717 [0.686, 0.751] | 0.730 [0.718, 0.741] |
| lof_no_peers | S4 score_jump↔proxy | 0.677 [0.655, 0.698] | 0.574 [0.547, 0.603] | 0.557 [0.529, 0.586] |
| ocsvm | all | 0.894 [0.887, 0.901] | 0.857 [0.848, 0.866] | 0.789 [0.776, 0.803] |
| ocsvm | final | 0.902 [0.895, 0.909] | 0.865 [0.856, 0.874] | 0.800 [0.786, 0.814] |
| ocsvm | S1 rapid_response↔preknowledge | 0.822 [0.805, 0.837] | 0.785 [0.771, 0.800] | 0.817 [0.800, 0.834] |
| ocsvm | S2 excessive_changes↔lookup | 0.994 [0.993, 0.996] | 0.992 [0.989, 0.994] | 0.817 [0.794, 0.838] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [1.000, 1.000] | 0.873 [0.854, 0.890] |
| ocsvm | S4 score_jump↔proxy | 0.732 [0.709, 0.755] | 0.602 [0.578, 0.627] | 0.567 [0.543, 0.595] |
| ocsvm_no_peers | all | 0.785 [0.776, 0.795] | 0.821 [0.810, 0.833] | 0.753 [0.740, 0.767] |
| ocsvm_no_peers | final | 0.803 [0.795, 0.810] | 0.833 [0.820, 0.845] | 0.768 [0.754, 0.783] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.829 [0.811, 0.846] | 0.802 [0.788, 0.816] | 0.818 [0.803, 0.833] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.996 [0.994, 0.997] | 0.993 [0.991, 0.995] | 0.827 [0.806, 0.848] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.526 [0.494, 0.557] | 0.819 [0.796, 0.842] | 0.702 [0.684, 0.718] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.731 [0.709, 0.754] | 0.602 [0.579, 0.626] | 0.568 [0.541, 0.598] |
| mcd | all | 0.804 [0.788, 0.820] † 6/20 | 0.822 [0.804, 0.841] | 0.714 [0.702, 0.726] |
| mcd | final | 0.805 [0.787, 0.823] † 6/20 | 0.820 [0.801, 0.838] | 0.711 [0.700, 0.722] |
| mcd | S1 rapid_response↔preknowledge | 0.752 [0.730, 0.774] † 6/20 | 0.788 [0.767, 0.810] | 0.740 [0.718, 0.762] |
| mcd | S2 excessive_changes↔lookup | 0.877 [0.850, 0.904] † 6/20 | 0.908 [0.873, 0.942] | 0.664 [0.643, 0.685] |
| mcd | S3 collusion↔copy_peer | 0.939 [0.867, 1.000] † 6/20 | 0.987 [0.975, 0.997] | 0.849 [0.816, 0.877] |
| mcd | S4 score_jump↔proxy | 0.606 [0.581, 0.632] † 6/20 | 0.532 [0.504, 0.561] | 0.550 [0.525, 0.574] |
| mcd_no_peers | all | 0.730 [0.718, 0.742] † 4/20 | 0.820 [0.807, 0.834] | 0.695 [0.683, 0.706] |
| mcd_no_peers | final | 0.730 [0.718, 0.743] † 4/20 | 0.818 [0.805, 0.832] | 0.694 [0.682, 0.706] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.761 [0.746, 0.776] † 4/20 | 0.799 [0.786, 0.814] | 0.769 [0.755, 0.782] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.923 [0.915, 0.930] † 4/20 | 0.918 [0.890, 0.946] | 0.675 [0.658, 0.691] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.555 [0.524, 0.584] † 4/20 | 0.944 [0.937, 0.950] | 0.695 [0.676, 0.712] |
| mcd_no_peers | S4 score_jump↔proxy | 0.609 [0.584, 0.633] † 4/20 | 0.538 [0.513, 0.564] | 0.564 [0.542, 0.587] |

## Average precision

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | all | 0.764 [0.754, 0.775] | 0.699 [0.682, 0.716] | 0.439 [0.413, 0.466] |
| lof | final | 0.885 [0.879, 0.891] | 0.858 [0.847, 0.870] | 0.721 [0.698, 0.744] |
| lof | S1 rapid_response↔preknowledge | 0.430 [0.392, 0.465] | 0.251 [0.229, 0.275] | 0.199 [0.176, 0.222] |
| lof | S2 excessive_changes↔lookup | 0.939 [0.927, 0.950] | 0.744 [0.705, 0.782] | 0.207 [0.174, 0.241] |
| lof | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 0.990 [0.980, 0.997] | 0.468 [0.430, 0.505] |
| lof | S4 score_jump↔proxy | 0.063 [0.054, 0.073] | 0.035 [0.029, 0.041] | 0.032 [0.028, 0.036] |
| lof_no_peers | all | 0.574 [0.556, 0.592] | 0.505 [0.483, 0.528] | 0.373 [0.348, 0.398] |
| lof_no_peers | final | 0.778 [0.768, 0.788] | 0.761 [0.745, 0.778] | 0.674 [0.648, 0.700] |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.431 [0.395, 0.465] | 0.254 [0.231, 0.279] | 0.196 [0.175, 0.219] |
| lof_no_peers | S2 excessive_changes↔lookup | 0.949 [0.936, 0.959] | 0.772 [0.735, 0.809] | 0.214 [0.181, 0.247] |
| lof_no_peers | S3 collusion↔copy_peer | 0.058 [0.048, 0.069] | 0.084 [0.072, 0.097] | 0.246 [0.217, 0.276] |
| lof_no_peers | S4 score_jump↔proxy | 0.062 [0.053, 0.073] | 0.033 [0.029, 0.039] | 0.031 [0.027, 0.035] |
| ocsvm | all | 0.769 [0.757, 0.781] | 0.703 [0.690, 0.716] | 0.486 [0.460, 0.511] |
| ocsvm | final | 0.887 [0.879, 0.895] | 0.846 [0.837, 0.856] | 0.735 [0.712, 0.757] |
| ocsvm | S1 rapid_response↔preknowledge | 0.444 [0.411, 0.477] | 0.274 [0.250, 0.300] | 0.265 [0.238, 0.294] |
| ocsvm | S2 excessive_changes↔lookup | 0.925 [0.913, 0.935] | 0.838 [0.809, 0.868] | 0.206 [0.172, 0.241] |
| ocsvm | S3 collusion↔copy_peer | 1.000 [1.000, 1.000] | 1.000 [0.999, 1.000] | 0.573 [0.528, 0.615] |
| ocsvm | S4 score_jump↔proxy | 0.075 [0.064, 0.087] | 0.039 [0.033, 0.045] | 0.039 [0.032, 0.046] |
| ocsvm_no_peers | all | 0.590 [0.574, 0.604] | 0.555 [0.536, 0.574] | 0.396 [0.372, 0.419] |
| ocsvm_no_peers | final | 0.783 [0.776, 0.790] | 0.781 [0.768, 0.796] | 0.685 [0.660, 0.710] |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.482 [0.443, 0.517] | 0.303 [0.280, 0.326] | 0.269 [0.245, 0.295] |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.944 [0.931, 0.956] | 0.851 [0.821, 0.881] | 0.230 [0.194, 0.265] |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.058 [0.047, 0.069] | 0.127 [0.110, 0.145] | 0.193 [0.166, 0.221] |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.085 [0.072, 0.099] | 0.040 [0.033, 0.047] | 0.037 [0.031, 0.044] |
| mcd | all | 0.449 [0.410, 0.485] † 6/20 | 0.527 [0.483, 0.575] | 0.336 [0.314, 0.360] |
| mcd | final | 0.699 [0.667, 0.730] † 6/20 | 0.743 [0.714, 0.774] | 0.601 [0.582, 0.621] |
| mcd | S1 rapid_response↔preknowledge | 0.113 [0.096, 0.131] † 6/20 | 0.163 [0.130, 0.198] | 0.135 [0.110, 0.160] |
| mcd | S2 excessive_changes↔lookup | 0.160 [0.127, 0.193] † 6/20 | 0.404 [0.273, 0.537] | 0.066 [0.056, 0.075] |
| mcd | S3 collusion↔copy_peer | 0.857 [0.711, 1.000] † 6/20 | 0.845 [0.692, 0.962] | 0.507 [0.438, 0.571] |
| mcd | S4 score_jump↔proxy | 0.040 [0.031, 0.051] † 6/20 | 0.029 [0.025, 0.034] | 0.032 [0.028, 0.037] |
| mcd_no_peers | all | 0.254 [0.238, 0.271] † 4/20 | 0.420 [0.367, 0.474] | 0.276 [0.264, 0.288] |
| mcd_no_peers | final | 0.561 [0.540, 0.582] † 4/20 | 0.701 [0.668, 0.735] | 0.569 [0.553, 0.587] |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.130 [0.116, 0.144] † 4/20 | 0.184 [0.159, 0.210] | 0.189 [0.172, 0.209] |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.199 [0.174, 0.232] † 4/20 | 0.415 [0.267, 0.569] | 0.079 [0.072, 0.086] |
| mcd_no_peers | S3 collusion↔copy_peer | 0.046 [0.039, 0.053] † 4/20 | 0.238 [0.215, 0.260] | 0.154 [0.135, 0.174] |
| mcd_no_peers | S4 score_jump↔proxy | 0.041 [0.033, 0.049] † 4/20 | 0.029 [0.026, 0.032] | 0.035 [0.030, 0.039] |

Cells: mean over seeds [95 % percentile-bootstrap CI over seeds]. Slot rows compare that slot's cheaters with all normal test attempts (LEGACY/SIM-FI type ↔ SIM-BL mechanism).

## Median width (pp) of the within-run cohort-bootstrap 95 % CI of recall@5 %

What a single-dataset study would report as its uncertainty.

| Detector | Slot | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| lof | S1 | 27.5 | 26.4 | 24.1 |
| lof | S2 | 8.5 | 9.6 | 28.7 |
| lof | S3 | 0.0 | 0.0 | 29.3 |
| lof | S4 | 29.6 | 17.9 | 16.8 |
| lof_no_peers | S1 | 25.4 | 26.6 | 24.3 |
| lof_no_peers | S2 | 8.5 | 8.0 | 30.8 |
| lof_no_peers | S3 | 21.1 | 31.2 | 19.5 |
| lof_no_peers | S4 | 27.7 | 17.5 | 16.0 |
| ocsvm | S1 | 27.9 | 25.0 | 26.5 |
| ocsvm | S2 | 8.4 | 7.4 | 31.4 |
| ocsvm | S3 | 0.0 | 0.0 | 33.5 |
| ocsvm | S4 | 31.6 | 24.3 | 24.5 |
| ocsvm_no_peers | S1 | 26.0 | 24.1 | 25.5 |
| ocsvm_no_peers | S2 | 8.4 | 8.0 | 31.9 |
| ocsvm_no_peers | S3 | 23.0 | 36.9 | 21.3 |
| ocsvm_no_peers | S4 | 32.2 | 21.5 | 21.4 |
| mcd | S1 | 9.7 | 20.6 | 20.6 |
| mcd | S2 | 11.8 | 14.5 | 19.0 |
| mcd | S3 | 0.0 | 0.0 | 39.7 |
| mcd | S4 | 7.3 | 14.0 | 16.9 |
| mcd_no_peers | S1 | 10.7 | 20.6 | 22.6 |
| mcd_no_peers | S2 | 0.0 | 9.0 | 21.4 |
| mcd_no_peers | S3 | 14.9 | 41.1 | 25.9 |
| mcd_no_peers | S4 | 15.2 | 14.2 | 19.6 |
