#!/usr/bin/env python3

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import re
import sys
import unicodedata


ROOT = Path(__file__).resolve().parents[1]
TESTDATA = ROOT / "skills" / "engelbart" / "testdata"
SKILL = ROOT / "skills" / "engelbart" / "SKILL.md"
PUBLISH_SCRIPT = ROOT / "skills" / "engelbart" / "scripts" / "publish_meeting_canvas.py"
BASELINE_OUTPUT = TESTDATA / "ai_devops_onboarding_output.baseline.md"
CURRENT_OUTPUT = TESTDATA / "ai_devops_onboarding_output.md"
BAD_CANVAS_READBACK = TESTDATA / "ai_devops_onboarding_output.bad_canvas_readback.md"
CURRENT_HANDOFF = TESTDATA / "ai_devops_onboarding_handoff.md"
BAD_HANDOFF = TESTDATA / "ai_devops_onboarding_handoff.bad.md"
TRANSCRIPT = TESTDATA / "ai_devops_onboarding_transcript.txt"

BASELINE_EXPECTED_MIN = 25
CURRENT_PASS_LINE = 92
MIN_IMPROVEMENT = 10


@dataclass(frozen=True)
class RubricItem:
    name: str
    weight: int
    required: tuple[str, ...]
    source: str = "output"


