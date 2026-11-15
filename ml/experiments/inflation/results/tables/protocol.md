# Post hoc: robustness of the evaluation protocol (exploratory)

**Added after independent review (amendment of 15 Nov 2026).** Main configuration, seeds 1001–1020, frozen detectors. (i) label-free threshold: `numpy.quantile` (linear interpolation) at 0.95 of the scores of validation normal attempts, no positives used; (ii) contaminated fitting: train-cohort cheaters presented to the frozen pipeline as normal (reference statistics and models fitted on all train-cohort attempts), validation and test untouched, threshold rule of the main stage; (iii) final-attempt evaluation: FPR and precision against final normal attempts only (positives exist only among final attempts). Mean over seeds [95 % CI], %.

Check: the `main protocol` rows reproduce `main.json` (max |Δ recall| = 0.0 over 900 seed × cells).

Share of train-cohort attempts that are cheaters, i.e. presented as normal under (ii) contaminated fitting (mean over seeds): LEGACY 12.05 % (range 11.17–12.39); SIM-FI 12.00 % (range 11.25–12.61); SIM-BL 12.00 % (range 11.25–12.61).

## Recall, main protocol

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 48.6 [46.7, 50.5] | 53.6 [51.9, 55.3] | 27.8 [25.8, 29.7] |
| IF + peers | S1 rapid_response↔preknowledge | 38.6 [35.0, 42.1] | 35.4 [32.5, 38.4] | 24.2 [22.1, 26.4] |
| IF + peers | S2 excessive_changes↔lookup | 50.0 [46.6, 53.4] | 68.0 [63.5, 72.4] | 14.4 [11.7, 17.2] |
| IF + peers | S3 collusion↔copy_peer | 88.8 [84.9, 92.5] | 100.0 [100.0, 100.0] | 61.0 [56.5, 65.4] |
| IF + peers | S4 score_jump↔proxy | 6.6 [4.1, 9.1] | 5.1 [3.1, 7.2] | 9.5 [7.8, 11.3] |
| IF + peers | realized FPR (all attempts) | 5.1 [4.7, 5.5] | 5.1 [4.6, 5.7] | 5.0 [4.4, 5.6] |
| IF | all | 40.6 [38.2, 42.9] | 46.3 [43.9, 48.5] | 22.9 [21.0, 24.9] |
| IF | S1 rapid_response↔preknowledge | 57.9 [54.0, 61.8] | 55.6 [52.5, 58.5] | 29.8 [27.4, 32.3] |
| IF | S2 excessive_changes↔lookup | 74.1 [70.7, 77.4] | 91.1 [88.9, 93.2] | 23.2 [20.7, 26.0] |
| IF | S3 collusion↔copy_peer | 1.4 [0.5, 2.4] | 11.0 [7.7, 14.5] | 21.5 [18.7, 24.3] |
| IF | S4 score_jump↔proxy | 6.3 [4.4, 8.3] | 5.6 [3.6, 8.1] | 8.3 [5.8, 11.1] |
| IF | realized FPR (all attempts) | 4.9 [4.5, 5.3] | 5.3 [4.8, 6.0] | 5.3 [4.7, 6.0] |
| Rules | all | 68.6 [66.8, 70.4] | 61.9 [59.8, 64.0] | 36.2 [33.0, 39.3] |
| Rules | S1 rapid_response↔preknowledge | 49.7 [46.1, 53.2] | 34.1 [29.9, 38.2] | 34.3 [29.9, 38.6] |
| Rules | S2 excessive_changes↔lookup | 99.0 [98.5, 99.6] | 97.8 [96.7, 98.8] | 28.0 [23.1, 33.0] |
| Rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 65.5 [60.1, 70.9] |
| Rules | S4 score_jump↔proxy | 14.0 [10.5, 17.6] | 12.3 [8.2, 17.4] | 10.7 [7.3, 14.2] |
| Rules | realized FPR (all attempts) | 4.2 [3.9, 4.6] | 5.3 [4.7, 6.0] | 5.2 [4.5, 5.8] |

