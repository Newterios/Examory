# Post hoc: robustness of the evaluation protocol at low prevalence (exploratory)

**Added after independent review (amendment of 15 Nov 2026).** Low-prevalence configuration of the `prevalence` stage (anomaly-rate 0.0167, ≈ 5 % of final attempts), seeds 1001–1020, frozen detectors. (i) label-free threshold: `numpy.quantile` (linear interpolation) at 0.95 of the scores of validation normal attempts, no positives used; (ii) contaminated fitting: train-cohort cheaters presented to the frozen pipeline as normal (reference statistics and models fitted on all train-cohort attempts), validation and test untouched, threshold rule of the main stage; (iii) final-attempt evaluation: FPR and precision against final normal attempts only (positives exist only among final attempts). Mean over seeds [95 % CI], %.

Check: the `main protocol` rows reproduce `prevalence.json` (max |Δ recall| = 0.0 over 876 seed × cells).

Share of train-cohort attempts that are cheaters, i.e. presented as normal under (ii) contaminated fitting (mean over seeds): LEGACY 1.65 % (range 1.47–1.86); SIM-FI 1.64 % (range 1.36–1.92); SIM-BL 1.64 % (range 1.36–1.92).

## Recall, main protocol

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 45.1 [38.0, 52.8] | 47.0 [43.2, 50.8] | 23.5 [19.9, 27.4] |
| IF + peers | S1 rapid_response↔preknowledge | 44.4 [33.8, 55.3] | 27.1 [20.1, 34.3] | 13.6 [7.7, 20.3] |
| IF + peers | S2 excessive_changes↔lookup | 39.9 [29.8, 50.3] | 53.7 [42.8, 64.1] | 10.9 [5.0, 17.1] |
| IF + peers | S3 collusion↔copy_peer | 67.4 [54.6, 79.4] | 99.5 [98.4, 100.0] | 65.9 [53.2, 77.6] |
| IF + peers | S4 score_jump↔proxy | 8.6 [2.0, 16.4] | 3.2 [0.0, 7.4] | 5.7 [1.5, 10.7] |
| IF + peers | realized FPR (all attempts) | 3.7 [3.3, 4.2] | 4.0 [3.3, 4.8] | 3.3 [2.5, 4.1] |
| IF | all | 37.1 [30.2, 44.9] | 39.7 [33.0, 46.7] | 14.8 [11.6, 18.0] |
| IF | S1 rapid_response↔preknowledge | 56.8 [47.3, 66.6] | 47.2 [39.4, 54.8] | 18.4 [11.9, 25.1] |
| IF | S2 excessive_changes↔lookup | 58.4 [46.2, 69.9] | 81.7 [72.8, 90.2] | 15.8 [9.2, 23.0] |
| IF | S3 collusion↔copy_peer | 1.4 [0.0, 4.2] | 11.6 [2.1, 24.2] | 16.0 [9.2, 23.5] |
| IF | S4 score_jump↔proxy | 3.9 [0.0, 9.8] | 0.8 [0.0, 2.5] | 4.2 [0.0, 9.2] |
| IF | realized FPR (all attempts) | 3.3 [2.8, 3.8] | 3.8 [3.2, 4.4] | 4.2 [3.4, 5.0] |
| Rules | all | 70.1 [64.2, 76.4] | 55.0 [48.5, 61.0] | 32.3 [27.7, 37.1] |
| Rules | S1 rapid_response↔preknowledge | 48.9 [36.6, 61.8] | 24.5 [16.9, 32.4] | 22.0 [14.7, 29.6] |
| Rules | S2 excessive_changes↔lookup | 97.6 [93.6, 100.0] | 88.3 [79.0, 95.7] | 19.9 [11.0, 30.1] |
| Rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 82.5 [70.5, 92.9] |
| Rules | S4 score_jump↔proxy | 9.6 [3.4, 16.2] | 5.7 [0.7, 11.7] | 6.4 [2.3, 11.5] |
| Rules | realized FPR (all attempts) | 3.3 [2.5, 4.2] | 3.2 [2.4, 4.1] | 3.7 [3.0, 4.5] |

