# Simulator replay on M4

Run these commands from the repository root on a local Linux Docker host with cgroup v2, Bash, GNU Make, Go (see `sim/go.mod`) and Python 3.11 or newer. The comparator uses the model and Python dependencies already installed in the M4 scorer image. Use a disposable research stack. The client creates synthetic users, quizzes and attempts in it.

## Generate the two source worlds

```bash
make m4
umask 077
study=$(mktemp -d)
(cd sim && go build -o "$study/examory-sim" ./cmd/examory-sim)
(cd sim && go build -o "$study/keys" ./cmd/examory-replay-keys)
for mode in parity load; do
    if [[ $mode == parity ]]; then students=8; attempts=2; else students=300; attempts=3; fi
    "$study/examory-sim" generate --students "$students" --attempts "$attempts" --questions 20 \
        --seed 42 --item-sampling fixed --start-at 2026-10-01T09:00:00Z \
        --params ml/ednet/calibration.json --mechanisms sim/params/mechanisms.json \
        --worlds bl --out-dir "$study/$mode"
    "$study/examory-sim" validate "$study/$mode/bl.csv"
    "$study/keys" --manifest "$study/$mode/manifest.json" --out "$study/$mode/keys.json"
done
```

The key exporter regenerates the simulator's fixed roster from its seed and checks the parameter-file digests. The CSV and key file stay in the private study directory. The client checks the question order, option hashes, reported scores and durations against the keys before starting the replay. The tested scope is a single fixed quiz with four options per question and complete attempts.

## Compare every attempt

```bash
make replay-parity REPLAY_WORLD="$study/parity/bl.csv" \
    REPLAY_KEYS="$study/parity/keys.json" REPLAY_OUT="$study/parity-run"
```

The same workflow can run directly via `bash ml/replay/run.sh parity "$study/parity/bl.csv" "$study/parity/keys.json" "$study/parity-run"`. Supply a new output directory for each run. The driver reads the M4 administrator credentials from the local `.env`, uses the gateway for provisioning and the loopback quizzes research endpoint for time-stamped requests. Default ports are 8000 and 8001. A separately named research stack can set `COMPOSE_PROJECT_NAME`, `COMPOSE_FILE`, `E2E_GATEWAY_URL` and `E2E_QUIZZES_URL`.

`parity.json` compares each native offline Isolation Forest raw score with its stored service score. A second comparison projects the original CSV into live quiz and option IDs, then checks the retained events, features reconstructed from them, terminal status, risk level and published evidence. The service retains a raw score for each scorable attempt and publishes a risk only for flagged attempts. The feature reconstruction uses the final cohort event history. Direct comparison with the stored raw scores can expose differences caused by the history available at scoring time. The comparator reads a consistent database snapshot and verifies the frozen model against the SHA-256 pin used by the scorer.

Numeric fields use an absolute tolerance of `1e-9`. Identifiers and statuses require equality. When parity fails, the command exits unsuccessfully and writes the differences to the report if the inputs were valid. The report uses synthetic attempt numbers. The separate private `replay.json` maps simulator IDs to the platform's IDs.

## Measure fifteen minutes of load

```bash
make replay-load REPLAY_WORLD="$study/load/bl.csv" \
    REPLAY_KEYS="$study/load/keys.json" REPLAY_OUT="$study/load-run"
```

The default load is 20 event-bearing operations per second for 900 seconds. Quiz starts, answer saves and submissions each consume one slot. Question displays are additional API requests. A run uses the first 18,000 event-bearing operations in virtual-time order and can end mid-attempt. Provisioning finishes before the measured window. For shorter diagnostics set `REPLAY_RATE` or `REPLAY_DURATION`, for example `REPLAY_DURATION=2m`. Choose an input large enough for that many operations. A prefix containing zero submissions cannot supply latency measurements.

`load.json` reports actual dispatches per minute, HTTP failures, durable event counts on both sides of the broker, submitted and partial attempts, backlog at window end and time to drain. The wait for the scorer and publisher defaults to 1,800 seconds after the load window (`REPLAY_DRAIN_SECONDS`). An incomplete run exits unsuccessfully. For signalled attempts, latency starts at creation of the quiz submission outbox row and ends when the anomaly risk outbox records the broker acknowledgement. Both timestamps are stored in PostgreSQL. With the local E-05 metrics patch, the start is the submission transaction's wall time and the end is the scorer's UTC delivery-callback time on the same host. On the uninstrumented PR #31 base, the end is the time of the database transaction that records acknowledgement. No latency is assigned to attempts that do not emit a signal.

`resources.csv` samples the scorer's cumulative cgroup CPU time and current memory usage every two seconds. The summary covers the measured window only, with interpolated boundary samples. CPU is a percentage of one logical core. Memory includes charged cache. The reported peak is the largest sampled value in the window. The private replay manifest and the original samples can be used to recompute the report before the research database is removed. Keep these files outside Git.

## Verification

```bash
(cd sim && go test -race ./cmd/examory-replay ./cmd/examory-replay-keys)
python -m unittest ml.replay.test_load_report
# In an environment with ml/service/requirements.txt installed
python -m unittest ml.replay.test_parity
```

The workload and observed results are in [the benchmark report](../service/benchmarks.md). The [client README](../../sim/cmd/examory-replay/README.md) documents scheduling, limits and the private manifest format.