## Recall, (i) label-free threshold

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 49.5 [47.5, 51.5] | 54.3 [52.4, 56.2] | 28.6 [26.5, 30.6] |
| IF + peers | S1 rapid_response↔preknowledge | 39.6 [36.0, 43.2] | 36.0 [33.0, 39.1] | 25.0 [22.7, 27.4] |
| IF + peers | S2 excessive_changes↔lookup | 51.1 [47.7, 54.5] | 69.3 [64.7, 73.8] | 15.3 [12.2, 18.5] |
| IF + peers | S3 collusion↔copy_peer | 89.9 [86.0, 93.4] | 100.0 [100.0, 100.0] | 61.9 [57.6, 65.9] |
| IF + peers | S4 score_jump↔proxy | 6.8 [4.3, 9.3] | 5.8 [3.7, 7.9] | 10.1 [8.4, 11.8] |
| IF + peers | realized FPR (all attempts) | 5.2 [4.9, 5.6] | 5.5 [4.9, 6.1] | 5.4 [4.8, 6.1] |
| IF | all | 41.5 [39.0, 43.9] | 46.9 [44.7, 49.1] | 24.0 [22.2, 25.9] |
| IF | S1 rapid_response↔preknowledge | 58.7 [54.6, 62.8] | 56.3 [53.5, 59.2] | 30.5 [28.3, 32.7] |
| IF | S2 excessive_changes↔lookup | 75.4 [72.1, 78.6] | 91.7 [89.4, 93.8] | 24.9 [22.1, 27.8] |
| IF | S3 collusion↔copy_peer | 2.1 [1.2, 3.1] | 11.9 [8.6, 15.4] | 23.4 [20.6, 26.1] |
| IF | S4 score_jump↔proxy | 7.2 [5.1, 9.3] | 5.6 [3.6, 8.1] | 8.8 [6.1, 11.7] |
| IF | realized FPR (all attempts) | 5.2 [4.8, 5.6] | 5.5 [4.9, 6.2] | 5.5 [4.9, 6.2] |
| Rules | all | 69.5 [67.7, 71.2] | 62.9 [61.0, 64.9] | 38.4 [35.1, 41.4] |
| Rules | S1 rapid_response↔preknowledge | 51.2 [47.6, 54.7] | 36.0 [32.3, 39.9] | 37.8 [32.9, 42.6] |
| Rules | S2 excessive_changes↔lookup | 99.0 [98.5, 99.6] | 97.9 [96.8, 98.9] | 30.0 [25.0, 34.9] |
| Rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] | 100.0 [100.0, 100.0] | 66.9 [61.7, 72.1] |
| Rules | S4 score_jump↔proxy | 16.5 [13.8, 19.5] | 14.1 [9.9, 19.1] | 11.6 [8.5, 14.8] |
| Rules | realized FPR (all attempts) | 5.0 [4.6, 5.5] | 5.8 [5.1, 6.5] | 5.8 [5.1, 6.5] |

## Recall, (ii) contaminated fitting

