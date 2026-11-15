# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, exact two-sided sign-flip permutation test on the per-seed differences, all 2^n sign patterns). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). MCSE = Monte Carlo standard error of Δ (paired: SD of the per-seed differences / √n; unpaired: √(s²_x/n_x + s²_y/n_y)). Holm correction within each family (one family per comparison × metric) of 24 slot × detector tests (number of detectors × 4 slots); the family size is printed with each block.

## SIM-FI-minus-SIM-BL, recall@5 (Holm family of 24)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 7.8 [2.7, 12.8] | 2.59 | 20 | 0.0070 | 0.0910 |
| lof | S2 excessive_changes↔lookup | 56.0 [51.2, 60.9] | 2.51 | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 29.0 [26.1, 31.7] | 1.47 | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.1 [-2.9, 3.3] | 1.61 | 20 | 0.9431 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | 8.9 [4.4, 14.0] | 2.52 | 20 | 0.0013 | 0.0200 |
| lof_no_peers | S2 excessive_changes↔lookup | 55.9 [51.2, 60.9] | 2.49 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -23.0 [-27.5, -18.4] | 2.43 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 1.3 [-1.5, 4.2] | 1.51 | 20 | 0.4031 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | -2.9 [-8.6, 2.3] | 2.81 | 20 | 0.3097 | 1.0000 |
| ocsvm | S2 excessive_changes↔lookup | 56.4 [50.2, 62.7] | 3.26 | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 27.0 [23.3, 30.6] | 1.90 | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 0.8 [-3.4, 5.0] | 2.20 | 20 | 0.7319 | 1.0000 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | -1.1 [-5.8, 3.4] | 2.41 | 20 | 0.6606 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 53.0 [47.2, 58.9] | 3.04 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -9.6 [-15.1, -3.4] | 3.11 | 20 | 0.0074 | 0.0910 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.6 [-3.2, 4.6] | 2.06 | 20 | 0.7596 | 1.0000 |
| mcd | S1 rapid_response↔preknowledge | 0.6 [-6.8, 8.2] | 3.97 | 20 | 0.8885 | 1.0000 |
| mcd | S2 excessive_changes↔lookup | 37.7 [19.0, 56.5] | 10.03 | 20 | 0.0019 | 0.0263 |
| mcd | S3 collusion↔copy_peer | 29.1 [17.1, 40.1] | 5.95 | 20 | 0.0003 | 0.0042 |
| mcd | S4 score_jump↔proxy | -1.2 [-4.6, 2.6] | 1.87 | 20 | 0.5328 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -6.8 [-12.0, -1.4] | 2.78 | 20 | 0.0264 | 0.2637 |
| mcd_no_peers | S2 excessive_changes↔lookup | 29.5 [10.6, 49.2] | 10.08 | 20 | 0.0080 | 0.0910 |
| mcd_no_peers | S3 collusion↔copy_peer | 30.9 [22.9, 38.6] | 4.14 | 20 | <0.0001 | 0.0002 |
| mcd_no_peers | S4 score_jump↔proxy | -0.7 [-3.6, 2.6] | 1.62 | 20 | 0.6700 | 1.0000 |

## LEGACY-minus-SIM-BL, recall@5 (Holm family of 24)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 11.7 [6.2, 17.0] | 2.82 | 20 | 0.0002 | 0.0023 |
| lof | S2 excessive_changes↔lookup | 56.3 [51.4, 61.0] | 2.53 | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 29.0 [26.0, 31.6] | 1.47 | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 8.0 [3.6, 12.2] | 2.22 | 20 | 0.0015 | 0.0100 |
| lof_no_peers | S1 rapid_response↔preknowledge | 12.5 [7.3, 17.6] | 2.68 | 20 | <0.0001 | 0.0005 |
| lof_no_peers | S2 excessive_changes↔lookup | 55.5 [51.1, 59.9] | 2.34 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -32.4 [-35.1, -29.7] | 1.44 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 8.3 [4.0, 12.5] | 2.24 | 20 | 0.0014 | 0.0100 |
| ocsvm | S1 rapid_response↔preknowledge | 5.7 [0.4, 10.8] | 2.75 | 20 | 0.0601 | 0.1593 |
| ocsvm | S2 excessive_changes↔lookup | 56.1 [50.3, 61.8] | 3.01 | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 27.0 [23.3, 30.5] | 1.90 | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 9.9 [5.8, 14.3] | 2.17 | 20 | 0.0003 | 0.0025 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 6.4 [1.5, 11.1] | 2.54 | 20 | 0.0200 | 0.0798 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 53.1 [47.5, 58.5] | 2.90 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -25.5 [-29.6, -21.3] | 2.15 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S4 score_jump↔proxy | 11.8 [6.6, 17.1] | 2.71 | 20 | 0.0003 | 0.0025 |
| mcd | S1 rapid_response↔preknowledge | -16.2 [-22.4, -9.9] | 3.30 | 20 | 0.0001 | 0.0013 |
| mcd | S2 excessive_changes↔lookup | -6.1 [-9.7, -2.6] | 1.85 | 20 | 0.0037 | 0.0186 |
| mcd | S3 collusion↔copy_peer | 24.7 [6.2, 40.2] | 8.85 | 20 | <0.0001 | 0.0011 |
| mcd | S4 score_jump↔proxy | -3.0 [-5.9, -0.2] | 1.48 | 20 | 0.0531 | 0.1593 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -23.8 [-27.5, -20.2] | 1.91 | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | -5.9 [-12.6, 3.3] | 4.24 | 20 | 0.0001 | 0.0012 |
| mcd_no_peers | S3 collusion↔copy_peer | -16.2 [-20.7, -11.9] | 2.31 | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S4 score_jump↔proxy | -2.4 [-5.6, 0.7] | 1.61 | 20 | 0.2272 | 0.2272 |

