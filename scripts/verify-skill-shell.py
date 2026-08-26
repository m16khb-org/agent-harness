#!/usr/bin/env python3

from __future__ import annotations

import re
import shlex
import shutil
import subprocess
import sys
import os
from pathlib import Path
from typing import NamedTuple, Sequence


FENCE_START = re.compile(r"^```(bash|sh|zsh|shell|console)(?:\s+.*)?$")
FENCE_END = re.compile(r"^```\s*$")
ANNOTATION = re.compile(
    r'^<!--\s*skill-shell:\s*(skip|destructive)\s+'
    r'(reason|recovery)="([^"]+)"\s*-->\s*$'
)
FABRICATED_DEFAULT = re.compile(r"\|\|\s*echo\s+0(?:\s|$|\))")
PARAMETER_ZERO_DEFAULT = re.compile(r"\$\{[A-Za-z_][A-Za-z0-9_]*:-0\}")
FAILURE_SWALLOW = re.compile(r"\|\|\s*(?:true|:)(?:\s|$|\))")
UNQUOTED_BISECT_COMMAND = re.compile(r"\bgit\s+bisect\s+run\s+\$[A-Za-z_][A-Za-z0-9_]*")
UNQUOTED_COMMAND_LOOP = re.compile(r"\bfor\s+[A-Za-z_][A-Za-z0-9_]*\s+in\s+\$\(")
UNQUOTED_VARIABLE_LOOP = re.compile(r"\bfor\s+[A-Za-z_][A-Za-z0-9_]*\s+in\s+\$[A-Za-z_][A-Za-z0-9_]*")
DESTRUCTIVE_GIT = re.compile(
    r"\bgit\s+(?:"
    r"reset\s+--hard|"
    r"clean\s+(?:--force\b|-[a-zA-Z]*f[a-zA-Z]*)|"
    r"branch\s+-D|"
    r"rebase(?:\s|$)|"
    r"bisect\s+(?:start|reset|skip|bad|good)\b"
    r")"
)
DYNAMIC_SHELL = re.compile(
    r"(?:^|[;&|]\s*)(?:eval|source|\.)\s+|"
    r"(?:^|[;&|]\s*)(?:ba|z|k)?sh\s+-c(?:\s|$)|"
    r"\$\(\s*(?:eval|source|(?:ba|z|k)?sh\s+-c)\b"
)
RECOVERY_EVIDENCE = re.compile(
    r"\b(?:verify|verified|record|retain|confirm|approval|backup|reset|abort|restore\s+from)\b",
    re.IGNORECASE,
)
DOCUMENTED_PLACEHOLDER = re.compile(r"<[A-Za-z][^>\n]*>")
SHELL_LAUNCHERS = frozenset({"sh", "bash", "zsh", "ksh", "dash", "ash", "fish"})
USAGE = "usage: verify-skill-shell.py [SKILL.md|skill-directory ...]"


class Annotation(NamedTuple):
    kind: str
    detail: str


class ShellBlock(NamedTuple):
    path: Path
    language: str
    line: int
    content: str
    annotation: Annotation | None


class Violation(NamedTuple):
    path: Path
    line: int
    code: str
    message: str


def verify_paths(paths: Sequence[Path]) -> list[Violation]:
    violations = symlink_violations(paths)
    for path in markdown_paths(paths):
        for block in shell_blocks(path):
            violations.extend(verify_block(block))
    return sorted(violations, key=lambda item: (str(item.path), item.line, item.code))


def markdown_paths(paths: Sequence[Path]) -> list[Path]:
    discovered: set[Path] = set()
    for raw_path in paths:
        if raw_path.is_symlink():
            continue
        path = raw_path.resolve()
        if path.is_file():
            if is_skill_contract_markdown(path):
                discovered.add(path)
            continue
        if not path.is_dir():
            continue
        for candidate in path.rglob("*.md"):
            if not candidate.is_symlink() and is_skill_contract_markdown(candidate):
                discovered.add(candidate.resolve())
    return sorted(discovered)


def symlink_violations(paths: Sequence[Path]) -> list[Violation]:
    violations: list[Violation] = []
    seen: set[Path] = set()
    for raw_path in paths:
        if raw_path.is_symlink():
            seen.add(raw_path)
            continue
        if not raw_path.is_dir():
            continue
        for root, directories, files in os.walk(raw_path, followlinks=False):
            root_path = Path(root)
            for name in [*directories, *files]:
                candidate = root_path / name
                if not candidate.is_symlink():
                    continue
                if root_path == raw_path and is_external_skill_link(candidate):
                    continue
                seen.add(candidate)
    for path in sorted(seen):
        violations.append(
            Violation(
                path,
                1,
                "symlink-not-allowed",
                "skill contracts and reference ancestry must remain inside the scanned tree",
            )
        )
    return violations


