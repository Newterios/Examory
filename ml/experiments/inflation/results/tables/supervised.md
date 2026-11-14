# Post hoc: supervised detectors on the frozen feature set (exploratory)

**Added after the main results had been seen (amendment of 15 Nov 2026); exploratory, p-values unadjusted.** Feature table: the frozen pipeline's (`build_features` with the training world's normal-train reference, `train.matrix`), 15 features (+ peers) or 12 attempt features. Models with scikit-learn defaults, no tuning: HistGradientBoostingClassifier (`random_state = seed`, primary) and LogisticRegression on standardized features (secondary). Labels: any cheating vs none, training world's train cohorts. Threshold: `choose_threshold` (max recall s.t. FPR ≤ 5 %) on the training world's validation cohorts. Metrics on the target world's test cohorts. Mean over seeds [95 % percentile-bootstrap CI over seeds]; recall and FPR in %.

SIM-BL test data identical to the main stage's (CSV SHA-256 prefix) in 20 of 20 seeds.

## Recall (%) at the training-world validation 5 % FPR threshold

| Model | Train → test | realized FPR | all | S1 rapid_response↔preknowledge | S2 excessive_changes↔lookup | S3 collusion↔copy_peer | S4 score_jump↔proxy |
|---|---|---:|---:|---:|---:|---:|---:|
| HGB + peers (15) | FI → FI | 5.3 [4.8, 5.9] | 79.8 [78.3, 81.4] | 79.9 [77.3, 82.4] | 99.0 [98.4, 99.6] | 99.4 [98.2, 100.0] | 21.9 [17.2, 26.5] |
| HGB + peers (15) | FI → BL | 5.3 [4.8, 5.9] | 45.1 [42.6, 47.7] | 52.8 [49.1, 56.5] | 29.1 [24.4, 33.6] | 64.4 [59.0, 70.0] | 24.8 [21.4, 28.5] |
| HGB + peers (15) | BL → FI | 5.5 [5.0, 6.0] | 48.7 [46.0, 51.3] | 33.9 [30.8, 37.1] | 27.5 [22.7, 32.4] | 100.0 [100.0, 100.0] | 41.7 [37.8, 45.5] |
| HGB + peers (15) | BL → BL | 5.6 [5.1, 6.0] | 69.6 [67.5, 71.9] | 77.2 [74.5, 80.0] | 75.6 [71.6, 79.6] | 75.2 [71.3, 79.1] | 34.5 [29.2, 39.9] |
| HGB + peers (15) | LEGACY → FI | 14.2 [12.1, 16.5] | 85.0 [83.5, 86.5] | 89.5 [87.1, 91.7] | 99.7 [99.1, 100.0] | 98.9 [97.8, 99.8] | 32.2 [28.2, 36.0] |
| HGB + peers (15) | LEGACY → BL | 14.3 [12.1, 16.6] | 57.2 [53.9, 60.5] | 66.2 [62.2, 70.3] | 38.9 [33.3, 44.5] | 77.5 [72.9, 82.2] | 38.8 [33.7, 44.3] |
| HGB (12) | FI → FI | 5.2 [4.8, 5.7] | 70.7 [69.2, 72.2] | 77.2 [75.3, 79.0] | 98.1 [97.2, 99.0] | 70.3 [66.6, 74.4] | 14.8 [11.9, 17.5] |
| HGB (12) | FI → BL | 5.2 [4.8, 5.7] | 32.7 [30.8, 34.7] | 45.7 [43.0, 48.4] | 21.7 [18.3, 25.0] | 37.2 [33.3, 41.3] | 15.7 [12.8, 18.7] |
| HGB (12) | BL → FI | 5.3 [4.9, 5.7] | 27.7 [25.2, 30.2] | 37.6 [33.6, 41.3] | 30.2 [24.9, 35.6] | 2.2 [0.6, 4.5] | 39.0 [34.9, 43.3] |
| HGB (12) | BL → BL | 5.3 [4.9, 5.7] | 61.5 [59.7, 63.4] | 74.4 [71.3, 77.6] | 73.9 [69.7, 78.0] | 45.3 [42.4, 48.0] | 34.7 [30.2, 39.2] |
| HGB (12) | LEGACY → FI | 15.4 [13.1, 17.9] | 66.4 [63.9, 68.9] | 88.8 [85.8, 91.6] | 99.8 [99.5, 100.0] | 21.9 [17.0, 26.9] | 26.8 [22.1, 31.3] |
| HGB (12) | LEGACY → BL | 15.4 [13.1, 17.9] | 45.0 [41.6, 48.8] | 59.6 [56.0, 63.8] | 29.6 [23.9, 35.3] | 45.0 [40.5, 49.6] | 36.6 [30.1, 43.5] |
| LogReg + peers (15) | FI → FI | 5.0 [4.6, 5.4] | 79.4 [77.7, 81.0] | 83.1 [80.3, 85.7] | 98.9 [98.3, 99.6] | 99.7 [99.0, 100.0] | 11.4 [8.3, 14.5] |
| LogReg + peers (15) | FI → BL | 5.1 [4.6, 5.5] | 43.9 [41.6, 46.3] | 47.5 [45.3, 49.7] | 29.6 [24.8, 34.4] | 72.8 [69.7, 75.8] | 17.2 [14.1, 20.1] |
| LogReg + peers (15) | BL → FI | 5.4 [5.0, 5.8] | 52.5 [49.6, 55.3] | 34.9 [31.6, 38.3] | 44.0 [38.8, 49.0] | 99.5 [98.5, 100.0] | 38.0 [33.5, 42.7] |
| LogReg + peers (15) | BL → BL | 5.4 [5.0, 5.8] | 69.5 [67.6, 71.4] | 77.3 [74.8, 79.9] | 77.5 [73.8, 81.2] | 71.8 [67.9, 75.8] | 35.4 [30.8, 40.5] |
| LogReg + peers (15) | LEGACY → FI | 40.8 [37.7, 43.7] | 94.7 [93.3, 95.9] | 98.0 [96.4, 99.4] | 99.7 [99.2, 100.0] | 100.0 [100.0, 100.0] | 71.8 [65.8, 77.2] |
| LogReg + peers (15) | LEGACY → BL | 40.8 [37.7, 43.7] | 83.5 [80.9, 85.9] | 88.3 [85.1, 91.2] | 75.1 [70.7, 79.3] | 90.4 [87.2, 93.3] | 77.3 [72.9, 81.8] |
| LogReg (12) | FI → FI | 5.6 [5.2, 6.0] | 65.8 [63.6, 67.9] | 80.2 [78.1, 82.5] | 99.1 [98.5, 99.6] | 49.5 [42.7, 56.5] | 5.9 [3.9, 8.2] |
| LogReg (12) | FI → BL | 5.6 [5.2, 6.0] | 28.0 [26.2, 29.8] | 34.7 [32.4, 36.9] | 13.2 [10.2, 16.2] | 45.9 [42.5, 49.3] | 11.1 [8.1, 14.1] |
| LogReg (12) | BL → FI | 5.2 [4.8, 5.6] | 33.1 [30.5, 35.4] | 41.1 [38.2, 44.0] | 49.7 [44.9, 54.4] | 0.1 [0.0, 0.3] | 35.8 [31.6, 40.0] |
| LogReg (12) | BL → BL | 5.2 [4.8, 5.6] | 62.5 [60.2, 64.9] | 77.4 [74.7, 80.2] | 77.3 [73.3, 81.2] | 42.6 [39.2, 46.0] | 34.3 [29.6, 39.6] |
| LogReg (12) | LEGACY → FI | 59.4 [57.1, 61.3] | 96.6 [95.1, 97.9] | 99.8 [99.5, 100.0] | 100.0 [100.0, 100.0] | 93.9 [89.8, 97.2] | 88.3 [83.6, 92.3] |
| LogReg (12) | LEGACY → BL | 59.4 [57.1, 61.3] | 92.5 [90.4, 94.3] | 93.3 [90.7, 95.4] | 87.8 [84.5, 90.8] | 96.2 [94.7, 97.7] | 93.8 [90.8, 96.2] |

