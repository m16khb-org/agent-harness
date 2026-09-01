#!/usr/bin/env python3
"""Parnas find+verify driver for Omo: `omo -p` agent fan-out on zai/glm-5.3-flash.

Host adapter prescribed by SKILL.md §2 ("Other hosts: dispatch the finderPrompt /
skepticPrompt strings from references/workflow.js with the host's sub-agent tool and
apply the prescreen and verdict rule in references/verification.md"). The prompt text
below is a port of workflow.js — workflow.js stays the single source of truth; if it
changes, re-sync this file. The verdict rule (prescreen → blind tracer → reproducer,
KILL ≥ 70, low-confidence non-refutations abstain) is identical; only the budgets
differ per profile.

Profiles:
  standard    workflow.js budgets (finder 10 turns, skeptic 8, args.maxCandidates).
  omo-flash   cheap-token host profile (default): finder 24 turns, skeptic 18,
              maxCandidates floored at 40, 10-way concurrency, thinking=high —
              GLM flash pricing buys wider search and longer traces, not a looser rule.

Usage:
  python3 scripts/omo_driver.py --args <out_dir>/workflow_args.json [--phase all]
      [--profile omo-flash] [--provider zai] [--model glm-5.3-flash]
      [--thinking-finder high] [--thinking-tracer high] [--thinking-reproducer medium]
  python3 scripts/omo_driver.py --args <out_dir>/workflow_args.json --phase verify
      --retry-degraded-from <out_dir>/workflow-result.json
  python3 scripts/omo_driver.py --args <out_dir>/workflow_args.json --phase verify
      --retry-failed-units

Outputs <out_dir>/workflow-result.json (or workflow-retry-result.json for a degraded-only
retry) and prints a per-agent token/cost line read from the Omo session logs
(sessions/*<sid>.jsonl).
Verification refuses an incomplete find-stage. --retry-failed-units reruns only missing,
failed, or duplicate-coverage finder units, rewrites find-stage.json, and proceeds only when
the rebuilt stage is complete.
Provider 429s use dense staged backoff (5s→10s→15s→20s→30s→40s→50s→60s→60s,
jittered, up to 10 attempts per
agent) and the engine's model fallback is force-disabled through SENPI_NO_FALLBACK=1, so a
rate-limited agent fails on the pinned model as `rate_limited` instead of silently switching
providers or being mislabeled as a format error.
The provider and model are pinned; non-pinned values are rejected before any agent starts.
"""
from __future__ import annotations

import argparse
import json
import math
import os
import random
import re
import signal
import shutil
import subprocess
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

HOME = Path.home()
OMO_SESSIONS = HOME / ".omo" / "agent" / "sessions"
PARNAS_OUTPUT_EXTENSION = Path(__file__).parents[1] / "extensions" / "structured_output.js"
KILL = 70
UNVERIFIED_MAX_CONFIDENCE = 40
SUPPRESS_EXEMPT = {"security", "data"}
ORDER = ["low", "medium", "high", "critical"]
PINNED_PROVIDER = "zai"
PINNED_MODEL = "glm-5.3-flash"
PINNED_MODEL_REF = f"{PINNED_PROVIDER}/{PINNED_MODEL}"
PERMISSION_PRESETS = {"read-only", "workspace"}
VALID_SEVERITIES = set(ORDER)
VALID_CATEGORIES = {"bug", "security", "performance", "business-logic", "data", "api-contract", "test", "rule", "scope"}
VALID_SEVERITY_ADJUSTMENTS = {"keep", "lower", "raise"}
READ_ONLY_TOOLS = ("read", "grep", "find", "ls")
REPRODUCER_TOOLS = READ_ONLY_TOOLS + ("bash", "edit", "write")
FAILURE_COUNT_KEYS = ("parse_failure", "schema_failure", "timeout", "process_failure",
                      "model_mismatch", "rate_limited", "low_confidence_abstain", "coverage_gap")
OUTPUT_TOOL_BY_KIND = {"finder": "submit_parnas_finder", "verdict": "submit_parnas_verdict"}
# Provider rate limits (shared-account 429 bursts) use staged backoff with
# ±25% jitter while keeping ten attempts responsive for short provider bursts.
RATE_LIMIT_MARKERS = ("429", "rate limit", "ratelimit", "usage limit", "gousagelimiterror")
RATE_LIMIT_BACKOFF_SECONDS = (5.0, 10.0, 15.0, 20.0, 30.0, 40.0, 50.0, 60.0, 60.0)
MAX_AGENT_ATTEMPTS = 10

PROFILES = {
    "standard": {"finder_turns": 10, "skeptic_turns": 8, "candidate_floor": 0,
                 "workers": {"finder": 6, "tracer": 6, "reproducer": 4},
                 "thinking": {"finder": "high", "tracer": "medium", "reproducer": "medium"}},
    # Cheap-token host profile: wider search + deeper traces to close the gap to the
    # Claude Code (opus) run. Verdict thresholds are NOT relaxed — recall grows, the
    # tracer/reproducer rule and the inline bars still own precision. Concurrency stays
    # high: transient 429s come from shared-account bursts, and the per-agent backoff
    # retry absorbs them (measured 2026-08-29: mass agent death was zai 429 + fallback
    # onto a weekly-dead opencode plan, not an inherent provider cap).
    "omo-flash": {"finder_turns": 24, "skeptic_turns": 18, "candidate_floor": 40,
                  "workers": {"finder": 10, "tracer": 10, "reproducer": 8},
                  "thinking": {"finder": "high", "tracer": "high", "reproducer": "high"}},
}
USAGE_KEYS = ("input", "output", "cacheRead", "cacheWrite", "reasoning", "totalTokens", "cost_total")


def canonical_path(value: str | Path) -> str:
    return str(Path(value).expanduser().resolve())


def canonicalize_runtime_paths(a: dict) -> dict:
    for key in ("checkout", "outDir", "omoInputDir"):
        if a.get(key):
            a[key] = canonical_path(a[key])
    return a


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

def prompt_root(a: dict) -> str:
    return canonical_path(a.get("omoInputDir") or a["outDir"])


