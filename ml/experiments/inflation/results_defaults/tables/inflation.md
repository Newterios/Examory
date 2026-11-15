> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, two-sided Wilcoxon signed-rank). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). Holm correction within each family of 12 slot × detector tests.

## SIM-FI-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 16.3 [12.1, 20.2] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 66.5 [61.2, 71.4] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 48.0 [42.8, 52.8] | 20 | <0.0001 | 0.0008 |
| iforest | S4 score_jump↔proxy | -1.7 [-4.9, 1.5] | 20 | 0.2219 | 0.6656 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 26.9 [23.3, 30.3] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 76.4 [72.8, 79.7] | 20 | <0.0001 | 0.0008 |
| iforest_no_peers | S3 collusion↔copy_peer | 3.4 [0.8, 5.8] | 20 | 0.0214 | 0.1068 |
| iforest_no_peers | S4 score_jump↔proxy | -1.3 [-3.8, 1.2] | 20 | 0.4430 | 0.8861 |
| quantile_rules | S1 rapid_response↔preknowledge | -0.7 [-4.5, 3.2] | 20 | 0.7439 | 0.8861 |
| quantile_rules | S2 excessive_changes↔lookup | 74.2 [69.4, 78.8] | 20 | <0.0001 | 0.0008 |
| quantile_rules | S3 collusion↔copy_peer | 45.2 [40.4, 50.3] | 20 | <0.0001 | 0.0008 |
| quantile_rules | S4 score_jump↔proxy | -4.0 [-8.1, 0.3] | 20 | 0.0840 | 0.3361 |

## LEGACY-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 19.9 [15.7, 24.1] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 36.4 [31.5, 41.2] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 42.7 [35.7, 49.7] | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -2.1 [-5.8, 1.5] | 20 | 0.3072 | 0.6852 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 38.5 [33.9, 43.0] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 55.0 [50.1, 59.5] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -2.2 [-4.2, -0.3] | 20 | 0.0516 | 0.2066 |
| iforest_no_peers | S4 score_jump↔proxy | 0.1 [-3.1, 3.2] | 20 | 0.9131 | 0.9131 |
| quantile_rules | S1 rapid_response↔preknowledge | 19.4 [14.5, 24.5] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S2 excessive_changes↔lookup | 73.4 [68.6, 78.0] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 45.2 [40.4, 50.3] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | -4.0 [-9.0, 1.0] | 20 | 0.2284 | 0.6852 |

## SIM-FI-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.059 [0.045, 0.073] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 0.267 [0.242, 0.291] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.209 [0.182, 0.237] | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -0.004 [-0.015, 0.007] | 20 | 0.5197 | 1.0000 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.095 [0.081, 0.109] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.325 [0.313, 0.337] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | 0.008 [0.002, 0.014] | 20 | 0.0262 | 0.1309 |
| iforest_no_peers | S4 score_jump↔proxy | -0.005 [-0.012, 0.002] | 20 | 0.1841 | 0.7363 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.007 [-0.008, 0.022] | 20 | 0.5459 | 1.0000 |
| quantile_rules | S2 excessive_changes↔lookup | 0.411 [0.397, 0.425] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.249 [0.224, 0.273] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | -0.012 [-0.026, 0.002] | 20 | 0.2162 | 0.7363 |

## LEGACY-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.077 [0.061, 0.092] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 0.162 [0.144, 0.180] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.153 [0.120, 0.185] | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -0.004 [-0.015, 0.008] | 20 | 0.3131 | 0.9394 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.170 [0.153, 0.186] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.249 [0.233, 0.265] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -0.006 [-0.012, -0.001] | 20 | 0.0690 | 0.2761 |
| iforest_no_peers | S4 score_jump↔proxy | 0.003 [-0.006, 0.012] | 20 | 0.6535 | 0.9394 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.106 [0.087, 0.125] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S2 excessive_changes↔lookup | 0.401 [0.384, 0.419] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.247 [0.222, 0.274] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | -0.011 [-0.030, 0.007] | 20 | 0.4570 | 0.9394 |
