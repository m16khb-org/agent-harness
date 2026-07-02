#!/usr/bin/env python3
"""
publish_meeting_canvas.py

Engelbart 회의 Canvas 발행 규칙을 하나의 원자적 절차로 묶는다:

  1) canvases.create        - 회의록 Canvas 생성 (markdown 본문)
  2) canvases.access.set     - Bubbletap 누구나 볼 수 있음 정책:
                               public channel_ids 로 read 권한 부여
  3) slackLists.items.list   - 기존 회의 인덱스 List 컨벤션 확인
  4) slackLists.items.create - 회의 인덱스 List에 한 행 자동 등록

의존성 없음(파이썬 표준 라이브러리만 사용). 유료 Slack 플랜 + 아래 scope 필요:
  canvases:write, lists:write
  restricted participant-only 모드에서 이름 조회 시 users:read

환경변수:
  SLACK_TOKEN               xoxb-... 또는 xoxp-...  (필수)
  MEETING_LIST_ID           회의 인덱스 List의 list_id (예: F0BDM7J7AV6)  (필수)
  PARTICIPANT_NAMES         회의 참여자 이름, 콤마 구분 (이름 기반 자동 조회)
  PARTICIPANT_USER_IDS      회의 참여자 user_id, 콤마 구분 (직접 지정 시)
  CANVAS_ACCESS_MODE        bubbletap_anyone | channel | participants (기본: bubbletap_anyone)
  CANVAS_ACCESS_CHANNEL_IDS Bubbletap 누구나 볼 수 있음 대상 public channel_id, 콤마 구분
  PARTICIPANT_ACCESS_LEVEL  read | write (기본: read)

기본 access grant는 channel_ids 를 사용한다. Slack API는 channel_ids 와 user_ids 를
한 호출에 함께 받지 않으므로 섞지 않는다. participants 모드의 이름 기반 조회는
users.list 에서 유일 매칭일 때만 자동 부여한다.

Slack List의 meeting_canvas 링크는 `https://{workspace}.slack.com/docs/{team_id}/{canvas_id}`
형식의 workspace docs URL만 사용한다. 범용 `https://slack.com/canvas/{canvas_id}`
fallback은 기존 List 컨벤션과 다르므로 저장하지 않는다.

컬럼 매핑(List 컬럼명 -> 역할). List 실제 컬럼명에 맞게 조정:
  COL_NAME(이름/title), COL_DATE, COL_PARTICIPANTS, COL_CANVAS_URL
"""

import json
import os
import re
import sys
import unicodedata
import urllib.request
import urllib.error

SLACK_API = "https://slack.com/api/"
MAX_METADATA_CELL_WIDTH = 56
MAX_METADATA_ROW_WIDTH = 88
SLACK_DOCS_CANVAS_URL_RE = re.compile(r"^https://[^/\s]+\.slack\.com/docs/T[A-Z0-9]+/F[A-Z0-9]+$")


def usage() -> str:
    return """usage: publish_meeting_canvas.py [--help]

필수 환경변수:
  SLACK_TOKEN
  MEETING_LIST_ID
  CANVAS_MARKDOWN    완성된 회의록 Markdown. Web API-safe `> 회의일 ...` 상태줄,
                     `### 원문 전사본 전문`, ```text 코드블록 포함.
  PARTICIPANT_NAMES 또는 PARTICIPANT_USER_IDS
  CANVAS_ACCESS_CHANNEL_IDS  기본 CANVAS_ACCESS_MODE=bubbletap_anyone 에서 필요

선택 환경변수:
  MEETING_TITLE, MEETING_DATE, CANVAS_ACCESS_MODE, PARTICIPANT_ACCESS_LEVEL,
  COL_NAME, COL_DATE, COL_PARTICIPANTS, COL_CANVAS_URL
"""


def split_env_list(name: str) -> list[str]:
    return [x.strip() for x in os.environ.get(name, "").split(",") if x.strip()]


def canvas_access_mode() -> str:
    mode = os.environ.get("CANVAS_ACCESS_MODE", "bubbletap_anyone").strip()
    return mode or "bubbletap_anyone"


def display_width(text: str) -> int:
    width = 0
    for char in text:
        width += 2 if unicodedata.east_asian_width(char) in {"F", "W"} else 1
    return width


def table_cells(line: str) -> list[str]:
    stripped = line.strip()
    if not stripped.startswith("|") or not stripped.endswith("|"):
        return []
    return [cell.strip() for cell in stripped.strip("|").split("|")]


