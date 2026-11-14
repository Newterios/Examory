# examory-sim: behaviour-level quiz simulator

`examory-sim` generates synthetic Examory quiz event logs in which cheating
arises from simulated *behaviour* (actions taken while answering) rather than
from direct edits of the features a detector computes. It produces three
paired worlds from one set of honest attempts, so that the effect of the
anomaly-generation method on detector performance can be measured with
everything else held fixed. It is the detector-independent evaluation set of
`docs/08_verification/verification_plan.md` §3b.

The output CSVs use the event contract of `docs/03_architecture/events.md`
(`quiz.answered` v1, `quiz.submitted` v1) and the CSV dialect of the legacy
generator `ml/datasets/generate_quiz_behavior.py`; they are accepted
unchanged by `ml/anomaly/features.py:read_attempts`.

Module: `github.com/Newterios/Examory/sim`, Go 1.23, standard library only.

## 1. Paired-world design

For one configuration the simulator draws one population of students, one
item set (or one item set per attempt) and one honest attempt per
(student, attempt). Three worlds are then derived:

| world | non-cheater attempts | cheater attempts (final attempt of a selected student) |
|---|---|---|
| `honest` | honest | honest, labelled `is_anomaly=0`, `anomaly_type=none` |
| `fi` | honest | honest attempt transformed by a legacy *feature-aligned injection* recipe (§4) |
| `bl` | honest | honest attempt re-played under a *behaviour-level* cheating mechanism (§3) |

Properties guaranteed by construction and checked by tests:

1. The set of cheater students, their slot type and the collusion pairs are
   identical in `fi` and `bl` (and recorded for the same students in the
   `honest` rows of `truth.csv`).
2. Every attempt that is not a cheater attempt is the *same object* in all
   three worlds: its rows, including `event_id`s, are byte-identical in
   `honest.csv`, `fi.csv` and `bl.csv`.
3. Common random numbers: every variate is taken from an independent stream
   (§6), and mechanisms reuse the honest attempt's draws. A mechanism
   therefore changes only the parts of a response it acts on; e.g. a
   disguising pre-knowledge cheater keeps the honest response-time draw of
   every item, and an unaffected item of a cheater attempt is identical to
   the honest one.

Slot to mechanism mapping (value of the `anomaly_type` column):

| slot | `fi` | `bl` |
|---|---|---|
| rapid_response | `rapid_response` | `preknowledge` |
| excessive_changes | `excessive_changes` | `lookup` |
| collusion | `collusion` | `copy_peer` |
| score_jump | `score_jump` | `proxy` |

The pairing is by slot, not by claimed equivalence: a `bl` mechanism is the
behaviour that a `fi` recipe is a stylised feature signature of.

### Cheater selection

Selection reproduces the legacy `make_labels`:

* `requested = floor(S·K·rate + 0.5)` anomalous attempts for `S` students,
  `K` attempts, rate `rate ≤ 1/K`.
* Quotas by largest remainder on the weights (stable ordering on ties); if
  the collusion count is odd it is decreased by one and the slot with the
  largest positive weight receives the attempt.
* Cohorts are blocks of 20 consecutive students (`group_%03d`). Collusion
  pairs are formed by shuffling each cohort and pairing consecutive members;
  the list of pairs is shuffled and the first `collusion/2` are used. The
  remaining students are shuffled and assigned the other slots in legacy
  order.
* Only the final attempt (`k = K−1`) of a cheater is anomalous.

For the defaults (2000 students, 3 attempts, rate 0.12, weights
.35/.25/.25/.15) the quotas are rapid_response 252, excessive_changes 180,
collusion 180, score_jump 108, identical to the legacy generator (tested
against values computed with the Python code for eight configurations). The
random permutations differ from the legacy generator because Go and Python
use different generators.

## 2. Honest latent model

Notation: student `i`, zero-based attempt `k`, item `j`. All times are
integer milliseconds.

### 2.1 Persons

```
(θ_i, τ_i):  θ_i = σ_θ z₁,  τ_i = σ_τ (ρ z₁ + √(1−ρ²) z₂),  z₁, z₂ ~ N(0,1)
g_i  = 0 with probability p_zero, else Beta(a_g, b_g)      (rapid-guess propensity)
r_i  ~ Beta(a_r, b_r)                                      (revision propensity)
δ_i  ~ N(μ_δ, σ_δ²)                                        (practice gain)
θ_i(k) = θ_i + k δ_i                                       (ability at attempt k)
```

