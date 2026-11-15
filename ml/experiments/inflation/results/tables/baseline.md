# Post hoc: honest-baseline variants (exploratory)

**Added after independent review (amendment of 15 Nov 2026).** Each variant is a complete main-protocol run (seeds 1001–1020, frozen detectors, SIM-FI and SIM-BL); the first column repeats the main stage for reference. Recall at the validation 5 % threshold, %, mean [95 % CI]; Δ = SIM-FI − SIM-BL paired by seed with exact sign-flip p (unadjusted).

(a) Revision multiplier m = 0.1843 on both `revision.change_mult_*` (≡ scaling the honest revision propensity r_i while r_i·mult < 0.95), found by bisection on log m on the honest world of seed 1001: mean answer changes per 20-item attempt 0.840 (target 0.84 ± 0.005), 95th percentile 4.0; calibrated parameters (m = 1): mean 4.662, 95th percentile 13.0.

(c) closing time at P90: `--close-after-ms 1792521` `--auto-submit-jitter-ms 2000` (examory-sim-next).

(c) closing time at P75: `--close-after-ms 1600450` `--auto-submit-jitter-ms 2000` (examory-sim-next).

## Recall SIM-FI / SIM-BL

| Detector | Subset | main stage (calibrated) | (a) exam-like revision rate | (b) per-attempt item sampling | (c) closing time at P90 | (c) closing time at P75 |
|---|---|---:|---:|---:|---:|---:|
| IF + peers | all | 53.6 [51.9, 55.3] / 27.8 [25.8, 29.7] | 56.5 [54.7, 58.5] / 30.3 [27.4, 33.3] | 55.5 [52.8, 57.9] / 27.4 [24.6, 30.0] | 52.6 [50.3, 54.9] / 27.7 [25.4, 30.0] | 54.2 [52.2, 56.2] / 30.1 [27.3, 32.8] |
| IF + peers | S1 rapid_response↔preknowledge | 35.4 [32.5, 38.4] / 24.2 [22.1, 26.4] | 33.9 [30.6, 37.3] / 19.5 [16.7, 22.6] | 37.0 [33.6, 40.4] / 20.9 [18.0, 23.6] | 33.8 [31.2, 36.4] / 22.6 [20.4, 24.9] | 37.6 [34.5, 40.7] / 24.0 [21.7, 26.4] |
| IF + peers | S2 excessive_changes↔lookup | 68.0 [63.5, 72.4] / 14.4 [11.7, 17.2] | 91.8 [90.3, 93.4] / 32.5 [27.5, 37.2] | 72.9 [67.6, 77.9] / 13.7 [10.8, 16.7] | 67.6 [61.8, 73.2] / 20.1 [16.5, 23.8] | 75.9 [71.6, 79.8] / 32.9 [28.3, 37.7] |
| IF + peers | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] / 61.0 [56.5, 65.4] | 86.8 [82.9, 90.4] / 58.8 [54.2, 63.7] | 100.0 [100.0, 100.0] / 65.6 [59.9, 71.3] | 97.8 [96.0, 99.3] / 57.4 [52.5, 62.2] | 90.0 [86.7, 93.3] / 51.1 [45.7, 56.0] |
| IF + peers | S4 score_jump↔proxy | 5.1 [3.1, 7.2] / 9.5 [7.8, 11.3] | 7.3 [5.1, 9.5] / 9.2 [6.8, 11.5] | 5.3 [3.4, 7.4] / 9.4 [6.7, 12.3] | 5.9 [4.0, 7.9] / 8.5 [6.5, 10.5] | 6.3 [3.7, 9.7] / 8.5 [6.2, 10.8] |
| IF + peers | realized FPR | 5.1 [4.6, 5.7] / 5.0 [4.4, 5.6] | 5.1 [4.6, 5.6] / 5.0 [4.5, 5.5] | 5.0 [4.4, 5.7] / 5.0 [4.5, 5.7] | 4.7 [4.2, 5.3] / 4.5 [3.9, 5.0] | 5.0 [4.4, 5.6] / 4.8 [4.3, 5.4] |
| IF | all | 46.3 [43.9, 48.5] / 22.9 [21.0, 24.9] | 45.2 [42.8, 47.5] / 24.8 [22.9, 26.7] | 44.7 [42.6, 47.1] / 21.4 [19.9, 23.0] | 45.5 [43.4, 47.5] / 22.6 [20.6, 24.7] | 45.6 [43.6, 47.6] / 22.2 [20.5, 23.9] |
| IF | S1 rapid_response↔preknowledge | 55.6 [52.5, 58.5] / 29.8 [27.4, 32.3] | 50.6 [47.3, 53.9] / 23.3 [21.0, 25.7] | 53.6 [50.5, 57.1] / 27.8 [25.2, 30.6] | 54.8 [52.1, 57.3] / 30.0 [27.7, 32.4] | 55.5 [52.7, 58.3] / 29.2 [27.3, 31.1] |
| IF | S2 excessive_changes↔lookup | 91.1 [88.9, 93.2] / 23.2 [20.7, 26.0] | 98.9 [98.0, 99.7] / 46.9 [42.4, 51.3] | 89.5 [87.0, 91.9] / 21.3 [18.5, 23.9] | 90.6 [88.5, 92.7] / 22.9 [19.4, 26.9] | 90.3 [88.0, 92.7] / 22.1 [19.0, 25.6] |
| IF | S3 collusion↔copy_peer | 11.0 [7.7, 14.5] / 21.5 [18.7, 24.3] | 5.0 [3.2, 6.9] / 15.4 [12.4, 18.6] | 9.1 [6.2, 12.0] / 21.2 [17.5, 25.2] | 9.7 [6.8, 13.1] / 21.5 [18.6, 24.4] | 9.9 [7.0, 13.3] / 21.1 [18.2, 23.8] |
| IF | S4 score_jump↔proxy | 5.6 [3.6, 8.1] / 8.3 [5.8, 11.1] | 6.1 [3.6, 9.2] / 6.2 [3.9, 8.6] | 5.0 [3.0, 7.1] / 8.6 [6.2, 11.0] | 5.2 [3.3, 7.3] / 8.0 [5.3, 11.0] | 5.1 [3.3, 7.2] / 8.5 [5.7, 11.5] |
| IF | realized FPR | 5.3 [4.8, 6.0] / 5.3 [4.7, 6.0] | 5.0 [4.5, 5.6] / 5.3 [4.8, 5.9] | 5.2 [4.7, 5.7] / 5.3 [4.8, 5.9] | 5.3 [4.7, 6.0] / 5.3 [4.7, 6.0] | 5.2 [4.7, 5.7] / 5.2 [4.8, 5.7] |
| Rules | all | 61.9 [59.8, 64.0] / 36.2 [33.0, 39.3] | 63.1 [60.7, 65.5] / 42.3 [39.0, 45.7] | 59.3 [56.9, 61.4] / 23.4 [20.2, 26.8] | 60.9 [59.0, 62.8] / 37.1 [33.5, 40.6] | 60.7 [58.8, 62.6] / 34.9 [31.8, 37.9] |
| Rules | S1 rapid_response↔preknowledge | 34.1 [29.9, 38.2] / 34.3 [29.9, 38.6] | 33.3 [29.1, 37.6] / 36.9 [32.1, 41.8] | 33.1 [29.7, 36.3] / 28.7 [24.7, 32.8] | 31.8 [28.2, 35.5] / 34.8 [30.1, 39.5] | 30.9 [27.2, 34.8] / 32.6 [28.2, 37.0] |
| Rules | S2 excessive_changes↔lookup | 97.8 [96.7, 98.8] / 28.0 [23.1, 33.0] | 100.0 [100.0, 100.0] / 44.1 [38.1, 50.1] | 97.4 [95.9, 98.7] / 22.4 [18.2, 26.8] | 97.5 [96.1, 98.8] / 31.3 [25.2, 37.6] | 97.2 [96.0, 98.2] / 28.9 [24.9, 33.0] |
| Rules | S3 collusion↔copy_peer | 100.0 [100.0, 100.0] / 65.5 [60.1, 70.9] | 100.0 [100.0, 100.0] / 67.3 [62.4, 72.2] | 90.3 [82.0, 97.0] / 25.4 [18.7, 32.5] | 100.0 [100.0, 100.0] / 65.3 [60.1, 70.6] | 100.0 [100.0, 100.0] / 63.0 [57.6, 68.2] |
| Rules | S4 score_jump↔proxy | 12.3 [8.2, 17.4] / 10.7 [7.3, 14.2] | 18.5 [14.2, 23.3] / 15.4 [12.0, 18.5] | 10.5 [8.1, 13.1] / 9.1 [6.2, 12.3] | 12.0 [7.9, 17.0] / 10.4 [7.5, 13.5] | 12.9 [8.6, 18.0] / 8.6 [5.6, 12.0] |
| Rules | realized FPR | 5.3 [4.7, 6.0] / 5.2 [4.5, 5.8] | 5.1 [4.5, 5.6] / 5.2 [4.5, 5.9] | 5.0 [4.4, 5.7] / 5.0 [4.2, 5.8] | 5.3 [4.5, 6.0] / 5.3 [4.5, 6.0] | 4.9 [4.3, 5.6] / 4.9 [4.3, 5.6] |