## Standardized partial AUC, FPR ∈ [0, 0.05] (McClish; 0.5 = chance)

| Model | Train → test | all | S1 rapid_response↔preknowledge | S2 excessive_changes↔lookup | S3 collusion↔copy_peer | S4 score_jump↔proxy |
|---|---|---:|---:|---:|---:|---:|
| HGB + peers (15) | FI → FI | 0.861 [0.853, 0.868] | 0.831 [0.819, 0.844] | 0.989 [0.985, 0.992] | 0.994 [0.987, 0.999] | 0.535 [0.524, 0.547] |
| HGB + peers (15) | FI → BL | 0.646 [0.634, 0.659] | 0.668 [0.654, 0.683] | 0.561 [0.548, 0.575] | 0.774 [0.745, 0.803] | 0.547 [0.534, 0.562] |
| HGB + peers (15) | BL → FI | 0.680 [0.667, 0.692] | 0.588 [0.575, 0.601] | 0.565 [0.549, 0.584] | 0.999 [0.998, 1.000] | 0.608 [0.589, 0.628] |
| HGB + peers (15) | BL → BL | 0.761 [0.744, 0.778] | 0.777 [0.754, 0.799] | 0.772 [0.743, 0.800] | 0.838 [0.817, 0.859] | 0.592 [0.572, 0.613] |
| HGB + peers (15) | LEGACY → FI | 0.803 [0.776, 0.828] | 0.759 [0.714, 0.798] | 0.975 [0.960, 0.986] | 0.892 [0.845, 0.932] | 0.506 [0.499, 0.514] |
| HGB + peers (15) | LEGACY → BL | 0.584 [0.574, 0.595] | 0.601 [0.584, 0.617] | 0.510 [0.503, 0.518] | 0.689 [0.665, 0.712] | 0.521 [0.511, 0.531] |
| HGB (12) | FI → FI | 0.789 [0.782, 0.795] | 0.813 [0.802, 0.825] | 0.985 [0.980, 0.990] | 0.728 [0.713, 0.744] | 0.515 [0.508, 0.522] |
| HGB (12) | FI → BL | 0.590 [0.583, 0.598] | 0.635 [0.623, 0.648] | 0.538 [0.529, 0.547] | 0.629 [0.611, 0.647] | 0.520 [0.512, 0.528] |
| HGB (12) | BL → FI | 0.574 [0.564, 0.587] | 0.610 [0.596, 0.624] | 0.584 [0.561, 0.610] | 0.491 [0.488, 0.496] | 0.603 [0.586, 0.620] |
| HGB (12) | BL → BL | 0.721 [0.708, 0.734] | 0.778 [0.758, 0.799] | 0.767 [0.743, 0.792] | 0.671 [0.658, 0.684] | 0.588 [0.569, 0.608] |
| HGB (12) | LEGACY → FI | 0.729 [0.718, 0.741] | 0.794 [0.779, 0.810] | 0.985 [0.980, 0.989] | 0.504 [0.497, 0.511] | 0.501 [0.496, 0.508] |
| HGB (12) | LEGACY → BL | 0.561 [0.554, 0.568] | 0.600 [0.590, 0.610] | 0.511 [0.501, 0.521] | 0.586 [0.571, 0.602] | 0.520 [0.510, 0.530] |
| LogReg + peers (15) | FI → FI | 0.855 [0.847, 0.862] | 0.830 [0.818, 0.842] | 0.980 [0.976, 0.984] | 0.994 [0.987, 0.998] | 0.513 [0.505, 0.522] |
| LogReg + peers (15) | FI → BL | 0.653 [0.643, 0.663] | 0.663 [0.650, 0.675] | 0.571 [0.558, 0.586] | 0.809 [0.792, 0.825] | 0.536 [0.526, 0.548] |
| LogReg + peers (15) | BL → FI | 0.701 [0.688, 0.714] | 0.602 [0.590, 0.616] | 0.640 [0.617, 0.664] | 0.990 [0.981, 0.997] | 0.603 [0.585, 0.621] |
| LogReg + peers (15) | BL → BL | 0.759 [0.747, 0.772] | 0.788 [0.772, 0.804] | 0.778 [0.754, 0.802] | 0.809 [0.790, 0.828] | 0.590 [0.573, 0.607] |
| LogReg + peers (15) | LEGACY → FI | 0.703 [0.685, 0.720] | 0.686 [0.672, 0.701] | 0.848 [0.833, 0.863] | 0.694 [0.654, 0.736] | 0.513 [0.502, 0.526] |
| LogReg + peers (15) | LEGACY → BL | 0.604 [0.595, 0.613] | 0.608 [0.597, 0.619] | 0.529 [0.518, 0.540] | 0.723 [0.704, 0.742] | 0.543 [0.531, 0.555] |
| LogReg (12) | FI → FI | 0.749 [0.742, 0.757] | 0.795 [0.787, 0.803] | 0.972 [0.967, 0.977] | 0.610 [0.591, 0.631] | 0.499 [0.493, 0.506] |
| LogReg (12) | FI → BL | 0.582 [0.574, 0.591] | 0.599 [0.586, 0.612] | 0.524 [0.514, 0.534] | 0.674 [0.662, 0.685] | 0.509 [0.502, 0.517] |
| LogReg (12) | BL → FI | 0.602 [0.590, 0.615] | 0.632 [0.618, 0.646] | 0.668 [0.643, 0.692] | 0.488 [0.487, 0.488] | 0.597 [0.580, 0.615] |
| LogReg (12) | BL → BL | 0.724 [0.711, 0.738] | 0.788 [0.773, 0.803] | 0.782 [0.758, 0.805] | 0.655 [0.641, 0.669] | 0.590 [0.574, 0.608] |
| LogReg (12) | LEGACY → FI | 0.678 [0.669, 0.689] | 0.743 [0.732, 0.754] | 0.853 [0.836, 0.868] | 0.508 [0.499, 0.518] | 0.503 [0.497, 0.510] |
| LogReg (12) | LEGACY → BL | 0.547 [0.542, 0.552] | 0.582 [0.572, 0.593] | 0.512 [0.506, 0.517] | 0.549 [0.535, 0.563] | 0.524 [0.512, 0.536] |