| Detector | Subset | LEGACY | SIM-FI | SIM-BL |
|---|---|---:|---:|---:|
| IF + peers | all | 43.1 [40.7, 45.4] | 46.3 [44.2, 48.5] | 23.6 [21.6, 25.5] |
| IF + peers | S1 rapid_response↔preknowledge | 30.1 [26.9, 33.4] | 26.1 [23.0, 29.1] | 18.5 [16.1, 21.0] |
| IF + peers | S2 excessive_changes↔lookup | 49.3 [45.6, 53.0] | 55.8 [50.9, 60.9] | 10.5 [8.1, 13.0] |
| IF + peers | S3 collusion↔copy_peer | 78.7 [73.7, 83.7] | 97.9 [96.1, 99.4] | 57.8 [52.9, 62.7] |
| IF + peers | S4 score_jump↔proxy | 5.2 [3.2, 7.2] | 2.9 [1.5, 4.6] | 6.4 [4.5, 8.4] |
| IF + peers | realized FPR (all attempts) | 5.2 [4.8, 5.6] | 4.8 [4.3, 5.3] | 5.0 [4.5, 5.4] |
| IF | all | 32.7 [30.5, 34.9] | 36.2 [34.7, 37.6] | 15.7 [14.1, 17.3] |
| IF | S1 rapid_response↔preknowledge | 39.1 [35.4, 42.5] | 37.5 [35.7, 39.4] | 21.1 [18.8, 23.7] |
| IF | S2 excessive_changes↔lookup | 70.8 [67.0, 74.4] | 80.1 [77.0, 83.4] | 14.5 [12.1, 17.1] |
| IF | S3 collusion↔copy_peer | 0.4 [0.0, 1.0] | 8.3 [5.8, 11.0] | 15.9 [13.5, 18.2] |
| IF | S4 score_jump↔proxy | 5.5 [3.8, 7.2] | 5.1 [2.7, 7.8] | 5.6 [3.7, 7.5] |
| IF | realized FPR (all attempts) | 4.8 [4.4, 5.3] | 5.2 [4.7, 5.8] | 5.0 [4.3, 5.7] |
| Rules | all | 28.9 [26.1, 31.9] | 26.1 [23.6, 28.6] | 20.6 [18.3, 22.9] |
| Rules | S1 rapid_response↔preknowledge | 25.8 [22.4, 28.9] | 18.6 [15.3, 21.7] | 20.0 [17.5, 22.7] |
| Rules | S2 excessive_changes↔lookup | 30.5 [27.1, 34.4] | 31.0 [27.6, 34.3] | 17.2 [13.9, 20.6] |
| Rules | S3 collusion↔copy_peer | 38.9 [30.4, 47.2] | 39.4 [31.3, 47.7] | 36.4 [30.7, 42.6] |
| Rules | S4 score_jump↔proxy | 17.0 [13.1, 21.2] | 15.6 [11.1, 21.0] | 4.3 [2.5, 6.0] |
| Rules | realized FPR (all attempts) | 4.7 [4.3, 5.2] | 5.4 [4.8, 5.9] | 5.2 [4.6, 5.8] |

## Δ = recall(SIM-FI) − recall(SIM-BL), pp, paired by seed

Exact sign-flip permutation p, unadjusted (exploratory).