`τ` is speed: larger `τ` means shorter response times.

### 2.2 Items

Four options per item, one key. If `items` in the parameter file is empty,
items are drawn from the prior:

```
b_j ~ N(b_mean, b_sd²)            a_j = exp(N(log_a_mean, log_a_sd²))      c_j = item_prior.c
β_j ~ N(beta_mean, beta_sd²)      σ_j = exp(N(log_sigma_mean, log_sigma_sd²))
w_j ~ Dirichlet(distractor_alpha), sorted descending, assigned to the three
      wrong options in a random order;  key ~ U{0..3}
```

A pool item (`{"id","a","b","c","beta","sigma","distractor_weights":[3]}`)
provides all parameters except the key and the assignment of its distractor
weights to option positions; these are drawn from a stream keyed by
`(seed, id)`, so a pool item has the same key in every attempt of a run.
`β_j` is on the natural-log-millisecond scale.

Item sampling (`--item-sampling`):

* `fixed`: one quiz (`quiz_001`, `variant_001`) of `--questions` items
  shared by every student and attempt (legacy design). Question ids are
  `question_%03d` (prior) or the pool ids (sampled once without replacement).
* `per-attempt`: every attempt receives its own items, sampled without
  replacement from the pool, or drawn from the prior with ids
  `gen_sSSSSS_aKK_qQQQ`. `quiz_id`/`variant_id` remain `quiz_001`/`variant_001`
  so that the score history used by the detector's `z_score` is defined.
  The two members of a collusion pair answer the same items in `fi` and `bl`:
  both are (re-)simulated on the item set of the lower-index member.

### 2.3 Item response

Each item response uses one pre-drawn vector of variates (§6):
`U_rapid, U_corr, U_opt ~ U(0,1)`, `ε, ε_rg, ε_gap ~ N(0,1)` and, for each of
up to four changes, `(U_change, U_target, U_opt', ε_delay)`.

**Rapid guess** (`U_rapid < g_i`): `T = max(1, round(exp(μ_rg + s_rg ε_rg)))`;
the option is uniform over four (`key` if `U_corr < 1/4`, else one of the
three wrong options chosen by `U_opt`). Rapid guesses are never revised.

**Solution behaviour** otherwise:

```
L    = logistic(a_j (θ_i(k) − b_j))
P    = c_j + (1 − c_j) L
knows   ⇔ U_corr < L
correct ⇔ U_corr < P
```

Using one uniform gives `P(knows) = L`, `P(correct | ¬knows) = c_j` and
`knows ⇒ correct`. A wrong first answer is a distractor chosen by the
weights `w_j` with `U_opt`.

Response time (lognormal model, van der Linden 2006):

```
log T = β_j − τ_i + σ_j ε,     T = max(min_response_ms, round(exp(log T)))
```

**Revisions.** After the first answer the current option is changed with
probability

```
p₁ = min(0.95, r_i · change_mult_wrong)   if the current answer is wrong
p₁ = min(0.95, r_i · change_mult_right)   if it is right
p_m = second_change_prob                  for the 2nd, 3rd, 4th change
```

(at most four changes; the process stops at the first change not made). A
change from a wrong answer goes to the key with probability
`wrong_to_right`, otherwise to another wrong option by distractor weight
(excluding the current one); a change from the key goes to a distractor by
weight. Each change happens `max(1, round(exp(μ_d + s_d ε_delay)))` ms after
the previous answer.

### 2.4 Timeline

Navigation is linear. Item `q` is shown at `shown_q`; its events occur at
`shown_q + time_since_shown`; the student leaves the item at
`shown_q + dwell_q` (the time of its last in-place event) and the next item
is shown after a gap `max(1, round(exp(μ_gap + s_gap ε_gap)))`. After the
last item a review pause `max(1, round(exp(μ_rev + s_rev ε_rev)))` precedes
submission:

```
shown_0 = 0,   shown_{q+1} = shown_q + dwell_q + gap_q,
duration = shown_{n−1} + dwell_{n−1} + review
```

`time_since_shown_ms` is the wall-clock time since the item was first shown
(cumulative for changes); `time_since_attempt_start_ms = shown_q +
time_since_shown_ms`. Attempt `k` starts at
`start-at + k days + offset`, `offset ~ U{0..600000}` ms (legacy).

### 2.5 Default honest parameters (`params/default.json`)

