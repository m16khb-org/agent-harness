#!/usr/bin/env python3
"""Offline tests for review_threads.py (no network, no gh/glab)."""
from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("review_threads.py")


def load():
    spec = importlib.util.spec_from_file_location("review_threads", SCRIPT)
    assert spec and spec.loader
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class ClassifyAuthorTest(unittest.TestCase):
    def setUp(self):
        self.m = load()

    def test_known_reviewers_by_login(self):
        cases = {
            "coderabbitai[bot]": ("bot", "coderabbit", "@coderabbitai"),
            "coderabbitai": ("bot", "coderabbit", "@coderabbitai"),
            "copilot-pull-request-reviewer[bot]": ("bot", "copilot", None),
            "gemini-code-assist[bot]": ("bot", "gemini", "@gemini-code-assist"),
            "kody-bot": ("bot", "kody", "@kody"),
        }
        for login, want in cases.items():
            self.assertEqual(self.m.classify_author(login, ""), want, login)

    def test_kody_via_body_marker_on_project_bot(self):
        got = self.m.classify_author("project_123_bot_ab12", "<!-- kody-codereview --> Severity: high")
        self.assertEqual(got, ("bot", "kody", "@kody"))

    def test_unknown_bot_by_type_or_pattern(self):
        self.assertEqual(self.m.classify_author("some-bot", "")[0:2], ("bot", "unknown"))
        self.assertEqual(self.m.classify_author("whatever", "", author_type="Bot")[0:2], ("bot", "unknown"))
        self.assertEqual(self.m.classify_author("project_9_bot_x", "", is_bot_flag=True)[0:2], ("bot", "unknown"))

    def test_human(self):
        self.assertEqual(self.m.classify_author("m16khb", "LGTM"), ("human", None, None))


class ParseRefTest(unittest.TestCase):
    def setUp(self):
        self.m = load()

    def test_urls_and_shorthands(self):
        self.assertEqual(self.m.parse_ref("https://github.com/acme/billing/pull/412"), ("github.com", "acme/billing", 412))
        self.assertEqual(self.m.parse_ref("https://gitlab.example.com/platform/api/-/merge_requests/5581"), ("gitlab.example.com", "platform/api", 5581))
        self.assertEqual(self.m.parse_ref("https://gitlab.example.com/grp/sub/api/merge_requests/7"), ("gitlab.example.com", "grp/sub/api", 7))
        self.assertEqual(self.m.parse_ref("!12"), (None, None, 12))
        self.assertEqual(self.m.parse_ref("#12"), (None, None, 12))
        self.assertEqual(self.m.parse_ref("12"), (None, None, 12))

    def test_bad_ref(self):
        with self.assertRaises(SystemExit):
            self.m.parse_ref("nope")


class PlanTest(unittest.TestCase):
    def setUp(self):
        self.m = load()

    def test_defaults_from_verdict(self):
        plan = {"threads": [
            {"thread_id": "a", "verdict": "valid", "reply": "ok"},
            {"thread_id": "b", "verdict": "invalid", "reply": "no"},
            {"thread_id": "c", "verdict": "hold", "reply": "wait", "reason_open": "human decision pending"},
        ]}
        got = self.m.normalize_plan(plan)
        self.assertEqual([(g["reaction"], g["resolve"]) for g in got], [("up", True), ("down", True), ("none", False)])

    def test_hold_without_reason_rejected(self):
        with self.assertRaises(SystemExit):
            self.m.normalize_plan({"threads": [{"thread_id": "a", "verdict": "valid", "reply": "x", "resolve": False}]})

    def test_missing_reply_rejected_unless_skip(self):
        with self.assertRaises(SystemExit):
            self.m.normalize_plan({"threads": [{"thread_id": "a", "verdict": "valid"}]})
        got = self.m.normalize_plan({"threads": [{"thread_id": "a", "verdict": "valid", "skip_reply": True}]})
        self.assertEqual(got[0]["reply"], "")

    def test_bad_verdict_rejected(self):
        with self.assertRaises(SystemExit):
            self.m.normalize_plan({"threads": [{"thread_id": "a", "verdict": "maybe", "reply": "x"}]})


class ReplyComposeTest(unittest.TestCase):
    def setUp(self):
        self.m = load()

    def test_mention_and_marker(self):
        body = self.m.compose_reply("이 리뷰는 **타당합니다.**", "T1", "valid", "@kody")
        self.assertTrue(body.startswith("@kody 이 리뷰는"))
        self.assertEqual(self.m.find_marker(body), ("T1", "valid"))

    def test_no_mention_for_non_learning_reviewer(self):
        body = self.m.compose_reply("본문", "T2", "invalid", None)
        self.assertTrue(body.startswith("본문"))
        self.assertEqual(self.m.find_marker(body), ("T2", "invalid"))

    def test_mention_not_duplicated(self):
        body = self.m.compose_reply("@coderabbitai 본문", "T3", "partial", "@coderabbitai")
        self.assertEqual(body.count("@coderabbitai"), 1)


