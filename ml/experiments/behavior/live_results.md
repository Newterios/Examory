# Live input and reference experiments

World SHA-256 `6352744b615ca955fde9fb14ab3618010bb853404ea6e7675c8d5ab23ae2f8be`. Fitted model SHA-256 `717c54991b6c035d1ce3bd4079a23ecf10722058097d57b85f7c4825453a18ef`. Training source and held-out cohorts are shared with E-16/E-17.

## E-14. Complete-attempt parity

Smallest held-out cohort by ID, group_002. All 60 complete simulator attempts were accepted by both readers. All 15 model inputs matched, including missing-history masks.

| Detector | Compared attempts | Maximum absolute raw-score difference | Different alert decisions |
|---|---:|---:|---:|
| iforest | 60 | 0 | 0 |
| quantile_rules | 60 | 0 | 0 |
