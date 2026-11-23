#!/usr/bin/env bash
# Run from the repository root, against a local stack started by make m4.
set -euo pipefail
umask 077
if [[ $# != 4 || ( $1 != parity && $1 != load ) || -z $2 || -z $3 || -z $4 ]]; then
    echo 'Usage: bash ml/replay/run.sh parity|load world.csv keys.json new-output-directory' >&2
    exit 2
fi
mode=$1
world=$(realpath -e -- "$2")
keys=$(realpath -e -- "$3")
mkdir -m 700 -- "$4"
out=$(realpath -- "$4")
root=$(pwd -P)
set -a
. ./.env
set +a
export E2E_ADMIN_EMAIL="${E2E_ADMIN_EMAIL:-$AUTH_BOOTSTRAP_ADMIN_EMAIL}"
export E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-$AUTH_BOOTSTRAP_ADMIN_PASSWORD}"
export COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml:docker-compose.research.yml}"
(cd sim && go build -o "$out/examory-replay" ./cmd/examory-replay)

sampler=
stop_sampler() {
    if [[ -n $sampler ]]; then
        local pid=$sampler
        sampler=
        kill -TERM "$pid" 2>/dev/null || true
        wait "$pid"
    fi
}
if [[ $mode == load ]]; then
    scorer=$(docker compose ps -q anomaly-scorer)
    postgres=$(docker compose ps -q postgres)
    python3 -m ml.replay.resources --container "$scorer" --output "$out/resources.csv" &
    sampler=$!
    trap stop_sampler EXIT
    for ((i=0; i<100; i++)); do
        [[ -s "$out/resources.csv" ]] && break
        kill -0 "$sampler"
        sleep 0.1
    done
    [[ -s "$out/resources.csv" ]] || { echo 'No initial scorer resource sample' >&2; exit 1; }
fi

client_status=0
"$out/examory-replay" --mode "$mode" --world "$world" --keys "$keys" --out "$out/replay.json" \
    --rate "${REPLAY_RATE:-20}" --duration "${REPLAY_DURATION:-15m}" || client_status=$?
if (( client_status != 0 )); then
    if [[ ! -s "$out/replay.json" ]] || ! python3 - "$out/replay.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    manifest = json.load(stream)
sys.exit(0 if manifest.get("window_start") and manifest.get("window_end") else 1)
PY
    then
        echo 'Replay stopped before a complete measurement window. See client output.' >&2
        exit "$client_status"
    fi
fi
if [[ $mode == parity ]]; then
    docker compose run --rm --no-deps -T --user "$(id -u):$(id -g)" \
        -v "$root/ml/replay:/app/ml/replay:ro" -v "$world:/source.csv:ro" -v "$out:/replay" \
        --entrypoint python anomaly-scorer -m ml.replay.parity \
        --world /source.csv --manifest /replay/replay.json --model /model/model.pkl \
        --report /replay/parity.json --wait-seconds "${REPLAY_DRAIN_SECONDS:-180}"
else
    # The sampler's final point brackets the end even when the last request finishes between ticks.
    stop_sampler
    trap - EXIT
    python3 -m ml.replay.load_report --manifest "$out/replay.json" --samples "$out/resources.csv" \
        --postgres "$postgres" --output "$out/load.json" --drain-seconds "${REPLAY_DRAIN_SECONDS:-1800}"
fi
exit "$client_status"