## ROC-AUC

| Model | Train → test | all | S1 rapid_response↔preknowledge | S2 excessive_changes↔lookup | S3 collusion↔copy_peer | S4 score_jump↔proxy |
|---|---|---:|---:|---:|---:|---:|
| HGB + peers (15) | FI → FI | 0.954 [0.950, 0.957] | 0.958 [0.953, 0.963] | 0.997 [0.995, 0.999] | 0.998 [0.995, 1.000] | 0.812 [0.800, 0.823] |
| HGB + peers (15) | FI → BL | 0.875 [0.868, 0.881] | 0.911 [0.901, 0.920] | 0.851 [0.836, 0.865] | 0.890 [0.867, 0.912] | 0.806 [0.792, 0.820] |
| HGB + peers (15) | BL → FI | 0.866 [0.853, 0.879] | 0.828 [0.812, 0.843] | 0.800 [0.772, 0.828] | 1.000 [1.000, 1.000] | 0.861 [0.850, 0.872] |
| HGB + peers (15) | BL → BL | 0.933 [0.928, 0.938] | 0.953 [0.946, 0.959] | 0.952 [0.943, 0.960] | 0.931 [0.921, 0.941] | 0.862 [0.848, 0.877] |
| HGB + peers (15) | LEGACY → FI | 0.934 [0.928, 0.941] | 0.943 [0.934, 0.952] | 0.996 [0.993, 0.998] | 0.987 [0.981, 0.992] | 0.746 [0.729, 0.763] |
| HGB + peers (15) | LEGACY → BL | 0.838 [0.827, 0.849] | 0.870 [0.856, 0.885] | 0.781 [0.759, 0.802] | 0.896 [0.880, 0.911] | 0.781 [0.765, 0.796] |
| HGB (12) | FI → FI | 0.933 [0.929, 0.937] | 0.952 [0.947, 0.957] | 0.996 [0.993, 0.998] | 0.948 [0.939, 0.956] | 0.774 [0.763, 0.786] |
| HGB (12) | FI → BL | 0.836 [0.829, 0.844] | 0.887 [0.877, 0.898] | 0.819 [0.804, 0.832] | 0.813 [0.799, 0.828] | 0.780 [0.764, 0.796] |
| HGB (12) | BL → FI | 0.775 [0.763, 0.786] | 0.834 [0.821, 0.847] | 0.819 [0.798, 0.841] | 0.578 [0.561, 0.595] | 0.853 [0.840, 0.867] |
| HGB (12) | BL → BL | 0.911 [0.906, 0.917] | 0.951 [0.946, 0.956] | 0.951 [0.942, 0.959] | 0.846 [0.836, 0.855] | 0.852 [0.835, 0.868] |
| HGB (12) | LEGACY → FI | 0.861 [0.853, 0.869] | 0.947 [0.941, 0.954] | 0.997 [0.995, 0.999] | 0.681 [0.651, 0.708] | 0.707 [0.691, 0.723] |
| HGB (12) | LEGACY → BL | 0.779 [0.768, 0.791] | 0.842 [0.831, 0.852] | 0.715 [0.690, 0.740] | 0.773 [0.751, 0.793] | 0.755 [0.735, 0.774] |
| LogReg + peers (15) | FI → FI | 0.948 [0.943, 0.953] | 0.968 [0.964, 0.971] | 0.997 [0.996, 0.998] | 0.999 [0.999, 1.000] | 0.756 [0.738, 0.773] |
| LogReg + peers (15) | FI → BL | 0.865 [0.857, 0.872] | 0.872 [0.862, 0.883] | 0.858 [0.848, 0.869] | 0.923 [0.912, 0.933] | 0.774 [0.759, 0.790] |
| LogReg + peers (15) | BL → FI | 0.883 [0.875, 0.889] | 0.825 [0.813, 0.835] | 0.871 [0.855, 0.886] | 0.998 [0.995, 1.000] | 0.861 [0.848, 0.874] |
| LogReg + peers (15) | BL → BL | 0.934 [0.930, 0.938] | 0.959 [0.954, 0.964] | 0.961 [0.956, 0.966] | 0.916 [0.902, 0.929] | 0.860 [0.846, 0.873] |
| LogReg + peers (15) | LEGACY → FI | 0.911 [0.904, 0.918] | 0.927 [0.922, 0.933] | 0.978 [0.975, 0.982] | 0.957 [0.948, 0.965] | 0.705 [0.686, 0.722] |
| LogReg + peers (15) | LEGACY → BL | 0.809 [0.801, 0.819] | 0.840 [0.827, 0.852] | 0.730 [0.713, 0.746] | 0.893 [0.873, 0.911] | 0.752 [0.736, 0.768] |
| LogReg (12) | FI → FI | 0.912 [0.907, 0.918] | 0.960 [0.957, 0.964] | 0.997 [0.996, 0.998] | 0.936 [0.929, 0.943] | 0.646 [0.626, 0.666] |
| LogReg (12) | FI → BL | 0.760 [0.748, 0.773] | 0.792 [0.779, 0.806] | 0.703 [0.686, 0.723] | 0.829 [0.818, 0.842] | 0.679 [0.662, 0.697] |
| LogReg (12) | BL → FI | 0.786 [0.774, 0.798] | 0.853 [0.843, 0.862] | 0.888 [0.874, 0.902] | 0.512 [0.488, 0.535] | 0.859 [0.846, 0.872] |
| LogReg (12) | BL → BL | 0.914 [0.908, 0.920] | 0.960 [0.955, 0.964] | 0.961 [0.955, 0.967] | 0.830 [0.814, 0.845] | 0.856 [0.843, 0.870] |
| LogReg (12) | LEGACY → FI | 0.848 [0.840, 0.857] | 0.942 [0.938, 0.947] | 0.978 [0.975, 0.981] | 0.692 [0.669, 0.715] | 0.657 [0.640, 0.674] |
| LogReg (12) | LEGACY → BL | 0.740 [0.729, 0.751] | 0.783 [0.771, 0.795] | 0.665 [0.648, 0.682] | 0.769 [0.747, 0.791] | 0.724 [0.705, 0.743] |

