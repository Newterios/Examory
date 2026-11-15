# Sensitivity of inflation to assumed mechanism and injection parameters (v2)

**Replaces v1 after independent review (amendment of 15 Nov 2026); v1 is kept as `stages/sensitivity_v1.json`.** 60 Latin-hypercube configurations × 10 seeds (1001–1010) over 14 parameters (10 behaviour-level as in v1 + 4 injection parameters). Columns decorrelated by within-column swaps (RNG seed 20261115): max |Spearman ρ| between parameter columns 0.261 → 0.100 (target ≤ 0.1). Δ per configuration = mean over its seeds of metric(SIM-FI) − metric(SIM-BL), paired. The default configuration (`sim/params/mechanisms.json`) was run on the same seeds; its Δ is shown with a paired bootstrap CI.

| Metric | Detector | Slot | median Δ [IQR] | range | configs Δ>0 | configs Δ>10 pp | default config Δ [CI] |
|---|---|---|---:|---:|---:|---:|---:|
| recall@5 | IF + peers | S1 rapid_response↔preknowledge | 9.2pp [-7.19, 26.6] | -34.4 … 43.8 | 63 % | 50 % | 10.8 [7.8, 14.3] |
| recall@5 | IF + peers | S2 excessive_changes↔lookup | 40pp [19.4, 51.8] | -2.98 … 72.7 | 98 % | 92 % | 49.4 [43.0, 55.8] |
| recall@5 | IF + peers | S3 collusion↔copy_peer | 41.1pp [22.7, 57.8] | -22.4 … 85.9 | 93 % | 90 % | 39.0 [32.2, 45.8] |
| recall@5 | IF + peers | S4 score_jump↔proxy | -3.28pp [-4.67, -2.15] | -6.81 … 1.25 | 10 % | 0 % | -4.7 [-7.7, -1.7] |
| recall@5 | IF | S1 rapid_response↔preknowledge | 18.1pp [-3.06, 38.7] | -38.6 … 66.4 | 72 % | 63 % | 26.1 [22.6, 29.7] |
| recall@5 | IF | S2 excessive_changes↔lookup | 59pp [38.2, 77.1] | 10.2 … 91 | 100 % | 100 % | 68.4 [63.3, 73.5] |
| recall@5 | IF | S3 collusion↔copy_peer | -13.4pp [-18.8, -7.81] | -33.2 … -0.826 | 0 % | 0 % | -12.0 [-15.3, -8.7] |
| recall@5 | IF | S4 score_jump↔proxy | -1.38pp [-2.78, -0.715] | -4.74 … 0.4 | 5 % | 0 % | -1.1 [-5.9, 3.3] |
| recall@5 | Rules | S1 rapid_response↔preknowledge | -5.88pp [-17.3, 7.2] | -38.6 … 24.8 | 42 % | 18 % | -0.7 [-7.0, 5.2] |
| recall@5 | Rules | S2 excessive_changes↔lookup | 50.3pp [30.5, 72.6] | 3.26 … 94.6 | 100 % | 97 % | 67.7 [60.1, 75.3] |
| recall@5 | Rules | S3 collusion↔copy_peer | 40.2pp [17.9, 58.6] | -31.7 … 80.9 | 92 % | 90 % | 34.6 [27.6, 40.6] |
| recall@5 | Rules | S4 score_jump↔proxy | 0.193pp [-4.84, 3.63] | -10.7 … 12.8 | 50 % | 7 % | -2.3 [-7.9, 3.4] |
| pauc_std | IF + peers | S1 rapid_response↔preknowledge | 0.0257 [-0.0356, 0.101] | -0.143 … 0.173 | 58 % | NA | 0.038 [0.018, 0.059] |
| pauc_std | IF + peers | S2 excessive_changes↔lookup | 0.158 [0.079, 0.22] | 0.00103 … 0.313 | 100 % | NA | 0.211 [0.187, 0.235] |
| pauc_std | IF + peers | S3 collusion↔copy_peer | 0.213 [0.115, 0.277] | -0.0922 … 0.388 | 93 % | NA | 0.192 [0.158, 0.227] |
| pauc_std | IF + peers | S4 score_jump↔proxy | -0.0124 [-0.0164, -0.00873] | -0.0241 … -0.00303 | 0 % | NA | -0.017 [-0.027, -0.006] |
| pauc_std | IF | S1 rapid_response↔preknowledge | 0.0646 [-0.026, 0.168] | -0.191 … 0.282 | 68 % | NA | 0.096 [0.085, 0.109] |
| pauc_std | IF | S2 excessive_changes↔lookup | 0.242 [0.141, 0.33] | 0.0266 … 0.406 | 100 % | NA | 0.298 [0.280, 0.315] |
| pauc_std | IF | S3 collusion↔copy_peer | -0.0344 [-0.058, -0.0171] | -0.143 … 0.00233 | 5 % | NA | -0.035 [-0.043, -0.027] |
| pauc_std | IF | S4 score_jump↔proxy | -0.000528 [-0.00265, 0.000717] | -0.00752 … 0.00325 | 42 % | NA | -0.000 [-0.012, 0.013] |
| pauc_std | Rules | S1 rapid_response↔preknowledge | -0.0283 [-0.071, 0.0348] | -0.152 … 0.122 | 38 % | NA | -0.003 [-0.028, 0.023] |
| pauc_std | Rules | S2 excessive_changes↔lookup | 0.273 [0.147, 0.375] | 0.00753 … 0.464 | 100 % | NA | 0.363 [0.339, 0.390] |
| pauc_std | Rules | S3 collusion↔copy_peer | 0.227 [0.119, 0.333] | -0.157 … 0.429 | 93 % | NA | 0.198 [0.163, 0.227] |
| pauc_std | Rules | S4 score_jump↔proxy | 0.00159 [-0.0108, 0.0119] | -0.0321 … 0.0386 | 53 % | NA | -0.002 [-0.018, 0.015] |