| parameter | value | role |
|---|---|---|
| `person.theta_sd` | 1.0 | σ_θ |
| `person.tau_sd` | 0.30 | σ_τ |
| `person.theta_tau_corr` | 0.25 | ρ |
| `person.practice_gain_mean`, `_sd` | 0.10, 0.10 | μ_δ, σ_δ |
| `person.disengage.p_zero`, `beta_a`, `beta_b` | 0.75, 1.2, 12.0 | rapid-guess propensity |
| `person.revise.beta_a`, `beta_b` | 1.5, 20.0 | revision propensity |
| `item_prior.b_mean`, `b_sd` | −0.3, 1.0 | difficulty |
| `item_prior.log_a_mean`, `log_a_sd` | 0.0, 0.3 | discrimination |
| `item_prior.c` | 0.2 | guessing |
| `item_prior.beta_mean`, `beta_sd` | 10.1, 0.35 | time intensity (median ≈ 24 s) |
| `item_prior.log_sigma_mean`, `log_sigma_sd` | −0.7, 0.2 | residual log-time SD (≈ 0.50) |
| `item_prior.distractor_alpha` | [2.0, 1.0, 0.5] | distractor attraction |
| `rapid_guess.log_ms_mean`, `log_sd` | 7.6, 0.5 | rapid-guess time (median ≈ 2.0 s) |
| `revision.change_mult_wrong`, `change_mult_right` | 1.6, 0.8 | |
| `revision.wrong_to_right` | 0.55 | |
| `revision.delay_log_ms_mean`, `delay_log_sd` | 8.3, 0.7 | change delay (median ≈ 4.0 s) |
| `revision.second_change_prob` | 0.15 | |
| `gap.log_ms_mean`, `log_sd` | 7.3, 0.5 | inter-item gap (median ≈ 1.5 s) |
| `review.log_ms_mean`, `log_sd` | 9.9, 0.8 | review pause (median ≈ 20 s) |
| `min_response_ms` | 500 | floor of solution-behaviour times |

These defaults are literature-style priors, not estimates (`source.kind` is
`defaults`). An EdNet-calibrated file in the same schema (with a non-empty
`items` pool) replaces them for reported experiments; its `source` block
documents which values were estimated.

## 3. Behaviour-level mechanisms (`bl` world)

All mechanisms start from the student's honest final attempt (same draws,
start offset, gaps and review pause) and change only what the mechanism
acts on. `--intensity x` replaces the per-item coverage/propensity of every
mechanism (preknowledge `coverage`, lookup `propensity`, copy_peer
`coverage`) and, for proxy, scales the ability gain (default scale 1). The
value used is recorded in `truth.csv:intensity_used`.

### 3.1 `preknowledge` (leaked items; slot rapid_response)

Each item is compromised with probability `coverage`; a compromised item is
recalled with probability `recall`. A recalled item is answered with the key,
without revision and never as a rapid guess. The cheater is *naive* with
probability `naive_prob` (person level): naive cheaters answer recalled items
in `T' = max(min_response_ms, round(T · U(speedup_low, speedup_high)))`,
where `T` is the honest solution-time draw of that item; *disguising*
cheaters use `T` itself. Other items are honest.

| parameter | default |
|---|---|
| coverage | 0.65 |
| recall | 0.92 |
| naive_prob | 0.5 |
| speedup_low, speedup_high | 0.25, 0.60 |

### 3.2 `lookup` (external resource; slot excessive_changes)

On each item that is not a rapid guess and whose latent `knows` is false,
the student looks the answer up with probability `propensity`. The lookup
takes `L = max(1, round(exp(N(duration_log_ms_mean, duration_log_sd²))))`
and yields the key with probability `success`, otherwise a distractor by
weight. With probability `verify_mode_prob` (person level) the cheater is
in *verify* mode: the honest first answer is entered at the honest time `T`
and, if the looked-up answer differs, a change to it is entered at `T + L`.
Otherwise (*lookup-then-answer*) the single first answer is the looked-up
one at `T + L`. Honest revisions of looked-up items are dropped; the student
leaves the item at `T + L`.

| parameter | default |
|---|---|
| propensity | 0.70 |
| success | 0.90 |
| duration_log_ms_mean, duration_log_sd | 10.46, 0.60 (median ≈ 35 s) |
| verify_mode_prob | 0.50 |

### 3.3 `copy_peer` (copying from a peer; slot collusion)

