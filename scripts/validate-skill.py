#!/usr/bin/env python3
"""
Validate agent-harness skill metadata without relying on host-managed system
skill copies or optional PyYAML installation.
"""

from __future__ import annotations

import ast
import re
import sys
from pathlib import Path


MAX_SKILL_NAME_LENGTH = 64
ALLOWED_PROPERTIES = {"name", "description", "license", "allowed-tools", "metadata"}


class FrontmatterError(ValueError):
    pass


def _parse_scalar(raw: str):
    value = raw.strip()
    if value == "":
        return ""
    if value in {"true", "True"}:
        return True
    if value in {"false", "False"}:
        return False
    if value in {"null", "Null", "NULL", "~"}:
        return None
    if value[0] in {"'", '"'}:
        try:
            return ast.literal_eval(value)
        except (SyntaxError, ValueError) as exc:
            raise FrontmatterError(f"invalid quoted scalar: {exc}") from exc
    return value


def parse_frontmatter(text: str) -> dict[str, object]:
    data: dict[str, object] = {}
    current_key = ""
    current_block: list[str] = []

    def flush_block() -> None:
        nonlocal current_key, current_block
        if current_key:
            data[current_key] = "\n".join(current_block).rstrip("\n")
            current_key = ""
            current_block = []

    for line_no, line in enumerate(text.splitlines(), start=1):
        if current_key:
            if line.startswith((" ", "\t")) or line.strip() == "":
                current_block.append(line)
                continue
            flush_block()

        if line.lstrip().startswith("#") or line.strip() == "":
            continue
        if ":" not in line:
            raise FrontmatterError(f"line {line_no}: expected key: value")
        key, raw_value = line.split(":", 1)
        key = key.strip()
        if not key:
            raise FrontmatterError(f"line {line_no}: empty key")
        if key in data:
            raise FrontmatterError(f"line {line_no}: duplicate key {key!r}")
        if raw_value.strip() == "":
            current_key = key
            current_block = []
            continue
        data[key] = _parse_scalar(raw_value)

    flush_block()
    return data


def validate_skill(skill_path: Path) -> tuple[bool, str]:
    skill_md = skill_path / "SKILL.md"
    if not skill_md.exists():
        return False, "SKILL.md not found"

    content = skill_md.read_text(encoding="utf-8")
    if not content.startswith("---"):
        return False, "No YAML frontmatter found"

    match = re.match(r"^---\n(.*?)\n---", content, re.DOTALL)
    if not match:
        return False, "Invalid frontmatter format"

    try:
        frontmatter = parse_frontmatter(match.group(1))
    except FrontmatterError as exc:
        return False, f"Invalid frontmatter: {exc}"

    unexpected_keys = set(frontmatter) - ALLOWED_PROPERTIES
    if unexpected_keys:
        allowed = ", ".join(sorted(ALLOWED_PROPERTIES))
        unexpected = ", ".join(sorted(unexpected_keys))
        return (
            False,
            f"Unexpected key(s) in SKILL.md frontmatter: {unexpected}. "
            f"Allowed properties are: {allowed}",
        )

    if "name" not in frontmatter:
        return False, "Missing 'name' in frontmatter"
    if "description" not in frontmatter:
        return False, "Missing 'description' in frontmatter"

    name = frontmatter.get("name", "")
    if not isinstance(name, str):
        return False, f"Name must be a string, got {type(name).__name__}"
    name = name.strip()
    if name:
        if not re.match(r"^[a-z0-9-]+$", name):
            return (
                False,
                f"Name '{name}' should be hyphen-case "
                "(lowercase letters, digits, and hyphens only)",
            )
        if name.startswith("-") or name.endswith("-") or "--" in name:
            return (
                False,
                f"Name '{name}' cannot start/end with hyphen or contain consecutive hyphens",
            )
        if len(name) > MAX_SKILL_NAME_LENGTH:
            return (
                False,
                f"Name is too long ({len(name)} characters). "
                f"Maximum is {MAX_SKILL_NAME_LENGTH} characters.",
            )

    description = frontmatter.get("description", "")
    if not isinstance(description, str):
        return False, f"Description must be a string, got {type(description).__name__}"
    if not description.strip():
        return False, "Description cannot be empty"

    return True, "Skill is valid!"


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("Usage: validate-skill.py <skill-path> [<skill-path>...]", file=sys.stderr)
        return 2

    failed = False
    for arg in argv[1:]:
        skill_path = Path(arg)
        ok, message = validate_skill(skill_path)
        if len(argv) == 2:
            print(message)
        else:
            status = "ok" if ok else "error"
            print(f"{skill_path}: {status}: {message}")
        failed = failed or not ok
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
