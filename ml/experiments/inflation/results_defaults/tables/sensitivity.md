> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# Sensitivity of inflation to assumed mechanism parameters

40 Latin-hypercube configurations × 3 seeds. Δ per configuration = mean over its seeds of [metric(SIM-FI) − metric(SIM-BL)] (paired).

| Metric | Detector | Slot | median Δ [IQR] | range | configs Δ>0 | configs Δ>10 pp |
|---|---|---|---:|---:|---:|---:|
| recall@5 | iforest | S1 rapid_response↔preknowledge | 18.3pp [6.07, 24.2] | -13.4 … 30.1 | 85 % | 68 % |
| recall@5 | iforest | S2 excessive_changes↔lookup | 71pp [60.4, 76.6] | 27.7 … 79.8 | 100 % | 100 % |
| recall@5 | iforest | S3 collusion↔copy_peer | 55.9pp [34.9, 66.7] | 20.8 … 85.3 | 100 % | 100 % |
| recall@5 | iforest | S4 score_jump↔proxy | -1.84pp [-3.19, -0.218] | -6.06 … 4.14 | 25 % | 0 % |
| recall@5 | iforest_no_peers | S1 rapid_response↔preknowledge | 23.9pp [11, 36.4] | -17.2 … 41 | 88 % | 80 % |
| recall@5 | iforest_no_peers | S2 excessive_changes↔lookup | 78pp [56.3, 89.4] | 27.4 … 94.7 | 100 % | 100 % |
| recall@5 | iforest_no_peers | S3 collusion↔copy_peer | -1.2pp [-6.83, 1.52] | -27.6 … 2.63 | 35 % | 0 % |
| recall@5 | iforest_no_peers | S4 score_jump↔proxy | -2.65pp [-3.82, -0.347] | -5.03 … 1.95 | 20 % | 0 % |
| recall@5 | quantile_rules | S1 rapid_response↔preknowledge | -2.93pp [-8.29, -0.741] | -22 … 2.41 | 12 % | 0 % |
| recall@5 | quantile_rules | S2 excessive_changes↔lookup | 67.4pp [51.9, 85.5] | 25.7 … 92.4 | 100 % | 100 % |
| recall@5 | quantile_rules | S3 collusion↔copy_peer | 60.1pp [34, 70] | 16.9 … 89.1 | 100 % | 100 % |
| recall@5 | quantile_rules | S4 score_jump↔proxy | -7.01pp [-10.6, -1.2] | -20.1 … 2.15 | 8 % | 0 % |
| pauc_std | iforest | S1 rapid_response↔preknowledge | 0.0668 [0.0149, 0.0862] | -0.0668 … 0.103 | 82 % | NA |
| pauc_std | iforest | S2 excessive_changes↔lookup | 0.289 [0.251, 0.301] | 0.154 … 0.313 | 100 % | NA |
| pauc_std | iforest | S3 collusion↔copy_peer | 0.209 [0.131, 0.253] | 0.0247 … 0.318 | 100 % | NA |
| pauc_std | iforest | S4 score_jump↔proxy | 0.000345 [-0.00953, 0.00569] | -0.0223 … 0.0133 | 52 % | NA |
| pauc_std | iforest_no_peers | S1 rapid_response↔preknowledge | 0.0887 [0.0433, 0.128] | -0.0831 … 0.145 | 85 % | NA |
| pauc_std | iforest_no_peers | S2 excessive_changes↔lookup | 0.321 [0.255, 0.347] | 0.133 … 0.36 | 100 % | NA |
| pauc_std | iforest_no_peers | S3 collusion↔copy_peer | 0.000515 [-0.0207, 0.00533] | -0.0918 … 0.0077 | 52 % | NA |
| pauc_std | iforest_no_peers | S4 score_jump↔proxy | -0.000345 [-0.000956, 0.00153] | -0.0037 … 0.00386 | 40 % | NA |
| pauc_std | quantile_rules | S1 rapid_response↔preknowledge | 0.00185 [-0.0154, 0.0107] | -0.0917 … 0.018 | 55 % | NA |
| pauc_std | quantile_rules | S2 excessive_changes↔lookup | 0.396 [0.319, 0.46] | 0.191 … 0.484 | 100 % | NA |
| pauc_std | quantile_rules | S3 collusion↔copy_peer | 0.326 [0.195, 0.408] | 0.0974 … 0.473 | 100 % | NA |
| pauc_std | quantile_rules | S4 score_jump↔proxy | -0.023 [-0.0446, -0.00819] | -0.0677 … 0.0104 | 8 % | NA |

## Spearman ρ between parameter and Δ (* nominal p < 0.05, uncorrected; exploratory)

