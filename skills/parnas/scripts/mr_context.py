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
  defs.md        changed symbols → definition + one-hop callers/callees (codegraph or rg)
  pack/<unit>.md one self-contained pack per finder unit (lens × shard): diff of its files, defs, rules, threads
  hunks/<f>.patch per-file patch for skeptics
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
NESTED_GUIDELINE_NAMES = {"AGENTS.md", "CLAUDE.md"}
NESTED_GUIDELINE_SUFFIX = "/.greptile/rules.md"
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
    seen: set[str] = set()
    for rel in RULE_CANDIDATES:
        p = Path(repo_dir) / rel
        if p.exists():
            out.append({"path": rel, "bytes": p.stat().st_size, "kind": "instructions", "scope_prefix": ""})
            seen.add(rel)
    tracked = run(["git", "ls-files"], cwd=repo_dir, check=False).splitlines()
    for rel in sorted(tracked):
        if rel in seen:
            continue
        path = Path(rel)
        if path.name in NESTED_GUIDELINE_NAMES:
            scope = path.parent.as_posix()
        elif rel == ".greptile/rules.md" or rel.endswith(NESTED_GUIDELINE_SUFFIX):
            scope = path.parent.parent.as_posix()
        else:
            continue
        scope = "" if scope == "." else scope
        p = Path(repo_dir) / rel
        if p.is_file():
            out.append({"path": rel, "bytes": p.stat().st_size, "kind": "instructions", "scope_prefix": scope})
            seen.add(rel)
    for rules_dir, kind in ((".kody/rules", "kody-rule"), (".coderabbit", "coderabbit"), (".github/instructions", "copilot"), (".cursor/rules", "cursor")):
        d = Path(repo_dir) / rules_dir
        if d.is_dir():
            for p in sorted(list(d.glob("*.md")) + list(d.glob("*.mdc")) + list(d.glob("*.yaml"))):
                txt = p.read_text(errors="ignore")
                title = re.search(r'^title:\s*"?(.+?)"?\s*$', txt, re.M)
                paths = re.search(r"^(?:applyTo|path|globs):\s*(\[?.+?\]?)\s*$", txt, re.M)
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


# ----------------------------------------------------------------------------- context pack: definitions

# 추가된 라인/hunk 컨텍스트에서 정의된 심볼 이름만 뽑는다. 한 번 계산해 finder 전원이 공유하므로
# finder가 직접 Bash로 정의를 찾아 헤매지 않는다.
SYMBOL_KEYWORDS = {"if", "for", "while", "switch", "return", "import", "from", "export", "default", "const", "let", "var",
                   "new", "await", "async", "else", "catch", "try", "case", "function", "class", "def", "func", "type",
                   "interface", "enum", "package", "this", "super", "throw", "with", "not", "and", "print", "self", "cls"}
DEF_LEAD_RE = re.compile(r"^\+?\s*(?:export\s+|default\s+|public\s+|private\s+|protected\s+|static\s+|abstract\s+|readonly\s+)*"
                         r"(?:async\s+)?(?:def|class|func(?:\s*\([^)]*\))?|function|interface|type|enum)\s+([A-Za-z_][A-Za-z0-9_]{2,})")
METHOD_RE = re.compile(r"^\+\s+(?:public\s+|private\s+|protected\s+|static\s+|readonly\s+|abstract\s+)*(?:async\s+)?([A-Za-z_][A-Za-z0-9_]{2,})\s*(?:<[^>]*>)?\([^)]*\)?\s*(?::[^{=]*)?\s*\{?\s*$")


TEST_PATH_RE = re.compile(r"(^|/)(test|tests|__tests__|spec)/|\.(spec|test)\.[a-z]+$|_test\.go$|(^|/)test_[^/]+\.py$")
TEST_HELPER_NAMES = {"expect", "it", "describe", "test", "beforeEach", "afterEach", "beforeAll", "afterAll", "jest", "vi", "assert", "constructor"}


