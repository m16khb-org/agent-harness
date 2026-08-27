#!/usr/bin/env python3
"""Provider-neutral review-thread feedback helper for GitHub PRs and GitLab MRs.

Subcommands (all print one JSON document to stdout; never print tokens):

  list    --pr <ref> [--repo-dir .] [--provider github|gitlab] [--project owner/repo]
          [--hostname host] [--all]
          Normalised review threads. By default only threads opened by a review
          agent (bot) are returned; --all includes human threads.

  apply   --pr <ref> --plan <plan.json> [--dry-run] [same provider flags]
          Executes reply / reaction / resolve per thread from the plan, then
          re-reads every touched thread and prints a verified ledger.

  reply   --pr <ref> --thread <thread_id> --body-file <path>
  react   --pr <ref> --note <note_id> --reaction up|down
  resolve --pr <ref> --thread <thread_id>   (and `unresolve`)
  verify  --pr <ref> --thread <thread_id> [--reply-id id] [--note id --reaction up|down]
          Single primitives, for hosts or cases where `apply` is too coarse.

Provider is auto-detected from the `origin` remote host (github.com → `gh`,
anything else → `glab`); override with --provider/--project/--hostname. Both CLIs
must already be authenticated for the target host (`gh auth status`,
`glab auth status --hostname <host>`).

Plan file schema (`apply`):

  {"threads": [
     {"thread_id": "<id from list>",           # required
      "verdict": "valid|partial|invalid|out_of_scope|hold",
      "reply": "<Korean markdown, no mention needed>",  # required unless "skip_reply": true
      "reaction": "up|down|none",              # optional; default derived from verdict
      "resolve": true|false,                   # optional; default: verdict != hold
      "reason_open": "<why it stays open>"     # required when resolve is false
     }]}

Idempotency: every reply carries `<!-- review-agent-feedback thread=<id> verdict=<v> -->`.
A thread whose latest reply by the current user already carries the marker is
not replied to again; reactions are not duplicated; resolve is skipped when the
thread is already resolved. The ledger reports what was observed after the run,
not what the POST responses claimed.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.parse
from pathlib import Path

CALL_TIMEOUT = int(os.environ.get("REVIEW_FEEDBACK_CALL_TIMEOUT", "90"))
MARKER_RE = re.compile(r"<!--\s*review-agent-feedback\s+thread=(\S+)\s+verdict=(\w+)\s*-->")
VERDICTS = ("valid", "partial", "invalid", "out_of_scope", "hold")
DEFAULT_REACTION = {"valid": "up", "partial": "up", "invalid": "down", "out_of_scope": "down", "hold": "none"}

# Known review agents. `mention` is the handle that makes the agent read the reply
# (feedback loop); None means the agent does not learn from replies.
REVIEWERS = [
    {"key": "kody", "mention": "@kody", "logins": (r"^kody", r"^kodus"), "markers": ("kody-codereview", "kody-code-review", "kody-pr-summary", "kodus")},
    {"key": "coderabbit", "mention": "@coderabbitai", "logins": (r"^coderabbitai",), "markers": ("coderabbit",)},
    {"key": "copilot", "mention": None, "logins": (r"^copilot",), "markers": ("copilot-pull-request",)},
    {"key": "gemini", "mention": "@gemini-code-assist", "logins": (r"^gemini-code-assist",), "markers": ("gemini-code-assist",)},
    {"key": "parnas", "mention": None, "logins": (), "markers": ("parnas-review", "parnas-finding")},
]
BOT_LOGIN_RE = re.compile(r"(\[bot\]$|^project_\d+_bot|^group_\d+_bot|[-_]bot$|^bot[-_])", re.I)
SECRET_RE = re.compile(r"\b(glpat-[A-Za-z0-9_\-]{10,}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9\-]{10,})\b")


# ----------------------------------------------------------------------------- shell


def run(cmd: list[str], cwd: str | None = None, check: bool = True) -> str:
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=CALL_TIMEOUT)
    except subprocess.TimeoutExpired:
        raise SystemExit(f"command timed out after {CALL_TIMEOUT}s: {' '.join(cmd[:4])}... (set REVIEW_FEEDBACK_CALL_TIMEOUT)")
    if check and p.returncode != 0:
        raise SystemExit(f"command failed ({p.returncode}): {' '.join(cmd[:4])}...\n{SECRET_RE.sub('***', p.stderr.strip()[:500])}")
    return p.stdout


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


def api(cli: str, hostname: str | None, path: str, method: str = "GET", body: dict | None = None, paginate: bool = False):
    """`gh api` / `glab api` with a JSON body passed via --input (multiline-safe)."""
    cmd = [cli, "api", path, "--method", method]
    if hostname:
        cmd += ["--hostname", hostname]
    if paginate:
        cmd.append("--paginate")
    tmp = None
    if body is not None:
        tmp = tempfile.NamedTemporaryFile("w", suffix=".json", delete=False)
        json.dump(body, tmp, ensure_ascii=False)
        tmp.close()
        cmd += ["--header", "Content-Type: application/json", "--input", tmp.name]
    try:
        out = run(cmd)
    finally:
        if tmp:
            Path(tmp.name).unlink(missing_ok=True)
    if paginate:
        return merge_paginated(out)
    return json.loads(out) if out.strip() else {}


# ----------------------------------------------------------------------------- classification (pure)


def classify_author(login: str, body: str, author_type: str | None = None, is_bot_flag: bool | None = None) -> tuple[str, str | None, str | None]:
    """Return (author_kind, reviewer_key, mention). author_kind ∈ {bot, human}."""
    login_l = (login or "").lower()
    body_l = (body or "").lower()
    for r in REVIEWERS:
        if any(re.search(p, login_l) for p in r["logins"]) or any(m in body_l for m in r["markers"]):
            return "bot", r["key"], r["mention"]
    if is_bot_flag or (author_type or "").lower() == "bot" or BOT_LOGIN_RE.search(login_l):
        return "bot", "unknown", None
    return "human", None, None


def marker(thread_id: str, verdict: str) -> str:
    return f"<!-- review-agent-feedback thread={thread_id} verdict={verdict} -->"


def find_marker(body: str) -> tuple[str, str] | None:
    m = MARKER_RE.search(body or "")
    return (m.group(1), m.group(2)) if m else None


def normalize_plan(plan: dict) -> list[dict]:
    items = plan.get("threads")
    if not isinstance(items, list) or not items:
        raise SystemExit("plan.threads must be a non-empty list")
    out = []
    for i, t in enumerate(items):
        tid = str(t.get("thread_id") or "").strip()
        verdict = str(t.get("verdict") or "").strip()
        if not tid:
            raise SystemExit(f"plan.threads[{i}]: thread_id required")
        if verdict not in VERDICTS:
            raise SystemExit(f"plan.threads[{i}]: verdict must be one of {VERDICTS}, got {verdict!r}")
        reply = (t.get("reply") or "").strip()
        if not reply and not t.get("skip_reply"):
            raise SystemExit(f"plan.threads[{i}]: reply required (or skip_reply: true)")
        reaction = t.get("reaction") or DEFAULT_REACTION[verdict]
        if reaction not in ("up", "down", "none"):
            raise SystemExit(f"plan.threads[{i}]: reaction must be up|down|none")
        resolve = t.get("resolve")
        if resolve is None:
            resolve = verdict != "hold"
        if not resolve and not (t.get("reason_open") or "").strip():
            raise SystemExit(f"plan.threads[{i}]: reason_open required when resolve is false")
        out.append({"thread_id": tid, "verdict": verdict, "reply": reply, "reaction": reaction, "resolve": bool(resolve), "reason_open": (t.get("reason_open") or "").strip()})
    return out


def compose_reply(reply: str, thread_id: str, verdict: str, mention: str | None) -> str:
    body = reply.strip()
    if mention and not body.startswith(mention):
        body = f"{mention} {body}"
    return f"{body}\n\n{marker(thread_id, verdict)}"


def normalize_thread(raw: dict, me: str) -> dict:
    """raw: provider-agnostic dict produced by a Provider.threads() implementation."""
    notes = raw["notes"]
    first = notes[0]
    kind, reviewer, mention = classify_author(first["author"], first["body"], first.get("author_type"), first.get("author_bot"))
    my_replies = [n for n in notes[1:] if (n["author"] or "").lower() == (me or "").lower()]
    last_marker = None
    for n in reversed(my_replies):
        last_marker = find_marker(n["body"])
        if last_marker:
            break
    return {
        "thread_id": raw["thread_id"],
        "resolve_id": raw.get("resolve_id") or raw["thread_id"],
        "note_id": first["id"],
        "resolvable": bool(raw.get("resolvable", True)),
        "resolved": bool(raw.get("resolved")),
        "outdated": bool(raw.get("outdated")),
        "path": raw.get("path"),
        "line": raw.get("line"),
        "author": first["author"],
        "author_kind": kind,
        "reviewer": reviewer,
        "mention": mention,
        "body": first["body"],
        "url": first.get("url"),
        "reply_count": len(notes) - 1,
        "my_reply_count": len(my_replies),
        "already_handled": last_marker[1] if last_marker else None,
    }


# ----------------------------------------------------------------------------- providers


def detect_remote(repo_dir: str) -> tuple[str, str]:
    url = run(["git", "remote", "get-url", "origin"], cwd=repo_dir).strip()
    m = re.match(r"(?:https?://|ssh://git@|git@)([^/:]+)(?::\d+)?[:/](.+?)(?:\.git)?/?$", url)
    if not m:
        raise SystemExit(f"cannot parse origin remote: {url}")
    return m.group(1), m.group(2)


def parse_ref(ref: str) -> tuple[str | None, str | None, int | None]:
    """Return (host, project, number) from a URL, or (None, None, number) from !n/#n/n."""
    m = re.match(r"https?://([^/]+)/(.+?)/(?:-/)?(?:merge_requests|pull)/(\d+)", ref)
    if m:
        return m.group(1), m.group(2), int(m.group(3))
    m = re.match(r"^[!#]?(\d+)$", ref.strip())
    if m:
        return None, None, int(m.group(1))
    raise SystemExit(f"cannot parse PR/MR reference: {ref}")