## Recall per configuration: median [min, max] over configurations, and the default configuration

| Detector | Slot | SIM-FI recall % | SIM-BL recall % | default SIM-FI | default SIM-BL |
|---|---|---:|---:|---:|---:|
| IF + peers | S1 rapid_response↔preknowledge | 30.6 [12.3, 61.5] | 22.8 [9.6, 49.6] | 35.4 [32.3, 38.5] | 24.6 [21.6, 27.7] |
| IF + peers | S2 excessive_changes↔lookup | 57.0 [25.2, 79.8] | 12.8 [5.7, 66.7] | 63.5 [57.0, 69.4] | 14.1 [9.8, 18.5] |
| IF + peers | S3 collusion↔copy_peer | 100.0 [56.9, 100.0] | 52.0 [14.1, 80.0] | 100.0 [100.0, 100.0] | 61.0 [54.2, 67.8] |
| IF + peers | S4 score_jump↔proxy | 4.1 [1.7, 7.8] | 7.6 [3.8, 11.1] | 3.2 [1.0, 5.8] | 8.0 [6.1, 9.9] |
| IF | S1 rapid_response↔preknowledge | 50.4 [17.9, 78.5] | 28.2 [9.6, 61.8] | 55.7 [51.7, 59.7] | 29.6 [26.3, 33.2] |
| IF | S2 excessive_changes↔lookup | 84.9 [43.0, 96.8] | 15.7 [4.1, 80.8] | 91.1 [87.7, 94.0] | 22.6 [19.0, 27.0] |
| IF | S3 collusion↔copy_peer | 7.3 [6.5, 7.6] | 20.5 [7.9, 40.3] | 7.3 [4.4, 10.4] | 19.3 [16.0, 22.5] |
| IF | S4 score_jump↔proxy | 5.1 [4.3, 5.6] | 6.5 [5.2, 9.5] | 5.1 [3.4, 7.0] | 6.2 [2.7, 10.5] |
| Rules | S1 rapid_response↔preknowledge | 30.3 [10.5, 57.8] | 34.1 [28.5, 53.0] | 33.2 [28.4, 38.2] | 33.9 [27.4, 40.5] |
| Rules | S2 excessive_changes↔lookup | 91.8 [43.7, 100.0] | 29.6 [5.0, 89.4] | 97.9 [96.5, 99.2] | 30.2 [22.2, 38.2] |
| Rules | S3 collusion↔copy_peer | 100.0 [58.4, 100.0] | 54.5 [19.1, 90.3] | 100.0 [100.0, 100.0] | 65.4 [59.4, 72.4] |
| Rules | S4 score_jump↔proxy | 8.6 [4.9, 17.4] | 11.2 [3.8, 18.3] | 8.1 [5.5, 10.9] | 10.4 [5.6, 15.1] |

## Partial rank correlation (PRCC, all 14 parameters jointly) between parameter and Δ (* nominal p < 0.05; 336 tests, uncorrected; exploratory)

