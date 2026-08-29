#!/usr/bin/env python3
"""Parnas find+verify driver for Omo: `omo -p` agent fan-out on zai/glm-5.3-flash.

Host adapter prescribed by SKILL.md §2 ("Other hosts: dispatch the finderPrompt /
skepticPrompt strings from references/workflow.js with the host's sub-agent tool and
apply the prescreen and verdict rule in references/verification.md"). The prompt text
below is a port of workflow.js — workflow.js stays the single source of truth; if it
changes, re-sync this file. The verdict rule (prescreen → blind tracer → reproducer,
KILL ≥ 70, abstain ≤ 2 verdicts) is identical; only the budgets differ per profile.

Profiles:
  standard    workflow.js budgets (finder 10 turns, skeptic 8, args.maxCandidates).
  omo-flash   cheap-token host profile (default): finder 20 turns, skeptic 14,
              maxCandidates floored at 32, 10-way concurrency, thinking=high —
              GLM flash pricing buys wider search and longer traces, not a looser rule.

Usage:
  python3 scripts/omo_driver.py --args <out_dir>/workflow_args.json [--phase all]
      [--profile omo-flash] [--provider zai] [--model glm-5.3-flash]
      [--thinking-finder high] [--thinking-tracer high] [--thinking-reproducer medium]

Outputs <out_dir>/workflow-result.json (same shape workflow.js returns) and prints a
per-agent token/cost line read from the Omo session logs (sessions/*<sid>.jsonl).
"""
from __future__ import annotations

import argparse
import json
import math
import re
import subprocess
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

HOME = Path.home()
OMO_SESSIONS = HOME / ".omo" / "agent" / "sessions"
KILL = 70
SUPPRESS_EXEMPT = {"security", "data"}
ORDER = ["low", "medium", "high", "critical"]

PROFILES = {
    "standard": {"finder_turns": 10, "skeptic_turns": 8, "candidate_floor": 0,
                 "workers": {"finder": 6, "tracer": 6, "reproducer": 4},
                 "thinking": {"finder": "high", "tracer": "medium", "reproducer": "medium"}},
    # Cheap-token host profile: wider search + deeper traces to close the gap to the
    # Claude Code (opus) run. Verdict thresholds are NOT relaxed — recall grows, the
    # tracer/reproducer rule and the inline bars still own precision.
    "omo-flash": {"finder_turns": 24, "skeptic_turns": 18, "candidate_floor": 40,
                  "workers": {"finder": 10, "tracer": 10, "reproducer": 8},
                  "thinking": {"finder": "high", "tracer": "high", "reproducer": "high"}},
}
USAGE_KEYS = ("input", "output", "cacheRead", "cacheWrite", "reasoning", "totalTokens", "cost_total")


# ---------------------------------------------------------------- budgets

def resolve_budget(args_json: dict, profile: str) -> dict:
    p = PROFILES[profile]
    max_candidates = max(int(args_json.get("maxCandidates", 24)), p["candidate_floor"])
    lens_count = len(args_json.get("lenses") or []) or 1
    per_lens_cap = max(3, math.ceil(max_candidates / lens_count) + 1)
    return {"finder_turns": p["finder_turns"], "skeptic_turns": p["skeptic_turns"],
            "max_candidates": max_candidates, "per_lens_cap": per_lens_cap,
            "workers": p["workers"], "thinking": p["thinking"]}


# ---------------------------------------------------------------- prompt templates (port of workflow.js; keep in sync)