def markdown_section(markdown: str, heading: str) -> str:
    match = re.search(rf"(?m)^{re.escape(heading)}\s*$", markdown)
    if not match:
        return ""
    next_heading = re.search(r"(?m)^##\s+", markdown[match.end():])
    end = match.end() + next_heading.start() if next_heading else len(markdown)
    return markdown[match.end():end]


def validate_metadata_table_shape(markdown: str) -> list[str]:
    """Prevent Slack Canvas metadata tables from rendering as wide or corrupted tables."""
    failures: list[str] = []
    if re.search(r"(?m)^\|\|\|", markdown):
        failures.append("CANVAS_MARKDOWN must not contain Slack blank-table rows such as `|||`; recreate from clean source markdown")
    if re.search(r"(?m)^\|\|Value\|", markdown) or re.search(r"(?m)^\|Date\|\|", markdown):
        failures.append("CANVAS_MARKDOWN must not contain malformed metadata table rows left by partial Slack table repair")

    metadata_heading_count = len(re.findall(r"(?m)^## 메타데이터\s*$", markdown))
    if metadata_heading_count != 1:
        failures.append("CANVAS_MARKDOWN must contain exactly one `## 메타데이터` section")

    header_count = len(re.findall(r"(?m)^\|\s*Field\s*\|\s*Value\s*\|$", markdown))
    if header_count != 1:
        failures.append("CANVAS_MARKDOWN must contain exactly one compact `|Field|Value|` metadata table")

    metadata = markdown_section(markdown, "## 메타데이터")
    lines = [line for line in metadata.splitlines() if line.strip()]
    table_lines = [line for line in lines if line.strip().startswith("|")]
    if not table_lines:
        failures.append("CANVAS_MARKDOWN `## 메타데이터` must start with a compact 2-column table")
        return failures

    for line in table_lines:
        cells = table_cells(line)
        if len(cells) != 2:
            failures.append(f"metadata_table_columns: expected 2 cells, got {len(cells)} in `{line}`")
            continue
        if display_width(line) > MAX_METADATA_ROW_WIDTH:
            failures.append(
                "metadata_table_row_width: keep metadata rows short; move detail to `### 메타데이터 메모`"
            )
        for cell in cells:
            if re.fullmatch(r":?-{3,}:?", cell.replace(" ", "")):
                continue
            if display_width(cell) > MAX_METADATA_CELL_WIDTH:
                failures.append(
                    f"metadata_table_cell_width: `{cell[:30]}` is too long for Slack Canvas metadata; use a short summary and move detail to `### 메타데이터 메모`"
                )
    return failures


def validate_required_inputs() -> None:
    """Slack 쓰기 전에 참석자 목록과 원문 전사본 입력을 강제한다."""
    missing = []
    if not (split_env_list("PARTICIPANT_NAMES") or split_env_list("PARTICIPANT_USER_IDS")):
        missing.append("PARTICIPANT_NAMES 또는 PARTICIPANT_USER_IDS")

    canvas_markdown = os.environ.get("CANVAS_MARKDOWN", "").strip()
    if not canvas_markdown:
        missing.append("CANVAS_MARKDOWN 환경변수가 필요합니다")
    else:
        if "### 원문 전사본 전문" not in canvas_markdown:
            missing.append("CANVAS_MARKDOWN 안에 ### 원문 전사본 전문 섹션")
        if "```text" not in canvas_markdown:
            missing.append("CANVAS_MARKDOWN 안에 원문 전사본 ```text 코드블록")
        if "::: {.callout}" in canvas_markdown:
            missing.append("CANVAS_MARKDOWN은 Web API-safe `> 회의일 ...` 상태줄을 사용해야 하며 `::: {.callout}`는 literal로 렌더링됩니다")
        if not re.search(r"(?m)^>\s*회의일\s+", canvas_markdown):
            missing.append("CANVAS_MARKDOWN 안에 Web API-safe `> 회의일 ...` 상태줄")
        missing.extend(validate_metadata_table_shape(canvas_markdown))

    mode = canvas_access_mode()
    if mode not in {"bubbletap_anyone", "channel", "participants"}:
        missing.append("CANVAS_ACCESS_MODE 값은 bubbletap_anyone, channel, participants 중 하나")
    if mode in {"bubbletap_anyone", "channel"} and not split_env_list("CANVAS_ACCESS_CHANNEL_IDS"):
        missing.append("CANVAS_ACCESS_CHANNEL_IDS")

    if missing:
        raise SystemExit("필수 회의 입력이 없습니다: " + ", ".join(missing))


