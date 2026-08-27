#!/usr/bin/env python3
"""Deterministic quality gate for the changed files of an MR/PR (runs in the head worktree).

Usage:
  quality_gate.py --context DIR/context.json [--limits service=500,controller=300,method=50]
                  [--skip lint,typecheck,test] [--timeout 600]

Writes DIR/gate.json and DIR/gate.md and prints gate.md. Exit 0 always (the gate reports;
the moderator decides). Each tool failure becomes a deterministic candidate in gate.json →
`candidates` with confidence 95 (lint/typecheck/test) or 90 (limit breach), ready to be
merged into findings.json without skeptics.

Checks (auto-detected from the repo):
  node   biome/eslint on changed files · scoped tsc --noEmit (changed files as roots) ·
         jest on changed spec files + sibling specs of changed sources
  go     go vet + go test on packages containing changed files
  python ruff on changed files · pytest on changed test files
Metrics (per changed non-test source file, post-change):
  LOC, longest function LOC, functions over the method limit, max nesting depth,
  approx cyclomatic complexity of the worst function, and the diff's test-to-source added-line
  ratio. Limits default to CLAUDE.md-style Service 500 / Controller 300 / Method 50.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import tempfile
from pathlib import Path

FUNC_START_RE = re.compile(r"^\s*(?:(?:public|private|protected|static|async|export|default|readonly|override|abstract)\s+)*"
                           r"(?:function\s*\*?\s*)?([A-Za-z_$][\w$]*)\s*(?:<[^>(]*>)?\s*\("
                           r"|^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\(|[A-Za-z_$][\w$]*\s*=>)"
                           r"|^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(")
KEYWORDS = {"if", "for", "while", "switch", "catch", "else", "return", "new", "await", "typeof", "constructor?", "super", "throw", "do", "try", "with", "yield", "delete", "void", "in", "of", "case", "default", "expect", "it", "describe", "beforeEach", "afterEach", "test"}
BRANCH_RE = re.compile(r"\b(if|for|while|case|catch)\b|&&|\|\||\?[^.:]")


def run(cmd: list[str], cwd: str, timeout: int) -> tuple[int, str]:
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=timeout)
        return p.returncode, (p.stdout + "\n" + p.stderr).strip()
    except subprocess.TimeoutExpired:
        return 124, f"timed out after {timeout}s"
    except FileNotFoundError:
        return 127, f"not found: {cmd[0]}"


def tail(s: str, n: int = 40) -> str:
    lines = s.splitlines()
    return "\n".join(lines[-n:])


# ----------------------------------------------------------------------------- metrics

def function_metrics(text: str) -> list[dict]:
    """Brace-balanced scan; good enough for TS/JS/Go/Java-like code."""
    lines = text.splitlines()
    funcs = []
    stack: list[dict] = []
    depth = 0
    pending: dict | None = None  # signature seen, waiting for its opening brace (multi-line params)
    for i, line in enumerate(lines, 1):
        stripped = line.split("//")[0]
        m = FUNC_START_RE.match(line)
        if m:
            name = next(g for g in m.groups() if g)
            if name not in KEYWORDS:
                pending = {"name": name, "start": i, "depth_at_open": depth, "max_depth": 0, "branches": 0, "paren": 0}
        if pending and pending["start"] != i and stripped.rstrip().endswith(";") and pending["paren"] <= 0:
            pending = None  # declaration / call statement, not a body
        for ch in stripped:
            if pending:
                if ch == "(":
                    pending["paren"] += 1
                elif ch == ")":
                    pending["paren"] -= 1
            if ch == "{":
                if pending and pending["paren"] <= 0 and depth == pending["depth_at_open"]:
                    stack.append(pending)
                    pending = None
                depth += 1
                for f in stack:
                    f["max_depth"] = max(f["max_depth"], depth - f["depth_at_open"])
            elif ch == "}":
                depth -= 1
                if stack and depth == stack[-1]["depth_at_open"]:
                    f = stack.pop()
                    f["end"] = i
                    f["loc"] = i - f["start"] + 1
                    f["nesting"] = max(0, f["max_depth"] - 1)
                    f["cyclomatic"] = 1 + f["branches"]
                    f.pop("paren", None)
                    funcs.append(f)
        if pending and i - pending["start"] > 12:
            pending = None
        for f in stack:
            f["branches"] += len(BRANCH_RE.findall(stripped))
    return funcs


def file_kind(path: str) -> str:
    p = path.lower()
    if p.endswith("controller.ts"):
        return "controller"
    if p.endswith("service.ts"):
        return "service"
    return "other"


# ----------------------------------------------------------------------------- gates

def excluded_by(base_cfg: Path, path: str) -> bool:
    import fnmatch
    try:
        cfg = json.loads(re.sub(r"//.*", "", base_cfg.read_text()))
    except Exception:
        return False
    pats = cfg.get("exclude", [])
    if not pats and cfg.get("extends"):
        return excluded_by(base_cfg.parent / cfg["extends"], path)
    for pat in pats:
        if fnmatch.fnmatch(path, pat) or fnmatch.fnmatch(path, pat.replace("**/", "")):
            return True
    return False


def node_gates(wt: str, files: list[str], ctx: dict, skip: set, timeout: int) -> list[dict]:
    out = []
    added = {f["path"]: set(f.get("added_lines", [])) for f in ctx["files"]}
    ts = [f for f in files if re.search(r"\.[cm]?[tj]sx?$", f)]
    if not ts:
        return out
    bin_dir = Path(wt) / "node_modules" / ".bin"
    if "lint" not in skip:
        if (bin_dir / "biome").exists():
            rc, o = run([str(bin_dir / "biome"), "check", "--reporter=github", *ts], wt, timeout)
            diags = []
            for m in re.finditer(r"^::(error|warning) title=([^,]+),file=([^,]+),line=(\d+)[^:]*::(.*)$", o, re.M):
                diags.append({"level": m.group(1), "rule": m.group(2), "file": m.group(3), "line": int(m.group(4)), "msg": m.group(5)[:160]})
            on_changed = [d for d in diags if d["line"] in added.get(d["file"], set())]
            fmt = [d for d in diags if "format" in d["rule"]]
            out.append({"gate": "lint", "tool": "biome check", "rc": rc, "passed": rc == 0 or not on_changed, "diagnostics": diags, "on_changed": on_changed,
                        "format_only": bool(on_changed) and all("format" in d["rule"] for d in on_changed),
                        "summary": f"변경 라인 {len(on_changed)}건 · 변경 파일 내 기존 {len(diags)}건" + (f" (포맷 {len(fmt)})" if fmt else ""),
                        "output": "\n".join(f"{d['file']}:{d['line']} {d['rule']} {d['msg']}" for d in (on_changed or diags)[:20])})
        elif (bin_dir / "eslint").exists():
            rc, o = run([str(bin_dir / "eslint"), *ts], wt, timeout)
            out.append({"gate": "lint", "tool": "eslint", "rc": rc, "passed": rc == 0, "output": tail(o)})
    if "typecheck" not in skip and (bin_dir / "tsc").exists():
        base = next((c for c in ("tsconfig.typecheck.json", "tsconfig.json") if (Path(wt) / c).exists()), None)
        if base:
            roots = [f for f in ts if f.endswith((".ts", ".tsx")) and not excluded_by(Path(wt) / base, f)]
            if roots:
                cfg = {"extends": f"./{base}", "files": roots, "include": [], "exclude": [],
                       "compilerOptions": {"noEmit": True, "incremental": False, "composite": False}}
                tmp = Path(wt) / ".parnas.tsconfig.json"
                tmp.write_text(json.dumps(cfg))
                try:
                    rc, o = run([str(bin_dir / "tsc"), "-p", str(tmp), "--noEmit"], wt, timeout)
                finally:
                    tmp.unlink(missing_ok=True)
                errs = [l for l in o.splitlines() if re.search(r"error TS\d+", l)]
                on_changed, in_changed_files = [], []
                for l in errs:
                    m = re.match(r"^(\S+?)\((\d+),\d+\)", l)
                    if m and m.group(1) in ts:
                        in_changed_files.append(l)
                        if int(m.group(2)) in added.get(m.group(1), set()):
                            on_changed.append(l)
                out.append({"gate": "typecheck", "tool": f"tsc -p (scoped, extends {base}; roots exclude {base}'s exclude patterns)", "rc": rc,
                            "passed": rc == 0 or (not in_changed_files), "output": tail("\n".join(on_changed or in_changed_files or errs)),
                            "errors_on_changed_lines": len(on_changed), "errors_in_changed_files": len(in_changed_files), "errors_total": len(errs),
                            "summary": f"변경 라인 {len(on_changed)}건 · 변경 파일 내 기존 {len(in_changed_files)}건 · 전이 포함 {len(errs)}건"})
            else:
                out.append({"gate": "typecheck", "tool": "tsc", "rc": None, "passed": None, "summary": f"all changed .ts files are excluded by {base}", "output": ""})
    if "test" not in skip and (bin_dir / "jest").exists():
        specs = [f for f in ts if re.search(r"\.(spec|test)\.[tj]sx?$", f)]
        for f in ts:
            if re.search(r"\.(spec|test)\.[tj]sx?$", f):
                continue
            for cand in (re.sub(r"\.([tj]sx?)$", r".spec.\1", f), re.sub(r"\.([tj]sx?)$", r".test.\1", f)):
                if (Path(wt) / cand).exists() and cand not in specs:
                    specs.append(cand)
        if specs:
            cfg = next((c for c in ("test/jest.unit.config.ts", "test/jest.unit.config.js", "jest.config.ts", "jest.config.js") if (Path(wt) / c).exists()), None)
            cmd = [str(bin_dir / "jest")] + (["--config", cfg] if cfg else []) + ["--runInBand", "--silent", *specs]
            rc, o = run(cmd, wt, timeout)
            m = re.search(r"Tests:\s+(.*)", o)
            out.append({"gate": "test", "tool": "jest (changed + sibling specs)", "rc": rc, "passed": rc == 0, "specs": specs,
                        "summary": m.group(1) if m else None, "output": tail(o, 60)})
        else:
            out.append({"gate": "test", "tool": "jest", "rc": None, "passed": None, "specs": [], "summary": "no spec files touched and no sibling specs found", "output": ""})
    return out


def go_gates(wt: str, files: list[str], skip: set, timeout: int) -> list[dict]:
    out = []
    gofiles = [f for f in files if f.endswith(".go")]
    if not gofiles or not shutil.which("go"):
        return out
    pkgs = sorted({"./" + str(Path(f).parent) for f in gofiles})
    if "lint" not in skip:
        rc, o = run(["go", "vet", *pkgs], wt, timeout)
        out.append({"gate": "lint", "tool": "go vet", "rc": rc, "passed": rc == 0, "output": tail(o)})
    if "test" not in skip:
        rc, o = run(["go", "test", "-count=1", *pkgs], wt, timeout)
        out.append({"gate": "test", "tool": "go test (changed packages)", "rc": rc, "passed": rc == 0, "output": tail(o, 60)})
    return out


def python_gates(wt: str, files: list[str], skip: set, timeout: int) -> list[dict]:
    out = []
    py = [f for f in files if f.endswith(".py")]
    if not py:
        return out
    if "lint" not in skip and shutil.which("ruff"):
        rc, o = run(["ruff", "check", *py], wt, timeout)
        out.append({"gate": "lint", "tool": "ruff", "rc": rc, "passed": rc == 0, "output": tail(o)})
    tests = [f for f in py if re.search(r"(^|/)test_.*\.py$|_test\.py$", f)]
    if "test" not in skip and tests and shutil.which("pytest"):
        rc, o = run(["pytest", "-q", *tests], wt, timeout)
        out.append({"gate": "test", "tool": "pytest (changed tests)", "rc": rc, "passed": rc == 0, "output": tail(o, 60)})
    return out


# ----------------------------------------------------------------------------- main

def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--context", required=True)
    ap.add_argument("--limits", default="service=500,controller=300,method=50")
    ap.add_argument("--skip", default="")
    ap.add_argument("--timeout", type=int, default=600)
    a = ap.parse_args()
    ctx = json.loads(Path(a.context).read_text())
    wt = ctx.get("worktree") or ctx["repo_dir"]
    out_dir = Path(ctx["out_dir"])
    limits = {k: int(v) for k, v in (kv.split("=") for kv in a.limits.split(","))}
    skip = {s.strip() for s in a.skip.split(",") if s.strip()}

    files = [f for f in ctx["files"] if not f.get("deleted_file") and "generated" not in f["tags"] and "docs" not in f["tags"] and (Path(wt) / f["path"]).exists()]
    paths = [f["path"] for f in files]
    added_lines = {f["path"]: set(f.get("added_lines", [])) for f in files}

    gates = node_gates(wt, paths, ctx, skip, a.timeout) + go_gates(wt, paths, skip, a.timeout) + python_gates(wt, paths, skip, a.timeout)

    metrics, candidates = [], []
    src_added = sum(f["added"] for f in files if "test" not in f["tags"])
    test_added = sum(f["added"] for f in files if "test" in f["tags"])
    base_sha = ctx["diff_refs"].get("base_sha")
    for f in files:
        if "test" in f["tags"]:
            continue
        text = (Path(wt) / f["path"]).read_text(errors="ignore")
        loc = len(text.splitlines())
        funcs = function_metrics(text)
        base_text = ""
        if base_sha and not f.get("new_file"):
            rc, base_text = run(["git", "show", f"{base_sha}:{f.get('old_path') or f['path']}"], wt, 30)
            base_text = base_text if rc == 0 else ""
        base_loc = len(base_text.splitlines()) if base_text else 0
        base_fn = {fn["name"]: fn["loc"] for fn in function_metrics(base_text)} if base_text else {}
        touched = [fn for fn in funcs if any(fn["start"] <= l <= fn["end"] for l in added_lines[f["path"]])]
        worst = max(touched or funcs or [{"name": "-", "loc": 0, "cyclomatic": 0, "nesting": 0, "start": 0}], key=lambda x: x["loc"])
        over = [fn for fn in touched if fn["loc"] > limits["method"]]
        kind = file_kind(f["path"])
        file_limit = limits.get(kind)
        m = {"path": f["path"], "kind": kind, "loc": loc, "base_loc": base_loc, "file_limit": file_limit, "added": f["added"], "removed": f["removed"],
             "touched_functions": len(touched), "longest_touched_fn": {"name": worst["name"], "loc": worst["loc"], "cyclomatic": worst["cyclomatic"], "nesting": worst["nesting"], "line": worst["start"]},
             "functions_over_method_limit": [{"name": fn["name"], "loc": fn["loc"], "line": fn["start"]} for fn in over]}
        metrics.append(m)
        if file_limit and loc > file_limit and f["added"] > 0:
            first_added = min(added_lines[f["path"]]) if added_lines[f["path"]] else 1
            pre = base_loc > file_limit
            candidates.append({"path": f["path"], "new_line": first_added, "severity": "low", "category": "rule", "confidence": 60 if pre else 90, "pre_existing": pre,
                               "title": f"{kind} 파일 LOC 초과: {loc} (한도 {file_limit}{', 변경 전 ' + str(base_loc) if pre else ''})",
                               "what": f"`{f['path'].split('/')[-1]}` 가 {base_loc} → {loc} LOC (+{f['added']}) 로 {kind} 한도 {file_limit} 을 넘습니다." + (" 초과는 변경 전부터 있었습니다." if pre else ""),
                               "why": "CLAUDE.md 파일 LOC 한도 위반. 파일이 커질수록 영향 범위 추적과 리뷰 정확도가 떨어집니다.",
                               "how": "이번에 추가한 응집도 높은 블록(헬퍼·유틸)을 별도 파일로 분리하면 됩니다. 기존 코드 이동은 요구하지 않습니다.",
                               "metrics": {"loc": loc, "base_loc": base_loc, "limit": file_limit, "kind": kind},
                               "evidence": [f"{f['path']} — wc -l = {loc} (limit {file_limit})"], "rule": "CLAUDE.md LOC Limits", "source": "gate:loc"})
        for fn in over:
            base_len = base_fn.get(fn["name"], 0)
            pre = base_len > limits["method"]
            grew = fn["loc"] - base_len
            anchor = min((l for l in added_lines[f["path"]] if fn["start"] <= l <= fn["end"]), default=fn["start"])
            minor = (fn["loc"] - limits["method"]) <= 10 and fn["cyclomatic"] <= 3
            candidates.append({"path": f["path"], "new_line": anchor, "severity": "low", "category": "rule", "confidence": 60 if pre else (70 if minor else 90), "pre_existing": pre, "minor": minor,
                               "metrics": {"loc": fn["loc"], "base_loc": base_len, "cyclomatic": fn["cyclomatic"], "nesting": fn["nesting"], "start": fn["start"], "end": fn["end"], "limit": limits["method"]},
                               "title": f"`{fn['name']}` 메서드 LOC 초과: {fn['loc']} (한도 {limits['method']}{', 변경 전 ' + str(base_len) if pre else ''})",
                               "what": f"`{fn['name']}` ({fn['start']}행) 가 {base_len} → {fn['loc']} LOC 로 늘었습니다." if not pre else f"`{fn['name']}` ({fn['start']}행) 는 변경 전에도 {base_len} LOC 로 한도를 넘었고 이번에 {grew:+d} 줄 변했습니다.",
                               "why": f"CLAUDE.md 메서드 한도({limits['method']}) 위반. 길어질수록 단위 테스트와 원인 추적이 어려워집니다.",
                               "how": f"이번에 추가한 블록({f['path'].split('/')[-1]}:{anchor}~ 부근)을 private 메서드로 빼는 정도면 충분합니다(동작 변경 없음)." if not pre else "이번에 추가한 블록만 헬퍼로 빼서 더 커지지 않게 하면 됩니다(기존 코드 이동은 요구하지 않음).",
                               "evidence": [f"{f['path']}:{fn['start']}-{fn['start'] + fn['loc'] - 1} — {fn['loc']} LOC, cyclomatic≈{fn['cyclomatic']}, nesting {fn['nesting']}, base {base_len} LOC"],
                               "rule": "CLAUDE.md LOC Limits", "source": "gate:method-loc"})
    for g in gates:
        if g.get("passed") is False:
            n = g["gate"]
            fmt_only = g.get("format_only")
            first = paths[0] if paths else None
            line_m = re.search(r"^(\S+?)[\(:](\d+)", g.get("output", ""), re.M)
            path, line = (line_m.group(1), int(line_m.group(2))) if line_m and line_m.group(1) in paths else (first, 1)
            if g.get("on_changed"):
                path, line = g["on_changed"][0]["file"], g["on_changed"][0]["line"]
            if line not in added_lines.get(path, set()):
                line = min(added_lines[path]) if added_lines.get(path) else 1
            candidates.append({"path": path, "new_line": line, "severity": "high" if n in ("typecheck", "test") else ("low" if fmt_only else "medium"), "category": "bug" if n != "lint" else "rule",
                               "confidence": 95, "title": (f"{g['tool']} 포맷 미적용 파일이 있습니다" if fmt_only else f"{g['tool']} 가 변경 파일에서 실패합니다"), "what": f"게이트 `{n}` 실패 (rc={g['rc']}). {g.get('summary') or ''}",
                               "why": "같은 실패가 CI 에서도 재현되어 병합을 막습니다.", "how": "아래 출력의 첫 오류부터 고치면 됩니다.",
                               "evidence": [f"{g['tool']} → rc={g['rc']}", *g.get("output", "").splitlines()[:8]], "source": f"gate:{n}"})

    for i, c in enumerate(candidates, 1):
        c["id"] = f"G{i}"
        c.setdefault("verification", f"quality_gate.py {c['source']} (deterministic, vs base_sha)")
        if c["new_line"] not in added_lines.get(c["path"], set()):
            c["summary_only"] = True
            c["confidence"] = min(c["confidence"], 50)
    ratio = round(test_added / src_added, 2) if src_added else None
    result = {"worktree": wt, "limits": limits, "gates": gates, "metrics": metrics, "test_to_source_added_ratio": ratio,
              "src_added": src_added, "test_added": test_added, "candidates": candidates}
    (out_dir / "gate.json").write_text(json.dumps(result, ensure_ascii=False, indent=1))

    L = ["### 자동 검사 (변경 파일 기준)", "", "| 검사 | 도구 | 결과 | 비고 |", "|---|---|---|---|"]
    for g in gates:
        res = "✅ 통과" if g.get("passed") else ("⏭ 생략" if g.get("passed") is None else "❌ 실패")
        tool = g["tool"].split(" (")[0]
        note = (g.get("summary") or "").replace("passed, ", "통과 / ").replace(" total", " 전체").replace("failed", "실패")
        L.append(f"| {g['gate']} | {tool} | {res} | {note} |")
    L += ["", f"테스트 : 소스 추가 비율 **{ratio if ratio is not None else '-'}** (+{test_added} / +{src_added})", ""]
    rows, hidden = [], []
    for m in metrics:
        w = m["longest_touched_fn"]
        delta = f" ({m['loc'] - m['base_loc']:+d})" if m["base_loc"] else ""
        file_over = bool(m["file_limit"] and m["loc"] > m["file_limit"])
        file_pre = file_over and m["base_loc"] > m["file_limit"]
        flag = (" ⚠(기존)" if file_pre else " ⚠") if file_over else ""
        lim = f"{m['loc']}{delta} / {m['file_limit']}" if m["file_limit"] else f"{m['loc']}{delta}"
        fn_over = w["loc"] > limits["method"]
        row = f"| `{m['path'].split('/')[-1]}` | {lim}{flag} | {m['touched_functions']} | `{w['name']}` | {w['loc']}{' ⚠' if fn_over else ''} | {w['cyclomatic']} | {w['nesting']} |"
        (rows if (file_over or fn_over) else hidden).append(row)
    header = ["| 파일 | LOC (Δ) / 한도 | 변경 함수 | 최장 변경 함수 | LOC | 복잡도≈ | 중첩 |", "|---|---|---|---|---|---|---|"]
    if rows:
        L += header + rows + [""]
    if hidden:
        L += [f"<details><summary>한도 이내 파일 {len(hidden)}개</summary>", ""] + header + hidden + ["", "</details>", ""]
    md = "\n".join(L)
    (out_dir / "gate.md").write_text(md)
    print(md)


if __name__ == "__main__":
    main()
