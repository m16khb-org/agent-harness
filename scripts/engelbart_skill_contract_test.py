#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import hashlib
import re
import subprocess
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
PUBLISH_SCRIPT = ROOT / "skills" / "engelbart" / "scripts" / "publish_meeting_canvas.py"


class EngelbartSkillContractTest(unittest.TestCase):
    def read_skill(self) -> str:
        self.assertTrue(SKILL.exists(), "skills/engelbart/SKILL.md must exist")
        return SKILL.read_text(encoding="utf-8")

    def load_publish_script(self):
        spec = importlib.util.spec_from_file_location("publish_meeting_canvas", PUBLISH_SCRIPT)
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

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

        self.assertIn("#sample-platform-team", content)
        self.assertIn("기본 대상 채널", content)
        self.assertIn("다른 채널", content)
        self.assertIn("채널 override", content)
        self.assertIn("Default artifact: create the individual meeting Canvas for `#sample-platform-team`", content)
        self.assertIn("register the meeting in the existing Slack List", content)
        self.assertIn("Slack List registration is automatic for published meeting Canvases", content)
        self.assertIn("After a Canvas is created and read back, register or update the existing Slack List row", content)
        self.assertNotIn("When Slack List tools are available", content)
        self.assertNotIn("Do not create, update, search, or manage Slack Lists", content)

    def test_local_background_file_is_configured_for_term_and_speaker_resolution(self) -> None:
        content = self.read_skill()
        gitignore = GITIGNORE.read_text(encoding="utf-8")

        self.assertIn("## Local Background", content)
        self.assertIn("skills/engelbart/background.local.md", content)
        self.assertIn("skills/engelbart/background.local.example.md", content)
        self.assertIn("speaker labels", content)
        self.assertIn("high-confidence correction candidates", content)
        self.assertIn("Standalone Korean honorific-like words", content)
        self.assertIn("Standalone Korean honorific-like words may be name misrecognitions", content)
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
        self.assertRegex(local_background, r"(?m)^.*프로님.*$", "local background should define the 프로님 alias")

    def test_required_meeting_inputs_are_requested_sequentially(self) -> None:
        content = self.read_skill()

        self.assertIn("## Required Meeting Inputs", content)
        self.assertIn("Input collection order is sequential", content)
        self.assertIn("When no participant list is present, ask only for the participant list", content)
        self.assertIn("Do not ask for the transcript in the same response", content)
        self.assertIn("After the participant list is provided, confirm the received participants", content)
        self.assertIn("then ask for the meeting transcript text", content)
        self.assertIn("When both inputs are present, continue with the normal Engelbart workflow", content)

    def test_synthetic_fixture_family_has_no_identified_meeting_data(self) -> None:
        forbidden_fingerprints = (
            (6, 264263212423224, "0e3df5670eda1a187577a904e72466d118a48df6e1b81925dc7ea2d1a09a9e23"),
            (6, 266323265947531, "061cdcb5a59894767355f44c46e53da39ff9ac1778a00126d375e73883dd0ff7"),
            (6, 267491847855948, "cd07f8a79313c9b09aede56be3de3511dfab486f307fa57b0cc0df7d2f9f2016"),
            (10, 12948026869183762353, "90c1dbd608e3775069ed0d85efa3ee5de8d81b1967898c92dc4e078220687120"),
            (3, 4577505, "25f0f2b3c3a7ee2320155ec701726bac70102b7913ea2ab94db8dc28bed6a4eb"),
            (5, 310860962933, "327ff10ffaff80e02b02d84cb5d8e2224d26c5955bb5601f5c858c97eedef35b"),
            (12, 6330130600746392734, "57e699bb5ee6477c577b89f0b1e80b6ca35aa36aabc4bccd82fa2c3120ec921d"),
            (10, 12750189356887995490, "0ac875879aec9e4c761b531fe2f7b43c1dbb5bccb959bf107760f91051a6ee58"),
            (15, 10931722883136722, "b465c860684913c52bdb50b0f8391f16d96469c56fadaab652286a6595c36b36"),
            (17, 17524967233299049020, "4fb98d750c37ac63990945f14ed34a1a6b9365002efb2d5065f3f83201054fae"),
            (9, 3571922820466116722, "d04a926d0b5f5620279af27a48c70ded03df9a5216ba9bdc433acb66b71f30ec"),
            (9, 2682281289702676113, "327f595a0c22e75310f4d4f5d6cf6fa68a8180a2a86ef20f43558d6591a1fa8a"),
            (11, 18082672136676787444, "755efbe5fafe6d2d6abcd77db6691d50439dff688a983b31aa1e51f13f788720"),
            (19, 7713303281797564269, "8a73bd1e4fe0b2287b1167958c09be3a3919041bfaf9e3aa0d928b5eea20ad8d"),
            (9, 11282430282381183162, "aeac23962a5b369384bd399221cd1f01811541418b04a60bce353c1547dcb7ce"),
            (11, 117375040596452939, "ce61aa9aa1b75b926a8665183d31c8688a2b2b5a721c4420974082f8104c11c9"),
            (12, 10975145978023288744, "c053ef24eb19cfeab807405bcd0024f8c0e841fb732b408baa87164a2be7caf6"),
            (11, 15383415639973988732, "c483723d14e8691990f8d6676205166e885e803cf582e7940f4e6156437c8c1f"),
            (7, 31751348347462992, "68c212025519e0efb1009cc068490ee34edce510089ae51fbace4902c504f31a"),
            (5, 459729468423, "322888f88a59b930b950650ae9ded400a09859b52ee325d67c4427e11687a871"),
        )
        fingerprint_index = {
            (length, rolling): digest
            for length, rolling, digest in forbidden_fingerprints
        }
        tracked = subprocess.run(
            ["git", "ls-files", "-z"],
            cwd=ROOT,
            check=True,
            capture_output=True,
        ).stdout.split(b"\0")
        for relative in tracked:
            if not relative:
                continue
            path = ROOT / relative.decode()
            try:
                content_bytes = path.read_bytes()
                content = content_bytes.decode("utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            for length in {item[0] for item in forbidden_fingerprints}:
                if len(content_bytes) < length:
                    continue
                power = pow(257, length - 1, 1 << 64)
                rolling = 0
                for value in content_bytes[:length]:
                    rolling = (rolling * 257 + value + 1) & ((1 << 64) - 1)
                for index in range(len(content_bytes) - length + 1):
                    digest = fingerprint_index.get((length, rolling))
                    if digest is not None:
                        candidate = content_bytes[index : index + length]
                        self.assertNotEqual(
                            hashlib.sha256(candidate).hexdigest(),
                            digest,
                            f"{path} contains identified fixture data",
                        )
                    if index + length == len(content_bytes):
                        break
                    rolling = (
                        (rolling - (content_bytes[index] + 1) * power) * 257
                        + content_bytes[index + length]
                        + 1
                    ) & ((1 << 64) - 1)
            for workspace, team_id in re.findall(
                r"https://([a-z0-9-]+)\.slack\.com/docs/(T[A-Z0-9]+)",
                content,
            ):
                self.assertEqual(
                    (workspace, team_id),
                    ("example", "T00000000"),
                    f"{path} contains a non-synthetic Slack workspace identity",
                )

    def test_canvas_access_defaults_to_workspace_anyone_can_view(self) -> None:
        content = self.read_skill()
        script = PUBLISH_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("워크스페이스 공개 채널 구성원 열람", content)
        self.assertIn("participant list remains required metadata", content)
        self.assertIn("public channel based `read` access", content)
        self.assertIn("CANVAS_ACCESS_CHANNEL_IDS", content)
        self.assertIn("Do not pass `channel_ids` and `user_ids` together", content)
        self.assertNotIn("Do NOT grant via a public-channel `channel_ids` share", content)

        self.assertIn("CANVAS_ACCESS_MODE", script)
        self.assertIn('"workspace_anyone"', script)
        self.assertIn("def canvas_document_body", script)
        self.assertIn("Slack Web API title", script)
        self.assertIn('"markdown": canvas_document_body(meeting["canvas_markdown"])', script)
        self.assertIn('"channel_ids": access_channel_ids', script)
        self.assertIn('"user_ids": participant_ids', script)

    def test_slack_list_registration_is_default_publish_contract(self) -> None:
        content = self.read_skill()

        self.assertIn("## Slack List Registration", content)
        self.assertIn("The default published artifact is not complete until both surfaces are done", content)
        self.assertIn("Inspect the existing Slack List convention", content)
        self.assertIn("Create or update one Slack List row with the verified Canvas URL", content)
        self.assertIn("Read back or otherwise verify the row/item ID", content)
        self.assertIn("Slack List 등록 값", content)
        for field in ["- 이름:", "- Date:", "- Topic:", "- Status:", "- Counts:", "- Meeting Canvas:"]:
            self.assertIn(field, content)
        self.assertIn("Do not skip Slack List registration for a published meeting Canvas", content)
        self.assertIn("If List tooling or permission fails, verify the failure", content)
        self.assertIn("manual handoff fallback", content)
        self.assertIn("Do not create an index Canvas as a substitute for the List", content)
        self.assertIn("Do not put an index row preview in the meeting Canvas body by default", content)
        self.assertIn("set the List `Date` to the same value as `Last updated`", content)
        self.assertIn("Use Slack's built-in List `이름` field as the meeting title", content)
        self.assertIn("workspace docs URL", content)
        self.assertIn("`https://{workspace}.slack.com/docs/{team_id}/{canvas_id}`", content)
        self.assertIn("Do not store `https://slack.com/canvas/", content)
        self.assertNotIn("## Meeting Index List Schema", content)
        self.assertNotIn("Render the persistent index as a Slack List by default", content)
        self.assertNotIn("The user manually binds that Canvas into their Slack List", content)

    def test_publish_script_uses_workspace_docs_url_for_list_links(self) -> None:
        script = PUBLISH_SCRIPT.read_text(encoding="utf-8")
        module = self.load_publish_script()

        self.assertIn("build_workspace_docs_canvas_url", script)
        self.assertIn("auth.test", script)
        self.assertIn("validate_list_canvas_url", script)
        self.assertIn("SLACK_DOCS_CANVAS_URL_RE", script)
        self.assertNotIn('f"https://slack.com/canvas/{canvas_id}"', script)

        self.assertEqual(
            module.build_workspace_docs_canvas_url(
                "F00000000",
                {"url": "https://example.slack.com/", "team_id": "T00000000"},
            ),
            "https://example.slack.com/docs/T00000000/F00000000",
        )
        module.validate_list_canvas_url("https://example.slack.com/docs/T00000000/F00000000")
        with self.assertRaises(SystemExit):
            module.validate_list_canvas_url("https://slack.com/canvas/F0BE78HN5P1")

    def test_manual_index_binding_handoff_is_forward_tested(self) -> None:
        self.assertTrue(HANDOFF.exists(), "good manual handoff fixture must exist")
        self.assertTrue(BAD_HANDOFF.exists(), "bad manual handoff fixture must exist")
        self.assertTrue(RUBRIC.exists(), "quality rubric script must exist")

        handoff = HANDOFF.read_text(encoding="utf-8")
        bad_handoff = BAD_HANDOFF.read_text(encoding="utf-8")
        rubric = RUBRIC.read_text(encoding="utf-8")

        for field in [
            "수동 List 바인딩 값",
            "- 이름: 샘플 플랫폼 자동화 및 이벤트 파이프라인 온보딩",
            "- Date: 2026-06-24",
            "- Topic: 온보딩",
            "- Status: Follow-up 필요",
            "- Counts: 결정 7 / 액션 8 / 질문 4",
            "- Meeting Canvas: https://example.slack.com/docs/T00000000/F00000000",
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
        self.assertIn("Top status block UI", content)
        self.assertIn("Web API-safe quote line", content)
        self.assertIn("raw Slack Web API `canvases.create`/`canvases.edit` does not support that callout syntax", content)
        self.assertIn("Progressive disclosure", content)
        self.assertIn("Layer-cake headings", content)
        self.assertIn("Use tables only where comparison helps", content)
        self.assertIn("::: {.callout}", content)
        self.assertIn("## Slack Canvas UI Block Palette", content)
        for block in [
            "Callout/status quote",
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
        self.assertIn("Status block", content)
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
        self.assertIn("|Field|Value|", content)
        self.assertIn("|  ---  |  ---  |", content)
        self.assertIn("| Date | YYYY-MM-DD |", content)
        self.assertIn("### 메타데이터 메모", content)
        self.assertIn("긴 참석자 전체 명단", content)
        self.assertIn("short summary in the table", content)
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

        self.assertIn("top status block", content)
        self.assertIn("scope/status facts", content)
        self.assertIn("Web API Markdown support does not include callout blocks", content)
        self.assertIn("default UI recipe", content)
        self.assertIn("top-level divider before the audit appendix", content)
        self.assertIn("Render `TL;DR` as 2-4 bullets", content)
        self.assertIn("Do not use layout columns for the default meeting minutes body", content)
        self.assertIn("300 cells", content)
        self.assertIn("용어 보정 1", content)
        self.assertIn("Any table over 5 columns is a quality failure", content)
        self.assertIn("Keep the metadata table narrow", content)
        self.assertIn("wide metadata cells", content)
        self.assertIn("Slack Canvas table width follows the longest cell", content)
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
        self.assertIn("register it in the existing Slack List", content)
        self.assertIn("verified row/item ID", content)
        self.assertIn("exactly one `### 원문 전사본 전문` heading", content)
        self.assertIn("Duplicate adjacent transcript headings", content)
        self.assertIn("compact top status block", content)
        self.assertIn("literal `::: {.callout}` is a failed Canvas", content)
        self.assertIn("TL;DR` -> `결정사항` -> `액션 보드` -> `주제별 논의` -> `후속 확인` -> `리스크/열린 질문`", content)
        self.assertIn("decisions use multi-line bullets", content)
        self.assertIn("two-line meeting summary is a failed output", content)
        self.assertIn("A created or read-back meeting Canvas must not retain placeholder cells", content)
        self.assertIn("must not contain Canvas URL placeholders such as `미정`, `{Canvas 링크}`, or `생성 후 인덱스 참조`", content)
        self.assertIn("raw table edits can leave unaddressable empty rows", content)
        self.assertIn("duplicate metadata tables", content)
        self.assertIn("no long participant/source/tracking/access cells", content)
        self.assertIn("`|||` blank table rows", content)
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
        self.assertIn("Sample Platform R and R", content)
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
        self.assertIn("wrong or too wide after publish", content)
        self.assertIn("Slack List row", content)
        self.assertIn("Do not leave the skeleton as the final artifact", content)
        self.assertIn("Invalid text passed", content)
        self.assertIn("use tools/API by default", content)
        self.assertIn("First inspect the existing List convention", content)
        self.assertIn("verify the resulting row/item ID", content)
        self.assertIn("Avoid destructive full-canvas replace", content)

    def test_publish_script_rejects_wide_or_corrupt_metadata_tables(self) -> None:
        script = PUBLISH_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("MAX_METADATA_CELL_WIDTH", script)
        self.assertIn("MAX_METADATA_ROW_WIDTH", script)
        self.assertIn("def display_width", script)
        self.assertIn("def validate_metadata_table_shape", script)
        self.assertIn("Slack blank-table rows such as `|||`", script)
        self.assertIn("malformed metadata table rows left by partial Slack table repair", script)
        self.assertIn("metadata_table_cell_width", script)
        self.assertIn("metadata_table_row_width", script)
        self.assertIn("move detail to `### 메타데이터 메모`", script)


if __name__ == "__main__":
    unittest.main()