def external_skill_links(paths: Sequence[Path]) -> list[Path]:
    """Local, non-portable links to whole external skills directly under a scanned root."""
    links: list[Path] = []
    for raw_path in paths:
        if raw_path.is_symlink() or not raw_path.is_dir():
            continue
        for candidate in sorted(raw_path.iterdir()):
            if candidate.is_symlink() and is_external_skill_link(candidate):
                links.append(candidate)
    return links


def is_external_skill_link(path: Path) -> bool:
    """A symlink directly under a scanned skills root that resolves to a whole skill
    directory (one carrying its own SKILL.md) is a local link to an external skill,
    not a shipped contract. The installer and `inspect` already ignore such
    entries, so the verifier skips them instead of failing; nested symlinks and
    links without a SKILL.md remain violations because they would let a shipped
    contract reach outside the scanned tree."""
    return path.is_symlink() and path.is_dir() and (path / "SKILL.md").is_file()


def is_skill_contract_markdown(path: Path) -> bool:
    return path.name == "SKILL.md" or "references" in path.parts


def shell_blocks(path: Path) -> list[ShellBlock]:
    lines = path.read_text(encoding="utf-8").splitlines()
    blocks: list[ShellBlock] = []
    last_nonblank = ""
    index = 0
    while index < len(lines):
        start = FENCE_START.match(lines[index].strip())
        if start is None:
            if lines[index].strip():
                last_nonblank = lines[index].strip()
            index += 1
            continue
        annotation = parse_annotation(last_nonblank)
        language = start.group(1)
        fence_line = index + 1
        content_lines: list[str] = []
        index += 1
        while index < len(lines) and FENCE_END.match(lines[index].strip()) is None:
            content_lines.append(lines[index])
            index += 1
        if index >= len(lines):
            blocks.append(
                ShellBlock(
                    path=path,
                    language=language,
                    line=fence_line,
                    content="\n".join(content_lines),
                    annotation=annotation,
                )
            )
            break
        blocks.append(
            ShellBlock(
                path=path,
                language=language,
                line=fence_line,
                content="\n".join(content_lines) + "\n",
                annotation=annotation,
            )
        )
        last_nonblank = ""
        index += 1
    return blocks


def parse_annotation(line: str) -> Annotation | None:
    match = ANNOTATION.match(line)
    if match is None:
        return None
    kind, field, detail = match.groups()
    if kind == "skip" and field != "reason":
        return None
    if kind == "destructive" and field != "recovery":
        return None
    return Annotation(kind=kind, detail=detail)


def verify_block(block: ShellBlock) -> list[Violation]:
    violations: list[Violation] = []
    if block.annotation is None or block.annotation.kind != "skip":
        syntax_error = check_syntax(block)
        if syntax_error is not None:
            violations.append(syntax_error)
    for offset, raw_line in shell_policy_lines(block.content):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        source_line = block.line + offset
        if FABRICATED_DEFAULT.search(line) or PARAMETER_ZERO_DEFAULT.search(line):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "fabricated-default",
                    "measurement failure must not be converted to a plausible zero",
                )
            )
            continue
        if FAILURE_SWALLOW.search(line):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "failure-swallow",
                    "shell failure must be classified instead of ignored with || true or || :",
                )
            )
        if UNQUOTED_BISECT_COMMAND.search(line):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "command-expansion",
                    "git bisect run requires a checked executable wrapper, not an unquoted command variable",
                )
            )
        if UNQUOTED_COMMAND_LOOP.search(line) or UNQUOTED_VARIABLE_LOOP.search(line):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "word-splitting-loop",
                    "command substitution must not be iterated through unquoted shell word splitting",
                )
            )
        tokens = shell_tokens(line)
        if DYNAMIC_SHELL.search(line) or has_dynamic_shell_tokens(tokens):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "dynamic-shell",
                    "dynamic shell evaluation must not appear in executable skill fences",
                )
            )
        destructive = DESTRUCTIVE_GIT.search(line) is not None or has_destructive_tokens(tokens)
        if destructive and (
            block.annotation is None or block.annotation.kind != "destructive"
        ):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "destructive-unannotated",
                    "destructive Git examples require a skill-shell destructive recovery annotation",
                )
            )
        elif destructive and not RECOVERY_EVIDENCE.search(block.annotation.detail):
            violations.append(
                Violation(
                    block.path,
                    source_line,
                    "destructive-recovery-weak",
                    "destructive recovery annotation must name verifiable rollback or approval evidence",
                )
            )
    return violations


def shell_policy_lines(content: str) -> list[tuple[int, str]]:
    logical: list[tuple[int, str]] = []
    start = 1
    current = ""
    for offset, raw_line in enumerate(content.splitlines(), start=1):
        if not current:
            start = offset
        stripped = raw_line.rstrip()
        if stripped.endswith("\\"):
            current += stripped[:-1]
            continue
        current += raw_line
        if not shell_quotes_closed(current):
            current += "\n"
            continue
        logical.append((start, current))
        current = ""
    if current:
        logical.append((start, current))
    return logical