## Δ = SIM-FI − SIM-BL (pp)

| Detector | Subset | main stage (calibrated) | (a) exam-like revision rate | (b) per-attempt item sampling | (c) closing time at P90 | (c) closing time at P75 |
|---|---|---:|---:|---:|---:|---:|
| IF + peers | all | 25.8 [23.9, 27.7], p <0.0001 | 26.3 [23.4, 28.9], p <0.0001 | 28.1 [26.2, 30.2], p <0.0001 | 24.8 [22.4, 27.2], p <0.0001 | 24.1 [22.2, 26.1], p <0.0001 |
| IF + peers | S1 rapid_response↔preknowledge | 11.2 [8.0, 14.7], p <0.0001 | 14.5 [10.5, 18.4], p <0.0001 | 16.2 [13.0, 19.2], p <0.0001 | 11.2 [8.0, 14.2], p <0.0001 | 13.6 [10.5, 16.7], p <0.0001 |
| IF + peers | S2 excessive_changes↔lookup | 53.6 [48.8, 58.3], p <0.0001 | 59.4 [54.1, 64.8], p <0.0001 | 59.2 [53.9, 64.6], p <0.0001 | 47.5 [40.9, 53.7], p <0.0001 | 43.0 [37.4, 48.5], p <0.0001 |
| IF + peers | S3 collusion↔copy_peer | 39.0 [34.6, 43.5], p <0.0001 | 28.0 [22.5, 33.3], p <0.0001 | 34.4 [28.7, 40.1], p <0.0001 | 40.4 [36.0, 44.9], p <0.0001 | 39.0 [33.9, 44.3], p <0.0001 |
| IF + peers | S4 score_jump↔proxy | -4.4 [-6.6, -2.2], p 0.0017 | -1.9 [-4.6, 0.9], p 0.2048 | -4.1 [-7.2, -1.2], p 0.0127 | -2.6 [-5.3, -0.0], p 0.0693 | -2.2 [-5.4, 1.2], p 0.2063 |
| IF | all | 23.4 [21.0, 25.8], p <0.0001 | 20.4 [18.0, 22.8], p <0.0001 | 23.3 [20.7, 26.0], p <0.0001 | 22.9 [20.4, 25.5], p <0.0001 | 23.4 [21.0, 25.9], p <0.0001 |
| IF | S1 rapid_response↔preknowledge | 25.7 [23.0, 28.4], p <0.0001 | 27.4 [23.6, 31.0], p <0.0001 | 25.8 [21.8, 29.5], p <0.0001 | 24.8 [21.9, 27.6], p <0.0001 | 26.3 [23.3, 29.1], p <0.0001 |
| IF | S2 excessive_changes↔lookup | 67.9 [64.7, 71.0], p <0.0001 | 52.0 [47.3, 56.8], p <0.0001 | 68.3 [64.4, 72.4], p <0.0001 | 67.7 [63.7, 71.6], p <0.0001 | 68.2 [63.8, 72.5], p <0.0001 |
| IF | S3 collusion↔copy_peer | -10.5 [-14.3, -6.7], p <0.0001 | -10.4 [-13.4, -7.4], p <0.0001 | -12.2 [-15.5, -8.6], p <0.0001 | -11.8 [-15.5, -7.7], p <0.0001 | -11.2 [-14.5, -7.5], p <0.0001 |
| IF | S4 score_jump↔proxy | -2.7 [-5.5, 0.1], p 0.0791 | -0.1 [-2.7, 2.8], p 0.9570 | -3.7 [-6.4, -0.8], p 0.0239 | -2.8 [-5.7, 0.0], p 0.0715 | -3.4 [-5.9, -0.6], p 0.0241 |
| Rules | all | 25.8 [23.0, 28.6], p <0.0001 | 20.8 [17.6, 24.1], p <0.0001 | 35.9 [32.8, 38.8], p <0.0001 | 23.8 [21.0, 26.7], p <0.0001 | 25.8 [23.4, 28.2], p <0.0001 |
| Rules | S1 rapid_response↔preknowledge | -0.2 [-6.1, 5.3], p 0.9366 | -3.5 [-8.4, 1.2], p 0.1725 | 4.4 [-0.9, 9.8], p 0.1364 | -3.0 [-8.9, 2.7], p 0.3282 | -1.7 [-7.2, 3.6], p 0.5466 |
| Rules | S2 excessive_changes↔lookup | 69.8 [64.9, 74.6], p <0.0001 | 55.9 [49.9, 61.9], p <0.0001 | 75.0 [70.7, 79.0], p <0.0001 | 66.2 [60.2, 72.1], p <0.0001 | 68.2 [64.2, 72.3], p <0.0001 |
| Rules | S3 collusion↔copy_peer | 34.5 [29.1, 39.9], p <0.0001 | 32.7 [27.8, 37.6], p <0.0001 | 64.9 [56.7, 72.7], p <0.0001 | 34.7 [29.4, 39.9], p <0.0001 | 37.0 [31.8, 42.4], p <0.0001 |
| Rules | S4 score_jump↔proxy | 1.6 [-3.3, 6.9], p 0.5579 | 3.1 [-1.5, 7.8], p 0.2151 | 1.5 [-2.2, 4.9], p 0.4349 | 1.6 [-3.0, 7.3], p 0.5855 | 4.3 [-0.8, 9.9], p 0.1360 |

