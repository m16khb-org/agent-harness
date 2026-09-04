#!/usr/bin/env python3
"""Pre-start API documentation gate for atomic-commit-push.

Runs deterministic API doc checks and the agent-backed harness API doc reviewer
on the target repo. The gate skips cleanly when no staged API candidate files
exist and exits non-zero on static omissions or blocking Swagger/OpenAPI drift.
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path


def find_harness_binary() -> str:
    env_bin = os.environ.get("ISSUEOPS_BIN")
    if env_bin:
        return env_bin

    script = Path(__file__).resolve()
    # skills/atomic-commit-push/scripts/api_doc_gate.py -> repo root
    repo_root = script.parents[3]
    for name in ("issueops", "issueops"):
        local_bin = repo_root / "bin" / name
        if local_bin.exists():
            return str(local_bin)

    for name in ("issueops", "issueops"):
        found = shutil.which(name)
        if found:
            return found
    return "issueops"


def main(argv: list[str]) -> int:
    repo = argv[1] if len(argv) > 1 else "."
    cmd = [find_harness_binary(), "api-doc", "check", "--repo", repo, "--json"]
    proc = subprocess.run(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)

    if proc.stdout:
        print(proc.stdout, end="")
    if proc.stderr:
        print(proc.stderr, file=sys.stderr, end="")

    if proc.returncode != 0:
        return proc.returncode

    try:
        payload = json.loads(proc.stdout or "{}")
    except json.JSONDecodeError:
        return 0

    return 0 if payload.get("ok", False) else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
