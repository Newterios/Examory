# Appendix: simulator defaults (uncalibrated), 14 Nov 2026

Outputs of the first full run, made with the **uncalibrated** simulator parameters
`sim/params/default.json` (simulator binary SHA-256 `65de41612e47…`, built from the sources
committed at 31485bd plus the `sync_submit` amendment), before `ml/ednet/calibration.json`
existed. They are kept only as an appendix that shows the effect of calibration; the reported
results are in `../results/` (EdNet-calibrated parameters).

Contents: `summary.json` (compact; key `label` marks it), `tables/*.md` (each starts with the
same label), `figures/F1_recall_by_slot.{png,pdf}` (old detector/condition labels).
The per-seed stage files of this run were not kept (size). The run is reproducible with
`python -m ml.experiments.inflation.run <stage> --params sim/params/default.json`.
