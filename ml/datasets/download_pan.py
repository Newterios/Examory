# ml/datasets/download_pan.py
"""Download verified PAN-PC-11 split RAR archives from the official Zenodo record.

Python's standard library cannot extract multipart RAR. --extract uses an
installed 7-Zip executable; no additional Python dependency is introduced.
Without --download, only print/save the official metadata and download sizes.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import urllib.request
from pathlib import Path
from typing import Any

RECORD = "https://zenodo.org/api/records/3250095"
EXPECTED = {
    "pan-plagiarism-corpus-2011.part1.rar": "b2930f859497dd48ba5bb606d3f4a4f3",
    "pan-plagiarism-corpus-2011.part2.rar": "b23d86c17a47d2bfbdc4c314ea5810df",
}


def request(url: str) -> Any:
    return urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": "Examory-research/0.1"}), timeout=120)


def md5(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "md5").hexdigest()


def download_file(url: str, target: Path, expected: str) -> None:
    if target.exists() and md5(target) == expected:
        print(f"Verified existing {target.name}")
        return
    temporary = target.with_suffix(target.suffix + ".partial")
    with request(url) as response, temporary.open("wb") as stream:
        shutil.copyfileobj(response, stream, length=1024 * 1024)
    if md5(temporary) != expected:
        raise ValueError(f"Checksum failed: {temporary}; do not extract")
    temporary.replace(target)
    print(f"Verified {target.name}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path("ml/datasets/raw/pan-pc-11"))
    parser.add_argument("--download", action="store_true")
    parser.add_argument("--extract", action="store_true")
    parser.add_argument("--extractor", help="Path to installed 7z/7zz executable")
    args = parser.parse_args()
    if args.extract and not args.download:
        parser.error("--extract requires --download (existing verified archives are reused)")
    with request(RECORD) as response:
        record = json.load(response)
    files = [f for f in record["files"] if f["key"] in EXPECTED]
    if len(files) != 2 or any(f["checksum"] != "md5:" + EXPECTED[f["key"]] for f in files):
        raise ValueError("Official record differs from the pinned release")
    args.output.mkdir(parents=True, exist_ok=True)
    metadata = {"record": RECORD, "doi": "10.5281/zenodo.3250095", "files": files,
                "terms": "Official description permits use free of charge for research; no standard license ID is stated."}
    (args.output / "download_manifest.json").write_text(json.dumps(metadata, indent=2), encoding="utf-8")
    print(f"PAN-PC-11: {sum(f['size'] for f in files):,} bytes; research-use terms at https://zenodo.org/records/3250095")
    if args.download:
        for file in sorted(files, key=lambda f: f["key"]):
            download_file(file["links"]["self"], args.output / file["key"], EXPECTED[file["key"]])
    if args.extract:
        candidates = [args.extractor, shutil.which("7z"), shutil.which("7zz"),
                      "C:/Program Files/7-Zip/7z.exe", "C:/Program Files (x86)/7-Zip/7z.exe"]
        executable = next((p for p in candidates if p and Path(p).is_file()), None)
        if executable is None:
            parser.error("Archives downloaded and verified. Install 7-Zip or extract both parts manually, starting with part1.rar.")
        destination = args.output / "extracted"
        destination.mkdir(exist_ok=True)
        subprocess.run([executable, "x", str((args.output / next(iter(EXPECTED))).resolve()),
                        "-o" + str(destination.resolve()), "-aos", "-y"], check=True)


if __name__ == "__main__":
    main()