class GitHub:
    name = "github"

    def __init__(self, host: str, project: str, number: int):
        self.host = None if host == "github.com" else host
        self.project, self.number = project, number
        self.owner, self.repo = project.split("/", 1)

    def me(self) -> str:
        return api("gh", self.host, "user").get("login", "")

    def graphql(self, query: str, variables: dict) -> dict:
        cmd = ["gh", "api", "graphql"]
        if self.host:
            cmd += ["--hostname", self.host]
        cmd += ["-f", f"query={query}"]
        for k, v in variables.items():
            cmd += (["-F", f"{k}={v}"] if isinstance(v, int) else ["-f", f"{k}={v}"])
        data = json.loads(run(cmd))
        if data.get("errors"):
            raise SystemExit(f"graphql error: {json.dumps(data['errors'])[:500]}")
        return data["data"]

    THREADS_Q = """query($owner:String!,$repo:String!,$number:Int!,$after:String){
      repository(owner:$owner,name:$repo){ pullRequest(number:$number){ headRefOid state
        reviewThreads(first:100,after:$after){ pageInfo{hasNextPage endCursor}
          nodes{ id isResolved isOutdated path line originalLine viewerCanResolve
            comments(first:100){ nodes{ databaseId body url author{ login __typename } } } } } } } }"""

    def threads(self) -> tuple[list[dict], dict]:
        out, after, meta = [], None, {}
        while True:
            vars_ = {"owner": self.owner, "repo": self.repo, "number": self.number}
            if after:
                vars_["after"] = after
            pr = self.graphql(self.THREADS_Q, vars_)["repository"]["pullRequest"]
            meta = {"head_sha": pr["headRefOid"], "state": pr["state"]}
            rt = pr["reviewThreads"]
            for n in rt["nodes"]:
                notes = [{"id": c["databaseId"], "body": c["body"], "url": c["url"],
                          "author": (c.get("author") or {}).get("login") or "",
                          "author_type": (c.get("author") or {}).get("__typename")} for c in n["comments"]["nodes"]]
                if not notes:
                    continue
                out.append({"thread_id": n["id"], "resolve_id": n["id"], "resolvable": bool(n["viewerCanResolve"]) or bool(n["isResolved"]),
                            "resolved": n["isResolved"], "outdated": n["isOutdated"], "path": n["path"],
                            "line": n["line"] or n["originalLine"], "notes": notes})
            if not rt["pageInfo"]["hasNextPage"]:
                break
            after = rt["pageInfo"]["endCursor"]
        return out, meta

    def thread(self, thread_id: str) -> dict:
        for t in self.threads()[0]:
            if t["thread_id"] == thread_id:
                return t
        raise SystemExit(f"thread not found: {thread_id}")

    def reply(self, thread: dict, body: str) -> str:
        first_id = thread["notes"][0]["id"]
        r = api("gh", self.host, f"repos/{self.project}/pulls/{self.number}/comments/{first_id}/replies", "POST", {"body": body})
        return str(r.get("id") or "")

    def react(self, note_id: str, reaction: str) -> None:
        content = {"up": "+1", "down": "-1"}[reaction]
        api("gh", self.host, f"repos/{self.project}/pulls/comments/{note_id}/reactions", "POST", {"content": content})

    def reactions_by(self, note_id: str, login: str) -> list[str]:
        rs = api("gh", self.host, f"repos/{self.project}/pulls/comments/{note_id}/reactions?per_page=100", paginate=True)
        back = {"+1": "up", "-1": "down"}
        return [back.get(r.get("content"), r.get("content")) for r in rs if ((r.get("user") or {}).get("login") or "").lower() == login.lower()]

    def set_resolved(self, thread_id: str, resolved: bool) -> None:
        mut = "resolveReviewThread" if resolved else "unresolveReviewThread"
        self.graphql(f"mutation($id:ID!){{ {mut}(input:{{threadId:$id}}){{ thread{{ id isResolved }} }} }}", {"id": thread_id})


