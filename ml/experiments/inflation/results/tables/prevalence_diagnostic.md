# Diagnostic: Rules recall on copy_peer, low prevalence (82.5 %) vs main (65.5 %)

**Added after independent review (amendment of 15 Nov 2026).** SIM-BL, 20 seeds each. *Mean per-seed recall* is what `prevalence.md` / `main.md` report (seeds without test positives drop out); *pooled recall* = detected / all test positives over seeds. *Composition-expected* = the main stage's pooled sync and non-sync recall weighted by the low-prevalence sync share.

| Detector | Stage | Subset | test positives (all seeds) | seeds with positives | mean per-seed recall % | pooled recall % |
|---|---|---|---:|---:|---:|---:|
| Rules | main (rate 0.12) | all copy_peer | 652 | 20/20 | 65.5 | 66.3 |
| Rules | main (rate 0.12) | sync pairs | 320 | 20/20 | 92.0 | 93.1 |
| Rules | main (rate 0.12) | non-sync pairs | 332 | 20/20 | 39.8 | 40.4 |
| Rules | main | sync share of positives | 49.1 % |  | validation FPR 4.58 % | test FPR 5.19 % |
| Rules | prevalence (rate 0.0167) | all copy_peer | 102 | 19/20 | 82.5 | 78.4 |
| Rules | prevalence (rate 0.0167) | sync pairs | 60 | 18/20 | 95.8 | 95.0 |
| Rules | prevalence (rate 0.0167) | non-sync pairs | 42 | 10/20 | 52.5 | 54.8 |
| Rules | prevalence | sync share of positives | 58.8 % |  | validation FPR 3.39 % | test FPR 3.68 % |
| Rules | prevalence | composition-expected pooled recall |  |  |  | 71.4 |
| IF + peers | main (rate 0.12) | all copy_peer | 652 | 20/20 | 61.0 | 61.5 |
| IF + peers | main (rate 0.12) | sync pairs | 320 | 20/20 | 86.0 | 86.6 |
| IF + peers | main (rate 0.12) | non-sync pairs | 332 | 20/20 | 36.1 | 37.3 |
| IF + peers | main | sync share of positives | 49.1 % |  | validation FPR 4.64 % | test FPR 4.98 % |
| IF + peers | prevalence (rate 0.0167) | all copy_peer | 102 | 19/20 | 65.9 | 63.7 |
| IF + peers | prevalence (rate 0.0167) | sync pairs | 60 | 18/20 | 78.2 | 76.7 |
| IF + peers | prevalence (rate 0.0167) | non-sync pairs | 42 | 10/20 | 45.8 | 45.2 |
| IF + peers | prevalence | sync share of positives | 58.8 % |  | validation FPR 3.18 % | test FPR 3.28 % |
| IF + peers | prevalence | composition-expected pooled recall |  |  |  | 66.3 |
