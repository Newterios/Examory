"""Constants, provenance records and small I/O helpers shared by all stages."""
from __future__ import annotations

import hashlib
import importlib.metadata
import json
import math
import platform
import subprocess
import sys
from pathlib import Path
from typing import Any

import numpy as np

REPO = Path(__file__).resolve().parents[3]
PACKAGE = Path(__file__).resolve().parent
RESULTS = PACKAGE / "results"
STAGES_DIR = RESULTS / "stages"
TABLES_DIR = RESULTS / "tables"
FIGURES_DIR = RESULTS / "figures"
EXP_DATA = REPO / "ml" / "datasets" / "raw" / "exp"
LEGACY_GENERATOR = REPO / "ml" / "datasets" / "generate_quiz_behavior.py"
SIM_BINARY = REPO / "sim" / "bin" / "examory-sim"
SIM_DEFAULT_PARAMS = REPO / "sim" / "params" / "default.json"
SIM_MECHANISMS = REPO / "sim" / "params" / "mechanisms.json"
CALIBRATION = REPO / "ml" / "ednet" / "calibration.json"
EDNET_DIR = REPO / "ml" / "datasets" / "raw" / "ednet"

DEFAULT_SEEDS = list(range(1001, 1021))
MAIN_DESIGN = {"students": 2000, "attempts": 3, "questions": 20, "anomaly_rate": 0.12}
PREVALENCE_RATE = 0.0167
MAX_WORKERS = 10

CONDITIONS = ("LEGACY", "SIM-FI", "SIM-BL")
DETECTORS = ("iforest", "iforest_no_peers", "quantile_rules")
ROBUST_DETECTORS = ("lof", "lof_no_peers", "ocsvm", "ocsvm_no_peers", "mcd", "mcd_no_peers")
FI_TYPES = ("rapid_response", "excessive_changes", "collusion", "score_jump")
BL_TYPES = ("preknowledge", "lookup", "copy_peer", "proxy")
SLOTS = ("S1", "S2", "S3", "S4")
SLOT_OF_TYPE = {**{t: s for s, t in zip(SLOTS, FI_TYPES, strict=True)},
                **{t: s for s, t in zip(SLOTS, BL_TYPES, strict=True)}}
SLOT_LABEL = {s: f"{f}↔{b}" for s, f, b in zip(SLOTS, FI_TYPES, BL_TYPES, strict=True)}
TYPES_OF_CONDITION = {"LEGACY": FI_TYPES, "SIM-FI": FI_TYPES, "SIM-BL": BL_TYPES}
FPR_TARGETS = (0.05, 0.01)
PAUC_MAX_FPR = 0.05
ROC_GRID = np.round(np.linspace(0.0, 0.1, 51), 6)
WITHIN_RUN_BOOTSTRAP = 500
TRANSFER_PAIRS = (("LEGACY", "SIM-BL"), ("SIM-FI", "SIM-BL"), ("SIM-BL", "SIM-FI"),
                  ("LEGACY", "SIM-FI"), ("LEGACY", "LEGACY"), ("SIM-FI", "SIM-FI"),
                  ("SIM-BL", "SIM-BL"))
INTENSITIES = (0.1, 0.25, 0.5, 0.75, 1.0)
TIMING_KEYS = {"wall_seconds", "timing", "started_at", "finished_at"}


def slot_of(anomaly_type: str) -> str | None:
    return SLOT_OF_TYPE.get(anomaly_type)


def sha256_file(path: str | Path) -> str:
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def sha256_text(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), default=str)


def _git(*args: str) -> str:
    try:
        result = subprocess.run(["git", *args], cwd=REPO, capture_output=True, text=True, check=False)
    except FileNotFoundError:
        return ""
    return result.stdout.strip() if result.returncode == 0 else ""