Both members of a pair are labelled; `truth.csv:role` is `source` or
`copier`. The member with the higher `θ_i(k)` is the source with probability
`stronger_is_source_prob` (ties: lower index counts as stronger).

* **Copy decisions.** For each item the copier copies with probability
  `coverage`.
* **Source.** Answers honestly; after every item the copier copies, its gap
  is extended by a communication overhead
  `LogNormal(source_overhead_log_ms_mean, source_overhead_log_sd)` (for the
  last item the review pause is extended).
* **Copier start.** `start_copier = start_source + LogNormal(start_lag_*)`.
* **Copied item.** With relay delay `R ~ LogNormal(relay_*)` and entry time
  `E ~ LogNormal(entry_*)`: the source's first answer becomes available at
  `t_avail = t_source_first + R`; the copier answers at
  `t = max(shown + E, t_avail)` with the source's answer as it stood at
  `t − R` (which includes source revisions made before then). Each later
  source change on that item is followed with probability
  `follow_change_prob`, `R'` ~ LogNormal(relay_*) after the source change
  (and at least 1 ms after the copier's previous event on that item), if it
  falls before the copier's submission and differs from the copier's current
  answer. Follow-up changes can occur after the copier has moved on to later
  items; `time_since_shown_ms` then exceeds the item's dwell.
* **Non-copied items** are the copier's honest responses (including honest
  revisions).
* **Synchronised submission** (added 2026-11-14, see §8 item 11). With
  probability `sync_submit_prob` per pair (one Bernoulli from its own stream
  `copy-sync(lo,k)`, together with a sign coin and one normal, always
  drawn) the pair finishes together: with
  `d = max(1, round(exp(N(sync_submit_log_ms_mean, sync_submit_log_sd²))))`
  and a fair-coin sign, the member who would submit first lengthens its
  review pause so that it submits at `t_later ± d`. A review pause is never
  shortened: if `t_later − d` lies before the earlier finisher's own
  submission, both submissions are left unchanged (the pair still counts as
  sync-submitting). Follow-up changes of the copier are filtered against its
  final submission time. Non-sync pairs are byte-identical to simulator
  version 0.1.0 (tested against the 0.1.0 seed-42 output hashes).
* **Causality invariant** (tested): every copier answer on a copied item
  reproduces a source answer given strictly earlier on the same item.

| parameter | default |
|---|---|
| coverage | 0.80 |
| relay_log_ms_mean, relay_log_sd | 8.70, 0.50 (median ≈ 6.0 s) |
| entry_log_ms_mean, entry_log_sd | 8.52, 0.40 (median ≈ 5.0 s) |
| start_lag_log_ms_mean, start_lag_log_sd | 9.62, 0.80 (median ≈ 15 s) |
| follow_change_prob | 0.50 |
| stronger_is_source_prob | 0.80 |
| source_overhead_log_ms_mean, source_overhead_log_sd | 8.00, 0.50 (median ≈ 3.0 s) |
| sync_submit_prob (optional; default 0.5 if absent) | 0.5 |
| sync_submit_log_ms_mean, sync_submit_log_sd (optional; defaults 7.6, 0.7) | 7.6, 0.7 (median ≈ 2.0 s) |

`--intensity` does not change `sync_submit_prob`.

### 3.4 `proxy` (impersonation; slot score_jump)

The final attempt is taken by another person with

```
θ* ~ N(theta_mean, theta_sd²) truncated to θ* ≥ θ_i(k) + min_gain
θ' = θ_i(k) + x (θ* − θ_i(k))       (x = intensity, default 1)
τ' = σ_τ (ρ θ'/σ_θ + √(1−ρ²) z),   r' ~ Beta(a_r, b_r),   g' = 0
```

i.e. speed is drawn from the population distribution conditional on the
proxy's ability, revision propensity from its marginal, and the proxy does
not rapid-guess. The attempt is re-simulated with the honest item draws,
start offset, gaps and review pause. Truncated-normal sampling uses
inversion of the upper tail (stable for truncation points far in the tail).
`truth.csv:proxy_theta` records θ'.

| parameter | default |
|---|---|
| theta_mean | 1.2 |
| theta_sd | 0.5 |
| min_gain | 0.5 |

## 4. Feature-aligned injection recipes (`fi` world)

The legacy recipes of `generate_quiz_behavior.py` are ported onto the
honest attempt of the same student (constants in `mechanisms.json → fi`).