## Recall, (i) label-free threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 50.6 [44.2, 57.5] | 51.2 [46.9, 55.6] | 28.0 [24.2, 31.9] |
| IF + peers | S1 rapid_response↔preknowledge | 47.4 [37.0, 58.2] | 34.4 [28.2, 40.7] | 19.9 [12.0, 28.3] |
| IF + peers | S2 excessive_changes↔lookup | 43.6 [33.2, 54.2] | 58.5 [47.2, 69.5] | 13.2 [7.4, 19.1] |
| IF + peers | S3 collusion↔copy_peer | 81.7 [72.9, 89.8] | 100.0 [100.0, 100.0] | 73.5 [60.4, 84.8] |
| IF + peers | S4 score_jump↔proxy | 8.6 [2.0, 16.4] | 3.2 [0.0, 7.4] | 7.4 [2.4, 13.2] |
| IF + peers | realized FPR (all attempts) | 5.2 [4.8, 5.6] | 5.2 [4.6, 5.8] | 5.2 [4.6, 5.8] |
| IF | all | 43.3 [37.3, 50.2] | 45.2 [39.9, 50.9] | 18.2 [15.4, 20.9] |
| IF | S1 rapid_response↔preknowledge | 63.2 [56.3, 70.6] | 54.1 [47.0, 60.8] | 22.5 [15.8, 29.4] |
| IF | S2 excessive_changes↔lookup | 69.3 [58.5, 79.2] | 90.7 [85.6, 95.4] | 17.3 [10.6, 24.4] |
| IF | S3 collusion↔copy_peer | 1.4 [0.0, 4.2] | 15.6 [4.7, 28.6] | 22.8 [14.0, 34.2] |
| IF | S4 score_jump↔proxy | 7.6 [2.0, 14.2] | 0.8 [0.0, 2.5] | 4.2 [0.0, 9.2] |
| IF | realized FPR (all attempts) | 4.9 [4.5, 5.4] | 5.4 [4.8, 5.9] | 5.4 [4.8, 5.9] |
| Rules | all | 75.4 [70.0, 80.6] | 61.7 [58.0, 65.4] | 36.8 [31.7, 42.1] |
| Rules | S1 rapid_response↔preknowledge | 62.3 [53.4, 71.4] | 34.2 [25.9, 42.5] | 28.9 [20.3, 38.2] |
| Rules | S2 excessive_changes↔lookup | 98.3 [95.0, 100.0] | 97.3 [94.2, 100.0] | 28.6 [17.5, 41.1] |
| Rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 84.4 [72.8, 94.2] |
| Rules | S4 score_jump↔proxy | 13.7 [5.9, 22.1] | 10.2 [3.8, 17.4] | 6.4 [2.3, 11.5] |
| Rules | realized FPR (all attempts) | 5.5 [5.0, 6.0] | 5.5 [4.9, 6.2] | 5.6 [4.9, 6.2] |

## Recall, (ii) contaminated fitting

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 49.3 [42.6, 56.5] | 47.5 [42.6, 52.3] | 22.8 [18.8, 27.0] |
| IF + peers | S1 rapid_response↔preknowledge | 44.5 [33.7, 56.0] | 25.9 [20.6, 31.0] | 13.9 [8.0, 20.6] |
| IF + peers | S2 excessive_changes↔lookup | 45.0 [36.1, 53.6] | 59.8 [49.1, 70.1] | 8.5 [3.8, 13.9] |
| IF + peers | S3 collusion↔copy_peer | 84.3 [73.8, 93.1] | 100.0 [100.0, 100.0] | 68.1 [54.0, 80.8] |
| IF + peers | S4 score_jump↔proxy | 5.6 [0.0, 13.0] | 0.8 [0.0, 2.5] | 5.7 [1.5, 10.7] |
| IF + peers | realized FPR (all attempts) | 3.7 [3.1, 4.3] | 3.4 [2.8, 4.0] | 4.0 [3.2, 4.6] |
| IF | all | 40.6 [33.6, 48.2] | 36.9 [30.6, 43.7] | 13.5 [10.6, 16.6] |
| IF | S1 rapid_response↔preknowledge | 57.6 [48.2, 67.2] | 43.6 [35.3, 51.5] | 18.3 [11.9, 25.2] |
| IF | S2 excessive_changes↔lookup | 69.7 [57.5, 80.6] | 78.4 [69.1, 87.2] | 12.3 [7.0, 17.8] |
| IF | S3 collusion↔copy_peer | 0.0 [0.0, 0.0] | 6.3 [0.0, 14.2] | 13.6 [7.7, 20.1] |
| IF | S4 score_jump↔proxy | 4.9 [0.0, 10.8] | 0.8 [0.0, 2.5] | 3.3 [0.0, 8.3] |
| IF | realized FPR (all attempts) | 3.7 [3.3, 4.1] | 3.8 [3.1, 4.4] | 3.6 [3.0, 4.2] |
| Rules | all | 65.7 [58.7, 72.6] | 55.2 [50.0, 60.2] | 28.5 [23.1, 34.0] |
| Rules | S1 rapid_response↔preknowledge | 49.9 [39.3, 60.3] | 27.2 [19.1, 35.6] | 18.5 [11.5, 26.0] |
| Rules | S2 excessive_changes↔lookup | 85.1 [72.8, 95.1] | 84.8 [75.1, 93.2] | 17.6 [9.0, 28.0] |
| Rules | S3 collusion↔copy_peer | 95.4 [90.3, 100.0] | 94.7 [86.8, 100.0] | 73.3 [58.4, 86.8] |
| Rules | S4 score_jump↔proxy | 10.5 [4.4, 17.2] | 10.2 [3.8, 17.5] | 6.4 [2.3, 11.5] |
| Rules | realized FPR (all attempts) | 4.0 [3.5, 4.5] | 4.5 [3.9, 5.1] | 3.9 [3.2, 4.5] |