def changed_symbols(patch: str, cap: int = 40, with_files: bool = False):
    """Definition names introduced or touched by the diff — source files first, then tests — capped.
    with_files=True returns [(name, path)] instead of names."""
    src: list[tuple[str, str]] = []
    tst: list[tuple[str, str]] = []
    path = ""

    def add(name: str) -> None:
        if not name or name in SYMBOL_KEYWORDS or name in TEST_HELPER_NAMES:
            return
        if any(name == n for n, _ in src) or any(name == n for n, _ in tst):
            return
        (tst if TEST_PATH_RE.search(path) else src).append((name, path))

    for line in patch.splitlines():
        if line.startswith("+++ "):
            path = line[4:].split("\t")[0]
            path = path[2:] if path.startswith("b/") else path
            continue
        if line.startswith("@@"):
            m = DEF_LEAD_RE.match(line.split("@@")[-1].strip())
            if m:
                add(m.group(1))
            continue
        if not line.startswith("+"):
            continue
        m = DEF_LEAD_RE.match(line)
        if m:
            add(m.group(1))
            continue
        # 메서드 시그니처는 소스 파일에서만 잡는다 — 테스트 파일의 `expect(` 같은 호출과 구분이 어렵다.
        if TEST_PATH_RE.search(path):
            continue
        m = METHOD_RE.match(line)
        if m and not line.rstrip().endswith(";"):
            add(m.group(1))
    pairs = (src + tst)[:cap]
    return pairs if with_files else [n for n, _ in pairs]