class GitLab:
    name = "gitlab"

    def __init__(self, host: str, project: str, number: int):
        self.host, self.project, self.number = host, project, number
        self.enc = urllib.parse.quote(project, safe="")

    def me(self) -> str:
        return api("glab", self.host, "user").get("username", "")

    def _base(self) -> str:
        return f"projects/{self.enc}/merge_requests/{self.number}"

    def _norm(self, d: dict) -> dict | None:
        notes = [n for n in d.get("notes", []) if not n.get("system")]
        if not notes:
            return None
        first = notes[0]
        pos = first.get("position") or {}
        return {"thread_id": d["id"], "resolve_id": d["id"], "resolvable": bool(first.get("resolvable")), "resolved": bool(first.get("resolved")),
                "outdated": False, "path": pos.get("new_path") or pos.get("old_path"), "line": pos.get("new_line") or pos.get("old_line"),
                "notes": [{"id": n["id"], "body": n.get("body", ""), "url": None,
                           "author": (n.get("author") or {}).get("username") or "", "author_bot": (n.get("author") or {}).get("bot")} for n in notes]}

    def threads(self) -> tuple[list[dict], dict]:
        mr = api("glab", self.host, self._base())
        meta = {"head_sha": mr.get("sha"), "state": mr.get("state")}
        out = []
        for d in api("glab", self.host, f"{self._base()}/discussions?per_page=100", paginate=True):
            t = self._norm(d)
            if t:
                out.append(t)
        return out, meta

    def thread(self, thread_id: str) -> dict:
        t = self._norm(api("glab", self.host, f"{self._base()}/discussions/{thread_id}"))
        if not t:
            raise SystemExit(f"thread not found: {thread_id}")
        return t

    def reply(self, thread: dict, body: str) -> str:
        r = api("glab", self.host, f"{self._base()}/discussions/{thread['thread_id']}/notes", "POST", {"body": body})
        return str(r.get("id") or "")

    def react(self, note_id: str, reaction: str) -> None:
        name = {"up": "thumbsup", "down": "thumbsdown"}[reaction]
        api("glab", self.host, f"{self._base()}/notes/{note_id}/award_emoji", "POST", {"name": name})

    def reactions_by(self, note_id: str, login: str) -> list[str]:
        rs = api("glab", self.host, f"{self._base()}/notes/{note_id}/award_emoji?per_page=100", paginate=True)
        back = {"thumbsup": "up", "thumbsdown": "down"}
        return [back.get(r.get("name"), r.get("name")) for r in rs if ((r.get("user") or {}).get("username") or "").lower() == login.lower()]

    def set_resolved(self, thread_id: str, resolved: bool) -> None:
        api("glab", self.host, f"{self._base()}/discussions/{thread_id}", "PUT", {"resolved": resolved})