| Detector | Subset | Δ main protocol | p | Δ (i) label-free threshold | p | Δ (ii) contaminated fitting | p |
|---|---|---:|---:|---:|---:|---:|---:|
| IF + peers | all | 25.8 [23.9, 27.7] | <0.0001 | 25.7 [23.8, 27.6] | <0.0001 | 22.8 [20.6, 25.0] | <0.0001 |
| IF + peers | S1 rapid_response↔preknowledge | 11.2 [8.0, 14.7] | <0.0001 | 11.0 [7.5, 14.5] | <0.0001 | 7.6 [4.4, 10.8] | 0.0002 |
| IF + peers | S2 excessive_changes↔lookup | 53.6 [48.8, 58.3] | <0.0001 | 54.1 [49.2, 59.0] | <0.0001 | 45.3 [39.6, 50.9] | <0.0001 |
| IF + peers | S3 collusion↔copy_peer | 39.0 [34.6, 43.5] | <0.0001 | 38.1 [34.1, 42.4] | <0.0001 | 40.1 [35.0, 45.3] | <0.0001 |
| IF + peers | S4 score_jump↔proxy | -4.4 [-6.6, -2.2] | 0.0017 | -4.4 [-6.9, -1.8] | 0.0053 | -3.4 [-5.8, -1.2] | 0.0125 |
| IF | all | 23.4 [21.0, 25.8] | <0.0001 | 22.9 [20.6, 25.1] | <0.0001 | 20.5 [18.7, 22.3] | <0.0001 |
| IF | S1 rapid_response↔preknowledge | 25.7 [23.0, 28.4] | <0.0001 | 25.9 [23.3, 28.5] | <0.0001 | 16.4 [14.1, 18.7] | <0.0001 |
| IF | S2 excessive_changes↔lookup | 67.9 [64.7, 71.0] | <0.0001 | 66.8 [63.0, 70.4] | <0.0001 | 65.6 [61.9, 69.5] | <0.0001 |
| IF | S3 collusion↔copy_peer | -10.5 [-14.3, -6.7] | <0.0001 | -11.5 [-15.0, -8.0] | <0.0001 | -7.6 [-10.7, -4.4] | 0.0004 |
| IF | S4 score_jump↔proxy | -2.7 [-5.5, 0.1] | 0.0791 | -3.1 [-6.2, -0.2] | 0.0596 | -0.5 [-3.3, 2.4] | 0.7247 |
| Rules | all | 25.8 [23.0, 28.6] | <0.0001 | 24.6 [21.9, 27.4] | <0.0001 | 5.5 [2.9, 8.2] | 0.0009 |
| Rules | S1 rapid_response↔preknowledge | -0.2 [-6.1, 5.3] | 0.9366 | -1.8 [-7.9, 4.0] | 0.5629 | -1.4 [-5.2, 2.3] | 0.5014 |
| Rules | S2 excessive_changes↔lookup | 69.8 [64.9, 74.6] | <0.0001 | 67.9 [63.1, 72.8] | <0.0001 | 13.7 [9.9, 17.8] | <0.0001 |
| Rules | S3 collusion↔copy_peer | 34.5 [29.1, 39.9] | <0.0001 | 33.1 [27.9, 38.3] | <0.0001 | 3.0 [-4.7, 11.4] | 0.4923 |
| Rules | S4 score_jump↔proxy | 1.6 [-3.3, 6.9] | 0.5579 | 2.5 [-2.3, 7.7] | 0.3453 | 11.3 [6.4, 16.8] | 0.0003 |

## (iii) Final-attempt evaluation, main protocol

FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts vs final test normal attempts only. Recall is unchanged (all positives are final attempts). Slot columns: precision of the slot's cheaters vs final normal attempts.

