"""Typed manifest and report contracts for project documentation checks."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import cast

SCHEMA_VERSION = 1


@dataclass(frozen=True, slots=True)
class Family:
    root: str
    module_dir: str
    responsibility: str


@dataclass(frozen=True, slots=True)
class Manifest:
    max_root_lines: int
    max_module_lines: int
    families: tuple[Family, ...]
    single_owner_topics: dict[str, str]


@dataclass(frozen=True, slots=True)
class Violation:
    code: str
    path: str
    message: str


@dataclass(frozen=True, slots=True)
class Report:
    ok: bool
    schema_version: int
    root: str
    documents_checked: int
    families_checked: int
    violations: tuple[Violation, ...]


def _require_mapping(value: object, label: str) -> dict[str, object]:
    if not isinstance(value, dict):
        raise TypeError(f"{label} must be a JSON object")
    return cast("dict[str, object]", value)


def _require_string(
    mapping: dict[str, object],
    key: str,
    label: str,
) -> str:
    value = mapping.get(key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{label}.{key} must be a non-empty string")
    return value


def _require_positive_int(
    mapping: dict[str, object],
    key: str,
    label: str,
) -> int:
    value = mapping.get(key)
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        raise ValueError(f"{label}.{key} must be a positive integer")
    return value


def load_manifest(path: Path) -> Manifest:
    decoded = cast("object", json.loads(path.read_text(encoding="utf-8")))
    raw = _require_mapping(decoded, "manifest")
    schema_version = _require_positive_int(raw, "schema_version", "manifest")
    if schema_version != SCHEMA_VERSION:
        raise ValueError(
            f"manifest.schema_version={schema_version} is unsupported; "
            + f"expected {SCHEMA_VERSION}",
        )
    raw_families = raw.get("families")
    if not isinstance(raw_families, list) or not raw_families:
        raise ValueError("manifest.families must be a non-empty array")
    families: list[Family] = []
    for index, value in enumerate(cast("list[object]", raw_families)):
        family = _require_mapping(value, f"manifest.families[{index}]")
        families.append(
            Family(
                root=_require_string(
                    family,
                    "root",
                    f"manifest.families[{index}]",
                ),
                module_dir=_require_string(
                    family,
                    "module_dir",
                    f"manifest.families[{index}]",
                ),
                responsibility=_require_string(
                    family,
                    "responsibility",
                    f"manifest.families[{index}]",
                ),
            ),
        )
    raw_topics = _require_mapping(
        raw.get("single_owner_topics"),
        "manifest.single_owner_topics",
    )
    topics = {
        str(topic): _require_string(
            raw_topics,
            str(topic),
            "single_owner_topics",
        )
        for topic in raw_topics
    }
    return Manifest(
        max_root_lines=_require_positive_int(
            raw,
            "max_root_lines",
            "manifest",
        ),
        max_module_lines=_require_positive_int(
            raw,
            "max_module_lines",
            "manifest",
        ),
        families=tuple(families),
        single_owner_topics=topics,
    )


def repo_path(root: Path, relative: str) -> Path:
    candidate = (root / relative).resolve()
    if not candidate.is_relative_to(root):
        raise ValueError(f"path escapes repository root: {relative}")
    return candidate