def finder_common(a: dict, cg: str) -> str:
    root = prompt_root(a)
    checkout = canonical_path(a["checkout"])
    return f"""You are one inspector in a formal design inspection (Parnas active design review).
Repository checkout at head: {checkout} (read-only; never edit, never checkout, never post).

How to work — this is a budget, not advice:
- Your pack file (named at the end of this message) is the whole context for your slice: the cumulative diff of every file you inspect, the definition headers enclosing each hunk, files that historically change together, the definitions and one-hop callers/callees of the symbols they define, the rules that apply, existing threads and prior lessons.
- Message 1: exactly one read of the pack file, whole (no offset/limit) — nothing else. Message 2: every follow-up read/search you need, all in that one message, chosen from the hops the pack names. Then at most a few more messages. Hard cap: {a['finder_turns']} assistant messages including the final JSON message. One call per message wastes the budget. Read file regions, never whole large files.
- Allowed tools: read, grep, find, ls. Forbidden tools: bash, eval, webfetch, edit, write, bash_output. Do not attempt a forbidden tool.
- Reserve the final assistant message for the JSON object. Stop all investigation and tool use one message before the cap; the final message must contain no tool call.
- You carry several lenses. Apply each one separately over the same pack and tag every candidate with the lens that found it (candidate.lens); a candidate two lenses would both report is reported once under the stronger lens. Lens-specific file scope: 'contract' looks at dto/controller/gateway/generated files, 'data' at db/repository/query code, 'async' at kafka/queue/stream/retry code, 'security' at auth/validation/secrets — skip files the lens does not apply to.
- Open the checkout only for a hop the pack names (a caller, a callee, a validator) or a symbol the pack does not list ({cg}). If {root}/gate.md exists, lint/typecheck/test results and LOC metrics are already measured there — never re-report them.
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
reviewed_files is a coverage receipt: copy every file path listed in your unit's pack exactly once,
even when it produced no candidate. Do not put symbols or extra paths in reviewed_files; inspected
may still list files and symbols actually investigated in depth.
Your FINAL action must be exactly one submit_parnas_finder tool call matching the schema below. Do not print the result as assistant text.
Schema: {{"lenses":["<lens ids>"],"reviewed_files":["<every exact unit file path, once>"],"inspected":["<files/symbols read in depth>"],"candidates":[{{"path":"...","new_line":N,"end_line":N|null,"severity":"critical|high|medium|low","category":"bug|security|performance|business-logic|data|api-contract|test|rule|scope","title":"...","what":"...","why":"...","how":"...","evidence":["..."],"upstream":"...","downstream":"...","suggestion":null,"rule":null,"confidence":0,"newly_reachable":false,"lens":"..."}}],"verified_ok":[]}}"""


def cg_hint(a: dict) -> str:
    return "one grep tool call, then read the file region at the line it names"


def finder_prompt(a: dict, u: dict) -> str:
    lens_lines = "\n".join(f"- {l} — {a['lensText'][l]}" for l in u["lenses"])
    root = prompt_root(a)
    return f"""{finder_common(a, cg_hint(a))}

Your unit: {u['id']}
Pack file (message 1: one Read, whole — it lists the files in your slice): {root}/{u['pack']}
Lenses:
{lens_lines}"""


SKEPTIC_TEXT = {
    "tracer": "Prove the claim rests on an inferred shape, an unchecked boundary, or a misreading of intent. (1) Open the real definition of every symbol in the claim. (2) Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto/schema constraints, caller preconditions) and downstream to the consumer of the result; report each hop as path:line. If any hop neutralises the scenario, refute. (3) Check the MR description (pack header) and the linked issue: is the behavior intentional AND correct with respect to what the issue asks? Intentional but wrong per the issue is still a defect — do not refute on intent alone.",
    "reproducer": "Prove the failure scenario cannot actually happen. Try to make it happen: write a throwaway unit test inside the checkout encoding the scenario and run it, or run the targeted typecheck/lint on the file, or execute a small script from the checkout. Paste command and outcome. Delete throwaway files afterwards. A scenario that cannot be reproduced and cannot be argued from definitions is refuted.",
}


def blind(c: dict) -> dict:
    return {k: c.get(k) for k in ("path", "new_line", "end_line", "severity", "category", "title", "what", "why",
                                  "newly_reachable", "lens")}


def skeptic_prompt(a: dict, sid: str, c: dict, prior: dict | None) -> str:
    prior_txt = ""
    if prior:
        prior_txt = (f"\nThe tracer already examined it and did not refute (confidence {prior.get('confidence')}): "
                     f"{prior.get('reason')}. Do not repeat its trace; attack the scenario itself.")
    root = prompt_root(a)
    checkout = canonical_path(a["checkout"])
    if sid == "tracer":
        tools = ("Allowed tools: read, grep, find, ls. Forbidden tools: bash, eval, webfetch, edit, write, "
                 "bash_output. Do not attempt a forbidden tool.")
    else:
        tools = ("Allowed tools: read, grep, find, ls, bash, edit, write. Forbidden tools: eval, webfetch, "
                 "bash_output. Never use /tmp, bash_output, background commands, detached processes, or paths "
                 "outside the checkout; run every command in the foreground with cwd inside the checkout.")
    return f"""You are a skeptic in a design inspection. Your job is to try to REFUTE this candidate defect.
refuted=true ONLY when you actually neutralised the scenario with evidence (a definition, a boundary, a run that shows it cannot happen). If you could not test it (no runnable environment, missing DB, tooling absent) or could not find the hop, return refuted=false with confidence ≤ 40 and reason starting with "미확인:" — inability to verify is not a refutation.
Budget: at most {a['skeptic_turns']} assistant messages including the final JSON message; batch independent reads in one message.
{tools}
Reserve the final assistant message for the JSON object. Stop all investigation and tool use one message before the cap; the final message must contain no tool call.
Checkout (read-only except throwaway test files you delete afterwards): {checkout}
Start here, in one message: {root}/hunks/{hunk_slug(c['path'])}.patch (the diff of the candidate's file) and the "## <symbol>" sections of {root}/defs.md for the symbols in the claim (use the grep tool to locate them). Open other files only for a hop those name.
Lens: {sid} — {SKEPTIC_TEXT[sid]}
Candidate: {json.dumps(blind(c) if sid == 'tracer' else c, ensure_ascii=False)}{prior_txt}
severity_adjust other than "keep" is accepted only with an evidence entry (path:line) that justifies it.
Scoring: 0-25 inferred/pre-existing; 50 real but rare; 75 real on a real path; 90-100 reproduced or proven from definitions with no escape upstream/downstream.
reason in Korean; evidence entries "path:line — what it shows" or "<command> → <outcome>".
Your FINAL action must be exactly one submit_parnas_verdict tool call with this object. Do not print it as assistant text:
{{"skeptic":"{sid}","refuted":true|false,"confidence":0,"reason":"...","evidence":["..."],"severity_adjust":"keep|lower|raise","corrected_line":null,"corrected_suggestion":null}}"""


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


def select_retry_candidates(candidates: list[dict], previous_result: dict) -> list[dict]:
    retry_keys = {
        (finding.get("path"), finding.get("new_line"))
        for finding in previous_result.get("findings") or []
        if finding.get("verification") == "skeptics unavailable (abstain)"
        and not finding.get("skeptics_passed")
    }
    return [candidate for candidate in candidates
            if (candidate.get("path"), candidate.get("new_line")) in retry_keys]


def dedup_candidates(finders: list[dict]) -> list[dict]:
    seen: list[dict] = []
    for r in finders:
        for c in r.get("candidates") or []:
            if not isinstance(c, dict):
                continue
            lens = c.get("lens") or (r.get("lenses") or ["logic"])[0]
            dup = None
            for x in seen:
                if (x.get("path") == c.get("path") and abs((x.get("new_line") or 0) - (c.get("new_line") or 0)) <= 2 and x.get("category") == c.get("category")) \
                        or similar(x.get("title"), c.get("title")) >= 0.5 \
                        or (x.get("rule") and x.get("rule") == c.get("rule") and similar(x.get("what"), c.get("what")) >= 0.4) \
                        or (x.get("category") == c.get("category") and similar(x.get("why"), c.get("why")) >= 0.5):
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