## SIM-BL copy_peer recall by synchronous submission and by auto-submission

Recall %, mean [95 % CI]; n = mean test cheaters of the sub-group per seed. Auto-submission exists only with a closing time (simulator 0.3.0, `truth.csv:auto_submitted`).

| Detector | Sub-group | main stage (calibrated) | (a) exam-like revision rate | (b) per-attempt item sampling | (c) closing time at P90 | (c) closing time at P75 |
|---|---|---:|---:|---:|---:|---:|
| IF + peers | non-sync pairs | 36.1 [29.5, 42.9] (n 16.6) | 35.1 [28.3, 42.4] (n 16.6) | 40.0 [32.3, 48.1] (n 16.6) | 37.8 [31.4, 44.7] (n 16.6) | 38.9 [33.0, 44.5] (n 16.6) |
| IF + peers | sync pairs | 86.0 [82.5, 89.5] (n 16.0) | 82.8 [79.0, 86.8] (n 16.0) | 91.6 [86.5, 96.0] (n 16.0) | 77.9 [72.8, 82.7] (n 16.0) | 64.0 [57.1, 70.7] (n 16.0) |
| IF + peers | cheater not auto-submitted | — | — | — | 52.9 [46.7, 58.4] (n 29.2) | 42.1 [36.0, 48.0] (n 24.7) |
| IF + peers | cheater auto-submitted | — | — | — | 93.4 [86.0, 99.3] (n 3.4) | 75.1 [67.9, 81.9] (n 7.9) |
| IF | non-sync pairs | 22.1 [19.4, 25.0] (n 16.6) | 16.1 [12.7, 19.9] (n 16.6) | 22.1 [17.7, 27.0] (n 16.6) | 23.0 [20.1, 25.9] (n 16.6) | 23.6 [20.3, 26.7] (n 16.6) |
| IF | sync pairs | 21.5 [16.3, 26.5] (n 16.0) | 15.4 [10.4, 20.8] (n 16.0) | 21.3 [15.4, 27.5] (n 16.0) | 20.5 [15.4, 25.4] (n 16.0) | 19.4 [14.6, 24.2] (n 16.0) |
| IF | cheater not auto-submitted | — | — | — | 20.8 [17.7, 24.0] (n 29.2) | 21.1 [18.0, 24.3] (n 24.7) |
| IF | cheater auto-submitted | — | — | — | 21.7 [13.1, 30.0] (n 3.4) | 20.1 [15.1, 25.0] (n 7.9) |
| Rules | non-sync pairs | 39.8 [32.1, 47.6] (n 16.6) | 41.4 [33.7, 49.2] (n 16.6) | 18.3 [13.3, 23.2] (n 16.6) | 42.3 [34.3, 50.5] (n 16.6) | 42.4 [35.0, 49.5] (n 16.6) |
| Rules | sync pairs | 92.0 [87.6, 96.1] (n 16.0) | 93.5 [88.8, 97.4] (n 16.0) | 33.4 [23.5, 43.8] (n 16.0) | 89.6 [84.7, 94.1] (n 16.0) | 84.7 [79.7, 89.5] (n 16.0) |
| Rules | cheater not auto-submitted | — | — | — | 62.0 [55.6, 68.3] (n 29.2) | 55.8 [49.8, 61.6] (n 24.7) |
| Rules | cheater auto-submitted | — | — | — | 90.0 [76.5, 100.0] (n 3.4) | 86.8 [77.2, 94.6] (n 7.9) |

