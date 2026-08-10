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
import queue
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path
from typing import Any

ROOT = Path.cwd()
BIN = ROOT / "bin" / "agent-harness"
LEGACY_BIN_RE = re.compile(r"/bin/harness (daemon --internal|mcp)\b")
HARNESS_DAEMON_RE = re.compile(r"agent-harness daemon --internal")
TEMP_WATCHER_RE = re.compile(r"scripts/codegraph-watcher\.mjs .*/T/tmp\.")
REGRESSION_TIMEOUT_SECONDS = 300
FULL_SELF_VERIFY_TIMEOUT_SECONDS = 5400
DOCTOR_ISSUE_LIMIT = 20
DOCTOR_CODE_LIMIT = 96
DOCTOR_SUMMARY_LIMIT = 320
TERMINAL_HANDLE_MAX_BYTES = 256
TERMINAL_HANDLE_RE = re.compile(r"^term_[A-Za-z0-9_-]+$")


def run(cmd: list[str], *, env: dict[str, str] | None = None, input_text: str | None = None, timeout: float = 60) -> dict[str, Any]:
    merged = os.environ.copy()
    if env:
        merged.update(env)
    start = time.time()
    try:
        proc = subprocess.run(
            cmd,
            input=input_text,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=merged,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or ""
        stderr = exc.stderr or ""
        if isinstance(stdout, bytes):
            stdout = stdout.decode(errors="replace")
        if isinstance(stderr, bytes):
            stderr = stderr.decode(errors="replace")
        return {
            "cmd": cmd,
            "returncode": -1,
            "stdout": stdout,
            "stderr": f"{stderr}\ncommand timed out after {timeout} seconds".strip(),
            "duration_ms": int((time.time() - start) * 1000),
            "timeout": True,
        }
    return {
        "cmd": cmd,
        "returncode": proc.returncode,
        "stdout": proc.stdout,
        "stderr": proc.stderr,
        "duration_ms": int((time.time() - start) * 1000),
        "timeout": False,
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


def is_noisy_user_prompt_context(ctx: str) -> bool:
    return "Required project docs" in ctx or "필수 프롬프트 주입중" in ctx


def mcp_smoke_env(env: dict[str, str]) -> dict[str, str]:
    return {**env, "HARNESS_MCP_DIRECT": "1"}


def run_mcp_jsonrpc_process(
    command: list[str],
    calls: list[dict[str, Any]],
    *,
    env: dict[str, str] | None = None,
    timeout: float,
) -> dict[str, Any]:
    expected_ids = [call["id"] for call in calls if "id" in call]
    proc = subprocess.Popen(
        command,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
        env={**os.environ, **(env or {})},
    )
    assert proc.stdin is not None and proc.stdout is not None and proc.stderr is not None
    events: queue.Queue[tuple[str, str | None]] = queue.Queue()

    def drain(name: str, stream: Any) -> None:
        for line in iter(stream.readline, ""):
            events.put((name, line))
        events.put((name, None))

    readers = [
        threading.Thread(target=drain, args=("stdout", proc.stdout), daemon=True),
        threading.Thread(target=drain, args=("stderr", proc.stderr), daemon=True),
    ]
    for reader in readers:
        reader.start()

    write_error = ""
    try:
        for call in calls:
            proc.stdin.write(json.dumps(call) + "\n")
        proc.stdin.flush()
    except (BrokenPipeError, OSError) as exc:
        write_error = str(exc)

    response_ids: list[Any] = []
    duplicate_ids: list[Any] = []
    unexpected_ids: list[Any] = []
    malformed_lines: list[str] = []
    rpc_errors: list[dict[str, Any]] = []
    stdout_lines: list[str] = []
    stderr_lines: list[str] = []

    def consume(name: str, line: str | None) -> None:
        if line is None:
            return
        if name == "stderr":
            stderr_lines.append(line)
            return
        stdout_lines.append(line)
        if not line.lstrip().startswith("{"):
            if line.strip():
                malformed_lines.append(line.rstrip("\n"))
            return
        try:
            response = json.loads(line)
        except json.JSONDecodeError:
            malformed_lines.append(line.rstrip("\n"))
            return
        if not isinstance(response, dict) or response.get("jsonrpc") != "2.0":
            malformed_lines.append(line.rstrip("\n"))
            return
        if "id" not in response:
            malformed_lines.append(line.rstrip("\n"))
            return
        response_id = response.get("id")
        if isinstance(response_id, bool) or not isinstance(response_id, (str, int, float)):
            malformed_lines.append(line.rstrip("\n"))
            return
        has_result = "result" in response
        has_error = "error" in response
        if has_result == has_error:
            malformed_lines.append(line.rstrip("\n"))
            return
        if has_error:
            error = response["error"]
            if (
                not isinstance(error, dict)
                or isinstance(error.get("code"), bool)
                or not isinstance(error.get("code"), int)
                or not isinstance(error.get("message"), str)
            ):
                malformed_lines.append(line.rstrip("\n"))
                return
        response_ids.append(response_id)
        if response_ids.count(response_id) > 1:
            duplicate_ids.append(response_id)
        if response_id not in expected_ids:
            unexpected_ids.append(response_id)
        if "error" in response:
            rpc_errors.append(response)

    deadline = time.monotonic() + timeout
    stdout_closed = False
    stderr_closed = False
    while time.monotonic() < deadline and not all(response_ids.count(item) == 1 for item in expected_ids):
        try:
            name, line = events.get(timeout=max(0.01, deadline - time.monotonic()))
        except queue.Empty:
            break
        consume(name, line)
        if line is None:
            if name == "stdout":
                stdout_closed = True
            else:
                stderr_closed = True
            if proc.poll() is not None and stdout_closed and stderr_closed:
                break

    received_all = all(response_ids.count(item) == 1 for item in expected_ids)
    stdin_closed_before_responses = not received_all
    proc.stdin.close()
    proc.stdin = None
    timed_out = False
    try:
        proc.wait(timeout=max(0.01, deadline - time.monotonic()))
    except subprocess.TimeoutExpired:
        timed_out = True
        proc.terminate()
        try:
            proc.wait(timeout=1)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=1)

    for reader in readers:
        reader.join(timeout=1)
    while not events.empty():
        name, line = events.get_nowait()
        consume(name, line)
    proc.stdout.close()
    proc.stderr.close()

    missing_ids = [item for item in expected_ids if response_ids.count(item) == 0]
    ok = (
        not write_error
        and not timed_out
        and proc.returncode == 0
        and not malformed_lines
        and not duplicate_ids
        and not unexpected_ids
        and not missing_ids
        and not rpc_errors
    )
    return {
        "ok": ok,
        "response_ids": response_ids,
        "duplicate_ids": duplicate_ids,
        "unexpected_ids": unexpected_ids,
        "missing_ids": missing_ids,
        "malformed_lines": malformed_lines,
        "rpc_errors": rpc_errors,
        "timed_out": timed_out,
        "returncode": proc.returncode,
        "stdin_closed_before_responses": stdin_closed_before_responses,
        "stdout": "".join(stdout_lines),
        "stderr": "".join(stderr_lines),
        "write_error": write_error,
    }


def ps_rows() -> list[dict[str, Any]]:
    ps_bin = shutil.which("ps") or "/bin/ps"
    try:
        proc = subprocess.run([ps_bin, "-axo", "pid,ppid,state,rss,command"], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    except (PermissionError, FileNotFoundError, OSError) as exc:
        print(f"[stability-audit] ps ({ps_bin}) unavailable — skipping process enumeration: {exc}", file=sys.stderr)
        return []
    rows: list[dict[str, Any]] = []
    for line in proc.stdout.splitlines()[1:]:
        parts = line.strip().split(None, 4)
        if len(parts) < 5:
            continue
        pid, ppid, state, rss, command = parts
        try:
            rows.append({"pid": int(pid), "ppid": int(ppid), "state": state, "rss_kb": int(rss), "command": command})
        except (ValueError, IndexError):
            continue
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


def _terminal_handle_arg(value: str) -> str:
    if len(value.encode("utf-8")) > TERMINAL_HANDLE_MAX_BYTES or TERMINAL_HANDLE_RE.fullmatch(value) is None:
        raise argparse.ArgumentTypeError("must be a term_* handle of at most 256 bytes")
    return value


def operational_doctor(report: dict[str, Any], preserve_terminal: str | None = None) -> None:
    cmd = [str(BIN), "doctor", "--repo", str(ROOT), "--sealed", "--json"]
    terminal = (
        _terminal_handle_arg(preserve_terminal)
        if preserve_terminal is not None
        else os.environ.get("ORCA_TERMINAL_HANDLE", "").strip()
    )
    if terminal:
        cmd.extend(["--preserve-terminal", terminal])
    res = run(cmd, timeout=120)

    parsed: dict[str, Any] = {}
    parse_error = ""
    try:
        value = parse_json_output(res["stdout"])
        if isinstance(value, dict):
            parsed = value
        else:
            parse_error = "doctor output is not a JSON object"
    except Exception as exc:
        parse_error = str(exc)[:DOCTOR_SUMMARY_LIMIT]

    ok = res["returncode"] == 0 and bool(parsed.get("ok")) and bool(parsed.get("healthy"))
    details: dict[str, Any] = {
        "returncode": res["returncode"],
        "doctor_ok": bool(parsed.get("ok")),
        "healthy": bool(parsed.get("healthy")),
    }
    if not ok:
        issues = []
        raw_issues = parsed.get("issues")
        if isinstance(raw_issues, list):
            for raw in raw_issues[:DOCTOR_ISSUE_LIMIT]:
                if not isinstance(raw, dict):
                    continue
                raw_code = raw.get("code")
                raw_summary = raw.get("summary")
                code = raw_code[:DOCTOR_CODE_LIMIT] if isinstance(raw_code, str) else ""
                summary = raw_summary[:DOCTOR_SUMMARY_LIMIT] if isinstance(raw_summary, str) else ""
                if code or summary:
                    issues.append({"code": code, "summary": summary})
        details["issues"] = issues
        if parse_error:
            details["parse_error"] = parse_error
    add_step(report, "operational_doctor", ok, **details)


def install_checks(report: dict[str, Any], full_install: bool) -> None:
    for name, cmd in [
        ("bootstrap_dry_json", [str(BIN), "bootstrap", "--dry-run", "--json"]),
        ("install_dry_json", [str(BIN), "install", "--dry-run", "--json"]),
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
        res = run([str(BIN), "install", "--json"], timeout=120)
        parsed = None
        ok = res["returncode"] == 0
        if ok:
            try:
                parsed = parse_json_output(res["stdout"])
                ok = bool(parsed.get("ok"))
            except Exception as exc:
                ok = False
                parsed = {"parse_error": str(exc)}
        add_step(report, "install_real_json", ok, parsed=parsed, stderr=res["stderr"][-1000:])


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
                            if is_noisy_user_prompt_context(ctx):
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
            ("state_write", [str(BIN), "state", "write", "--key", "stability-audit", "--value", "current-v1", "--json"], env_state),
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
                    if name == "state_read":
                        record = parsed.get("record", {})
                        expected = {
                            "schema_version": 1,
                            "key": "stability-audit",
                            "content": "current-v1",
                        }
                        if any(record.get(field) != value for field, value in expected.items()):
                            step_ok = False
                            parsed["validation_error"] = "unexpected_current_state_record"
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
        mcp = run_mcp_jsonrpc_process([str(BIN), "mcp"], calls, env=mcp_smoke_env(env), timeout=15)
        if not mcp["ok"]:
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
        add_step(report, "daemon_mcp_stress", ok and not new_pids, cycles=cycle_details, mcp_ids=mcp["response_ids"], rpc_errors=mcp["rpc_errors"], mcp_missing_ids=mcp["missing_ids"], mcp_duplicate_ids=mcp["duplicate_ids"], mcp_malformed_lines=mcp["malformed_lines"], mcp_timed_out=mcp["timed_out"], temp_mcp_pid=temp_pid, temp_mcp_leaked=temp_leaked, new_daemon_pids_after_stress=new_pids, mcp_stderr=mcp["stderr"][-1000:])


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
    with tempfile.TemporaryDirectory(prefix="agent-harness-stability-regression-") as td:
        isolated_root = Path(td)
        isolated_env = {
            "HARNESS_STATE_DIR": str(isolated_root / "state"),
            "HARNESS_ROOT": str(ROOT),
            "HARNESS_DAEMON_DIR": str(isolated_root / "daemon"),
            "HARNESS_WORKER_DIR": str(isolated_root / "worker"),
        }
        for key, path in isolated_env.items():
            if key == "HARNESS_ROOT":
                continue
            Path(path).mkdir(mode=0o700)
        for cmd in commands:
            env = isolated_env if cmd[:2] == ["go", "test"] else None
            res = run(cmd, env=env, timeout=REGRESSION_TIMEOUT_SECONDS)
            step_ok = res["returncode"] == 0
            ok = ok and step_ok
            details.append({"cmd": cmd, "ok": step_ok, "stderr_tail": res["stderr"][-1000:], "stdout_tail": res["stdout"][-1000:], "duration_ms": res["duration_ms"]})
    if self_verify:
        with tempfile.TemporaryDirectory() as td:
            res = run(
                [str(BIN), "self-verify", "--full", "--iterations=10", "--seed=100", "--target-score=95", "--llm-eval=false", "--progress=jsonl", "--json"],
                env={"HARNESS_STATE_DIR": td},
                timeout=FULL_SELF_VERIFY_TIMEOUT_SECONDS,
            )
            parsed = None
            parse_error = None
            try:
                parsed = parse_json_output(res["stdout"])
            except Exception as exc:
                parse_error = str(exc)
            step_ok = (
                res["returncode"] == 0
                and isinstance(parsed, dict)
                and bool(parsed.get("ok"))
                and bool(parsed.get("termination_eligible", True))
            )
            ok = ok and step_ok
            parsed_dict = parsed if isinstance(parsed, dict) else {}
            details.append(
                {
                    "cmd": "self-verify",
                    "ok": step_ok,
                    "returncode": res["returncode"],
                    "timed_out": bool(res.get("timeout")),
                    "parsed_ok": parsed_dict.get("ok"),
                    "termination_eligible": parsed_dict.get("termination_eligible"),
                    "summary": parsed_dict.get("summary") if parsed_dict else parsed,
                    "parse_error": parse_error,
                    "stderr_tail": res["stderr"][-1000:],
                    "stdout_tail": res["stdout"][-2000:],
                    "duration_ms": res["duration_ms"],
                }
            )
    add_step(report, "regression", ok, details=details)


def main() -> int:
    parser = argparse.ArgumentParser(description="Run agent-harness E2E stability audit")
    parser.add_argument("--full-install", action="store_true", help="run the canonical install command after dry-run checks")
    parser.add_argument("--cleanup-stale", action="store_true", help="terminate confirmed legacy/temp harness-owned stale processes")
    parser.add_argument(
        "--preserve-terminal",
        action="append",
        type=_terminal_handle_arg,
        default=None,
        help="preserve one exact current Orca terminal for the operational doctor gate",
    )
    parser.add_argument("--skip-race", action="store_true", help="skip go test -race")
    parser.add_argument("--skip-self-verify", action="store_true", help="skip 10-iteration self-verify")
    parser.add_argument("--daemon-cycles", type=int, default=8)
    parser.add_argument("--rss-rounds", type=int, default=3)
    parser.add_argument("--rss-calls", type=int, default=200)
    parser.add_argument("--json", action="store_true", help="print JSON report")
    args = parser.parse_args()
    if args.preserve_terminal is not None and len(args.preserve_terminal) != 1:
        parser.error("--preserve-terminal may be specified exactly once")
    preserve_terminal = args.preserve_terminal[0] if args.preserve_terminal is not None else None

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
    operational_doctor(report, preserve_terminal)
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
