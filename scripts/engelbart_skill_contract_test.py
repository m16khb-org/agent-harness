#!/usr/bin/env python3

from __future__ import annotations

import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILL = ROOT / "skills" / "engelbart" / "SKILL.md"
OPENAI = ROOT / "skills" / "engelbart" / "agents" / "openai.yaml"
GITIGNORE = ROOT / ".gitignore"
LOCAL_BACKGROUND = ROOT / "skills" / "engelbart" / "background.local.md"
TESTDATA = ROOT / "skills" / "engelbart" / "testdata"
HANDOFF = TESTDATA / "ai_devops_onboarding_handoff.md"
BAD_HANDOFF = TESTDATA / "ai_devops_onboarding_handoff.bad.md"
RUBRIC = ROOT / "scripts" / "engelbart_quality_rubric.py"


class EngelbartSkillContractTest(unittest.TestCase):
    def read_skill(self) -> str:
        self.assertTrue(SKILL.exists(), "skills/engelbart/SKILL.md must exist")
        return SKILL.read_text(encoding="utf-8")

    def assert_ordered(self, content: str, needles: list[str]) -> None:
        positions = [content.index(needle) for needle in needles]
        self.assertEqual(positions, sorted(positions), f"expected ordered headings: {needles}")

    def template_block(self, content: str) -> str:
        start = content.index("## Meeting Canvas Template")
        end = content.index("The full transcript must remain", start)
        return content[start:end]

    def test_skill_is_named_after_engelbart(self) -> None:
        content = self.read_skill()
        old_slug = "clova" + "-meeting-minutes"
        old_display_name = "Clova " + "Meeting Minutes"

        self.assertIn("name: engelbart", content)
        self.assertIn("# Engelbart", content)
        self.assertNotIn(old_slug, content)
        self.assertNotIn(old_display_name, content)

        self.assertTrue(OPENAI.exists(), "skills/engelbart/agents/openai.yaml must exist")
        metadata = OPENAI.read_text(encoding="utf-8")
        self.assertIn('display_name: "Engelbart"', metadata)
        self.assertIn("$engelbart", metadata)
        self.assertNotIn("$" + old_slug, metadata)

    def test_canvas_defaults_and_channel_override_are_documented(self) -> None:
        content = self.read_skill()

        self.assertIn("#dev-team-backend", content)
        self.assertIn("기본 대상 채널", content)
        self.assertIn("다른 채널", content)
        self.assertIn("채널 override", content)
        self.assertIn("Default artifact: create or draft only the individual meeting Canvas", content)
        self.assertIn("Do not create, update, search, or manage Slack Lists", content)
        self.assertIn("After a Canvas is created and read back, provide a separate manual index-binding handoff", content)
        self.assertNotIn("When Slack List tools are available", content)
        self.assertNotIn("reuse and update that List", content)
        self.assertNotIn("do not create a duplicate index List", content)

    def test_local_background_file_is_configured_for_term_and_speaker_resolution(self) -> None:
        content = self.read_skill()
        gitignore = GITIGNORE.read_text(encoding="utf-8")

        self.assertIn("## Local Background", content)
        self.assertIn("skills/engelbart/background.local.md", content)
        self.assertIn("skills/engelbart/background.local.example.md", content)
        self.assertIn("speaker labels", content)
        self.assertIn("high-confidence correction candidates", content)
        self.assertIn("Standalone Korean honorific-like words", content)
        self.assertIn("`프로님` can resolve to `이푸름 님`", content)
        self.assertIn("`{name} 프로님` should be treated as a title/honorific", content)
        self.assertIn("Do not silently convert a generic speaker label", content)
        self.assertIn("skills/*/background.local.md", gitignore)

    def test_local_background_is_optional_and_ignored(self) -> None:
        if not LOCAL_BACKGROUND.exists():
            self.skipTest("local background is gitignored and optional in clean checkouts")

        local_background = LOCAL_BACKGROUND.read_text(encoding="utf-8")

        self.assertIn("## Team", local_background)
        self.assertIn("## Products And Services", local_background)
        self.assertIn("## Aliases", local_background)
        self.assertIn("프로님: 이푸름", local_background)

    def test_required_meeting_inputs_are_requested_sequentially(self) -> None:
        content = self.read_skill()

        self.assertIn("## Required Meeting Inputs", content)
        self.assertIn("Input collection order is sequential", content)
        self.assertIn("When no participant list is present, ask only for the participant list", content)
        self.assertIn("Do not ask for the transcript in the same response", content)
        self.assertIn("After the participant list is provided, confirm the received participants", content)
        self.assertIn("then ask for the meeting transcript text", content)
        self.assertIn("When both inputs are present, continue with the normal Engelbart workflow", content)

    def test_manual_index_binding_handoff_replaces_list_control(self) -> None:
        content = self.read_skill()

        self.assertIn("## Manual Index Binding Handoff", content)
        self.assertIn("The agent creates and verifies the meeting Canvas only", content)
        self.assertIn("The user manually binds that Canvas into their Slack List", content)
        self.assertIn("수동 List 바인딩 값", content)
        for field in ["- 이름:", "- Date:", "- Topic:", "- Status:", "- Counts:", "- Meeting Canvas:"]:
            self.assertIn(field, content)
        self.assertIn("Do not call Slack List tools", content)
        self.assertIn("Do not create an index Canvas as a substitute for the List", content)
        self.assertIn("Do not put an index row preview in the meeting Canvas body by default", content)
        self.assertIn("set handoff `Date` to the same value as `Last updated`", content)
        self.assertIn("Use Slack's built-in List `이름` field as the meeting title", content)
        self.assertNotIn("## Meeting Index List Schema", content)
        self.assertNotIn("Render the persistent index as a Slack List by default", content)
        self.assertNotIn("Manual Slack List creation when tools are unavailable", content)

    def test_manual_index_binding_handoff_is_forward_tested(self) -> None:
        self.assertTrue(HANDOFF.exists(), "good manual handoff fixture must exist")
        self.assertTrue(BAD_HANDOFF.exists(), "bad manual handoff fixture must exist")
        self.assertTrue(RUBRIC.exists(), "quality rubric script must exist")

        handoff = HANDOFF.read_text(encoding="utf-8")
        bad_handoff = BAD_HANDOFF.read_text(encoding="utf-8")
        rubric = RUBRIC.read_text(encoding="utf-8")

        for field in [
            "수동 List 바인딩 값",
            "- 이름: AI DevOps R&R 및 추천 시스템 온보딩",
            "- Date: 2026-06-24",
            "- Topic: 온보딩",
            "- Status: Follow-up 필요",
            "- Counts: 결정 9 / 액션 10 / 질문 6",
            "- Meeting Canvas: https://bubbletap.slack.com/docs/T048JBUDF9U/F0BDLM3631N",
        ]:
            self.assertIn(field, handoff)
        self.assertNotIn("- Title:", handoff)
        self.assertNotIn("생성 후 인덱스 참조", handoff)

        self.assertIn("- Title:", bad_handoff)
        self.assertIn("생성 후 인덱스 참조", bad_handoff)
        self.assertIn("manual_handoff_failures", rubric)
        self.assertIn("manual_handoff_uses_title_field", rubric)
        self.assertIn("manual_handoff_placeholder", rubric)
        self.assertIn("manual_handoff_missing_canvas_url", rubric)

    def test_meeting_canvas_schema_preserves_audit_appendix(self) -> None:
        content = self.read_skill()

        self.assertIn("## Canvas UI/UX Principles", content)
        self.assertIn("Top callout box UI", content)
        self.assertIn("rounded box", content)
        self.assertIn("tinted background", content)
        self.assertIn("집계 기간", content)
        self.assertIn("Progressive disclosure", content)
        self.assertIn("Layer-cake headings", content)
        self.assertIn("Use tables only where comparison helps", content)
        self.assertIn("::: {.callout}", content)
        self.assertIn("## Slack Canvas UI Block Palette", content)
        for block in [
            "Callout box",
            "2-column vertical table",
            "Narrow table",
            "Checklist bullets",
            "Heading hierarchy",
            "Slack emoji codes in headings",
            "Layout columns",
            "Horizontal rule",
            "Block quote",
            "Code block",
            "Links and Slack references",
        ]:
            self.assertIn(block, content)
        self.assertIn("Default meeting Canvas UI recipe", content)
        self.assertIn("Do not put tables, callouts, or transcript/code blocks inside layouts", content)
        self.assertIn("## Canvas UI Pattern Examples", content)
        self.assertIn("Status callout", content)
        self.assertIn("Metadata table", content)
        self.assertIn("Action checklist", content)
        self.assertIn("Audit divider", content)
        self.assertIn("Transcript block", content)
        self.assertIn("Do not overfit the UI proof Canvas", content)
        self.assertIn("## Canvas Anti-Patterns", content)
        self.assertIn("Do not use one dense paragraph for TL;DR", content)
        self.assertIn("Do not use tables for decisions, actions, risks, or corrections by default", content)
        self.assertIn("Do not put callouts, tables, or code blocks inside layout columns", content)
        self.assertIn("Do not create a skeleton Canvas and stop there", content)
        self.assertIn("Do not hide uncertainty in decisions", content)
        self.assertIn("회의일 YYYY-MM-DD", content)
        self.assertIn("## 메타데이터", content)
        self.assertIn("| Field | Value |", content)
        self.assertIn("| Date | YYYY-MM-DD. If the source has no explicit meeting date, use `Last updated`. |", content)
        self.assertIn("| Last updated | YYYY-MM-DD |", content)
        self.assertIn("- [ ] {담당}: {산출물 중심 작업}", content)
        for heading in [
            "## TL;DR",
            "## 결정사항",
            "## 액션 보드",
            "## 주제별 논의",
            "## 후속 확인",
            "## 리스크/열린 질문",
            "## 보정 및 원문 부록",
            "### 용어 보정",
            "### 불확실 단어/문장 보정",
            "### 참석자/화자 보정",
            "### 원문 전사본 전문",
        ]:
            self.assertIn(heading, content)
        self.assert_ordered(
            self.template_block(content),
            [
                "## TL;DR",
                "## 결정사항",
                "## 액션 보드",
                "## 주제별 논의",
                "## 후속 확인",
                "## 리스크/열린 질문",
                "## 보정 및 원문 부록",
            ],
        )
        self.assertIn("multi-line bullets", content)
        self.assertIn("long one-line decision", content)
        self.assertIn("원문 표현", content)
        self.assertIn("보정 표현", content)
        self.assertIn("speaker/timestamp", content)
        self.assertIn("verbatim block", content)
        self.assertIn("표로 넣지 않는다", content)

    def test_quality_rules_cover_canvas_limits_and_sensitive_redaction(self) -> None:
        content = self.read_skill()

        self.assertIn("top callout box", content)
        self.assertIn("scope/status facts", content)
        self.assertIn("rounded, padded box", content)
        self.assertIn("default UI recipe", content)
        self.assertIn("top-level divider before the audit appendix", content)
        self.assertIn("Render `TL;DR` as 2-4 bullets", content)
        self.assertIn("Do not use layout columns for the default meeting minutes body", content)
        self.assertIn("300 cells", content)
        self.assertIn("용어 보정 1", content)
        self.assertIn("Any table over 5 columns is a quality failure", content)
        self.assertIn("absence of index-row sections", content)
        self.assertIn("Keep actions as checklist bullets", content)
        self.assertIn("secret/token/password", content)
        self.assertIn("[민감정보 생략]", content)
        self.assertIn("결정사항에는 불확실한 내용을 넣지 않는다", content)
        self.assertIn("Slack/GitLab/문서 링크", content)

    def test_forward_quality_rubric_is_documented(self) -> None:
        content = self.read_skill()

        self.assertIn("## Forward Quality Rubric", content)
        self.assertIn("baseline score", content)
        self.assertIn("pass line", content)
        self.assertIn("Do not fabricate meeting dates from the transcript", content)
        self.assertIn("set `Date` to the same value as `Last updated`", content)
        self.assertIn("실명이 전사본만으로 확정되지 않으면", content)
        self.assertIn("용어 보정 후보를 먼저 작성", content)
        self.assertIn("Verbatim full transcript handling", content)
        self.assertIn("The pasted transcript must appear exactly once", content)
        self.assertIn("marker count or hash", content)
        self.assertIn("Do not summarize, normalize, translate, or substitute representative blocks", content)
        self.assertIn("원문 전사본 발췌", content)
        self.assertIn("provide separate manual index-binding handoff values that use Slack's built-in `이름` field as the title", content)
        self.assertIn("exactly one `### 원문 전사본 전문` heading", content)
        self.assertIn("Duplicate adjacent transcript headings", content)
        self.assertIn("compact top callout box", content)
        self.assertIn("rounded box UI", content)
        self.assertIn("TL;DR` -> `결정사항` -> `액션 보드` -> `주제별 논의` -> `후속 확인` -> `리스크/열린 질문`", content)
        self.assertIn("decisions use multi-line bullets", content)
        self.assertIn("two-line meeting summary is a failed output", content)
        self.assertIn("A created or read-back meeting Canvas must not retain placeholder cells", content)
        self.assertIn("must not contain Canvas URL placeholders such as `미정`, `{Canvas 링크}`, or `생성 후 인덱스 참조`", content)
        self.assertIn("If local background and meeting role identify a team lead owner", content)
        self.assertIn("Do not leave team-lead-owned follow-ups as `참석자 1`", content)
        self.assertIn("A one-line risk that packs all fields into one sentence is a Slack Canvas readability failure", content)
        self.assertIn("python3 scripts/engelbart_quality_rubric.py", content)

    def test_slack_canvas_write_path_uses_incremental_updates(self) -> None:
        content = self.read_skill()

        self.assertIn("## Slack Canvas Write Path", content)
        self.assertIn("prefer direct Canvas creation", content)
        self.assertIn("Sanitize the Slack API title", content)
        self.assertIn("literal `&`", content)
        self.assertIn("AI DevOps R and R", content)
        self.assertIn("create the complete Canvas in one `create_canvas` call", content)
        self.assertIn("Read the created Canvas back", content)
        self.assertIn("retry once with a sanitized title", content)
        self.assertIn("fall back to a skeleton Canvas", content)
        self.assertIn("Read the created Canvas to capture section IDs", content)
        self.assertIn("When replacing a header section ID", content)
        self.assertIn("replacement body must not repeat the same heading", content)
        self.assertIn("not another `### 원문 전사본 전문` line", content)
        self.assertIn("reject adjacent duplicate headings", content)
        self.assertIn("2-column metadata table", content)
        self.assertIn("manual index-binding handoff", content)
        self.assertIn("Do not leave the skeleton as the final artifact", content)
        self.assertIn("Invalid text passed", content)
        self.assertIn("do not use tools", content)
        self.assertIn("The user manually binds the meeting Canvas into their Slack List", content)
        self.assertIn("Avoid destructive full-canvas replace", content)


if __name__ == "__main__":
    unittest.main()