| Detector | Condition | FPR, all attempts | FPR, final attempts | precision, all | precision, final | precision final, S1 | precision final, S2 | precision final, S3 | precision final, S4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| IF + peers | LEGACY | 5.1 [4.7, 5.5] | 4.4 [3.7, 5.1] | 56.5 [54.7, 58.4] | 86.1 [84.2, 88.0] | 63.6 [59.4, 67.5] | 62.1 [58.3, 66.2] | 72.6 [68.9, 76.3] | 11.9 [7.2, 16.9] |
| IF + peers | SIM-FI | 5.1 [4.6, 5.7] | 4.8 [4.1, 5.5] | 58.7 [56.4, 61.1] | 86.1 [84.2, 88.0] | 59.6 [55.7, 63.4] | 66.9 [63.2, 70.5] | 72.2 [68.3, 76.2] | 8.1 [4.9, 11.3] |
| IF + peers | SIM-BL | 5.0 [4.4, 5.6] | 4.6 [3.9, 5.2] | 43.3 [40.3, 46.3] | 76.9 [74.1, 79.7] | 51.3 [47.7, 55.1] | 30.5 [25.4, 35.7] | 62.4 [57.7, 67.2] | 16.3 [13.1, 19.8] |
| IF | LEGACY | 4.9 [4.5, 5.3] | 3.7 [3.2, 4.2] | 53.0 [50.8, 55.4] | 85.8 [84.0, 87.6] | 75.5 [72.8, 78.1] | 73.6 [70.3, 77.0] | 4.4 [1.7, 7.3] | 13.6 [9.1, 18.2] |
| IF | SIM-FI | 5.3 [4.8, 6.0] | 4.3 [3.7, 4.9] | 54.1 [51.5, 56.7] | 85.5 [83.8, 87.1] | 71.6 [68.8, 74.4] | 74.8 [71.8, 77.6] | 22.4 [17.2, 27.4] | 10.5 [6.9, 14.6] |
| IF | SIM-BL | 5.3 [4.7, 6.0] | 4.4 [3.8, 5.0] | 36.9 [34.1, 39.7] | 74.2 [71.3, 77.0] | 57.3 [53.4, 61.5] | 43.1 [39.0, 47.1] | 38.7 [33.5, 44.0] | 13.7 [9.9, 17.6] |
| Rules | LEGACY | 4.2 [3.9, 4.6] | 4.3 [3.7, 4.8] | 68.8 [66.7, 70.7] | 89.9 [88.6, 91.2] | 69.7 [66.6, 72.6] | 76.6 [74.0, 79.3] | 75.2 [71.8, 78.6] | 21.7 [16.9, 26.3] |
| Rules | SIM-FI | 5.3 [4.7, 6.0] | 5.9 [5.3, 6.6] | 61.5 [58.7, 64.3] | 85.2 [83.7, 86.6] | 52.5 [49.5, 55.4] | 69.7 [66.5, 72.9] | 67.8 [64.6, 71.0] | 14.6 [10.9, 18.9] |
| Rules | SIM-BL | 5.2 [4.5, 5.8] | 5.9 [5.2, 6.7] | 48.7 [46.0, 51.4] | 76.9 [74.5, 79.3] | 52.6 [48.5, 56.8] | 39.3 [33.7, 44.9] | 58.1 [54.4, 61.8] | 13.3 [9.3, 17.4] |

## (iii) Final-attempt evaluation, (i) label-free threshold

FPR and precision at the validation-frozen 5 % threshold with negatives = all test normal attempts vs final test normal attempts only. Recall is unchanged (all positives are final attempts). Slot columns: precision of the slot's cheaters vs final normal attempts.

| Detector | Condition | FPR, all attempts | FPR, final attempts | precision, all | precision, final | precision final, S1 | precision final, S2 | precision final, S3 | precision final, S4 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| IF + peers | LEGACY | 5.2 [4.9, 5.6] | 4.6 [4.0, 5.2] | 56.2 [54.4, 58.1] | 85.8 [84.0, 87.5] | 63.0 [59.1, 66.5] | 61.4 [57.7, 65.3] | 71.9 [68.5, 75.4] | 11.6 [7.3, 16.0] |
| IF + peers | SIM-FI | 5.5 [4.9, 6.1] | 5.1 [4.3, 5.8] | 57.7 [55.2, 60.2] | 85.6 [83.7, 87.5] | 58.7 [54.9, 62.4] | 66.1 [62.4, 69.7] | 71.2 [67.1, 75.2] | 8.3 [5.3, 11.6] |
| IF + peers | SIM-BL | 5.4 [4.8, 6.1] | 5.0 [4.4, 5.7] | 42.0 [39.0, 45.0] | 75.7 [72.9, 78.4] | 49.7 [46.4, 53.0] | 29.4 [24.3, 34.6] | 60.6 [56.2, 65.2] | 15.8 [12.8, 19.2] |
| IF | LEGACY | 5.2 [4.8, 5.6] | 3.9 [3.4, 4.5] | 52.0 [50.0, 54.2] | 85.4 [83.6, 87.3] | 74.7 [72.0, 77.3] | 72.9 [69.5, 76.4] | 5.9 [3.3, 8.7] | 14.3 [9.9, 18.7] |
| IF | SIM-FI | 5.5 [4.9, 6.2] | 4.5 [3.9, 5.2] | 53.6 [50.9, 56.2] | 85.2 [83.4, 86.9] | 71.2 [68.2, 74.1] | 74.1 [70.9, 77.2] | 23.5 [18.3, 28.4] | 10.3 [6.7, 14.3] |
| IF | SIM-BL | 5.5 [4.9, 6.2] | 4.5 [3.9, 5.2] | 37.2 [34.4, 39.9] | 74.5 [71.6, 77.4] | 57.2 [53.1, 61.5] | 44.0 [39.6, 48.3] | 40.1 [34.9, 45.2] | 14.0 [10.0, 18.0] |
| Rules | LEGACY | 5.0 [4.6, 5.5] | 5.1 [4.4, 5.8] | 65.4 [63.2, 67.5] | 88.4 [86.8, 89.9] | 66.6 [63.4, 69.7] | 73.4 [70.5, 76.4] | 71.9 [68.3, 75.6] | 22.4 [19.0, 26.1] |
| Rules | SIM-FI | 5.8 [5.1, 6.5] | 6.4 [5.6, 7.1] | 59.7 [57.1, 62.6] | 84.5 [82.8, 86.1] | 52.5 [49.6, 55.3] | 68.3 [64.8, 71.7] | 66.3 [62.8, 69.8] | 15.9 [11.9, 20.3] |
| Rules | SIM-BL | 5.8 [5.1, 6.5] | 6.4 [5.6, 7.2] | 47.3 [44.6, 50.0] | 76.5 [74.2, 78.9] | 52.9 [48.8, 57.2] | 39.2 [33.4, 45.0] | 56.8 [53.5, 60.0] | 13.6 [10.0, 17.2] |