## Δ = recall(SIM-FI) − recall(SIM-BL), pp, paired by seed

Exact sign-flip permutation p, unadjusted (exploratory).

| Detector | Subset | Δ main protocol | p | Δ (i) label-free threshold | p | Δ (ii) contaminated fitting | p |
|---|---|---:|---:|---:|---:|---:|---:|
| IF + peers | all | 23.5 [18.5, 28.8] | <0.0001 | 23.2 [19.0, 27.4] | <0.0001 | 24.7 [19.8, 30.1] | <0.0001 |
| IF + peers | S1 rapid_response↔preknowledge | 13.5 [5.3, 21.9] | 0.0065 | 14.5 [7.2, 21.9] | 0.0015 | 11.9 [5.7, 18.2] | 0.0014 |
| IF + peers | S2 excessive_changes↔lookup | 42.8 [31.2, 53.4] | <0.0001 | 45.4 [33.7, 56.2] | <0.0001 | 51.2 [41.3, 60.5] | <0.0001 |
| IF + peers | S3 collusion↔copy_peer | 33.6 [22.0, 46.1] | <0.0001 | 26.5 [15.2, 39.6] | 0.0002 | 31.9 [19.2, 46.0] | 0.0001 |
| IF + peers | S4 score_jump↔proxy | -2.5 [-8.9, 3.5] | 0.5312 | -4.2 [-10.8, 2.4] | 0.3125 | -4.9 [-9.9, -0.8] | 0.1250 |
| IF | all | 24.9 [17.6, 32.3] | <0.0001 | 27.0 [21.4, 33.0] | <0.0001 | 23.5 [17.5, 29.6] | <0.0001 |
| IF | S1 rapid_response↔preknowledge | 28.8 [19.4, 38.8] | <0.0001 | 31.6 [23.4, 40.1] | <0.0001 | 25.3 [16.2, 34.6] | <0.0001 |
| IF | S2 excessive_changes↔lookup | 65.9 [54.9, 76.3] | <0.0001 | 73.4 [64.0, 82.3] | <0.0001 | 66.1 [55.4, 75.8] | <0.0001 |
| IF | S3 collusion↔copy_peer | -4.4 [-18.3, 11.8] | 0.6090 | -7.2 [-24.3, 10.5] | 0.4546 | -7.3 [-16.6, 2.4] | 0.1689 |
| IF | S4 score_jump↔proxy | -3.3 [-8.3, 0.8] | 0.3750 | -3.3 [-8.3, 0.8] | 0.3750 | -2.5 [-7.5, 1.7] | 0.5000 |
| Rules | all | 22.8 [15.2, 30.0] | <0.0001 | 24.9 [19.2, 30.9] | <0.0001 | 26.7 [17.7, 35.6] | <0.0001 |
| Rules | S1 rapid_response↔preknowledge | 2.5 [-7.9, 13.2] | 0.6507 | 5.3 [-8.5, 18.5] | 0.4650 | 8.6 [-3.5, 21.1] | 0.1955 |
| Rules | S2 excessive_changes↔lookup | 68.4 [52.3, 81.2] | <0.0001 | 68.7 [56.2, 80.1] | <0.0001 | 67.2 [50.8, 80.3] | <0.0001 |
| Rules | S3 collusion↔copy_peer | 17.5 [7.1, 29.5] | 0.0078 | 15.6 [5.8, 27.2] | 0.0078 | 21.4 [5.0, 38.5] | 0.0283 |
| Rules | S4 score_jump↔proxy | -0.7 [-7.5, 6.7] | 0.9062 | 3.7 [-5.0, 12.5] | 0.4922 | 3.7 [-5.0, 12.9] | 0.4922 |

