"""Load and verify the model artifact.

SECURITY: the artifact is a Python pickle (written by ml/anomaly/train.py).
Unpickling executes code chosen by whoever wrote the file, so it must only be
loaded from a trusted local build (`make anomaly-model`). Mitigations here:
the file must be a regular, non-world-writable file; if ANOMALY_MODEL_SHA256
is set, the SHA-256 is checked BEFORE unpickling; after loading, the schema
version, required structure and library versions are verified.
"""
from __future__ import annotations

import hashlib
import importlib.metadata
import math
import os
import stat
from pathlib import Path
from typing import Any

from ml.anomaly.train import load_artifact

from .detectors import NAMES

SUPPORTED_SCHEMA_VERSION = 1
PINNED_LIBRARIES = ("numpy", "pandas", "scikit-learn")


class ArtifactError(RuntimeError):
    """The artifact is missing, untrusted or incompatible; the service must not start."""


def sha256_of(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def load_verified(path: str | Path, expected_sha256: str | None = None) -> dict[str, Any]:
    p = Path(path)
    try:
        info = p.stat()
    except OSError as exc:
        raise ArtifactError(f"model artifact not found at {p}") from exc
    if not stat.S_ISREG(info.st_mode):
        raise ArtifactError("model artifact must be a regular file")
    if info.st_mode & stat.S_IWOTH:
        raise ArtifactError("model artifact is world-writable; refusing to unpickle it")
    if expected_sha256:
        actual = sha256_of(p)
        if actual != expected_sha256.strip().lower():
            raise ArtifactError("model artifact SHA-256 does not match ANOMALY_MODEL_SHA256")
    try:
        artifact = load_artifact(p)  # frozen loader; checks schema_version == 1
    except (ValueError, KeyError) as exc:
        raise ArtifactError(f"unsupported artifact: {exc}") from exc
    verify(artifact)
    return artifact


def verify(artifact: dict[str, Any]) -> None:
    if not isinstance(artifact, dict) or artifact.get("schema_version") != SUPPORTED_SCHEMA_VERSION:
        raise ArtifactError("artifact schema_version is not supported")
    for name in NAMES:
        model = artifact.get("models", {}).get(name)
        if not isinstance(model, dict) or model.get("kind") != name:
            raise ArtifactError(f"missing or mismatched {name} model")
        specific = "sorted_train" if name == "quantile_rules" else "estimator"
        for key in ("columns", "medians", specific, "train_scores", "medium_threshold", "high_threshold", "bounds"):
            if key not in model:
                raise ArtifactError(f"{name} model lacks {key}")
        medium, high = model["medium_threshold"], model["high_threshold"]
        if not (math.isfinite(medium) and math.isfinite(high) and high >= medium):
            raise ArtifactError(f"invalid {name} thresholds")
        if name == "quantile_rules":
            ref = model["sorted_train"]
            if getattr(ref, "ndim", None) != 2 or ref.shape[1] != 2 * len(model["columns"]) or not len(ref):
                raise ArtifactError("invalid quantile_rules training reference shape")
    if "reference" not in artifact or "version" not in artifact:
        raise ArtifactError("artifact lacks reference/version")
    built = artifact.get("environment", {}).get("libraries", {})
    for lib in PINNED_LIBRARIES:
        if built.get(lib) != importlib.metadata.version(lib):
            raise ArtifactError(f"artifact built with {lib} {built.get(lib)}, runtime has "
                                f"{importlib.metadata.version(lib)}; retrain with `make anomaly-model`")


def expected_sha_from_env() -> str | None:
    value = os.environ.get("ANOMALY_MODEL_SHA256", "").strip()
    return value or None