## SIM-BL copy_peer by synchronous submission (recall %, 5 % threshold)

| Model | Train → test | copy_peer, non-sync pairs | copy_peer, sync pairs |
|---|---|---:|---:|
| HGB + peers (15) | FI → BL | 51.0 [43.7, 57.9] | 77.6 [71.1, 83.8] |
| HGB + peers (15) | BL → BL | 56.8 [49.9, 63.1] | 93.5 [89.9, 96.5] |
| HGB + peers (15) | LEGACY → BL | 64.4 [57.3, 71.4] | 90.2 [84.2, 95.2] |
| HGB (12) | FI → BL | 36.3 [31.7, 41.1] | 38.1 [32.2, 43.3] |
| HGB (12) | BL → BL | 43.6 [40.1, 47.0] | 46.5 [43.4, 49.4] |
| HGB (12) | LEGACY → BL | 48.0 [40.6, 55.0] | 42.4 [37.4, 47.7] |
| LogReg + peers (15) | FI → BL | 57.7 [52.4, 62.4] | 88.5 [84.9, 91.9] |
| LogReg + peers (15) | BL → BL | 54.3 [47.0, 61.6] | 89.4 [85.0, 93.2] |
| LogReg + peers (15) | LEGACY → BL | 82.0 [75.0, 88.0] | 98.9 [97.6, 100.0] |
| LogReg (12) | FI → BL | 45.3 [41.0, 49.7] | 46.6 [42.9, 50.2] |
| LogReg (12) | BL → BL | 40.3 [35.6, 44.7] | 44.5 [40.6, 48.2] |
| LogReg (12) | LEGACY → BL | 96.3 [94.2, 98.3] | 95.6 [93.1, 97.9] |

