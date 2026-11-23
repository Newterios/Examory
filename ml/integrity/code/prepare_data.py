"""Prepare a component-disjoint Java pair sample from the Toma BigCloneBench release."""

import argparse
import csv
import hashlib
import json
import random
import shutil
import tempfile
import urllib.request
import zipfile
from collections import Counter, defaultdict
from pathlib import Path

REVISION = "15f74517668318369d567680d2089d8d2a417cdc"
SOURCE = f"https://raw.githubusercontent.com/CGCL-codes/Toma/{REVISION}/dataset"
HASHES = {
    "id2sourcecode.zip": "24ec9c2c91ea487aae41e8f734ebb2689487767d5494bfa6568d8f9c4142fe7f",
    "type-1.csv": "32129f7371ecdf8c880213f15d34f08ee426d833a151a3d96b0d535040340d8b",
    "type-2.csv": "6695f0f9f29b0c3768e711a4581482bdd4c72fd8b917f895c8deb74f4345745b",
    "type-3.csv": "81003ea13ebf689e6dc736657ac3f47e9508faf535d52cc35d0cd1b02a1e1c5b",
    "type-4.csv": "38bb6ce102b68dbdd56c60baad20df1964b6ba64e9ce68ef2f10b0c73a4c001c",
    "type-5.csv": "2a6f80127f95ebb9a90572368bc2ca19a3b4b4b08f4684d8979fa20d85e76829",
    "nonclone.csv": "97b9a500bfcda83c371e6f278e6d1a33032dce379e6779d11c653f433a38e65a",
}
CLASSES = {"type-1.csv": "T1", "type-2.csv": "T2", "type-3.csv": "ST3",
           "type-4.csv": "MT3", "type-5.csv": "WT3/T4", "nonclone.csv": "nonclone"}


class Components:
    """Use the smallest function ID as a stable component key."""

    def __init__(self) -> None:
        self.parent: dict[int, int] = {}

    def find(self, function: int) -> int:
        self.parent.setdefault(function, function)
        while self.parent[function] != function:
            self.parent[function] = self.parent[self.parent[function]]
            function = self.parent[function]
        return function

    def join(self, left: int, right: int) -> None:
        left, right = self.find(left), self.find(right)
        self.parent[max(left, right)] = min(left, right)


