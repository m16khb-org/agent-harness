#!/usr/bin/env python3
"""Append this run's evidence-backed refutations to the repo's team memory.

Usage:
  record_refuted.py --result DIR/workflow-result.json --context DIR/context.json [--repo-dir .]

Writes one JSON line per refuted candidate to <repo>/.issueops/pr-review/refuted.jsonl (committed with the
repo; the review/ directory itself is gitignored). mr_context.py loads this file and workflow.js's prescreen
drops a new candidate on the same file whose title/what overlap ≥ 0.5 — except security/data, never suppressed.
Only refutations a skeptic proved with evidence (confidence ≥ 80) are recorded; prescreen kills are not.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--result", required=True)
    ap.add_argument("--context", required=True)
    ap.add_argument("--repo-dir", default=None)
    a = ap.parse_args()
    result = json.loads(Path(a.result).read_text())
    ctx = json.loads(Path(a.context).read_text())
    repo = Path(a.repo_dir or ctx["repo_dir"])
    target = repo / ".issueops" / "pr-review" / "refuted.jsonl"
    target.parent.mkdir(parents=True, exist_ok=True)
    existing = set()
    if target.exists():
        for line in target.read_text(errors="ignore").splitlines():
            try:
                d = json.loads(line)
                existing.add((d.get("path"), d.get("title")))
            except json.JSONDecodeError:
                continue
    added = 0
    with target.open("a") as fh:
        for r in result.get("refuted_for_history", []):
            key = (r.get("path"), r.get("title"))
            if key in existing:
                continue
            fh.write(json.dumps({**r, "mr": f"{ctx['provider']}-{ctx['iid']}", "head_sha": ctx["diff_refs"]["head_sha"]}, ensure_ascii=False) + "\n")
            existing.add(key)
            added += 1
    print(json.dumps({"file": str(target), "added": added, "total": len(existing)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