def frozen_pipeline_hashes() -> dict[str, Any]:
    """Git blob hashes of ml/anomaly/*.py at HEAD and in the working tree."""
    files = {}
    for path in sorted((REPO / "ml" / "anomaly").glob("*.py")):
        relative = path.relative_to(REPO).as_posix()
        head = _git("rev-parse", f"HEAD:{relative}")
        work = _git("hash-object", relative)
        files[relative] = {"head_blob": head or None, "worktree_blob": work or None,
                           "modified": bool(head) and head != work}
    return files


def library_versions() -> dict[str, str]:
    versions = {"python": platform.python_version()}
    for name in ("numpy", "pandas", "scikit-learn", "scipy", "matplotlib"):
        try:
            versions[name] = importlib.metadata.version(name)
        except importlib.metadata.PackageNotFoundError:
            versions[name] = "missing"
    return versions


def provenance() -> dict[str, Any]:
    status = _git("status", "--porcelain", "--", "ml/anomaly")
    record = {
        "git_head": _git("rev-parse", "HEAD") or None,
        "frozen_pipeline": frozen_pipeline_hashes(),
        "frozen_pipeline_clean": status == "",
        "libraries": library_versions(),
        "platform": platform.platform(),
        "python_executable": sys.executable,
        "harness_sha256": {p.name: sha256_file(p) for p in sorted(PACKAGE.glob("*.py"))},
    }
    if SIM_BINARY.exists():
        record["simulator_binary_sha256"] = sha256_file(SIM_BINARY)
    record["simulator_source_sha256"] = simulator_source_hash()
    return record


def simulator_source_hash() -> str | None:
    """SHA-256 over (path, content hash) of the simulator's Go sources (sim/ is untracked)."""
    root = REPO / "sim"
    files = sorted(p for p in root.rglob("*") if p.is_file() and (p.suffix == ".go" or p.name == "go.mod"))
    if not files:
        return None
    lines = [f"{p.relative_to(REPO).as_posix()} {sha256_file(p)}" for p in files]
    return sha256_text("\n".join(lines))


def clean(value: Any) -> Any:
    """Convert numpy/pandas scalars to JSON; NaN/inf become None."""
    if isinstance(value, dict):
        return {str(k): clean(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [clean(v) for v in value]
    if isinstance(value, np.ndarray):
        return [clean(v) for v in value.tolist()]
    if isinstance(value, (np.bool_, bool)):
        return bool(value)
    if isinstance(value, (np.integer,)):
        return int(value)
    if isinstance(value, (np.floating, float)):
        value = float(value)
        return value if math.isfinite(value) else None
    if isinstance(value, Path):
        try:
            return value.resolve().relative_to(REPO).as_posix()
        except ValueError:
            return str(value)
    return value


def round_floats(value: Any, digits: int = 6) -> Any:
    """Round every float to ``digits`` significant digits (compact committed outputs)."""
    if isinstance(value, dict):
        return {k: round_floats(v, digits) for k, v in value.items()}
    if isinstance(value, list):
        return [round_floats(v, digits) for v in value]
    if isinstance(value, float):
        return float(f"{value:.{digits}g}")
    return value


def write_json(path: str | Path, value: Any, compact: bool = False, digits: int = 6) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    value = clean(value)
    if compact:
        text = json.dumps(round_floats(value, digits), separators=(",", ":"), ensure_ascii=False, allow_nan=False)
    else:
        text = json.dumps(value, indent=1, ensure_ascii=False, allow_nan=False)
    path.write_text(text + "\n", encoding="utf-8")


def read_json(path: str | Path) -> Any:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def strip_timing(value: Any) -> Any:
    """Remove wall-clock entries (used by determinism checks)."""
    if isinstance(value, dict):
        return {k: strip_timing(v) for k, v in value.items() if k not in TIMING_KEYS}
    if isinstance(value, list):
        return [strip_timing(v) for v in value]
    return value


def relative(path: str | Path) -> str:
    path = Path(path).resolve()
    try:
        return path.relative_to(REPO).as_posix()
    except ValueError:
        return str(path)