| Metric | Detector | Slot | preknowledge.naive_prob | preknowledge.speedup_high | lookup.duration_median_s | lookup.verify_mode_prob | lookup.propensity | copy_peer.coverage | copy_peer.relay_median_s | copy_peer.start_lag_median_s | proxy.theta_mean | copy_peer.sync_submit_prob |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| recall@5 | iforest | S1 | -0.69* | +0.65* | +0.10 | +0.01 | +0.27 | +0.06 | +0.07 | +0.20 | +0.15 | -0.17 |
| recall@5 | iforest | S2 | +0.04 | -0.02 | -0.52* | -0.25 | -0.81* | -0.19 | -0.11 | -0.40* | -0.16 | +0.01 |
| recall@5 | iforest | S3 | +0.04 | +0.10 | +0.09 | +0.12 | -0.26 | -0.46* | -0.21 | -0.15 | -0.19 | -0.86* |
| recall@5 | iforest | S4 | -0.29 | -0.17 | +0.10 | -0.10 | +0.13 | +0.11 | -0.12 | -0.07 | -0.41* | +0.09 |
| recall@5 | iforest_no_peers | S1 | -0.76* | +0.62* | +0.06 | +0.03 | +0.22 | +0.11 | +0.03 | +0.17 | +0.15 | -0.15 |
| recall@5 | iforest_no_peers | S2 | +0.13 | -0.02 | -0.66* | -0.24 | -0.79* | -0.17 | -0.09 | -0.43* | -0.17 | +0.06 |
| recall@5 | iforest_no_peers | S3 | +0.11 | -0.04 | -0.11 | -0.10 | -0.43* | +0.16 | -0.45* | -0.88* | -0.13 | +0.05 |
| recall@5 | iforest_no_peers | S4 | -0.03 | -0.21 | -0.17 | +0.01 | -0.25 | -0.22 | -0.45* | -0.29 | -0.83* | -0.02 |
| recall@5 | quantile_rules | S1 | -0.60* | +0.65* | +0.04 | -0.01 | +0.12 | +0.13 | +0.11 | +0.07 | +0.05 | -0.08 |
| recall@5 | quantile_rules | S2 | +0.20 | -0.03 | -0.50* | -0.03 | -0.91* | -0.21 | -0.17 | -0.44* | -0.32* | +0.02 |
| recall@5 | quantile_rules | S3 | +0.08 | +0.13 | +0.04 | +0.07 | -0.22 | -0.66* | -0.16 | -0.07 | -0.24 | -0.72* |
| recall@5 | quantile_rules | S4 | +0.15 | -0.15 | -0.19 | +0.02 | -0.29 | -0.22 | -0.16 | -0.06 | -0.95* | +0.01 |
| pauc_std | iforest | S1 | -0.71* | +0.63* | +0.07 | -0.02 | +0.26 | +0.09 | +0.07 | +0.18 | +0.13 | -0.13 |
| pauc_std | iforest | S2 | +0.11 | -0.07 | -0.50* | -0.16 | -0.86* | -0.25 | -0.21 | -0.39* | -0.25 | -0.03 |
| pauc_std | iforest | S3 | +0.05 | +0.05 | +0.11 | +0.09 | -0.29 | -0.45* | -0.28 | -0.24 | -0.23 | -0.85* |
| pauc_std | iforest | S4 | -0.21 | -0.24 | +0.15 | -0.02 | +0.04 | -0.02 | -0.25 | -0.10 | -0.50* | +0.15 |
| pauc_std | iforest_no_peers | S1 | -0.73* | +0.65* | +0.05 | +0.01 | +0.25 | +0.10 | +0.05 | +0.16 | +0.15 | -0.14 |
| pauc_std | iforest_no_peers | S2 | +0.15 | +0.00 | -0.67* | -0.21 | -0.79* | -0.16 | -0.08 | -0.40* | -0.20 | +0.10 |
| pauc_std | iforest_no_peers | S3 | +0.13 | -0.05 | -0.16 | -0.15 | -0.43* | +0.10 | -0.41* | -0.90* | -0.17 | +0.06 |
| pauc_std | iforest_no_peers | S4 | +0.08 | -0.16 | -0.11 | -0.02 | -0.29 | -0.22 | -0.25 | -0.13 | -0.99* | +0.03 |
| pauc_std | quantile_rules | S1 | -0.63* | +0.68* | +0.05 | +0.06 | +0.18 | +0.04 | +0.13 | +0.13 | +0.12 | -0.10 |
| pauc_std | quantile_rules | S2 | +0.20 | -0.04 | -0.50* | -0.04 | -0.91* | -0.19 | -0.16 | -0.45* | -0.30 | +0.02 |
| pauc_std | quantile_rules | S3 | +0.11 | +0.10 | -0.00 | +0.05 | -0.21 | -0.75* | -0.16 | -0.06 | -0.25 | -0.66* |
| pauc_std | quantile_rules | S4 | +0.04 | -0.09 | -0.11 | -0.00 | -0.22 | -0.22 | -0.19 | -0.07 | -0.98* | +0.03 |
