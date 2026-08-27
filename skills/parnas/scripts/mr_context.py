#!/usr/bin/env python3
"""Build a deterministic review context pack for one GitLab MR or GitHub PR.

Usage:
  mr_context.py --mr <number|!n|#n|URL|source-branch> [--repo-dir .] [--out DIR]
                [--provider gitlab|github] [--project owner/repo] [--history 5]
                [--worktree] [--no-fetch]

Provider is auto-detected from the `origin` remote host (github.com → `gh`,
anything else → `glab`). Both CLIs must already be authenticated.

Outputs (inside --out, default: <repo>/.agent-harness/issues/<issue>/review/<provider>-<n>/ when the
MR/PR names its issue (branch prefix `<issue>-…` or `#<issue>` in title/description), otherwise
<repo>/.agent-harness/tmp/parnas/mr-<n>/ — both paths are gitignored working areas):
  context.json   machine-readable pack (meta, diff_refs, files+hunks, rule pack,
                 existing threads, prior review lessons, verification hints, worktree)
  diff.patch     cumulative diff (base...head), unified
  summary.md     compact agent-readable summary
  worktree/      (with --worktree) detached checkout of head_sha; node_modules symlinked

Never prints tokens.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import urllib.parse
from pathlib import Path

RULE_CANDIDATES = [
    "CLAUDE.md",
    "AGENTS.md",
    ".agent-harness/CONSTITUTION.md",
    ".agent-harness/CONVENTIONS.md",
    ".agent-harness/CAUTIONS.md",
    ".agent-harness/ARCHITECTURE.md",
    ".agent-harness/TESTING.md",
    ".agent-harness/OPEN_API_SPEC.md",
    "agent-docs/constitution.md",
    "agent-docs/constitution-compact.md",
]
# 이 스킬이 게시한 리뷰 마커.
REVIEW_MARKER_RE = re.compile(r"<!--\s*parnas-review\s+head=([0-9a-f]{7,40})")
REVIEW_MARKER_TOKENS = ("parnas-review", "parnas-finding")
BOT_MARKERS = ("kody-codereview", "kody-pr-summary", "coderabbit", "copilot-pull-request", "gemini-code-assist")
HUNK_RE = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")


# ----------------------------------------------------------------------------- helpers

CALL_TIMEOUT = int(os.environ.get("PARNAS_CALL_TIMEOUT", "90"))


def run(cmd: list[str], cwd: str | None = None, check: bool = True) -> str:
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=CALL_TIMEOUT)
    except subprocess.TimeoutExpired:
        raise SystemExit(f"command timed out after {CALL_TIMEOUT}s: {' '.join(cmd[:5])}... (remote slow? set PARNAS_CALL_TIMEOUT)")
    if check and p.returncode != 0:
        raise SystemExit(f"command failed ({p.returncode}): {' '.join(cmd[:5])}...\n{p.stderr.strip()[:600]}")
    return p.stdout


def ok(cmd: list[str], cwd: str | None = None) -> bool:
    return subprocess.run(cmd, cwd=cwd, capture_output=True).returncode == 0


def merge_paginated(out: str) -> list:
    merged: list = []
    dec = json.JSONDecoder()
    s, i = out.strip(), 0
    while i < len(s):
        obj, i = dec.raw_decode(s, i)
        merged.extend(obj if isinstance(obj, list) else [obj])
        while i < len(s) and s[i] in " \n\r\t,":
            i += 1
    return merged


def strip_md(body: str, n: int = 400) -> str:
    return re.sub(r"\s+", " ", re.sub(r"!\[[^\]]*\]\([^)]*\)", "", body or ""))[:n]


def parse_hunks(diff_text: str) -> dict:
    new_lines: set[int] = set()
    added_lines: set[int] = set()
    hunks = []
    added = removed = 0
    cur_new = None
    for line in diff_text.splitlines():
        m = HUNK_RE.match(line)
        if m:
            hunks.append({"old_start": int(m.group(1)), "old_len": int(m.group(2) or 1), "new_start": int(m.group(3)), "new_len": int(m.group(4) or 1)})
            cur_new = int(m.group(3))
            continue
        if cur_new is None:
            continue
        if line.startswith("+"):
            new_lines.add(cur_new); added_lines.add(cur_new); cur_new += 1; added += 1
        elif line.startswith("-"):
            removed += 1
        elif line.startswith("\\"):
            continue
        else:
            new_lines.add(cur_new); cur_new += 1
    return {"new_lines": sorted(new_lines), "added_lines": sorted(added_lines), "hunks": hunks, "added": added, "removed": removed}


def classify(path: str) -> list[str]:
    tags = []
    p = path.lower()
    if re.search(r"\.(spec|test)\.[tj]sx?$", p) or "/test/" in p or "/__tests__/" in p or p.endswith("_test.go"):
        tags.append("test")
    if "/migration/" in p or "/migrations/" in p:
        tags.append("migration")
    if p.endswith("dto.ts") or "/dtos/" in p or "/dto/" in p:
        tags.append("dto")
    if p.endswith("controller.ts") or "/handler" in p or "/routes" in p:
        tags.append("controller")
    if "gateway" in p:
        tags.append("gateway")
    if "consumer" in p or "scheduler" in p or "kafka" in p or "worker" in p:
        tags.append("async")
    if p.endswith(("entity.ts", "repository.ts")) or "typeorm" in p or "/sql/" in p:
        tags.append("db")
    if re.search(r"(auth|guard|token|secret|credential|permission)", p):
        tags.append("security")
    if re.search(r"(\.gitlab-ci\.yml|^\.github/|^deploy/|^scripts/|dockerfile|\.ya?ml$|\.sh$)", p):
        tags.append("ops")
    if re.search(r"(idl/|generated|\.pb\.|_pb\.|\.pb\.go$)", p):
        tags.append("generated")
    if p.endswith((".md", ".txt")):
        tags.append("docs")
    return tags or ["source"]


def detect_remote(repo_dir: str) -> tuple[str, str]:
    url = run(["git", "remote", "get-url", "origin"], cwd=repo_dir).strip()
    m = re.match(r"(?:https?://|ssh://git@|git@)([^/:]+)(?::\d+)?[:/](.+?)(?:\.git)?/?$", url)
    if not m:
        raise SystemExit(f"cannot parse origin remote: {url}")
    return m.group(1), m.group(2)


# ----------------------------------------------------------------------------- providers

class GitLab:
    name = "gitlab"

    def __init__(self, hostname: str, project: str, repo_dir: str):
        self.host, self.project, self.repo_dir = hostname, project, repo_dir
        self.enc = urllib.parse.quote(project, safe="")

    def api(self, path: str, paginate: bool = False):
        cmd = ["glab", "api", path, "--hostname", self.host] + (["--paginate"] if paginate else [])
        out = run(cmd, cwd=self.repo_dir)
        return merge_paginated(out) if paginate else json.loads(out)

    def resolve(self, ref: str) -> int:
        m = re.search(r"/merge_requests/(\d+)", ref)
        if m:
            return int(m.group(1))
        if re.fullmatch(r"[!#]?\d+", ref):
            return int(ref.lstrip("!#"))
        q = urllib.parse.quote(ref, safe="")
        found = self.api(f"projects/{self.enc}/merge_requests?source_branch={q}&state=opened&per_page=1")
        if not found:
            older = self.api(f"projects/{self.enc}/merge_requests?source_branch={q}&per_page=3&order_by=updated_at")
            hint = ", ".join(f"!{m['iid']}({m['state']})" for m in older) or "none"
            raise SystemExit(f"no OPEN MR for source branch {ref}; non-open candidates: {hint}. Pass the iid explicitly.")
        return int(found[0]["iid"])

    def meta(self, n: int) -> dict:
        m = self.api(f"projects/{self.enc}/merge_requests/{n}")
        refs = m.get("diff_refs") or {}
        return {
            "number": n, "web_url": m.get("web_url"), "title": m.get("title"), "description": m.get("description") or "",
            "author": (m.get("author") or {}).get("username"), "source_branch": m.get("source_branch"), "target_branch": m.get("target_branch"),
            "labels": m.get("labels", []), "state": m.get("state"), "draft": bool(m.get("draft") or m.get("work_in_progress")),
            "diff_refs": {"base_sha": refs.get("base_sha"), "start_sha": refs.get("start_sha"), "head_sha": refs.get("head_sha") or m.get("sha")},
            "fetch_ref": f"refs/merge-requests/{n}/head",
        }

    def changes(self, n: int) -> list[dict]:
        out = []
        for c in self.api(f"projects/{self.enc}/merge_requests/{n}/diffs?per_page=100", paginate=True):
            out.append({"path": c.get("new_path"), "old_path": c.get("old_path"), "new_file": c.get("new_file"), "deleted_file": c.get("deleted_file"),
                        "renamed_file": c.get("renamed_file"), "generated": bool(c.get("generated_file")), "diff": c.get("diff", "")})
        return out

    def threads(self, n: int) -> list[dict]:
        out = []
        for d in self.api(f"projects/{self.enc}/merge_requests/{n}/discussions?per_page=100", paginate=True):
            notes = [x for x in d.get("notes", []) if not x.get("system")]
            if not notes:
                continue
            first = notes[0]
            pos = first.get("position") or {}
            out.append({"thread_id": d["id"], "author": (first.get("author") or {}).get("username"), "body": first.get("body", ""),
                        "path": pos.get("new_path") or pos.get("old_path"), "new_line": pos.get("new_line"),
                        "resolvable": first.get("resolvable"), "resolved": first.get("resolved"), "note_count": len(notes),
                        "replies": [x.get("body", "") for x in notes[1:3]]})
        return out

    def mrs_for_commit(self, sha: str) -> list[int]:
        return [int(m["iid"]) for m in self.api(f"projects/{self.enc}/repository/commits/{sha}/merge_requests") if m.get("state") == "merged"]

    def issue(self, n: int) -> dict | None:
        try:
            i = self.api(f"projects/{self.enc}/issues/{n}")
        except SystemExit:
            return None
        return {"number": n, "title": i.get("title"), "state": i.get("state"), "labels": i.get("labels", []), "web_url": i.get("web_url"),
                "description": i.get("description") or ""}


class GitHub:
    name = "github"

    def __init__(self, hostname: str, project: str, repo_dir: str):
        self.host, self.project, self.repo_dir = hostname, project, repo_dir
        self.enc = project

    def api(self, path: str, paginate: bool = False):
        cmd = ["gh", "api", path, "--hostname", self.host] + (["--paginate"] if paginate else [])
        out = run(cmd, cwd=self.repo_dir)
        return merge_paginated(out) if paginate else json.loads(out)

    def resolve(self, ref: str) -> int:
        m = re.search(r"/pull/(\d+)", ref)
        if m:
            return int(m.group(1))
        if re.fullmatch(r"[!#]?\d+", ref):
            return int(ref.lstrip("!#"))
        owner = self.project.split("/")[0]
        found = self.api(f"repos/{self.project}/pulls?head={owner}:{urllib.parse.quote(ref, safe='')}&state=open&per_page=1")
        if not found:
            older = self.api(f"repos/{self.project}/pulls?head={owner}:{urllib.parse.quote(ref, safe='')}&state=all&per_page=3")
            hint = ", ".join(f"#{m['number']}({m['state']})" for m in older) or "none"
            raise SystemExit(f"no OPEN PR for head branch {ref}; non-open candidates: {hint}. Pass the number explicitly.")
        return int(found[0]["number"])

    def meta(self, n: int) -> dict:
        m = self.api(f"repos/{self.project}/pulls/{n}")
        state = "opened" if m.get("state") == "open" else ("merged" if m.get("merged_at") else "closed")
        return {
            "number": n, "web_url": m.get("html_url"), "title": m.get("title"), "description": m.get("body") or "",
            "author": (m.get("user") or {}).get("login"), "source_branch": (m.get("head") or {}).get("ref"), "target_branch": (m.get("base") or {}).get("ref"),
            "labels": [l["name"] for l in m.get("labels", [])], "state": state, "draft": bool(m.get("draft")),
            "diff_refs": {"base_sha": (m.get("base") or {}).get("sha"), "start_sha": (m.get("base") or {}).get("sha"), "head_sha": (m.get("head") or {}).get("sha")},
            "fetch_ref": f"refs/pull/{n}/head",
        }

    def changes(self, n: int) -> list[dict]:
        out = []
        for c in self.api(f"repos/{self.project}/pulls/{n}/files?per_page=100", paginate=True):
            status = c.get("status")
            out.append({"path": c.get("filename"), "old_path": c.get("previous_filename") or c.get("filename"), "new_file": status == "added",
                        "deleted_file": status == "removed", "renamed_file": status == "renamed", "generated": False, "diff": c.get("patch") or ""})
        return out

    def threads(self, n: int) -> list[dict]:
        out = []
        by_root: dict[int, list] = {}
        for c in self.api(f"repos/{self.project}/pulls/{n}/comments?per_page=100", paginate=True):
            root = c.get("in_reply_to_id") or c["id"]
            by_root.setdefault(root, []).append(c)
        for root, cs in by_root.items():
            first = cs[0]
            out.append({"thread_id": str(root), "author": (first.get("user") or {}).get("login"), "body": first.get("body", ""),
                        "path": first.get("path"), "new_line": first.get("line") or first.get("original_line"),
                        "resolvable": True, "resolved": None, "note_count": len(cs), "replies": [x.get("body", "") for x in cs[1:3]]})
        for c in self.api(f"repos/{self.project}/issues/{n}/comments?per_page=100", paginate=True):
            out.append({"thread_id": f"issue-{c['id']}", "author": (c.get("user") or {}).get("login"), "body": c.get("body", ""),
                        "path": None, "new_line": None, "resolvable": False, "resolved": None, "note_count": 1, "replies": []})
        for r in self.api(f"repos/{self.project}/pulls/{n}/reviews?per_page=100", paginate=True):
            if r.get("body"):
                out.append({"thread_id": f"review-{r['id']}", "author": (r.get("user") or {}).get("login"), "body": r.get("body", ""),
                            "path": None, "new_line": None, "resolvable": False, "resolved": None, "note_count": 1, "replies": []})
        return out

    def mrs_for_commit(self, sha: str) -> list[int]:
        return [int(p["number"]) for p in self.api(f"repos/{self.project}/commits/{sha}/pulls") if p.get("merged_at")]

    def issue(self, n: int) -> dict | None:
        try:
            i = self.api(f"repos/{self.project}/issues/{n}")
        except SystemExit:
            return None
        if i.get("pull_request"):
            return None
        return {"number": n, "title": i.get("title"), "state": i.get("state"), "labels": [l["name"] for l in i.get("labels", [])],
                "web_url": i.get("html_url"), "description": i.get("body") or ""}


def make_provider(kind: str | None, hostname: str, project: str, repo_dir: str):
    h = hostname.lower()
    if kind == "github" or (kind is None and (h == "github.com" or h.startswith("github."))):
        return GitHub(hostname, project, repo_dir)
    return GitLab(hostname, project, repo_dir)


# ----------------------------------------------------------------------------- context assembly

ISSUE_REF_RE = re.compile(r"(?:(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?|relate[sd]? to|issue)\s+)?#(\d+)", re.I)


def issue_number_hint(meta: dict, this_n: int) -> int | None:
    """Issue number the MR/PR names locally (branch prefix first, then #N refs) — no network."""
    bm = re.search(r"(?:^|/)(\d{2,7})-", meta.get("source_branch") or "")
    if bm:
        return int(bm.group(1))
    for m in ISSUE_REF_RE.finditer((meta.get("description") or "") + "\n" + (meta.get("title") or "")):
        if int(m.group(1)) != this_n:
            return int(m.group(1))
    return None


def default_out_dir(repo_dir: str, provider: str, meta: dict, n: int) -> Path:
    """Per-issue artifact folder (#480) when the issue is known; legacy tmp path otherwise."""
    issue = issue_number_hint(meta, n)
    if issue is not None:
        return Path(repo_dir) / ".agent-harness" / "issues" / str(issue) / "review" / f"{provider}-{n}"
    return Path(repo_dir) / ".agent-harness" / "tmp" / "parnas" / f"mr-{n}"


def linked_issues(p, meta: dict, this_n: int, limit: int = 3) -> list[dict]:
    """Issues referenced by the description/title (Closes #N, #N) and the branch prefix (`2795-...`, `fix/2795-...`)."""
    nums: list[int] = []
    for m in ISSUE_REF_RE.finditer((meta.get("description") or "") + "\n" + (meta.get("title") or "")):
        n = int(m.group(1))
        if n != this_n and n not in nums:
            nums.append(n)
    bm = re.search(r"(?:^|/)(\d{2,7})-", meta.get("source_branch") or "")
    if bm and int(bm.group(1)) not in nums:
        nums.insert(0, int(bm.group(1)))
    issues = []
    for n in nums[:limit]:
        i = p.issue(n)
        if i:
            checklist = re.findall(r"^\s*[-*]\s*\[( |x|X)\]\s*(.+)$", i["description"], re.M)
            i["checklist"] = [{"done": c[0].lower() == "x", "text": c[1].strip()} for c in checklist]
            issues.append(i)
    return issues


def prior_review_lessons(p, repo_dir: str, files: list[str], this_n: int, limit: int, target_branch: str | None) -> list:
    if limit <= 0 or not files:
        return []
    ref = f"origin/{target_branch}" if target_branch and ok(["git", "rev-parse", "--verify", "-q", f"origin/{target_branch}"], repo_dir) else "HEAD"
    log = run(["git", "log", "-n", str(limit * 3), "--format=%H", ref, "--"] + files, cwd=repo_dir, check=False)
    nums: list[int] = []
    for sha in log.split():
        try:
            for n in p.mrs_for_commit(sha):
                if n != this_n and n not in nums:
                    nums.append(n)
        except SystemExit:
            continue
        if len(nums) >= limit:
            break
    lessons, fileset = [], set(files)
    for n in nums[:limit]:
        try:
            ths = p.threads(n)
        except SystemExit:
            continue
        for t in ths:
            if not t["path"]:
                continue
            lessons.append({"mr": n, "path": t["path"], "new_line": t["new_line"], "author": t["author"], "same_file": t["path"] in fileset,
                            "is_bot": any(m in t["body"] for m in BOT_MARKERS), "finding": strip_md(t["body"], 500),
                            "replies": [strip_md(r, 500) for r in t["replies"]], "resolved": t["resolved"]})
    lessons.sort(key=lambda l: (not l["same_file"], l["mr"]))
    return lessons[: max(limit * 4, 8)]


def detect_verification(repo_dir: str) -> dict:
    info: dict = {"package_manager": None, "scripts": {}, "toolchains": [], "codegraph": (Path(repo_dir) / ".codegraph").exists()}
    pkg = Path(repo_dir) / "package.json"
    if pkg.exists():
        try:
            scripts = json.loads(pkg.read_text()).get("scripts", {})
        except json.JSONDecodeError:
            scripts = {}
        info["scripts"] = {k: scripts[k][:120] for k in scripts if re.search(r"^(lint|lint:check|typecheck|test|test:unit|swagger:check|format:check|serialization:check|build)$", k)}
        for lock, pm in (("pnpm-lock.yaml", "pnpm"), ("yarn.lock", "yarn"), ("package-lock.json", "npm"), ("bun.lockb", "bun")):
            if (Path(repo_dir) / lock).exists():
                info["package_manager"] = pm
                break
        info["toolchains"].append("node")
    for f, tool in (("go.mod", "go"), ("Cargo.toml", "cargo"), ("pyproject.toml", "python"), ("pom.xml", "maven"), ("build.gradle", "gradle"), ("build.gradle.kts", "gradle")):
        if (Path(repo_dir) / f).exists():
            info["toolchains"].append(tool)
    return info


def rule_pack(repo_dir: str) -> list[dict]:
    out = []
    for rel in RULE_CANDIDATES:
        p = Path(repo_dir) / rel
        if p.exists():
            out.append({"path": rel, "bytes": p.stat().st_size, "kind": "instructions"})
    for rules_dir, kind in ((".kody/rules", "kody-rule"), (".coderabbit", "coderabbit"), (".github/instructions", "copilot"), (".cursor/rules", "cursor")):
        d = Path(repo_dir) / rules_dir
        if d.is_dir():
            for p in sorted(list(d.glob("*.md")) + list(d.glob("*.mdc")) + list(d.glob("*.yaml"))):
                txt = p.read_text(errors="ignore")
                title = re.search(r'^title:\s*"?(.+?)"?\s*$', txt, re.M)
                paths = re.search(r"^path:\s*(\[.*\])", txt, re.M)
                sev = re.search(r'^severity_min:\s*"?(\w+)', txt, re.M)
                out.append({"path": str(p.relative_to(repo_dir)), "bytes": p.stat().st_size, "kind": kind, "title": title.group(1) if title else p.stem,
                            "globs": paths.group(1) if paths else None, "severity_min": sev.group(1) if sev else None})
    return out


LENS_TABLE_RE = re.compile(r"^\| `([a-z]+)` \| (.+?) \| (.+?) \|$", re.M)


def load_lenses() -> dict[str, dict]:
    md = Path(__file__).resolve().parent.parent / "references" / "lenses.md"
    out = {}
    for m in LENS_TABLE_RE.finditer(md.read_text(errors="ignore")):
        out[m.group(1)] = {"applies": m.group(2).strip(), "text": m.group(3).strip()}
    return out


def select_lenses(files: list[dict], patch: str, issues: list, lessons: list) -> list[str]:
    tags = {t for f in files for t in f["tags"]}
    low = patch.lower()
    chosen = ["logic", "boundary", "tests", "rules", "scope"]
    if tags & {"security", "controller", "gateway", "dto", "ops"} or re.search(r"auth|token|secret|permission|process\.env", low):
        chosen.append("security")
    if tags & {"db", "migration"} or re.search(r"repository|createquerybuilder|\.query\(|transaction", low):
        chosen.append("data")
    if tags & {"async", "gateway"} or re.search(r"kafka|queue|stream|websocket|retry|timeout|debounce|job", low):
        chosen.append("async")
    if tags & {"dto", "controller", "gateway", "generated"}:
        chosen.append("contract")
    if issues:
        chosen.append("intent")
    return chosen


def make_worktree(repo_dir: str, out_dir: Path, head_sha: str) -> str:
    wt = out_dir / "worktree"
    if wt.exists():
        if run(["git", "rev-parse", "HEAD"], cwd=str(wt), check=False).strip() == head_sha:
            return str(wt)
        run(["git", "worktree", "remove", "--force", str(wt)], cwd=repo_dir, check=False)
    run(["git", "worktree", "add", "--detach", str(wt), head_sha], cwd=repo_dir)
    for dep in ("node_modules", ".venv", "vendor"):
        src, dst = Path(repo_dir) / dep, wt / dep
        if src.exists() and not dst.exists():
            os.symlink(src, dst)
    return str(wt)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--mr", required=True)
    ap.add_argument("--repo-dir", default=".")
    ap.add_argument("--provider", choices=["gitlab", "github"])
    ap.add_argument("--project", help="owner/repo or group/sub/project (default: from origin)")
    ap.add_argument("--out")
    ap.add_argument("--history", type=int, default=5)
    ap.add_argument("--worktree", action="store_true")
    ap.add_argument("--no-fetch", action="store_true")
    a = ap.parse_args()

    repo_dir = str(Path(a.repo_dir).resolve())
    hostname, project = detect_remote(repo_dir)
    if a.project:
        project = a.project
    p = make_provider(a.provider, hostname, project, repo_dir)
    n = p.resolve(a.mr.strip())
    meta = p.meta(n)
    refs = meta["diff_refs"]
    head_sha = refs["head_sha"]

    out_dir = Path(a.out) if a.out else default_out_dir(repo_dir, p.name, meta, n)
    out_dir.mkdir(parents=True, exist_ok=True)

    if not a.no_fetch:
        run(["git", "fetch", "--quiet", "origin", meta["fetch_ref"]], cwd=repo_dir, check=False)
        if refs.get("base_sha"):
            run(["git", "fetch", "--quiet", "origin", refs["base_sha"]], cwd=repo_dir, check=False)
    have_head = bool(head_sha) and ok(["git", "cat-file", "-e", f"{head_sha}^{{commit}}"], repo_dir)

    files, patch_parts = [], []
    for c in p.changes(n):
        h = parse_hunks(c["diff"])
        entry = {k: c[k] for k in ("path", "old_path", "new_file", "deleted_file", "renamed_file", "generated")}
        entry.update({"tags": classify(c["path"]), "added": h["added"], "removed": h["removed"], "hunks": h["hunks"],
                      "commentable_new_lines": h["new_lines"], "added_lines": h["added_lines"]})
        files.append(entry)
        patch_parts.append(f"diff --git a/{c['old_path']} b/{c['path']}\n--- a/{c['old_path']}\n+++ b/{c['path']}\n{c['diff']}")
    (out_dir / "diff.patch").write_text("\n".join(patch_parts) + "\n")

    raw_threads = p.threads(n)
    review_heads = sorted({m.group(1) for t in raw_threads for m in REVIEW_MARKER_RE.finditer(t["body"])})
    threads = [{"thread_id": t["thread_id"], "author": t["author"], "is_bot": any(m in t["body"] for m in BOT_MARKERS),
                "is_own_review": any(tok in t["body"] for tok in REVIEW_MARKER_TOKENS), "path": t["path"], "new_line": t["new_line"],
                "resolvable": t["resolvable"], "resolved": t["resolved"], "note_count": t["note_count"], "excerpt": strip_md(t["body"])} for t in raw_threads]
    already = head_sha in review_heads

    issues = linked_issues(p, meta, n)
    src_files = [f["path"] for f in files if not ({"test", "docs"} & set(f["tags"]))]
    lessons = prior_review_lessons(p, repo_dir, src_files, n, a.history, meta["target_branch"])
    worktree = make_worktree(repo_dir, out_dir, head_sha) if (a.worktree and have_head) else None

    reasons = []
    if meta["state"] != "opened":
        reasons.append(f"state={meta['state']}")
    if meta["draft"]:
        reasons.append("draft")
    if already:
        reasons.append("a review by this skill is already posted for this head_sha")
    eligibility = {"state": meta["state"], "draft": meta["draft"], "already_reviewed_head": already, "eligible": not reasons, "reasons": reasons}

    ctx = {
        "provider": p.name, "hostname": hostname, "project": project, "project_encoded": p.enc, "iid": n, "number": n,
        "web_url": meta["web_url"], "title": meta["title"], "description": meta["description"], "author": meta["author"],
        "source_branch": meta["source_branch"], "target_branch": meta["target_branch"], "labels": meta["labels"],
        "diff_refs": refs, "head_available_locally": have_head, "eligibility": eligibility, "files": files,
        "totals": {"files": len(files), "added": sum(f["added"] for f in files), "removed": sum(f["removed"] for f in files)},
        "linked_issues": issues, "rule_pack": rule_pack(repo_dir), "existing_threads": threads, "prior_review_lessons": lessons,
        "verification": detect_verification(repo_dir), "worktree": worktree, "repo_dir": repo_dir, "out_dir": str(out_dir),
    }
    scale = {"files": len(files), "added": ctx["totals"]["added"], "large": len(files) > 40 or ctx["totals"]["added"] > 2000}
    ctx["scale"] = scale
    ctx["prior_review_threads"] = [t for t in threads if t["is_own_review"]]
    (out_dir / "context.json").write_text(json.dumps(ctx, ensure_ascii=False, indent=1))
    lenses = load_lenses()
    chosen = [l for l in select_lenses(files, "\n".join(patch_parts), issues, lessons) if l in lenses]
    wf_args = {"outDir": str(out_dir), "checkout": worktree or repo_dir, "codegraph": ctx["verification"]["codegraph"],
               "lenses": chosen, "lensText": {l: lenses[l]["text"] for l in chosen},
               "maxCandidates": 12 if scale["large"] else 24, "perLensCap": 6}
    (out_dir / "workflow_args.json").write_text(json.dumps(wf_args, ensure_ascii=False, indent=1))

    L = [f"# {p.name} {'!' if p.name == 'gitlab' else '#'}{n} — {ctx['title']}", ctx["web_url"] or "",
         f"author={ctx['author']} source={ctx['source_branch']} → target={ctx['target_branch']} labels={','.join(ctx['labels']) or '-'}",
         f"head={head_sha} base={refs.get('base_sha')} start={refs.get('start_sha')} head_local={have_head} worktree={worktree or '-'}",
         f"eligible={eligibility['eligible']} reasons={reasons or '-'}",
         f"scale: files={scale['files']} added={scale['added']} large={scale['large']} → lenses={','.join(chosen)} (workflow_args.json)",
         f"prior threads by this skill at other heads: {len(ctx['prior_review_threads'])}", "", "## Description", ctx["description"][:3000] or "(empty)", "",
         f"## Files ({ctx['totals']['files']}, +{ctx['totals']['added']}/-{ctx['totals']['removed']})"]
    for f in files:
        hs = ",".join(f"{h['new_start']}-{h['new_start'] + max(h['new_len'], 1) - 1}" for h in f["hunks"])
        L.append(f"- {f['path']} [{'/'.join(f['tags'])}] +{f['added']}/-{f['removed']} hunks(new): {hs}")
    L += ["", f"## Linked issues ({len(issues)})"]
    for i in issues:
        L += [f"### #{i['number']} {i['title']} [{i['state']}] labels={','.join(i['labels']) or '-'}", i["web_url"] or "", "",
              i["description"][:6000] or "(empty)", ""]
        if i["checklist"]:
            L += ["체크리스트:"] + [f"- [{'x' if c['done'] else ' '}] {c['text']}" for c in i["checklist"]] + [""]
    L += ["", "## Rule pack"]
    for r in ctx["rule_pack"]:
        extra = f" — {r['title']} (globs {r['globs']}, min {r['severity_min']})" if r["kind"] != "instructions" else ""
        L.append(f"- {r['path']} ({r['bytes']}B){extra}")
    L += ["", f"## Existing threads ({len(threads)})"]
    for t in threads:
        who = "BOT" if t["is_bot"] else ("OURS" if t["is_own_review"] else t["author"])
        L.append(f"- [{who}] {t['path'] or '(general)'}:{t['new_line'] or '-'} resolved={t['resolved']} notes={t['note_count']} :: {t['excerpt'][:200]}")
    L += ["", f"## Prior review lessons from merged changes to the same files ({len(lessons)})"]
    for l in lessons:
        L.append(f"- {l['mr']} {l['path']}:{l['new_line'] or '-'} [{'bot' if l['is_bot'] else l['author']}] same_file={l['same_file']} resolved={l['resolved']}")
        L.append(f"  finding: {l['finding'][:300]}")
        L += [f"  reply: {r[:300]}" for r in l["replies"]]
    L += ["", "## Verification", json.dumps(ctx["verification"], ensure_ascii=False)]
    (out_dir / "summary.md").write_text("\n".join(L) + "\n")
    print(json.dumps({"provider": p.name, "out_dir": str(out_dir), "number": n, "head_sha": head_sha, "eligible": eligibility["eligible"],
                      "reasons": reasons, "files": len(files), "added": scale["added"], "large": scale["large"], "lenses": chosen,
                      "worktree": worktree, "workflow_args": str(out_dir / "workflow_args.json")}, ensure_ascii=False))


if __name__ == "__main__":
    main()