## Manipulation check, (a) exam-like revision rate

Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share of cheater attempts in which at least one of the four observables differs from the counterfactual.

| World | Type | cheaters/seed | Δ score (pts) | Δ duration (s) | Δ answer changes | Δ median first-answer time (s) | share changed |
|---|---|---:|---:|---:|---:|---:|---:|
| SIM-FI | rapid_response | 252.0 | 0.00 [0.00, 0.00] | -191.35 [-196.85, -185.89] | 0.00 [0.00, 0.00] | -9.80 [-10.06, -9.51] | 1.00 [1.00, 1.00] |
| SIM-FI | excessive_changes | 180.0 | 0.00 [0.00, 0.00] | -2.83 [-3.19, -2.47] | 29.42 [29.19, 29.64] | -9.98 [-10.32, -9.68] | 1.00 [1.00, 1.00] |
| SIM-FI | collusion | 180.0 | -3.53 [-3.66, -3.40] | 18.28 [10.75, 25.59] | -0.79 [-0.84, -0.73] | 0.17 [-0.07, 0.40] | 1.00 [1.00, 1.00] |
| SIM-FI | score_jump | 108.0 | 3.97 [3.84, 4.11] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.97 [0.96, 0.98] |
| SIM-BL | preknowledge | 252.0 | 4.99 [4.75, 5.24] | -80.80 [-84.19, -77.24] | -0.51 [-0.54, -0.48] | -4.00 [-4.19, -3.81] | 0.99 [0.99, 0.99] |
| SIM-BL | lookup | 180.0 | 4.96 [4.76, 5.17] | 272.53 [261.16, 284.71] | 2.41 [2.29, 2.54] | 4.50 [4.14, 4.85] | 0.99 [0.99, 1.00] |
| SIM-BL | copy_peer | 180.0 | 0.68 [0.61, 0.76] | 65.39 [59.45, 71.65] | -0.25 [-0.29, -0.21] | -1.65 [-1.89, -1.39] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=0 | 90.1 | 0.68 [0.60, 0.76] | 55.18 [45.93, 64.47] | -0.26 [-0.31, -0.22] | -1.59 [-1.97, -1.22] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=1 | 89.9 | 0.69 [0.58, 0.81] | 75.21 [67.29, 82.72] | -0.23 [-0.28, -0.18] | -1.74 [-2.06, -1.44] | 1.00 [1.00, 1.00] |
| SIM-BL | proxy | 108.0 | 3.45 [3.30, 3.60] | -50.54 [-59.67, -41.24] | -0.18 [-0.23, -0.13] | -2.14 [-2.55, -1.72] | 1.00 [1.00, 1.00] |