RUBRIC: tuple[RubricItem, ...] = (
    RubricItem(
        name="canvas_target_and_renderable_structure",
        weight=10,
        required=(
            "#sample-platform-team",
            "회의일 2026-06-24 · 대상 #sample-platform-team · Source synthetic transcript · Status Follow-up 필요",
            "## 메타데이터",
            "## TL;DR",
        ),
    ),
    RubricItem(
        name="date_falls_back_to_last_updated",
        weight=8,
        required=(
            "| Date | 2026-06-24 |",
            "| Last updated | 2026-06-24 |",
            "회의일 2026-06-24",
        ),
    ),
    RubricItem(
        name="decisions_are_specific_and_attributed",
        weight=14,
        required=(
            "팀 역할을 서비스 아키텍처, 자동화 인프라 운영, 에이전트 도구 통합으로 정리한다.",
            "업무는 이슈 기반, 기능 브랜치 기반, agent-assisted spec-driven workflow로 진행한다.",
            "샘플 플랜 Unlimited 대응은 1차적으로 플랫폼담당이 맡는다.",
            "금요일부터 Aurora Sandbox staging migration 준비를 시작한다.",
            "main 변경분을 release/stg로 가져오고 conflict는 백엔드담당이 해결한다.",
        ),
    ),
    RubricItem(
        name="actions_are_executable",
        weight=14,
        required=(
            "신규담당을 샘플 저장소와 #sample-platform-team 채널에 초대한다.",
            "샘플 이벤트 타임시리즈 구축 태스크를 이슈로 구체화한다.",
            "포인트 production 배포를 오전 중 진행한다.",
            "정책 A와 정책 B 변경을 에이전트 문서에 반영한다.",
            "샘플 캐시 서비스 비용과 429 응답을 외부 지원팀에 문의한다.",
        ),
    ),
    RubricItem(
        name="domain_fidelity",
        weight=16,
        required=(
            "Aurora Sandbox staging migration",
            "외부 콘텐츠 소스 A/B",
            "샘플 캐시 서비스",
            "Flask 기반 Python microservice",
            "trainer pod와 serving pod",
            "Harbor Object Store",
            "gRPC Gateway",
            "Nimbus Cache",
        ),
    ),
    RubricItem(
        name="uncertainty_and_open_questions",
        weight=10,
        required=(
            "참석자 실명",
            "확인 필요",
            "외부 콘텐츠 소스 A/B 무료 모델 정책",
            "Sample target architecture",
            "Agent 문서 반영 강제화",
        ),
    ),
    RubricItem(
        name="correction_maps",
        weight=12,
        required=(
            "### 용어 보정",
            "`RNR` -> `R&R`",
            "`대부/대보/대부 업수` -> `플랫폼 자동화`",
            "`gipc 게이트` -> `gRPC Gateway`",
            "### 불확실 단어/문장 보정",
            "### 참석자/화자 보정",
        ),
    ),
    RubricItem(
        name="audit_appendix_preserves_source_shape",
        weight=8,
        required=(
            "### 원문 전사본 전문",
            "```text",
            "진행자 00:00",
            "진행자 08:00",
        ),
    ),
    RubricItem(
        name="skill_instructs_forward_quality_guardrails",
        weight=8,
        source="skill",
        required=(
            "Do not fabricate meeting dates from the transcript",
            "set `Date` to the same value as `Last updated`",
            "실명이 전사본만으로 확정되지 않으면",
            "용어 보정 후보를 먼저 작성",
            "Verbatim full transcript handling",
            "Do not summarize, normalize, translate, or substitute representative blocks",
            "원문 전사본 발췌",
            "Slack List Registration",
            "The default published artifact is not complete until both surfaces are done",
            "Inspect the existing Slack List convention",
            "Create or update one Slack List row with the verified Canvas URL",
            "Do not skip Slack List registration for a published meeting Canvas",
            "Do not create an index Canvas as a substitute for the List",
            "Do not put an index row preview in the meeting Canvas body by default",
            "set the List `Date` to the same value as `Last updated`",
            "Use Slack's built-in List `이름` field as the meeting title",
            "Any table over 5 columns is a quality failure",
            "Canvas UI/UX Principles",
            "Slack Canvas UI Block Palette",
            "Default meeting Canvas UI recipe",
            "Product and service names from local background are preferred",
            "`킹글 스테이징` should resolve to `팅글 staging`",
            "Canvas UI Pattern Examples",
            "Status block",
            "Metadata table",
            "Action checklist",
            "Audit divider",
            "Transcript block",
            "Do not overfit the UI proof Canvas",
            "Canvas Anti-Patterns",
            "Do not use one dense paragraph for TL;DR",
            "Do not use tables for decisions, actions, risks, or corrections by default",
            "Do not put callouts, tables, or code blocks inside layout columns",
            "Do not create a skeleton Canvas and stop there",
            "Do not hide uncertainty in decisions",
            "Top status block UI",
            "Web API-safe quote line",
            "compact top status block",
            "literal `::: {.callout}` is a failed Canvas",
            "2-column vertical table",
            "Checklist bullets",
            "Heading hierarchy",
            "Horizontal rule",
            "Do not use layout columns for the default meeting minutes body",
            "Sanitize the Slack API title",
            "literal `&`",
            "retry once with a sanitized title",
            "Standalone Korean honorific-like words",
            "Standalone Korean honorific-like words may be name misrecognitions",
            "`{name} 프로님` should be treated as a title/honorific",
            "top-level divider before the audit appendix",
            "Render `TL;DR` as 2-4 bullets",
            "decisions use multi-line bullets",
            "Canvas UI recipe",
            "TL;DR` -> `결정사항` -> `액션 보드` -> `주제별 논의` -> `후속 확인` -> `리스크/열린 질문`",
            "two-line meeting summary is a failed output",
            "baseline score",
            "pass line",
            "exactly one `### 원문 전사본 전문` heading",
            "Duplicate adjacent transcript headings",
            "replacement body must not repeat the same heading",
            "The pasted transcript must appear exactly once",
            "marker count or hash",
            "Required Meeting Inputs",
            "Before producing or creating meeting minutes",
            "If either the participant list or transcript is missing, stop and ask for the missing input",
            "Input collection order is sequential",
            "When no participant list is present, ask only for the participant list",
            "Do not ask for the transcript in the same response",
            "After the participant list is provided, confirm the received participants",
            "then ask for the meeting transcript text",
            "When both inputs are present, continue with the normal Engelbart workflow",
            "워크스페이스 공개 채널 구성원 열람",
            "participant list remains required metadata",
            "public channel based `read` access",
            "CANVAS_ACCESS_CHANNEL_IDS",
            "Do not pass `channel_ids` and `user_ids` together",
            "A final meeting Canvas must not be created from fallback placeholder content",
            "A created or read-back meeting Canvas must not retain placeholder cells",
            "register it in the existing Slack List",
            "verified row/item ID",
            "raw table edits can leave unaddressable empty rows",
            "duplicate metadata tables",
            "wide metadata cells",
            "no long participant/source/tracking/access cells",
            "`|||` blank table rows",
            "If local background and meeting role identify a team lead owner",
            "Do not leave team-lead-owned follow-ups as `참석자 1`",
            "Render every risk/open-question item as a titled multi-line bullet",
            "A one-line risk that packs all fields into one sentence is a Slack Canvas readability failure",
        ),
    ),
)


def score_item(item: RubricItem, output: str, skill: str) -> tuple[int, list[str]]:
    haystack = skill if item.source == "skill" else output
    missing = [needle for needle in item.required if needle not in haystack]
    if not missing:
        return item.weight, []
    earned = round(item.weight * (len(item.required) - len(missing)) / len(item.required))
    return earned, missing