## SIM-FI-minus-SIM-BL, pauc_std (Holm family of 24)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 0.022 [0.005, 0.042] | 0.0097 | 20 | 0.0259 | 0.2845 |
| lof | S2 excessive_changes↔lookup | 0.296 [0.275, 0.317] | 0.0111 | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.231 [0.217, 0.245] | 0.0074 | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.004 [-0.004, 0.013] | 0.0044 | 20 | 0.3971 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.027 [0.009, 0.048] | 0.0101 | 20 | 0.0073 | 0.1017 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.300 [0.278, 0.322] | 0.0115 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -0.113 [-0.131, -0.095] | 0.0092 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 0.004 [-0.004, 0.012] | 0.0043 | 20 | 0.4013 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | 0.004 [-0.013, 0.022] | 0.0091 | 20 | 0.6271 | 1.0000 |
| ocsvm | S2 excessive_changes↔lookup | 0.323 [0.299, 0.345] | 0.0117 | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.197 [0.180, 0.215] | 0.0090 | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | -0.001 [-0.014, 0.013] | 0.0069 | 20 | 0.8771 | 1.0000 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.014 [-0.003, 0.032] | 0.0093 | 20 | 0.1357 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.312 [0.288, 0.335] | 0.0120 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -0.056 [-0.077, -0.033] | 0.0118 | 20 | 0.0004 | 0.0067 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.003 [-0.009, 0.015] | 0.0063 | 20 | 0.6682 | 1.0000 |
| mcd | S1 rapid_response↔preknowledge | 0.008 [-0.019, 0.035] | 0.0141 | 20 | 0.5724 | 1.0000 |
| mcd | S2 excessive_changes↔lookup | 0.180 [0.095, 0.264] | 0.0454 | 20 | 0.0018 | 0.0274 |
| mcd | S3 collusion↔copy_peer | 0.160 [0.082, 0.230] | 0.0382 | 20 | 0.0010 | 0.0166 |
| mcd | S4 score_jump↔proxy | -0.006 [-0.016, 0.005] | 0.0054 | 20 | 0.2906 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -0.016 [-0.036, 0.004] | 0.0105 | 20 | 0.1337 | 1.0000 |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.153 [0.060, 0.251] | 0.0500 | 20 | 0.0074 | 0.1017 |
| mcd_no_peers | S3 collusion↔copy_peer | 0.041 [0.015, 0.066] | 0.0133 | 20 | 0.0078 | 0.1017 |
| mcd_no_peers | S4 score_jump↔proxy | -0.009 [-0.018, -0.000] | 0.0046 | 20 | 0.0621 | 0.6209 |

## LEGACY-minus-SIM-BL, pauc_std (Holm family of 24)

| Detector | Slot | Δ mean [95 % CI] | MCSE | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 0.096 [0.074, 0.117] | 0.0111 | 20 | <0.0001 | <0.0001 |
| lof | S2 excessive_changes↔lookup | 0.352 [0.334, 0.370] | 0.0097 | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.234 [0.221, 0.248] | 0.0071 | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.034 [0.022, 0.046] | 0.0062 | 20 | <0.0001 | 0.0001 |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.098 [0.077, 0.118] | 0.0108 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.351 [0.334, 0.369] | 0.0093 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -0.133 [-0.147, -0.118] | 0.0077 | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S4 score_jump↔proxy | 0.034 [0.021, 0.046] | 0.0065 | 20 | <0.0001 | 0.0003 |
| ocsvm | S1 rapid_response↔preknowledge | 0.075 [0.054, 0.095] | 0.0106 | 20 | <0.0001 | <0.0001 |
| ocsvm | S2 excessive_changes↔lookup | 0.348 [0.330, 0.364] | 0.0088 | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.197 [0.180, 0.214] | 0.0091 | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 0.035 [0.022, 0.049] | 0.0069 | 20 | <0.0001 | 0.0003 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.081 [0.059, 0.102] | 0.0111 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.338 [0.318, 0.356] | 0.0099 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | -0.097 [-0.113, -0.081] | 0.0083 | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.046 [0.031, 0.061] | 0.0076 | 20 | <0.0001 | <0.0001 |
| mcd | S1 rapid_response↔preknowledge | -0.048 [-0.068, -0.027] | 0.0106 | 20 | 0.0018 | 0.0071 |
| mcd | S2 excessive_changes↔lookup | -0.013 [-0.025, -0.001] | 0.0062 | 20 | 0.0741 | 0.2222 |
| mcd | S3 collusion↔copy_peer | 0.164 [0.071, 0.242] | 0.0446 | 20 | 0.0001 | 0.0006 |
| mcd | S4 score_jump↔proxy | -0.005 [-0.016, 0.005] | 0.0053 | 20 | 0.1884 | 0.3768 |
| mcd_no_peers | S1 rapid_response↔preknowledge | -0.080 [-0.092, -0.068] | 0.0063 | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | -0.030 [-0.046, -0.010] | 0.0093 | 20 | 0.0001 | 0.0006 |
| mcd_no_peers | S3 collusion↔copy_peer | -0.074 [-0.091, -0.057] | 0.0088 | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S4 score_jump↔proxy | -0.004 [-0.015, 0.007] | 0.0057 | 20 | 0.4734 | 0.4734 |
