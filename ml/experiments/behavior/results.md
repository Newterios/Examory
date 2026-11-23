# False positives in student strata

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
