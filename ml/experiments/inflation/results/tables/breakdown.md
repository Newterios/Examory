# SIM-BL recall by latent truth sub-mechanism

Each row: that sub-group's cheaters vs all normal test attempts; mean over seeds [95 % CI].

| Sub-mechanism | Detector | Recall@5 % | pAUC (std) | positives/seed |
|---|---|---:|---:|---:|
| copy_peer:role=copier | iforest | 71.3 [66.3, 76.2] | 0.807 [0.783, 0.832] | 16.3 |
| copy_peer:role=copier | iforest_no_peers | 40.8 [35.2, 46.2] | 0.595 [0.575, 0.616] | 16.3 |
| copy_peer:role=copier | quantile_rules | 69.3 [63.4, 75.1] | 0.787 [0.764, 0.811] | 16.3 |
| copy_peer:role=source | iforest | 50.7 [45.5, 55.6] | 0.653 [0.633, 0.672] | 16.3 |
| copy_peer:role=source | iforest_no_peers | 2.2 [0.8, 3.9] | 0.492 [0.488, 0.496] | 16.3 |
| copy_peer:role=source | quantile_rules | 61.7 [56.4, 67.1] | 0.767 [0.743, 0.791] | 16.3 |
| copy_peer:sync_submit=0 | iforest | 36.1 [29.5, 42.9] | 0.625 [0.600, 0.650] | 16.6 |
| copy_peer:sync_submit=0 | iforest_no_peers | 22.1 [19.4, 25.0] | 0.548 [0.535, 0.560] | 16.6 |
| copy_peer:sync_submit=0 | quantile_rules | 39.8 [32.1, 47.6] | 0.651 [0.617, 0.685] | 16.6 |
| copy_peer:sync_submit=1 | iforest | 86.0 [82.5, 89.5] | 0.838 [0.815, 0.861] | 16.0 |
| copy_peer:sync_submit=1 | iforest_no_peers | 21.5 [16.3, 26.5] | 0.543 [0.525, 0.563] | 16.0 |
| copy_peer:sync_submit=1 | quantile_rules | 92.0 [87.6, 96.1] | 0.908 [0.885, 0.930] | 16.0 |
| lookup:mech_flag=lookup_then_answer | iforest | 20.1 [15.0, 26.0] | 0.544 [0.530, 0.560] | 17.7 |
| lookup:mech_flag=lookup_then_answer | iforest_no_peers | 30.6 [26.3, 35.3] | 0.577 [0.561, 0.594] | 17.7 |
| lookup:mech_flag=lookup_then_answer | quantile_rules | 30.4 [25.1, 35.8] | 0.577 [0.562, 0.593] | 17.7 |
| lookup:mech_flag=verify | iforest | 9.2 [6.5, 12.0] | 0.516 [0.506, 0.527] | 18.3 |
| lookup:mech_flag=verify | iforest_no_peers | 16.3 [12.3, 20.4] | 0.523 [0.516, 0.531] | 18.3 |
| lookup:mech_flag=verify | quantile_rules | 25.4 [20.1, 30.8] | 0.565 [0.548, 0.584] | 18.3 |
| preknowledge:mech_flag=disguising | iforest | 10.8 [8.3, 13.6] | 0.520 [0.510, 0.533] | 25.6 |
| preknowledge:mech_flag=disguising | iforest_no_peers | 9.3 [5.9, 12.9] | 0.509 [0.499, 0.520] | 25.6 |
| preknowledge:mech_flag=disguising | quantile_rules | 28.4 [22.2, 35.0] | 0.567 [0.548, 0.587] | 25.6 |
| preknowledge:mech_flag=naive | iforest | 38.6 [33.8, 43.3] | 0.627 [0.606, 0.649] | 24.3 |
| preknowledge:mech_flag=naive | iforest_no_peers | 51.7 [48.1, 55.4] | 0.674 [0.652, 0.695] | 24.3 |
| preknowledge:mech_flag=naive | quantile_rules | 41.0 [36.5, 45.8] | 0.615 [0.598, 0.630] | 24.3 |
