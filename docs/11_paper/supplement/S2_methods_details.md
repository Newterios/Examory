# Supplement S2. Details of the methods

## S2.1 Features of the frozen detectors

The detector code is under `ml/anomaly/` at tag `pre-defense-1` (23 Oct 2026); its file hashes are recorded with every result.

Twelve attempt-level features are computed from `quiz.answered` and `quiz.submitted` events:

- mean, median, 10th percentile and variance of first-answer times;
- the fraction of answers faster than the item's 10th-percentile time among ordinary training attempts (items with fewer than 30 training observations use the global 10th percentile);
- the time since display of the final answer to the last-answered question;
- attempt duration;
- number and fraction of answer changes;
- percentage score;
- the standardised difference between the score and the student's earlier attempts on the same quiz;
- an indicator of missing history.

Three peer features are computed against other students of the same cohort, quiz and variant who submitted within ±5 s: the number of such submissions, the maximum fraction of identical final answers, and the same agreement weighted by the rarity of the shared answers. Peer features are 0 when no such peer exists.

Time, count and rarity features are log-transformed. Every feature has a missing-value indicator, so the forest without peer features sees 24 columns and the other two detectors see 30. The rules take the maximum two-sided mid-rank over all 30 columns. The forests use 200 trees, a subsample of 256, `contamination="auto"` and a fixed random state.

The threshold rule of the frozen pipeline maximises validation recall subject to a validation FPR of at most 5 % and then minimises false positives. It therefore sits at the score of a validation positive; the mean validation FPR is 4.3–4.8 %.

## S2.2 Injection recipes: the original generator and its port

The original generator draws ordinary attempts from three response-time profiles (lognormal, medians 18–38 s), a Beta-distributed probability of a correct answer and a per-profile probability of changing an answer. The four recipes are given in Section 3.3 of the article. In SIM-FI they are applied to the honest attempts of the simulated students with the same constants. The port differs in these details:

- the forced collusion option is the item's least attractive distractor;
- on the other items the pair shares the leader's honest final answer, whereas the original generator shares the key with probability 0.85 and a random distractor otherwise;
- collusion durations are the leader's honest item times, and collusion attempts contain no answer revisions, whereas the original generator keeps the profile's ordinary changes;
- the score jump shifts the item-response probability of a correct first answer (capped at 0.98) and keeps the number of changes and all timings at their honest values.

The random-number streams also differ, so LEGACY and SIM-FI do not share cheaters or attempts.

## S2.3 Mechanism and injection parameters

Defaults are in `sim/params/mechanisms.json`. Medians of lognormal durations were sampled log-uniformly in the sensitivity analysis; "–" means the parameter was held at its default. The values are assumptions; no source in the literature fixes them.

| Construction | Parameter | Default | Sensitivity range |
|---|---|---:|---:|
| Pre-knowledge | per-item coverage (leak probability) | 0.65 | – (varied as intensity) |
| Pre-knowledge | recall of a leaked item | 0.92 | – |
| Pre-knowledge | share of naive cheaters | 0.5 | 0–1 |
| Pre-knowledge | speed-up factor, upper bound | 0.60 | 0.3–0.9 |
| Pre-knowledge | speed-up factor, lower bound | 0.25 | 0.4 × upper bound |
| Look-up | propensity on unknown items | 0.70 | 0.3–1.0 |
| Look-up | success probability | 0.90 | – |
| Look-up | duration, median (log-SD 0.60) | 35 s | 10–90 s |
| Look-up | share of cheaters who answer first and correct afterwards | 0.5 | 0–1 |
| Peer copying | per-item coverage | 0.80 | 0.3–1.0 |
| Peer copying | relay delay, median (log-SD 0.50) | 6.0 s | 2–30 s |
| Peer copying | answer entry time, median (log-SD 0.40) | 5.0 s | – |
| Peer copying | start lag, median (log-SD 0.80) | 15 s | 5–300 s |
| Peer copying | probability of following a source change | 0.50 | – |
| Peer copying | probability that the abler student is the source | 0.80 | – |
| Peer copying | source communication overhead, median (log-SD 0.50) | 3.0 s | – |
| Peer copying | share of synchronously submitting pairs | 0.5 | 0–1 |
| Peer copying | synchronous submission offset, median (log-SD 0.70) | 2.0 s | – |
| Proxy | mean ability of the proxy | 1.2 | 0.5–2.0 |
| Proxy | SD of the proxy's ability | 0.5 | – |
| Proxy | minimum ability gain | 0.5 | – |
| Injection | rapid response: per-item probability | 0.65 | 0.30–0.90 |
| Injection | excessive changes: per-item probability, U(0.45, 0.75) | × 1 | × 0.5–1.25 |
| Injection | collusion: submission window | 1 s | 0.5–20 s (log-uniform) |
| Injection | score jump: shift of the correct-answer probability, U(0.18, 0.32) | × 1 | × 0.5–1.5 |

The sensitivity design has 60 configurations of these 14 varied parameters (Latin hypercube, columns decorrelated by within-column swaps to a maximum absolute Spearman correlation of 0.0999) and 10 replicates per configuration (seeds 1001–1010). An earlier version (40 configurations of the ten mechanism parameters, three replicates each) is kept as `results/stages/sensitivity_v1.json`.

**Dose.** The injection dose λ multiplies the per-item probabilities of rapid response and excessive changes, the copy probability of collusion and the score-jump shift. The behavioural intensity x replaces the per-item coverage or propensity of pre-knowledge, look-up and peer copying, and scales the ability gain of the proxy; x = 1 is therefore a stronger dose than the defaults.

## S2.4 Variants of honest behaviour

- **Exam-like revision rate.** The honest revision propensity is multiplied by 0.1843, which gives 0.84 answer changes per 20-item attempt (95th percentile 4), the value reported for regular examinees by Ranger et al. (2020).
- **Item sets drawn per attempt.** Each attempt receives its own 20 items from the calibrated pool of 1,089. Two students who submit together then share two to three items on average, and about 60 % of such pairs share none, so answer agreement is rarely defined.
- **Closing time.** The quiz closes at a fixed time after the sitting opens (simulator 0.3.0); attempts still open are submitted automatically within 2 s, and unanswered items score zero. The two closing times are the 90th and 75th percentile of the natural honest submission time in one replicate (seed 1001); averaged over the 20 replicates, 5 % and 16 % of ordinary attempts are submitted automatically. For automatically submitted attempts the event stream reports the score as a percentage of the answered items, a concession to the frozen event reader. The model does not include students who speed up before the deadline.

## S2.5 Classical indices

- **Answer similarity.** For two attempts of different students in the same cohort and quiz sharing at least 10 items, z = (matches − Σ p_k) / √Σ p_k(1 − p_k), where a match is an identical final option and p_k is the probability that two random attempts choose the same option on item k, estimated from option frequencies in the training cohorts without labels. The score of an attempt is its maximum z over partners. The variant restricted to incorrect answers counts only items that the attempt answered incorrectly.
- **Response-time person-fit.** A two-way model ln t_ik = β_k − τ_i + ε_ik is fitted to first-answer times of the training cohorts; σ_k is the residual SD of item k. For an attempt, τ_i is the mean of β_k − ln t_ik, and the score is the sum of squared standardised residuals (Marianti et al., 2014).

Both are evaluated under the protocol of the main detectors.
