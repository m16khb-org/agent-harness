#!/usr/bin/env python3
"""E2E operational stability audit for agent-harness.

Default mode avoids destructive cleanup and real host install. Use --full-install
for installed-surface verification and --cleanup-stale to terminate confirmed
legacy/temp harness-owned leftovers.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Any

ROOT = Path.cwd()
BIN = ROOT / "bin" / "agent-harness"
LEGACY_BIN_RE = re.compile(r"/bin/harness (daemon --internal|mcp)\b")
HARNESS_DAEMON_RE = re.compile(r"agent-harness daemon --internal")
TEMP_WATCHER_RE = re.compile(r"scripts/codegraph-watcher\.mjs .*/T/tmp\.")


def run(cmd: list[str], *, env: dict[str, str] | None = None, input_text: str | None = None, timeout: int = 60) -> dict[str, Any]:
    merged = os.environ.copy()
    if env:
        merged.update(env)
    start = time.time()
    proc = subprocess.run(
        cmd,
        input=input_text,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=merged,
        timeout=timeout,
    )
    return {
        "cmd": cmd,
        "returncode": proc.returncode,
        "stdout": proc.stdout,
        "stderr": proc.stderr,
        "duration_ms": int((time.time() - start) * 1000),
    }


def add_step(report: dict[str, Any], name: str, ok: bool, **data: Any) -> None:
    item = {"name": name, "ok": bool(ok), **data}
    report["steps"].append(item)
    if not ok:
        report["failures"].append(item)


def parse_json_output(text: str) -> Any:
    start = text.find("{")
    if start < 0:
        raise ValueError("no JSON object in output")
    return json.loads(text[start:])


def ps_rows() -> list[dict[str, Any]]:
    proc = subprocess.run(["ps", "-axo", "pid,ppid,state,rss,command"], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    rows: list[dict[str, Any]] = []
    for line in proc.stdout.splitlines()[1:]:
        parts = line.strip().split(None, 4)
        if len(parts) < 5:
            continue
        pid, ppid, state, rss, command = parts
        rows.append({"pid": int(pid), "ppid": int(ppid), "state": state, "rss_kb": int(rss), "command": command})
    return rows


def process_alive(pid: int) -> bool:
    return subprocess.run(["kill", "-0", str(pid)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0


def terminate(pid: int) -> str:
    try:
        os.kill(pid, signal.SIGTERM)
        time.sleep(0.4)
        if process_alive(pid):
            os.kill(pid, signal.SIGKILL)
            return "sigkill"
        return "sigterm"
    except ProcessLookupError:
        return "gone"
    except PermissionError:
        return "permission_denied"


def classify_processes(rows: list[dict[str, Any]]) -> dict[str, Any]:
    current_daemons = [r for r in rows if HARNESS_DAEMON_RE.search(r["command"])]
    legacy = [r for r in rows if LEGACY_BIN_RE.search(r["command"])]
    temp_watchers = [r for r in rows if TEMP_WATCHER_RE.search(r["command"])]
    zombies = [r for r in rows if "Z" in r["state"] and re.search(r"agent-harness|bin/harness|codegraph", r["command"])]
    return {"current_daemons": current_daemons, "legacy_harness": legacy, "temp_watchers": temp_watchers, "zombies": zombies}


def build(report: dict[str, Any]) -> None:
    res = run(["go", "build", "-o", str(BIN), "./cmd/harness"], timeout=120)
    add_step(report, "build", res["returncode"] == 0, result={k: res[k] for k in ("returncode", "stderr", "duration_ms")})


def install_checks(report: dict[str, Any], full_install: bool) -> None:
    for name, cmd in [
        ("bootstrap_dry_json", [str(BIN), "bootstrap", "--skip-upstream-tools", "--dry-run", "--json"]),
        ("update_dry_json", [str(BIN), "update", "--skip-upstream-tools", "--dry-run", "--json"]),
        ("install_native_dry_json", [str(BIN), "install-native", "--dry-run", "--json"]),
    ]:
        res = run(cmd, timeout=120)
        ok = res["returncode"] == 0
        parsed = None
        if ok:
            try:
                parsed = parse_json_output(res["stdout"])
                ok = bool(parsed.get("ok"))
            except Exception as exc:
                ok = False
                parsed = {"parse_error": str(exc)}
        add_step(report, name, ok, parsed=parsed, stderr=res["stderr"][-1000:])
    if full_install:
        res = run([str(BIN), "install-native", "--json"], timeout=120)
        parsed = None
        ok = res["returncode"] == 0
        if ok:
            try:
                parsed = parse_json_output(res["stdout"])
                ok = bool(parsed.get("ok"))
            except Exception as exc:
                ok = False
                parsed = {"parse_error": str(exc)}
        add_step(report, "install_native_real_json", ok, parsed=parsed, stderr=res["stderr"][-1000:])


def hook_smoke(report: dict[str, Any]) -> None:
    hooks_path = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex"))) / "hooks.json"
    if not hooks_path.exists():
        add_step(report, "hook_smoke", False, message=f"missing {hooks_path}")
        return
    hooks = json.loads(hooks_path.read_text())
    payloads = {
        "UserPromptSubmit": {"prompt": "stability audit smoke", "cwd": str(ROOT)},
        "PreToolUse": {"tool_name": "shell", "tool_input": {"command": "git status"}, "cwd": str(ROOT)},
        "PostToolUse": {"tool_name": "shell", "tool_input": {"command": "git status"}, "tool_response": {"exit_code": 0}, "cwd": str(ROOT)},
        "Notification": {"message": "smoke", "cwd": str(ROOT)},
        "Stop": {"stop_hook_active": False, "cwd": str(ROOT)},
        "SubagentStop": {"stop_hook_active": False, "cwd": str(ROOT)},
        "SessionStart": {"source": "startup", "cwd": str(ROOT)},
        "SessionEnd": {"reason": "smoke", "cwd": str(ROOT)},
        "PreCompact": {"trigger": "manual", "cwd": str(ROOT)},
        "PostCompact": {"cwd": str(ROOT)},
    }
    results = []
    ok = True
    for event, matchers in hooks.get("hooks", {}).items():
        for matcher_index, matcher in enumerate(matchers):
            for hook_index, hook in enumerate(matcher.get("hooks", [])):
                cmd = hook.get("command")
                res = subprocess.run(
                    cmd,
                    input=json.dumps(payloads.get(event, {"cwd": str(ROOT)})),
                    text=True,
                    shell=True,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    timeout=8,
                )
                item: dict[str, Any] = {"event": event, "matcher": matcher_index, "hook": hook_index, "returncode": res.returncode, "stdout_head": res.stdout[:240], "stderr_head": res.stderr[:240]}
                if res.returncode != 0:
                    ok = False
                    item["error"] = "non_zero_exit"
                elif res.stdout.strip():
                    try:
                        obj = json.loads(res.stdout)
                        if "suppressOutput" in obj:
                            ok = False
                            item["error"] = "unsupported_suppressOutput"
                        if event in ("Stop", "SubagentStop") and set(obj) - {"decision", "reason"}:
                            ok = False
                            item["error"] = "invalid_stop_json_keys"
                        if event == "UserPromptSubmit":
                            ctx = obj.get("hookSpecificOutput", {}).get("additionalContext", "")
                            if "\n" in ctx or "Required project docs" in ctx or "필수 프롬프트 주입중" in ctx:
                                ok = False
                                item["error"] = "noisy_user_prompt_context"
                    except Exception as exc:
                        ok = False
                        item["error"] = f"invalid_json:{exc}"
                results.append(item)
    add_step(report, "hook_smoke", ok, results=results)


def temp_state_worker_policy(report: dict[str, Any]) -> None:
    with tempfile.TemporaryDirectory() as state, tempfile.TemporaryDirectory() as worker:
        env_state = {"HARNESS_STATE_DIR": state}
        env_worker = {"HARNESS_WORKER_DIR": worker}
        commands = [
            ("state_migrate", [str(BIN), "state", "migrate", "--json"], env_state),
            ("state_write", [str(BIN), "state", "write", "--key", "stability-audit", "--value", "ok", "--json"], env_state),
            ("state_read", [str(BIN), "state", "read", "--key", "stability-audit", "--json"], env_state),
            ("state_doctor", [str(BIN), "state", "doctor", "--json"], env_state),
            ("policy_check", [str(BIN), "policy", "check", "--workspace-root", str(ROOT), "--cwd", str(ROOT), "--json", "--", "git", "status", "--short"], None),
            ("worker_enqueue", [str(BIN), "worker", "enqueue", "--kind", "stability-audit", "--payload", "{}", "--json"], env_worker),
            ("worker_list", [str(BIN), "worker", "list", "--json"], env_worker),
            ("worker_run", [str(BIN), "worker", "run", "--read-only", "--kind", "stability-audit", "--workspace-root", str(ROOT), "--cwd", str(ROOT), "--json", "--", "git", "status", "--short"], env_worker),
        ]
        details = []
        ok = True
        for name, cmd, env in commands:
            res = run(cmd, env=env, timeout=30)
            parsed = None
            step_ok = res["returncode"] == 0
            if step_ok:
                try:
                    parsed = parse_json_output(res["stdout"])
                    step_ok = bool(parsed.get("ok", True))
                except Exception as exc:
                    step_ok = False
                    parsed = {"parse_error": str(exc)}
            ok = ok and step_ok
            details.append({"name": name, "ok": step_ok, "parsed": parsed, "stderr": res["stderr"][-500:]})
        add_step(report, "state_worker_policy", ok, details=details)


def daemon_and_mcp_stress(report: dict[str, Any], cycles: int) -> None:
    baseline = {r["pid"] for r in classify_processes(ps_rows())["current_daemons"] + classify_processes(ps_rows())["legacy_harness"]}
    with tempfile.TemporaryDirectory() as td:
        env = {"HARNESS_DAEMON_DIR": str(Path(td) / "daemon"), "HARNESS_STATE_DIR": str(Path(td) / "state"), "HARNESS_ROOT": str(ROOT)}
        ok = True
        cycle_details = []
        for i in range(cycles):
            start = run([str(BIN), "daemon", "start", "--json"], env=env, timeout=20)
            pid = 0
            try:
                obj = parse_json_output(start["stdout"])
                pid = int(obj.get("pid") or 0)
            except Exception:
                ok = False
            status = run([str(BIN), "daemon", "status", "--json"], env=env, timeout=10)
            stop = run([str(BIN), "daemon", "stop", "--json"], env=env, timeout=10)
            time.sleep(0.08)
            alive = bool(pid and process_alive(pid))
            if start["returncode"] or status["returncode"] or stop["returncode"] or alive:
                ok = False
            cycle_details.append({"cycle": i, "pid": pid, "alive_after_stop": alive, "start_rc": start["returncode"], "status_rc": status["returncode"], "stop_rc": stop["returncode"]})
        # Standalone MCP JSON-RPC smoke.
        proc = subprocess.Popen([str(BIN), "mcp"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env={**os.environ, **env})
        calls = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "stability-audit", "version": "1"}}},
            {"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
            {"jsonrpc": "2.0", "id": 3, "method": "resources/list", "params": {}},
            {"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "harness_inspect", "arguments": {}}},
            {"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "docs_index", "arguments": {}}},
            {"jsonrpc": "2.0", "id": 6, "method": "tools/call", "params": {"name": "state_doctor", "arguments": {}}},
            {"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": {"name": "project_docs_route", "arguments": {"task": "install hook mcp daemon operations"}}},
            {"jsonrpc": "2.0", "id": 8, "method": "tools/call", "params": {"name": "daemon_status", "arguments": {}}},
        ]
        assert proc.stdin is not None
        for call in calls:
            proc.stdin.write(json.dumps(call) + "\n")
            proc.stdin.flush()
        proc.stdin.close()
        out, err = proc.communicate(timeout=15)
        ids = []
        rpc_errors = []
        for line in out.splitlines():
            if line.startswith("{"):
                obj = json.loads(line)
                ids.append(obj.get("id"))
                if "error" in obj:
                    rpc_errors.append(obj)
        if ids != [1, 2, 3, 4, 5, 6, 7, 8] or rpc_errors:
            ok = False
        st = run([str(BIN), "daemon", "status", "--json"], env=env, timeout=10)
        try:
            temp_pid = int(parse_json_output(st["stdout"]).get("pid") or 0)
        except Exception:
            temp_pid = 0
        run([str(BIN), "daemon", "stop", "--json"], env=env, timeout=10)
        time.sleep(0.1)
        temp_leaked = bool(temp_pid and process_alive(temp_pid))
        if temp_leaked:
            ok = False
        after = {r["pid"] for r in classify_processes(ps_rows())["current_daemons"] + classify_processes(ps_rows())["legacy_harness"]}
        new_pids = sorted(after - baseline)
        add_step(report, "daemon_mcp_stress", ok and not new_pids, cycles=cycle_details, mcp_ids=ids, rpc_errors=rpc_errors, temp_mcp_pid=temp_pid, temp_mcp_leaked=temp_leaked, new_daemon_pids_after_stress=new_pids, mcp_stderr=err[-1000:])


def rss_sample(report: dict[str, Any], rounds: int, calls: int) -> None:
    status = run([str(BIN), "daemon", "status", "--json"], timeout=10)
    try:
        pid = int(parse_json_output(status["stdout"]).get("pid") or 0)
    except Exception:
        add_step(report, "rss_stability", False, message="cannot read daemon pid")
        return
    if not pid:
        add_step(report, "rss_stability", False, message="daemon not running")
        return
    samples = []
    for i in range(rounds):
        before = int(subprocess.check_output(["ps", "-o", "rss=", "-p", str(pid)], text=True).strip() or "0")
        for _ in range(calls):
            run([str(BIN), "daemon", "status", "--json"], timeout=5)
        after = int(subprocess.check_output(["ps", "-o", "rss=", "-p", str(pid)], text=True).strip() or "0")
        samples.append({"round": i + 1, "before_kb": before, "after_kb": after, "delta_kb": after - before})
    # Allow Go runtime warmup; fail only if every later round grows materially.
    suspicious = len(samples) >= 3 and all(s["delta_kb"] > 2048 for s in samples[1:])
    add_step(report, "rss_stability", not suspicious, pid=pid, samples=samples, caveat="single warmup jumps are not treated as leaks; repeated post-warmup growth >2MB is suspicious")


def host_mcp_checks(report: dict[str, Any]) -> None:
    details = []
    ok = True
    if shutil.which("codex"):
        res = run(["codex", "mcp", "get", "agent_harness"], timeout=20)
        step_ok = res["returncode"] == 0 and "agent-harness" in (res["stdout"] + res["stderr"])
        ok = ok and step_ok
        details.append({"name": "codex_mcp_get", "ok": step_ok, "stdout_head": res["stdout"][:500], "stderr_head": res["stderr"][:500]})
    if shutil.which("claude"):
        res = run(["claude", "mcp", "list"], timeout=40)
        text = res["stdout"] + res["stderr"]
        conflict = "Conflicting scopes" in text and "agent_harness" in text
        legacy = "/bin/harness mcp" in text
        step_ok = res["returncode"] == 0 and not conflict and not legacy
        ok = ok and step_ok
        details.append({"name": "claude_mcp_list", "ok": step_ok, "conflict": conflict, "legacy_bin_harness": legacy, "output_head": text[:1000]})
    add_step(report, "host_mcp_checks", ok, details=details)


def cleanup_stale(report: dict[str, Any], enabled: bool) -> None:
    classified = classify_processes(ps_rows())
    stale = classified["legacy_harness"] + classified["temp_watchers"]
    actions = []
    if enabled:
        for proc in stale:
            actions.append({"pid": proc["pid"], "command": proc["command"], "action": terminate(proc["pid"])})
    add_step(report, "process_hygiene", not classified["zombies"], classified=classified, cleanup_enabled=enabled, cleanup_actions=actions)


def regression(report: dict[str, Any], race: bool, self_verify: bool) -> None:
    commands = [["go", "test", "./...", "-count=1"]]
    if race:
        commands.append(["go", "test", "-race", "./...", "-count=1"])
    commands.append(["go", "build", "-o", str(BIN), "./cmd/harness"])
    details = []
    ok = True
    for cmd in commands:
        res = run(cmd, timeout=180)
        step_ok = res["returncode"] == 0
        ok = ok and step_ok
        details.append({"cmd": cmd, "ok": step_ok, "stderr_tail": res["stderr"][-1000:], "stdout_tail": res["stdout"][-1000:], "duration_ms": res["duration_ms"]})
    if self_verify:
        with tempfile.TemporaryDirectory() as td:
            res = run([str(BIN), "self-verify", "--iterations=10", "--seed=100", "--target-score=95", "--json"], env={"HARNESS_STATE_DIR": td}, timeout=120)
            parsed = None
            step_ok = res["returncode"] == 0
            if step_ok:
                try:
                    parsed = parse_json_output(res["stdout"])
                    step_ok = bool(parsed.get("ok")) and bool(parsed.get("termination_eligible", True))
                except Exception as exc:
                    step_ok = False
                    parsed = {"parse_error": str(exc)}
            ok = ok and step_ok
            details.append({"cmd": "self-verify", "ok": step_ok, "summary": (parsed or {}).get("summary") if isinstance(parsed, dict) else parsed, "duration_ms": res["duration_ms"]})
    add_step(report, "regression", ok, details=details)


def main() -> int:
    parser = argparse.ArgumentParser(description="Run agent-harness E2E stability audit")
    parser.add_argument("--full-install", action="store_true", help="run real install-native after dry-run checks")
    parser.add_argument("--cleanup-stale", action="store_true", help="terminate confirmed legacy/temp harness-owned stale processes")
    parser.add_argument("--skip-race", action="store_true", help="skip go test -race")
    parser.add_argument("--skip-self-verify", action="store_true", help="skip 10-iteration self-verify")
    parser.add_argument("--daemon-cycles", type=int, default=8)
    parser.add_argument("--rss-rounds", type=int, default=3)
    parser.add_argument("--rss-calls", type=int, default=200)
    parser.add_argument("--json", action="store_true", help="print JSON report")
    args = parser.parse_args()

    report: dict[str, Any] = {
        "ok": True,
        "root": str(ROOT),
        "full_install": args.full_install,
        "cleanup_stale": args.cleanup_stale,
        "steps": [],
        "failures": [],
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }

    add_step(report, "git_status", True, output=run(["git", "status", "--short", "--branch"], timeout=10)["stdout"])
    build(report)
    install_checks(report, args.full_install)
    host_mcp_checks(report)
    hook_smoke(report)
    temp_state_worker_policy(report)
    daemon_and_mcp_stress(report, max(1, args.daemon_cycles))
    cleanup_stale(report, args.cleanup_stale)
    rss_sample(report, max(1, args.rss_rounds), max(1, args.rss_calls))
    regression(report, not args.skip_race, not args.skip_self_verify)

    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    report["ok"] = not report["failures"]
    if args.json:
        print(json.dumps(report, ensure_ascii=False, indent=2))
    else:
        print(f"stability audit ok={report['ok']} failures={len(report['failures'])}")
        for failure in report["failures"]:
            print(f"FAIL {failure['name']}")
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