def shell_quotes_closed(content: str) -> bool:
    quote = ""
    escaped = False
    for character in content:
        if escaped:
            escaped = False
            continue
        if quote == "'":
            if character == "'":
                quote = ""
            continue
        if character == "\\":
            escaped = True
            continue
        if quote == '"':
            if character == '"':
                quote = ""
            continue
        if character in {"'", '"'}:
            quote = character
    return quote == ""


def shell_tokens(line: str) -> list[str]:
    try:
        lexer = shlex.shlex(line, posix=True, punctuation_chars=";&|()")
        lexer.whitespace_split = True
        lexer.commenters = "#"
        return list(lexer)
    except ValueError:
        return []


def has_dynamic_shell_tokens(tokens: Sequence[str]) -> bool:
    command_position = True
    for index, token in enumerate(tokens):
        if token in {";", "&&", "||", "|", "&", "("}:
            command_position = True
            continue
        if token == ")":
            command_position = False
            continue
        if command_position and "=" in token and not token.startswith("="):
            continue
        if command_position and ("$" in token or "`" in token):
            return True
        if token in {"eval", "source"}:
            return True
        if os.path.basename(token) in SHELL_LAUNCHERS and any(
            shell_command_option(option) for option in tokens[index + 1 :]
        ):
            return True
        command_position = False
    return False


def shell_command_option(token: str) -> bool:
    if token == "--command" or token.startswith("--command="):
        return True
    return token.startswith("-") and not token.startswith("--") and "c" in token[1:]


def has_destructive_tokens(tokens: Sequence[str]) -> bool:
    for index, token in enumerate(tokens):
        if token == "rm":
            option_letters = "".join(
                option.lstrip("-")
                for option in tokens[index + 1 :]
                if option.startswith("-")
            )
            if "r" in option_letters and "f" in option_letters:
                return True
        if token != "git":
            continue
        cursor = index + 1
        while cursor < len(tokens) and tokens[cursor].startswith("-"):
            option = tokens[cursor]
            cursor += 1
            if option in {"-C", "--git-dir", "--work-tree", "--namespace", "-c"} and cursor < len(tokens):
                cursor += 1
        if cursor >= len(tokens):
            continue
        command = tokens[cursor]
        arguments = tokens[cursor + 1 :]
        if command == "reset" and "--hard" in arguments:
            return True
        if command == "clean" and any(
            option == "--force" or option.startswith("-") and "f" in option
            for option in arguments
        ):
            return True
        if command == "branch" and "-D" in arguments:
            return True
        if command == "rebase":
            return True
        if command == "bisect" and any(
            action in arguments for action in {"start", "reset", "skip", "bad", "good"}
        ):
            return True
        if command == "push" and any(
            option == "--force" or option.startswith("--force=") or option == "-f"
            for option in arguments
        ):
            return True
    return False


def check_syntax(block: ShellBlock) -> Violation | None:
    if block.language == "console":
        return None
    shell_name = {"bash": "bash", "shell": "bash", "sh": "sh", "zsh": "zsh"}[block.language]
    executable = shutil.which(shell_name)
    if executable is None:
        return Violation(
            block.path,
            block.line,
            "shell-unavailable",
            f"{shell_name} is required to syntax-check this fence",
        )
    result = subprocess.run(
        [executable, "-n"],
        input=DOCUMENTED_PLACEHOLDER.sub("PLACEHOLDER", block.content),
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode == 0:
        return None
    detail = result.stderr.strip().splitlines()
    message = detail[-1] if detail else f"{shell_name} syntax check failed"
    return Violation(block.path, block.line, "syntax", message)


def main(argv: Sequence[str]) -> int:
    if len(argv) == 1 and argv[0] in {"-h", "--help"}:
        print(USAGE)
        return 0
    paths = [Path(value) for value in argv] if argv else [Path("skills")]
    invalid_paths = [path for path in paths if not path.exists()]
    if invalid_paths:
        for path in invalid_paths:
            print(f"{path}: input path does not exist", file=sys.stderr)
        return 2
    for link in external_skill_links(paths):
        print(f"{link}: skipped external skill link (not part of the shipped tree)", file=sys.stderr)
    symlinks = symlink_violations(paths)
    if symlinks:
        for violation in symlinks:
            print(
                f"{violation.path}:{violation.line}: "
                f"{violation.code}: {violation.message}",
                file=sys.stderr,
            )
        return 2
    discovered = markdown_paths(paths)
    if not discovered:
        print("no skill contract markdown files found", file=sys.stderr)
        return 2
    violations = verify_paths(discovered)
    for violation in violations:
        print(
            f"{violation.path}:{violation.line}: "
            f"{violation.code}: {violation.message}",
            file=sys.stderr,
        )
    if violations:
        print(f"skill shell verification failed: {len(violations)} violation(s)", file=sys.stderr)
        return 1
    print(f"skill shell verification passed: {len(discovered)} file(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