def slack_call(method: str, payload: dict, token: str) -> dict:
    """Slack Web API 호출 (application/json)."""
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        SLACK_API + method,
        data=data,
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json; charset=utf-8",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        raise SystemExit(f"[{method}] HTTP {e.code}: {e.read().decode('utf-8')}")
    if not body.get("ok"):
        raise SystemExit(f"[{method}] Slack error: {body.get('error')} | {body}")
    return body


def build_workspace_docs_canvas_url(canvas_id: str, auth: dict) -> str:
    workspace_url = (auth.get("url") or "").strip()
    team_id = (auth.get("team_id") or "").strip()
    if not workspace_url or not team_id:
        raise SystemExit("auth.test 결과에 workspace url/team_id가 없어 Slack List용 Canvas docs URL을 만들 수 없습니다.")
    return f"{workspace_url.rstrip('/')}/docs/{team_id}/{canvas_id}"


def validate_list_canvas_url(canvas_url: str) -> None:
    if SLACK_DOCS_CANVAS_URL_RE.match(canvas_url):
        return
    raise SystemExit(
        "Slack List `meeting_canvas` 링크는 workspace docs URL이어야 합니다: "
        "`https://{workspace}.slack.com/docs/{team_id}/{canvas_id}`. "
        f"잘못된 URL: {canvas_url}"
    )


def rich_text_value(column_id: str, text: str) -> dict:
    """텍스트 컬럼은 rich_text 블록으로 보내야 한다(Slack 요구사항)."""
    return {
        "column_id": column_id,
        "rich_text": [
            {
                "type": "rich_text",
                "elements": [
                    {
                        "type": "rich_text_section",
                        "elements": [{"type": "text", "text": text}],
                    }
                ],
            }
        ],
    }


def _norm(s: str) -> str:
    """이름 비교용 정규화: 소문자 + 공백 제거 + 흔한 경칭 제거."""
    s = (s or "").lower().replace(" ", "")
    for suffix in ("님", "프로", "매니저", "팀장", "팀리더", "리더"):
        if s.endswith(suffix):
            s = s[: -len(suffix)]
    return s


def fetch_workspace_users(token: str) -> list:
    """users.list 페이지네이션으로 활성 인간 사용자만 수집."""
    users, cursor = [], ""
    while True:
        body = slack_call("users.list", {"limit": 200, "cursor": cursor}, token)
        for u in body.get("members", []):
            if u.get("deleted") or u.get("is_bot") or u.get("id") == "USLACKBOT":
                continue
            users.append(u)
        cursor = body.get("response_metadata", {}).get("next_cursor", "")
        if not cursor:
            return users


def resolve_by_name(name: str, users: list) -> list:
    """이름 -> 매칭되는 user_id 목록. real_name/display_name 를 정규화 비교.

    반환 길이 1 == 유일 매칭(자동 부여 대상), 0 == 없음, 2+ == 동명이인.
    """
    target = _norm(name)
    matched = []
    for u in users:
        p = u.get("profile", {})
        candidates = {
            _norm(u.get("real_name")),
            _norm(u.get("name")),
            _norm(p.get("real_name")),
            _norm(p.get("display_name")),
            _norm(p.get("real_name_normalized")),
            _norm(p.get("display_name_normalized")),
        }
        candidates.discard("")
        if target in candidates:
            matched.append(u["id"])
    return list(dict.fromkeys(matched))  # 중복 제거, 순서 유지


def resolve_participants(token: str):
    """회의 참여자를 user_id 로 해석한다. (resolved_ids, report) 반환.

    우선순위: PARTICIPANT_USER_IDS(직접) > PARTICIPANT_NAMES(이름 조회).
    이름 기반은 유일 매칭만 자동 부여하고, 없음/동명이인은 report 에 남겨
    추측 부여로 회의록이 엉뚱한 사람에게 노출되는 것을 막는다.
    """
    ids = split_env_list("PARTICIPANT_USER_IDS")
    report = []
    names = split_env_list("PARTICIPANT_NAMES")
    if names:
        users = fetch_workspace_users(token)
        for name in names:
            hits = resolve_by_name(name, users)
            if len(hits) == 1:
                ids.append(hits[0])
                report.append((name, "resolved", hits[0]))
            elif not hits:
                report.append((name, "no_match", None))
            else:
                report.append((name, f"ambiguous({len(hits)})", hits))
    return list(dict.fromkeys(ids)), report


def short_sample(value) -> str:
    text = json.dumps(value, ensure_ascii=False) if not isinstance(value, str) else value
    return text if len(text) <= 90 else text[:87] + "..."


