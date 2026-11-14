# Sensitivity of inflation to assumed mechanism parameters

40 Latin-hypercube configurations × 3 seeds. Δ per configuration = mean over its seeds of [metric(SIM-FI) − metric(SIM-BL)] (paired).

| Metric | Detector | Slot | median Δ [IQR] | range | configs Δ>0 | configs Δ>10 pp |
|---|---|---|---:|---:|---:|---:|
| recall@5 | iforest | S1 rapid_response↔preknowledge | 9.05pp [1.23, 20] | -22.8 … 23.5 | 78 % | 42 % |
| recall@5 | iforest | S2 excessive_changes↔lookup | 53.5pp [43.7, 58.8] | 9.95 … 63 | 100 % | 98 % |
| recall@5 | iforest | S3 collusion↔copy_peer | 48.7pp [31, 61.6] | 24.8 … 77 | 100 % | 100 % |
| recall@5 | iforest | S4 score_jump↔proxy | -5.87pp [-7.58, -3.94] | -12.7 … -2.38 | 0 % | 0 % |
| recall@5 | iforest_no_peers | S1 rapid_response↔preknowledge | 20.1pp [10.1, 34.1] | -21.4 … 39.9 | 88 % | 75 % |
| recall@5 | iforest_no_peers | S2 excessive_changes↔lookup | 67.1pp [50.8, 82.4] | 15 … 87.4 | 100 % | 100 % |
| recall@5 | iforest_no_peers | S3 collusion↔copy_peer | -16.4pp [-22.2, -13.7] | -39.2 … -5.74 | 0 % | 0 % |
| recall@5 | iforest_no_peers | S4 score_jump↔proxy | -7.5pp [-8.65, -7.41] | -9.08 … -6.26 | 0 % | 0 % |
| recall@5 | quantile_rules | S1 rapid_response↔preknowledge | -6.06pp [-10, -0.413] | -27.1 … 3.08 | 20 % | 0 % |
| recall@5 | quantile_rules | S2 excessive_changes↔lookup | 62pp [38.8, 84.2] | 10.7 … 94.6 | 100 % | 100 % |
| recall@5 | quantile_rules | S3 collusion↔copy_peer | 51.5pp [25.3, 62.5] | 12.7 … 76.4 | 100 % | 100 % |
| recall@5 | quantile_rules | S4 score_jump↔proxy | -7.28pp [-9.22, -1.15] | -15.8 … 1.23 | 2 % | 0 % |
| pauc_std | iforest | S1 rapid_response↔preknowledge | 0.0435 [0.0051, 0.0864] | -0.0857 … 0.101 | 82 % | NA |
| pauc_std | iforest | S2 excessive_changes↔lookup | 0.231 [0.202, 0.25] | 0.0653 … 0.264 | 100 % | NA |
| pauc_std | iforest | S3 collusion↔copy_peer | 0.235 [0.141, 0.282] | 0.0873 … 0.351 | 100 % | NA |
| pauc_std | iforest | S4 score_jump↔proxy | -0.0189 [-0.0271, -0.017] | -0.0439 … -0.00583 | 0 % | NA |
| pauc_std | iforest_no_peers | S1 rapid_response↔preknowledge | 0.0975 [0.0405, 0.152] | -0.104 … 0.171 | 85 % | NA |
| pauc_std | iforest_no_peers | S2 excessive_changes↔lookup | 0.299 [0.231, 0.336] | 0.0664 … 0.347 | 100 % | NA |
| pauc_std | iforest_no_peers | S3 collusion↔copy_peer | -0.0427 [-0.0719, -0.0289] | -0.144 … -0.00353 | 0 % | NA |
| pauc_std | iforest_no_peers | S4 score_jump↔proxy | -0.01 [-0.0126, -0.0093] | -0.0166 … -0.00851 | 0 % | NA |
| pauc_std | quantile_rules | S1 rapid_response↔preknowledge | -0.026 [-0.0402, -0.00664] | -0.106 … 0.00257 | 5 % | NA |
| pauc_std | quantile_rules | S2 excessive_changes↔lookup | 0.353 [0.251, 0.427] | 0.0799 … 0.454 | 100 % | NA |
| pauc_std | quantile_rules | S3 collusion↔copy_peer | 0.292 [0.158, 0.373] | 0.0889 … 0.421 | 100 % | NA |
| pauc_std | quantile_rules | S4 score_jump↔proxy | -0.00717 [-0.0179, -0.00371] | -0.03 … 0.00262 | 12 % | NA |

## Spearman ρ between parameter and Δ (* nominal p < 0.05, uncorrected; exploratory)

