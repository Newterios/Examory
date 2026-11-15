# Report: detector-independent evaluation and article draft (LMS-35)

Date: 15 November 2026 (first version 14 November). Owner: Kaiyrbekuly Aitbek (task taken over from Akhanov Y. by agreement inside the team). Branch: `LMS-35-behaviour-simulation`.

This report answers point 4 of the supervisor review of 23 October 2026 (`docs/10_supervisor_feedback.md`): the only measured result was on the team's own generator, which injects anomalies along the detector's features.

## What was done

| Deliverable | Location | State |
|---|---|---|
| Behaviour-level quiz simulator (Go, standard library only): latent learner model, four cheating mechanisms, paired worlds honest / feature-aligned injection / behaviour-level; optional fixed closing time (version 0.3.0) | `sim/` | tested (`go test -race`, Go 1.23 and 1.25), methods in `sim/README.md` |
| EdNet-KT2 pipeline: extraction of Part 5, conversion of real attempts to the event contract, calibration of the learner model | `ml/ednet/` | run on the full archive; `calibration.json`, `calibration_report.md` |
| Experiment harness and statistics around the frozen detectors of `ml/anomaly/` | `ml/experiments/inflation/` | all stages run; tables and figures under `results/`; 44 unit tests |
| Article draft, three supplements and bibliography (83 entries, DOIs checked against Crossref) | `docs/11_paper/` | revised after an independent methods review; numbers cross-checked against the generated tables |
| Conference version for IEEE SIST 2027 (6 pages, IEEE two-column format, PDF), with the venue's requirements checked | `docs/11_paper/sist2027/` | ready except for the e-mail addresses and the acknowledgment; submission deadline 12 Feb 2027 |
| Outcome of hypothesis H1 | `docs/08_verification/verification_plan.md` §0 | recorded, with a caveat on training-data purging |

## Revision of 15 November

An independent review of the first draft (recommendation: major revision) raised these points, all of which were addressed by new analyses or by rewording:

- the difference between injection and behaviour-level cheating depends on the dose of both, and the injection dose had not been varied;
- the sensitivity analysis used three atypical replicates per configuration and a correlated design;
- thresholds and training data used labels (two oracles);
- the honest model differs from examinations (revision rate, no closing time, repeated identical form);
- the field experiment of Ranger et al. (2020) was described inaccurately;
- three results that did not fit the narrative had been left out;
- no supervised or psychometric detector had been evaluated.

A scoping sample of 48 published studies was coded to check the premise of the article (`docs/11_paper/supplement/S3_scoping_survey.md`).

## Main results (calibrated simulator, 20 replicates, FPR fixed at 5 % on validation cohorts)

- At the constants of the original generator and default mechanism parameters, overall recall of the same detector on the same simulated students is 61.9 % → 36.2 % (rules), 46.3 % → 22.9 % (Isolation Forest), 53.6 % → 27.8 % (Isolation Forest + peer features) when feature-aligned injection is replaced by behaviour-level cheating.
- The difference is not a constant: 54–70 pp for answer look-up, 34–39 pp for peer copying (detectors with peer features), 11–26 pp for pre-knowledge (forests only), none for proxy test-taking. It changes sign when the injected dose is lowered or the behavioural dose raised.
- Injected collusion is detected at 99–100 % at every dose, because the recipe always satisfies the ±5 s synchrony of the peer features. Behaviour-level pairs are detected at 86–92 % when they submit together and at 36–40 % when they do not.
- A gradient-boosting classifier trained on injected labels loses 35 pp on behaviour-level cheating. Trained on behaviour-level labels it reaches 69.6 %, so the features carry the signal and the unsupervised detectors miss it.
- The pattern holds with a label-free threshold, an exam-like revision rate, item sets drawn per attempt and a fixed closing time.
- If cheating attempts (12 % of training attempts) are not removed before fitting, the rules lose most of their recall (61.9 % → 26.1 % under injection). At a prevalence of 5 % of final attempts this effect almost disappears.
- H1 is not supported under the main protocol (−4.5 pp on peer copying, −1.2 pp on proxy); the rules are adopted, with the caveat above. The test on real anonymized logs (LMS-36) remains open.
- On real EdNet attempts (no cheating labels), detectors fitted on synthetic ordinary behaviour flag 11–39 % of attempts at a nominal 5 %.

## What is not done

- No real labelled cheating cases and no AITU quiz logs were used; every detection result is on simulated cheating. The experimental pre-knowledge data of Toton and Maynes (2019) would be a labelled anchor, but no openly licensed copy was found; the authors could be asked.
- Mechanism parameters are assumptions, varied in a sensitivity analysis. One mechanism suggested by the field experiment of Ranger et al. (answers received late and corrected in bulk) was not simulated.
- The behaviour simulator has not yet been replayed through the platform's quiz service; this check needs the M4 services of LMS-38 in `main`.
- The venue recommended by the supervisor is the IEEE SIST 2027 conference; the conference version is prepared, but the e-mail addresses and the acknowledgment are still open and nothing was submitted; the authors are the student and the supervisor. The long manuscript (Markdown, about 8,700 words with tables) is kept for a later journal version; a shortlist of journals with their requirements was prepared for the supervisor.

## Reproduction

Commands are in `ml/experiments/inflation/README.md` (on a 14-core laptop the stages of the first version took about 16 minutes and the stages added on 15 November about 33 minutes, of which the sensitivity analysis 21; 3.5 GB of temporary data), `ml/ednet/README.md` and `sim/README.md`.