def table_cells_under_limit(output: str, limit: int = 300) -> tuple[bool, int]:
    max_cells = 0
    for block in re.split(r"\n(?=## |### )", output):
        rows = [line for line in block.splitlines() if line.startswith("|") and line.endswith("|")]
        if not rows:
            continue
        cells = sum(max(0, line.count("|") - 1) for line in rows)
        max_cells = max(max_cells, cells)
    return max_cells < limit, max_cells


def wide_table_violations(output: str, max_columns: int = 5) -> list[str]:
    violations: list[str] = []
    for line_no, line in enumerate(output.splitlines(), start=1):
        if line.startswith("|") and line.endswith("|"):
            columns = max(0, line.count("|") - 1)
            if columns > max_columns:
                violations.append(f"line {line_no}: {columns} columns")
    return violations


def display_width(text: str) -> int:
    width = 0
    for char in text:
        width += 2 if unicodedata.east_asian_width(char) in {"F", "W"} else 1
    return width


def metadata_table_width_violations(output: str, max_cell_width: int = 56, max_row_width: int = 88) -> list[str]:
    block = section_between(output, "## 메타데이터", "## TL;DR")
    violations: list[str] = []
    if not block:
        return ["metadata section missing"]
    if block.count("| Field | Value |") + block.count("|Field|Value|") != 1:
        violations.append("expected exactly one Field/Value metadata table")
    for line_no, line in enumerate(block.splitlines(), start=1):
        stripped = line.strip()
        if not stripped.startswith("|") or not stripped.endswith("|"):
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if len(cells) != 2:
            violations.append(f"line {line_no}: expected 2 metadata cells, got {len(cells)}")
            continue
        if display_width(stripped) > max_row_width:
            violations.append(f"line {line_no}: metadata row too wide")
        for cell in cells:
            if re.fullmatch(r":?-{3,}:?", cell.replace(" ", "")):
                continue
            if display_width(cell) > max_cell_width:
                violations.append(f"line {line_no}: metadata cell too wide `{cell[:30]}`")
    if "|||" in block or "||Value|" in block or "|Date||" in block:
        violations.append("malformed blank metadata table row remains")
    return violations


def has_substantial_long_meeting_body(output: str) -> bool:
    decision_block = section_between(output, "## 결정사항", "## 액션 보드")
    decision_count = len(re.findall(r"^- \*\*", decision_block, flags=re.MULTILINE))
    action_count = len(re.findall(r"^- \[ \] ", output, flags=re.MULTILINE))
    topic_count = len(re.findall(r"^### ", output, flags=re.MULTILINE))
    return decision_count >= 7 and action_count >= 8 and topic_count >= 8


def section_between(output: str, start_marker: str, end_marker: str) -> str:
    start = output.find(start_marker)
    if start == -1:
        return ""
    end = output.find(end_marker, start + len(start_marker))
    if end == -1:
        return output[start:]
    return output[start:end]


def transcript_section(output: str) -> str:
    marker = "### 원문 전사본 전문"
    start = output.find(marker)
    if start == -1:
        return ""
    return output[start:]


def transcript_heading_count(output: str) -> int:
    return output.count("### 원문 전사본 전문")


def preserves_transcript_verbatim(output: str) -> bool:
    transcript = TRANSCRIPT.read_text(encoding="utf-8").strip()
    section = transcript_section(output)
    return transcript in section


def transcript_body_count(output: str) -> int:
    transcript = TRANSCRIPT.read_text(encoding="utf-8").strip()
    return transcript_section(output).count(transcript)


def section_order_is_readable(output: str) -> bool:
    headings = [
        "## TL;DR",
        "## 결정사항",
        "## 액션 보드",
        "## 주제별 논의",
        "## 후속 확인",
        "## 리스크/열린 질문",
        "## 보정 및 원문 부록",
    ]
    positions = [output.find(heading) for heading in headings]
    return all(position != -1 for position in positions) and positions == sorted(positions)


def decisions_are_multiline(output: str) -> bool:
    decision_block = section_between(output, "## 결정사항", "## 액션 보드")
    return (
        len(re.findall(r"^- \*\*", decision_block, flags=re.MULTILINE)) >= 7
        and "  - 내용:" in decision_block
        and "  - 근거:" in decision_block
        and "  - 영향:" in decision_block
        and "  - 결정자/동의자:" in decision_block
        and "  - 상태:" in decision_block
    )