def make_provider(args) -> GitHub | GitLab:
    url_host, url_project, number = parse_ref(args.pr)
    host = args.hostname or url_host
    project = args.project or url_project
    if not host or not project:
        r_host, r_project = detect_remote(args.repo_dir)
        host, project = host or r_host, project or r_project
    provider = args.provider or ("github" if host == "github.com" or "github" in host else "gitlab")
    if provider == "github":
        return GitHub(host, project, number)
    return GitLab(host, project, number)


# ----------------------------------------------------------------------------- commands


def emit(obj) -> None:
    print(SECRET_RE.sub("***", json.dumps(obj, ensure_ascii=False, indent=2)))


def cmd_list(args) -> None:
    p = make_provider(args)
    me = p.me()
    raw, meta = p.threads()
    threads = [normalize_thread(t, me) for t in raw]
    if not args.all:
        threads = [t for t in threads if t["author_kind"] == "bot"]
    emit({"provider": p.name, "host": p.host or "github.com", "project": p.project, "number": p.number, "me": me, **meta,
          "thread_count": len(threads), "threads": threads})


def observe(p, thread_id: str, me: str, want_reply_id: str | None, want_reaction: str | None) -> dict:
    raw = p.thread(thread_id)
    t = normalize_thread(raw, me)
    reply_ok = None
    if want_reply_id:
        reply_ok = any(str(n["id"]) == str(want_reply_id) for n in raw["notes"][1:])
    reaction_ok = None
    if want_reaction and want_reaction != "none":
        reaction_ok = want_reaction in p.reactions_by(t["note_id"], me)
    return {"thread_id": thread_id, "note_id": t["note_id"], "resolved": t["resolved"], "resolvable": t["resolvable"],
            "reply_verified": reply_ok, "reaction_verified": reaction_ok, "my_reactions": p.reactions_by(t["note_id"], me) if want_reaction is None else None,
            "already_handled": t["already_handled"]}


