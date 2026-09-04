#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("e2e_stability_audit.py")
SPEC = importlib.util.spec_from_file_location("e2e_stability_audit", SCRIPT)
assert SPEC is not None
audit = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(audit)


class StabilityAuditScriptTest(unittest.TestCase):
    def test_run_returns_structured_timeout_result(self) -> None:
        result = audit.run(
            [sys.executable, "-c", "import time; time.sleep(2)"],
            timeout=0.1,
        )

        self.assertEqual(result["returncode"], -1)
        self.assertTrue(result["timeout"])
        self.assertIn("timed out", result["stderr"])
        self.assertGreaterEqual(result["duration_ms"], 0)

    def test_self_verify_timeout_has_bounded_single_pass_budget(self) -> None:
        self.assertGreaterEqual(audit.SELF_VERIFY_TIMEOUT_SECONDS, 600)
        self.assertLessEqual(audit.SELF_VERIFY_TIMEOUT_SECONDS, 1800)

    def test_self_verify_command_matches_current_cli_contract(self) -> None:
        self.assertEqual(
            audit.self_verify_command(),
            [
                str(audit.BIN),
                "self-verify",
                "--seed=100",
                "--target-score=95",
                "--llm-eval=false",
                "--progress=jsonl",
                "--json",
            ],
        )

    def test_self_verify_result_requires_strict_complete_payload(self) -> None:
        result = {"returncode": 0}
        valid = {
            "ok": True,
            "termination_eligible": True,
            "summary": {"termination_eligible": True},
        }
        self.assertTrue(audit.self_verify_result_ok(result, valid))
        for invalid in [
            {"ok": "false", "termination_eligible": True, "summary": {"termination_eligible": True}},
            {"ok": True, "summary": {"termination_eligible": True}},
            {"ok": True, "termination_eligible": True},
            {"ok": True, "termination_eligible": True, "summary": {"termination_eligible": False}},
        ]:
            with self.subTest(invalid=invalid):
                self.assertFalse(audit.self_verify_result_ok(result, invalid))
        self.assertFalse(audit.self_verify_result_ok({"returncode": 1}, valid))

    def test_user_prompt_compact_turn_hint_is_not_noisy(self) -> None:
        ctx = "\n".join(
            [
                "[issueops]",
                "- docs: use project docs only when repo-specific context matters",
                "- profile: github/managed@github.com, Go, backend+cli",
                "- rule: verify with repo/tool evidence before changing files",
            ]
        )
        self.assertFalse(audit.is_noisy_user_prompt_context(ctx))

    def test_user_prompt_catalog_injection_is_noisy(self) -> None:
        self.assertTrue(audit.is_noisy_user_prompt_context("Required project docs:\n- ADR.md"))
        self.assertTrue(audit.is_noisy_user_prompt_context("필수 프롬프트 주입중: .issueops/TESTING.md"))

    def test_mcp_smoke_env_forces_direct_stream_transport(self) -> None:
        env = audit.mcp_smoke_env({"ISSUEOPS_DAEMON_DIR": "/tmp/daemon"})
        self.assertEqual(env["ISSUEOPS_DAEMON_DIR"], "/tmp/daemon")
        self.assertEqual(env["ISSUEOPS_MCP_DIRECT"], "1")


def _row(command: str, *, state: str = "S", pid: int = 100, ppid: int = 1, rss_kb: int = 1024) -> dict:
    return {"pid": pid, "ppid": ppid, "state": state, "rss_kb": rss_kb, "command": command}


class ClassifyProcessesTest(unittest.TestCase):
    """Pins the process-hygiene classification logic (quality program Q4 종결조건 ②;
    classify_processes was previously untested). This is a layer-C contract: it asserts
    that the daemon/legacy/temp-watcher/zombie buckets match the documented signatures on
    synthetic ps rows — it does NOT prove the live ps enumeration or cleanup actions are
    correct, only that the classifier routes a known command string to the expected bucket."""

    def test_current_daemon_matches_only_current_bucket(self) -> None:
        result = audit.classify_processes([_row("/usr/local/bin/issueops daemon --internal --socket /tmp/x.sock")])
        self.assertEqual(len(result["current_daemons"]), 1)
        self.assertEqual(result["legacy_harness"], [])
        self.assertEqual(result["temp_watchers"], [])
        self.assertEqual(result["zombies"], [])

    def test_legacy_bin_harness_daemon_and_mcp_match_legacy_bucket(self) -> None:
        rows = [
            _row("/old/path/bin/issueops daemon --internal"),
            _row("/old/path/bin/issueops mcp"),
        ]
        result = audit.classify_processes(rows)
        self.assertEqual(len(result["legacy_harness"]), 2)
        self.assertEqual(result["current_daemons"], [])

    def test_temp_codegraph_watcher_matches_temp_bucket(self) -> None:
        cmd = "node /repo/scripts/codegraph-watcher.mjs /var/folders/ab/T/tmp.XyZ123/graph"
        result = audit.classify_processes([_row(cmd)])
        self.assertEqual(len(result["temp_watchers"]), 1)

    def test_zombie_requires_both_z_state_and_harness_command(self) -> None:
        rows = [
            _row("/usr/local/bin/issueops daemon --internal", state="Z"),  # harness + Z -> zombie
            _row("/usr/bin/python3 some_unrelated_script.py", state="Z"),       # Z but not harness -> not zombie
            _row("codegraph index", state="Z+"),                                # codegraph + Z -> zombie
        ]
        result = audit.classify_processes(rows)
        self.assertEqual(len(result["zombies"]), 2)
        zombie_cmds = [r["command"] for r in result["zombies"]]
        self.assertNotIn("/usr/bin/python3 some_unrelated_script.py", zombie_cmds)

    def test_unrelated_process_matches_no_bucket(self) -> None:
        result = audit.classify_processes([_row("vim notes.txt"), _row("/bin/zsh -l")])
        self.assertEqual(result["current_daemons"], [])
        self.assertEqual(result["legacy_harness"], [])
        self.assertEqual(result["temp_watchers"], [])
        self.assertEqual(result["zombies"], [])

    def test_empty_rows_yield_empty_buckets(self) -> None:
        result = audit.classify_processes([])
        self.assertEqual(result, {"current_daemons": [], "legacy_harness": [], "temp_watchers": [], "zombies": []})


if __name__ == "__main__":
    unittest.main()