def finder_common(a: dict, cg: str) -> str:
    return f"""You are one inspector in a formal design inspection (Parnas active design review).
Repository checkout at head: {a['checkout']} (read-only; never edit, never checkout, never post).

How to work — this is a budget, not advice:
- Your pack file (named at the end of this message) is the whole context for your slice: the cumulative diff of every file you inspect, the definition headers enclosing each hunk, files that historically change together, the definitions and one-hop callers/callees of the symbols they define, the rules that apply, existing threads and prior lessons.
- Message 1: exactly one Read of the pack file, whole (no offset/limit) — nothing else. Message 2: every follow-up read you need, all in that one message (several Read/Bash calls at once), chosen from the hops the pack names. Then at most a few more messages. Hard cap: {a['finder_turns']} assistant messages including the final JSON message. One call per message wastes the budget. Read file regions (offset/limit or sed -n), never whole large files.
- You carry several lenses. Apply each one separately over the same pack and tag every candidate with the lens that found it (candidate.lens); a candidate two lenses would both report is reported once under the stronger lens. Lens-specific file scope: 'contract' looks at dto/controller/gateway/generated files, 'data' at db/repository/query code, 'async' at kafka/queue/stream/retry code, 'security' at auth/validation/secrets — skip files the lens does not apply to.
- Open the checkout only for a hop the pack names (a caller, a callee, a validator) or a symbol the pack does not list ({cg}). If {a['outDir']}/gate.md exists, lint/typecheck/test results and LOC metrics are already measured there — never re-report them.
- When the budget is nearly spent, stop and report what you verified; an unverified hunch is not a candidate.

Inspect ONLY through your lens. For every candidate defect you MUST, before reporting:
1. Open the real definition of every symbol the claim depends on — a diff hunk is not a definition.
2. Walk one hop upstream (who calls / what validates the input) and one hop downstream (who consumes the result) and state what you saw. If a hop is not findable (external library, dynamic dispatch), say so in upstream/downstream and cap confidence at 50 — do not guess it and do not silently drop the candidate.
3. Confirm the defect is on lines this change added/modified (hunks listed in the pack). If it is on an untouched line that the change makes newly reachable, set newly_reachable=true and say in why which changed line makes it reachable; otherwise the prescreen drops it.
Report a candidate only with a concrete failure scenario (input/state → wrong result). Return at most {a['per_lens_cap']} candidates PER LENS, strongest first — the cap is deliberate: the {a['per_lens_cap']} you can defend, not the first {a['per_lens_cap']} you noticed. Confidence follows the rubric (25 inferred / 50 verified-but-rare / 75 verified on a real path / 90+ proven with no escape upstream or downstream), not enthusiasm.
Do NOT report: style, naming, things a linter/typechecker/CI catches, pre-existing issues on untouched lines, speculative refactors, "consider adding" without a failure scenario, anything on a line that already has a thread in the pack unless you contradict that thread, anything listed under prior lessons.
verified_ok: things that looked risky and you cleared — {{"concern": "<우려, 한 구절>", "why_ok": "<왜 괜찮은지, 한 문장, 근거 파일:라인 포함>", "loc": "file:line", "thread": "<existing thread id you are contradicting, else null>"}}. Max 3; prefer ones that contradict an existing thread.
suggestion is REQUIRED when category is api-contract or the fix is a decorator/description/config one-liner.
All prose fields in Korean; identifiers/paths verbatim. evidence entries: "path:line — what it proves". Return lenses = the lens ids you applied.
Your FINAL message must be ONLY the JSON object matching the schema below — no markdown fences, no prose before or after.
Schema: {{"lenses":["<lens ids>"],"inspected":["<files/symbols read>"],"candidates":[{{"path":"...","new_line":N,"end_line":N|null,"severity":"critical|high|medium|low","category":"bug|security|performance|business-logic|data|api-contract|test|rule|scope","title":"...","what":"...","why":"...","how":"...","evidence":["..."],"upstream":"...","downstream":"...","suggestion":null,"rule":null,"confidence":0,"newly_reachable":false,"lens":"..."}}],"verified_ok":[]}}"""


def cg_hint(a: dict) -> str:
    return '`codegraph explore "<symbol>"` prints definitions + call paths' if a.get("codegraph") else "one `rg -n` call, then read the file region at the line it names"


def finder_prompt(a: dict, u: dict) -> str:
    lens_lines = "\n".join(f"- {l} — {a['lensText'][l]}" for l in u["lenses"])
    return f"""{finder_common(a, cg_hint(a))}

Your unit: {u['id']}
Pack file (message 1: one Read, whole — it lists the files in your slice): {a['outDir']}/{u['pack']}
Lenses:
{lens_lines}"""


