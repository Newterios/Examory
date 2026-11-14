> **Appendix — simulator defaults (uncalibrated), 14 Nov 2026.** Generated with `sim/params/default.json`; superseded by the EdNet-calibrated run in `results/`.

# SIM-BL recall by latent truth sub-mechanism

Each row: that sub-group's cheaters vs all normal test attempts; mean over seeds [95 % CI].

| Sub-mechanism | Detector | Recall@5 % | pAUC (std) | positives/seed |
|---|---|---:|---:|---:|
| copy_peer:role=copier | iforest | 48.4 [41.6, 55.3] | 0.641 [0.615, 0.668] | 16.3 |
| copy_peer:role=copier | iforest_no_peers | 3.7 [1.8, 5.8] | 0.494 [0.490, 0.499] | 16.3 |
| copy_peer:role=copier | quantile_rules | 55.1 [50.1, 60.1] | 0.736 [0.712, 0.762] | 16.3 |
| copy_peer:role=source | iforest | 43.9 [38.1, 49.8] | 0.615 [0.596, 0.636] | 16.3 |
| copy_peer:role=source | iforest_no_peers | 3.5 [1.5, 6.0] | 0.497 [0.490, 0.506] | 16.3 |
| copy_peer:role=source | quantile_rules | 54.4 [49.1, 59.6] | 0.728 [0.703, 0.753] | 16.3 |
| copy_peer:sync_submit=0 | iforest | 19.3 [13.0, 26.0] | 0.545 [0.524, 0.567] | 16.6 |
| copy_peer:sync_submit=0 | iforest_no_peers | 4.8 [2.2, 7.9] | 0.500 [0.492, 0.511] | 16.6 |
| copy_peer:sync_submit=0 | quantile_rules | 22.1 [16.7, 27.4] | 0.581 [0.554, 0.608] | 16.6 |
| copy_peer:sync_submit=1 | iforest | 73.2 [66.4, 79.7] | 0.713 [0.681, 0.745] | 16.0 |
| copy_peer:sync_submit=1 | iforest_no_peers | 2.3 [0.6, 4.4] | 0.491 [0.487, 0.496] | 16.0 |
| copy_peer:sync_submit=1 | quantile_rules | 88.3 [79.5, 95.5] | 0.888 [0.847, 0.924] | 16.0 |
| lookup:mech_flag=lookup_then_answer | iforest | 9.5 [6.3, 13.2] | 0.507 [0.499, 0.516] | 17.7 |
| lookup:mech_flag=lookup_then_answer | iforest_no_peers | 13.1 [10.0, 16.6] | 0.522 [0.513, 0.532] | 17.7 |
| lookup:mech_flag=lookup_then_answer | quantile_rules | 23.8 [19.1, 28.4] | 0.564 [0.547, 0.579] | 17.7 |
| lookup:mech_flag=verify | iforest | 17.2 [12.5, 22.5] | 0.536 [0.521, 0.555] | 18.3 |
| lookup:mech_flag=verify | iforest_no_peers | 25.5 [19.4, 31.9] | 0.546 [0.532, 0.561] | 18.3 |
| lookup:mech_flag=verify | quantile_rules | 27.7 [21.4, 34.5] | 0.574 [0.555, 0.594] | 18.3 |
| preknowledge:mech_flag=disguising | iforest | 7.7 [5.6, 9.8] | 0.506 [0.499, 0.513] | 25.6 |
| preknowledge:mech_flag=disguising | iforest_no_peers | 3.5 [1.7, 5.5] | 0.492 [0.489, 0.497] | 25.6 |
| preknowledge:mech_flag=disguising | quantile_rules | 24.0 [20.4, 27.2] | 0.566 [0.555, 0.576] | 25.6 |
| preknowledge:mech_flag=naive | iforest | 30.3 [26.4, 34.3] | 0.590 [0.575, 0.606] | 24.3 |
| preknowledge:mech_flag=naive | iforest_no_peers | 36.5 [32.9, 40.4] | 0.609 [0.593, 0.626] | 24.3 |
| preknowledge:mech_flag=naive | quantile_rules | 37.3 [31.7, 42.6] | 0.607 [0.589, 0.626] | 24.3 |
