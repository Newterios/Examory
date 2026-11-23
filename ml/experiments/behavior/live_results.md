# Live input and reference experiments

World SHA-256 `6352744b615ca955fde9fb14ab3618010bb853404ea6e7675c8d5ab23ae2f8be`. Fitted model SHA-256 `717c54991b6c035d1ce3bd4079a23ecf10722058097d57b85f7c4825453a18ef`. Training source and held-out cohorts are shared with E-16/E-17.

## E-14. Complete-attempt parity

Smallest held-out cohort by ID, group_002. All 60 complete simulator attempts were accepted by both readers. All 15 model inputs matched, including missing-history masks.

| Detector | Compared attempts | Maximum absolute raw-score difference | Different alert decisions |
|---|---:|---:|---:|
| iforest | 60 | 0 | 0 |
| quantile_rules | 60 | 0 | 0 |

## E-15. Normal alerts after warm-up

The test cohorts use a new quiz and variant ID. The estimator and validation cutoffs remain fixed. At 2026-10-03T09:21:28.728000+00:00, the snapshot used 500 strictly earlier eligible attempts. The replay simulated 19 prior positive teacher confirmations and excluded them. The retained history contains 12 unconfirmed anomalous attempts. Synthetic labels are used to simulate these decisions and evaluate outcomes. The reference worker consumes teacher decisions and has no access to simulation labels.

| Detector | Future normal attempts | Fixed synthetic FP / FPR | Warm quiz FP / FPR | Change (pp) |
|---|---:|---:|---:|---:|
| iforest | 76 | 4 / 5.26% | 1 / 1.32% | -3.95 |
| quantile_rules | 76 | 14 / 18.42% | 4 / 5.26% | -13.16 |

The cold-start policy suppresses publication until a historical snapshot is available. Its zero alerts during warm-up represent abstention. The warm comparison above counts decisions on the same future normal attempts. One synthetic quiz provides limited evidence for transfer to real courses.
