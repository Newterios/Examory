# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, exact two-sided sign-flip permutation test on the per-seed differences, all 2^n sign patterns). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). MCSE = Monte Carlo standard error of Δ (paired: SD of the per-seed differences / √n; unpaired: √(s²_x/n_x + s²_y/n_y)). Holm correction within each family (one family per comparison × metric) of 12 slot × detector tests (number of detectors × 4 slots); the family size is printed with each block.

## SIM-FI-minus-SIM-BL, recall@5 (Holm family of 12)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 11.2 [8.0, 14.7] | 1.75 | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 53.6 [48.8, 58.3] | 2.52 | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 39.0 [34.6, 43.5] | 2.32 | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -4.4 [-6.6, -2.2] | 1.15 | 20 | 0.0017 | 0.0068 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 25.7 [23.0, 28.4] | 1.41 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 67.9 [64.7, 71.0] | 1.66 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -10.5 [-14.3, -6.7] | 1.97 | 20 | <0.0001 | 0.0003 |
| iforest_no_peers | S4 score_jump↔proxy | -2.7 [-5.5, 0.1] | 1.44 | 20 | 0.0791 | 0.2373 |
| quantile_rules | S1 rapid_response↔preknowledge | -0.2 [-6.1, 5.3] | 2.98 | 20 | 0.9366 | 1.0000 |
| quantile_rules | S2 excessive_changes↔lookup | 69.8 [64.9, 74.6] | 2.49 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 34.5 [29.1, 39.9] | 2.84 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 1.6 [-3.3, 6.9] | 2.64 | 20 | 0.5579 | 1.0000 |

## LEGACY-minus-SIM-BL, recall@5 (Holm family of 12)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 14.4 [10.2, 18.5] | 2.18 | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 35.5 [31.1, 39.9] | 2.30 | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 27.8 [22.0, 33.6] | 3.07 | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -2.9 [-6.0, 0.1] | 1.58 | 20 | 0.0895 | 0.2685 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 28.1 [23.3, 32.6] | 2.42 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 50.9 [46.6, 55.0] | 2.22 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -20.1 [-23.1, -17.1] | 1.55 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S4 score_jump↔proxy | -2.0 [-5.2, 1.3] | 1.73 | 20 | 0.4973 | 0.4973 |
| quantile_rules | S1 rapid_response↔preknowledge | 15.4 [9.8, 20.8] | 2.93 | 20 | <0.0001 | 0.0002 |
| quantile_rules | S2 excessive_changes↔lookup | 71.0 [65.9, 75.7] | 2.59 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 34.5 [29.1, 40.0] | 2.84 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 3.3 [-1.5, 8.3] | 2.56 | 20 | 0.2075 | 0.4151 |

## SIM-FI-minus-SIM-BL, pauc_std (Holm family of 12)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.036 [0.023, 0.050] | 0.0070 | 20 | <0.0001 | 0.0003 |
| iforest | S2 excessive_changes↔lookup | 0.221 [0.201, 0.242] | 0.0106 | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.195 [0.172, 0.220] | 0.0124 | 20 | <0.0001 | <0.0001 |
| iforest | S4 score_jump↔proxy | -0.015 [-0.024, -0.006] | 0.0048 | 20 | 0.0065 | 0.0259 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.093 [0.082, 0.104] | 0.0059 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.299 [0.283, 0.314] | 0.0081 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -0.030 [-0.043, -0.017] | 0.0066 | 20 | 0.0003 | 0.0013 |
| iforest_no_peers | S4 score_jump↔proxy | -0.005 [-0.013, 0.004] | 0.0043 | 20 | 0.2879 | 0.5759 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.004 [-0.014, 0.021] | 0.0093 | 20 | 0.6711 | 0.6711 |
| quantile_rules | S2 excessive_changes↔lookup | 0.369 [0.351, 0.387] | 0.0091 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.198 [0.173, 0.220] | 0.0121 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 0.010 [-0.003, 0.023] | 0.0068 | 20 | 0.1481 | 0.4444 |

## LEGACY-minus-SIM-BL, pauc_std (Holm family of 12)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| iforest | S1 rapid_response↔preknowledge | 0.051 [0.035, 0.067] | 0.0084 | 20 | <0.0001 | <0.0001 |
| iforest | S2 excessive_changes↔lookup | 0.154 [0.137, 0.172] | 0.0093 | 20 | <0.0001 | <0.0001 |
| iforest | S3 collusion↔copy_peer | 0.051 [0.020, 0.083] | 0.0166 | 20 | 0.0144 | 0.0431 |
| iforest | S4 score_jump↔proxy | -0.012 [-0.022, -0.000] | 0.0056 | 20 | 0.0076 | 0.0302 |
| iforest_no_peers | S1 rapid_response↔preknowledge | 0.130 [0.110, 0.149] | 0.0101 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S2 excessive_changes↔lookup | 0.234 [0.218, 0.249] | 0.0080 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S3 collusion↔copy_peer | -0.054 [-0.065, -0.043] | 0.0058 | 20 | <0.0001 | <0.0001 |
| iforest_no_peers | S4 score_jump↔proxy | -0.003 [-0.013, 0.008] | 0.0055 | 20 | 0.7240 | 0.7240 |
| quantile_rules | S1 rapid_response↔preknowledge | 0.102 [0.081, 0.122] | 0.0107 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S2 excessive_changes↔lookup | 0.400 [0.381, 0.416] | 0.0092 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S3 collusion↔copy_peer | 0.203 [0.178, 0.227] | 0.0129 | 20 | <0.0001 | <0.0001 |
| quantile_rules | S4 score_jump↔proxy | 0.018 [0.003, 0.033] | 0.0078 | 20 | 0.0239 | 0.0478 |
