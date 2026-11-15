> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# Inflation of detector performance by feature-aligned injection

Δ = metric(injection world) − metric(SIM-BL). Recall in percentage points; pAUC in standardized-pAUC units. SIM-FI − SIM-BL is paired by seed (paired bootstrap CI, two-sided Wilcoxon signed-rank). LEGACY − SIM-BL is unpaired (independent bootstrap CI, two-sided Mann–Whitney U). Holm correction within each family of 12 slot × detector tests.

## SIM-FI-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | -1.7 [-6.0, 2.6] | 20 | 0.3760 | 1.0000 |
| lof | S2 excessive_changes↔lookup | 63.3 [58.0, 68.1] | 20 | <0.0001 | 0.0017 |
| lof | S3 collusion↔copy_peer | 56.6 [51.7, 61.1] | 20 | <0.0001 | 0.0017 |
| lof | S4 score_jump↔proxy | -2.2 [-6.1, 1.4] | 20 | 0.2094 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | -3.1 [-7.3, 1.0] | 20 | 0.1329 | 0.9901 |
| lof_no_peers | S2 excessive_changes↔lookup | 62.5 [57.5, 67.2] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | 7.9 [3.2, 13.0] | 20 | 0.0090 | 0.0898 |
| lof_no_peers | S4 score_jump↔proxy | -3.0 [-7.6, 1.1] | 20 | 0.3490 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | -2.8 [-6.7, 0.8] | 20 | 0.1625 | 0.9901 |
| ocsvm | S2 excessive_changes↔lookup | 63.0 [59.4, 66.5] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 50.0 [44.9, 54.8] | 20 | <0.0001 | 0.0017 |
| ocsvm | S4 score_jump↔proxy | -6.6 [-9.9, -3.4] | 20 | 0.0015 | 0.0176 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | -0.7 [-4.7, 3.5] | 20 | 0.6874 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 60.2 [55.4, 64.5] | 20 | <0.0001 | 0.0017 |
| ocsvm_no_peers | S3 collusion↔copy_peer | 14.6 [10.6, 19.0] | 20 | 0.0001 | 0.0017 |
| ocsvm_no_peers | S4 score_jump↔proxy | -6.7 [-11.0, -2.3] | 20 | 0.0121 | 0.1088 |
| mcd | S1 rapid_response↔preknowledge | 11.1 [6.7, 15.2] | 20 | 0.0008 | 0.0109 |
| mcd | S2 excessive_changes↔lookup | 57.7 [42.6, 71.5] | 20 | 0.0002 | 0.0027 |
| mcd | S3 collusion↔copy_peer | 44.6 [26.7, 60.5] | 20 | 0.0009 | 0.0116 |
| mcd | S4 score_jump↔proxy | 0.5 [-2.3, 3.2] | 20 | 0.9249 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | 9.7 [7.0, 12.5] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | 68.2 [63.7, 72.8] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S3 collusion↔copy_peer | -3.2 [-4.7, -1.8] | 20 | 0.0022 | 0.0244 |
| mcd_no_peers | S4 score_jump↔proxy | 1.8 [-0.3, 3.7] | 20 | 0.1238 | 0.9901 |

## LEGACY-minus-SIM-BL, recall@5

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 18.7 [13.7, 23.4] | 20 | <0.0001 | <0.0001 |
| lof | S2 excessive_changes↔lookup | 60.6 [55.3, 65.6] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 56.6 [51.8, 61.1] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 3.6 [-0.9, 8.2] | 20 | 0.1759 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | 16.3 [11.8, 20.5] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S2 excessive_changes↔lookup | 59.9 [54.8, 64.8] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -1.3 [-4.4, 1.8] | 20 | 0.4324 | 1.0000 |
| lof_no_peers | S4 score_jump↔proxy | 1.3 [-3.7, 6.1] | 20 | 0.5160 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | 17.1 [12.7, 21.5] | 20 | <0.0001 | <0.0001 |
| ocsvm | S2 excessive_changes↔lookup | 60.5 [56.8, 64.1] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 50.0 [45.0, 54.8] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 1.8 [-3.2, 6.7] | 20 | 0.4648 | 1.0000 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 19.8 [15.5, 23.9] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 58.2 [53.4, 62.7] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | 4.1 [0.5, 7.7] | 20 | 0.0618 | 0.5561 |
| ocsvm_no_peers | S4 score_jump↔proxy | 1.9 [-4.0, 7.6] | 20 | 0.3864 | 1.0000 |
| mcd | S1 rapid_response↔preknowledge | -0.1 [-2.2, 2.0] | 20 | 0.7968 | 1.0000 |
| mcd | S2 excessive_changes↔lookup | -21.7 [-29.1, -13.7] | 20 | 0.0007 | 0.0088 |
| mcd | S3 collusion↔copy_peer | 53.9 [34.8, 70.0] | 20 | <0.0001 | 0.0009 |
| mcd | S4 score_jump↔proxy | 0.9 [-2.3, 3.9] | 20 | 0.6086 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | 2.4 [0.3, 4.4] | 20 | 0.0110 | 0.1099 |
| mcd_no_peers | S2 excessive_changes↔lookup | -23.5 [-31.3, -13.3] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S3 collusion↔copy_peer | 2.1 [-0.5, 4.7] | 20 | 0.1386 | 1.0000 |
| mcd_no_peers | S4 score_jump↔proxy | 3.4 [0.9, 5.8] | 20 | 0.0090 | 0.0993 |

