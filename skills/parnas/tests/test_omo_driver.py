from __future__ import annotations

import importlib.util
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

SCRIPTS = Path(__file__).parents[1] / "scripts"
spec = importlib.util.spec_from_file_location("omo_driver", SCRIPTS / "omo_driver.py")
omo_driver = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(omo_driver)

ARGS = {
    "hunkRanges": {"src/a.go": [[10, 20]], "src/other.go": [[10, 20]]},
    "refutedHistory": [{"path": "src/a.go", "title": "order 를 화이트리스트 없이 넘겨 ORDER BY 주입", "what": "", "category": "bug"}],
    "maxCandidates": 24,
    "lenses": ["logic", "boundary", "tests", "rules", "scope", "security"],
}


class ProfileTest(unittest.TestCase):
    def test_omo_flash_profile_raises_budgets_over_args(self) -> None:
        b = omo_driver.resolve_budget(ARGS, "omo-flash")
        self.assertEqual(b["finder_turns"], 24)
        self.assertEqual(b["skeptic_turns"], 18)
        self.assertEqual(b["max_candidates"], 40)          # floor raises 24 → 40
        self.assertGreater(b["per_lens_cap"], 5)           # recomputed from the raised cap
        self.assertEqual(b["workers"]["finder"], 10)         # high concurrency kept; backoff absorbs bursts
        self.assertEqual(b["thinking"], {"finder": "high", "tracer": "high", "reproducer": "high"})

    def test_standard_profile_keeps_workflow_budgets(self) -> None:
        b = omo_driver.resolve_budget(ARGS, "standard")
        self.assertEqual(b["finder_turns"], 10)
        self.assertEqual(b["skeptic_turns"], 8)
        self.assertEqual(b["max_candidates"], 24)          # no floor applied
        self.assertEqual(b["thinking"]["tracer"], "medium")

    def test_omo_runner_is_pinned_to_zai_flash(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        cmd = runner._cmd("sid", "high")
        self.assertEqual(cmd[cmd.index("--provider") + 1], "zai")
        self.assertEqual(cmd[cmd.index("--model") + 1], "zai/glm-5.3-flash")
        self.assertIn("--no-model-fallback", cmd)
        self.assertEqual(cmd[cmd.index("--permission-preset") + 1], "read-only")
        with self.assertRaises(ValueError):
            omo_driver.OmoRunner("opencode-go", "kimi-k2.6")

    def test_role_permission_preset_is_explicit(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        self.assertEqual(runner._cmd("sid", "high", "workspace")[
            runner._cmd("sid", "high", "workspace").index("--permission-preset") + 1
        ], "workspace")
        with self.assertRaises(ValueError):
            runner._cmd("sid", "high", "full-access")

    def test_omo_cli_contract_requires_no_model_fallback(self) -> None:
        help_text = "--provider --model --permission-preset --session-id"
        with patch.object(omo_driver.subprocess, "run",
                          return_value=subprocess.CompletedProcess([], 0, help_text, "")):
            with self.assertRaisesRegex(RuntimeError, "--no-model-fallback"):
                omo_driver.validate_omo_cli("omo")

    def test_session_models_reads_model_change_and_assistant_records(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            Path(tmp, "prefix-sid.jsonl").write_text(
                '{"type":"model_change","provider":"zai","modelId":"glm-5.3-flash"}\n'
                '{"type":"message","message":{"role":"assistant","provider":"zai","model":"glm-5.3-flash"}}\n',
                encoding="utf-8",
            )
            with patch.object(omo_driver, "OMO_SESSIONS", Path(tmp)):
                self.assertEqual(omo_driver.session_models("sid"), {"zai/glm-5.3-flash"})


class PromptBoundaryTest(unittest.TestCase):
    def test_prompts_reserve_final_json_turn_and_declare_tools(self) -> None:
        args = {
            "checkout": "/repo", "outDir": "/review", "finder_turns": 24,
            "skeptic_turns": 18, "per_lens_cap": 3, "codegraph": False,
            "lensText": {"logic": "logic"},
        }
        finder = omo_driver.finder_prompt(
            args, {"id": "logic@all", "pack": "pack.md", "lenses": ["logic"]}
        )
        tracer = omo_driver.skeptic_prompt(
            args, "tracer",
            {"path": "a.go", "new_line": 1, "severity": "high", "category": "bug",
             "title": "t", "what": "w", "why": "why", "lens": "logic"},
            None,
        )
        reproducer = omo_driver.skeptic_prompt(
            args, "reproducer",
            {"path": "a.go", "new_line": 1, "severity": "high", "category": "bug",
             "title": "t", "what": "w", "why": "why", "lens": "logic"},
            None,
        )

        for prompt in (finder, tracer, reproducer):
            self.assertIn("Reserve the final assistant message", prompt)
            self.assertIn("no tool call", prompt)
        self.assertIn("reviewed_files is a coverage receipt", finder)
        self.assertIn("Allowed tools: read, grep, find, ls", tracer)
        self.assertIn("Forbidden tools: bash, eval, webfetch", tracer)
        self.assertIn("Allowed tools: read, grep, find, ls, bash, edit, write", reproducer)
        self.assertIn("Never use /tmp, bash_output, background commands", reproducer)

    def test_review_inputs_are_mirrored_under_checkout(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            checkout = root / "worktree"
            source = root / "review"
            checkout.mkdir()
            (source / "pack").mkdir(parents=True)
            (source / "hunks").mkdir()
            (source / "pack" / "behavior_all.md").write_text("pack", encoding="utf-8")
            (source / "hunks" / "a.patch").write_text("patch", encoding="utf-8")
            (source / "defs.md").write_text("defs", encoding="utf-8")
            (source / "gate.md").write_text("gate", encoding="utf-8")
            args = {"checkout": str(checkout), "outDir": str(source)}

            input_dir = omo_driver.prepare_omo_inputs(args)
            try:
                self.assertTrue(input_dir.is_relative_to(checkout.resolve()))
                self.assertEqual((input_dir / "pack" / "behavior_all.md").read_text(), "pack")
                self.assertEqual((input_dir / "hunks" / "a.patch").read_text(), "patch")
                self.assertEqual((input_dir / "pack" / "behavior_all.md").stat().st_mode & 0o777, 0o444)
                args.update({"codegraph": False, "lensText": {"logic": "logic"},
                             "finder_turns": 24, "per_lens_cap": 6})
                prompt = omo_driver.finder_prompt(
                    args, {"id": "behavior@all", "pack": "pack/behavior_all.md", "lenses": ["logic"]}
                )
                self.assertIn(str(input_dir / "pack" / "behavior_all.md"), prompt)
                self.assertIn(str(input_dir / "gate.md"), prompt)
                self.assertNotIn(str(source / "pack"), prompt)
            finally:
                omo_driver.cleanup_omo_inputs(input_dir)
            self.assertFalse(input_dir.exists())

    def test_review_paths_are_canonicalized_for_omo(self) -> None:
        with tempfile.TemporaryDirectory(dir="/tmp") as tmp:
            root = Path(tmp)
            checkout = root / "worktree"
            source = root / "review"
            checkout.mkdir()
            (source / "pack").mkdir(parents=True)
            (source / "hunks").mkdir()
            (source / "pack" / "behavior_all.md").write_text("pack", encoding="utf-8")
            (source / "hunks" / "a.patch").write_text("patch", encoding="utf-8")
            args = {"checkout": str(checkout), "outDir": str(source)}

            input_dir = omo_driver.prepare_omo_inputs(args)
            try:
                self.assertEqual(args["checkout"], str(checkout.resolve()))
                self.assertEqual(args["outDir"], str(source.resolve()))
                self.assertEqual(str(input_dir), str(input_dir.resolve()))
            finally:
                omo_driver.cleanup_omo_inputs(input_dir)


class PrescreenTest(unittest.TestCase):
    def test_drops_off_hunk_line_without_newly_reachable(self) -> None:
        c = {"path": "src/a.go", "new_line": 999, "newly_reachable": False, "category": "bug"}
        why = omo_driver.prescreen(ARGS, c)
        self.assertIsNotNone(why)
        self.assertIn("outside every hunk", why)

    def test_keeps_hunk_line_and_newly_reachable(self) -> None:
        self.assertIsNone(omo_driver.prescreen(ARGS, {"path": "src/a.go", "new_line": 15, "category": "bug"}))
        self.assertIsNone(omo_driver.prescreen(ARGS, {"path": "src/a.go", "new_line": 999, "newly_reachable": True, "category": "bug"}))

    def test_newly_reachable_unchanged_file_survives(self) -> None:
        """Consumer regressions on untouched files must reach a skeptic when flagged (dogfood #347)."""
        c = {"path": "src/never-changed.vue", "new_line": 407, "newly_reachable": True, "category": "bug", "title": "t", "what": "w"}
        self.assertIsNone(omo_driver.prescreen(ARGS, c))
        silent = {**c, "newly_reachable": False}
        self.assertIsNotNone(omo_driver.prescreen(ARGS, silent))

    def test_refuted_history_suppresses_same_file_overlap_only(self) -> None:
        c = {"path": "src/a.go", "new_line": 15, "category": "bug",
             "title": "order 화이트리스트 없이 ORDER BY 주입", "what": ""}
        self.assertIsNotNone(omo_driver.prescreen(ARGS, c))
        other = {"path": "src/other.go", "new_line": 15, "category": "bug",
                 "title": "order 화이트리스트 없이 ORDER BY 주입", "what": ""}
        self.assertIsNone(omo_driver.prescreen(ARGS, other))

    def test_security_is_never_suppressed(self) -> None:
        c = {"path": "src/a.go", "new_line": 15, "category": "security",
             "title": "order 를 화이트리스트 없이 넘겨 ORDER BY 주입", "what": ""}
        self.assertIsNone(omo_driver.prescreen(ARGS, c))


class PureHelperTest(unittest.TestCase):
    def test_severity_moves_one_step_only_with_direction(self) -> None:
        self.assertEqual(omo_driver.severity_shift("medium", "raise"), "high")
        self.assertEqual(omo_driver.severity_shift("critical", "raise"), "critical")
        self.assertEqual(omo_driver.severity_shift("low", "lower"), "low")
        self.assertEqual(omo_driver.severity_shift("medium", "keep"), "medium")

    def test_extract_json_takes_outermost_object_from_prose(self) -> None:
        text = '설명입니다\n{"refuted": true, "confidence": 90}\n끝'
        self.assertEqual(omo_driver.extract_json(text), {"refuted": True, "confidence": 90})
        self.assertIsNone(omo_driver.extract_json("JSON 아님"))

    def test_similar_is_symmetric_bigram_coefficient(self) -> None:
        self.assertGreaterEqual(omo_driver.similar("ORDER BY 주입", "ORDER BY 인젝션"), 0.4)
        self.assertEqual(omo_driver.similar("abc", "xyz"), 0.0)

    def test_dedup_merges_same_spot_and_keeps_security_category(self) -> None:
        finders = [
            {"lenses": ["logic"], "candidates": [
                {"path": "a.go", "new_line": 10, "category": "bug", "title": "널 역참조", "confidence": 60, "lens": "logic"}]},
            {"lenses": ["security"], "candidates": [
                {"path": "a.go", "new_line": 11, "category": "security", "title": "널 역참조", "confidence": 80, "lens": "security"}]},
        ]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["category"], "security")   # security reading survives dedup
        self.assertEqual(merged[0]["confidence"], 80)
        self.assertEqual(sorted(merged[0]["lenses"]), ["logic", "security"])

    def test_coverage_report_requires_every_assignment_exactly_once(self) -> None:
        units = [
            {"id": "behavior@all", "files": ["src/a.go", "src/b.go"]},
            {"id": "intent@all", "files": ["src/a.go"]},
        ]
        finders = [
            {"unit": "behavior@all", "reviewed_files": ["src/a.go", "src/a.go", "src/extra.go"]},
            {"unit": "intent@all", "reviewed_files": ["src/a.go"]},
        ]

        report = omo_driver.coverage_report(units, finders)

        self.assertEqual(report["expected_assignments"], 3)
        self.assertEqual(report["covered_assignments"], 2)
        self.assertEqual(report["gaps"], [
            {"unit": "behavior@all", "missing_files": ["src/b.go"]},
        ])
        self.assertEqual(report["duplicates"], [
            {"unit": "behavior@all", "files": ["src/a.go"]},
        ])
        self.assertEqual(report["unexpected"], [
            {"unit": "behavior@all", "files": ["src/extra.go"]},
        ])
        self.assertFalse(report["complete"])

    def test_blind_hides_finder_evidence_from_tracer(self) -> None:
        c = {"path": "a.go", "new_line": 1, "severity": "high", "category": "bug", "title": "t",
             "what": "w", "why": "why", "evidence": ["a.go:1"], "upstream": "u", "downstream": "d", "lens": "logic"}
        b = omo_driver.blind(c)
        self.assertNotIn("evidence", b)
        self.assertNotIn("upstream", b)
        self.assertIn("what", b)
        self.assertIn("why", b)

    def test_agent_payload_rejects_malformed_numeric_fields(self) -> None:
        candidate = {
            "path": "a.go", "new_line": "1", "end_line": None, "severity": "high",
            "category": "bug", "title": "t", "what": "w", "why": "why",
            "evidence": [], "confidence": "75", "lens": "logic",
        }
        finder = {"lenses": ["logic"], "reviewed_files": ["a.go"], "inspected": [],
                  "candidates": [candidate], "verified_ok": []}
        self.assertIsNone(omo_driver.validate_agent_payload(finder, "finder"))

    def test_agent_payload_requires_reviewed_files_receipt(self) -> None:
        finder = {"lenses": ["logic"], "inspected": [], "candidates": [], "verified_ok": []}
        self.assertIsNone(omo_driver.validate_agent_payload(finder, "finder"))
        finder["reviewed_files"] = ["a.go"]
        self.assertEqual(omo_driver.validate_agent_payload(finder, "finder"), finder)

    def test_unverified_reason_is_rejected_even_with_higher_confidence(self) -> None:
        verdict = {
            "skeptic": "tracer", "refuted": False, "confidence": 50,
            "reason": "미확인: 검증 경로가 없음", "evidence": [], "severity_adjust": "keep",
        }
        self.assertTrue(omo_driver._is_unverified_verdict(verdict))

    def test_unverified_marker_inside_reason_overrides_inflated_confidence(self) -> None:
        verdict = {
            "skeptic": "tracer", "refuted": False, "confidence": 85,
            "reason": "검증 완료가 아니라 미확인: 결정적 근거를 읽지 못했다",
            "evidence": [], "severity_adjust": "keep",
        }
        self.assertTrue(omo_driver._is_unverified_verdict(verdict))

    def test_run_retries_without_model_fallback_and_keeps_permission(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        first = subprocess.CompletedProcess([], 1, "", "429")
        second = subprocess.CompletedProcess([], 0, '{"skeptic": "tracer", "refuted": false, "confidence": 75, "reason": "ok", "evidence": [], "severity_adjust": "keep"}', "")
        with patch.object(omo_driver, "run_omo_process", side_effect=[first, second]) as run, \
             patch.object(omo_driver.time, "sleep") as sleep:
            parsed, _, _, diagnostics = runner.run(
                "prompt", "/tmp", "high", permission_preset="read-only",
                payload_kind="verdict", allowed_tools=("read", "grep"),
            )
        self.assertEqual(parsed["confidence"], 75)
        self.assertEqual(run.call_count, 2)
        self.assertEqual(sleep.call_count, 1)
        self.assertEqual(diagnostics["failure_kind"], None)
        first_cmd = run.call_args_list[0].args[0]
        self.assertEqual(first_cmd[first_cmd.index("--model") + 1], "zai/glm-5.3-flash")
        self.assertEqual(first_cmd[first_cmd.index("--permission-preset") + 1], "read-only")
        self.assertEqual(first_cmd[first_cmd.index("--tools") + 1], "read,grep,submit_parnas_verdict")
        self.assertIn("--no-model-fallback", first_cmd)

    def test_format_retry_resends_original_prompt_and_disables_tools(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        original = 'ORIGINAL REQUEST\nCandidate: {"path":"src/a.go","new_line":10}'
        first = subprocess.CompletedProcess([], 0, "조사 메모만 남김", "Permission denied: webfetch")
        second = subprocess.CompletedProcess(
            [], 0,
            '{"skeptic":"tracer","refuted":false,"confidence":75,"reason":"ok",'
            '"evidence":[],"severity_adjust":"keep"}',
            "",
        )
        with patch.object(omo_driver, "run_omo_process", side_effect=[first, second]) as run, \
             patch.object(omo_driver, "session_usage", return_value={k: 0 for k in omo_driver.USAGE_KEYS}), \
             patch.object(omo_driver, "session_models", return_value=set()):
            parsed, _, _, diagnostics = runner.run(
                original, "/tmp", "high", payload_kind="verdict",
                allowed_tools=("read", "grep", "find", "ls"),
                max_turns=18,
            )

        retry_cmd = run.call_args_list[1].args[0]
        retry_prompt = retry_cmd[-1]
        self.assertEqual(parsed["confidence"], 75)
        self.assertIn(original, retry_prompt)
        self.assertIn("조사 메모만 남김", retry_prompt)
        self.assertNotIn("--no-tools", retry_cmd)
        self.assertEqual(retry_cmd[retry_cmd.index("--tools") + 1], "submit_parnas_verdict")
        self.assertIn("--extension", retry_cmd)
        self.assertEqual(retry_cmd[retry_cmd.index("--permission") + 1],
                         "submit_parnas_verdict=allow")
        self.assertEqual(retry_cmd[retry_cmd.index("--parnas-max-turns") + 1], "1")
        self.assertEqual(len(diagnostics["attempts"]), 2)

    def test_run_accepts_schema_validated_tool_payload_without_final_text(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        verdict = {
            "skeptic": "tracer", "refuted": False, "confidence": 75,
            "reason": "ok", "evidence": [], "severity_adjust": "keep",
        }
        response = subprocess.CompletedProcess([], 0, "", "")
        with patch.object(omo_driver, "run_omo_process", return_value=response), \
             patch.object(omo_driver, "session_tool_payload", return_value=verdict), \
             patch.object(omo_driver, "session_usage", return_value={k: 0 for k in omo_driver.USAGE_KEYS}), \
             patch.object(omo_driver, "session_models", return_value=set()):
            parsed, _, tail, diagnostics = runner.run(
                "prompt", "/tmp", "high", payload_kind="verdict",
                allowed_tools=omo_driver.READ_ONLY_TOOLS,
            )

        self.assertEqual(parsed, verdict)
        self.assertEqual(tail, "")
        self.assertEqual(diagnostics["failure_kind"], None)
        self.assertEqual(diagnostics["attempts"][0]["output_source"], "structured_tool")

    def test_session_tool_payload_reads_latest_matching_tool_call(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            Path(tmp, "prefix-sid.jsonl").write_text(
                '{"type":"message","message":{"role":"assistant","content":['
                '{"type":"toolCall","name":"submit_parnas_verdict","arguments":'
                '{"skeptic":"tracer","refuted":false,"confidence":75,"reason":"ok",'
                '"evidence":[],"severity_adjust":"keep"}}]}}\n',
                encoding="utf-8",
            )
            with patch.object(omo_driver, "OMO_SESSIONS", Path(tmp)):
                payload = omo_driver.session_tool_payload("sid", "verdict")

        self.assertEqual(payload["confidence"], 75)

    def test_select_retry_candidates_keeps_only_prior_abstentions(self) -> None:
        candidates = [
            {"path": f"src/{i}.go", "new_line": i, "title": f"candidate {i}"}
            for i in range(18)
        ]
        previous = {
            "findings": [
                {
                    **candidate,
                    "verification": "skeptics unavailable (abstain)",
                }
                for candidate in candidates[:14]
            ] + [
                {**candidate, "skeptics_passed": True}
                for candidate in candidates[14:]
            ],
        }

        selected = omo_driver.select_retry_candidates(candidates, previous)

        self.assertEqual(len(selected), 14)
        self.assertEqual(selected[0]["path"], "src/0.go")
        self.assertEqual(selected[-1]["path"], "src/13.go")

    def test_run_preserves_parse_and_tool_denial_diagnostics(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        first = subprocess.CompletedProcess([], 0, "analysis only", "Permission denied: webfetch")
        second = subprocess.CompletedProcess([], 0, '{"skeptic":"tracer"', "bash_output is disabled")
        with patch.object(omo_driver, "run_omo_process", side_effect=[first, second]), \
             patch.object(omo_driver, "session_usage", return_value={k: 0 for k in omo_driver.USAGE_KEYS}), \
             patch.object(omo_driver, "session_models", return_value=set()):
            parsed, _, tail, diagnostics = runner.run(
                "prompt", "/tmp", "high", payload_kind="verdict",
                allowed_tools=("read", "grep"),
            )

        self.assertIsNone(parsed)
        self.assertEqual(tail, '{"skeptic":"tracer"')
        self.assertEqual(diagnostics["failure_kind"], "parse_failure")
        self.assertEqual(diagnostics["attempts"][0]["stdout"], "analysis only")
        self.assertEqual(diagnostics["attempts"][0]["stderr"], "Permission denied: webfetch")
        self.assertEqual(diagnostics["attempts"][0]["returncode"], 0)
        self.assertIn("Permission denied: webfetch", diagnostics["attempts"][0]["tool_denials"])
        self.assertTrue(diagnostics["attempts"][1]["parse_error"])

    def test_run_timeout_preserves_partial_output_diagnostics(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        timeout = subprocess.TimeoutExpired(
            ["omo"], 5, output="partial stdout", stderr="Permission denied: bash"
        )
        with patch.object(omo_driver, "run_omo_process", side_effect=timeout), \
             patch.object(omo_driver, "session_usage", return_value={k: 0 for k in omo_driver.USAGE_KEYS}):
            parsed, _, tail, diagnostics = runner.run("prompt", "/tmp", "high", payload_kind="verdict")

        self.assertIsNone(parsed)
        self.assertEqual(tail, "partial stdout")
        self.assertEqual(diagnostics["failure_kind"], "timeout")
        self.assertTrue(diagnostics["attempts"][0]["timed_out"])
        self.assertEqual(diagnostics["attempts"][0]["stdout"], "partial stdout")

    def test_timeout_kills_the_entire_omo_process_group(self) -> None:
        class FakeProcess:
            pid = 123
            returncode = -9

            def __init__(self):
                self.calls = 0

            def communicate(self, timeout=None):
                self.calls += 1
                if self.calls == 1:
                    raise subprocess.TimeoutExpired(["omo"], timeout)
                return "", "killed"

        process = FakeProcess()
        with patch.object(omo_driver.subprocess, "Popen", return_value=process) as popen, \
             patch.object(omo_driver.os, "killpg") as killpg:
            with self.assertRaises(subprocess.TimeoutExpired):
                omo_driver.run_omo_process(["omo"], "/tmp", 1)
        self.assertEqual(popen.call_args.kwargs["start_new_session"], True)
        self.assertEqual(popen.call_args.kwargs["cwd"], str(Path("/tmp").resolve()))
        killpg.assert_called_once_with(123, omo_driver.signal.SIGKILL)
        self.assertEqual(process.calls, 2)

    def test_run_rejects_a_session_that_used_another_model(self) -> None:
        runner = omo_driver.OmoRunner("zai", "glm-5.3-flash")
        response = subprocess.CompletedProcess(
            [], 0,
            '{"skeptic": "tracer", "refuted": false, "confidence": 75, "reason": "ok", "evidence": [], "severity_adjust": "keep"}',
            "",
        )
        with patch.object(omo_driver, "run_omo_process", return_value=response), \
             patch.object(omo_driver, "session_models", return_value={"opencode-go/kimi-k2.6"}):
            parsed, _, tail, diagnostics = runner.run("prompt", "/tmp", "high", payload_kind="verdict")
        self.assertIsNone(parsed)
        self.assertIn("unpinned model", tail)
        self.assertEqual(diagnostics["failure_kind"], "model_mismatch")

    def test_run_batch_passes_role_permission_and_payload_kind(self) -> None:
        calls = []

        class FakeRunner:
            def run(self, *args, **kwargs):
                calls.append(kwargs)
                return (
                    {"skeptic": "tracer", "refuted": False, "confidence": 75, "reason": "ok",
                     "evidence": [], "severity_adjust": "keep"},
                    {k: 0 for k in omo_driver.USAGE_KEYS},
                    "",
                )

        results = omo_driver.run_batch(FakeRunner(), [{
            "prompt": "p", "cwd": "/tmp", "thinking": "high", "permission_preset": "workspace",
            "payload_kind": "verdict", "allowed_tools": ("read", "bash"),
            "max_turns": 18,
            "label": "repro:a.go:1",
        }], 1)
        self.assertEqual(results[0]["parsed"]["confidence"], 75)
        self.assertEqual(calls, [{
            "permission_preset": "workspace", "payload_kind": "verdict",
            "allowed_tools": ("read", "bash"),
            "max_turns": 18,
        }])


class SelfReviewFixTest(unittest.TestCase):
    """PR #498 parnas 자기 리뷰가 발견한 결함의 회귀 테스트 (F1~F6)."""

    def test_dedup_merges_on_why_similarity_not_what(self) -> None:
        """F3: workflow.js 는 마지막 절에 why 유사도를 쓴다 — what 이 아니라."""
        finders = [
            {"lenses": ["logic"], "candidates": [
                {"path": "a.go", "new_line": 10, "category": "bug", "title": "제목 하나",
                 "what": "전혀 다른 서술 alpha", "why": "null 역참조로 프로세스가 크래시", "confidence": 60, "lens": "logic"}]},
            {"lenses": ["boundary"], "candidates": [
                {"path": "b.go", "new_line": 10, "category": "bug", "title": "관계없는 제목 둘",
                 "what": "전혀 다른 서술 gamma", "why": "null 역참조로 프로세스가 크래시", "confidence": 70, "lens": "boundary"}]},
        ]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["confidence"], 70)

    def test_dedup_skips_non_dict_candidates(self) -> None:
        """F6: 스키마를 벗어난 finder 응답이 크래시를 일으키지 않는다."""
        finders = [{"lenses": ["logic"], "candidates": ["junk-string",
                                                            {"path": "a.go", "new_line": 1, "category": "bug", "title": "t", "lens": "logic"}]}]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["path"], "a.go")

    def test_phase_find_empty_units_returns_empty_without_agents(self) -> None:
        """F2: units 가 비면 에이전트 없이 빈 구조체를 반환한다 (IndexError 아님)."""
        a = {"units": [], "checkout": "/tmp"}
        found = omo_driver.phase_find(a, runner=None)
        self.assertEqual(found["finders"], [])
        self.assertEqual(found["candidates"], [])
        self.assertEqual(found["prescreened"], [])
        self.assertEqual(found["verified_ok"], [])

    def test_phase_find_marks_malformed_agent_response_degraded(self) -> None:
        class FakeRunner:
            def run(self, *args, **kwargs):
                return (
                    {"lenses": ["logic"], "reviewed_files": ["a.go"], "inspected": [],
                     "verified_ok": [], "candidates": [{
                        "path": "a.go", "new_line": "1", "end_line": None, "severity": "high",
                        "category": "bug", "title": "t", "what": "w", "why": "why",
                        "evidence": [], "confidence": "75", "lens": "logic",
                    }]},
                    {k: 0 for k in omo_driver.USAGE_KEYS},
                    "malformed",
                )

        a = {
            "units": [{"id": "logic@all", "lenses": ["logic"], "pack": "pack.md"}],
            "checkout": "/tmp", "outDir": "/tmp", "lensText": {"logic": "logic"},
            "thinking": {"finder": "high"}, "finder_turns": 24, "per_lens_cap": 3,
            "workers": {"finder": 1},
            "max_candidates": 24, "hunkRanges": {}, "refutedHistory": [],
        }
        found = omo_driver.phase_find(a, FakeRunner())
        self.assertTrue(found["degraded"])
        self.assertEqual(found["candidates"], [])
        self.assertEqual(found["agent_failures"], ["find:logic@all"])

    def test_phase_find_marks_missing_file_receipt_degraded(self) -> None:
        class FakeRunner:
            def run(self, *args, **kwargs):
                return (
                    {"lenses": ["logic"], "reviewed_files": ["src/a.go"],
                     "inspected": ["src/a.go"], "verified_ok": [], "candidates": []},
                    {k: 0 for k in omo_driver.USAGE_KEYS},
                    "",
                )

        a = {
            "units": [{"id": "logic@all", "lenses": ["logic"], "pack": "pack.md",
                       "files": ["src/a.go", "src/b.go"]}],
            "checkout": "/tmp", "outDir": "/tmp", "lensText": {"logic": "logic"},
            "thinking": {"finder": "high"}, "finder_turns": 24, "per_lens_cap": 3,
            "workers": {"finder": 1},
            "max_candidates": 24, "hunkRanges": {}, "refutedHistory": [],
        }

        found = omo_driver.phase_find(a, FakeRunner())

        self.assertTrue(found["degraded"])
        self.assertEqual(found["failure_counts"]["coverage_gap"], 1)
        self.assertEqual(found["coverage"]["gaps"], [
            {"unit": "logic@all", "missing_files": ["src/b.go"]},
        ])

    def test_phase_verify_preserves_carried_findings(self) -> None:
        carried = [{"path": "src/a.go", "new_line": 10, "title": "carried"}]
        a = {
            "checkout": "/tmp", "workers": {"tracer": 1, "reproducer": 1},
            "thinking": {"tracer": "high", "reproducer": "high"},
            "profile": "omo-flash", "provider": "zai", "model": "glm-5.3-flash",
            "carried": carried,
        }
        found = {
            "candidates": [], "prescreened": [], "finders": [],
            "verified_ok": [], "usage_find": {k: 0 for k in omo_driver.USAGE_KEYS},
        }
        result = omo_driver.phase_verify(a, None, found)
        self.assertEqual(result["findings"], carried)
        self.assertEqual(result["status"], "ok")

    def test_phase_verify_marks_failed_skeptics_degraded(self) -> None:
        class FakeRunner:
            def run(self, *args, **kwargs):
                return None, {k: 0 for k in omo_driver.USAGE_KEYS}, "failed"

        a = {
            "checkout": "/tmp", "outDir": "/tmp", "skeptic_turns": 18,
            "workers": {"tracer": 1, "reproducer": 1},
            "thinking": {"tracer": "high", "reproducer": "high"},
            "profile": "omo-flash", "provider": "zai", "model": "glm-5.3-flash",
            "carried": [],
        }
        candidate = {
            "path": "src/a.go", "new_line": 10, "end_line": None, "severity": "medium",
            "category": "bug", "title": "t", "what": "w", "why": "why",
            "evidence": [], "confidence": 75, "lens": "logic",
        }
        found = {
            "candidates": [candidate], "prescreened": [], "finders": [],
            "verified_ok": [], "usage_find": {k: 0 for k in omo_driver.USAGE_KEYS},
        }
        result = omo_driver.phase_verify(a, FakeRunner(), found)
        self.assertEqual(result["status"], "degraded")
        self.assertEqual(result["agent_failures"], ["tracer:a.go:10", "repro:a.go:10"])
        self.assertEqual(result["failure_counts"]["parse_failure"], 2)
        self.assertEqual(result["failure_counts"]["low_confidence_abstain"], 0)

    def test_phase_verify_abstains_on_unverified_low_confidence_verdicts(self) -> None:
        class FakeRunner:
            def __init__(self):
                self.calls = 0

            def run(self, *args, **kwargs):
                self.calls += 1
                skeptic = "tracer" if self.calls == 1 else "reproducer"
                return (
                    {"skeptic": skeptic, "refuted": False, "confidence": 20,
                     "reason": "미확인: 파일을 읽지 못함", "evidence": [],
                     "severity_adjust": "keep"},
                    {k: 0 for k in omo_driver.USAGE_KEYS},
                    "",
                )

        a = {
            "checkout": "/tmp", "outDir": "/tmp", "skeptic_turns": 18,
            "workers": {"tracer": 1, "reproducer": 1},
            "thinking": {"tracer": "high", "reproducer": "high"},
            "profile": "omo-flash", "provider": "zai", "model": "glm-5.3-flash",
            "carried": [],
        }
        candidate = {
            "path": "src/a.go", "new_line": 10, "end_line": None, "severity": "medium",
            "category": "bug", "title": "t", "what": "w", "why": "why",
            "evidence": [], "confidence": 75, "lens": "logic",
        }
        found = {
            "candidates": [candidate], "prescreened": [], "finders": [],
            "verified_ok": [], "usage_find": {k: 0 for k in omo_driver.USAGE_KEYS},
        }

        result = omo_driver.phase_verify(a, FakeRunner(), found)

        self.assertEqual(result["status"], "degraded")
        self.assertEqual(result["agent_failures"], [])
        self.assertEqual(result["low_confidence_abstains"], ["tracer:a.go:10", "repro:a.go:10"])
        self.assertEqual(result["failure_counts"]["parse_failure"], 0)
        self.assertEqual(result["failure_counts"]["low_confidence_abstain"], 2)
        self.assertEqual(result["failure_counts"]["timeout"], 0)
        self.assertEqual(len(result["findings"]), 1)
        self.assertEqual(result["findings"][0]["confidence"], 50)
        self.assertEqual(result["findings"][0]["verification"], "skeptics unavailable (abstain)")
        self.assertNotIn("skeptics_passed", result["findings"][0])

    def test_unverified_reason_abstains_even_with_high_confidence(self) -> None:
        verdict = {
            "refuted": False, "confidence": 75, "reason": "미확인: 파일을 읽지 못함",
        }
        self.assertTrue(omo_driver._is_unverified_verdict(verdict))


if __name__ == "__main__":
    unittest.main()