def coverage_report(units: list[dict], finders: list[dict]) -> dict:
    """Compare deterministic unit assignments with explicit finder file receipts."""
    by_unit = {finder.get("unit"): finder for finder in finders}
    gaps, duplicates, unexpected = [], [], []
    expected_assignments = covered_assignments = 0
    for unit in units:
        expected = list(dict.fromkeys(unit.get("files") or []))
        expected_set = set(expected)
        reviewed = (by_unit.get(unit.get("id")) or {}).get("reviewed_files") or []
        counts = {path: reviewed.count(path) for path in set(reviewed)}
        missing = [path for path in expected if counts.get(path, 0) == 0]
        repeated = sorted(path for path in expected_set if counts.get(path, 0) > 1)
        extra = sorted(path for path in counts if path not in expected_set)
        expected_assignments += len(expected)
        covered_assignments += len(expected) - len(missing)
        if missing:
            gaps.append({"unit": unit.get("id"), "missing_files": missing})
        if repeated:
            duplicates.append({"unit": unit.get("id"), "files": repeated})
        if extra:
            unexpected.append({"unit": unit.get("id"), "files": extra})
    return {
        "expected_assignments": expected_assignments,
        "covered_assignments": covered_assignments,
        "gaps": gaps,
        "duplicates": duplicates,
        "unexpected": unexpected,
        "complete": not gaps and not duplicates,
    }


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
    decoder = json.JSONDecoder()
    parsed = None
    for start, char in enumerate(text):
        if char != "{":
            continue
        try:
            value, _ = decoder.raw_decode(text[start:])
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            parsed = value
    return parsed


def prepare_omo_inputs(a: dict) -> Path:
    """Copy review inputs under the Omo worker's checkout permission boundary."""
    canonicalize_runtime_paths(a)
    source = Path(a["outDir"])
    checkout = Path(a["checkout"])
    input_dir = checkout / f".parnas-input-{uuid.uuid4().hex}"
    input_dir.mkdir()
    try:
        for directory in ("pack", "hunks"):
            shutil.copytree(source / directory, input_dir / directory)
        for filename in ("defs.md", "gate.md"):
            path = source / filename
            if path.is_file():
                shutil.copy2(path, input_dir / filename)
        for path in input_dir.rglob("*"):
            if path.is_file():
                path.chmod(0o444)
    except Exception:
        shutil.rmtree(input_dir)
        raise
    a["omoInputDir"] = str(input_dir)
    return input_dir


def cleanup_omo_inputs(input_dir: Path) -> None:
    """Remove temporary review inputs after every Omo worker has finished."""
    for path in input_dir.rglob("*"):
        if path.is_file():
            path.chmod(0o644)
    shutil.rmtree(input_dir)


def _is_int(value: object) -> bool:
    return isinstance(value, int) and not isinstance(value, bool)


def _valid_candidate(value: object) -> bool:
    if not isinstance(value, dict):
        return False
    required = ("path", "new_line", "severity", "category", "title", "what", "why", "evidence", "confidence", "lens")
    if any(key not in value for key in required):
        return False
    if not isinstance(value["path"], str) or not _is_int(value["new_line"]):
        return False
    if value.get("end_line") is not None and not _is_int(value["end_line"]):
        return False
    if value["severity"] not in VALID_SEVERITIES or value["category"] not in VALID_CATEGORIES:
        return False
    if not all(isinstance(value[key], str) for key in ("title", "what", "why", "lens")):
        return False
    if "newly_reachable" in value and not isinstance(value["newly_reachable"], bool):
        return False
    for key in ("upstream", "downstream", "suggestion", "rule"):
        if key in value and value[key] is not None and not isinstance(value[key], str):
            return False
    return isinstance(value["evidence"], list) and all(isinstance(item, str) for item in value["evidence"]) \
        and _is_int(value["confidence"])


def _valid_verified_ok(value: object) -> bool:
    return isinstance(value, dict) and all(isinstance(value.get(key), str) for key in ("concern", "why_ok", "loc")) \
        and (value.get("thread") is None or isinstance(value.get("thread"), str))


def _valid_verdict(value: object) -> bool:
    if not isinstance(value, dict):
        return False
    required = ("skeptic", "refuted", "confidence", "reason", "evidence", "severity_adjust")
    if any(key not in value for key in required):
        return False
    if not isinstance(value["skeptic"], str) or not isinstance(value["refuted"], bool):
        return False
    if not _is_int(value["confidence"]) or not isinstance(value["reason"], str):
        return False
    if not isinstance(value["evidence"], list) or not all(isinstance(item, str) for item in value["evidence"]):
        return False
    if value["severity_adjust"] not in VALID_SEVERITY_ADJUSTMENTS:
        return False
    if value.get("corrected_line") is not None and not _is_int(value["corrected_line"]):
        return False
    return value.get("corrected_suggestion") is None or isinstance(value.get("corrected_suggestion"), str)


def _is_unverified_verdict(value: object) -> bool:
    """A valid JSON verdict with low confidence is an abstention, not evidence."""
    return isinstance(value, dict) and value.get("refuted") is False \
        and isinstance(value.get("reason"), str) and _is_int(value.get("confidence")) \
        and (value["confidence"] <= UNVERIFIED_MAX_CONFIDENCE
             or "미확인:" in value["reason"])


def validate_agent_payload(payload: object, kind: str) -> dict | None:
    """Accept only the fields later phases compare arithmetically or dereference."""
    if kind == "finder":
        if not isinstance(payload, dict):
            return None
        if not isinstance(payload.get("lenses"), list) or not all(isinstance(item, str) for item in payload["lenses"]):
            return None
        if not isinstance(payload.get("reviewed_files"), list) or not all(isinstance(item, str) for item in payload["reviewed_files"]):
            return None
        if not isinstance(payload.get("inspected"), list) or not all(isinstance(item, str) for item in payload["inspected"]):
            return None
        if not isinstance(payload.get("candidates"), list) or not all(_valid_candidate(item) for item in payload["candidates"]):
            return None
        if not isinstance(payload.get("verified_ok"), list) or not all(_valid_verified_ok(item) for item in payload["verified_ok"]):
            return None
        return payload
    if kind == "verdict":
        return payload if _valid_verdict(payload) else None
    raise ValueError(f"unknown agent payload kind: {kind}")


def normalize_pinned_model(provider: str, model: str) -> tuple[str, str]:
    if provider != PINNED_PROVIDER or model not in {PINNED_MODEL, PINNED_MODEL_REF}:
        raise ValueError(f"Parnas Omo driver is pinned to {PINNED_MODEL_REF}; got {provider}/{model}")
    return PINNED_PROVIDER, PINNED_MODEL


