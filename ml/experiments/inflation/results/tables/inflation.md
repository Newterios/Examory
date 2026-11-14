# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, two-sided Wilcoxon signed-rank). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). Holm correction within each family of 12 slot × detector tests.

## SIM-FI-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 11.2 [8.0, 14.7] | 20 | 0.0001 | 0.0008 |
| iforest | S2 excessive_changes↔lookup | 53.6 [48.8, 58.3] | 20 | <0.0001 | 0.0008 |
| iforest | S3 collusion↔copy_peer | 39.0 [34.6, 43.5] | 20 | <0.0001 | 0.0008 |
| iforest | S4 score_jump↔proxy | -4.4 [-6.6, -2.2] | 20 | 0.0041 | 0.0165 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 25.7 [23.0, 28.4] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 67.9 [64.7, 71.0] | 20 | <0.0001 | 0.0008 |
| iforest_no_peers | S3 collusion↔copy_peer | -10.5 [-14.3, -6.7] | 20 | 0.0003 | 0.0016 |
| iforest_no_peers | S4 score_jump↔proxy | -2.7 [-5.5, 0.1] | 20 | 0.1194 | 0.3581 |
| quantile_rules | S1 rapid_response↔preknowledge | -0.2 [-6.1, 5.3] | 20 | 0.7475 | 1.0000 |
| quantile_rules | S2 excessive_changes↔lookup | 69.8 [64.9, 74.6] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 34.5 [29.1, 39.9] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 1.6 [-3.3, 6.9] | 20 | 0.5301 | 1.0000 |

## LEGACY-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 14.4 [10.2, 18.5] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 35.5 [31.1, 39.9] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 27.8 [22.0, 33.6] | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -2.9 [-6.0, 0.1] | 20 | 0.0895 | 0.2685 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 28.1 [23.3, 32.6] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 50.9 [46.6, 55.0] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -20.1 [-23.1, -17.1] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S4 score_jump↔proxy | -2.0 [-5.2, 1.3] | 20 | 0.4973 | 0.4973 |
| quantile_rules | S1 rapid_response↔preknowledge | 15.4 [9.8, 20.8] | 20 | <0.0001 | 0.0002 |
| quantile_rules | S2 excessive_changes↔lookup | 71.0 [65.9, 75.7] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 34.5 [29.1, 40.0] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 3.3 [-1.5, 8.3] | 20 | 0.2075 | 0.4151 |

## SIM-FI-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.036 [0.023, 0.050] | 20 | <0.0001 | 0.0004 |
| iforest | S2 excessive_changes↔lookup | 0.221 [0.201, 0.242] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.195 [0.172, 0.220] | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -0.015 [-0.024, -0.006] | 20 | 0.0056 | 0.0223 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.093 [0.082, 0.104] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.299 [0.283, 0.314] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -0.030 [-0.043, -0.017] | 20 | 0.0002 | 0.0008 |
| iforest_no_peers | S4 score_jump↔proxy | -0.005 [-0.013, 0.004] | 20 | 0.1446 | 0.4337 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.004 [-0.014, 0.021] | 20 | 0.6477 | 0.6477 |
| quantile_rules | S2 excessive_changes↔lookup | 0.369 [0.351, 0.387] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.198 [0.173, 0.220] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 0.010 [-0.003, 0.023] | 20 | 0.1650 | 0.4337 |

## LEGACY-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.051 [0.035, 0.067] | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 0.154 [0.137, 0.172] | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.051 [0.020, 0.083] | 20 | 0.0144 | 0.0431 |
| iforest | S4 score_jump↔proxy | -0.012 [-0.022, -0.000] | 20 | 0.0076 | 0.0302 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.130 [0.110, 0.149] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.234 [0.218, 0.249] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -0.054 [-0.065, -0.043] | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S4 score_jump↔proxy | -0.003 [-0.013, 0.008] | 20 | 0.7240 | 0.7240 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.102 [0.081, 0.122] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S2 excessive_changes↔lookup | 0.400 [0.381, 0.416] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.203 [0.178, 0.227] | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 0.018 [0.003, 0.033] | 20 | 0.0239 | 0.0478 |
