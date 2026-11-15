# Post hoc: recall by behavioural sub-population (main stage, SIM-BL)

**Added after independent review (amendment of 15 Nov 2026).** From `main.json` (20 seeds, frozen detectors, 5 % threshold). `truth.csv:mech_flag` records the pre-knowledge strategy per cheater: `naive` (recalled items answered at a sped-up time) or `disguising` (recalled items at the honest time draw), which is the strategic variant. Each row: that sub-group's test cheaters vs all normal test attempts; recall %, mean [95 % CI over seeds]; positives = mean test cheaters per seed.

| Sub-population | positives/seed | IF + peers | IF | Rules |
|---|---:|---:|---:|---:|
| preknowledge, naive | 24.3 | 38.6 [33.8, 43.3] | 51.7 [48.1, 55.4] | 41.0 [36.5, 45.8] |
| preknowledge, disguising (strategic) | 25.6 | 10.8 [8.3, 13.6] | 9.3 [5.9, 12.9] | 28.4 [22.2, 35.0] |
| preknowledge, all | 49.9 | 24.2 [22.1, 26.4] | 29.8 [27.4, 32.3] | 34.3 [29.9, 38.6] |
| copy_peer, non-sync pairs | 16.6 | 36.1 [29.5, 42.9] | 22.1 [19.4, 25.0] | 39.8 [32.1, 47.6] |
| copy_peer, sync pairs | 16.0 | 86.0 [82.5, 89.5] | 21.5 [16.3, 26.5] | 92.0 [87.6, 96.1] |
| copy_peer, all | 32.6 | 61.0 [56.5, 65.4] | 21.5 [18.7, 24.3] | 65.5 [60.1, 70.9] |

## Cheating with no observable change (all cohorts, 20 seeds pooled)

From the `protocol` stage (same data as the main stage). *Event-identical*: every quiz.answered / quiz.submitted event of the cheater attempt (event id, timestamp, payload incl. answer hashes, score, duration, change count) equals the same attempt in the honest world, i.e. the attempt is observationally honest. *Score unchanged*: submitted score equals the counterfactual's. *0 items affected*: `truth.csv:items_affected = 0` (for score_jump: no item's correctness flipped). All of them remain labelled positives (is_anomaly = 1) and count in every recall denominator; nothing is excluded.

| World | Type | cheaters | event-identical | score unchanged | 0 items affected |
|---|---|---:|---:|---:|---:|
| SIM-FI | collusion | 3600 | 0 (0.00 %) | 148 (4.1 %) | 0 (0.00 %) |
| SIM-FI | excessive_changes | 3600 | 0 (0.00 %) | 3600 (100.0 %) | 0 (0.00 %) |
| SIM-FI | rapid_response | 5040 | 0 (0.00 %) | 5040 (100.0 %) | 0 (0.00 %) |
| SIM-FI | score_jump | 2160 | 49 (2.27 %) | 166 (7.7 %) | 74 (3.43 %) |
| SIM-BL | copy_peer | 3600 | 0 (0.00 %) | 1993 (55.4 %) | 0 (0.00 %) |
| SIM-BL | lookup | 3600 | 31 (0.86 %) | 110 (3.1 %) | 31 (0.86 %) |
| SIM-BL | preknowledge | 5040 | 28 (0.56 %) | 79 (1.6 %) | 0 (0.00 %) |
| SIM-BL | proxy | 2160 | 0 (0.00 %) | 249 (11.5 %) | 0 (0.00 %) |
