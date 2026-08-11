# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Validate agent-harness operating-document ownership and navigation."""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Literal, cast
from urllib.parse import unquote, urlparse

from .project_docs_optimize.documentation_contract import (
    SCHEMA_VERSION,
    Manifest,
    Report,
    Violation,
    load_manifest,
    repo_path,
)

MARKDOWN_LINK = re.compile(r"!?\[[^\]]*]\(([^)]+)\)")
INLINE_CODE = re.compile(r"`[^`]*`")

def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate modular agent-harness operating documentation.",
    )
    _ = parser.add_argument("--root", type=Path, default=Path.cwd())
    _ = parser.add_argument(
        "--mode",
        choices=("report", "check"),
        default="check",
    )
    _ = parser.add_argument("--json", action="store_true", dest="json_output")
    return parser.parse_args(argv)


def line_count(path: Path) -> int:
    return len(path.read_text(encoding="utf-8").splitlines())


def markdown_targets(path: Path) -> tuple[str, ...]:
    targets: list[str] = []
    fenced = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced:
            continue
        prose = INLINE_CODE.sub("", line)
        targets.extend(
            match.group(1).strip() for match in MARKDOWN_LINK.finditer(prose)
        )
    return tuple(targets)


def local_target(source: Path, raw_target: str) -> Path | None:
    target = raw_target.strip("<>").split(maxsplit=1)[0]
    parsed = urlparse(target)
    if parsed.scheme or target.startswith("#"):
        return None
    relative = unquote(target.split("#", maxsplit=1)[0])
    if not relative:
        return None
    return (source.parent / relative).resolve()


def validate(root: Path, manifest: Manifest) -> Report:
    docs_root = root / ".agent-harness"
    violations: list[Violation] = []
    markdown_files = sorted(docs_root.rglob("*.md"))
    agents = root / "AGENTS.md"
    if agents.is_file():
        markdown_files.append(agents)

    for family in manifest.families:
        root_doc = repo_path(root, family.root)
        module_dir = repo_path(root, family.module_dir)
        if not root_doc.is_file():
            violations.append(
                Violation("missing_root", family.root, "required root index is missing"),
            )
            continue
        if line_count(root_doc) > manifest.max_root_lines:
            violations.append(
                Violation(
                    "line_budget_exceeded",
                    family.root,
                    f"root index exceeds {manifest.max_root_lines} lines",
                ),
            )
        if not module_dir.is_dir():
            violations.append(
                Violation(
                    "missing_module_dir",
                    family.module_dir,
                    "module directory is missing",
                ),
            )
            continue
        modules = sorted(module_dir.rglob("*.md"))
        if not modules:
            violations.append(
                Violation(
                    "empty_module_dir",
                    family.module_dir,
                    "module directory has no Markdown documents",
                ),
            )
        for module in modules:
            if line_count(module) > manifest.max_module_lines:
                violations.append(
                    Violation(
                        "line_budget_exceeded",
                        str(module.relative_to(root)),
                        f"module exceeds {manifest.max_module_lines} lines",
                    ),
                )
        linked_paths = {
            target
            for raw in markdown_targets(root_doc)
            if (target := local_target(root_doc, raw)) is not None
        }
        if not any(
            target == module_dir or target.is_relative_to(module_dir)
            for target in linked_paths
        ):
            violations.append(
                Violation(
                    "module_dir_unlinked",
                    family.root,
                    f"root index does not link into {family.module_dir}",
                ),
            )

    for topic, owner in manifest.single_owner_topics.items():
        if not repo_path(root, owner).is_file():
            violations.append(
                Violation(
                    "missing_owner",
                    owner,
                    f"canonical owner for {topic!r} is missing",
                ),
            )

    for document in markdown_files:
        for raw in markdown_targets(document):
            target = local_target(document, raw)
            if target is not None and not target.exists():
                violations.append(
                    Violation(
                        "broken_link",
                        str(document.relative_to(root)),
                        f"target does not exist: {raw}",
                    ),
                )

    ordered = tuple(
        sorted(violations, key=lambda item: (item.path, item.code, item.message)),
    )
    return Report(
        ok=not ordered,
        schema_version=SCHEMA_VERSION,
        root=str(root),
        documents_checked=len(markdown_files),
        families_checked=len(manifest.families),
        violations=ordered,
    )


def render(report: Report, *, json_output: bool) -> None:
    if json_output:
        print(json.dumps(asdict(report), ensure_ascii=False, indent=2))
        return
    status = "ok" if report.ok else "failed"
    print(
        f"project docs check {status}: {report.documents_checked} documents, "
        + f"{len(report.violations)} violations",
    )
    for violation in report.violations:
        print(f"- [{violation.code}] {violation.path}: {violation.message}")


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = cast("Path", args.root).resolve()
    mode = cast("Literal['report', 'check']", args.mode)
    json_output = cast("bool", args.json_output)
    try:
        manifest = load_manifest(
            root / ".agent-harness" / "documentation" / "manifest.json",
        )
        report = validate(root, manifest)
    except (OSError, TypeError, ValueError, json.JSONDecodeError) as error:
        report = Report(
            ok=False,
            schema_version=SCHEMA_VERSION,
            root=str(root),
            documents_checked=0,
            families_checked=0,
            violations=(Violation("invalid_input", str(root), str(error)),),
        )
    render(report, json_output=json_output)
    return 1 if mode == "check" and not report.ok else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