def checksum(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def verify_inputs(cache: Path, download: bool) -> None:
    cache.mkdir(parents=True, exist_ok=True)
    for name, expected in HASHES.items():
        dest = cache / name
        if not dest.is_file() and download:
            with tempfile.TemporaryDirectory(dir=cache) as staging:
                partial = Path(staging) / name
                with urllib.request.urlopen(f"{SOURCE}/{name}", timeout=90) as response, partial.open("wb") as out:
                    shutil.copyfileobj(response, out)
                if checksum(partial) != expected:
                    raise ValueError(f"Unexpected SHA-256 for download {name}")
                partial.replace(dest)
        elif not dest.is_file():
            raise FileNotFoundError(f"Missing {dest}. Use --download to fetch the pinned files.")
        if checksum(dest) != expected:
            raise ValueError(f"Unexpected SHA-256 for {dest}")


def fold(seed: int, family: int) -> str:
    byte = hashlib.sha256(f"{seed}:{family}".encode()).digest()[0]
    return "test" if byte % 2 else "validation"


def sample(pairs: list[tuple[int, int]], families: dict[int, int],
           count: int, seed: str) -> list[tuple[int, int]]:
    """Round-robin across left-endpoint families after a seeded shuffle."""
    rng = random.Random(seed)
    buckets: dict[int, list[tuple[int, int]]] = defaultdict(list)
    for a, b in sorted(pairs):
        buckets[families[a]].append((a, b))
    active = sorted(buckets)
    rng.shuffle(active)
    for family in active:
        rng.shuffle(buckets[family])
    result = []
    while active and len(result) < count:
        for family in active:
            result.append(buckets[family].pop())
            if len(result) == count:
                break
        active = [family for family in active if buckets[family]]
    return result


def prepare(cache: Path, output: Path, seed: int, per_class: int) -> None:
    groups = Components()
    first_copy: dict[bytes, int] = {}
    members: dict[int, str] = {}
    labels: dict[tuple[int, int], str] = {}
    with zipfile.ZipFile(cache / "id2sourcecode.zip") as archive:
        for entry in archive.infolist():
            if entry.is_dir():
                continue
            path = Path(entry.filename)
            if path.suffix != ".java" or not path.stem.isdecimal():
                raise ValueError(f"Unexpected archive member {entry.filename}")
            index = int(path.stem)
            if index in members:
                raise ValueError(f"Duplicate source ID {index}")
            members[index] = entry.filename
            raw = archive.read(entry)
            raw.decode("utf-8")  # Fail rather than silently change source offsets.
            digest = hashlib.sha256(raw).digest()
            groups.join(index, first_copy.setdefault(digest, index))

        for filename, label in CLASSES.items():
            with (cache / filename).open(encoding="utf-8", newline="") as inp:
                for row in csv.reader(inp):
                    if row[:2] == ["FUNCTION_ID_ONE", "FUNCTION_ID_TWO"]:
                        continue
                    left, right = sorted(map(int, row[:2]))
                    if left == right or left not in members or right not in members:
                        raise ValueError(f"Invalid pair in {filename}: {left}, {right}")
                    if labels.setdefault((left, right), label) != label:
                        raise ValueError(f"Conflicting labels for {left}, {right}")
                    if label != "nonclone":
                        groups.join(left, right)

        families = {f: groups.find(f) for f in members}
        folds = {g: fold(seed, g) for g in set(families.values())}
        eligible: dict[tuple[str, str], list[tuple[int, int]]] = defaultdict(list)
        for (left, right), label in labels.items():
            split = folds[families[left]]
            if label != "WT3/T4" and split == folds[families[right]]:
                eligible[split, label].append((left, right))
        selected = []
        for split in ("validation", "test"):
            for label, count in (("T1", per_class), ("T2", per_class), ("ST3", per_class // 2),
                                 ("MT3", per_class - per_class // 2), ("nonclone", per_class)):
                pairs = sample(eligible[split, label], families, count, f"{seed}:{split}:{label}")
                if not pairs:
                    raise ValueError(f"No {label} pairs available in {split} for seed {seed}")
                selected.extend((a, b, label, split) for a, b in pairs)

        # Assign both endpoints of each selected negative pair to one bootstrap block.
        # Work on a second graph: negative edges must never determine the fold split.
        blocks = Components()
        for a, b, _, _ in selected:
            blocks.join(families[a], families[b])
        output.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=output) as tmp:
            directory = Path(tmp)
            ids = sorted({i for a, b, _, _ in selected for i in (a, b)})
            with (directory / "functions.jsonl").open("w", encoding="utf-8", newline="\n") as out:
                for i in ids:
                    func = {"id": str(i), "code": archive.read(members[i]).decode("utf-8")}
                    out.write(json.dumps(func, ensure_ascii=False) + "\n")
            with (directory / "pairs.csv").open("w", encoding="utf-8", newline="") as out:
                writer = csv.writer(out, lineterminator="\n")
                writer.writerow(["left_id", "right_id", "clone_type", "split", "group", "source_type"])
                for a, b, label, split in selected:
                    typ = "T3" if label in {"ST3", "MT3"} else label
                    writer.writerow([a, b, typ, split, f"g{blocks.find(families[a])}", label])
            counts = Counter((split, label) for _, _, label, split in selected)
            provenance = {
                "dataset": "BigCloneBench (Toma study distribution)", "revision": REVISION, "source": SOURCE,
                "input_sha256": HASHES, "seed": seed, "requested_pairs_per_class_per_split": per_class,
                "functions_in_archive": len(members), "source_components": len(folds), "functions_selected": len(ids),
                "unique_pairs_in_release": dict(Counter(labels.values())),
                "eligible_pairs": {f"{s}/{t}": len(p) for (s, t), p in eligible.items()},
                "sampled_pairs": {f"{s}/{t}": n for (s,t), n in counts.items()},
                "output_sha256": {name: checksum(directory / name) for name in ("pairs.csv", "functions.jsonl")},
            }
            (directory / "provenance.json").write_text(json.dumps(provenance, indent=2, sort_keys=True) + "\n", encoding="utf-8")
            files = list(directory.iterdir())
            for p in files:
                dest = output / p.name
                if dest.exists() and checksum(dest) != checksum(p):
                    raise FileExistsError(f"{dest} differs; choose a fresh output directory.")
            for p in files:
                p.replace(output / p.name)
    print(f"{len(selected)} labeled pairs and {len(ids)} Java functions written to {output}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--download", action="store_true", help="Fetch missing files from the pinned Toma release")
    parser.add_argument("--seed", default=42, type=int)
    parser.add_argument("--per-class", default=64, type=int)
    args = parser.parse_args()
    if args.per_class < 2:
        parser.error("--per-class must be at least 2 to include strong and moderate Type-3")
    verify_inputs(args.cache, args.download)
    prepare(args.cache, args.output, args.seed, args.per_class)


if __name__ == "__main__":
    main()