| Metric | Detector | Slot | preknowledge.naive_prob | preknowledge.speedup_high | lookup.duration_median_s | lookup.verify_mode_prob | lookup.propensity | copy_peer.coverage | copy_peer.relay_median_s | copy_peer.start_lag_median_s | proxy.theta_mean | copy_peer.sync_submit_prob | fi.rapid_item_prob | fi.changes_prob_multiplier | fi.collusion_submit_window_ms | fi.score_jump_multiplier |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| recall@5 | IF + peers | S1 | -0.91* | +0.85* | +0.26 | +0.09 | -0.13 | +0.09 | +0.11 | -0.06 | -0.14 | -0.12 | +0.97* | +0.11 | +0.17 | +0.09 |
| recall@5 | IF + peers | S2 | -0.18 | +0.11 | -0.61* | +0.60* | -0.76* | -0.12 | -0.04 | -0.06 | +0.04 | -0.10 | -0.06 | +0.92* | -0.15 | -0.20 |
| recall@5 | IF + peers | S3 | +0.12 | -0.04 | +0.17 | +0.07 | +0.08 | -0.77* | +0.37* | -0.30* | +0.17 | -0.88* | -0.04 | -0.21 | -0.76* | -0.34* |
| recall@5 | IF + peers | S4 | -0.20 | +0.02 | +0.05 | -0.27 | +0.05 | +0.12 | -0.06 | -0.00 | -0.71* | -0.31* | -0.10 | -0.33* | +0.14 | +0.63* |
| recall@5 | IF | S1 | -0.94* | +0.88* | +0.12 | +0.25 | -0.19 | +0.02 | -0.03 | -0.16 | -0.06 | -0.05 | +0.97* | +0.19 | +0.10 | +0.18 |
| recall@5 | IF | S2 | -0.25 | -0.10 | -0.83* | +0.48* | -0.84* | -0.20 | -0.02 | +0.16 | -0.00 | -0.07 | -0.11 | +0.89* | -0.11 | -0.15 |
| recall@5 | IF | S3 | -0.25 | +0.10 | -0.08 | +0.05 | +0.14 | -0.94* | +0.04 | -0.86* | +0.04 | -0.21 | +0.08 | -0.04 | +0.02 | +0.29 |
| recall@5 | IF | S4 | -0.03 | +0.11 | -0.28 | -0.22 | +0.35* | -0.05 | -0.05 | -0.31* | -0.94* | -0.09 | +0.09 | +0.06 | +0.02 | +0.47* |
| recall@5 | Rules | S1 | -0.81* | +0.81* | +0.15 | -0.06 | -0.08 | +0.18 | +0.14 | -0.13 | -0.10 | -0.14 | +0.98* | +0.16 | -0.05 | +0.06 |
| recall@5 | Rules | S2 | -0.26 | +0.05 | -0.83* | +0.28 | -0.90* | -0.25 | -0.09 | +0.06 | +0.00 | +0.07 | -0.19 | +0.90* | -0.09 | -0.12 |
| recall@5 | Rules | S3 | +0.15 | +0.05 | +0.18 | +0.06 | +0.07 | -0.84* | +0.33* | -0.29* | +0.19 | -0.88* | +0.02 | -0.16 | -0.75* | -0.35* |
| recall@5 | Rules | S4 | +0.35* | +0.10 | -0.02 | +0.04 | +0.10 | -0.04 | +0.03 | -0.27 | -0.94* | -0.10 | -0.16 | -0.07 | +0.05 | +0.94* |
| pauc_std | IF + peers | S1 | -0.88* | +0.85* | +0.20 | +0.02 | -0.10 | +0.12 | +0.12 | -0.02 | -0.07 | -0.15 | +0.96* | +0.08 | +0.11 | +0.16 |
| pauc_std | IF + peers | S2 | -0.17 | +0.10 | -0.62* | +0.59* | -0.74* | -0.12 | -0.04 | -0.12 | +0.04 | -0.07 | -0.09 | +0.94* | -0.24 | -0.16 |
| pauc_std | IF + peers | S3 | +0.16 | -0.05 | +0.21 | +0.03 | +0.07 | -0.82* | +0.32* | -0.36* | +0.16 | -0.87* | -0.08 | -0.24 | -0.75* | -0.32* |
| pauc_std | IF + peers | S4 | -0.15 | +0.03 | +0.02 | -0.10 | +0.12 | +0.05 | -0.12 | -0.07 | -0.63* | -0.20 | -0.30* | -0.32* | +0.19 | +0.57* |
| pauc_std | IF | S1 | -0.89* | +0.85* | +0.15 | +0.08 | -0.12 | -0.01 | -0.01 | -0.12 | -0.09 | -0.08 | +0.95* | +0.13 | +0.08 | +0.13 |
| pauc_std | IF | S2 | -0.22 | +0.04 | -0.78* | +0.57* | -0.78* | -0.20 | -0.00 | -0.01 | -0.01 | -0.01 | -0.04 | +0.93* | -0.25 | -0.20 |
| pauc_std | IF | S3 | -0.26 | +0.09 | -0.06 | +0.01 | +0.28 | -0.94* | +0.03 | -0.88* | -0.02 | -0.07 | +0.04 | -0.04 | -0.00 | +0.16 |
| pauc_std | IF | S4 | +0.16 | -0.10 | -0.20 | -0.03 | -0.03 | -0.13 | -0.15 | -0.01 | -0.95* | +0.24 | +0.09 | -0.16 | -0.08 | +0.88* |
| pauc_std | Rules | S1 | -0.73* | +0.78* | +0.27 | -0.13 | -0.01 | +0.26 | +0.18 | -0.12 | -0.12 | -0.20 | +0.97* | +0.12 | -0.03 | +0.12 |
| pauc_std | Rules | S2 | -0.31* | +0.04 | -0.77* | +0.33* | -0.87* | -0.23 | -0.15 | -0.07 | +0.04 | +0.06 | -0.21 | +0.94* | -0.24 | -0.15 |
| pauc_std | Rules | S3 | +0.13 | +0.02 | +0.16 | +0.05 | +0.05 | -0.88* | +0.23 | -0.26 | +0.21 | -0.86* | +0.02 | -0.18 | -0.73* | -0.30* |
| pauc_std | Rules | S4 | +0.24 | +0.18 | -0.03 | -0.08 | +0.03 | +0.01 | -0.11 | -0.07 | -0.96* | -0.01 | -0.15 | -0.06 | +0.13 | +0.94* |