## (iii) Final-attempt evaluation, main protocol

FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts vs final test normal attempts only. Recall is unchanged (all positives are final attempts). Slot columns: precision of the slot's cheaters vs final normal attempts.

| Detector | Condition | FPR, all attempts | FPR, final attempts | precision, all | precision, final | precision final, S1 | precision final, S2 | precision final, S3 | precision final, S4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| IF + peers | LEGACY | 3.7 [3.3, 4.2] | 2.9 [2.4, 3.4] | 18.1 [15.5, 20.6] | 46.3 [41.0, 51.2] | 23.0 [18.7, 27.2] | 19.4 [14.1, 24.7] | 25.0 [19.0, 31.2] | 3.2 [0.9, 5.7] |
| IF + peers | SIM-FI | 4.0 [3.3, 4.8] | 3.2 [2.5, 3.9] | 19.7 [15.1, 25.0] | 47.1 [39.1, 55.6] | 15.9 [10.8, 21.5] | 21.8 [14.3, 30.7] | 34.5 [26.0, 44.0] | 1.1 [0.0, 2.2] |
| IF + peers | SIM-BL | 3.3 [2.5, 4.1] | 2.6 [2.0, 3.2] | 17.4 [10.6, 26.5] | 39.9 [29.7, 51.4] | 13.4 [5.7, 23.1] | 6.6 [2.3, 12.6] | 33.7 [23.4, 45.3] | 3.2 [0.9, 6.3] |
| IF | LEGACY | 3.3 [2.8, 3.8] | 2.1 [1.7, 2.5] | 17.5 [14.5, 20.6] | 50.4 [43.4, 57.8] | 37.8 [30.1, 47.3] | 32.3 [24.9, 41.4] | 0.6 [0.0, 1.8] | 1.3 [0.0, 3.5] |
| IF | SIM-FI | 3.8 [3.2, 4.4] | 2.7 [2.2, 3.2] | 15.3 [13.6, 17.1] | 44.1 [39.9, 48.5] | 25.3 [21.8, 28.7] | 28.8 [24.2, 33.7] | 4.2 [0.9, 8.2] | 0.6 [0.0, 1.7] |
| IF | SIM-BL | 4.2 [3.4, 5.0] | 3.0 [2.3, 3.7] | 6.6 [5.0, 8.5] | 22.9 [18.5, 27.5] | 10.2 [6.1, 15.6] | 7.6 [4.2, 11.6] | 6.9 [3.9, 10.2] | 1.3 [0.0, 2.9] |
| Rules | LEGACY | 3.3 [2.5, 4.2] | 3.2 [2.4, 4.0] | 34.0 [26.3, 42.6] | 58.0 [50.9, 65.5] | 28.2 [21.8, 34.4] | 37.9 [29.9, 46.6] | 36.1 [27.2, 45.5] | 4.3 [1.1, 8.7] |
| Rules | SIM-FI | 3.2 [2.4, 4.1] | 3.3 [2.3, 4.3] | 27.4 [22.6, 32.8] | 53.0 [45.8, 60.6] | 12.0 [8.5, 15.5] | 32.1 [24.5, 40.7] | 35.8 [27.6, 44.9] | 1.5 [0.3, 3.0] |
| Rules | SIM-BL | 3.7 [3.0, 4.5] | 4.0 [3.3, 4.9] | 15.0 [11.7, 19.0] | 31.9 [26.3, 37.8] | 10.0 [6.7, 13.4] | 6.0 [3.5, 8.7] | 23.2 [18.3, 28.7] | 2.2 [0.7, 4.1] |

## (iii) Final-attempt evaluation, (i) label-free threshold

FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts vs final test normal attempts only. Recall is unchanged (all positives are final attempts). Slot columns: precision of the slot's cheaters vs final normal attempts.

