# Normal student strata and the history-based gain rule

SIM-BL 0.3.0, seed 42, 1200 students, 3 attempts each, 20 fixed questions. The cohorts are disjoint across training, validation and test.

World SHA-256 `6352744b615ca955fde9fb14ab3618010bb853404ea6e7675c8d5ab23ae2f8be`. Truth SHA-256 `198cacbadea7bbdfb06a3653b423af5330369fee5c48884bab6c6995ada567fa`.

## E-16. False positives in normal attempts

Strata use simulator traits solely for reporting. Quartiles were fixed on unique training students with a normal observation. The fast-and-strong group crosses the upper quartiles of speed and initial ability. Slow uses the bottom speed quartile. Frequent reviser uses the top revision-propensity quartile. Strata can overlap. Counts include every labelled-normal test attempt.

| Stratum | Normal attempts | Students | Forest FP | Forest FPR | Rules FP | Rules FPR |
|---|---:|---:|---:|---:|---:|---:|
| All normal | 645 | 240 | 39 | 6.05% | 28 | 4.34% |
| Fast and strong | 72 | 26 | 9 | 12.50% | 3 | 4.17% |
| Slow | 165 | 62 | 10 | 6.06% | 10 | 6.06% |
| Frequent reviser | 146 | 53 | 6 | 4.11% | 6 | 4.11% |

iforest: fast and strong had the highest subgroup FPR, 12.50%, +6.45 percentage points versus all normal attempts.

quantile_rules: slow had the highest subgroup FPR, 6.06%, +1.72 percentage points versus all normal attempts.

## E-17. Recognising preknowledge from score gains

The gain rule uses only earlier scores for the same student, quiz and variant. On normal training attempts it estimates the expected percentage-point gain by history length and prior-mean score band (width 20). Sparse bands fall back to history length, then to all normal transitions. Residuals are divided by 1.4826 × the median absolute deviation with a five-point floor. A first attempt has no comparable score and receives no flag. Simulator traits and latent practice_gain never enter the detector.

The validation cutoff maximises preknowledge recall under 5% FPR among final normal attempts with history. The two original detectors retain their frozen validation thresholds. The comparison uses final test attempts and includes all labelled preknowledge positives.

| Detector | TP / preknowledge | Recall | FP / normal | FPR | Cutoff |
|---|---:|---:|---:|---:|---:|
| iforest | 3 / 24 | 12.50% | 3 / 165 | 1.82% | 0.568868 |
| quantile_rules | 3 / 24 | 12.50% | 7 / 165 | 4.24% | 0.994715 |
| gain_rule | 11 / 24 | 45.83% | 6 / 165 | 3.64% | 1.5176 |

The history rule's recall differed from the better baseline by +33.33 percentage points on this split. This is one synthetic world. A shift in reported scores is compatible with learning, item familiarity and random variation.
