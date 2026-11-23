"""Sample a local Docker scorer's Linux cgroup v2 counters from the host."""
from __future__ import annotations

import argparse
import csv
import json
import math
import signal
import subprocess
import threading
import time
from pathlib import Path


def cgroup_dir(container: str) -> Path:
    result = subprocess.run(["docker", "inspect", "--type", "container", "--", container],
                            check=True, text=True, capture_output=True)
    info = json.loads(result.stdout)[0]
    if info["Config"]["Labels"].get("com.docker.compose.service") != "anomaly-scorer":
        raise ValueError("Select the anomaly-scorer container from the research Compose project")
    if not info["State"]["Running"]:
        raise ValueError("The scorer container is not running")
    proc = Path("/proc") / str(int(info["State"]["Pid"])) / "cgroup"
    for line in proc.read_text().splitlines():
        if line.startswith("0::"):
            hierarchy = Path("/sys/fs/cgroup")
            target = (hierarchy / line[3:].lstrip("/")).resolve()
            if target == hierarchy or not target.is_relative_to(hierarchy):
                break
            if (target / "cpu.stat").is_file() and (target / "memory.current").is_file():
                return target
    raise RuntimeError("Run on the Linux Docker host with access to the scorer's cgroup v2")


def sample(directory: Path) -> tuple[float, float, int, int]:
    monotonic = time.monotonic()
    wall = time.time()
    fields = dict(line.split() for line in (directory / "cpu.stat").read_text().splitlines())
    return wall, monotonic, int(fields["usage_usec"]), int((directory / "memory.current").read_text())


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--container", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--interval", default=2.0, type=float, help="Seconds between snapshots")
    args = parser.parse_args()
    if not math.isfinite(args.interval) or args.interval <= 0:
        parser.error("--interval must be positive and finite")
    directory = cgroup_dir(args.container)
    stopped = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stopped.set())
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("x", newline="", encoding="utf-8") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(("wall_epoch_s", "monotonic_s", "cpu_usage_usec", "memory_bytes"))
        while True:
            writer.writerow(sample(directory))
            stream.flush()
            if stopped.wait(args.interval):
                writer.writerow(sample(directory))
                break


if __name__ == "__main__":
    main()
