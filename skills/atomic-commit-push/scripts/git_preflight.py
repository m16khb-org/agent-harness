#!/usr/bin/env python3
"""Read-only git preflight for the atomic-commit-push skill."""
from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

SECRET_PATH_RE = re.compile(
    r"(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)",
    re.IGNORECASE,
)
CONVENTIONAL_SUBJECT_RE = re.compile(
    r"^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+"
)


def run(repo: Path, args: list[str]) -> tuple[int, str, str]:
    proc = subprocess.run(
        ["git", *args],
        cwd=repo,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    return proc.returncode, proc.stdout.strip(), proc.stderr.strip()


def git(repo: Path, args: list[str], default: str = "") -> str:
    code, out, _ = run(repo, args)
    return out if code == 0 else default


def redact_remote(url: str) -> str:
    # Remove userinfo from https://user:token@host/path and similar forms.
    url = re.sub(r"(https?://)[^/@]+@", r"\1<redacted>@", url)
    url = re.sub(r"(://)([^:/@]+):([^/@]+)@", r"\1<redacted>:<redacted>@", url)
    return url


def parse_status(lines: list[str]) -> dict[str, Any]:
    unstaged: list[str] = []
    staged: list[str] = []
    untracked: list[str] = []
    secret_like: list[str] = []

    for line in lines:
        if not line or line.startswith("## "):
            continue
        status = line[:2]
        path = line[3:] if len(line) > 3 else ""
        if " -> " in path:
            path = path.split(" -> ", 1)[1]
        if status == "??":
            untracked.append(path)
        else:
            if status[0] != " ":
                staged.append(path)
            if status[1] != " ":
                unstaged.append(path)
        if SECRET_PATH_RE.search(path):
            secret_like.append(path)

    return {
        "staged_files": sorted(set(staged)),
        "unstaged_files": sorted(set(unstaged)),
        "untracked_files": sorted(set(untracked)),
        "secret_like_paths": sorted(set(secret_like)),
    }


def recent_commits(repo: Path, limit: int = 5) -> list[dict[str, str]]:
    out = git(repo, ["log", f"-{limit}", "--pretty=format:%h%x09%s"])
    commits: list[dict[str, str]] = []
    for line in out.splitlines():
        if not line:
            continue
        parts = line.split("\t", 1)
        if len(parts) == 2:
            commits.append({"sha": parts[0], "subject": parts[1]})
    return commits


def commit_style_hints(repo: Path, limit: int = 10) -> dict[str, Any]:
    subjects = [item["subject"] for item in recent_commits(repo, limit)]
    bodies = git(repo, ["log", f"-{limit}", "--pretty=format:%B%x1e"]).split("\x1e")
    conventional_count = sum(1 for subject in subjects if CONVENTIONAL_SUBJECT_RE.match(subject))
    lore_count = sum(1 for body in bodies if re.search(r"(?m)^(Lore:|Lore-[A-Za-z]+:)", body))
    return {
        "recent_count": len(subjects),
        "conventional_subjects": conventional_count,
        "lore_bodies": lore_count,
        "recommended": "conventional_subject_plus_lore_body",
    }


def main() -> int:
    repo_arg = Path(sys.argv[1]) if len(sys.argv) > 1 else Path.cwd()
    repo_arg = repo_arg.expanduser().resolve()

    code, root, err = run(repo_arg, ["rev-parse", "--show-toplevel"])
    if code != 0:
        print(json.dumps({"ok": False, "error": "not_git_repo", "path": str(repo_arg), "detail": err}, ensure_ascii=False, indent=2))
        return 1

    root_path = Path(root)
    branch = git(root_path, ["branch", "--show-current"])
    head = git(root_path, ["rev-parse", "--short", "HEAD"])
    upstream = git(root_path, ["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"])
    status_out = git(root_path, ["status", "--porcelain=v1", "--branch"])
    status_lines = status_out.splitlines() if status_out else []
    parsed = parse_status(status_lines)

    remotes = []
    for line in git(root_path, ["remote", "-v"]).splitlines():
        parts = line.split()
        if len(parts) >= 3 and parts[2] == "(fetch)":
            remotes.append({"name": parts[0], "url": redact_remote(parts[1])})

    ahead = behind = None
    if upstream:
        counts = git(root_path, ["rev-list", "--left-right", "--count", f"{upstream}...HEAD"])
        if counts:
            left, right = counts.split()[:2]
            behind, ahead = int(left), int(right)

    result: dict[str, Any] = {
        "ok": True,
        "repo_root": str(root_path),
        "branch": branch,
        "head": head,
        "upstream": upstream or None,
        "ahead": ahead,
        "behind": behind,
        "is_clean": not any(parsed[key] for key in ("staged_files", "unstaged_files", "untracked_files")),
        "status_lines": status_lines,
        "remotes": remotes,
        "last_commit": git(root_path, ["log", "-1", "--pretty=format:%h %s"]),
        "recent_commits": recent_commits(root_path),
        "commit_style_hints": commit_style_hints(root_path),
        **parsed,
    }

    warnings = []
    if not branch:
        warnings.append("detached_head")
    if not upstream:
        warnings.append("no_upstream")
    if parsed["secret_like_paths"]:
        warnings.append("secret_like_paths_present")
    result["warnings"] = warnings

    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