## Pre-declared contrasts (recall@5 %, pp; paired by seed)

Paired percentile-bootstrap CI (10 000 resamples) and two-sided Wilcoxon signed-rank p, unadjusted. Slot rows compare the slot's FI type in FI→FI with its BL mechanism in FI→BL. 'Best unsupervised' = the main-stage detector (Rules, IF, IF + peers) with the highest mean SIM-BL recall on that subset (chosen on the same seeds, which favours the comparator).

| Contrast | Model | Subset | comparator | Δ mean [95 % CI] | n | p |
|---|---|---|---|---:|---:|---:|
| FI→FI − FI→BL | HGB + peers (15) (primary) | all |  | 34.8 [32.6, 36.9] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB + peers (15) (primary) | all |  | 24.5 [21.7, 27.4] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB + peers (15) (primary) | all | Rules | 33.5 [30.8, 36.2] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB + peers (15) (primary) | S1 rapid_response↔preknowledge |  | 27.0 [23.3, 31.0] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB + peers (15) (primary) | S1 rapid_response↔preknowledge |  | 24.4 [20.4, 28.1] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB + peers (15) (primary) | S1 rapid_response↔preknowledge | Rules | 42.9 [38.7, 46.9] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB + peers (15) (primary) | S2 excessive_changes↔lookup |  | 69.9 [65.6, 74.3] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB + peers (15) (primary) | S2 excessive_changes↔lookup |  | 46.4 [40.2, 52.7] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB + peers (15) (primary) | S2 excessive_changes↔lookup | Rules | 47.6 [43.6, 51.9] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB + peers (15) (primary) | S3 collusion↔copy_peer |  | 35.0 [29.4, 40.5] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB + peers (15) (primary) | S3 collusion↔copy_peer |  | 10.7 [6.0, 15.1] | 20 | 0.0015 |
| BL→BL − best unsupervised on BL | HGB + peers (15) (primary) | S3 collusion↔copy_peer | Rules | 9.7 [6.2, 13.2] | 20 | 0.0005 |
| FI→FI − FI→BL | HGB + peers (15) (primary) | S4 score_jump↔proxy |  | -2.9 [-7.7, 2.2] | 20 | 0.2330 |
| BL→BL − FI→BL | HGB + peers (15) (primary) | S4 score_jump↔proxy |  | 9.7 [3.8, 15.6] | 20 | 0.0064 |
| BL→BL − best unsupervised on BL | HGB + peers (15) (primary) | S4 score_jump↔proxy | Rules | 23.8 [18.8, 28.9] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB (12) | all |  | 38.0 [36.3, 39.8] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB (12) | all |  | 28.7 [26.4, 30.9] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB (12) | all | Rules | 25.3 [22.3, 28.4] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB (12) | S1 rapid_response↔preknowledge |  | 31.4 [28.5, 34.5] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB (12) | S1 rapid_response↔preknowledge |  | 28.7 [25.5, 31.6] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB (12) | S1 rapid_response↔preknowledge | Rules | 40.1 [35.4, 44.8] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB (12) | S2 excessive_changes↔lookup |  | 76.5 [73.1, 79.6] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB (12) | S2 excessive_changes↔lookup |  | 52.3 [46.7, 57.3] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | HGB (12) | S2 excessive_changes↔lookup | Rules | 45.9 [43.2, 48.9] | 20 | <0.0001 |
| FI→FI − FI→BL | HGB (12) | S3 collusion↔copy_peer |  | 33.1 [27.7, 38.5] | 20 | <0.0001 |
| BL→BL − FI→BL | HGB (12) | S3 collusion↔copy_peer |  | 8.1 [3.9, 12.1] | 20 | 0.0038 |
| BL→BL − best unsupervised on BL | HGB (12) | S3 collusion↔copy_peer | Rules | -20.2 [-26.5, -14.3] | 20 | 0.0002 |
| FI→FI − FI→BL | HGB (12) | S4 score_jump↔proxy |  | -0.9 [-4.0, 2.2] | 20 | 0.5067 |
| BL→BL − FI→BL | HGB (12) | S4 score_jump↔proxy |  | 19.0 [13.9, 23.9] | 20 | 0.0002 |
| BL→BL − best unsupervised on BL | HGB (12) | S4 score_jump↔proxy | Rules | 24.0 [19.2, 28.6] | 20 | 0.0001 |
| FI→FI − FI→BL | LogReg + peers (15) | all |  | 35.4 [33.3, 37.6] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg + peers (15) | all |  | 25.5 [23.4, 27.7] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg + peers (15) | all | Rules | 33.3 [30.4, 36.3] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg + peers (15) | S1 rapid_response↔preknowledge |  | 35.6 [33.0, 38.2] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg + peers (15) | S1 rapid_response↔preknowledge |  | 29.8 [27.0, 32.6] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg + peers (15) | S1 rapid_response↔preknowledge | Rules | 43.0 [38.8, 47.1] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg + peers (15) | S2 excessive_changes↔lookup |  | 69.4 [64.6, 74.0] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg + peers (15) | S2 excessive_changes↔lookup |  | 47.9 [42.8, 53.5] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg + peers (15) | S2 excessive_changes↔lookup | Rules | 49.5 [44.8, 54.1] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg + peers (15) | S3 collusion↔copy_peer |  | 26.9 [23.7, 30.1] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg + peers (15) | S3 collusion↔copy_peer |  | -1.0 [-3.9, 2.0] | 20 | 0.5870 |
| BL→BL − best unsupervised on BL | LogReg + peers (15) | S3 collusion↔copy_peer | Rules | 6.3 [1.9, 10.8] | 20 | 0.0166 |
| FI→FI − FI→BL | LogReg + peers (15) | S4 score_jump↔proxy |  | -5.8 [-9.4, -2.2] | 20 | 0.0139 |
| BL→BL − FI→BL | LogReg + peers (15) | S4 score_jump↔proxy |  | 18.2 [13.4, 23.4] | 20 | 0.0002 |
| BL→BL − best unsupervised on BL | LogReg + peers (15) | S4 score_jump↔proxy | Rules | 24.7 [20.4, 29.5] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg (12) | all |  | 37.8 [35.7, 39.8] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg (12) | all |  | 34.5 [32.1, 36.8] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg (12) | all | Rules | 26.4 [23.3, 29.4] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg (12) | S1 rapid_response↔preknowledge |  | 45.5 [42.6, 48.3] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg (12) | S1 rapid_response↔preknowledge |  | 42.7 [39.3, 46.0] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg (12) | S1 rapid_response↔preknowledge | Rules | 43.1 [39.0, 47.0] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg (12) | S2 excessive_changes↔lookup |  | 85.9 [82.8, 88.8] | 20 | <0.0001 |
| BL→BL − FI→BL | LogReg (12) | S2 excessive_changes↔lookup |  | 64.1 [59.2, 69.0] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg (12) | S2 excessive_changes↔lookup | Rules | 49.3 [44.6, 53.8] | 20 | <0.0001 |
| FI→FI − FI→BL | LogReg (12) | S3 collusion↔copy_peer |  | 3.5 [-2.5, 10.4] | 20 | 0.5274 |
| BL→BL − FI→BL | LogReg (12) | S3 collusion↔copy_peer |  | -3.4 [-7.3, 0.4] | 20 | 0.1549 |
| BL→BL − best unsupervised on BL | LogReg (12) | S3 collusion↔copy_peer | Rules | -22.9 [-29.1, -17.0] | 20 | 0.0001 |
| FI→FI − FI→BL | LogReg (12) | S4 score_jump↔proxy |  | -5.1 [-8.5, -1.7] | 20 | 0.0097 |
| BL→BL − FI→BL | LogReg (12) | S4 score_jump↔proxy |  | 23.2 [17.9, 29.2] | 20 | <0.0001 |
| BL→BL − best unsupervised on BL | LogReg (12) | S4 score_jump↔proxy | Rules | 23.6 [19.1, 28.5] | 20 | <0.0001 |

Fit diagnostics: HGB + peers (15) mean iterations 100.0; HGB (12) mean iterations 100.0; LogReg + peers (15) mean iterations 30.7, all converged: True; LogReg (12) mean iterations 28.3, all converged: True.
