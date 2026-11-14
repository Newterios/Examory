# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, two-sided Wilcoxon signed-rank). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). Holm correction within each family of 12 slot × detector tests.

## SIM-FI-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 7.8 [2.7, 12.8] | 20 | 0.0112 | 0.1461 |
| lof | S2 excessive_changes↔lookup | 56.0 [51.2, 60.9] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 29.0 [26.1, 31.7] | 20 | <0.0001 | 0.0018 |
| lof | S4 score_jump↔proxy | 0.1 [-2.9, 3.3] | 20 | 0.9249 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | 8.9 [4.4, 14.0] | 20 | 0.0021 | 0.0312 |
| lof_no_peers | S2 excessive_changes↔lookup | 55.9 [51.2, 60.9] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -23.0 [-27.5, -18.4] | 20 | <0.0001 | 0.0018 |
| lof_no_peers | S4 score_jump↔proxy | 1.3 [-1.5, 4.2] | 20 | 0.2759 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | -2.9 [-8.6, 2.3] | 20 | 0.4552 | 1.0000 |
| ocsvm | S2 excessive_changes↔lookup | 56.4 [50.2, 62.7] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 27.0 [23.3, 30.6] | 20 | <0.0001 | 0.0018 |
| ocsvm | S4 score_jump↔proxy | 0.8 [-3.4, 5.0] | 20 | 0.6874 | 1.0000 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | -1.1 [-5.8, 3.4] | 20 | 0.8124 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 53.0 [47.2, 58.9] | 20 | <0.0001 | 0.0018 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -9.6 [-15.1, -3.4] | 20 | 0.0096 | 0.1339 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.6 [-3.2, 4.6] | 20 | 0.5870 | 1.0000 |
| mcd | S1 rapid_response↔preknowledge | 0.6 [-6.8, 8.2] | 20 | 0.9826 | 1.0000 |
| mcd | S2 excessive_changes↔lookup | 37.7 [19.0, 56.5] | 20 | 0.0172 | 0.2062 |
| mcd | S3 collusion↔copy_peer | 29.1 [17.1, 40.1] | 20 | 0.0008 | 0.0133 |
| mcd | S4 score_jump↔proxy | -1.2 [-4.6, 2.6] | 20 | 0.3436 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -6.8 [-12.0, -1.4] | 20 | 0.0400 | 0.4399 |
| mcd_no_peers | S2 excessive_changes↔lookup | 29.5 [10.6, 49.2] | 20 | 0.0836 | 0.8356 |
| mcd_no_peers | S3 collusion↔copy_peer | 30.9 [22.9, 38.6] | 20 | 0.0002 | 0.0031 |
| mcd_no_peers | S4 score_jump↔proxy | -0.7 [-3.6, 2.6] | 20 | 0.5700 | 1.0000 |

## LEGACY-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 11.7 [6.2, 17.0] | 20 | 0.0002 | 0.0023 |
| lof | S2 excessive_changes↔lookup | 56.3 [51.4, 61.0] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 29.0 [26.0, 31.6] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 8.0 [3.6, 12.2] | 20 | 0.0015 | 0.0100 |
| lof_no_peers | S1 rapid_response↔preknowledge | 12.5 [7.3, 17.6] | 20 | <0.0001 | 0.0005 |
| lof_no_peers | S2 excessive_changes↔lookup | 55.5 [51.1, 59.9] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -32.4 [-35.1, -29.7] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 8.3 [4.0, 12.5] | 20 | 0.0014 | 0.0100 |
| ocsvm | S1 rapid_response↔preknowledge | 5.7 [0.4, 10.8] | 20 | 0.0601 | 0.1593 |
| ocsvm | S2 excessive_changes↔lookup | 56.1 [50.3, 61.8] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 27.0 [23.3, 30.5] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 9.9 [5.8, 14.3] | 20 | 0.0003 | 0.0025 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 6.4 [1.5, 11.1] | 20 | 0.0200 | 0.0798 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 53.1 [47.5, 58.5] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -25.5 [-29.6, -21.3] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S4 score_jump↔proxy | 11.8 [6.6, 17.1] | 20 | 0.0003 | 0.0025 |
| mcd | S1 rapid_response↔preknowledge | -16.2 [-22.4, -9.9] | 20 | 0.0001 | 0.0013 |
| mcd | S2 excessive_changes↔lookup | -6.1 [-9.7, -2.6] | 20 | 0.0037 | 0.0186 |
| mcd | S3 collusion↔copy_peer | 24.7 [6.2, 40.2] | 20 | <0.0001 | 0.0011 |
| mcd | S4 score_jump↔proxy | -3.0 [-5.9, -0.2] | 20 | 0.0531 | 0.1593 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -23.8 [-27.5, -20.2] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | -5.9 [-12.6, 3.3] | 20 | 0.0001 | 0.0012 |
| mcd_no_peers | S3 collusion↔copy_peer | -16.2 [-20.7, -11.9] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S4 score_jump↔proxy | -2.4 [-5.6, 0.7] | 20 | 0.2272 | 0.2272 |

