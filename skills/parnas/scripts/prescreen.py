#!/usr/bin/env python3
"""Deterministic candidate stage for the inline effort levels (low/medium/high/xhigh).

`max` runs this inside references/workflow.js. The levels below it have no Workflow
runtime and no skeptics, so without this script they would silently lose the dedup,
the off-hunk screen and the committed refutation memory — the parts that cost no
agent at all. Port of workflow.js `seen`/`prescreen`/`maxCandidates`; keep the two
in sync, and treat workflow.js as the source of truth when they differ.

Usage:
  prescreen.py --args DIR/workflow_args.json --candidates DIR/candidates.json [--out DIR/prescreened.json]

--candidates accepts a JSON list of finder candidates, or {"candidates": [...]}.
Writes (and prints when --out is absent) {"candidates", "prescreened", "stats"}:
kept candidates ordered by confidence, and every dropped one carrying the reason.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

# security/data readings are never suppressed by learned refutations (Greptile/Kodus).
SUPPRESS_EXEMPT = {"security", "data"}
TOKEN_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]{2,}|[가-힣]{2,}")


def bigrams(t: str | None) -> set[str]:
    s = "".join(ch for ch in (t or "") if ch.isalnum())
    return {s[i:i + 2] for i in range(len(s) - 1)}


def similar(a: str | None, b: str | None) -> float:
    A, B = bigrams(a), bigrams(b)
    return len(A & B) / max(1, min(len(A), len(B)))


def tokens(*parts: str | None) -> set[str]:
    return {m.lower() for m in TOKEN_RE.findall(" ".join(p or "" for p in parts))}


def overlap(a: list[str | None], b: list[str | None]) -> float:
    A, B = tokens(*a), tokens(*b)
    if not A or not B:
        return 0.0
    return len(A & B) / min(len(A), len(B))


def dedup(raw: list[dict]) -> list[dict]:
    """workflow.js `seen` — same defect reported by several lenses collapses to one."""
    seen: dict[str, dict] = {}
    for c in raw:
        lens = c.get("lens") or "?"
        key = f"{c.get('path')}:{c.get('new_line')}:{(c.get('title') or '')[:24]}"
        dup = next(
            (x for x in seen.values() if
             (x.get("path") == c.get("path") and abs((x.get("new_line") or 0) - (c.get("new_line") or 0)) <= 2
              and x.get("category") == c.get("category"))
             or similar(x.get("title"), c.get("title")) >= 0.5
             or (x.get("rule") and x.get("rule") == c.get("rule") and similar(x.get("what"), c.get("what")) >= 0.4)
             or (x.get("category") == c.get("category") and similar(x.get("why"), c.get("why")) >= 0.5)),
            None)
        if dup is not None:
            dup["lenses"].append(lens)
            dup["confidence"] = max(dup.get("confidence") or 0, c.get("confidence") or 0)
            if c.get("category") in SUPPRESS_EXEMPT:
                dup["category"] = c["category"]
            dup["evidence"] = list(dict.fromkeys((dup.get("evidence") or []) + (c.get("evidence") or [])))
            dup["alternates"] = (dup.get("alternates") or []) + [
                {"path": c.get("path"), "new_line": c.get("new_line"), "title": c.get("title")}]
            continue
        seen[key] = {**c, "lenses": [lens]}
    return sorted(seen.values(), key=lambda c: c.get("confidence") or 0, reverse=True)


def prescreen(c: dict, hunk_ranges: dict, refuted_history: list[dict]) -> str | None:
    """Returns the refusal reason, or None to keep. Port of workflow.js `prescreen`."""
    ranges = hunk_ranges.get(c.get("path"))
    if not ranges:
        return f"prescreen: {c.get('path')} is not a changed file"
    line = c.get("new_line")
    in_hunk = any(a <= line <= b for a, b in ranges) if isinstance(line, int) else False
    if not in_hunk and not c.get("newly_reachable"):
        return f"prescreen: line {line} is outside every hunk of {c.get('path')} and not marked newly_reachable"
    if c.get("category") not in SUPPRESS_EXEMPT:
        for h in refuted_history:
            if h.get("path") == c.get("path") and overlap([h.get("title"), h.get("what")],
                                                          [c.get("title"), c.get("what")]) >= 0.5:
                return f"prescreen: refuted before on this file — {(h.get('title') or '')[:120]}"
    return None


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--args", required=True, help="workflow_args.json from mr_context.py")
    ap.add_argument("--candidates", required=True, help="JSON list of finder candidates, or {\"candidates\": [...]}")
    ap.add_argument("--out", help="write here instead of stdout")
    a = ap.parse_args()

    wf = json.loads(Path(a.args).read_text())
    payload = json.loads(Path(a.candidates).read_text())
    raw = payload.get("candidates", []) if isinstance(payload, dict) else payload
    if not isinstance(raw, list):
        sys.exit("--candidates must be a JSON list or an object with a 'candidates' list")

    hunk_ranges = wf.get("hunkRanges") or {}
    refuted_history = wf.get("refutedHistory") or []
    max_candidates = wf.get("maxCandidates") or len(raw)

    deduped = dedup(raw)
    kept, dropped = [], []
    for c in deduped:
        why = prescreen(c, hunk_ranges, refuted_history)
        if why:
            dropped.append({**c, "verdicts": [{"skeptic": "prescreen", "refuted": True, "confidence": 90,
                                               "reason": why, "evidence": [], "severity_adjust": "keep"}]})
        else:
            kept.append(c)
    over_cap = kept[max_candidates:]
    kept = kept[:max_candidates]

    result = {"candidates": kept, "prescreened": dropped,
              "stats": {"level": wf.get("level"), "raw": len(raw), "unique": len(deduped),
                        "prescreened": len(dropped), "over_cap": len(over_cap),
                        "max_candidates": max_candidates, "kept": len(kept)}}
    out = json.dumps(result, ensure_ascii=False, indent=1)
    if a.out:
        Path(a.out).write_text(out)
        print(json.dumps(result["stats"], ensure_ascii=False))
    else:
        print(out)


if __name__ == "__main__":
    main()
