# Report: detector-independent evaluation and article draft (LMS-35)

Date: 14 November 2026. Owner: Kaiyrbekuly Aitbek (task taken over from Akhanov Y. by agreement inside the team). Branch: `LMS-35-behaviour-simulation`.

This report answers point 4 of the supervisor review of 23 October 2026 (`docs/10_supervisor_feedback.md`): the only measured result was on the team's own generator, which injects anomalies along the detector's features.

## What was done

| Deliverable | Location | State |
|---|---|---|
| Behaviour-level quiz simulator (Go, standard library only): latent learner model, four cheating mechanisms, paired worlds honest / feature-aligned injection / behaviour-level | `sim/` | tested (`go test -race`), methods described in `sim/README.md` |
| EdNet-KT2 pipeline: extraction of Part 5, conversion of real attempts to the event contract, calibration of the learner model | `ml/ednet/` | run on the full archive; `calibration.json`, `calibration_report.md` |
| Experiment harness and statistics around the frozen detectors of `ml/anomaly/` | `ml/experiments/inflation/` | all stages run; tables and figures under `results/` |
| Article draft and bibliography checked against Crossref | `docs/11_paper/` | complete draft; numbers cross-checked against the generated tables |
| Outcome of hypothesis H1 | `docs/08_verification/verification_plan.md` §0 | recorded |

## Main results (calibrated simulator, 20 replicates, FPR fixed at 5 % on validation cohorts)

- Overall recall of the same detector on the same simulated students: 61.9 % → 36.2 % (rules), 46.3 % → 22.9 % (Isolation Forest), 53.6 % → 27.8 % (Isolation Forest + peer features) when feature-aligned injection is replaced by behaviour-level cheating.
- The loss depends on the type of cheating: 54–70 pp for answer look-up (all detectors), 34–39 pp for peer copying (detectors with peer features), 11–26 pp for pre-knowledge (forests only); proxy test-taking is detected near chance in every condition. Some detector–type cells show no or reversed difference; the article reports all of them.
- Peer copying is detected mainly when the pair submits within the ±5 s window of the peer features (86–92 % against 36–40 %).
- H1 is not supported on the simulated evaluation set: the forest with peer features does not exceed the rules (−4.5 pp on peer copying, −1.2 pp on proxy). By the criterion of the verification plan the rules are adopted. The test on real anonymized logs (LMS-36) remains open.
- On real EdNet attempts (no cheating labels), detectors fitted on synthetic ordinary behaviour flag 11–39 % of attempts at a nominal 5 %; after refitting on real attempts the flags concentrate on the fastest attempts and on both ends of the score distribution.

## What is not done

- No real labelled cheating cases and no AITU quiz logs were used; every detection result is on simulated cheating.
- Mechanism parameters are assumptions, varied in a sensitivity analysis; the simulator still differs from real behaviour (classifier two-sample AUC 0.83).
- The behaviour simulator has not yet been replayed through the platform's quiz service; the planned pipeline-equivalence check depends on the M4 services.
- Target journal, author list and declarations are not fixed; the manuscript is in Markdown and is longer than most journals allow.

## Reproduction

Commands are in `ml/experiments/inflation/README.md` (about 16 minutes on a 14-core laptop, 3.6 GB of temporary data), `ml/ednet/README.md` and `sim/README.md`.