| Detector | Condition | FPR, all attempts | FPR, final attempts | precision, all | precision, final | precision final, S1 | precision final, S2 | precision final, S3 | precision final, S4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| IF + peers | LEGACY | 5.2 [4.8, 5.6] | 3.9 [3.3, 4.7] | 14.6 [13.1, 16.1] | 42.3 [37.9, 46.8] | 20.1 [16.2, 24.1] | 16.6 [11.9, 21.6] | 23.2 [18.8, 28.0] | 2.3 [0.6, 4.3] |
| IF + peers | SIM-FI | 5.2 [4.6, 5.8] | 4.1 [3.5, 4.6] | 15.3 [12.9, 17.7] | 41.5 [35.3, 48.1] | 14.9 [11.1, 19.4] | 17.2 [12.1, 22.8] | 27.1 [21.3, 33.4] | 1.0 [0.0, 2.2] |
| IF + peers | SIM-BL | 5.2 [4.6, 5.8] | 4.2 [3.6, 4.8] | 8.9 [7.2, 10.8] | 28.2 [22.0, 35.1] | 8.1 [4.2, 13.1] | 4.9 [2.5, 7.6] | 20.9 [15.0, 27.4] | 2.1 [0.6, 3.8] |
| IF | LEGACY | 4.9 [4.5, 5.4] | 3.3 [2.8, 3.7] | 13.7 [11.8, 15.8] | 42.5 [37.9, 46.8] | 28.8 [25.1, 33.0] | 24.8 [20.2, 29.2] | 0.4 [0.0, 1.3] | 2.7 [0.7, 5.2] |
| IF | SIM-FI | 5.4 [4.8, 5.9] | 3.9 [3.4, 4.4] | 12.7 [11.2, 14.1] | 38.3 [34.3, 42.5] | 21.1 [18.0, 24.2] | 23.6 [19.3, 28.1] | 4.5 [1.6, 7.9] | 0.2 [0.0, 0.6] |
| IF | SIM-BL | 5.4 [4.8, 5.9] | 3.9 [3.4, 4.4] | 5.6 [4.6, 6.6] | 20.1 [17.3, 22.8] | 9.3 [6.7, 12.1] | 6.0 [3.7, 8.3] | 6.9 [4.9, 9.0] | 1.0 [0.0, 2.2] |
| Rules | LEGACY | 5.5 [5.0, 6.0] | 5.5 [4.9, 6.0] | 19.7 [18.2, 21.4] | 43.3 [40.4, 46.4] | 19.5 [16.0, 23.0] | 22.3 [19.2, 25.8] | 19.6 [16.6, 22.9] | 2.4 [1.0, 3.9] |
| Rules | SIM-FI | 5.5 [4.9, 6.2] | 5.7 [5.0, 6.5] | 16.6 [14.6, 18.8] | 37.2 [33.1, 41.6] | 9.9 [7.5, 12.3] | 18.5 [15.1, 22.3] | 20.2 [15.9, 24.8] | 1.8 [0.7, 3.0] |
| Rules | SIM-BL | 5.6 [4.9, 6.2] | 5.9 [5.2, 6.6] | 10.8 [8.8, 12.8] | 25.8 [21.5, 30.2] | 8.5 [6.0, 11.0] | 5.8 [3.9, 7.7] | 16.6 [13.1, 20.3] | 1.3 [0.5, 2.2] |

## (iii) Final-attempt evaluation, (ii) contaminated fitting

FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts vs final test normal attempts only. Recall is unchanged (all positives are final attempts). Slot columns: precision of the slot's cheaters vs final normal attempts.