def per_lens_cap(max_candidates: int, lens_count: int) -> int:
    """Spread the candidate cap over lenses (+1 slack) so finders stop producing candidates the cap will drop."""
    if lens_count <= 0:
        return 3
    return max(3, -(-max_candidates // lens_count) + 1)


CG_BULLET_RE = re.compile(r"^- `([A-Za-z_][A-Za-z0-9_]*)` \((.+?)\) — (.+)$")
CG_EDGE_RE = re.compile(r"^- (\S+) → (\S+)$")


def _codegraph_symbol(checkout: str, sym: str) -> list[str]:
    try:
        p = subprocess.run(["codegraph", "explore", "-p", checkout, "--max-files", "1", sym], capture_output=True, text=True, timeout=60)
    except (OSError, subprocess.TimeoutExpired):
        return []
    if p.returncode != 0:
        return []
    out, section = [], ""
    for line in p.stdout.splitlines():
        if line.startswith("**Source Code**"):
            break
        if line.startswith("**"):
            section = line.strip("* ").rstrip(":")
            continue
        m = CG_BULLET_RE.match(line)
        if m and m.group(1) == sym:
            out.append(f"- def {m.group(2)} — {m.group(3)}")
            continue
        m = CG_EDGE_RE.match(line)
        if m and sym in (m.group(1), m.group(2)) and section in ("calls", "references", "instantiates"):
            out.append(f"- {section}: {m.group(1)} → {m.group(2)}")
    return out[:25]


def _rg_symbol(checkout: str, sym: str) -> list[str]:
    cmd = ["rg", "-n", "--no-heading", "--color", "never", "-m", "3", "--glob", "!node_modules", "--glob", "!dist", "--glob", "!*lock*", "--glob", "!*.min.*", rf"\b{re.escape(sym)}\b", "."]
    try:
        p = subprocess.run(cmd, cwd=checkout, capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return []
    lines = [l for l in p.stdout.splitlines() if l.strip()][:15]
    return [f"- {l[2:] if l.startswith('./') else l}" for l in lines]


def collect_defs(checkout: str, symbols: list[tuple[str, str]], codegraph: bool) -> dict[str, dict]:
    """symbol → {file, rows}: where it is defined, who calls it, what it calls (one hop each way)."""
    out: dict[str, dict] = {}
    for sym, path in symbols:
        rows = _codegraph_symbol(checkout, sym) if codegraph else []
        if not rows:
            rows = _rg_symbol(checkout, sym)
        out[sym] = {"file": path, "rows": rows or ["- (not found in checkout — external or dynamic; cap confidence at 50 if a claim depends on it)"]}
    return out


def render_defs(checkout: str, defs: dict[str, dict], codegraph: bool) -> str:
    L = ["# Definitions and one-hop neighbours of changed symbols", "",
         f"source: {'codegraph explore' if codegraph else 'rg'} over {checkout}",
         "Read this before opening files: it names the definition and the callers/callees you must trace. Open a file only for a hop listed here or for a symbol missing here.", ""]
    if not defs:
        L.append("(no symbols detected in the diff — trace from diff.patch directly)")
        return "\n".join(L) + "\n"
    for sym, d in defs.items():
        L += [f"## {sym}", f"defined in: {d['file']}"] + d["rows"] + [""]
    return "\n".join(L) + "\n"


def build_defs(checkout: str, symbols: list[str], codegraph: bool) -> str:
    return render_defs(checkout, collect_defs(checkout, [(s, "?") for s in symbols], codegraph), codegraph)


# ----------------------------------------------------------------------------- context pack: per-lens packs

# finder 한 명이 읽는 팩의 상한. 넘으면 디렉터리 그룹 단위로 샤딩한다 (~40K 토큰).
# lens 묶음: 같은 팩을 lens 마다 다른 에이전트가 다시 읽는 비용이 lens 분리의 독립성 이득보다 컸다
# (2026-08-28 !5617 실측: lens×샤드 25명 44.5M vs lens당 1명 32.8M). 한 finder 가 묶음의 lens 를 모두 적용하고
# lens 별로 결과를 따로 낸다.
LENS_BUNDLES = [("behavior", ["logic", "boundary", "data", "async"]),
                ("contract", ["security", "contract", "rules"]),
                ("intent", ["tests", "scope", "intent"])]
# 모델: 전 역할 opus. 2026-08-28 !5617 실측 — 싼 모델은 턴을 더 써서 토큰이 줄지 않고(sonnet finder 31턴 ≈ opus 33턴),
# haiku critic 5명은 14.4M 토큰에 생존 후보 0, haiku merge 초안은 임계값 규칙을 무시했다. 비용 레버는 모델이 아니라
# 구조(팩·prescreen·incremental)다. 필요하면 args.models 로 역할별 덮어쓴다.
MODEL_DEFAULTS = {"finder": "opus", "tracer": "opus", "reproducer": "opus"}
PACK_CAP_BYTES = int(os.environ.get("PARNAS_PACK_CAP_BYTES", str(150_000)))


def hunk_slug(path: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.\-]", "_", path.replace("/", "__"))


def files_for_lens(lens: str, files: list[dict], per_file: dict[str, str]) -> list[dict]:
    """Which changed files a lens inspects — mirrors the `applies` column of lenses.md.
    Keyword lenses (security/data/async) match on each file's OWN diff, not the whole patch."""
    def has(f, *tags): return bool(set(tags) & set(f["tags"]))
    def diff_has(f, pat): return re.search(pat, per_file.get(f["path"], "").lower()) is not None
    if lens in ("scope", "intent"):
        return list(files)
    if lens in ("logic", "tests", "boundary", "rules"):
        return [f for f in files if not has(f, "docs")]
    if lens == "security":
        return [f for f in files if has(f, "security", "controller", "gateway", "dto", "ops") or (not has(f, "docs", "test", "generated") and diff_has(f, r"auth|token|secret|permission|process\.env"))]
    if lens == "data":
        return [f for f in files if has(f, "db", "migration") or (not has(f, "docs", "test") and diff_has(f, r"repository|createquerybuilder|\.query\(|transaction"))]
    if lens == "async":
        return [f for f in files if has(f, "async", "gateway") or (not has(f, "docs", "test", "generated") and diff_has(f, r"kafka|queue|stream|websocket|retry|timeout|debounce|job"))]
    if lens == "contract":
        return [f for f in files if has(f, "dto", "controller", "gateway", "generated")]
    return list(files)


def _group(path: str) -> str:
    parts = path.split("/")
    return "/".join(parts[:2]) if len(parts) > 2 else (parts[0] if len(parts) > 1 else ".")


def shard_files(files: list[dict], per_file: dict[str, str], cap_bytes: int = PACK_CAP_BYTES) -> list[dict]:
    """Bin-pack files into shards ≤ cap_bytes of diff text. Files are ordered by directory group so a
    shard stays coherent, but small groups share a shard — every shard costs one agent."""
    total = sum(len(per_file.get(f["path"], "")) for f in files)
    if total <= cap_bytes:
        return [{"id": "all", "files": list(files)}]
    ordered = sorted(files, key=lambda f: (_group(f["path"]), f["path"]))
    shards: list[dict] = []
    cur: list[dict] = []
    size = 0
    for f in ordered:
        sz = len(per_file.get(f["path"], ""))
        if cur and size + sz > cap_bytes:
            shards.append(cur); cur, size = [], 0
        cur.append(f); size += sz
    if cur:
        shards.append(cur)
    out = []
    for i, fs in enumerate(shards, 1):
        groups = sorted({_group(f["path"]) for f in fs})
        first = groups[0].split("/")[-1] or "root"
        first = "root" if first == "." else first
        label = first if len(groups) == 1 else f"{first}+{len(groups) - 1}"
        out.append({"id": f"{i}-{label}", "files": fs})
    return out


def render_pack(shard: dict, lenses: list[str], lens_texts: dict[str, str], per_file: dict[str, str], defs: dict[str, dict], meta: dict) -> str:
    paths = {f["path"] for f in shard["files"]}
    L = [f"# Inspection pack — lenses `{'`, `'.join(lenses)}`, shard `{shard['id']}`", "", f"MR: {meta['title']}", "",
         "This file is the whole context for your slice: the cumulative diff of every file you inspect, the definitions and one-hop neighbours of the symbols they define, matching rules, existing threads and prior lessons. Read it once; open the checkout only for a hop it names.", "",
         "## Lenses (apply each one separately; report every candidate under the lens that found it)"]
    L += [f"- `{l}`: {lens_texts[l]}" for l in lenses]
    L += ["", "## Description"]
    full = bool({"scope", "intent"} & set(lenses))
    L.append(meta["description"] if full else meta["description"][:1500])
    if "intent" in lenses and meta.get("issues_md"):
        L += ["", "## Linked issues", meta["issues_md"]]
    L += ["", f"## Files ({len(shard['files'])})"]
    for f in shard["files"]:
        hs = ",".join(f"{h['new_start']}-{h['new_start'] + max(h['new_len'], 1) - 1}" for h in f["hunks"])
        L.append(f"- {f['path']} [{'/'.join(f['tags'])}] +{f['added']}/-{f['removed']} hunks(new): {hs}")
        for h in f.get("enclosing", []):
            L.append(f"  - hunk {h['new_start']} inside: " + " > ".join(f"{ln}: {txt.strip()}" for ln, txt in h["context"]))
        if f.get("co_change"):
            L.append(f"  - changes together with (not in this diff): {', '.join(f['co_change'])}")
    rules = [r for r in meta.get("rules", []) if _rule_matches(r, paths)]
    if rules:
        L += ["", "## Rules that apply to these files",
              "Read applicable guidelines from root to leaf; when they conflict, the nearest directory scope wins."] + [
            f"- {r['path']}"
            + (f" — scope {r['scope_prefix'] or '/'}" if "scope_prefix" in r else "")
            + (f" — {r['title']} (globs {r['globs']})" if r.get("globs") else "")
            for r in rules
        ]
    threads = [t for t in meta.get("threads", []) if t.get("path") in paths]
    if threads:
        L += ["", "## Existing threads on these files (do not re-raise unless you contradict them)"]
        L += [f"- [{'BOT' if t['is_bot'] else t['author']}] {t['path']}:{t['new_line'] or '-'} resolved={t['resolved']} :: {t['excerpt'][:200]}" for t in threads]
    lessons = [l for l in meta.get("lessons", []) if l.get("path") in paths]
    if lessons:
        L += ["", "## Prior review lessons on these files (already refuted — never repeat)"]
        L += [f"- {l['mr']} {l['path']}:{l['new_line'] or '-'} :: {l['finding'][:300]}" for l in lessons]
    L += ["", "## Definitions and one-hop neighbours (symbols defined in these files)"]
    n = 0
    for sym, d in defs.items():
        if d["file"] in paths:
            n += 1
            L += [f"### {sym}", f"defined in: {d['file']}"] + d["rows"] + [""]
    if not n:
        L.append("(none detected — trace from the diff)")
    L += ["", "## Diff (cumulative, base...head)", ""]
    for f in shard["files"]:
        L += ["```diff", per_file.get(f["path"], "").rstrip("\n"), "```", ""]
    return "\n".join(L) + "\n"


def _rule_matches(rule: dict, paths: set[str]) -> bool:
    scope = rule.get("scope_prefix")
    if scope and not any(p == scope or p.startswith(f"{scope}/") for p in paths):
        return False
    globs = rule.get("globs")
    if not globs:
        return True
    import fnmatch
    pats = [g.strip().strip("'\"") for g in re.split(r"[,\s]+", globs.strip("[]")) if g.strip()]
    return any(fnmatch.fnmatch(p, g) or fnmatch.fnmatch(p, f"**/{g}") for p in paths for g in pats)


# ----------------------------------------------------------------------------- research-backed context helpers

def enclosing_context(lines: list[str], new_start: int, max_up: int = 30) -> list[tuple[int, str]]:
    """Definition headers enclosing a hunk (PR-Agent `dynamic_context`): walk up from the hunk start, collect
    def/class/function lines with strictly decreasing indentation, up to max_up lines. 1-based line numbers."""
    out: list[tuple[int, str]] = []
    idx = min(new_start, len(lines)) - 1
    if idx < 0:
        return out
    indent_limit = None
    for i in range(idx - 1, max(-1, idx - 1 - max_up), -1):
        line = lines[i]
        if not line.strip():
            continue
        ind = len(line) - len(line.lstrip())
        if indent_limit is not None and ind >= indent_limit:
            continue
        if DEF_LEAD_RE.match(line.strip()) or METHOD_RE.match("+" + line):
            out.append((i + 1, line.rstrip()))
            indent_limit = ind
            if ind == 0:
                break
    return list(reversed(out))


def co_change_files(repo_dir: str, path: str, changed: set[str], limit_commits: int = 30, min_hits: int = 3) -> list[str]:
    """Files that historically change together with `path` (CodeRabbit co-change signal) and are NOT in this diff."""
    out = run(["git", "log", f"-{limit_commits}", "--format=%H", "--name-only", "--", path], cwd=repo_dir, check=False)
    hits: dict[str, int] = {}
    for line in out.splitlines():
        line = line.strip()
        if not line or re.fullmatch(r"[0-9a-f]{40}", line) or line == path or line in changed:
            continue
        hits[line] = hits.get(line, 0) + 1
    return [f for f, n in sorted(hits.items(), key=lambda x: -x[1]) if n >= min_hits][:5]


REFUTED_HISTORY = ".agent-harness/parnas/refuted.jsonl"   # 레포에 커밋되는 팀 메모리 (review/ 디렉터리는 gitignore 됨)
SUPPRESS_EXEMPT = {"security", "data"}                   # Greptile/Kodus: 보안·데이터 지적은 절대 억제하지 않는다
TOKEN_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]{2,}|[가-힣]{2,}")


def load_refuted_history(repo_dir: str) -> list[dict]:
    p = Path(repo_dir) / REFUTED_HISTORY
    if not p.exists():
        return []
    out = []
    for line in p.read_text(errors="ignore").splitlines():
        try:
            out.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return out


def _tokens(*texts: str) -> set[str]:
    return {t.lower() for t in TOKEN_RE.findall(" ".join(x or "" for x in texts))}


def matches_refuted(history: list[dict], cand: dict, threshold: float = 0.5) -> bool:
    """Same file AND token overlap coefficient (|∩| / min) ≥ threshold on title/what; never for security/data."""
    if cand.get("category") in SUPPRESS_EXEMPT:
        return False
    ct = _tokens(cand.get("title", ""), cand.get("what", ""))
    if not ct:
        return False
    for h in history:
        if h.get("path") != cand.get("path"):
            continue
        ht = _tokens(h.get("title", ""), h.get("what", ""))
        if ht and len(ct & ht) / min(len(ct), len(ht)) >= threshold:
            return True
    return False


def incremental_plan(prev: dict, changed: set[str], units: list[dict]) -> dict:
    """Re-inspect only units whose files changed since the previous head; carry findings on untouched files."""
    keep = [u for u in units if any(f in changed for f in u.get("files", []))]
    carried, dropped = [], 0
    for f in prev.get("findings", []):
        if f.get("path") in changed:
            dropped += 1
        else:
            carried.append({**f, "carried_from": prev.get("head_sha")})
    return {"units": keep, "carried": carried, "dropped": dropped}


def make_worktree(repo_dir: str, out_dir: Path, head_sha: str) -> str:
    wt = (out_dir / "worktree").resolve()
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
    ap.add_argument("--incremental", action="store_true", help="re-inspect only shards touched since the previous run in --out (carry the rest)")
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

    out_dir = (Path(a.out).expanduser() if a.out else default_out_dir(repo_dir, p.name, meta, n)).resolve()
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
        "linked_issues": issues, "rule_pack": rule_pack(worktree or repo_dir), "existing_threads": threads, "prior_review_lessons": lessons,
        "verification": detect_verification(repo_dir), "worktree": worktree, "repo_dir": repo_dir, "out_dir": str(out_dir),
    }
    scale = {"files": len(files), "added": ctx["totals"]["added"], "large": len(files) > 40 or ctx["totals"]["added"] > 2000}
    ctx["scale"] = scale
    ctx["prior_review_threads"] = [t for t in threads if t["is_own_review"]]
    if (out_dir / "context.json").exists():
        (out_dir / "context.prev.json").write_text((out_dir / "context.json").read_text())
    (out_dir / "context.json").write_text(json.dumps(ctx, ensure_ascii=False, indent=1))
    lenses = load_lenses()
    chosen = [l for l in select_lenses(files, "\n".join(patch_parts), issues, lessons) if l in lenses]
    checkout = worktree or repo_dir
    full_patch = "\n".join(patch_parts)
    sym_pairs = changed_symbols(full_patch, with_files=True)
    defs = collect_defs(checkout, sym_pairs, ctx["verification"]["codegraph"])
    (out_dir / "defs.md").write_text(render_defs(checkout, defs, ctx["verification"]["codegraph"]))
    symbols = [n for n, _ in sym_pairs]
    # 조사 근거 컨텍스트: hunk 를 감싸는 정의 헤더(PR-Agent dynamic_context) 와 co-change 파일(CodeRabbit).
    changed_paths = {f["path"] for f in files}
    for f in files:
        f["enclosing"] = []
        src_path = Path(checkout) / f["path"]
        if src_path.is_file() and not ({"docs", "generated"} & set(f["tags"])):
            src_lines = src_path.read_text(errors="ignore").splitlines()
            for h in f["hunks"]:
                ctx_lines = enclosing_context(src_lines, h["new_start"])
                if ctx_lines:
                    f["enclosing"].append({"new_start": h["new_start"], "context": ctx_lines})
        f["co_change"] = co_change_files(repo_dir, f["path"], changed_paths) if not ({"docs", "generated", "test"} & set(f["tags"])) else []
    refuted_history = load_refuted_history(repo_dir)
    # 파일별 hunk 패치 — skeptic 이 후보 파일의 diff 만 읽는다.
    import shutil
    hunk_dir = out_dir / "hunks"
    shutil.rmtree(hunk_dir, ignore_errors=True)
    hunk_dir.mkdir()
    per_file = {f["path"]: part for f, part in zip(files, patch_parts)}
    for path, part in per_file.items():
        (hunk_dir / f"{hunk_slug(path)}.patch").write_text(part + "\n")
    # lens × shard 팩 — finder 한 명이 읽는 유일한 컨텍스트.
    pack_dir = out_dir / "pack"
    shutil.rmtree(pack_dir, ignore_errors=True)
    pack_dir.mkdir()
    issues_md = "\n".join(f"### #{i['number']} {i['title']} [{i['state']}]\n{i['description'][:6000]}\n" + ("\n".join(f"- [{'x' if c['done'] else ' '}] {c['text']}" for c in i["checklist"]) if i["checklist"] else "") for i in issues)
    pack_meta = {"title": ctx["title"], "description": ctx["description"] or "(empty)", "issues_md": issues_md, "rules": ctx["rule_pack"], "threads": threads, "lessons": lessons}
    max_candidates = 12 if scale["large"] else 24
    units = []
    for bundle, blenses in LENS_BUNDLES:
        active = [l for l in blenses if l in chosen]
        if not active:
            continue
        # 묶음이 보는 파일 = 묶음 안 lens 들이 보는 파일의 합집합 (lens 별 적용 범위는 팩 안 파일 태그로 finder 가 판단).
        seen_paths: set[str] = set()
        bfiles = []
        for l in active:
            for f in files_for_lens(l, files, per_file):
                if f["path"] not in seen_paths:
                    seen_paths.add(f["path"]); bfiles.append(f)
        bfiles.sort(key=lambda f: f["path"])
        for shard in shard_files(bfiles, per_file):
            uid = f"{bundle}@{shard['id']}"
            rel = f"pack/{hunk_slug(uid)}.md"
            (out_dir / rel).write_text(render_pack(shard, active, {l: lenses[l]["text"] for l in active}, per_file, defs, pack_meta))
            units.append({"id": uid, "lenses": active, "pack": rel, "files": [f["path"] for f in shard["files"]], "bytes": (out_dir / rel).stat().st_size})
    (out_dir / "units.json").write_text(json.dumps(units, ensure_ascii=False, indent=1))
    # incremental 재리뷰 (Ellipsis: "never the whole pull request again"): 이전 결과가 있고 head 가 움직였으면
    # 바뀐 파일을 포함한 유닛만 다시 조사하고, 안 바뀐 파일의 finding 은 이월한다.
    carried: list[dict] = []
    prev_result = out_dir / "workflow-result.json"
    prev_ctx = out_dir / "context.prev.json"
    if a.incremental and prev_result.exists() and prev_ctx.exists():
        prev_head = json.loads(prev_ctx.read_text()).get("diff_refs", {}).get("head_sha")
        if prev_head and prev_head != head_sha and ok(["git", "cat-file", "-e", f"{prev_head}^{{commit}}"], repo_dir):
            changed_since = set(run(["git", "diff", "--name-only", prev_head, head_sha], cwd=repo_dir, check=False).split())
            plan = incremental_plan({**json.loads(prev_result.read_text()), "head_sha": prev_head}, changed_since, units)
            units, carried = plan["units"], plan["carried"]
            print(json.dumps({"incremental": True, "since": prev_head, "changed_files": len(changed_since), "units": len(units), "carried": len(carried), "dropped": plan["dropped"]}, ensure_ascii=False))
    wf_args = {"outDir": str(out_dir), "checkout": checkout, "codegraph": ctx["verification"]["codegraph"],
               "lenses": chosen, "lensText": {l: lenses[l]["text"] for l in chosen},
               # 파일 목록은 팩 안에 있으므로 args 에는 싣지 않는다 (units.json 참조).
               "units": [{"id": u["id"], "lenses": u["lenses"], "pack": u["pack"], "files": u["files"]} for u in units],
               "maxCandidates": max_candidates, "perLensCap": per_lens_cap(max_candidates, len(chosen)),
               "models": MODEL_DEFAULTS, "gate": str(out_dir / "gate.json"), "skillDir": str(Path(__file__).resolve().parent.parent),
               "carried": carried,
               # 반박 이력(레포 커밋 메모리): path + title/what — workflow.js prescreen 이 토큰 Jaccard 로 비교한다.
               "refutedHistory": [{"path": h.get("path"), "title": h.get("title"), "what": (h.get("what") or "")[:200], "category": h.get("category")} for h in refuted_history][-400:],
               # 결정적 prescreen 입력: hunk 범위(변경/근접 컨텍스트)와 이미 반박된 지적 텍스트.
               "hunkRanges": {f["path"]: [[h["new_start"], h["new_start"] + max(h["new_len"], 1) - 1] for h in f["hunks"]] for f in files},
               "priorLessons": [l["finding"][:300] for l in lessons]}
    (out_dir / "workflow_args.json").write_text(json.dumps(wf_args, ensure_ascii=False, indent=1))

    L = [f"# {p.name} {'!' if p.name == 'gitlab' else '#'}{n} — {ctx['title']}", ctx["web_url"] or "",
         f"author={ctx['author']} source={ctx['source_branch']} → target={ctx['target_branch']} labels={','.join(ctx['labels']) or '-'}",
         f"head={head_sha} base={refs.get('base_sha')} start={refs.get('start_sha')} head_local={have_head} worktree={worktree or '-'}",
         f"eligible={eligibility['eligible']} reasons={reasons or '-'}",
         f"scale: files={scale['files']} added={scale['added']} large={scale['large']} → lenses={','.join(chosen)} (workflow_args.json)",
         f"defs.md: {len(symbols)} changed symbols with definition + one-hop callers/callees; pack/: {len(units)} finder units (lens bundle × shard, ≤ ~{PACK_CAP_BYTES // 1000}KB diff each); hunks/: per-file patches",
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
                      "units": len(units), "worktree": worktree, "workflow_args": str(out_dir / "workflow_args.json")}, ensure_ascii=False))


if __name__ == "__main__":
    main()
