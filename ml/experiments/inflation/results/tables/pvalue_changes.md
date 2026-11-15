# p-values that crossed 0.05 when Wilcoxon was replaced by the exact sign-flip test

Every paired test in `summary.json`, raw and Holm-adjusted where a Holm family exists. p (Wilcoxon) is the previous method (SciPy `wilcoxon`, two-sided, exact or normal approximation as SciPy chooses), recomputed on the same per-seed values; p (sign-flip) is the exact paired sign-flip permutation test now reported everywhere. Mann–Whitney (unpaired) p-values did not change and are not listed.

## Tables committed before the review (inflation, robust_inflation, h1, supervised; summary.json copy_peer_sync)

| Cell | p (Wilcoxon) | p (sign-flip) | direction |
|---|---:|---:|---|
| robust_detectors/inflation/SIM-FI-minus-SIM-BL|pauc_std[pauc_std|mcd_no_peers|S2] | 0.0973 | 0.0074 | now < 0.05 |
| robust_detectors/inflation/SIM-FI-minus-SIM-BL|pauc_std[pauc_std|mcd|S2] (Holm) | 0.1991 | 0.0274 | now < 0.05 |
| robust_detectors/inflation/SIM-FI-minus-SIM-BL|recall@5[recall@5|mcd_no_peers|S2] | 0.0836 | 0.0080 | now < 0.05 |
| robust_detectors/inflation/SIM-FI-minus-SIM-BL|recall@5[recall@5|mcd|S2] (Holm) | 0.2062 | 0.0263 | now < 0.05 |