| Detector | Condition | FPR, all attempts | FPR, final attempts | precision, all | precision, final | precision final, S1 | precision final, S2 | precision final, S3 | precision final, S4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| IF + peers | LEGACY | 3.7 [3.1, 4.3] | 2.3 [1.8, 2.9] | 20.4 [17.3, 23.9] | 55.2 [50.7, 59.8] | 28.6 [22.9, 34.4] | 26.0 [19.7, 32.5] | 33.7 [28.7, 38.6] | 2.4 [0.0, 5.8] |
| IF + peers | SIM-FI | 3.4 [2.8, 4.0] | 2.4 [1.9, 2.8] | 21.3 [17.8, 25.1] | 54.1 [46.5, 61.6] | 18.5 [13.2, 24.2] | 27.1 [20.4, 34.0] | 40.4 [32.2, 48.8] | 0.5 [0.0, 1.5] |
| IF + peers | SIM-BL | 4.0 [3.2, 4.6] | 3.0 [2.4, 3.6] | 14.1 [7.9, 24.3] | 34.1 [25.5, 44.4] | 9.3 [4.6, 15.2] | 3.8 [1.7, 6.0] | 27.9 [18.8, 39.2] | 1.8 [0.4, 3.3] |
| IF | LEGACY | 3.7 [3.3, 4.1] | 2.3 [2.0, 2.5] | 16.9 [13.8, 20.1] | 48.9 [42.5, 54.7] | 34.3 [28.6, 40.2] | 31.8 [25.8, 37.7] | 0.0 [0.0, 0.0] | 2.2 [0.0, 5.1] |
| IF | SIM-FI | 3.8 [3.1, 4.4] | 2.6 [2.1, 3.1] | 14.9 [12.8, 17.2] | 43.9 [39.5, 48.6] | 24.0 [20.0, 28.1] | 29.7 [24.1, 36.1] | 1.8 [0.0, 3.8] | 0.3 [0.0, 0.8] |
| IF | SIM-BL | 3.6 [3.0, 4.2] | 2.4 [2.0, 2.8] | 6.6 [5.1, 8.4] | 24.1 [19.4, 30.2] | 13.1 [8.5, 18.5] | 7.1 [4.0, 10.3] | 6.0 [3.5, 8.4] | 2.8 [0.0, 8.2] |
| Rules | LEGACY | 4.0 [3.5, 4.5] | 4.1 [3.5, 4.8] | 23.1 [20.4, 26.1] | 47.4 [43.0, 51.8] | 21.6 [16.3, 27.2] | 24.2 [19.3, 29.1] | 24.0 [19.9, 28.4] | 2.8 [1.1, 4.7] |
| Rules | SIM-FI | 4.5 [3.9, 5.1] | 4.6 [4.0, 5.2] | 18.2 [15.6, 20.9] | 39.4 [34.8, 44.3] | 9.5 [6.7, 12.5] | 19.4 [15.9, 22.8] | 23.1 [17.8, 28.9] | 2.2 [0.9, 3.7] |
| Rules | SIM-BL | 3.9 [3.2, 4.5] | 4.2 [3.4, 5.0] | 12.0 [9.3, 14.8] | 28.0 [22.9, 33.1] | 7.5 [4.6, 10.6] | 4.9 [2.7, 7.3] | 20.3 [15.6, 25.0] | 2.0 [0.7, 3.4] |

## H1 contrast on SIM-BL: recall(IF + peers) − recall(Rules), pp, paired by seed

Exact sign-flip permutation p, unadjusted (the pre-specified H1 test is the main-protocol S3/S4 pair with Holm over the 2 slots, `h1.md`; the main-protocol rows here repeat its estimates). H1 decision rule: ≥ 15 pp supported, < 5 pp rules adopted.

| Protocol | Subset | IF + peers recall | Rules recall | gain [95 % CI] | p |
|---|---|---:|---:|---:|---:|
| main protocol | S3 collusion↔copy_peer | 65.9 [53.2, 77.6] | 82.5 [70.5, 92.9] | -16.6 [-25.9, -7.6] | 0.0037 |
| main protocol | S4 score_jump↔proxy | 5.7 [1.5, 10.7] | 6.4 [2.3, 11.5] | -0.7 [-5.7, 4.3] | 1.0000 |
| main protocol | all | 23.5 [19.9, 27.4] | 32.3 [27.7, 37.1] | -8.7 [-13.2, -4.4] | 0.0013 |
| (i) label-free threshold | S3 collusion↔copy_peer | 73.5 [60.4, 84.8] | 84.4 [72.8, 94.2] | -10.9 [-19.1, -3.3] | 0.0186 |
| (i) label-free threshold | S4 score_jump↔proxy | 7.4 [2.4, 13.2] | 6.4 [2.3, 11.5] | 1.0 [-2.1, 5.0] | 1.0000 |
| (i) label-free threshold | all | 28.0 [24.2, 31.9] | 36.8 [31.7, 42.1] | -8.9 [-12.8, -5.0] | 0.0005 |
| (ii) contaminated fitting | S3 collusion↔copy_peer | 68.1 [54.0, 80.8] | 73.3 [58.4, 86.8] | -5.2 [-14.5, 4.6] | 0.3296 |
| (ii) contaminated fitting | S4 score_jump↔proxy | 5.7 [1.5, 10.7] | 6.4 [2.3, 11.5] | -0.7 [-5.7, 4.3] | 1.0000 |
| (ii) contaminated fitting | all | 22.8 [18.8, 27.0] | 28.5 [23.1, 34.0] | -5.7 [-10.9, -0.9] | 0.0403 |