SKEPTIC_TEXT = {
    "tracer": "Prove the claim rests on an inferred shape, an unchecked boundary, or a misreading of intent. (1) Open the real definition of every symbol in the claim. (2) Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto/schema constraints, caller preconditions) and downstream to the consumer of the result; report each hop as path:line. If any hop neutralises the scenario, refute. (3) Check the MR description (pack header) and the linked issue: is the behavior intentional AND correct with respect to what the issue asks? Intentional but wrong per the issue is still a defect — do not refute on intent alone.",
    "reproducer": "Prove the failure scenario cannot actually happen. Try to make it happen: write a throwaway unit test in the checkout encoding the scenario and run it, or run the targeted typecheck/lint on the file, or execute a small script. Paste command and outcome. Delete throwaway files afterwards. A scenario that cannot be reproduced and cannot be argued from definitions is refuted.",
}


def blind(c: dict) -> dict:
    return {k: c.get(k) for k in ("path", "new_line", "end_line", "severity", "category", "title", "what", "newly_reachable", "lens")}


def skeptic_prompt(a: dict, sid: str, c: dict, prior: dict | None) -> str:
    prior_txt = ""
    if prior:
        prior_txt = (f"\nThe tracer already examined it and did not refute (confidence {prior.get('confidence')}): "
                     f"{prior.get('reason')}. Do not repeat its trace; attack the scenario itself.")
    return f"""You are a skeptic in a design inspection. Your job is to try to REFUTE this candidate defect.
refuted=true ONLY when you actually neutralised the scenario with evidence (a definition, a boundary, a run that shows it cannot happen). If you could not test it (no runnable environment, missing DB, tooling absent) or could not find the hop, return refuted=false with confidence ≤ 40 and reason starting with "미확인:" — inability to verify is not a refutation.
Budget: at most {a['skeptic_turns']} assistant messages including the final JSON message; batch independent reads in one message.
Checkout (read-only except throwaway test files you delete afterwards): {a['checkout']}
Start here, in one message: {a['outDir']}/hunks/{hunk_slug(c['path'])}.patch (the diff of the candidate's file) and the "## <symbol>" sections of {a['outDir']}/defs.md for the symbols in the claim (grep -n "^## " to locate). Open other files only for a hop those name.
Lens: {sid} — {SKEPTIC_TEXT[sid]}
Candidate: {json.dumps(blind(c) if sid == 'tracer' else c, ensure_ascii=False)}{prior_txt}
severity_adjust other than "keep" is accepted only with an evidence entry (path:line) that justifies it.
Scoring: 0-25 inferred/pre-existing; 50 real but rare; 75 real on a real path; 90-100 reproduced or proven from definitions with no escape upstream/downstream.
reason in Korean; evidence entries "path:line — what it shows" or "<command> → <outcome>".
Your FINAL message must be ONLY the JSON object: {{"skeptic":"{sid}","refuted":true|false,"confidence":0,"reason":"...","evidence":["..."],"severity_adjust":"keep|lower|raise","corrected_line":null,"corrected_suggestion":null}}"""


# ---------------------------------------------------------------- helpers (port of workflow.js pure code)

def hunk_slug(p: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.\-]", "_", p.replace("/", "__"))


def bigrams(t: str) -> set:
    s = re.sub(r"[^\w]", "", t or "", flags=re.UNICODE)
    return {s[i:i + 2] for i in range(len(s) - 1)}


def similar(x: str | None, y: str | None) -> float:
    A, B = bigrams(x or ""), bigrams(y or "")
    if not A or not B:
        return 0.0
    return len(A & B) / max(1, min(len(A), len(B)))


TOK_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]{2,}|[가-힣]{2,}")


def tok_overlap(a: list[str | None], b: list[str | None]) -> float:
    A = {t.lower() for t in TOK_RE.findall(" ".join(x or "" for x in a))}
    B = {t.lower() for t in TOK_RE.findall(" ".join(x or "" for x in b))}
    if not A or not B:
        return 0.0
    return len(A & B) / min(len(A), len(B))


