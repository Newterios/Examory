"""Generate (or reuse) datasets and load them with integrity checks.

Every generated directory carries ``generation.json`` with the exact command and
the SHA-256 of every output; reuse re-hashes the files and fails on mismatch.
"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import pandas as pd

from ml.anomaly.features import read_attempts

from .common import (
    CALIBRATION,
    EXP_DATA,
    LEGACY_GENERATOR,
    SIM_BINARY,
    SIM_DEFAULT_PARAMS,
    SIM_MECHANISMS,
    TYPES_OF_CONDITION,
    canonical,
    read_json,
    relative,
    sha256_file,
    sha256_text,
    slot_of,
    write_json,
)

WORLD_OF_CONDITION = {"SIM-FI": "fi", "SIM-BL": "bl", "SIM-HONEST": "honest"}
HEX64 = re.compile(r"^[0-9a-f]{64}$")


class IntegrityError(RuntimeError):
    """Inputs disagree with their recorded hashes or with each other."""


@dataclass
class Dataset:
    condition: str
    csv: Path
    seed: int
    attempts: list[dict[str, Any]]
    truth: pd.DataFrame | None = None
    record: dict[str, Any] = field(default_factory=dict)


def simulator_available() -> bool:
    return SIM_BINARY.exists() and os.access(SIM_BINARY, os.X_OK)


def resolve_params(explicit: str | None = None) -> tuple[Path, str]:
    """Calibrated EdNet parameters when present, else the uncalibrated defaults."""
    if explicit:
        return Path(explicit), "explicit"
    if CALIBRATION.exists():
        return CALIBRATION, "ednet_calibration"
    return SIM_DEFAULT_PARAMS, "sim_defaults"


def _verify_outputs(directory: Path, expected: dict[str, str]) -> None:
    for name, digest in expected.items():
        path = directory / name
        if not path.exists():
            raise IntegrityError(f"{relative(path)} is missing")
        actual = sha256_file(path)
        if actual != digest:
            raise IntegrityError(f"{relative(path)} SHA-256 {actual} differs from recorded {digest}")


def _manifest_hashes(manifest: Any) -> dict[str, str]:
    """Collect file→sha256 claims from a simulator manifest of unknown layout."""
    found: dict[str, str] = {}

    def walk(node: Any) -> None:
        if isinstance(node, dict):
            name = next((node[k] for k in ("file", "path", "name") if isinstance(node.get(k), str)), None)
            digest = next((node[k] for k in ("sha256", "csv_sha256") if isinstance(node.get(k), str)), None)
            if name and digest and HEX64.match(digest):
                found[Path(name).name] = digest
            for key, value in node.items():
                if isinstance(value, str) and HEX64.match(value) and key.endswith(".csv"):
                    found[Path(key).name] = value
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(manifest)
    return found


def _generate(directory: Path, command: list[str], outputs: list[str], spec: dict[str, Any],
              stage_tmp: Path | None = None) -> dict[str, Any]:
    """Run the generator into a temporary directory, then move it into place atomically."""
    record_path = directory / "generation.json"
    if record_path.exists():
        record = read_json(record_path)
        if record["spec"] != spec:
            raise IntegrityError(f"{relative(directory)} was generated from a different spec")
        _verify_outputs(directory, record["sha256"])
        return record
    directory.parent.mkdir(parents=True, exist_ok=True)
    staging = Path(tempfile.mkdtemp(prefix=directory.name + ".", dir=stage_tmp or directory.parent))
    try:
        resolved = [str(staging) if part == "{OUT}" else
                    str(staging / part[len("{OUT}/"):]) if part.startswith("{OUT}/") else part
                    for part in command]
        result = subprocess.run(resolved, capture_output=True, text=True, check=False)
        if result.returncode != 0:
            raise RuntimeError(f"generator failed ({result.returncode}): {' '.join(resolved)}\n{result.stderr[-4000:]}")
        hashes = {name: sha256_file(staging / name) for name in outputs}
        if (staging / "manifest.json").exists():
            claims = _manifest_hashes(read_json(staging / "manifest.json"))
            for name, digest in claims.items():
                if name in hashes and hashes[name] != digest:
                    raise IntegrityError(f"simulator manifest claims {name}={digest}, file is {hashes[name]}")
            spec_claims = sorted(set(claims) & set(hashes))
            for role, ref in (read_json(staging / "manifest.json").get("params") or {}).items():
                key = "params_sha256" if role == "params" else "mechanisms_sha256"
                if key in spec and ref.get("sha256") != spec[key]:
                    raise IntegrityError(f"simulator read a different {role} file than requested")
        else:
            spec_claims = []
        record = {"spec": spec, "command": [part.replace(str(staging), "{OUT}") for part in resolved],
                  "sha256": hashes, "manifest_verified_files": spec_claims,
                  "stderr_tail": result.stderr[-2000:]}
        write_json(staging / "generation.json", record)
        if directory.exists():  # a concurrent worker finished first
            shutil.rmtree(staging)
            return _generate(directory, command, outputs, spec)
        staging.rename(directory)
        return record
    finally:
        if staging.exists():
            shutil.rmtree(staging, ignore_errors=True)


def remove_dataset(directory: Path, root: Path = EXP_DATA) -> None:
    """Delete one generated dataset directory (only inside <root>/data)."""
    directory = Path(directory).resolve()
    if (root / "data").resolve() not in directory.parents:
        raise ValueError(f"refusing to delete {directory}: not a generated dataset")
    shutil.rmtree(directory, ignore_errors=True)


def ensure_legacy(seed: int, design: dict[str, Any], root: Path = EXP_DATA) -> tuple[Path, dict[str, Any]]:
    spec = {"generator": "legacy", "generator_sha256": sha256_file(LEGACY_GENERATOR), "seed": seed, **design}
    directory = root / "data" / "legacy" / sha256_text(canonical(spec))[:16]
    command = [sys.executable, str(LEGACY_GENERATOR), "--students", str(design["students"]),
               "--attempts", str(design["attempts"]), "--questions", str(design["questions"]),
               "--anomaly-rate", repr(design["anomaly_rate"]), "--seed", str(seed),
               "--output", "{OUT}/legacy.csv"]
    record = _generate(directory, command, ["legacy.csv"], spec)
    return directory / "legacy.csv", record


def ensure_sim(seed: int, design: dict[str, Any], worlds: tuple[str, ...], params: Path,
               mechanisms: Path = SIM_MECHANISMS, item_sampling: str | None = None,
               intensity: float | None = None, root: Path = EXP_DATA,
               binary: Path = SIM_BINARY, extra_args: tuple[str, ...] | list[str] | None = None
               ) -> tuple[Path, dict[str, Any]]:
    """``extra_args`` (e.g. flags of a newer simulator build) enter the spec only when given, so the
    data directories of every earlier stage keep their spec hash."""
    if not binary.exists():
        raise FileNotFoundError(f"simulator binary {relative(binary)} not built")
    spec = {"generator": "examory-sim", "binary_sha256": sha256_file(binary), "seed": seed, **design,
            "worlds": list(worlds), "params_sha256": sha256_file(params),
            "mechanisms_sha256": sha256_file(mechanisms), "item_sampling": item_sampling,
            "intensity": intensity}
    if extra_args:
        spec["extra_args"] = list(extra_args)
    directory = root / "data" / "sim" / sha256_text(canonical(spec))[:16]
    command = [str(binary), "generate", "--students", str(design["students"]),
               "--attempts", str(design["attempts"]), "--questions", str(design["questions"]),
               "--anomaly-rate", repr(design["anomaly_rate"]), "--seed", str(seed),
               "--params", str(params), "--mechanisms", str(mechanisms),
               "--worlds", ",".join(worlds), "--out-dir", "{OUT}", "--force"]
    if item_sampling:
        command += ["--item-sampling", item_sampling]
    if intensity is not None:
        command += ["--intensity", repr(intensity)]
    if extra_args:
        command += list(extra_args)
    outputs = [f"{w}.csv" for w in worlds] + ["truth.csv", "manifest.json"]
    record = _generate(directory, command, outputs, spec)
    record = dict(record, params=relative(params), mechanisms=relative(mechanisms))
    return directory, record


def read_truth(path: Path, world: str) -> pd.DataFrame:
    truth = pd.read_csv(path, dtype=str, keep_default_na=False)
    for column in ("world", "attempt_id"):
        if column not in truth:
            raise IntegrityError(f"{relative(path)} lacks column {column}")
    truth = truth[truth.world == world]
    if truth.attempt_id.duplicated().any():
        raise IntegrityError(f"duplicate truth rows for world {world}")
    return truth.set_index("attempt_id")


def load_dataset(condition: str, csv: Path, seed: int, record: dict[str, Any],
                 truth_path: Path | None = None) -> Dataset:
    expected = record["sha256"].get(csv.name)
    if expected is None or sha256_file(csv) != expected:
        raise IntegrityError(f"{relative(csv)} differs from its generation record")
    attempts = read_attempts(csv)
    if any(a["is_anomaly"] is None for a in attempts):
        raise IntegrityError(f"{relative(csv)} lacks labels")
    allowed = set(TYPES_OF_CONDITION.get(condition, ())) | {"none"}
    observed = {a["anomaly_type"] for a in attempts}
    if condition in TYPES_OF_CONDITION and not observed <= allowed:
        raise IntegrityError(f"{condition}: unexpected anomaly types {sorted(observed - allowed)}")
    truth = None
    if truth_path is not None:
        truth = read_truth(truth_path, WORLD_OF_CONDITION[condition])
        missing = [a["attempt_id"] for a in attempts if a["attempt_id"] not in truth.index]
        if missing:
            raise IntegrityError(f"{condition}: {len(missing)} attempts have no truth row, e.g. {missing[0]}")
        for a in attempts:
            row = truth.loc[a["attempt_id"]]
            if "user_id" in truth and row["user_id"] != a["user_id"]:
                raise IntegrityError(f"{condition}: truth user mismatch for {a['attempt_id']}")
            if "is_anomaly" in truth and int(row["is_anomaly"]) != a["is_anomaly"]:
                raise IntegrityError(f"{condition}: truth is_anomaly differs for {a['attempt_id']}")
            if a["is_anomaly"] and "mechanism" in truth and row["mechanism"] not in ("", a["anomaly_type"]):
                raise IntegrityError(f"{condition}: truth mechanism {row['mechanism']} "
                                     f"≠ label {a['anomaly_type']} for {a['attempt_id']}")
    return Dataset(condition, csv, seed, attempts, truth, record)


def assert_paired(first: Dataset, second: Dataset, manifest_a: dict, manifest_b: dict) -> None:
    """Paired counterfactual worlds: same attempts, same cheaters, same cohort split."""
    if manifest_a["groups"] != manifest_b["groups"]:
        raise IntegrityError(f"cohort partition differs between {first.condition} and {second.condition}")
    ids_a = {a["attempt_id"] for a in first.attempts}
    ids_b = {a["attempt_id"] for a in second.attempts}
    if ids_a != ids_b:
        raise IntegrityError(f"attempt sets differ between {first.condition} and {second.condition}")
    cheat_a = {a["attempt_id"] for a in first.attempts if a["is_anomaly"]}
    cheat_b = {a["attempt_id"] for a in second.attempts if a["is_anomaly"]}
    if cheat_a != cheat_b:
        raise IntegrityError(f"cheater sets differ between {first.condition} and {second.condition}")
    slots_a = {a["attempt_id"]: a["anomaly_type"] for a in first.attempts if a["is_anomaly"]}
    slots_b = {a["attempt_id"]: a["anomaly_type"] for a in second.attempts if a["is_anomaly"]}
    if any(slot_of(slots_a[k]) != slot_of(slots_b[k]) for k in slots_a):
        raise IntegrityError("slot assignment differs between paired worlds")