| Metric | Detector | Slot | preknowledge.naive_prob | preknowledge.speedup_high | lookup.duration_median_s | lookup.verify_mode_prob | lookup.propensity | copy_peer.coverage | copy_peer.relay_median_s | copy_peer.start_lag_median_s | proxy.theta_mean | copy_peer.sync_submit_prob |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| recall@5 | iforest | S1 | -0.83* | +0.48* | +0.07 | +0.02 | +0.23 | +0.07 | +0.02 | +0.19 | +0.11 | -0.10 |
| recall@5 | iforest | S2 | +0.08 | +0.05 | -0.55* | +0.17 | -0.78* | -0.14 | -0.12 | -0.37* | -0.09 | +0.04 |
| recall@5 | iforest | S3 | +0.01 | +0.05 | +0.06 | +0.08 | -0.18 | -0.57* | +0.03 | +0.07 | -0.13 | -0.81* |
| recall@5 | iforest | S4 | -0.18 | +0.12 | -0.07 | -0.08 | -0.23 | -0.21 | -0.01 | +0.19 | -0.47* | +0.12 |
| recall@5 | iforest_no_peers | S1 | -0.79* | +0.55* | +0.08 | +0.02 | +0.26 | +0.10 | +0.04 | +0.19 | +0.12 | -0.14 |
| recall@5 | iforest_no_peers | S2 | +0.20 | +0.06 | -0.67* | +0.07 | -0.80* | -0.19 | -0.05 | -0.36* | -0.21 | +0.05 |
| recall@5 | iforest_no_peers | S3 | +0.27 | +0.05 | -0.28 | -0.14 | -0.37* | -0.74* | -0.38* | -0.56* | -0.28 | -0.05 |
| recall@5 | iforest_no_peers | S4 | +0.07 | +0.22 | +0.01 | +0.15 | +0.11 | +0.24 | +0.22 | +0.15 | +0.36* | +0.13 |
| recall@5 | quantile_rules | S1 | -0.83* | +0.41* | +0.05 | -0.07 | +0.19 | +0.05 | +0.02 | +0.09 | +0.04 | -0.08 |
| recall@5 | quantile_rules | S2 | +0.22 | -0.03 | -0.60* | +0.02 | -0.85* | -0.20 | -0.11 | -0.42* | -0.23 | +0.04 |
| recall@5 | quantile_rules | S3 | +0.03 | +0.08 | +0.02 | +0.06 | -0.11 | -0.68* | -0.01 | +0.14 | -0.15 | -0.72* |
| recall@5 | quantile_rules | S4 | +0.07 | -0.07 | -0.16 | -0.04 | -0.36* | -0.19 | -0.11 | -0.02 | -0.81* | +0.02 |
| pauc_std | iforest | S1 | -0.81* | +0.51* | +0.08 | -0.01 | +0.29 | +0.08 | +0.02 | +0.18 | +0.11 | -0.11 |
| pauc_std | iforest | S2 | +0.11 | -0.01 | -0.57* | +0.19 | -0.76* | -0.17 | -0.17 | -0.39* | -0.08 | +0.04 |
| pauc_std | iforest | S3 | +0.05 | +0.02 | +0.06 | +0.05 | -0.18 | -0.63* | -0.02 | +0.03 | -0.13 | -0.77* |
| pauc_std | iforest | S4 | -0.14 | +0.02 | -0.15 | -0.07 | -0.19 | -0.31 | -0.01 | +0.21 | -0.26 | +0.13 |
| pauc_std | iforest_no_peers | S1 | -0.74* | +0.63* | +0.07 | +0.02 | +0.24 | +0.10 | +0.06 | +0.18 | +0.13 | -0.15 |
| pauc_std | iforest_no_peers | S2 | +0.19 | +0.06 | -0.65* | +0.11 | -0.80* | -0.17 | -0.07 | -0.37* | -0.24 | +0.02 |
| pauc_std | iforest_no_peers | S3 | +0.26 | -0.00 | -0.28 | -0.20 | -0.38* | -0.78* | -0.33* | -0.53* | -0.25 | -0.01 |
| pauc_std | iforest_no_peers | S4 | +0.13 | -0.15 | -0.07 | -0.02 | -0.20 | -0.20 | -0.17 | -0.18 | -0.89* | +0.07 |
| pauc_std | quantile_rules | S1 | -0.80* | +0.51* | +0.08 | -0.01 | +0.24 | +0.11 | +0.09 | +0.14 | +0.13 | -0.07 |
| pauc_std | quantile_rules | S2 | +0.19 | -0.01 | -0.55* | +0.03 | -0.87* | -0.16 | -0.12 | -0.41* | -0.26 | +0.08 |
| pauc_std | quantile_rules | S3 | +0.08 | +0.07 | -0.02 | +0.01 | -0.12 | -0.75* | -0.01 | +0.10 | -0.16 | -0.67* |
| pauc_std | quantile_rules | S4 | +0.03 | -0.05 | -0.08 | -0.02 | -0.21 | -0.27 | -0.20 | -0.07 | -0.93* | +0.04 |