## Manipulation check, (b) per-attempt item sampling

Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share of cheater attempts in which at least one of the four observables differs from the counterfactual.

| World | Type | cheaters/seed | Δ score (pts) | Δ duration (s) | Δ answer changes | Δ median first-answer time (s) | share changed |
|---|---|---:|---:|---:|---:|---:|---:|
| SIM-FI | rapid_response | 252.0 | 0.00 [0.00, 0.00] | -202.91 [-205.72, -199.88] | 0.00 [0.00, 0.00] | -9.93 [-10.07, -9.79] | 1.00 [1.00, 1.00] |
| SIM-FI | excessive_changes | 180.0 | 0.00 [0.00, 0.00] | -14.95 [-15.81, -14.12] | 27.33 [27.09, 27.55] | -10.16 [-10.29, -10.04] | 1.00 [1.00, 1.00] |
| SIM-FI | collusion | 180.0 | -3.59 [-3.67, -3.50] | 18.14 [9.98, 26.16] | -4.32 [-4.43, -4.20] | 0.86 [0.62, 1.12] | 1.00 [1.00, 1.00] |
| SIM-FI | score_jump | 108.0 | 2.91 [2.84, 2.99] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.92 [0.91, 0.93] |
| SIM-BL | preknowledge | 252.0 | 4.85 [4.78, 4.92] | -94.64 [-97.79, -91.35] | -2.68 [-2.75, -2.62] | -4.10 [-4.24, -3.97] | 1.00 [0.99, 1.00] |
| SIM-BL | lookup | 180.0 | 4.48 [4.41, 4.55] | 264.57 [260.90, 268.24] | 0.56 [0.45, 0.66] | 4.46 [4.17, 4.76] | 0.99 [0.99, 0.99] |
| SIM-BL | copy_peer | 180.0 | 0.63 [0.55, 0.71] | 63.61 [57.35, 70.15] | -1.32 [-1.40, -1.25] | -1.49 [-1.76, -1.20] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=0 | 90.1 | 0.60 [0.48, 0.71] | 53.48 [43.81, 63.37] | -1.37 [-1.49, -1.25] | -1.39 [-1.79, -0.99] | 1.00 [1.00, 1.00] |
| SIM-BL | copy_peer:sync_submit=1 | 89.9 | 0.67 [0.57, 0.77] | 73.54 [65.39, 81.37] | -1.28 [-1.38, -1.17] | -1.61 [-1.94, -1.28] | 1.00 [1.00, 1.00] |
| SIM-BL | proxy | 108.0 | 2.98 [2.85, 3.11] | -54.76 [-63.23, -46.15] | -0.91 [-1.07, -0.73] | -2.19 [-2.60, -1.78] | 1.00 [1.00, 1.00] |

