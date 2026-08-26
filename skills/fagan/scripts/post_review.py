#!/usr/bin/env python3
"""Render and (optionally) post a Fagan review from findings.json to a GitLab MR or GitHub PR.

Usage:
  post_review.py --context DIR/context.json --findings DIR/findings.json [--post] [--force]
                 [--max-inline 8] [--min-confidence 80]

Default is DRY RUN: validates findings against the diff, renders every body to
DIR/rendered/ and prints them. With --post:
  gitlab → one inline discussion per finding (position from diff_refs) + one summary note
  github → one pull-request review (event from verdict) carrying all inline comments + body
then re-reads and prints a verification table. Refuses to post twice for the
same head_sha unless --force.

findings.json schema:
{
  "verdict": "approve|request_changes|comment",
  "summary": "<Korean: what the change does + overall risk read>",
  "verified_ok": ["<Korean: checked and found correct>", ...],
  "open_questions": ["<Korean: could not verify, worth asking>", ...],
  "rule_candidates": ["<Korean: durable lesson for .kody/rules / CAUTIONS>", ...],
  "findings": [{
      "id": "F1", "path": "apps/x/y.ts",
      "new_line": 59,            # must be a commentable line of the diff
      "end_line": 65,            # optional; multi-line committable suggestion range
      "severity": "critical|high|medium|low",
      "category": "bug|security|performance|business-logic|data|api-contract|test|rule|scope",
      "title": "<one Korean sentence>",
      "what": "<Korean>", "why": "<Korean concrete failure scenario>", "how": "<Korean fix>",
      "evidence": ["path:line — what it proves", ...],   # REQUIRED, non-empty
      "confidence": 0-100,
      "verification": "<test run / typecheck / trace that confirmed it>",
      "suggestion": "<replacement code for new_line..end_line, or null>",
      "rule": ".kody/rules/xxx.md (optional)"
  }]
}
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

SEV_ORDER = {"critical": 0, "high": 1, "medium": 2, "low": 3}
MIN_BY_SEV = {"critical": 50, "high": 50, "medium": 65, "low": 80}   # inline threshold by severity
GATE_INLINE_MAX = 1                                                    # deterministic gate findings inline; rest → table
SEV_BADGE = {
    "critical": "![critical](https://img.shields.io/badge/severity-critical-FF3D3D)",
    "high": "![high](https://img.shields.io/badge/severity-high-E65100)",
    "medium": "![medium](https://img.shields.io/badge/severity-medium-F9A825)",
    "low": "![low](https://img.shields.io/badge/severity-low-6B6B92)",
}
CAT_BADGE = "![{c}](https://img.shields.io/badge/{c}-1E88E5)"
BRAND = "![fagan](https://img.shields.io/badge/fagan-inspection-312B4B?labelColor=C9BBF2)"


CALL_TIMEOUT = int(os.environ.get("FAGAN_CALL_TIMEOUT", "90"))


def run(cmd: list[str], check: bool = True) -> str:
    try:
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=CALL_TIMEOUT)
    except subprocess.TimeoutExpired:
        raise SystemExit(f"command timed out after {CALL_TIMEOUT}s: {' '.join(cmd[:4])}... (remote slow? set FAGAN_CALL_TIMEOUT)")
    if check and p.returncode != 0:
        raise SystemExit(f"command failed ({p.returncode}): {' '.join(cmd[:4])}...\n{p.stderr.strip()[:500]}")
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


# ----------------------------------------------------------------------------- rendering

SECRET_RE = re.compile(r"(?i)([A-Za-z0-9_\-]*(?:token|secret|password|passwd|api[_-]?key|authorization|bearer|private[_-]?key|client[_-]?secret)[A-Za-z0-9_\-]*)(\s*[=:]\s*|\s+)([\"']?)([A-Za-z0-9_\-./+=]{8,})")
TOKEN_SHAPE_RE = re.compile(r"\b(glpat-[A-Za-z0-9_\-]{10,}|gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9\-]{16,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9\-]{10,}|eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{5,}\.[A-Za-z0-9_\-]{5,})\b")


def mask(text: str) -> str:
    out = SECRET_RE.sub(lambda m: f"{m.group(1)}{m.group(2)}{m.group(3)}***", text or "")
    return TOKEN_SHAPE_RE.sub("***", out)


def masked(f: dict) -> dict:
    g = dict(f)
    for k in ("title", "what", "why", "how", "verification", "suggestion"):
        if g.get(k):
            g[k] = mask(g[k])
    g["evidence"] = [mask(e) for e in g.get("evidence", [])]
    return g

def is_gate(f: dict) -> bool:
    return str(f.get("source", "")).startswith("gate:") or str(f.get("id", "")).startswith("G")


def render_gate_finding(f: dict) -> str:
    sev = f["severity"].lower()
    m = f.get("metrics") or {}
    parts = [f"{BRAND} {SEV_BADGE.get(sev, SEV_BADGE['low'])}", "", f"**{f['title']}**", ""]
    if m and "cyclomatic" in m:
        span = f"신규 {m.get('loc')} LOC" if not m.get("base_loc") else f"{m.get('base_loc')} → {m.get('loc')} LOC"
        parts.append(f"{span} · 복잡도≈{m.get('cyclomatic')} · 중첩 {m.get('nesting')} · 한도 {m.get('limit')} ({f.get('rule', 'CLAUDE.md')})")
    elif m:
        parts.append(f"{m.get('base_loc')} → {m.get('loc')} LOC · {m.get('kind')} 한도 {m.get('limit')} ({f.get('rule', 'CLAUDE.md')})")
    else:
        parts.append(f["what"])
    parts.append(f["how"])
    if f.get("evidence") and not m:
        parts += ["", "```", *f["evidence"][:8], "```"]
    parts += ["", f"<!-- fagan-finding id={f['id']} -->"]
    return "\n".join(parts)


def render_finding(f: dict, provider: str) -> str:
    if is_gate(f):
        return render_gate_finding(f)
    sev = f["severity"].lower()
    cat = f.get("category", "bug")
    parts = [f"{BRAND} {CAT_BADGE.format(c=cat)} {SEV_BADGE.get(sev, SEV_BADGE['medium'])}", "", f"**{f['title']}**", "",
             f"- **무엇**: {f['what']}", f"- **왜 문제**: {f['why']}", f"- **어떻게**: {f['how']}"]
    if f.get("suggestion"):
        start_l, end_l = int(f["new_line"]), int(f.get("end_line") or f["new_line"])
        fence = f"```suggestion:-0+{end_l - start_l}" if provider == "gitlab" else "```suggestion"
        parts += ["", fence, f["suggestion"].rstrip("\n"), "```"]
    if f.get("evidence"):
        parts += ["", "<details><summary>근거</summary>", ""]
        parts += [f"- {e}" for e in f["evidence"]]
        if f.get("verification"):
            parts.append(f"- 검증: {f['verification']}")
        parts += ["", "</details>"]
    llm = [f"File {f['path']}, line {f['new_line']}" + (f"-{f['end_line']}" if f.get("end_line") else "") + ":", "",
           f"WHAT: {f['what']}", f"WHY: {f['why']}", f"HOW: {f['how']}"]
    if f.get("upstream"):
        llm.append(f"UPSTREAM: {f['upstream']}")
    if f.get("downstream"):
        llm.append(f"DOWNSTREAM: {f['downstream']}")
    if f.get("evidence"):
        llm += ["EVIDENCE:"] + [f"- {e}" for e in f["evidence"][:6]]
    if f.get("suggestion"):
        llm += ["", "Suggested code:", f["suggestion"].rstrip("\n")]
    parts += ["", "<details><summary>LLM 에게 넘길 프롬프트</summary>", "", "```", *llm, "```", "", "</details>"]
    if f.get("rule"):
        parts += ["", f"<sub>규칙: `{f['rule']}`</sub>"]
    parts += [f"<!-- fagan-finding id={f['id']} -->"]
    return "\n".join(parts)


def norm_ok(x) -> str:
    """verified_ok entries are either strings or {concern, why_ok, loc, thread}."""
    if isinstance(x, dict):
        loc = f" ({x['loc']})" if x.get("loc") else ""
        tag = f" — Kody 스레드 오탐" if x.get("thread") else ""
        return f"**{x.get('concern', '').rstrip('?')}?**{loc} — {x.get('why_ok', '')}{tag}"
    return str(x)


def render_summary(ctx: dict, data: dict, posted: list[dict], skipped: list[dict], gate_md: str | None = None,
                   overflow: list[dict] | None = None, invalid: list[dict] | None = None, gate_table: list[dict] | None = None) -> str:
    head = ctx["diff_refs"]["head_sha"]
    verdict = data.get("verdict", "comment")
    n_inline = len(posted)
    verdict_ko = {"approve": "✅ 병합 가능", "request_changes": "🛑 수정 필요", "comment": "💬 병합 차단 없음"}.get(verdict, verdict)
    L = [f"## Fagan 리뷰 — {verdict_ko} (인라인 {n_inline})", "", (data.get("summary") or "").strip(), ""]
    if posted:
        L += [f"### 지적 ({len(posted)})", "", "| # | 심각도 | 분류 | 위치 | 요약 |", "|---|---|---|---|---|"]
        L += [f"| {f['id']} | {f['severity']} | {f.get('category', '')} | `{f['path']}:{f['new_line']}` | {f['title']} |" for f in posted]
        L.append("")
    ask = list(data.get("open_questions") or [])
    # medium+ findings that missed the inline bar are exposed as author questions, not hidden.
    for f in (skipped or []):
        if SEV_ORDER.get(f["severity"].lower(), 9) <= 2 and not f.get("pre_existing"):
            ask.append(f"`{f['path']}:{f['new_line']}` {f['title']} — {f['what']} (확신 {f.get('confidence', 0)}/100, 인라인 미게시)")
    if overflow:
        L += [f"### 인라인 상한을 넘어 여기에만 남긴 지적 ({len(overflow)})", ""]
        L += [f"- [{f['severity']}] `{f['path']}:{f['new_line']}` {f['title']} — {f['how']}" for f in overflow] + [""]
    if gate_table:
        L += [f"### 한도 초과 (자동 측정, 인라인 생략) ({len(gate_table)})", "", "| 위치 | 항목 | 측정 | 비고 |", "|---|---|---|---|"]
        for f in gate_table:
            m = f.get("metrics") or {}
            if m and "cyclomatic" in m:
                meas = (f"신규 {m.get('loc')} LOC" if not m.get("base_loc") else f"{m.get('base_loc')} → {m.get('loc')} LOC") + f", 복잡도≈{m.get('cyclomatic')}"
            elif m:
                meas = f"{m.get('base_loc')} → {m.get('loc')} LOC (한도 {m.get('limit')})"
            else:
                meas = f.get("what", "")
            note = "변경 전부터 초과" if f.get("pre_existing") else ("소폭 초과" if f.get("minor") else "이번 변경으로 초과")
            L.append(f"| `{f['path'].split('/')[-1]}:{f['new_line']}` | {f['title']} | {meas} | {note} |")
        L.append("")
    if gate_md:
        L += [gate_md.strip(), ""]
    if data.get("verified_ok"):
        L += ["### 검토했으나 문제 없음", ""] + [f"- {norm_ok(x)}" for x in data["verified_ok"]] + [""]
    if ask:
        L += ["### 저자 확인 요청", ""] + [f"- {x}" for x in ask] + [""]
    low = [f for f in (skipped or []) if not (SEV_ORDER.get(f["severity"].lower(), 9) <= 2 and not f.get("pre_existing")) and not is_gate(f)]
    if low:
        L += ["<details><summary>확신이 낮아 인라인으로 달지 않은 후보</summary>", ""]
        L += [f"- ({f.get('confidence', 0)}) `{f['path']}:{f['new_line']}` {f['title']}" for f in low] + ["", "</details>", ""]
    if invalid:
        L += ["<details><summary>위치 검증에 실패해 인라인으로 달지 못한 후보</summary>", ""]
        L += [f"- `{f.get('path')}:{f.get('new_line')}` {f.get('title')} — {f['invalid_reason']}" for f in invalid] + ["", "</details>", ""]
    if data.get("rule_candidates"):
        L += [f"<details><summary>다음 리뷰 규칙 제안 {len(data['rule_candidates'])}건 (별도 MR 로 반영)</summary>", ""] + [f"- {x}" for x in data["rule_candidates"]] + ["", "</details>", ""]
    L += [f"<sub>head `{head[:12]}` · 게시한 지적은 모두 정의·호출자 추적과 테스트/타입체크로 확인했습니다. 잘못된 지적은 👎 + 이유를 남겨 주세요. 다음 리뷰 규칙에 반영합니다.</sub>",
          f"<!-- fagan-review head={head} -->"]
    return "\n".join(L)


# ----------------------------------------------------------------------------- posting

def post_gitlab(ctx: dict, posted: list[dict], bodies: dict, summary_body: str) -> tuple[list[dict], bool]:
    host, proj, n, refs = ctx["hostname"], ctx["project_encoded"], ctx["number"], ctx["diff_refs"]
    results = []
    for f in posted:
        old_path = next((x.get("old_path") or f["path"]) for x in ctx["files"] if x["path"] == f["path"])
        body = {"body": bodies[f["id"]], "position": {"position_type": "text", "base_sha": refs["base_sha"], "start_sha": refs["start_sha"],
                                                       "head_sha": refs["head_sha"], "new_path": f["path"], "old_path": old_path, "new_line": int(f["new_line"])}}
        try:
            d = api("glab", host, f"projects/{proj}/merge_requests/{n}/discussions", "POST", body)
            results.append({"id": f["id"], "remote_id": d.get("id"), "ok": bool(d.get("id"))})
        except SystemExit as e:
            results.append({"id": f["id"], "remote_id": None, "ok": False, "error": str(e)[:300]})
    s = api("glab", host, f"projects/{proj}/merge_requests/{n}/notes", "POST", {"body": summary_body})
    discs = api("glab", host, f"projects/{proj}/merge_requests/{n}/discussions?per_page=100", paginate=True)
    seen = {d["id"] for d in discs}
    for r in results:
        r["verified"] = r["remote_id"] in seen
    marker_ok = any(f"fagan-review head={refs['head_sha']}" in (x.get("body") or "") for d in discs for x in d.get("notes", []))
    results.append({"id": "summary", "remote_id": s.get("id"), "ok": bool(s.get("id")), "verified": marker_ok})
    return results, marker_ok


def post_github(ctx: dict, posted: list[dict], bodies: dict, summary_body: str, verdict: str, allow_approve: bool = False) -> tuple[list[dict], bool]:
    host, proj, n, refs = ctx["hostname"], ctx["project"], ctx["number"], ctx["diff_refs"]
    event = {"approve": "APPROVE" if allow_approve else "COMMENT", "request_changes": "REQUEST_CHANGES"}.get(verdict, "COMMENT")
    comments = []
    for f in posted:
        c = {"path": f["path"], "line": int(f.get("end_line") or f["new_line"]), "side": "RIGHT", "body": bodies[f["id"]]}
        if f.get("end_line") and int(f["end_line"]) > int(f["new_line"]):
            c.update({"start_line": int(f["new_line"]), "start_side": "RIGHT"})
        comments.append(c)
    review = api("gh", host, f"repos/{proj}/pulls/{n}/reviews", "POST", {"commit_id": refs["head_sha"], "body": summary_body, "event": event, "comments": comments})
    rid = review.get("id")
    results = [{"id": "review", "remote_id": rid, "ok": bool(rid)}]
    marker_ok = False
    if rid:
        got = api("gh", host, f"repos/{proj}/pulls/{n}/reviews/{rid}")
        marker_ok = f"fagan-review head={refs['head_sha']}" in (got.get("body") or "")
        rc = api("gh", host, f"repos/{proj}/pulls/{n}/reviews/{rid}/comments?per_page=100", paginate=True)
        ids_seen = {m for c in rc for m in [c.get("body", "")]}
        for f in posted:
            results.append({"id": f["id"], "remote_id": next((c["id"] for c in rc if f"fagan-finding id={f['id']}" in c.get("body", "")), None),
                            "ok": True, "verified": any(f"fagan-finding id={f['id']}" in b for b in ids_seen)})
    results[0]["verified"] = marker_ok
    return results, marker_ok


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--context", required=True)
    ap.add_argument("--findings", required=True)
    ap.add_argument("--post", action="store_true")
    ap.add_argument("--force", action="store_true")
    ap.add_argument("--max-inline", type=int, default=8)
    ap.add_argument("--min-confidence", type=int, default=80, help="override the severity-weighted bar (critical/high 50, medium 65, low 80) with one number")
    ap.add_argument("--gate", help="gate.json from quality_gate.py; its table is embedded in the summary")
    ap.add_argument("--strict", action="store_true", help="fail on any invalid finding instead of demoting it to the summary")
    ap.add_argument("--allow-approve", action="store_true", help="github: allow event=APPROVE; default downgrades approve to COMMENT")
    a = ap.parse_args()

    ctx = json.loads(Path(a.context).read_text())
    data = json.loads(Path(a.findings).read_text())
    provider = ctx.get("provider", "gitlab")
    out_dir = Path(ctx["out_dir"]) / "rendered"
    out_dir.mkdir(parents=True, exist_ok=True)
    refs = ctx["diff_refs"]
    commentable = {f["path"]: set(f["commentable_new_lines"]) for f in ctx["files"]}
    hunks_of = {f["path"]: [(h["new_start"], h["new_start"] + h["new_len"] - 1) for h in f["hunks"]] for f in ctx["files"]}

    errors, eligible, skipped, invalid = [], [], [], []
    for f in data.get("findings", []):
        fid = f.get("id", "?")
        problems = []
        for k in ("id", "path", "new_line", "severity", "title", "what", "why", "how"):
            if k not in f:
                problems.append(f"missing field {k}")
        if f.get("path") not in commentable:
            problems.append(f"path not in diff: {f.get('path')}")
        elif int(f.get("new_line", -1)) not in commentable[f["path"]]:
            problems.append(f"line {f.get('new_line')} not a commentable diff line (hunks: {hunks_of[f['path']]})")
        elif f.get("end_line") and int(f["end_line"]) not in commentable[f["path"]]:
            problems.append(f"end_line {f['end_line']} not a commentable diff line")
        if f.get("end_line") and int(f["end_line"]) < int(f.get("new_line", 0)):
            problems.append("end_line < new_line")
        if f.get("severity", "").lower() not in SEV_ORDER:
            problems.append(f"severity must be one of {list(SEV_ORDER)}")
        if not f.get("evidence"):
            problems.append("evidence is empty — every finding must cite definitions/callers actually read")
        if problems:
            errors.append(f"{fid}: " + "; ".join(problems))
            if f.get("path") and f.get("title"):
                invalid.append(dict(f, invalid_reason="; ".join(problems)))
            continue
        sev = f["severity"].lower()
        conf = int(f.get("confidence", 0))
        bar = MIN_BY_SEV.get(sev, 80) if a.min_confidence == 80 else a.min_confidence
        if f.get("summary_only") or f.get("pre_existing") or f.get("minor"):
            skipped.append(f)
        elif conf >= bar or (f.get("skeptics_passed") and conf >= 50):
            eligible.append(f)
        else:
            skipped.append(f)
    if errors:
        print(("VALIDATION ERRORS (strict):" if a.strict else "VALIDATION WARNINGS (demoted to summary):") + "\n- " + "\n- ".join(errors), file=sys.stderr)
        if a.strict:
            sys.exit(2)
    eligible.sort(key=lambda f: (SEV_ORDER[f["severity"].lower()], -int(f.get("confidence", 0))))
    gate_el = sorted([f for f in eligible if is_gate(f)], key=lambda f: -((f.get("metrics") or {}).get("cyclomatic", 0) * 100 + (f.get("metrics") or {}).get("loc", 0)))
    gate_inline, gate_table = gate_el[:GATE_INLINE_MAX], gate_el[GATE_INLINE_MAX:] + [f for f in skipped if is_gate(f)]
    agent_el = [f for f in eligible if not is_gate(f)]
    posted = (agent_el + gate_inline)[: a.max_inline]
    overflow = [f for f in agent_el + gate_inline if f not in posted]
    posted = [masked(f) for f in posted]
    bodies = {f["id"]: render_finding(f, provider) for f in posted}
    gate_md = None
    if a.gate and Path(a.gate).exists():
        gm = Path(a.gate).with_suffix(".md")
        gate_md = gm.read_text() if gm.exists() else None
    summary_body = render_summary(ctx, data, posted, skipped, gate_md, overflow, invalid, gate_table)
    for fid, b in bodies.items():
        (out_dir / f"{fid}.md").write_text(b)
    (out_dir / "summary.md").write_text(summary_body)

    if not a.post:
        print(f"DRY RUN ({provider}) — rendered {len(bodies)} inline + summary to {out_dir}\n")
        for f in posted:
            print(f"===== {f['id']} {f['path']}:{f['new_line']} =====\n{bodies[f['id']]}\n")
        print(f"===== SUMMARY =====\n{summary_body}")
        return

    if ctx["eligibility"].get("already_reviewed_head") and not a.force:
        raise SystemExit(f"fagan review already posted for head {refs['head_sha'][:12]}; use --force to post again")
    # Live re-check right before writing.
    if provider == "gitlab":
        live = api("glab", ctx["hostname"], f"projects/{ctx['project_encoded']}/merge_requests/{ctx['number']}")
        live_state, live_head = live.get("state"), (live.get("diff_refs") or {}).get("head_sha")
    else:
        live = api("gh", ctx["hostname"], f"repos/{ctx['project']}/pulls/{ctx['number']}")
        live_state, live_head = ("opened" if live.get("state") == "open" else live.get("state")), (live.get("head") or {}).get("sha")
    if live_state != "opened" and not a.force:
        raise SystemExit(f"state is {live_state}; refusing to post (use --force)")
    if live_head != refs["head_sha"]:
        raise SystemExit("head moved since context was built; rebuild context (mr_context.py) and re-verify findings")

    if provider == "gitlab":
        results, marker_ok = post_gitlab(ctx, posted, bodies, summary_body)
    else:
        results, marker_ok = post_github(ctx, posted, bodies, summary_body, data.get("verdict", "comment"), a.allow_approve)

    print("| item | remote id | verified |")
    print("|---|---|---|")
    for r in results:
        print(f"| {r['id']} | {r.get('remote_id') or '-'} | {'yes' if r.get('verified') else 'NO ' + r.get('error', '')} |")
    Path(ctx["out_dir"], "post-receipt.json").write_text(json.dumps({"head_sha": refs["head_sha"], "results": results, "marker_verified": marker_ok}, indent=1))
    if not marker_ok or any(not r.get("ok") for r in results):
        sys.exit(1)


if __name__ == "__main__":
    main()