def cmd_apply(args) -> None:
    p = make_provider(args)
    me = p.me()
    plan = normalize_plan(json.loads(Path(args.plan).read_text(encoding="utf-8")))
    raw_threads, meta = p.threads()
    by_id = {t["thread_id"]: t for t in raw_threads}
    ledger = []
    for item in plan:
        tid = item["thread_id"]
        raw = by_id.get(tid)
        if raw is None:
            ledger.append({**item, "status": "error", "error": "thread not found on remote"})
            continue
        t = normalize_thread(raw, me)
        row = {"thread_id": tid, "note_id": t["note_id"], "reviewer": t["reviewer"], "path": t["path"], "line": t["line"], "verdict": item["verdict"],
               "reply_id": None, "reply_action": "skip", "reaction": item["reaction"], "reaction_action": "skip",
               "resolve_requested": item["resolve"], "resolve_action": "skip", "reason_open": item["reason_open"]}
        actions = []
        if item["reply"] and t["already_handled"] != item["verdict"]:
            actions.append("reply")
        if item["reaction"] != "none" and item["reaction"] not in p.reactions_by(t["note_id"], me):
            actions.append("react")
        if item["resolve"] and not t["resolved"] and t["resolvable"]:
            actions.append("resolve")
        if item["resolve"] and not t["resolvable"]:
            row["resolve_action"] = "not_resolvable"
        if args.dry_run:
            row.update({"status": "dry_run", "planned": actions, "reply_preview": compose_reply(item["reply"], tid, item["verdict"], t["mention"]) if "reply" in actions else None})
            ledger.append(row)
            continue
        try:
            if "reply" in actions:
                row["reply_id"] = p.reply(raw, compose_reply(item["reply"], tid, item["verdict"], t["mention"]))
                row["reply_action"] = "posted"
            if "react" in actions:
                p.react(t["note_id"], item["reaction"])
                row["reaction_action"] = "added"
            if "resolve" in actions:
                p.set_resolved(t["resolve_id"], True)
                row["resolve_action"] = "resolved"
            obs = observe(p, tid, me, row["reply_id"], item["reaction"])
            row.update({"status": "ok", "observed": obs})
            if row["reply_id"] and obs["reply_verified"] is False:
                row["status"] = "unverified"
            if item["reaction"] != "none" and obs["reaction_verified"] is False:
                row["status"] = "unverified"
            if item["resolve"] and t["resolvable"] and not obs["resolved"]:
                row["status"] = "unverified"
        except SystemExit as e:
            row.update({"status": "error", "error": str(e)[:400]})
        ledger.append(row)
    emit({"provider": p.name, "project": p.project, "number": p.number, "me": me, **meta, "dry_run": bool(args.dry_run),
          "ok": all(r.get("status") in ("ok", "dry_run") for r in ledger), "ledger": ledger})