## H1 contrast on SIM-BL: recall(IF + peers) − recall(Rules), pp, paired by seed

Exact sign-flip permutation p, unadjusted (the pre-specified H1 test is the main-protocol S3/S4 pair with Holm over the 2 slots, `h1.md`; the main-protocol rows here repeat its estimates). H1 decision rule: ≥ 15 pp supported, < 5 pp rules adopted.

| Protocol | Subset | IF + peers recall | Rules recall | gain [95 % CI] | p |
|---|---|---:|---:|---:|---:|
| main protocol | S3 collusion↔copy_peer | 61.0 [56.5, 65.4] | 65.5 [60.1, 70.9] | -4.5 [-7.4, -1.2] | 0.0135 |
| main protocol | S4 score_jump↔proxy | 9.5 [7.8, 11.3] | 10.7 [7.3, 14.2] | -1.2 [-3.8, 1.4] | 0.3900 |
| main protocol | all | 27.8 [25.8, 29.7] | 36.2 [33.0, 39.3] | -8.3 [-11.0, -5.7] | <0.0001 |
| (i) label-free threshold | S3 collusion↔copy_peer | 61.9 [57.6, 65.9] | 66.9 [61.7, 72.1] | -5.0 [-8.4, -1.2] | 0.0173 |
| (i) label-free threshold | S4 score_jump↔proxy | 10.1 [8.4, 11.8] | 11.6 [8.5, 14.8] | -1.4 [-3.6, 0.6] | 0.1982 |
| (i) label-free threshold | all | 28.6 [26.5, 30.6] | 38.4 [35.1, 41.4] | -9.8 [-12.6, -6.9] | <0.0001 |
| (ii) contaminated fitting | S3 collusion↔copy_peer | 57.8 [52.9, 62.7] | 36.4 [30.7, 42.6] | 21.4 [15.6, 26.6] | <0.0001 |
| (ii) contaminated fitting | S4 score_jump↔proxy | 6.4 [4.5, 8.4] | 4.3 [2.5, 6.0] | 2.1 [-0.0, 4.0] | 0.0608 |
| (ii) contaminated fitting | all | 23.6 [21.6, 25.5] | 20.6 [18.3, 22.9] | 3.0 [0.5, 5.4] | 0.0307 |
