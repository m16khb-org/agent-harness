import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("e2e_stability_audit.py")


def load_audit_module():
    spec = importlib.util.spec_from_file_location("e2e_stability_audit", SCRIPT)
    if spec is None or spec.loader is None:
        raise AssertionError(f"cannot load {SCRIPT}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class StabilityAuditScriptTest(unittest.TestCase):
    def test_operational_doctor_invokes_top_level_gate_with_exact_terminal(self) -> None:
        audit = load_audit_module()
        calls: list[list[str]] = []

        def fake_run(cmd, **_kwargs):
            calls.append([str(arg) for arg in cmd])
            return {
                "returncode": 0,
                "stdout": '{"ok": true, "healthy": true, "issues": []}',
                "stderr": "",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.dict(audit.os.environ, {"ORCA_TERMINAL_HANDLE": "term_current"}, clear=True):
            with mock.patch.object(audit, "run", side_effect=fake_run):
                audit.operational_doctor(report)

        self.assertEqual(
            calls,
            [[str(audit.BIN), "doctor", "--repo", str(audit.ROOT), "--sealed", "--json", "--preserve-terminal", "term_current"]],
        )
        self.assertEqual(report["steps"][0]["name"], "operational_doctor")
        self.assertTrue(report["steps"][0]["ok"])
        self.assertEqual(report["failures"], [])

    def test_operational_doctor_explicit_terminal_overrides_stale_environment(self) -> None:
        audit = load_audit_module()
        calls: list[list[str]] = []

        def fake_run(cmd, **_kwargs):
            calls.append([str(arg) for arg in cmd])
            return {
                "returncode": 0,
                "stdout": '{"ok": true, "healthy": true, "issues": []}',
                "stderr": "",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.dict(audit.os.environ, {"ORCA_TERMINAL_HANDLE": "term_stale"}, clear=True):
            with mock.patch.object(audit, "run", side_effect=fake_run):
                audit.operational_doctor(report, "term_sealed")

        self.assertEqual(
            calls,
            [[str(audit.BIN), "doctor", "--repo", str(audit.ROOT), "--sealed", "--json", "--preserve-terminal", "term_sealed"]],
        )

    def test_operational_doctor_omits_blank_terminal_and_requires_ok_and_healthy(self) -> None:
        audit = load_audit_module()
        cases = [
            ("nonzero", 1, '{"ok": true, "healthy": true}', False),
            ("not_ok", 0, '{"ok": false, "healthy": true}', False),
            ("unhealthy", 0, '{"ok": true, "healthy": false}', False),
            ("malformed", 0, "not-json", False),
            ("healthy", 0, '{"ok": true, "healthy": true}', True),
        ]

        for name, returncode, stdout, want_ok in cases:
            with self.subTest(name=name):
                calls: list[list[str]] = []

                def fake_run(cmd, **_kwargs):
                    calls.append([str(arg) for arg in cmd])
                    return {
                        "returncode": returncode,
                        "stdout": stdout,
                        "stderr": "",
                        "duration_ms": 1,
                        "timeout": False,
                    }

                report = {"steps": [], "failures": []}
                with mock.patch.dict(audit.os.environ, {"ORCA_TERMINAL_HANDLE": "   "}, clear=True):
                    with mock.patch.object(audit, "run", side_effect=fake_run):
                        audit.operational_doctor(report)

                self.assertEqual(calls, [[str(audit.BIN), "doctor", "--repo", str(audit.ROOT), "--sealed", "--json"]])
                self.assertEqual(report["steps"][0]["ok"], want_ok)
                self.assertEqual(bool(report["failures"]), not want_ok)

    def test_operational_doctor_failure_retains_only_bounded_issue_summaries(self) -> None:
        audit = load_audit_module()
        raw_issues = [
            {
                "code": "operational_dead_owner_" + "c" * 200,
                "summary": "dead cycle " + "s" * 500,
                "path": "/private/secret/path",
                "raw_message": "must-not-survive",
            }
            for _ in range(30)
        ]

        def fake_run(_cmd, **_kwargs):
            return {
                "returncode": 0,
                "stdout": json.dumps({"ok": True, "healthy": False, "issues": raw_issues}),
                "stderr": "raw stderr must not survive",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.dict(audit.os.environ, {}, clear=True):
            with mock.patch.object(audit, "run", side_effect=fake_run):
                audit.operational_doctor(report)

        failure = report["failures"][0]
        self.assertLessEqual(len(failure["issues"]), audit.DOCTOR_ISSUE_LIMIT)
        for issue in failure["issues"]:
            self.assertEqual(set(issue), {"code", "summary"})
            self.assertLessEqual(len(issue["code"]), audit.DOCTOR_CODE_LIMIT)
            self.assertLessEqual(len(issue["summary"]), audit.DOCTOR_SUMMARY_LIMIT)
        self.assertNotIn("must-not-survive", repr(failure))
        self.assertNotIn("raw stderr", repr(failure))
        self.assertNotIn("/private/secret/path", repr(failure))

    def test_main_runs_operational_doctor_immediately_after_build(self) -> None:
        audit = load_audit_module()
        order: list[str] = []
        steps = [
            "build",
            "operational_doctor",
            "install_checks",
            "host_mcp_checks",
            "hook_smoke",
            "temp_state_worker_policy",
            "daemon_and_mcp_stress",
            "cleanup_stale",
            "rss_sample",
            "regression",
        ]

        def record(name):
            def call(*_args, **_kwargs):
                order.append(name)

            return call

        status = {"returncode": 0, "stdout": "", "stderr": "", "duration_ms": 1, "timeout": False}
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(audit, "run", return_value=status))
            stack.enter_context(mock.patch.object(audit.sys, "argv", ["stability-audit", "--skip-race", "--skip-self-verify"]))
            stack.enter_context(mock.patch("builtins.print"))
            for name in steps:
                stack.enter_context(mock.patch.object(audit, name, side_effect=record(name)))
            self.assertEqual(audit.main(), 0)

        self.assertEqual(order[:3], ["build", "operational_doctor", "install_checks"])

    def test_main_forwards_explicit_preserve_terminal(self) -> None:
        audit = load_audit_module()
        status = {"returncode": 0, "stdout": "", "stderr": "", "duration_ms": 1, "timeout": False}
        action_names = [
            "build",
            "install_checks",
            "host_mcp_checks",
            "hook_smoke",
            "temp_state_worker_policy",
            "daemon_and_mcp_stress",
            "cleanup_stale",
            "rss_sample",
            "regression",
        ]
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(audit, "run", return_value=status))
            stack.enter_context(
                mock.patch.object(
                    audit.sys,
                    "argv",
                    ["stability-audit", "--preserve-terminal", "term_sealed", "--skip-race", "--skip-self-verify"],
                )
            )
            stack.enter_context(mock.patch("builtins.print"))
            for name in action_names:
                stack.enter_context(mock.patch.object(audit, name))
            operational_doctor = stack.enter_context(mock.patch.object(audit, "operational_doctor"))
            self.assertEqual(audit.main(), 0)

        operational_doctor.assert_called_once()
        self.assertEqual(operational_doctor.call_args.args[1], "term_sealed")

    def test_invalid_explicit_preserve_terminal_stops_before_any_action(self) -> None:
        invalid_argv = [
            ["--preserve-terminal", ""],
            ["--preserve-terminal", " "],
            ["--preserve-terminal", "terminal_wrong"],
            ["--preserve-terminal", "term_bad!"],
            ["--preserve-terminal", "term_" + "a" * 252],
            ["--preserve-terminal", "term_one", "--preserve-terminal", "term_two"],
        ]
        action_names = [
            "build",
            "operational_doctor",
            "install_checks",
            "host_mcp_checks",
            "hook_smoke",
            "temp_state_worker_policy",
            "daemon_and_mcp_stress",
            "cleanup_stale",
            "rss_sample",
            "regression",
        ]

        for argv in invalid_argv:
            with self.subTest(argv=argv):
                audit = load_audit_module()
                with contextlib.ExitStack() as stack:
                    run = stack.enter_context(mock.patch.object(audit, "run"))
                    actions = [stack.enter_context(mock.patch.object(audit, name)) for name in action_names]
                    stack.enter_context(mock.patch.object(audit.sys, "argv", ["stability-audit", *argv]))
                    stack.enter_context(mock.patch("builtins.print"))
                    stack.enter_context(contextlib.redirect_stderr(io.StringIO()))
                    with self.assertRaises(SystemExit):
                        audit.main()
                run.assert_not_called()
                for action in actions:
                    action.assert_not_called()

    def test_install_checks_use_canonical_install_command(self) -> None:
        audit = load_audit_module()
        calls: list[list[str]] = []

        def fake_run(cmd, **_kwargs):
            calls.append([str(arg) for arg in cmd])
            return {
                "returncode": 0,
                "stdout": '{"ok": true}',
                "stderr": "",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.object(audit, "run", side_effect=fake_run):
            audit.install_checks(report, full_install=True)

        self.assertEqual(
            [step["name"] for step in report["steps"]],
            ["bootstrap_dry_json", "install_dry_json", "install_real_json"],
        )
        self.assertEqual(
            [call[1:] for call in calls],
            [
                ["bootstrap", "--dry-run", "--json"],
                ["install", "--dry-run", "--json"],
                ["install", "--json"],
            ],
        )
        self.assertNotIn("install-native", [arg for call in calls for arg in call])

    def test_temp_state_worker_policy_uses_current_state_commands(self) -> None:
        audit = load_audit_module()
        calls: list[list[str]] = []

        def fake_run(cmd, **_kwargs):
            call = [str(arg) for arg in cmd]
            calls.append(call)
            payload = {"ok": True}
            if call[1:3] == ["state", "read"]:
                payload["record"] = {
                    "schema_version": 1,
                    "key": "stability-audit",
                    "content": "current-v1",
                    "bytes": 10,
                    "updated_at": "2026-08-02T00:00:00Z",
                }
            return {
                "returncode": 0,
                "stdout": json.dumps(payload),
                "stderr": "",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.object(audit, "run", side_effect=fake_run):
            audit.temp_state_worker_policy(report)

        retired = [str(audit.BIN), "state", "migrate", "--json"]
        self.assertNotIn(retired, calls)
        self.assertIn(
            [str(audit.BIN), "state", "write", "--key", "stability-audit", "--value", "current-v1", "--json"],
            calls,
        )
        self.assertIn([str(audit.BIN), "state", "read", "--key", "stability-audit", "--json"], calls)
        self.assertIn([str(audit.BIN), "state", "doctor", "--json"], calls)
        self.assertTrue(report["steps"][0]["ok"])

    def test_temp_state_worker_policy_rejects_invalid_readback(self) -> None:
        audit = load_audit_module()
        cases = [
            ("wrong_schema", {"schema_version": 0}),
            ("wrong_key", {"key": "other"}),
            ("wrong_content", {"content": "other"}),
        ]

        for name, override in cases:
            with self.subTest(name=name):
                def fake_run(cmd, **_kwargs):
                    call = [str(arg) for arg in cmd]
                    payload = {"ok": True}
                    if call[1:3] == ["state", "read"]:
                        record = {
                            "schema_version": 1,
                            "key": "stability-audit",
                            "content": "current-v1",
                        }
                        record.update(override)
                        payload["record"] = record
                    return {
                        "returncode": 0,
                        "stdout": json.dumps(payload),
                        "stderr": "",
                        "duration_ms": 1,
                        "timeout": False,
                    }

                report = {"steps": [], "failures": []}
                with mock.patch.object(audit, "run", side_effect=fake_run):
                    audit.temp_state_worker_policy(report)

                self.assertFalse(report["steps"][0]["ok"])
                state_read = next(item for item in report["steps"][0]["details"] if item["name"] == "state_read")
                self.assertFalse(state_read["ok"])

    def test_regression_timeouts_cover_observed_full_gate_durations(self) -> None:
        audit = load_audit_module()
        calls: list[tuple[list[str], float]] = []

        def fake_run(cmd, *, timeout=60, **_kwargs):
            call = [str(arg) for arg in cmd]
            calls.append((call, timeout))
            stdout = '{"ok": true, "termination_eligible": true, "summary": {}}' if "self-verify" in call else ""
            return {
                "returncode": 0,
                "stdout": stdout,
                "stderr": "",
                "duration_ms": 1,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.object(audit, "run", side_effect=fake_run):
            audit.regression(report, race=True, self_verify=True)

        regression_timeout = getattr(audit, "REGRESSION_TIMEOUT_SECONDS", 0)
        self.assertGreaterEqual(regression_timeout, 300)
        go_test_timeouts = [timeout for cmd, timeout in calls if cmd[:2] == ["go", "test"]]
        self.assertEqual(go_test_timeouts, [regression_timeout] * 2)
        self_verify_timeouts = [timeout for cmd, timeout in calls if "self-verify" in cmd]
        self.assertEqual(self_verify_timeouts, [audit.FULL_SELF_VERIFY_TIMEOUT_SECONDS])
        self_verify_commands = [cmd for cmd, _timeout in calls if "self-verify" in cmd]
        self.assertIn("--progress=jsonl", self_verify_commands[0])
        self.assertIn("--llm-eval=false", self_verify_commands[0])
        self.assertGreaterEqual(audit.FULL_SELF_VERIFY_TIMEOUT_SECONDS, 5400)

    def test_regression_isolates_harness_state_while_operational_doctor_stays_live(self) -> None:
        audit = load_audit_module()
        calls: list[tuple[list[str], dict[str, str] | None]] = []

        with tempfile.TemporaryDirectory() as live_td:
            live_root = Path(live_td)
            live_db = live_root / "issueops" / "harness.db"
            live_db.parent.mkdir(parents=True)
            live_db.write_text("live-session-projection")
            live_env = {
                "HARNESS_STATE_DIR": str(live_root),
                "HARNESS_ROOT": str(live_root / "root"),
                "HARNESS_DAEMON_DIR": str(live_root / "daemon"),
                "HARNESS_WORKER_DIR": str(live_root / "worker"),
            }

            def fake_run(cmd, *, env=None, **_kwargs):
                call = [str(arg) for arg in cmd]
                captured_env = dict(env) if env is not None else None
                calls.append((call, captured_env))
                if call[:2] == ["go", "test"]:
                    isolated_db = Path(captured_env["HARNESS_STATE_DIR"]) / "issueops" / "harness.db"
                    isolated_db.parent.mkdir(parents=True, exist_ok=True)
                    isolated_db.write_text("isolated-test-session")
                stdout = '{"ok": true, "healthy": true, "issues": []}' if "doctor" in call else ""
                return {
                    "returncode": 0,
                    "stdout": stdout,
                    "stderr": "",
                    "duration_ms": 1,
                    "timeout": False,
                }

            report = {"steps": [], "failures": []}
            with mock.patch.dict(audit.os.environ, live_env, clear=True):
                with mock.patch.object(audit, "run", side_effect=fake_run):
                    audit.operational_doctor(report)
                    audit.regression(report, race=True, self_verify=False)

            self.assertEqual(live_db.read_text(), "live-session-projection")

        doctor_calls = [(cmd, env) for cmd, env in calls if "doctor" in cmd]
        self.assertEqual(len(doctor_calls), 1)
        self.assertIsNone(doctor_calls[0][1])

        go_test_envs = [env for cmd, env in calls if cmd[:2] == ["go", "test"]]
        self.assertEqual(len(go_test_envs), 2)
        for isolated_env in go_test_envs:
            self.assertIsNotNone(isolated_env)
            self.assertEqual(set(isolated_env), set(live_env))
            self.assertEqual(isolated_env["HARNESS_ROOT"], str(audit.ROOT))
            for key in ("HARNESS_STATE_DIR", "HARNESS_DAEMON_DIR", "HARNESS_WORKER_DIR"):
                self.assertNotEqual(isolated_env[key], live_env[key])
        self.assertEqual(go_test_envs[0], go_test_envs[1])

    def test_regression_preserves_self_verify_failure_diagnostics(self) -> None:
        audit = load_audit_module()

        def fake_run(cmd, **_kwargs):
            call = [str(arg) for arg in cmd]
            stdout = '{"ok": false, "termination_eligible": false, "summary": {"failed_step": "go test"}}' if "self-verify" in call else ""
            return {
                "returncode": 1 if "self-verify" in call else 0,
                "stdout": stdout,
                "stderr": "self-verify diagnostic",
                "duration_ms": 123,
                "timeout": False,
            }

        report = {"steps": [], "failures": []}
        with mock.patch.object(audit, "run", side_effect=fake_run):
            audit.regression(report, race=False, self_verify=True)

        detail = report["steps"][0]["details"][-1]
        self.assertFalse(detail["ok"])
        self.assertEqual(detail["returncode"], 1)
        self.assertFalse(detail["timed_out"])
        self.assertFalse(detail["parsed_ok"])
        self.assertFalse(detail["termination_eligible"])
        self.assertEqual(detail["summary"], {"failed_step": "go test"})
        self.assertEqual(detail["stderr_tail"], "self-verify diagnostic")
        self.assertIn('"ok": false', detail["stdout_tail"])


def _child_program():
    return r'''
import json
import sys
import time

mode = sys.argv[1]
if mode == "premature_eof":
    raise SystemExit(0)
if mode == "timeout":
    time.sleep(10)
    raise SystemExit(0)

pending_ids = []
for line in sys.stdin:
    request = json.loads(line)
    if "id" not in request:
        continue
    if mode == "reordered":
        pending_ids.append(request["id"])
        if len(pending_ids) == 2:
            for response_id in reversed(pending_ids):
                print(json.dumps({"jsonrpc": "2.0", "id": response_id, "result": {}}), flush=True)
    elif mode == "malformed":
        print("not-json", flush=True)
    elif mode == "duplicate":
        response = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": {}})
        print(response, flush=True)
        print(response, flush=True)
    elif mode == "missing":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"] + 100, "result": {}}), flush=True)
    elif mode == "error":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32603, "message": "boom"}}), flush=True)
    elif mode == "bare_id":
        print(json.dumps({"id": request["id"]}), flush=True)
    elif mode == "wrong_version":
        print(json.dumps({"jsonrpc": "1.0", "id": request["id"], "result": {}}), flush=True)
    elif mode == "both_result_and_error":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": {}, "error": {"code": -32603, "message": "boom"}}), flush=True)
    elif mode == "neither_result_nor_error":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"]}), flush=True)
    elif mode == "non_object":
        print(json.dumps([]), flush=True)
    elif mode == "error_not_object":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "error": []}), flush=True)
    elif mode == "error_bool_code":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "error": {"code": True, "message": "boom"}}), flush=True)
    elif mode == "error_nonstring_message":
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32603, "message": 1}}), flush=True)
    else:
        print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": {}}), flush=True)
'''


class MCPJSONRPCProcessTest(unittest.TestCase):
    def test_waits_for_responses_before_closing_stdin_and_rejects_bad_transcripts(self) -> None:
        audit = load_audit_module()
        calls = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
            {"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
        ]
        processes = []
        popen = subprocess.Popen

        def track_process(*args, **kwargs):
            process = popen(*args, **kwargs)
            processes.append(process)
            return process

        with mock.patch.object(audit.subprocess, "Popen", side_effect=track_process):
            for mode, want_ok, want_problem, want_ids in [
                ("success", True, "", [1, 2]),
                ("reordered", True, "", [2, 1]),
                ("premature_eof", False, "missing_ids", None),
                ("malformed", False, "malformed_lines", None),
                ("duplicate", False, "duplicate_ids", None),
                ("missing", False, "missing_ids", None),
                ("error", False, "rpc_errors", None),
                ("timeout", False, "timed_out", None),
            ]:
                with self.subTest(mode=mode):
                    result = audit.run_mcp_jsonrpc_process(
                        [sys.executable, "-u", "-c", _child_program(), mode],
                        calls,
                        timeout=1,
                    )
                    self.assertEqual(want_ok, result["ok"], result)
                    if want_problem:
                        self.assertTrue(result[want_problem], result)
                    else:
                        self.assertEqual(want_ids, result["response_ids"], result)
                        self.assertFalse(result["stdin_closed_before_responses"], result)

        self.assertTrue(all(process.stdout is not None and process.stdout.closed for process in processes))
        self.assertTrue(all(process.stderr is not None and process.stderr.closed for process in processes))

    def test_rejects_malformed_jsonrpc_response_envelopes_without_crashing(self) -> None:
        audit = load_audit_module()
        calls = [{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}]

        for mode in [
            "bare_id",
            "wrong_version",
            "both_result_and_error",
            "neither_result_nor_error",
            "non_object",
            "error_not_object",
            "error_bool_code",
            "error_nonstring_message",
        ]:
            with self.subTest(mode=mode):
                result = audit.run_mcp_jsonrpc_process(
                    [sys.executable, "-u", "-c", _child_program(), mode],
                    calls,
                    timeout=1,
                )

                self.assertFalse(result["ok"], result)
                self.assertTrue(result["malformed_lines"], result)
                self.assertEqual(result["response_ids"], [], result)


if __name__ == "__main__":
    unittest.main()