def field_value_sample(field: dict) -> str:
    for key in ("value", "rich_text", "date", "user", "link"):
        if key in field:
            return short_sample(field[key])
    return short_sample(field)


def inspect_existing_list_convention(token: str, list_id: str) -> dict:
    """최근 List row를 읽어 실제 field key/column_id 컨벤션을 출력한다."""
    body = slack_call("slackLists.items.list", {"list_id": list_id, "limit": 3}, token)
    items = body.get("items", [])
    if not items:
        print("[3/4] List 컨벤션 확인: 기존 row 없음. 기본/환경변수 컬럼 매핑 사용")
        return {}

    print(f"[3/4] List 컨벤션 확인: 최근 row {len(items)}개")
    inferred = {}
    for item in items:
        print(f"    item {item.get('id', '?')}")
        for field in item.get("fields", []):
            key = field.get("key") or ""
            column_id = field.get("column_id") or ""
            sample = field_value_sample(field)
            print(f"      key={key or '-'} column_id={column_id or '-'} sample={sample}")

            if key == "name" and column_id:
                inferred.setdefault("name", column_id)
            elif key == "meeting_canvas" and column_id:
                inferred.setdefault("canvas_url", column_id)
            elif column_id and "date" in field:
                inferred.setdefault("date", column_id)
            elif column_id and "user" in field:
                inferred.setdefault("participants", column_id)
    return inferred


def default_list_title(canvas_title: str) -> str:
    """Canvas title에서 날짜 prefix를 제거해 Slack List `이름` 컨벤션에 맞춘다."""
    return re.sub(r"^\d{4}-\d{2}-\d{2}\s+", "", canvas_title).strip() or canvas_title


def canvas_document_body(markdown: str) -> str:
    """Slack Web API title과 중복되지 않도록 본문 첫 H1 제목을 제거한다."""
    lines = markdown.splitlines()
    if lines and lines[0].startswith("# "):
        lines = lines[1:]
        while lines and lines[0] == "":
            lines = lines[1:]
    return "\n".join(lines).strip()


def build_initial_fields(cols: dict, meta: dict, participant_ids: list) -> list:
    """컬럼 매핑 + 회의 메타데이터 -> initial_fields 배열.

    실측 스키마(List F0BDM7J7AV6) 기준 역할 키: name/date/participants/canvas_url.
    - name        : rich_text 컬럼 (List 제목, 날짜 prefix 없음: `[Topic] 제목`)
    - date        : date 컬럼, 값은 ["YYYY-MM-DD"] 문자열 (epoch 아님)
    - participants: user 컬럼, 값은 user_id 배열 (회의 참석자 메타데이터용;
                    기본 Canvas 권한 부여는 CANVAS_ACCESS_CHANNEL_IDS 를 사용)
    - canvas_url  : link 컬럼, 생성 payload 값은 [{"original_url": "..."}]
                    (readback 은 originalUrl 로 정규화될 수 있음)
    """
    fields = []
    if cols.get("name"):
        fields.append(rich_text_value(cols["name"], meta.get("list_title") or meta["title"]))
    if cols.get("date") and meta.get("date_str"):
        fields.append({"column_id": cols["date"], "date": [meta["date_str"]]})
    if cols.get("participants") and participant_ids:
        fields.append({"column_id": cols["participants"], "user": participant_ids})
    if cols.get("canvas_url") and meta.get("canvas_url"):
        validate_list_canvas_url(meta["canvas_url"])
        fields.append({
            "column_id": cols["canvas_url"],
            # slackLists.items.create validates snake_case, while readback may
            # return camelCase originalUrl for the same stored link field.
            "link": [{"original_url": meta["canvas_url"]}],
        })
    return fields