## Manipulation check, (c) closing time at P90

Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share of cheater attempts in which at least one of the four observables differs from the counterfactual.

| World | Type | cheaters/seed | Δ score (pts) | Δ duration (s) | Δ answer changes | Δ median first-answer time (s) | share changed |
|---|---|---:|---:|---:|---:|---:|---:|
| SIM-FI | rapid_response | 252.0 | 0.07 [0.05, 0.08] | -192.90 [-197.60, -188.04] | 0.04 [0.03, 0.04] | -9.78 [-10.05, -9.50] | 1.00 [1.00, 1.00] |
| SIM-FI | excessive_changes | 180.0 | 0.01 [0.01, 0.01] | -13.80 [-14.70, -12.84] | 27.20 [26.94, 27.45] | -10.00 [-10.34, -9.69] | 1.00 [1.00, 1.00] |
| SIM-FI | collusion | 180.0 | -3.61 [-3.73, -3.48] | 16.30 [8.72, 23.87] | -4.29 [-4.48, -4.10] | 0.91 [0.67, 1.15] | 1.00 [1.00, 1.00] |
| SIM-FI | score_jump | 108.0 | 2.89 [2.79, 3.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.92 [0.91, 0.93] |
| SIM-BL | preknowledge | 252.0 | 4.89 [4.69, 5.09] | -88.19 [-91.48, -84.76] | -2.64 [-2.72, -2.56] | -3.99 [-4.17, -3.81] | 0.99 [0.99, 1.00] |
| SIM-BL | lookup | 180.0 | 3.91 [3.77, 4.06] | 216.20 [208.88, 223.93] | 0.41 [0.30, 0.52] | 4.42 [4.06, 4.78] | 0.99 [0.98, 1.00] |
| SIM-BL | copy_peer | 180.0 | 0.53 [0.46, 0.59] | 60.32 [54.80, 66.09] | -1.31 [-1.39, -1.22] | -1.43 [-1.67, -1.16] | 0.99 [0.98, 0.99] |
| SIM-BL | copy_peer:sync_submit=0 | 90.1 | 0.52 [0.43, 0.61] | 49.96 [42.31, 57.98] | -1.36 [-1.49, -1.24] | -1.34 [-1.72, -0.98] | 0.99 [0.99, 1.00] |
| SIM-BL | copy_peer:sync_submit=1 | 89.9 | 0.54 [0.46, 0.63] | 70.17 [63.44, 77.05] | -1.25 [-1.37, -1.14] | -1.54 [-1.86, -1.22] | 0.99 [0.98, 0.99] |
| SIM-BL | proxy | 108.0 | 2.91 [2.75, 3.07] | -51.20 [-58.51, -43.73] | -0.88 [-1.03, -0.71] | -2.11 [-2.53, -1.69] | 1.00 [1.00, 1.00] |

## Manipulation check, (c) closing time at P75

Mean paired difference cheater attempt − the same attempt in the honest world (same attempt_id, same student, same draws), averaged over cheaters per seed, then mean over seeds [95 % CI]. Score in points out of 20; duration and median first-answer time in seconds; changes = answer-change events. Last column: share of cheater attempts in which at least one of the four observables differs from the counterfactual.

| World | Type | cheaters/seed | Δ score (pts) | Δ duration (s) | Δ answer changes | Δ median first-answer time (s) | share changed |
|---|---|---:|---:|---:|---:|---:|---:|
| SIM-FI | rapid_response | 252.0 | 0.22 [0.20, 0.24] | -176.30 [-180.01, -172.46] | 0.10 [0.08, 0.13] | -9.78 [-10.04, -9.50] | 1.00 [1.00, 1.00] |
| SIM-FI | excessive_changes | 180.0 | 0.03 [0.02, 0.04] | -11.99 [-12.79, -11.14] | 26.78 [26.49, 27.06] | -9.95 [-10.27, -9.66] | 1.00 [1.00, 1.00] |
| SIM-FI | collusion | 180.0 | -3.57 [-3.70, -3.45] | 12.17 [5.65, 18.81] | -4.22 [-4.41, -4.04] | 0.91 [0.67, 1.16] | 1.00 [1.00, 1.00] |
| SIM-FI | score_jump | 108.0 | 2.84 [2.74, 2.95] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] | 0.92 [0.90, 0.93] |
| SIM-BL | preknowledge | 252.0 | 4.92 [4.73, 5.13] | -79.02 [-81.82, -76.05] | -2.58 [-2.65, -2.50] | -4.01 [-4.19, -3.83] | 0.99 [0.99, 1.00] |
| SIM-BL | lookup | 180.0 | 3.29 [3.17, 3.41] | 165.55 [159.98, 171.65] | 0.20 [0.09, 0.31] | 4.38 [4.01, 4.76] | 0.99 [0.98, 0.99] |
| SIM-BL | copy_peer | 180.0 | 0.46 [0.37, 0.54] | 51.28 [45.88, 56.63] | -1.30 [-1.39, -1.21] | -1.44 [-1.68, -1.18] | 0.97 [0.96, 0.98] |
| SIM-BL | copy_peer:sync_submit=0 | 90.1 | 0.46 [0.36, 0.56] | 43.13 [35.81, 50.62] | -1.35 [-1.47, -1.22] | -1.39 [-1.76, -1.04] | 0.97 [0.97, 0.98] |
| SIM-BL | copy_peer:sync_submit=1 | 89.9 | 0.46 [0.35, 0.56] | 59.06 [53.37, 65.03] | -1.25 [-1.37, -1.14] | -1.50 [-1.80, -1.20] | 0.97 [0.96, 0.98] |
| SIM-BL | proxy | 108.0 | 2.94 [2.78, 3.10] | -45.35 [-51.87, -38.82] | -0.84 [-1.00, -0.68] | -2.11 [-2.51, -1.71] | 1.00 [1.00, 1.00] |