## SIM-FI-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 0.022 [0.005, 0.042] | 20 | 0.0362 | 0.4348 |
| lof | S2 excessive_changes↔lookup | 0.296 [0.275, 0.317] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.231 [0.217, 0.245] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.004 [-0.004, 0.013] | 20 | 0.6475 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.027 [0.009, 0.048] | 20 | 0.0073 | 0.1094 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.300 [0.278, 0.322] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -0.113 [-0.131, -0.095] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 0.004 [-0.004, 0.012] | 20 | 0.5277 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | 0.004 [-0.013, 0.022] | 20 | 0.6215 | 1.0000 |
| ocsvm | S2 excessive_changes↔lookup | 0.323 [0.299, 0.345] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.197 [0.180, 0.215] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | -0.001 [-0.014, 0.013] | 20 | 0.5958 | 1.0000 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.014 [-0.003, 0.032] | 20 | 0.2024 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.312 [0.288, 0.335] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -0.056 [-0.077, -0.033] | 20 | 0.0004 | 0.0067 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.003 [-0.009, 0.015] | 20 | 0.9039 | 1.0000 |
| mcd | S1 rapid_response↔preknowledge | 0.008 [-0.019, 0.035] | 20 | 0.6477 | 1.0000 |
| mcd | S2 excessive_changes↔lookup | 0.180 [0.095, 0.264] | 20 | 0.0153 | 0.1991 |
| mcd | S3 collusion↔copy_peer | 0.160 [0.082, 0.230] | 20 | 0.0007 | 0.0113 |
| mcd | S4 score_jump↔proxy | -0.006 [-0.016, 0.005] | 20 | 0.0826 | 0.8255 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -0.016 [-0.036, 0.004] | 20 | 0.1327 | 1.0000 |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.153 [0.060, 0.251] | 20 | 0.0973 | 0.8758 |
| mcd_no_peers | S3 collusion↔copy_peer | 0.041 [0.015, 0.066] | 20 | 0.0107 | 0.1496 |
| mcd_no_peers | S4 score_jump↔proxy | -0.009 [-0.018, -0.000] | 20 | 0.0702 | 0.7717 |

## LEGACY-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 0.096 [0.074, 0.117] | 20 | <0.0001 | <0.0001 |
| lof | S2 excessive_changes↔lookup | 0.352 [0.334, 0.370] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.234 [0.221, 0.248] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.034 [0.022, 0.046] | 20 | <0.0001 | 0.0001 |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.098 [0.077, 0.118] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.351 [0.334, 0.369] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -0.133 [-0.147, -0.118] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 0.034 [0.021, 0.046] | 20 | <0.0001 | 0.0003 |
| ocsvm | S1 rapid_response↔preknowledge | 0.075 [0.054, 0.095] | 20 | <0.0001 | <0.0001 |
| ocsvm | S2 excessive_changes↔lookup | 0.348 [0.330, 0.364] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.197 [0.180, 0.214] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 0.035 [0.022, 0.049] | 20 | <0.0001 | 0.0003 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.081 [0.059, 0.102] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.338 [0.318, 0.356] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -0.097 [-0.113, -0.081] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.046 [0.031, 0.061] | 20 | <0.0001 | <0.0001 |
| mcd | S1 rapid_response↔preknowledge | -0.048 [-0.068, -0.027] | 20 | 0.0018 | 0.0071 |
| mcd | S2 excessive_changes↔lookup | -0.013 [-0.025, -0.001] | 20 | 0.0741 | 0.2222 |
| mcd | S3 collusion↔copy_peer | 0.164 [0.071, 0.242] | 20 | 0.0001 | 0.0006 |
| mcd | S4 score_jump↔proxy | -0.005 [-0.016, 0.005] | 20 | 0.1884 | 0.3768 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -0.080 [-0.092, -0.068] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | -0.030 [-0.046, -0.010] | 20 | 0.0001 | 0.0006 |
| mcd_no_peers | S3 collusion↔copy_peer | -0.074 [-0.091, -0.057] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S4 score_jump↔proxy | -0.004 [-0.015, 0.007] | 20 | 0.4734 | 0.4734 |