* **rapid_response.** Each item with probability `rapid_item_prob` (0.65):
  every event time within the item is multiplied by
  `U(rapid_scale_low, rapid_scale_high)` (0.18, 0.55) and rounded; the first
  answer is floored at `rapid_min_ms` (500); later events are kept strictly
  increasing.
* **excessive_changes.** Per item `p ~ U(changes_prob_low, changes_prob_high)`
  (0.45, 0.75); with probability `p` the item's events are replaced by
  `n + 1` answers, `n ~ U{1..changes_max_count}` (4), forming a random option
  sequence that ends in the honest final answer with consecutive options
  different (generated backwards from the final answer as in legacy), at
  `time_since_shown = T·(m+1) div (n+1)`, `m = 0..n`, where `T` is the honest
  first-answer time. Honest revisions of the item are dropped.
* **collusion.** Per pair the leader is the lower student index. A shared
  pattern marks `round_half_even(n · collusion_forced_wrong_fraction)`
  (at least 1 if the fraction is positive; 6 of 20 by default) items as
  forced; the forced option is the item's least attractive distractor (the
  legacy `wrong[-1]`, whose legacy weight 0.1 was the smallest), and on the
  other items the shared option is the leader's honest final answer. Each
  member takes the shared option on forced items and with probability
  `collusion_copy_prob` (0.90) on the others, otherwise its own honest final
  answer. Each member's item durations are the leader's honest item dwell
  times times `U(1−jitter, 1+jitter)` (jitter 0.03), gaps are the leader's
  gaps, no revisions. Start offsets are the leader's honest offset plus
  `U{0..collusion_start_window_ms}` (8000). Both members submit at
  `pair offset + Σ ceil(dwell·(1+jitter)) + Σ gaps + start_window +
  leader review + U{0..collusion_submit_window_ms}` (1000), i.e. within one
  second of a common target, by adjusting the final pause (legacy).
* **score_jump.** `Δ ~ U(score_jump_low, score_jump_high)` (0.18, 0.32) per
  attempt. Each item's probability of a correct first answer becomes
  `max(P, min(score_prob_cap, P + Δ))` (cap 0.98) with the same uniform
  `U_corr`, so a correct honest first answer stays correct. Timings are held
  at their honest values: first-answer times are unchanged and the number of
  changes and their delays are fixed to the honest ones (only the options of
  the change events are re-derived from the new first answer with the honest
  revision uniforms). `truth.csv:intensity_used` records Δ.

## 5. Outputs

| file | content |
|---|---|
| `honest.csv`, `fi.csv`, `bl.csv` | event logs, one per world requested with `--worlds` |
| `truth.csv` | one row per attempt per world with latent variables |
| `manifest.json` | arguments, parameter-file SHA-256s, simulator and Go version, per-output SHA-256, quotas, counts by type, counts by mechanism flag, `copy_peer_sync_submit_pairs` (sync / nosync pair counts) |

### Event CSV

Columns exactly
`event_id,event_type,version,occurred_at,producer,actor,context,payload,is_anomaly,anomaly_type`,
CRLF line endings and minimal quoting (Python `csv` defaults, as legacy).
`actor`, `context`, `payload` are compact JSON objects with the key order of
the legacy generator.

* `actor = {"user_id":"student_%05d","role":"student"}`,
  `context = {"course_id":"course_001","group_id":"group_%03d","session_id":…}`.
* `attempt_id`, `session_id`: UUIDv5 (URL namespace) of
  `examory-sim:{seed}:attempt:{i}:{k}` / `…:exam_session:{i}:{k}`.
* `event_id`: UUIDv7 whose 48-bit timestamp is `occurred_at` in ms and
  whose 74 random bits are the low 74 bits of
  `SHA-256(["examory-sim",seed,i,k,suffix])`, laid out as in the legacy
  `event_identifier` (suffix `answer:{q}:{m}` or `submitted`).
* `occurred_at`: RFC 3339 UTC, millisecond precision, `Z`.
* `answer_hash`: lowercase hex SHA-256 of the bytes `{"option":N}`.
* Rows sorted by (`occurred_at`, `event_id`).
* `quiz.answered` payload: `attempt_id, question_id, question_index,
  time_since_shown_ms, is_change, time_since_attempt_start_ms, answer_hash`;
  `quiz.submitted` payload: `attempt_id, quiz_id, variant_id, score,
  max_score, duration_ms, answers_changed`. `score` counts correct final
  answers; `answers_changed` counts change events.