def tldr_is_scannable(output: str) -> bool:
    tldr_block = section_between(output, "## TL;DR", "## 결정사항")
    bullet_count = len(re.findall(r"^- ", tldr_block, flags=re.MULTILINE))
    return 2 <= bullet_count <= 4


def has_audit_divider(output: str) -> bool:
    appendix = output.find("## 보정 및 원문 부록")
    if appendix == -1:
        return False
    before_appendix = output[:appendix]
    return "\n---\n\n" in before_appendix[-500:]


def has_top_status_block(output: str) -> bool:
    return "::: {.callout}" in output or re.search(r"(?m)^>\s*회의일\s+", output) is not None


def has_metadata_table(output: str) -> bool:
    compact = "|Field|Value|" in output
    spaced = "| Field | Value |" in output
    return compact or spaced


def uses_default_canvas_ui_recipe(output: str) -> bool:
    return (
        has_top_status_block(output)
        and has_metadata_table(output)
        and re.search(r"^- \[ \] ", output, flags=re.MULTILINE) is not None
        and "```text" in output
        and "::: {.layout}" not in output
    )


def canvas_has_index_or_url_placeholder(output: str) -> bool:
    placeholders = ("생성 후 인덱스 참조", "{Canvas 링크}", "{Canvas 링크 또는 미정}")
    return "## 회의록 인덱스 항목" in output or any(placeholder in output for placeholder in placeholders)


def manual_handoff_failures(path: Path) -> list[str]:
    handoff = path.read_text(encoding="utf-8")
    failures: list[str] = []
    required = (
        "수동 List 바인딩 값",
        "- 이름: 샘플 플랫폼 자동화 및 이벤트 파이프라인 온보딩",
        "- Date: 2026-06-24",
        "- Topic: 온보딩",
        "- Status: Follow-up 필요",
        "- Counts: 결정 7 / 액션 8 / 질문 4",
        "- Meeting Canvas: https://example.slack.com/docs/T00000000/F00000000",
    )
    missing = [item for item in required if item not in handoff]
    if missing:
        failures.append(f"manual_handoff_required_fields: missing {missing}")
    if "- Title:" in handoff:
        failures.append("manual_handoff_uses_title_field: use Slack List `이름`, not a separate Title field")
    if re.search(r"(^|\n)\|.*\|", handoff):
        failures.append("manual_handoff_table: handoff must be bullet fields, not a Canvas table")
    placeholder_markers = ("미정", "생성 후 인덱스 참조", "{Canvas URL}", "{Canvas 링크}")
    if any(marker in handoff for marker in placeholder_markers):
        failures.append("manual_handoff_placeholder: handoff must not contain placeholders after Canvas URL exists")
    if "https://example.slack.com/docs/" not in handoff:
        failures.append("manual_handoff_missing_canvas_url: handoff must include the verified Canvas URL")
    return failures


def publish_script_required_input_failures() -> list[str]:
    script = PUBLISH_SCRIPT.read_text(encoding="utf-8")
    failures: list[str] = []
    required_snippets = (
        "def usage()",
        "--help",
        "validate_required_inputs",
        "PARTICIPANT_NAMES 또는 PARTICIPANT_USER_IDS",
        "CANVAS_MARKDOWN 환경변수가 필요합니다",
        "### 원문 전사본 전문",
        "```text",
        "Web API-safe `> 회의일 ...` 상태줄",
        "`::: {.callout}`는 literal로 렌더링됩니다",
        "MAX_METADATA_CELL_WIDTH",
        "MAX_METADATA_ROW_WIDTH",
        "validate_metadata_table_shape",
        "metadata_table_cell_width",
        "metadata_table_row_width",
        "Slack blank-table rows such as `|||`",
        "move detail to `### 메타데이터 메모`",
        "SLACK_DOCS_CANVAS_URL_RE",
        "build_workspace_docs_canvas_url",
        "auth.test",
        "validate_list_canvas_url",
        "`https://{workspace}.slack.com/docs/{team_id}/{canvas_id}`",
    )
    missing = [snippet for snippet in required_snippets if snippet not in script]
    if missing:
        failures.append(f"publish_required_input_contract: missing {missing}")
    if "canvas_markdown\": os.environ.get(\"CANVAS_MARKDOWN\", \"# 회의록\\n\\n(본문)\")" in script:
        failures.append("publish_placeholder_default: script must not create a Canvas from fallback placeholder markdown")
    return failures