def main(argv: list[str] | None = None):
    argv = sys.argv[1:] if argv is None else argv
    if any(arg in {"-h", "--help"} for arg in argv):
        print(usage())
        return
    if argv:
        raise SystemExit("지원하지 않는 인자입니다. --help 를 확인하세요: " + " ".join(argv))

    validate_required_inputs()

    token = os.environ.get("SLACK_TOKEN")
    list_id = os.environ.get("MEETING_LIST_ID")
    if not token or not list_id:
        raise SystemExit("SLACK_TOKEN 과 MEETING_LIST_ID 환경변수가 필요합니다.")

    access_mode = canvas_access_mode()
    access_channel_ids = split_env_list("CANVAS_ACCESS_CHANNEL_IDS")
    participant_ids = split_env_list("PARTICIPANT_USER_IDS")
    resolve_report = []
    if access_mode == "participants":
        participant_ids, resolve_report = resolve_participants(token)
    elif split_env_list("PARTICIPANT_NAMES"):
        resolve_report = [
            (name, "metadata_only", None)
            for name in split_env_list("PARTICIPANT_NAMES")
        ]
    access_level = os.environ.get("PARTICIPANT_ACCESS_LEVEL", "read")
    for name, status, val in resolve_report:
        if status == "resolved":
            print(f"    참석자 '{name}' -> {val} (자동 부여)")
        elif status == "metadata_only":
            print(f"    참석자 '{name}' -> metadata_only: 기본 Bubbletap 공유는 channel_ids 사용")
        else:
            print(f"    참석자 '{name}' -> {status}: 자동 부여 생략, 수동 확인 필요 ({val})")
    if access_mode == "participants" and not participant_ids:
        raise SystemExit("해석된 참석자 user_id가 없습니다. PARTICIPANT_USER_IDS를 지정하거나 PARTICIPANT_NAMES를 확인하세요.")

    # --- 회의 메타데이터 (실전에서는 engelbart 산출물에서 채워 넣는다) ---
    meeting = {
        "title": os.environ.get("MEETING_TITLE", "2026-07-01 [확인필요] 제목"),
        "date_str": os.environ.get("MEETING_DATE", "2026-07-01"),  # "YYYY-MM-DD"
        "canvas_markdown": os.environ["CANVAS_MARKDOWN"],
    }
    meeting["list_title"] = os.environ.get("MEETING_LIST_TITLE") or default_list_title(meeting["title"])

    # 1) Canvas 생성 -----------------------------------------------------------
    created = slack_call(
        "canvases.create",
        {
            "title": meeting["title"],
            "document_content": {
                "type": "markdown",
                "markdown": canvas_document_body(meeting["canvas_markdown"]),
            },
        },
        token,
    )
    canvas_id = created["canvas_id"]
    auth = slack_call("auth.test", {}, token)
    canvas_url = build_workspace_docs_canvas_url(canvas_id, auth)
    meeting["canvas_url"] = canvas_url
    print(f"[1/4] canvas 생성 완료: {canvas_id}")

    # 2) Bubbletap 누구나 볼 수 있음: public channel_ids 로 read 부여 ---------
    if access_mode in {"bubbletap_anyone", "channel"}:
        slack_call(
            "canvases.access.set",
            {
                "canvas_id": canvas_id,
                "access_level": access_level,
                "channel_ids": access_channel_ids,
            },
            token,
        )
        print(f"[2/4] Bubbletap 누구나 볼 수 있음 공유({access_level}): {access_channel_ids}")
    elif participant_ids:
        slack_call(
            "canvases.access.set",
            {
                "canvas_id": canvas_id,
                "access_level": access_level,
                "user_ids": participant_ids,  # channel_ids 와 동시 전달 불가
            },
            token,
        )
        print(f"[2/4] 제한 공유 참여자 {len(participant_ids)}명 초대({access_level}): {participant_ids}")
    else:
        print("[2/4] 초대할 참여자 ID/이메일이 없어 접근권한 변경 생략(생성자만 열람).")

    # 3) List 등록 -------------------------------------------------------------
    inferred_cols = inspect_existing_list_convention(token, list_id)
    # 실측 확정 컬럼 ID (List F0BDM7J7AV6). 다른 List 면 관측값 또는 COL_* 로 덮어쓴다.
    role_map = {
        "name": os.environ.get("COL_NAME") or inferred_cols.get("name") or "Col0BCWJ2LEA0",
        "date": os.environ.get("COL_DATE") or inferred_cols.get("date") or "Col0BCLJFH0RH",
        "participants": os.environ.get("COL_PARTICIPANTS")
        or inferred_cols.get("participants")
        or "Col0BD5G8GDE0",
        "canvas_url": os.environ.get("COL_CANVAS_URL")
        or inferred_cols.get("canvas_url")
        or "Col0BDM7XNCBS",
    }
    role_map = {k: v for k, v in role_map.items() if v}

    fields = build_initial_fields(role_map, meeting, participant_ids)
    if not fields:
        raise SystemExit("컬럼 매핑을 확정하지 못했습니다. COL_* 환경변수로 지정하세요.")

    item = slack_call(
        "slackLists.items.create",
        {"list_id": list_id, "initial_fields": fields},
        token,
    )
    print(f"[4/4] List 등록 완료: item {item.get('item', {}).get('id', '?')}")
    print(f"\n완료. Canvas URL: {canvas_url}")


if __name__ == "__main__":
    main()