The Go validator (`internal/events/validate.go`, `examory-sim validate`)
re-implements every check of `read_attempts` and additionally requires
strictly increasing times within a question, sorted rows, unique event ids,
and (in tests, with answer keys) that `score` equals the number of correct
final answers.

### truth.csv

| column | meaning |
|---|---|
| world | honest / fi / bl |
| attempt_id, user_id, group_id, attempt_index, is_final | identifiers |
| slot | the student's cheater slot (`none` for honest students); identical in all worlds, including `honest` |
| mechanism, is_anomaly | the value of `anomaly_type` / `is_anomaly` in that world |
| role | `source`/`copier` (copy_peer), `leader`/`member` (fi collusion) |
| partner_user_id | collusion partner |
| intensity_used | coverage/propensity (bl), ability-gain scale (proxy), Δ (fi score_jump), copy probability (fi collusion) |
| items_affected | items changed by the mechanism: recalled, looked up, copied (copier) or followed by overhead (source), all items (proxy), scaled / replaced / shared-option / correctness-flipped (fi) |
| theta, tau, disengage, revise, practice_gain, theta_attempt | latent person parameters; theta_attempt = θ_i(k) |
| mech_flag | `naive`/`disguising`, `verify`/`lookup_then_answer`, `source`/`copier`, `proxy`, `fi` |
| score, max_score, answers_changed, duration_ms, start_offset_ms | realised attempt summary |
| proxy_theta | θ' of the proxy |
| sync_submit | 1/0 for both members of a copy_peer pair (sync-submitting or not); empty for other rows |

## 6. Randomness and reproducibility

Every stream is `PCG(seed₁, seed₂)` from `math/rand/v2`, where
`(seed₁, seed₂)` are the first 16 bytes of
`SHA-256("examory-sim/v1:{seed}:{purpose}:{id₁}:{id₂}…")` (pool items:
`…:{purpose}\0{item id}`). Only raw 64-bit PCG output is used; uniforms,
normals (Box–Muller, two uniforms each), gamma (Marsaglia–Tsang), beta,
Dirichlet and integer sampling (Lemire) are implemented in `internal/rng` so
that the variate sequence does not depend on library sampling algorithms.

Streams: `person(i)`, `item(j)` / `quiz` / `attempt-items(i,k)` /
`attempt-item(i,k,q)` / `pool-item(id)`, `attempt(i,k)` (offset, review),
`item-response(i,k,q)` (all draws of one response, fixed count),
`labels`, and one stream per mechanism and recipe
(`preknowledge-person(i)`, `preknowledge-item(i,k,q)`, `lookup-person(i)`,
`lookup-item(i,k,q)`, `proxy(i)`, `copy-pair(lo,k)`, `copy-lag(lo,k)`,
`copy-item(copier,k,q)`, `fi-rapid(i,k,q)`, `fi-changes(i,k,q)`,
`fi-scorejump(i,k)`, `fi-collusion-pair(leader,k)`,
`fi-collusion-member(i,k)`). Because no stream is shared, adding or changing
a mechanism cannot shift any other variate.

The same arguments and parameter files give byte-identical outputs
(tested), on the same platform. Floating-point results can differ in the
last bit across CPU architectures (the Go compiler may fuse multiply–add on
arm64 but not amd64), which can change a rounded millisecond or a threshold
comparison; `manifest.json` records `go_version` and `goarch`. Changing
`--students` changes the cohort composition and the label shuffles, so
attempts are not nested across population sizes.

## 7. CLI

```
(cd sim && go build -o ../examory-sim ./cmd/examory-sim)     # from the repository root

examory-sim generate \
  --students 2000 --attempts 3 --questions 20 \
  --anomaly-rate 0.12 \
  --anomaly-weights "rapid_response=.35,excessive_changes=.25,collusion=.25,score_jump=.15" \
  --seed 42 --start-at 2026-10-01T09:00:00Z \
  --params sim/params/default.json --mechanisms sim/params/mechanisms.json \
  --item-sampling fixed            # or per-attempt
  [--intensity 0.5]                # optional override in [0,1]
  --worlds honest,fi,bl \
  --out-dir ml/datasets/raw/sim/seed42 [--force]

examory-sim validate ml/datasets/raw/sim/seed42/bl.csv
```