class NormalizeThreadTest(unittest.TestCase):
    def setUp(self):
        self.m = load()

    def raw(self, replies):
        return {"thread_id": "D1", "resolvable": True, "resolved": False, "path": "src/a.ts", "line": 10,
                "notes": [{"id": 1, "body": "<!-- kody-codereview --> null check", "author": "project_1_bot_x"}] + replies}

    def test_detects_prior_handling_by_me(self):
        m = self.m
        t = m.normalize_thread(self.raw([{"id": 2, "body": "@kody 타당 " + m.marker("D1", "valid"), "author": "m16khb"}]), "m16khb")
        self.assertEqual(t["reviewer"], "kody")
        self.assertEqual(t["already_handled"], "valid")
        self.assertEqual(t["my_reply_count"], 1)

    def test_other_users_reply_is_not_handling(self):
        t = self.m.normalize_thread(self.raw([{"id": 2, "body": "동의", "author": "someone"}]), "m16khb")
        self.assertIsNone(t["already_handled"])
        self.assertEqual(t["reply_count"], 1)


class GitLabNormTest(unittest.TestCase):
    def test_system_notes_and_position(self):
        m = load()
        gl = m.GitLab("gitlab.example.com", "grp/proj", 5)
        d = {"id": "abc", "notes": [
            {"id": 9, "system": True, "body": "changed the description", "author": {"username": "x"}},
            {"id": 10, "system": False, "body": "<!-- kody-codereview --> issue", "resolvable": True, "resolved": False,
             "author": {"username": "project_3_bot_1", "bot": True}, "position": {"new_path": "a.go", "new_line": 3}},
        ]}
        t = gl._norm(d)
        self.assertEqual(t["notes"][0]["id"], 10)
        self.assertEqual((t["path"], t["line"], t["resolvable"]), ("a.go", 3, True))
        self.assertEqual(gl.enc, "grp%2Fproj")


class CliDryRunTest(unittest.TestCase):
    def test_apply_dry_run_without_network(self):
        """Patch the provider to a fake so `apply --dry-run` runs end-to-end offline."""
        m = load()

        class Fake:
            name = "github"; host = None; project = "acme/billing"; number = 412
            def me(self): return "m16khb"
            def threads(self):
                return [{"thread_id": "T1", "resolve_id": "T1", "resolvable": True, "resolved": False, "outdated": False, "path": "a.ts", "line": 1,
                         "notes": [{"id": 100, "body": "x", "author": "coderabbitai[bot]", "author_type": "Bot"}]},
                        {"thread_id": "T2", "resolve_id": "T2", "resolvable": False, "resolved": False, "outdated": False, "path": None, "line": None,
                         "notes": [{"id": 101, "body": "<!-- kody-pr-summary -->", "author": "kody"}]}], {"head_sha": "deadbeef", "state": "OPEN"}
            def reactions_by(self, note_id, login): return ["up"] if note_id == 100 else []

        m.make_provider = lambda args: Fake()
        with tempfile.TemporaryDirectory() as td:
            plan = Path(td) / "plan.json"
            plan.write_text(json.dumps({"threads": [
                {"thread_id": "T1", "verdict": "valid", "reply": "맞습니다"},
                {"thread_id": "T2", "verdict": "invalid", "reply": "요약 스레드", "resolve": True},
                {"thread_id": "T9", "verdict": "valid", "reply": "없는 스레드"},
            ]}), encoding="utf-8")
            import io, contextlib
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                m.main(["apply", "--pr", "412", "--plan", str(plan), "--dry-run"])
            out = json.loads(buf.getvalue())
        self.assertTrue(out["dry_run"])
        rows = {r["thread_id"]: r for r in out["ledger"]}
        self.assertEqual(rows["T1"]["planned"], ["reply", "resolve"])  # reaction already present → not repeated
        self.assertIn("@coderabbitai", rows["T1"]["reply_preview"])
        self.assertEqual(rows["T2"]["resolve_action"], "not_resolvable")
        self.assertEqual(rows["T2"]["planned"], ["reply", "react"])
        self.assertEqual(rows["T9"]["status"], "error")
        self.assertFalse(out["ok"])


if __name__ == "__main__":
    unittest.main()
