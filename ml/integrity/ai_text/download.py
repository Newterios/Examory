"""Download the two pinned E-18 corpora into a private local directory."""

import argparse
import hashlib
import shutil
import tempfile
import urllib.request
from pathlib import Path

SOURCES = {
    "all.jsonl": (
        "https://huggingface.co/datasets/Hello-SimpleAI/HC3/resolve/4d0ff18143b5a7e1b1e79beb540c04549d1e59d3/all.jsonl",
        "ee231f82283d754050a4dc58ecf9331afaf5daea7283c6da0b096ed0fb2774cf",
    ),
    "toefl.json": (
        "https://raw.githubusercontent.com/Weixin-Liang/ChatGPT-Detector-Bias/1d05ed8242d4694a0ed7b73a79c52be8864470f9/Data_and_Results/Human_Data/TOEFL_real_91/data.json",
        "d5e2bf0bd4831262d60aa936b2ac8e532bd3859067411288b6c22ed16867666b",
    ),
}


def download(directory: Path):
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=directory) as scratch:
        for name, (url, digest) in SOURCES.items():
            dest = directory / name
            if dest.exists():
                with dest.open("rb") as f:
                    if hashlib.file_digest(f, "sha256").hexdigest() == digest:
                        continue
                raise ValueError("An existing input differs from its pinned checksum")
            tmp = Path(scratch) / name
            with (
                urllib.request.urlopen(url, timeout=120) as source,
                tmp.open("wb") as out,
            ):
                shutil.copyfileobj(source, out)
            with tmp.open("rb") as f:
                if hashlib.file_digest(f, "sha256").hexdigest() != digest:
                    raise ValueError("Corpus checksum mismatch")
            tmp.replace(dest)


if __name__ == "__main__":
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--directory", required=True, type=Path)
    download(ap.parse_args().directory)