## SIM-FI-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | -0.013 [-0.028, 0.000] | 20 | 0.1054 | 0.7378 |
| lof | S2 excessive_changes↔lookup | 0.383 [0.367, 0.398] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.352 [0.333, 0.369] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | -0.012 [-0.026, 0.001] | 20 | 0.2024 | 1.0000 |
| lof_no_peers | S1 rapid_response↔preknowledge | -0.016 [-0.029, -0.002] | 20 | 0.0441 | 0.3524 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.378 [0.363, 0.393] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | 0.022 [0.006, 0.039] | 20 | 0.0296 | 0.2662 |
| lof_no_peers | S4 score_jump↔proxy | -0.010 [-0.025, 0.003] | 20 | 0.3884 | 1.0000 |
| ocsvm | S1 rapid_response↔preknowledge | -0.002 [-0.018, 0.014] | 20 | 0.8408 | 1.0000 |
| ocsvm | S2 excessive_changes↔lookup | 0.382 [0.367, 0.395] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.312 [0.290, 0.331] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | -0.024 [-0.038, -0.010] | 20 | 0.0094 | 0.1038 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.004 [-0.012, 0.019] | 20 | 0.5958 | 1.0000 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.370 [0.355, 0.384] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.049 [0.034, 0.066] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S4 score_jump↔proxy | -0.023 [-0.040, -0.009] | 20 | 0.0094 | 0.1038 |
| mcd | S1 rapid_response↔preknowledge | 0.037 [0.021, 0.055] | 20 | 0.0003 | 0.0039 |
| mcd | S2 excessive_changes↔lookup | 0.302 [0.211, 0.383] | 20 | 0.0003 | 0.0039 |
| mcd | S3 collusion↔copy_peer | 0.263 [0.168, 0.349] | 20 | 0.0003 | 0.0039 |
| mcd | S4 score_jump↔proxy | 0.004 [-0.003, 0.010] | 20 | 0.2343 | 1.0000 |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.027 [0.021, 0.035] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S2 excessive_changes↔lookup | 0.387 [0.368, 0.406] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S3 collusion↔copy_peer | -0.009 [-0.013, -0.005] | 20 | 0.0016 | 0.0193 |
| mcd_no_peers | S4 score_jump↔proxy | 0.003 [-0.004, 0.010] | 20 | 0.3011 | 1.0000 |

## LEGACY-minus-SIM-BL, pauc_std

| Detector | Slot | Δ mean [95 % CI] | n | p | p (Holm) |
|---|---|---:|---:|---:|---:|
| lof | S1 rapid_response↔preknowledge | 0.104 [0.083, 0.123] | 20 | <0.0001 | <0.0001 |
| lof | S2 excessive_changes↔lookup | 0.353 [0.336, 0.369] | 20 | <0.0001 | <0.0001 |
| lof | S3 collusion↔copy_peer | 0.352 [0.334, 0.369] | 20 | <0.0001 | <0.0001 |
| lof | S4 score_jump↔proxy | 0.015 [-0.001, 0.030] | 20 | 0.0439 | 0.3080 |
| lof_no_peers | S1 rapid_response↔preknowledge | 0.098 [0.078, 0.117] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S2 excessive_changes↔lookup | 0.353 [0.336, 0.369] | 20 | <0.0001 | <0.0001 |
| lof_no_peers | S3 collusion↔copy_peer | -0.002 [-0.014, 0.011] | 20 | 0.8181 | 0.8181 |
| lof_no_peers | S4 score_jump↔proxy | 0.013 [-0.005, 0.029] | 20 | 0.0720 | 0.3604 |
| ocsvm | S1 rapid_response↔preknowledge | 0.101 [0.081, 0.121] | 20 | <0.0001 | <0.0001 |
| ocsvm | S2 excessive_changes↔lookup | 0.347 [0.332, 0.361] | 20 | <0.0001 | <0.0001 |
| ocsvm | S3 collusion↔copy_peer | 0.312 [0.291, 0.332] | 20 | <0.0001 | <0.0001 |
| ocsvm | S4 score_jump↔proxy | 0.005 [-0.013, 0.023] | 20 | 0.3942 | 0.7883 |
| ocsvm_no_peers | S1 rapid_response↔preknowledge | 0.115 [0.094, 0.134] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S2 excessive_changes↔lookup | 0.345 [0.330, 0.360] | 20 | <0.0001 | <0.0001 |
| ocsvm_no_peers | S3 collusion↔copy_peer | 0.015 [0.002, 0.028] | 20 | 0.0385 | 0.3080 |
| ocsvm_no_peers | S4 score_jump↔proxy | 0.013 [-0.007, 0.032] | 20 | 0.0601 | 0.3604 |
| mcd | S1 rapid_response↔preknowledge | 0.007 [-0.000, 0.014] | 20 | 0.0601 | 0.3604 |
| mcd | S2 excessive_changes↔lookup | -0.085 [-0.117, -0.050] | 20 | 0.0030 | 0.0301 |
| mcd | S3 collusion↔copy_peer | 0.313 [0.216, 0.395] | 20 | <0.0001 | 0.0007 |
| mcd | S4 score_jump↔proxy | 0.007 [-0.002, 0.016] | 20 | 0.1371 | 0.4112 |
| mcd_no_peers | S1 rapid_response↔preknowledge | 0.017 [0.009, 0.024] | 20 | <0.0001 | 0.0008 |
| mcd_no_peers | S2 excessive_changes↔lookup | -0.105 [-0.128, -0.078] | 20 | <0.0001 | <0.0001 |
| mcd_no_peers | S3 collusion↔copy_peer | 0.010 [0.003, 0.017] | 20 | 0.0238 | 0.2139 |
| mcd_no_peers | S4 score_jump↔proxy | 0.015 [0.006, 0.025] | 20 | 0.0007 | 0.0075 |