def cmd_reply(args) -> None:
    p = make_provider(args)
    raw = p.thread(args.thread)
    body = Path(args.body_file).read_text(encoding="utf-8")
    rid = p.reply(raw, body)
    emit({"thread_id": args.thread, "reply_id": rid, "verified": any(str(n["id"]) == rid for n in p.thread(args.thread)["notes"][1:])})


def cmd_react(args) -> None:
    p = make_provider(args)
    me = p.me()
    if args.reaction not in p.reactions_by(args.note, me):
        p.react(args.note, args.reaction)
    emit({"note_id": args.note, "reaction": args.reaction, "verified": args.reaction in p.reactions_by(args.note, me)})


def cmd_resolve(args, resolved: bool) -> None:
    p = make_provider(args)
    raw = p.thread(args.thread)
    if not raw["resolvable"] and resolved:
        raise SystemExit(f"thread {args.thread} is not resolvable")
    if raw["resolved"] != resolved:
        p.set_resolved(raw["resolve_id"], resolved)
    emit({"thread_id": args.thread, "resolved": p.thread(args.thread)["resolved"]})


def cmd_verify(args) -> None:
    p = make_provider(args)
    emit(observe(p, args.thread, p.me(), args.reply_id, args.reaction))


def main(argv: list[str] | None = None) -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    def common(sp):
        sp.add_argument("--pr", required=True, help="PR/MR number, !n, #n, or URL")
        sp.add_argument("--repo-dir", default=".")
        sp.add_argument("--provider", choices=["github", "gitlab"])
        sp.add_argument("--project", help="owner/repo or group/project")
        sp.add_argument("--hostname", help="GitHub Enterprise / self-managed GitLab host")

    s = sub.add_parser("list"); common(s); s.add_argument("--all", action="store_true", help="include human threads")
    s = sub.add_parser("apply"); common(s); s.add_argument("--plan", required=True); s.add_argument("--dry-run", action="store_true")
    s = sub.add_parser("reply"); common(s); s.add_argument("--thread", required=True); s.add_argument("--body-file", required=True)
    s = sub.add_parser("react"); common(s); s.add_argument("--note", required=True); s.add_argument("--reaction", required=True, choices=["up", "down"])
    s = sub.add_parser("resolve"); common(s); s.add_argument("--thread", required=True)
    s = sub.add_parser("unresolve"); common(s); s.add_argument("--thread", required=True)
    s = sub.add_parser("verify"); common(s); s.add_argument("--thread", required=True); s.add_argument("--reply-id"); s.add_argument("--reaction", choices=["up", "down", "none"])
    args = ap.parse_args(argv)
    {"list": cmd_list, "apply": cmd_apply, "reply": cmd_reply, "react": cmd_react,
     "resolve": lambda a: cmd_resolve(a, True), "unresolve": lambda a: cmd_resolve(a, False), "verify": cmd_verify}[args.cmd](args)


if __name__ == "__main__":
    main()