def prescreen(args_json: dict, c: dict) -> str | None:
    ranges = args_json.get("hunkRanges", {}).get(c.get("path"))
    if not ranges:
        # Unchanged file: only allowed when the finder marked the defect newly reachable
        # (it must name the changed line that makes it reachable in `why`). Dogfood
        # 2026-08-29 agentrq#347: the highest-value candidate (App.vue toast reading the
        # renamed key) set newly_reachable=true and was still killed here — same
        # short-circuit exists in workflow.js.
        if c.get("newly_reachable"):
            return None
        return f"prescreen: {c.get('path')} is not a changed file"
    line = c.get("new_line", -1)
    if not any(lo <= line <= hi for lo, hi in ranges) and not c.get("newly_reachable"):
        return f"prescreen: line {line} is outside every hunk of {c.get('path')} and not marked newly_reachable"
    if c.get("category") not in SUPPRESS_EXEMPT:
        for h in args_json.get("refutedHistory", []):
            if h.get("path") == c.get("path") and tok_overlap([h.get("title"), h.get("what")], [c.get("title"), c.get("what")]) >= 0.5:
                return f"prescreen: refuted before on this file — {(h.get('title') or '')[:120]}"
    return None


def dedup_candidates(finders: list[dict]) -> list[dict]:
    seen: list[dict] = []
    for r in finders:
        for c in r.get("candidates") or []:
            lens = c.get("lens") or (r.get("lenses") or ["logic"])[0]
            dup = None
            for x in seen:
                if (x.get("path") == c.get("path") and abs((x.get("new_line") or 0) - (c.get("new_line") or 0)) <= 2 and x.get("category") == c.get("category")) \
                        or similar(x.get("title"), c.get("title")) >= 0.5 \
                        or (x.get("rule") and x.get("rule") == c.get("rule") and similar(x.get("what"), c.get("what")) >= 0.4) \
                        or (x.get("category") == c.get("category") and similar(x.get("what"), c.get("what")) >= 0.5):
                    dup = x
                    break
            if dup:
                dup.setdefault("lenses", []).append(lens)
                dup["confidence"] = max(dup.get("confidence", 0), c.get("confidence", 0))
                if c.get("category") in SUPPRESS_EXEMPT:
                    dup["category"] = c["category"]
                dup["evidence"] = sorted({*(dup.get("evidence") or []), *(c.get("evidence") or [])})
                dup.setdefault("alternates", []).append({"path": c.get("path"), "new_line": c.get("new_line"), "title": c.get("title")})
                continue
            seen.append({**c, "lenses": [lens]})
    return seen


def severity_shift(sev: str, adj: str) -> str:
    i = ORDER.index(sev) if sev in ORDER else 1
    if adj == "lower":
        return ORDER[max(0, i - 1)]
    if adj == "raise":
        return ORDER[min(3, i + 1)]
    return sev


def extract_json(text: str | None):
    if not text:
        return None
    lo, hi = text.find("{"), text.rfind("}")
    if lo < 0 or hi <= lo:
        return None
    try:
        return json.loads(text[lo:hi + 1])
    except json.JSONDecodeError:
        return None


# ---------------------------------------------------------------- omo agent runner

class OmoRunner:
    """One agent = one `omo -p` process; usage is summed from the Omo session log."""

    def __init__(self, provider: str, model: str):
        self.provider, self.model = provider, model

    def _cmd(self, sid: str, thinking: str) -> list[str]:
        return ["omo", "--provider", self.provider, "--model", self.model, "--no-skills",
                "--no-context-files", "--no-extensions", "--permission-preset", "workspace",
                "--session-id", sid, "--thinking", thinking]

    def run(self, prompt: str, cwd: str, thinking: str, timeout: int = 1800) -> tuple[object, dict, str]:
        sid = uuid.uuid4().hex[:12]
        try:
            p = subprocess.run(self._cmd(sid, thinking) + ["-p", prompt], cwd=cwd,
                               capture_output=True, text=True, timeout=timeout)
        except subprocess.TimeoutExpired:
            return None, session_usage(sid), "omo timed out"
        parsed = extract_json(p.stdout)
        tail = p.stdout[-400:]
        if parsed is None:  # one retry, the analogue of agent() schema retry
            sid2 = sid + "r"
            try:
                p2 = subprocess.run(self._cmd(sid2, thinking) +
                                    ["-p", "이전 출력은 JSON 파싱에 실패했다. 요청된 스키마의 순수 JSON 오브젝트만 다시 출력하라. 마크다운 금지.\n\n" + p.stdout[-6000:]],
                                    cwd=cwd, capture_output=True, text=True, timeout=timeout)
                tail = p2.stdout[-400:]
                parsed = extract_json(p2.stdout)
            except subprocess.TimeoutExpired:
                pass
            usage = add_usage(session_usage(sid), session_usage(sid2))
        else:
            usage = session_usage(sid)
        return parsed, usage, tail