def known_team_lead_owner_leaks(output: str) -> bool:
    followup_block = section_between(output, "## 후속 확인", "## 리스크/열린 질문")
    risk_block = section_between(output, "## 리스크/열린 질문", "## 보정 및 원문 부록")
    known_team_lead_topics = ("Sample target architecture", "Agent 문서 반영 강제화")
    for block in (followup_block, risk_block):
        for topic in known_team_lead_topics:
            topic_pos = block.find(topic)
            if topic_pos == -1:
                continue
            next_topic = block.find("\n- **", topic_pos + len(topic))
            topic_text = block[topic_pos:] if next_topic == -1 else block[topic_pos:next_topic]
            if "참석자 1" in topic_text and "백엔드리더 팀리더" not in topic_text:
                return True
    return False


def dense_risk_lines(output: str) -> list[str]:
    risk_block = section_between(output, "## 리스크/열린 질문", "## 보정 및 원문 부록")
    violations: list[str] = []
    for line_no, line in enumerate(risk_block.splitlines(), start=1):
        if not line.startswith("- **"):
            continue
        packed_fields = sum(marker in line for marker in ("확인 담당:", "확인 방법:", "상태:"))
        if packed_fields >= 2:
            violations.append(f"risk line {line_no}: packed {packed_fields} fields")
    return violations


def evaluate(path: Path) -> tuple[int, list[str]]:
    output = path.read_text(encoding="utf-8")
    skill = SKILL.read_text(encoding="utf-8")
    score = 0
    failures: list[str] = []
    for item in RUBRIC:
        earned, missing = score_item(item, output, skill)
        score += earned
        if missing:
            failures.append(f"{item.name}: {earned}/{item.weight}; missing {missing}")
    under_limit, max_cells = table_cells_under_limit(output)
    if not under_limit:
        failures.append(f"canvas_table_cell_limit: max table cells {max_cells} exceeds 299")
        score = max(0, score - 5)
    if path in {CURRENT_OUTPUT, BAD_CANVAS_READBACK}:
        wide_tables = wide_table_violations(output)
        if wide_tables:
            failures.append(f"slack_canvas_wide_tables: {wide_tables}")
            score = max(0, score - 20)
        metadata_width = metadata_table_width_violations(output)
        if metadata_width:
            failures.append(f"slack_canvas_metadata_width: {metadata_width}")
            score = max(0, score - 20)
        if "12-column index schema" in output:
            failures.append("slack_canvas_index_rendering: internal schema leaked into output")
            score = max(0, score - 10)
        if not has_substantial_long_meeting_body(output):
            failures.append("long_meeting_substance: expected at least 7 decisions, 8 actions, and 8 topic/correction sections")
            score = max(0, score - 15)
        if not section_order_is_readable(output):
            failures.append("section_order: expected TL;DR -> 결정사항 -> 액션 보드 -> 주제별 논의 -> 후속 확인 -> 리스크/열린 질문 -> 보정 및 원문 부록")
            score = max(0, score - 15)
        if not decisions_are_multiline(output):
            failures.append("decision_readability: decisions must use multi-line bullets with separated fields")
            score = max(0, score - 15)
        if not tldr_is_scannable(output):
            failures.append("tldr_readability: multi-topic meetings must render TL;DR as 2-4 bullets")
            score = max(0, score - 10)
        if not has_audit_divider(output):
            failures.append("audit_separator: long meeting Canvas must include one top-level divider before 보정 및 원문 부록")
            score = max(0, score - 8)
        if not uses_default_canvas_ui_recipe(output):
            failures.append("canvas_ui_recipe: expected top status block, vertical metadata table, checklist actions, transcript code block, and no default layout columns")
            score = max(0, score - 12)
        if canvas_has_index_or_url_placeholder(output):
            failures.append("meeting_canvas_placeholder: Canvas body must not include index-row sections or Canvas URL placeholders after creation")
            score = max(0, score - 12)
        if known_team_lead_owner_leaks(output):
            failures.append("known_role_owner_mapping: use 백엔드리더 팀리더 for team-lead-owned follow-ups/risks when local background and meeting role establish it")
            score = max(0, score - 12)
        risk_violations = dense_risk_lines(output)
        if risk_violations:
            failures.append(f"risk_readability: risks/open questions must use titled multi-line bullets, not dense one-line records: {risk_violations}")
            score = max(0, score - 10)
        if not preserves_transcript_verbatim(output):
            failures.append("transcript_verbatim: 원문 전사본 전문 must contain the input transcript text verbatim, not representative timestamped notes")
            score = max(0, score - 20)
        if transcript_body_count(output) != 1:
            failures.append("transcript_body_count: 원문 전사본 전문 must contain the input transcript body exactly once")
            score = max(0, score - 20)
        if transcript_heading_count(output) != 1:
            failures.append("transcript_heading_count: expected exactly one ### 원문 전사본 전문 heading")
            score = max(0, score - 15)
        source_section = transcript_section(output)
        summary_markers = ("...", "대표 timestamped notes", "대표 블록", "대표 발췌")
        if any(marker in source_section for marker in summary_markers):
            failures.append("transcript_summary_leak: 원문 전사본 전문 contains summary/representative markers")
            score = max(0, score - 10)
    return score, failures


