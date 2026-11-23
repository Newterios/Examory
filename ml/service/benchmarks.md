# E-06 simulator replay benchmark

The scorer ran on a local M4 research stack based on PR #31 at `ac76cb6`. The experiment used the E-05 instrumentation patch and the existing Isolation Forest. The artifact was trained by `make m4` on the standard seed-42, 2,000-student demo set. Both replay modes used that frozen artifact. Its version was `0.1.0` and its SHA-256 was `b145ed42a116e4ac55b5708fad899ac58ff35995e39e4c97febe0fc17c160c9a`.

| Measurement host | Configuration |
|---|---|
| Processor | AMD Ryzen 5 7535HS, 6 physical cores, 12 logical processors |
| OS and memory | Windows build 26300, 15.25 GiB usable host RAM. Ubuntu 24.04 on WSL2, kernel 6.6.87.2, 7.38 GiB visible to Linux |
| Containers | Docker Engine 29.1.3, Compose 2.40.3, one anomaly-scorer container without an explicit CPU or memory quota |
| Software | Go 1.26.11, Python 3.12.16, scikit-learn 1.7.2, NumPy 2.3.5, pandas 3.0.1 |

## Offline and service parity

The small world had eight students, two attempts each and 20 fixed questions. The replay used the behavioural world (`bl`) with seed 42, the EdNet calibration and the standard simulator mechanisms. It restored simulated UTC event times through the quizzes API. The original CSV SHA-256 was `d251a70aec8ee638df732960157c9896a7bd83a53c28a91033bb062cefa01775`.

| Observation | Result |
|---|---:|
| CSV answer and submission events | 406 |
| Attempts submitted through the real API and scored | 16 / 16 |
| Native offline raw scores equal to the stored service scores | 16 / 16 |
| Maximum absolute raw-score difference | 0 |
| Reconstructed feature comparisons within absolute tolerance 1e-9 | 240 / 240 |
| Flagged attempts with a published risk | 5 |
| Scored attempts below the risk threshold | 11 |
| Attempts excluded from reconstruction | 0 |

Eight attempts had previous score history. Eight had missing history. None had a peer submitting within five seconds. A separate regression test covers the positive peer case, including a missing peer in the risk evidence and a changed final answer. The five alerts in this experiment establish agreement with the offline detector. Estimating detection precision would require comparing predictions with independent labels.

## Load measurement

The load world contained 300 students, three attempts each and 20 fixed questions. The client took the chronological prefix for 20 event-bearing operations per second over 900 seconds. Question displays were additional HTTP calls. Provisioning and compilation were outside the measured interval. The load stack had its own database, broker, scorer and volumes.

The first provisioning attempt stopped at the auth service's rate limit before it emitted any quiz events. The client then spaced administrative writes at least 250 ms apart and respected HTTP 429 responses. The measured run used fresh cohorts and the original platform limits. The final client includes additional provisioning error handling. The workload scheduler and quiz requests used in the measurement are the same.

The measured window ran from 22:05:24.259438508 to 22:20:24.259675867 UTC on 22 November 2026 (03:05–03:20 on 23 November in Kazakhstan). All 15 one-minute intervals contained 1,200 successful event dispatches. The window covered 900.000237 seconds.

| Lifecycle event | Successful client operations | Quizzes outbox | Scorer inbox |
|---|---:|---:|---:|
| `quiz.started` | 884 | 884 | 884 |
| `quiz.answered` | 16,516 | 16,516 | 16,516 |
| `quiz.submitted` | 600 | 600 | 600 |
| **Total** | **18,000** | **18,000** | **18,000** |

The API also accepted 13,770 question displays. There were zero HTTP failures. Event dispatch lag behind the scheduled slot was 1.18 ms at p99, with a maximum of 194.16 ms. No event was dispatched outside the nominal window. At the cutoff, 284 started attempts had not reached submission. All 600 submitted attempts had received a final score. Of these, 159 produced an acknowledged risk and 441 were below the alert threshold. The maximum number of submitted attempts awaiting a score at any instant was 27. The count at window end was zero.

The latency origin was the real creation time of the quiz submission outbox row. The endpoint was the scorer's broker delivery callback as recorded by E-05 in `risk_outbox.published_at`. Both processes shared the local host clock. This interval includes the quiz outbox relay, Kafka consumption, scoring and delivery of the risk event. Simulated event time was used for feature engineering only. The query followed submitted attempts through the drain period and matched them by live attempt ID. Attempts below the alert threshold have no risk-publication timestamp.

The sampler ran on the Linux Docker host and collected the scorer's cgroup v2 CPU and memory counters every two seconds. CPU percentages use one logical processor as the denominator. Average memory is time-weighted and includes charged cache. Short peaks between samples may be larger.

| Submission to acknowledged signal | Observed time, 159 signals |
|---|---:|
| p50 | 2,702.957 ms |
| p95 | 5,844.713 ms |
| p99 | 8,582.081 ms |
| Mean | 3,053.314 ms |

| Scorer resource | Mean | Sampled peak |
|---|---:|---:|
| CPU, percent of one logical processor | 13.68% | 96.86% over a sampling interval |
| Container memory, MiB | 126.64 | 142.23 |

The scorer used 123.09 CPU-seconds across 451 sample intervals and had zero restarts. These figures describe one 15-minute run with one scorer on this machine at the specified load.

Both final Make targets also completed successfully on M4. `make replay-parity` repeated the 16-attempt result, including all start times and the eight expected missing-history z-scores. A 10-second `make replay-load` smoke run delivered 200 events to both databases, with five submissions, one acknowledged risk and zero HTTP failures. The latency and resource figures above are from the full 900-second run.

See the [replay instructions](../replay/README.md) for the world generation flags, Make targets and metric definitions.