def session_usage(sid: str) -> dict:
    usage = {k: 0 for k in USAGE_KEYS}
    if not OMO_SESSIONS.exists():
        return usage
    for f in OMO_SESSIONS.rglob(f"*{sid}.jsonl"):
        try:
            for line in f.read_text(errors="ignore").splitlines():
                try:
                    o = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if o.get("type") != "message":
                    continue
                u = (o.get("message") or {}).get("usage") or {}
                for k in ("input", "output", "cacheRead", "cacheWrite", "reasoning", "totalTokens"):
                    usage[k] += u.get(k, 0) or 0
                cost = (u.get("cost") or {}).get("total")
                if cost:
                    usage["cost_total"] += cost
        except OSError:
            continue
    return usage


def add_usage(a: dict, b: dict) -> dict:
    return {k: (a.get(k, 0) or 0) + (b.get(k, 0) or 0) for k in USAGE_KEYS}


def run_batch(runner: OmoRunner, tasks: list[dict], workers: int) -> list[dict]:
    results: list[dict | None] = [None] * len(tasks)
    with ThreadPoolExecutor(max_workers=workers) as ex:
        futs = {ex.submit(runner.run, t["prompt"], t["cwd"], t["thinking"]): i for i, t in enumerate(tasks)}
        for fut in as_completed(futs):
            i = futs[fut]
            try:
                parsed, usage, tail = fut.result()
            except Exception as e:  # noqa: BLE001
                parsed, usage, tail = None, {k: 0 for k in USAGE_KEYS}, f"driver error: {e}"
            results[i] = {"parsed": parsed, "usage": usage, "tail": tail, "label": tasks[i]["label"]}
            print(f"[driver] {tasks[i]['label']}: {'OK' if parsed is not None else 'PARSE_FAIL'} "
                  f"(in {usage['input']:,} out {usage['output']:,} cacheRead {usage['cacheRead']:,} "
                  f"reasoning {usage['reasoning']:,} ${usage['cost_total']:.4f})", flush=True)
    return results


# ---------------------------------------------------------------- phases

def phase_find(a: dict, runner: OmoRunner) -> dict:
    units, cwd = a["units"], a["checkout"]
    usage_all = {k: 0 for k in USAGE_KEYS}
    finders: list[dict] = []

    def acc(u: dict):
        nonlocal usage_all
        usage_all = add_usage(usage_all, u)

    print(f"[driver] {len(units)} finder units (first alone to warm the provider cache, rest ×{a['workers']['finder']})", flush=True)
    t0 = [units[0]]
    r0 = run_batch(runner, [{"prompt": finder_prompt(a, u), "cwd": cwd, "thinking": a["thinking"]["finder"],
                             "label": f"find:{u['id']}"} for u in t0], 1)[0]
    acc(r0["usage"])
    if r0["parsed"]:
        finders.append({**r0["parsed"], "unit": units[0]["id"], "lenses": units[0]["lenses"]})
    rest = units[1:]
    if rest:
        for r, u in zip(run_batch(runner, [{"prompt": finder_prompt(a, x), "cwd": cwd, "thinking": a["thinking"]["finder"],
                                            "label": f"find:{x['id']}"} for x in rest], a["workers"]["finder"]), rest):
            acc(r["usage"])
            if r["parsed"]:
                finders.append({**r["parsed"], "unit": u["id"], "lenses": u["lenses"]})

    seen = dedup_candidates(finders)
    candidates = sorted(seen, key=lambda c: -c.get("confidence", 0))
    prescreened, kept = [], []
    for c in candidates:
        why = prescreen(a, c)
        (prescreened if why else kept).append(
            {**c, **({"verdicts": [{"skeptic": "prescreen", "refuted": True, "confidence": 90, "reason": why, "evidence": [], "severity_adjust": "keep"}]} if why else {})})
    if len(kept) > a["max_candidates"]:
        print(f"[driver] dropping {len(kept) - a['max_candidates']} lowest-confidence candidates (cap {a['max_candidates']})")
        kept = kept[: a["max_candidates"]]
    print(f"[driver] finders={len(finders)}/{len(units)} unique={len(seen)} prescreened={len(prescreened)} to_verify={len(kept)}", flush=True)
    return {"finders": finders, "verified_ok": [v for r in finders for v in (r.get("verified_ok") or [])],
            "candidates": kept, "prescreened": prescreened, "usage_find": usage_all}