## History-, item- and peer-based features under each baseline (mean over seeds)

History features (`history_missing`, `z_score`) are keyed by (user, quiz_id, variant_id); the simulator keeps one quiz_id/variant_id under per-attempt sampling, so score histories stay defined although the items differ between a student's attempts.

| Quantity | main configuration (protocol stage), SIM-FI | main configuration (protocol stage), SIM-BL | (a) exam-like revision rate, SIM-FI | (a) exam-like revision rate, SIM-BL | (b) per-attempt item sampling, SIM-FI | (b) per-attempt item sampling, SIM-BL | (c) closing time at P90, SIM-FI | (c) closing time at P90, SIM-BL | (c) closing time at P75, SIM-FI | (c) closing time at P75, SIM-BL |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| share of attempts auto-submitted | — | — | — | — | — | — | 0.0509 | 0.0584 | 0.155 | 0.167 |
| share of normal attempts auto-submitted | — | — | — | — | — | — | 0.0529 | 0.0529 | 0.16 | 0.16 |
| share of cheater attempts auto-submitted | — | — | — | — | — | — | 0.036 | 0.0983 | 0.118 | 0.222 |
| share of normal attempts with a synchronous peer (5 s) | 0.174 | 0.174 | 0.176 | 0.176 | 0.173 | 0.174 | 0.209 | 0.21 | 0.317 | 0.318 |
| share without score history (history_missing) | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 | 0.333 |
| share with z_score defined | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 | 0.667 |
| distinct items | 20 | 20 | 20 | 20 | 1.09e+03 | 1.09e+03 | 20 | 20 | 20 | 20 |
| share of responses with an own item timing reference | 1 | 1 | 1 | 1 | 1 | 1 | 1 | 1 | 1 | 1 |
| share of attempts with a synchronous peer (5 s) | 0.198 | 0.187 | 0.2 | 0.189 | 0.198 | 0.186 | 0.23 | 0.226 | 0.332 | 0.334 |
| mean common items of synchronous pairs | 20 | 20 | 20 | 20 | 2.99 | 2.06 | 19.3 | 19.1 | 17.4 | 17.1 |
| share of synchronous pairs with no common item | 0 | 0 | 0 | 0 | 0.595 | 0.628 | 0 | 0 | 0 | 0 |
| mean max_answer_agreement | 0.106 | 0.0954 | 0.108 | 0.0974 | 0.0557 | 0.0429 | 0.12 | 0.115 | 0.177 | 0.178 |
