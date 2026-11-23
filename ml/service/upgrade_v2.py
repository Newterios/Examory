"""Package a verified v1 estimator for the v2 live attempt reader."""
from __future__ import annotations

import argparse
import os
import pickle
from pathlib import Path
from tempfile import TemporaryDirectory

from .artifact import load_verified, sha256_of
from .pipeline_v2 import POLICY


def upgrade(source: Path, digest: str, output: Path) -> str:
    if not digest or source.resolve() == output.resolve():
        raise ValueError("Supply the trusted source SHA-256 and a separate v2 output")
    artifact = load_verified(source, digest)
    if artifact["schema_version"] != 1:
        raise ValueError("The source must be v1")
    artifact.update(schema_version=2, version=f"{artifact['version']}-v2", input_policy=POLICY,
                    parent_sha256=digest.strip().lower())
    output.parent.mkdir(parents=True, exist_ok=True)
    with TemporaryDirectory(dir=output.parent) as tmp:
        target = Path(tmp) / "model.pkl"
        with target.open("wb") as stream:
            pickle.dump(artifact, stream, protocol=pickle.HIGHEST_PROTOCOL)
        target.chmod(0o644)
        # Link completed bytes only, without overwriting an existing version.
        os.link(target, output)
    return sha256_of(output)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model", type=Path, required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    print(upgrade(args.model, args.sha256, args.output))