All flags shown have the displayed defaults except `--out-dir` (required).
Invalid arguments or parameter files produce an error message and exit code
2; an existing non-empty `--out-dir` is refused unless `--force` (which
overwrites the output files only). Files are written atomically. Parameter
files are validated for presence of every field, finiteness and ranges;
unknown fields are ignored (forward compatibility); `schema_version` must
be 1. A 2000 × 3 × 20 run of all three worlds takes about 1 s.

Evaluation with the detector pipeline (from the repository root):

```
.venv/bin/python -m ml.anomaly.split --csv DIR/bl.csv --seed 42 --output DIR/eval_bl/split.json
.venv/bin/python -m ml.anomaly.train --csv DIR/bl.csv --manifest DIR/eval_bl/split.json --output DIR/eval_bl
.venv/bin/python -m ml.anomaly.evaluate --csv DIR/bl.csv --model DIR/eval_bl/model.pkl --bootstrap 50 --output DIR/eval_bl/results.md
```

Tests: `cd sim && go test -race ./...`.

## 8. Limitations and assumptions

1. **Calibration.** With `params/default.json` every honest-model parameter
   is an assumed prior. A calibrated file in the same schema replaces some
   of them with estimates; which values were estimated, and from what, must
   be read from that file's `source` block. Any parameter not listed there
   as estimated is an assumption, and the paper should say so per
   parameter.
2. **All mechanism parameters are assumed.** No dataset of labelled
   cheating behaviour was available; `mechanisms.json` values are plausible
   magnitudes chosen before any detector was run, not estimates. Results
   should be reported as a function of `--intensity`, not at one setting.
3. **Mechanism repertoire.** Each slot has one mechanism with simple
   person-level heterogeneity (naive/disguising, verify/lookup-then-answer,
   source/copier). Mixed strategies within an attempt, cheating on
   non-final attempts, groups larger than two, adaptive cheaters who react
   to the detector, and collusion across cohorts are not modelled.
4. **Response model.** Local independence given θ; no item position,
   fatigue or time-pressure effects; no time limit; linear navigation
   (except copier follow-ups, which revisit items); a constant `c` per item;
   rapid-guess propensity independent of θ and of the item; revisions
   independent of response time.
5. **Proxy speed** is drawn conditional on the proxy's ability using the
   honest population's θ–τ correlation; this is an interpretation of
   "freshly drawn from the person distribution".
6. **fi collusion** uses the least attractive distractor as the forced
   option (port of the legacy rule) and the leader's honest dwell times as
   the legacy "durations".
7. **fi score_jump** shifts the probability of a correct *first* answer
   and holds the number of answer changes at the honest value so that
   timings stay unchanged; a boosted item that the honest attempt revised
   can therefore end on a distractor. The legacy recipe instead drew the
   final answer directly. The shift acts on the IRT probability of each
   item, so the realised score gain depends on the item and person
   parameters (2.6 points of 20 on average for the defaults, seed 42).
8. **Rapid-guess times** are floored at 1 ms, not at `min_response_ms`
   (the floor applies to solution behaviour only).
9. **Per-attempt sampling** keeps a single `quiz_id`/`variant_id`, so the
   detector's per-item timing references (≥ 30 observations per
   `question_id`) are mostly unavailable and fall back to the global
   reference; this is intended for realism comparisons, not for the main
   detector evaluation.
11. **Post-pilot amendment (2026-11-14, simulator 0.2.0).** The
    `copy_peer.sync_submit_*` parameters were added after a single seed-42
    pilot run on the uncalibrated defaults, before any multi-seed
    experiment. In the pilot only 9 of 90 copy_peer pairs submitted
    within the detector's 5-second peer window (median gap between the two
    submissions 20 s; the copier starts a lognormal lag after the source and
    the two then proceed at their own pace), against all 90 pairs of the fi
    collusion recipe, so the peer features rarely saw copying pairs at all. Colluding pairs plausibly
    often finish together, and the amendment moves the simulation in the
    direction that makes copying *easier* for the frozen detector to find,
    so it cannot inflate the gap between the fi and bl worlds in the bl
    world's disfavour. The value 0.5 is an assumption, not an estimate; no
    other parameter was changed, and results should be reported for
    `sync_submit_prob = 0` as well.
10. **Synthetic results are synthetic.** Detection rates on these data
    measure how a detector responds to the simulated behaviours; they are
    not estimates of real-misconduct detection rates.