def phase_verify(a: dict, runner: OmoRunner, found: dict) -> dict:
    cwd, candidates = a["checkout"], found["candidates"]
    usage_all = found["usage_find"]
    print(f"[driver] verify: {len(candidates)} tracers (×{a['workers']['tracer']})", flush=True)
    tasks = [{"prompt": skeptic_prompt(a, "tracer", c, None), "cwd": cwd, "thinking": a["thinking"]["tracer"],
              "label": f"tracer:{c['path'].split('/')[-1]}:{c.get('new_line')}"} for c in candidates]
    tracer_results = run_batch(runner, tasks, a["workers"]["tracer"]) if tasks else []

    repro_tasks, repro_idx, stages, skipped = [], [], [], 0
    for c, tr in zip(candidates, tracer_results):
        usage_all = add_usage(usage_all, tr["usage"])
        t = tr["parsed"]
        stages.append({"candidate": c, "tracer": t})
        if t and t.get("refuted") and t.get("confidence", 0) >= KILL:
            skipped += 1
        else:
            repro_idx.append(len(stages) - 1)
            repro_tasks.append({"prompt": skeptic_prompt(a, "reproducer", c, t), "cwd": cwd,
                                "thinking": a["thinking"]["reproducer"],
                                "label": f"repro:{c['path'].split('/')[-1]}:{c.get('new_line')}"})
    if repro_tasks:
        print(f"[driver] {len(repro_tasks)} reproducers (×{a['workers']['reproducer']}, tracer killed {skipped})", flush=True)
        for idx, r in zip(repro_idx, run_batch(runner, repro_tasks, a["workers"]["reproducer"])):
            usage_all = add_usage(usage_all, r["usage"])
            stages[idx]["reproducer"] = r["parsed"]

    findings, refuted = [], list(found["prescreened"])
    for s in stages:
        verdicts = [v for v in (s.get("tracer"), s.get("reproducer")) if v]
        c = s["candidate"]
        if any(v.get("refuted") and v.get("confidence", 0) >= KILL for v in verdicts):
            refuted.append({**c, "verdicts": verdicts})
            continue
        if len(verdicts) < 2:
            print(f"[driver] abstain: {c['path']}:{c.get('new_line')} had {len(verdicts)} verdict(s); kept at confidence 50")
            findings.append({**c, "confidence": min(50, c.get("confidence", 50)), "verification": "skeptics unavailable (abstain)"})
            continue
        keep = [v for v in verdicts if not v.get("refuted")]
        if len(keep) < 2:
            refuted.append({**c, "verdicts": verdicts})
            continue
        confidence = min([c.get("confidence", 50)] + [v.get("confidence", 50) for v in keep])
        adj = next((v.get("severity_adjust") for v in keep if v.get("evidence") and v.get("severity_adjust") != "keep"), "keep")
        corrected = next((v for v in keep if v.get("corrected_line")), None)
        corrected_sug = next((v for v in keep if v.get("corrected_suggestion")), None)
        findings.append({**c, "severity": severity_shift(c.get("severity", "medium"), adj), "confidence": confidence,
                         "skeptics_passed": True,
                         "new_line": corrected["corrected_line"] if corrected else c.get("new_line"),
                         "suggestion": corrected_sug["corrected_suggestion"] if corrected_sug else c.get("suggestion"),
                         "verification": " / ".join(f"{v.get('skeptic')}: {(v.get('reason') or '')[:80]}" for v in keep),
                         "evidence": list({*(c.get("evidence") or []), *[e for v in keep for e in (v.get("evidence") or [])]})[:10]})

    partial = []
    for r in refuted:
        residual = [f"{v['skeptic']}({v.get('confidence')}): {v.get('reason')}" for v in r.get("verdicts", [])
                    if not v.get("refuted") and v.get("confidence", 0) >= KILL]
        if residual:
            partial.append({"path": r.get("path"), "new_line": r.get("new_line"), "title": r.get("title"),
                            "killed_by": [f"{v['skeptic']}({v.get('confidence')}): {v.get('reason')}"
                                          for v in r.get("verdicts", []) if v.get("refuted")],
                            "residual": residual})
    refuted_for_history = [{"path": r.get("path"), "new_line": r.get("new_line"), "title": r.get("title"),
                            "what": (r.get("what") or "")[:300], "category": r.get("category"),
                            "killed_by": [f"{v['skeptic']}({v.get('confidence')}): {(v.get('reason') or '')[:200]}"
                                          for v in r.get("verdicts", []) if v.get("refuted")]}
                           for r in refuted
                           if any(v.get("refuted") and v.get("skeptic") != "prescreen" and v.get("confidence", 0) >= 80
                                  for v in r.get("verdicts", []))]
    print(f"[driver] {len(findings)} confirmed, {len(refuted)} refuted ({len(found['prescreened'])} by prescreen, "
          f"{skipped} reproducers skipped, {len(partial)} residual risk)", flush=True)
    return {"findings": findings, "refuted": refuted, "partial": partial,
            "refuted_for_history": refuted_for_history, "verified_ok": found["verified_ok"],
            "inspected": [{"unit": r["unit"], "lenses": r["lenses"], "inspected": r.get("inspected", [])}
                          for r in found["finders"]],
            "cost": {"finders": len(found["finders"]), "candidates_in": len(candidates), "tracers": len(candidates),
                     "reproducers": len(repro_tasks), "reproducers_skipped": skipped,
                     "prescreened": len(found["prescreened"]), "usage": usage_all,
                     "profile": a["profile"], "model": f"{a['provider']}/{a['model']}"}}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--args", required=True, help="workflow_args.json written by mr_context.py")
    ap.add_argument("--phase", choices=["find", "verify", "all"], default="all")
    ap.add_argument("--profile", choices=sorted(PROFILES), default="omo-flash")
    ap.add_argument("--provider", default="zai")
    ap.add_argument("--model", default="glm-5.3-flash")
    ap.add_argument("--thinking-finder"), ap.add_argument("--thinking-tracer"), ap.add_argument("--thinking-reproducer")
    ns = ap.parse_args()

    args_json = json.loads(Path(ns.args).read_text())
    budget = resolve_budget(args_json, ns.profile)
    thinking = budget["thinking"]
    if ns.thinking_finder:
        thinking["finder"] = ns.thinking_finder
    if ns.thinking_tracer:
        thinking["tracer"] = ns.thinking_tracer
    if ns.thinking_reproducer:
        thinking["reproducer"] = ns.thinking_reproducer
    a = {**args_json, **budget, "thinking": thinking, "profile": ns.profile,
         "provider": ns.provider, "model": ns.model}
    print(f"[driver] profile={ns.profile} model={ns.provider}/{ns.model} finder_turns={budget['finder_turns']} "
          f"skeptic_turns={budget['skeptic_turns']} maxCandidates={budget['max_candidates']} "
          f"perLensCap={budget['per_lens_cap']} workers={budget['workers']}", flush=True)
    t0 = time.time()
    runner = OmoRunner(ns.provider, ns.model)
    found = phase_find(a, runner)
    if ns.phase == "find":
        (Path(a["outDir"]) / "find-stage.json").write_text(json.dumps(found, ensure_ascii=False, indent=1))
        return 0
    result = phase_verify(a, runner, found)
    out = Path(a["outDir"]) / "workflow-result.json"
    out.write_text(json.dumps(result, ensure_ascii=False, indent=1))
    u = result["cost"]["usage"]
    print(f"[driver] done in {time.time() - t0:.0f}s → {out}")
    print(f"[driver] TOKENS: input={u['input']:,} output={u['output']:,} cacheRead={u['cacheRead']:,} "
          f"cacheWrite={u['cacheWrite']:,} reasoning={u['reasoning']:,} cost=${u['cost_total']:.4f}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
