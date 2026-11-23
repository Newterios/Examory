"""Validate events against docs/03_architecture/schemas (draft 2020-12)."""
from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from jsonschema import Draft202012Validator
from referencing import Registry, Resource

BASE = "https://examory.dev/schemas/"


def find_dir(start: Path | None = None) -> Path:
    here = (start or Path(__file__)).resolve()
    for parent in [here, *here.parents]:
        candidate = parent / "docs" / "03_architecture" / "schemas"
        if candidate.is_dir():
            return candidate
    raise FileNotFoundError("docs/03_architecture/schemas not found")


class EventValidator:
    def __init__(self, directory: Path) -> None:
        resources = []
        for path in sorted(directory.glob("*.schema.json")):
            doc = json.loads(path.read_text(encoding="utf-8"))
            resources.append((BASE + path.name, Resource.from_contents(doc)))
        if not resources:
            raise FileNotFoundError(f"no schemas in {directory}")
        self._registry = Registry().with_resources(resources)
        self._cache: dict[str, Draft202012Validator] = {}

    def errors(self, event: dict[str, Any]) -> list[str]:
        name = f"{event.get('event_type')}.v{event.get('version')}.schema.json"
        validator = self._cache.get(name)
        if validator is None:
            try:
                schema = self._registry.contents(BASE + name)
            except Exception:  # noqa: BLE001 - unknown schema
                return [f"no schema {name}"]
            validator = Draft202012Validator(schema, registry=self._registry)
            self._cache[name] = validator
        return [f"{'/'.join(map(str, e.absolute_path))}: {e.message}"[:300] for e in validator.iter_errors(event)]