def main() -> int:
    baseline_score, baseline_failures = evaluate(BASELINE_OUTPUT)
    current_score, current_failures = evaluate(CURRENT_OUTPUT)
    bad_score, bad_failures = evaluate(BAD_CANVAS_READBACK)
    handoff_failures = manual_handoff_failures(CURRENT_HANDOFF)
    bad_handoff_failures = manual_handoff_failures(BAD_HANDOFF)
    publish_failures = publish_script_required_input_failures()

    print(f"baseline_score={baseline_score} expected_min={BASELINE_EXPECTED_MIN}")
    print(f"current_score={current_score} pass_line={CURRENT_PASS_LINE}")
    print(f"bad_canvas_readback_score={bad_score} must_fail_below={CURRENT_PASS_LINE}")
    print(f"manual_handoff_failures={len(handoff_failures)}")
    print(f"bad_manual_handoff_failures={len(bad_handoff_failures)}")
    print(f"publish_required_input_failures={len(publish_failures)}")
    print(f"improvement={current_score - baseline_score} min_improvement={MIN_IMPROVEMENT}")
    if baseline_failures:
        print("baseline_failures:")
        for failure in baseline_failures:
            print(f"- {failure}")
    if current_failures:
        print("current_failures:")
        for failure in current_failures:
            print(f"- {failure}")
    if bad_failures:
        print("bad_canvas_readback_failures:")
        for failure in bad_failures:
            print(f"- {failure}")
    if handoff_failures:
        print("manual_handoff_failures:")
        for failure in handoff_failures:
            print(f"- {failure}")
    if bad_handoff_failures:
        print("bad_manual_handoff_failures:")
        for failure in bad_handoff_failures:
            print(f"- {failure}")
    if publish_failures:
        print("publish_required_input_failures:")
        for failure in publish_failures:
            print(f"- {failure}")

    if baseline_score < BASELINE_EXPECTED_MIN:
        print("error: baseline fixture drifted below expected baseline minimum", file=sys.stderr)
        return 1
    expected_bad_failure_names = (
        "meeting_canvas_placeholder",
        "known_role_owner_mapping",
        "risk_readability",
    )
    missing_bad_failures = [
        name for name in expected_bad_failure_names if not any(failure.startswith(name) for failure in bad_failures)
    ]
    if missing_bad_failures:
        print(f"error: bad Canvas fixture did not trigger expected failures: {missing_bad_failures}", file=sys.stderr)
        return 1
    if bad_score >= CURRENT_PASS_LINE:
        print("error: bad Canvas fixture unexpectedly meets the pass line", file=sys.stderr)
        return 1
    if handoff_failures:
        print("error: current manual handoff fixture failed quality checks", file=sys.stderr)
        return 1
    if publish_failures:
        print("error: publish script does not enforce required meeting inputs", file=sys.stderr)
        return 1
    expected_bad_handoff_failure_names = (
        "manual_handoff_required_fields",
        "manual_handoff_uses_title_field",
        "manual_handoff_placeholder",
        "manual_handoff_missing_canvas_url",
    )
    missing_bad_handoff_failures = [
        name
        for name in expected_bad_handoff_failure_names
        if not any(failure.startswith(name) for failure in bad_handoff_failures)
    ]
    if missing_bad_handoff_failures:
        print(f"error: bad handoff fixture did not trigger expected failures: {missing_bad_handoff_failures}", file=sys.stderr)
        return 1
    if current_score < CURRENT_PASS_LINE:
        print("error: current fixture is below current pass line", file=sys.stderr)
        return 1
    if current_score - baseline_score < MIN_IMPROVEMENT:
        print("error: current fixture did not improve enough over baseline", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