def validate_omo_cli(executable: str = "omo", timeout: int = 30) -> None:
    try:
        p = subprocess.run([executable, "--help"], capture_output=True, text=True, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise RuntimeError(f"could not inspect {executable} --help: {exc}") from exc
    if p.returncode != 0:
        raise RuntimeError(f"{executable} --help failed with exit {p.returncode}")
    help_text = f"{p.stdout}\n{p.stderr}"
    required = ("--provider", "--model", "--no-model-fallback", "--permission-preset", "--session-id",
                "--tools", "--no-tools")
    missing = [flag for flag in required if flag not in help_text]
    if missing:
        raise RuntimeError(f"{executable} is missing required flags: {', '.join(missing)}")


# ---------------------------------------------------------------- omo agent runner

def rate_limit_hit(stdout: str | None, stderr: str | None) -> bool:
    text = f"{stdout or ''}\n{stderr or ''}".lower()
    return any(marker in text for marker in RATE_LIMIT_MARKERS)


def rate_limit_delay(attempt: int) -> float:
    return RATE_LIMIT_BACKOFF_SECONDS[min(attempt, len(RATE_LIMIT_BACKOFF_SECONDS) - 1)]


def _jitter(delay: float) -> float:
    """±25% jitter so concurrent agents do not retry in lockstep."""
    return random.uniform(delay * 0.75, delay * 1.25)


def run_omo_process(cmd: list[str], cwd: str, timeout: int) -> subprocess.CompletedProcess:
    """Run Omo in its own process group so timeout cleanup reaches its child engine.

    The engine's model fallback is force-disabled through the environment: the
    CLI flag path (`--no-model-fallback`) is wiped when the engine rebuilds
    extension flag defaults during project-trust reloads, but the agent session
    reads SENPI/OMO_NO_FALLBACK directly from process.env on every path
    (measured 2026-08-30: zai 429 -> silent opencode-go/kimi-k3 switch).
    """
    cwd = canonical_path(cwd)
    env = {**os.environ, "SENPI_NO_FALLBACK": "1", "OMO_NO_FALLBACK": "1"}
    process = subprocess.Popen(cmd, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               text=True, start_new_session=True, env=env)
    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired as exc:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        stdout, stderr = process.communicate()
        raise subprocess.TimeoutExpired(cmd, timeout, output=stdout, stderr=stderr) from exc
    return subprocess.CompletedProcess(cmd, process.returncode, stdout, stderr)


class OmoRunner:
    """One agent = one `omo -p` process; usage is summed from the Omo session log."""

    def __init__(self, provider: str, model: str):
        self.provider, self.model = normalize_pinned_model(provider, model)

    def _cmd(self, sid: str, thinking: str, permission_preset: str = "read-only",
             allowed_tools: tuple[str, ...] | None = None, output_tool: str | None = None,
             max_turns: int | None = None) -> list[str]:
        if permission_preset not in PERMISSION_PRESETS:
            raise ValueError(f"unsupported Omo permission preset: {permission_preset}")
        cmd = ["omo", "--provider", self.provider, "--model", PINNED_MODEL_REF, "--no-skills",
               "--no-context-files", "--no-extensions", "--permission-preset", permission_preset,
               "--no-model-fallback",  # engine flag-rebuild can drop it (2026-08-30); run_omo_process sets SENPI_NO_FALLBACK=1 as the enforced channel
               "--session-id", sid, "--thinking", thinking]
        selected_tools = tuple(allowed_tools or ())
        if output_tool:
            cmd.extend(["--extension", str(PARNAS_OUTPUT_EXTENSION),
                        "--permission", f"{output_tool}=allow"])
            if max_turns is not None:
                cmd.extend(["--parnas-max-turns", str(max_turns)])
            selected_tools += (output_tool,)
        if selected_tools:
            cmd.extend(["--tools", ",".join(dict.fromkeys(selected_tools))])
        elif allowed_tools == ():
            cmd.append("--no-tools")
        return cmd

    def run(self, prompt: str, cwd: str, thinking: str, timeout: int = 1800,
            permission_preset: str = "read-only", payload_kind: str | None = None,
            allowed_tools: tuple[str, ...] | None = None,
            max_turns: int = 18) -> tuple[object, dict, str, dict]:
        sid = uuid.uuid4().hex[:12]
        p = None
        attempts = []
        output_tool = OUTPUT_TOOL_BY_KIND.get(payload_kind)
        parsed = parse_error = schema_error = output_source = None
        attempt_sid = sid
        for attempt in range(MAX_AGENT_ATTEMPTS):
            attempt_sid = sid if attempt == 0 else f"{sid}x{attempt}"
            try:
                p = run_omo_process(
                    self._cmd(attempt_sid, thinking, permission_preset, allowed_tools, output_tool, max_turns)
                    + ["-p", prompt],
                                    cwd, timeout)
            except subprocess.TimeoutExpired as exc:
                stdout, stderr = _timeout_text(exc.output), _timeout_text(exc.stderr)
                attempts.append(_attempt_diagnostics(
                    attempt_sid, None, stdout, stderr, timed_out=True,
                    parse_error="Omo timed out before a complete JSON response",
                ))
                diagnostics = {"failure_kind": "timeout", "attempts": attempts}
                return None, session_usage(attempt_sid), stdout[-400:] or "omo timed out", diagnostics
            evidence = session_evidence(attempt_sid, payload_kind)
            parsed, parse_error, schema_error, output_source = parse_agent_output(
                p.stdout, payload_kind, attempt_sid,
            )
            attempt_diagnostic = _attempt_diagnostics(
                attempt_sid, p.returncode, p.stdout, p.stderr,
                parse_error=parse_error, schema_error=schema_error, output_source=output_source,
            )
            unexpected_models = set(evidence["models"])
            unexpected_models.discard(PINNED_MODEL_REF)
            attempt_diagnostic.update({
                "session_rate_limited": evidence["rate_limited"],
                "rate_limited": evidence["rate_limited"] or rate_limit_hit(p.stdout, p.stderr),
                "model_fallback": evidence["model_fallback"],
                "models": sorted(evidence["models"]),
                "structured_tool_submitted": evidence["structured_tool_submitted"],
            })
            attempts.append(attempt_diagnostic)
            if evidence["model_fallback"]:
                parsed = None
                break
            if attempt_diagnostic["rate_limited"] and not evidence["structured_tool_submitted"]:
                parsed = None
                if attempt == MAX_AGENT_ATTEMPTS - 1:
                    break
                delay = rate_limit_delay(attempt)
                print(f"[driver] provider rate limit; backing off ~{delay:.0f}s "
                      f"before retry {attempt + 2}/{MAX_AGENT_ATTEMPTS}", flush=True)
                time.sleep(_jitter(delay))
                continue
            if unexpected_models:
                parsed = None
                break
            if parsed is not None or p.returncode == 0 or (p.stdout or "").strip():
                break
            if attempt == MAX_AGENT_ATTEMPTS - 1:
                break
            if attempt == 0:
                time.sleep(4)  # transient process failure keeps the original single retry
            else:
                break
        base_usage = {k: 0 for k in USAGE_KEYS}
        for attempt_record in attempts:
            base_usage = add_usage(base_usage, session_usage(attempt_record["session_id"]))
        tail = p.stdout[-400:]
        contributing_sessions = [attempt_sid]
        last_attempt = attempts[-1]
        format_retry_allowed = (
            parsed is None
            and p.returncode == 0
            and not last_attempt["rate_limited"]
            and not last_attempt["model_fallback"]
            and (bool((p.stdout or "").strip()) or last_attempt["structured_tool_submitted"])
        )
        if format_retry_allowed:  # one retry, the analogue of agent() schema retry
            sid2 = sid + "r"
            retry_prompt = format_retry_prompt(prompt, p.stdout, payload_kind)
            try:
                p2 = run_omo_process(self._cmd(sid2, thinking, permission_preset, (), output_tool, 1) +
                                     ["-p", retry_prompt],
                                     cwd, timeout)
                tail = p2.stdout[-400:]
                evidence = session_evidence(sid2, payload_kind)
                parsed, parse_error, schema_error, output_source = parse_agent_output(
                    p2.stdout, payload_kind, sid2,
                )
                attempt_diagnostic = _attempt_diagnostics(
                    sid2, p2.returncode, p2.stdout, p2.stderr,
                    parse_error=parse_error, schema_error=schema_error, output_source=output_source,
                )
                attempt_diagnostic.update({
                    "session_rate_limited": evidence["rate_limited"],
                    "rate_limited": evidence["rate_limited"] or rate_limit_hit(p2.stdout, p2.stderr),
                    "model_fallback": evidence["model_fallback"],
                    "models": sorted(evidence["models"]),
                    "structured_tool_submitted": evidence["structured_tool_submitted"],
                })
                attempts.append(attempt_diagnostic)
                unexpected_retry_models = set(evidence["models"])
                unexpected_retry_models.discard(PINNED_MODEL_REF)
                if (evidence["model_fallback"] or unexpected_retry_models
                        or (attempt_diagnostic["rate_limited"]
                            and not evidence["structured_tool_submitted"])):
                    parsed = None
            except subprocess.TimeoutExpired as exc:
                stdout, stderr = _timeout_text(exc.output), _timeout_text(exc.stderr)
                tail = stdout[-400:] or "omo timed out"
                attempts.append(_attempt_diagnostics(
                    sid2, None, stdout, stderr, timed_out=True,
                    parse_error="Omo timed out before a complete JSON response",
                ))
            contributing_sessions.append(sid2)
            usage = add_usage(base_usage, session_usage(sid2))
        else:
            usage = base_usage
        # Model pinning is judged only on the sessions whose output was actually
        # accepted: a rate-limited earlier attempt may carry the engine fallback
        # model in its log, and that must not discard a clean pinned verdict.
        unexpected_models = session_models(*contributing_sessions)
        unexpected_models.discard(PINNED_MODEL_REF)
        diagnostics = {"failure_kind": _failure_kind(parsed, attempts), "attempts": attempts}
        if unexpected_models or any(record.get("model_fallback") for record in attempts):
            diagnostics["failure_kind"] = "model_mismatch"
            selected = ", ".join(sorted(unexpected_models)) or "fallback model change"
            return None, usage, f"omo selected unpinned model(s): {selected}", diagnostics
        return parsed, usage, tail, diagnostics


def _timeout_text(value: str | bytes | None) -> str:
    if isinstance(value, bytes):
        return value.decode(errors="replace")
    return value or ""


def _tool_denials(stdout: str, stderr: str) -> list[str]:
    markers = ("permission denied", "not allowed", "blocked", "is disabled", "tool denial")
    return [line.strip() for line in f"{stdout}\n{stderr}".splitlines()
            if any(marker in line.lower() for marker in markers)]


def _attempt_diagnostics(session_id: str, returncode: int | None, stdout: str | None, stderr: str | None,
                         *, timed_out: bool = False, parse_error: str | None = None,
                         schema_error: str | None = None, output_source: str | None = None) -> dict:
    raw_stdout, raw_stderr = stdout or "", stderr or ""
    return {"session_id": session_id, "returncode": returncode, "timed_out": timed_out,
            "stdout": raw_stdout, "stderr": raw_stderr,
            "tool_denials": _tool_denials(raw_stdout, raw_stderr),
            "parse_error": parse_error, "schema_error": schema_error,
            "output_source": output_source}


def parse_agent_stdout(stdout: str | None, payload_kind: str | None) -> tuple[object, str | None, str | None]:
    parsed = extract_json(stdout)
    if parsed is None:
        return None, "stdout did not contain a complete JSON object", None
    if payload_kind and validate_agent_payload(parsed, payload_kind) is None:
        return None, None, f"JSON object failed the {payload_kind} schema"
    return parsed, None, None


def parse_agent_output(stdout: str | None, payload_kind: str | None,
                       session_id: str) -> tuple[object, str | None, str | None, str | None]:
    if payload_kind:
        tool_payload = session_tool_payload(session_id, payload_kind)
        if tool_payload is not None:
            parsed = validate_agent_payload(tool_payload, payload_kind)
            if parsed is not None:
                return parsed, None, None, "structured_tool"
            return None, None, f"structured tool arguments failed the {payload_kind} schema", "structured_tool"
    parsed, parse_error, schema_error = parse_agent_stdout(stdout, payload_kind)
    return parsed, parse_error, schema_error, "stdout" if parsed is not None else None


def format_retry_prompt(original_prompt: str, previous_stdout: str | None, payload_kind: str | None) -> str:
    schema_name = payload_kind or "requested"
    output_tool = OUTPUT_TOOL_BY_KIND.get(payload_kind)
    final_action = (f"Call {output_tool} exactly once with the complete result. Do not print JSON as assistant text."
                    if output_tool else
                    "Return exactly one complete JSON object matching the original response schema.")
    return f"""The previous response failed {schema_name} JSON parsing or schema validation.
Do not call any investigation tool. {final_action}
No markdown fences, analysis, notes, or prose may appear before or after the final action.

ORIGINAL REQUEST (including the original candidate JSON):
{original_prompt}

PREVIOUS STDOUT:
{previous_stdout or "(empty: the prior agent ended without a final assistant response)"}"""


def _failure_kind(parsed: object, attempts: list[dict]) -> str | None:
    if parsed is not None:
        return None
    last = attempts[-1]
    if last["timed_out"]:
        return "timeout"
    if last.get("rate_limited") or rate_limit_hit(last["stdout"], last["stderr"]):
        return "rate_limited"
    if last["schema_error"]:
        return "schema_failure"
    if last["parse_error"]:
        return "parse_failure"
    if last["returncode"] not in (0, None):
        return "process_failure"
    return "parse_failure"


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


def session_models(*sids: str) -> set[str]:
    models: set[str] = set()
    if not OMO_SESSIONS.exists():
        return models
    for sid in sids:
        for f in OMO_SESSIONS.rglob(f"*{sid}.jsonl"):
            try:
                for line in f.read_text(errors="ignore").splitlines():
                    try:
                        o = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if o.get("type") == "model_change":
                        provider, model = o.get("provider"), o.get("modelId")
                    elif o.get("type") == "message":
                        message = o.get("message") or {}
                        provider, model = message.get("provider"), message.get("model")
                    else:
                        continue
                    if provider and model:
                        models.add(f"{provider}/{model}")
            except OSError:
                continue
    return models


def _session_error_messages(value: object):
    if isinstance(value, dict):
        for key, item in value.items():
            if key == "errorMessage" and isinstance(item, str):
                yield item
            else:
                yield from _session_error_messages(item)
    elif isinstance(value, list):
        for item in value:
            yield from _session_error_messages(item)


def session_evidence(sid: str, payload_kind: str | None) -> dict:
    """Read provider errors, model switches, and structured submission from one session."""
    evidence = {
        "rate_limited": False,
        "model_fallback": False,
        "models": set(),
        "structured_tool_submitted": False,
    }
    tool_name = OUTPUT_TOOL_BY_KIND.get(payload_kind)
    if not OMO_SESSIONS.exists():
        return evidence
    for f in OMO_SESSIONS.rglob(f"*{sid}.jsonl"):
        try:
            for line in f.read_text(errors="ignore").splitlines():
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if any(rate_limit_hit(None, message)
                       for message in _session_error_messages(record)):
                    evidence["rate_limited"] = True
                if record.get("type") == "model_change":
                    provider, model = record.get("provider"), record.get("modelId")
                    if record.get("reason") == "fallback":
                        evidence["model_fallback"] = True
                elif record.get("type") == "message":
                    message = record.get("message") or {}
                    provider, model = message.get("provider"), message.get("model")
                    if message.get("role") == "assistant" and tool_name:
                        for content in message.get("content") or []:
                            if (isinstance(content, dict) and content.get("type") == "toolCall"
                                    and content.get("name") == tool_name):
                                evidence["structured_tool_submitted"] = True
                else:
                    continue
                if provider and model:
                    evidence["models"].add(f"{provider}/{model}")
        except OSError:
            continue
    return evidence


def session_tool_payload(sid: str, payload_kind: str) -> dict | None:
    tool_name = OUTPUT_TOOL_BY_KIND[payload_kind]
    payload = None
    if not OMO_SESSIONS.exists():
        return None
    for f in OMO_SESSIONS.rglob(f"*{sid}.jsonl"):
        try:
            for line in f.read_text(errors="ignore").splitlines():
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                message = record.get("message") if record.get("type") == "message" else None
                if not isinstance(message, dict) or message.get("role") != "assistant":
                    continue
                for content in message.get("content") or []:
                    if (isinstance(content, dict) and content.get("type") == "toolCall"
                            and content.get("name") == tool_name and isinstance(content.get("arguments"), dict)):
                        payload = content["arguments"]
        except OSError:
            continue
    return payload


def add_usage(a: dict, b: dict) -> dict:
    return {k: (a.get(k, 0) or 0) + (b.get(k, 0) or 0) for k in USAGE_KEYS}


def empty_failure_counts() -> dict[str, int]:
    return {key: 0 for key in FAILURE_COUNT_KEYS}


def agent_diagnostic(result: dict) -> dict:
    return {"label": result["label"], **result["diagnostics"]}


def run_batch(runner: OmoRunner, tasks: list[dict], workers: int) -> list[dict]:
    results: list[dict | None] = [None] * len(tasks)
    with ThreadPoolExecutor(max_workers=workers) as ex:
        futs = {
            ex.submit(
                runner.run,
                t["prompt"],
                t["cwd"],
                t["thinking"],
                permission_preset=t.get("permission_preset", "read-only"),
                payload_kind=t.get("payload_kind"),
                allowed_tools=t.get("allowed_tools"),
                max_turns=t.get("max_turns", 18),
            ): i
            for i, t in enumerate(tasks)
        }
        for fut in as_completed(futs):
            i = futs[fut]
            try:
                outcome = fut.result()
                if len(outcome) == 4:
                    parsed, usage, tail, diagnostics = outcome
                else:
                    parsed, usage, tail = outcome
                    diagnostics = {"failure_kind": "parse_failure" if parsed is None else None,
                                   "attempts": []}
                if tasks[i].get("payload_kind"):
                    validated = validate_agent_payload(parsed, tasks[i]["payload_kind"])
                    if parsed is not None and validated is None:
                        diagnostics["failure_kind"] = "schema_failure"
                    parsed = validated
            except Exception as e:  # noqa: BLE001
                parsed, usage, tail = None, {k: 0 for k in USAGE_KEYS}, f"driver error: {e}"
                diagnostics = {"failure_kind": "process_failure", "attempts": [],
                               "driver_error": str(e)}
            results[i] = {"parsed": parsed, "usage": usage, "tail": tail, "label": tasks[i]["label"],
                          "diagnostics": diagnostics}
            status = "OK" if parsed is not None else (
                diagnostics.get("failure_kind") or "parse_failure"
            ).upper()
            print(f"[driver] {tasks[i]['label']}: {status} "
                  f"(in {usage['input']:,} out {usage['output']:,} cacheRead {usage['cacheRead']:,} "
                  f"reasoning {usage['reasoning']:,} ${usage['cost_total']:.4f})", flush=True)
    return results


# ---------------------------------------------------------------- phases

def finish_find_stage(a: dict, units: list[dict], finders: list[dict],
                      usage_all: dict, agent_diagnostics: list[dict]) -> dict:
    failure_counts = empty_failure_counts()
    agent_failures = []
    for diagnostic in agent_diagnostics:
        kind = diagnostic.get("failure_kind")
        if kind:
            agent_failures.append(diagnostic["label"])
            failure_counts[kind if kind in failure_counts else "process_failure"] += 1
    seen = dedup_candidates(finders)
    candidates = sorted(seen, key=lambda c: -c.get("confidence", 0))
    prescreened, kept = [], []
    for c in candidates:
        why = prescreen(a, c)
        (prescreened if why else kept).append(
            {**c, **({"verdicts": [{"skeptic": "prescreen", "refuted": True, "confidence": 90, "reason": why, "evidence": [], "severity_adjust": "keep"}]} if why else {})})
    max_candidates = a.get("max_candidates", len(kept))
    if len(kept) > max_candidates:
        print(f"[driver] dropping {len(kept) - max_candidates} lowest-confidence candidates (cap {max_candidates})")
        kept = kept[:max_candidates]
    coverage = coverage_report(units, finders)
    failure_counts["coverage_gap"] = sum(len(gap["missing_files"]) for gap in coverage["gaps"]) + sum(
        len(item["files"]) for item in coverage["duplicates"]
    )
    print(f"[driver] finders={len(finders)}/{len(units)} unique={len(seen)} prescreened={len(prescreened)} "
          f"to_verify={len(kept)} failures={len(agent_failures)} "
          f"coverage={coverage['covered_assignments']}/{coverage['expected_assignments']}", flush=True)
    return {"finders": finders, "verified_ok": [v for r in finders for v in (r.get("verified_ok") or [])],
            "candidates": kept, "prescreened": prescreened, "usage_find": usage_all,
            "agent_failures": agent_failures, "failure_counts": failure_counts,
            "agent_diagnostics": agent_diagnostics, "coverage": coverage,
            "degraded": bool(agent_failures) or not coverage["complete"]}


def phase_find(a: dict, runner: OmoRunner) -> dict:
    units, cwd = a["units"], canonical_path(a["checkout"])
    usage_all = {k: 0 for k in USAGE_KEYS}
    finders: list[dict] = []
    agent_diagnostics = []

    def accept(result: dict, unit: dict) -> None:
        nonlocal usage_all
        usage_all = add_usage(usage_all, result["usage"])
        agent_diagnostics.append(agent_diagnostic(result))
        if isinstance(result["parsed"], dict):
            finders.append({**result["parsed"], "unit": unit["id"], "lenses": unit["lenses"]})

    if not units:
        print("[driver] 0 finder units — nothing to inspect (empty units list)", flush=True)
        return finish_find_stage(a, [], [], usage_all, [])
    print(f"[driver] {len(units)} finder units (first alone to warm the provider cache, rest ×{a['workers']['finder']})", flush=True)
    first = units[0]
    first_result = run_batch(runner, [{
        "prompt": finder_prompt(a, first), "cwd": cwd, "thinking": a["thinking"]["finder"],
        "permission_preset": "read-only", "payload_kind": "finder",
        "allowed_tools": READ_ONLY_TOOLS, "max_turns": a["finder_turns"],
        "label": f"find:{first['id']}",
    }], 1)[0]
    accept(first_result, first)
    rest = units[1:]
    if rest:
        rest_results = run_batch(runner, [{
            "prompt": finder_prompt(a, unit), "cwd": cwd, "thinking": a["thinking"]["finder"],
            "permission_preset": "read-only", "payload_kind": "finder",
            "allowed_tools": READ_ONLY_TOOLS, "max_turns": a["finder_turns"],
            "label": f"find:{unit['id']}",
        } for unit in rest], a["workers"]["finder"])
        for result, unit in zip(rest_results, rest):
            accept(result, unit)
    return finish_find_stage(a, units, finders, usage_all, agent_diagnostics)


def failed_find_unit_ids(a: dict, found: dict) -> list[str]:
    """Return incomplete finder units in deterministic workflow order."""
    units = a.get("units") or []
    finders = found.get("finders") or []
    counts: dict[str, int] = {}
    for finder in finders:
        unit_id = finder.get("unit")
        if unit_id:
            counts[unit_id] = counts.get(unit_id, 0) + 1
    coverage = coverage_report(units, finders)
    failed = {unit.get("id") for unit in units if counts.get(unit.get("id"), 0) != 1}
    failed.update(row.get("unit") for row in coverage["gaps"])
    failed.update(row.get("unit") for row in coverage["duplicates"])
    return [unit["id"] for unit in units if unit.get("id") in failed]


def retry_failed_find_units(a: dict, runner: OmoRunner, found: dict) -> dict:
    """Rerun only incomplete finder units and rebuild the full find stage."""
    failed_ids = failed_find_unit_ids(a, found)
    if not failed_ids:
        return finish_find_stage(
            a, a.get("units") or [], found.get("finders") or [],
            found.get("usage_find") or {k: 0 for k in USAGE_KEYS},
            found.get("agent_diagnostics") or [],
        )
    failed = set(failed_ids)
    retry_units = [unit for unit in a["units"] if unit["id"] in failed]
    print(f"[driver] retrying {len(retry_units)} failed finder unit(s): {', '.join(failed_ids)}", flush=True)
    retried = phase_find({**a, "units": retry_units}, runner)
    finders = [finder for finder in (found.get("finders") or []) if finder.get("unit") not in failed]
    finders.extend(retried["finders"])
    retry_labels = {f"find:{unit_id}" for unit_id in failed_ids}
    diagnostics = [
        diagnostic for diagnostic in (found.get("agent_diagnostics") or [])
        if diagnostic.get("label") not in retry_labels
    ]
    diagnostics.extend(retried["agent_diagnostics"])
    usage = add_usage(
        found.get("usage_find") or {k: 0 for k in USAGE_KEYS},
        retried["usage_find"],
    )
    merged = finish_find_stage(a, a["units"], finders, usage, diagnostics)
    merged["retried_units"] = failed_ids
    return merged


def phase_verify(a: dict, runner: OmoRunner, found: dict) -> dict:
    incomplete = failed_find_unit_ids(a, found)
    if incomplete:
        raise ValueError(
            "incomplete find-stage; rerun failed units with --phase verify --retry-failed-units: "
            + ", ".join(incomplete)
        )
    cwd, candidates = canonical_path(a["checkout"]), found["candidates"]
    usage_all = found["usage_find"]
    agent_failures = list(found.get("agent_failures") or [])
    low_confidence_abstains = list(found.get("low_confidence_abstains") or [])
    failure_counts = {**empty_failure_counts(), **(found.get("failure_counts") or {})}
    agent_diagnostics = list(found.get("agent_diagnostics") or [])
    coverage = found.get("coverage") or coverage_report(a.get("units") or [], found.get("finders") or [])
    print(f"[driver] verify: {len(candidates)} tracers (×{a['workers']['tracer']})", flush=True)
    tasks = [{"prompt": skeptic_prompt(a, "tracer", c, None), "cwd": cwd, "thinking": a["thinking"]["tracer"],
              "permission_preset": "read-only", "payload_kind": "verdict",
              "allowed_tools": READ_ONLY_TOOLS,
              "max_turns": a["skeptic_turns"],
              "label": f"tracer:{c['path'].split('/')[-1]}:{c.get('new_line')}"} for c in candidates]
    tracer_results = run_batch(runner, tasks, a["workers"]["tracer"]) if tasks else []
    for r in tracer_results:
        agent_diagnostics.append(agent_diagnostic(r))
        if r["parsed"] is None:
            agent_failures.append(r["label"])
            failure_counts[r["diagnostics"]["failure_kind"] or "parse_failure"] += 1
        elif _is_unverified_verdict(r["parsed"]):
            low_confidence_abstains.append(r["label"])
            failure_counts["low_confidence_abstain"] += 1

    repro_tasks, repro_idx, stages, skipped = [], [], [], 0
    for c, tr in zip(candidates, tracer_results):
        usage_all = add_usage(usage_all, tr["usage"])
        raw_t = tr["parsed"]
        t = None if _is_unverified_verdict(raw_t) else raw_t
        stages.append({"candidate": c, "tracer": t})
        if t and t.get("refuted") and t.get("confidence", 0) >= KILL:
            skipped += 1
        else:
            repro_idx.append(len(stages) - 1)
            repro_tasks.append({"prompt": skeptic_prompt(a, "reproducer", c, raw_t), "cwd": cwd,
                                "thinking": a["thinking"]["reproducer"],
                                "permission_preset": "workspace", "payload_kind": "verdict",
                                "allowed_tools": REPRODUCER_TOOLS,
                                "max_turns": a["skeptic_turns"],
                                "label": f"repro:{c['path'].split('/')[-1]}:{c.get('new_line')}"})
    if repro_tasks:
        print(f"[driver] {len(repro_tasks)} reproducers (×{a['workers']['reproducer']}, tracer killed {skipped})", flush=True)
        for idx, r in zip(repro_idx, run_batch(runner, repro_tasks, a["workers"]["reproducer"])):
            usage_all = add_usage(usage_all, r["usage"])
            agent_diagnostics.append(agent_diagnostic(r))
            if r["parsed"] is None:
                agent_failures.append(r["label"])
                failure_counts[r["diagnostics"]["failure_kind"] or "parse_failure"] += 1
            elif _is_unverified_verdict(r["parsed"]):
                low_confidence_abstains.append(r["label"])
                failure_counts["low_confidence_abstain"] += 1
            stages[idx]["reproducer"] = None if _is_unverified_verdict(r["parsed"]) else r["parsed"]

    findings, refuted, abstained = [], list(found["prescreened"]), 0
    for s in stages:
        verdicts = [v for v in (s.get("tracer"), s.get("reproducer")) if v]
        c = s["candidate"]
        if any(v.get("refuted") and v.get("confidence", 0) >= KILL for v in verdicts):
            refuted.append({**c, "verdicts": verdicts})
            continue
        if len(verdicts) < 2:
            abstained += 1
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
    carried = list(a.get("carried") or [])
    status = "degraded" if agent_failures or low_confidence_abstains or not coverage["complete"] else "ok"
    print(f"[driver] {len(findings) - abstained} confirmed, {abstained} abstained (+{len(carried)} carried), {len(refuted)} refuted "
          f"({len(found['prescreened'])} by prescreen, {skipped} reproducers skipped, {len(partial)} residual risk, "
          f"status={status})", flush=True)
    return {"findings": findings + carried, "refuted": refuted, "partial": partial,
            "refuted_for_history": refuted_for_history, "verified_ok": found["verified_ok"],
            "inspected": [{"unit": r["unit"], "lenses": r["lenses"],
                           "reviewed_files": r.get("reviewed_files", []), "inspected": r.get("inspected", [])}
                          for r in found["finders"]],
            "cost": {"finders": len(found["finders"]), "candidates_in": len(candidates), "tracers": len(candidates),
                     "reproducers": len(repro_tasks), "reproducers_skipped": skipped,
                     "prescreened": len(found["prescreened"]), "usage": usage_all,
                     "profile": a["profile"], "model": f"{a['provider']}/{a['model']}"},
            "status": status, "degraded": status == "degraded", "agent_failures": agent_failures,
            "low_confidence_abstains": low_confidence_abstains,
            "failure_counts": failure_counts, "agent_diagnostics": agent_diagnostics,
            "coverage": coverage}


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--args", required=True, help="workflow_args.json written by mr_context.py")
    ap.add_argument("--phase", choices=["find", "verify", "all"], default="all")
    ap.add_argument("--profile", choices=sorted(PROFILES), default="omo-flash")
    ap.add_argument("--provider", default=PINNED_PROVIDER, help=f"pinned to {PINNED_PROVIDER}")
    ap.add_argument("--model", default=PINNED_MODEL, help=f"pinned to {PINNED_MODEL_REF}")
    ap.add_argument("--retry-degraded-from",
                    help="with --phase verify, rerun only prior skeptics-unavailable findings")
    ap.add_argument("--retry-failed-units", action="store_true",
                    help="with --phase verify, rerun only incomplete finder units before verification")
    ap.add_argument("--thinking-finder"), ap.add_argument("--thinking-tracer"), ap.add_argument("--thinking-reproducer")
    ns = ap.parse_args()
    if ns.retry_failed_units and ns.retry_degraded_from:
        print("[driver] ERROR: --retry-failed-units and --retry-degraded-from cannot be combined",
              file=sys.stderr)
        return 2
    if ns.retry_failed_units and ns.phase != "verify":
        print("[driver] ERROR: --retry-failed-units requires --phase verify", file=sys.stderr)
        return 2
    if ns.retry_degraded_from and ns.phase != "verify":
        print("[driver] ERROR: --retry-degraded-from requires --phase verify", file=sys.stderr)
        return 2

    try:
        provider, model = normalize_pinned_model(ns.provider, ns.model)
        validate_omo_cli()
    except (ValueError, RuntimeError) as exc:
        print(f"[driver] ERROR: {exc}", file=sys.stderr, flush=True)
        return 2
    args_json = json.loads(Path(ns.args).read_text())
    # The level is a contract, not a label: post_review.py discloses which verification ran, so a
    # pack built for an inline level must never reach the fan-out that would understate it.
    level = args_json.get("level")
    if level and level != "max":
        print(f"[driver] ERROR: this driver runs the max pipeline; workflow_args.json says level={level}. "
              "Re-run preflight with --level max, or run the inline path from SKILL.md instead.", file=sys.stderr)
        return 2
    budget = resolve_budget(args_json, ns.profile)
    thinking = budget["thinking"]
    if ns.thinking_finder:
        thinking["finder"] = ns.thinking_finder
    if ns.thinking_tracer:
        thinking["tracer"] = ns.thinking_tracer
    if ns.thinking_reproducer:
        thinking["reproducer"] = ns.thinking_reproducer
    a = {**args_json, **budget, "thinking": thinking, "profile": ns.profile,
         "provider": provider, "model": model}
    canonicalize_runtime_paths(a)
    print(f"[driver] profile={ns.profile} model={provider}/{model} finder_turns={budget['finder_turns']} "
          f"skeptic_turns={budget['skeptic_turns']} maxCandidates={budget['max_candidates']} "
          f"perLensCap={budget['per_lens_cap']} workers={budget['workers']}", flush=True)
    t0 = time.time()
    runner = OmoRunner(provider, model)
    input_dir = prepare_omo_inputs(a)
    try:
        find_stage = Path(a["outDir"]) / "find-stage.json"
        if ns.phase == "verify":
            if not find_stage.exists():
                raise SystemExit(f"find-stage.json not found at {find_stage} — run --phase find first")
            found = json.loads(find_stage.read_text())
            print(f"[driver] loaded find stage from {find_stage}", flush=True)
            incomplete = failed_find_unit_ids(a, found)
            if incomplete and not ns.retry_failed_units:
                print("[driver] ERROR: incomplete find-stage; rerun with --phase verify "
                      f"--retry-failed-units ({', '.join(incomplete)})", file=sys.stderr, flush=True)
                return 2
            if ns.retry_failed_units:
                found = retry_failed_find_units(a, runner, found)
                find_stage.write_text(json.dumps(found, ensure_ascii=False, indent=1))
                incomplete = failed_find_unit_ids(a, found)
                if incomplete:
                    print("[driver] ERROR: finder units still incomplete after retry: "
                          + ", ".join(incomplete), file=sys.stderr, flush=True)
                    return 2
            if ns.retry_degraded_from:
                previous_path = Path(ns.retry_degraded_from)
                previous_result = json.loads(previous_path.read_text())
                found["candidates"] = select_retry_candidates(found["candidates"], previous_result)
                print(f"[driver] selected {len(found['candidates'])} degraded candidates from "
                      f"{previous_path}", flush=True)
        else:
            found = phase_find(a, runner)
            # Persist before verify so a crash in verify does not lose the find spend.
            find_stage.write_text(json.dumps(found, ensure_ascii=False, indent=1))
            if ns.phase == "find":
                return 2 if found.get("degraded") else 0
        result = phase_verify(a, runner, found)
        out_name = "workflow-retry-result.json" if ns.retry_degraded_from else "workflow-result.json"
        out = Path(a["outDir"]) / out_name
        out.write_text(json.dumps(result, ensure_ascii=False, indent=1))
        u = result["cost"]["usage"]
        print(f"[driver] done in {time.time() - t0:.0f}s → {out}")
        print(f"[driver] TOKENS: input={u['input']:,} output={u['output']:,} cacheRead={u['cacheRead']:,} "
              f"cacheWrite={u['cacheWrite']:,} reasoning={u['reasoning']:,} cost=${u['cost_total']:.4f}")
        return 2 if result["degraded"] else 0
    finally:
        cleanup_omo_inputs(input_dir)


if __name__ == "__main__":
    raise SystemExit(main())
