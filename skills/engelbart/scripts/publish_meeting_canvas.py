#!/usr/bin/env python3
"""
publish_meeting_canvas.py

Engelbart 회의 Canvas 발행 규칙을 하나의 원자적 절차로 묶는다:

  1) canvases.create        - 회의록 Canvas 생성 (markdown 본문)
  2) canvases.access.set     - 회의 참여자를 user_ids 로 초대(read)
                               => 참여자만 열람 가능 (워크스페이스 전체 공개 아님)
  3) slackLists.items.create - 회의 인덱스 List에 한 행 자동 등록

의존성 없음(파이썬 표준 라이브러리만 사용). 유료 Slack 플랜 + 아래 scope 필요:
  canvases:write, lists:write, users:read   (이메일 조회 안 쓰므로 users:read.email 불필요)

환경변수:
  SLACK_TOKEN               xoxb-... 또는 xoxp-...  (필수)
  MEETING_LIST_ID           회의 인덱스 List의 list_id (예: F0BDM7J7AV6)  (필수)
  PARTICIPANT_NAMES         회의 참여자 이름, 콤마 구분 (이름 기반 자동 조회)
  PARTICIPANT_USER_IDS      회의 참여자 user_id, 콤마 구분 (직접 지정 시)
  PARTICIPANT_ACCESS_LEVEL  read | write (기본: read)

이름 기반 조회는 users.list 에서 유일 매칭일 때만 자동 부여한다.
동명이인/불일치는 부여를 생략하고 로그로 보고한다(오초대 방지).

컬럼 매핑(List 컬럼명 -> 역할). List 실제 컬럼명에 맞게 조정:
  COL_NAME(이름/title), COL_DATE, COL_PARTICIPANTS, COL_CANVAS_URL
"""

import json
import os
import urllib.request
import urllib.error

SLACK_API = "https://slack.com/api/"


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
    ids = [x.strip() for x in os.environ.get("PARTICIPANT_USER_IDS", "").split(",") if x.strip()]
    report = []
    names = [x.strip() for x in os.environ.get("PARTICIPANT_NAMES", "").split(",") if x.strip()]
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


def build_initial_fields(cols: dict, meta: dict, participant_ids: list) -> list:
    """컬럼 매핑 + 회의 메타데이터 -> initial_fields 배열.

    실측 스키마(List F0BDM7J7AV6) 기준 역할 키: name/date/participants/canvas_url.
    - name        : rich_text 컬럼 (회의 제목)
    - date        : date 컬럼, 값은 ["YYYY-MM-DD"] 문자열 (epoch 아님)
    - participants: user 컬럼, 값은 user_id 배열 (Canvas 권한 부여 목록과 동일)
    - canvas_url  : link 컬럼, 값은 [{"originalUrl": "..."}] (key 는 url 아님)
    """
    fields = []
    if cols.get("name"):
        fields.append(rich_text_value(cols["name"], meta["title"]))
    if cols.get("date") and meta.get("date_str"):
        fields.append({"column_id": cols["date"], "date": [meta["date_str"]]})
    if cols.get("participants") and participant_ids:
        fields.append({"column_id": cols["participants"], "user": participant_ids})
    if cols.get("canvas_url") and meta.get("canvas_url"):
        fields.append({
            "column_id": cols["canvas_url"],
            "link": [{"originalUrl": meta["canvas_url"]}],
        })
    return fields


def main():
    token = os.environ.get("SLACK_TOKEN")
    list_id = os.environ.get("MEETING_LIST_ID")
    if not token or not list_id:
        raise SystemExit("SLACK_TOKEN 과 MEETING_LIST_ID 환경변수가 필요합니다.")

    participant_ids, resolve_report = resolve_participants(token)
    access_level = os.environ.get("PARTICIPANT_ACCESS_LEVEL", "read")
    for name, status, val in resolve_report:
        if status == "resolved":
            print(f"    참석자 '{name}' -> {val} (자동 부여)")
        else:
            print(f"    참석자 '{name}' -> {status}: 자동 부여 생략, 수동 확인 필요 ({val})")

    # --- 회의 메타데이터 (실전에서는 engelbart 산출물에서 채워 넣는다) ---
    meeting = {
        "title": os.environ.get("MEETING_TITLE", "2026-07-01 [확인필요] 제목"),
        "date_str": os.environ.get("MEETING_DATE", "2026-07-01"),  # "YYYY-MM-DD"
        "canvas_markdown": os.environ.get("CANVAS_MARKDOWN", "# 회의록\n\n(본문)"),
    }

    # 1) Canvas 생성 -----------------------------------------------------------
    created = slack_call(
        "canvases.create",
        {
            "title": meeting["title"],
            "document_content": {
                "type": "markdown",
                "markdown": meeting["canvas_markdown"],
            },
        },
        token,
    )
    canvas_id = created["canvas_id"]
    canvas_url = created.get("canvas", {}).get("url") \
        or f"https://slack.com/canvas/{canvas_id}"
    meeting["canvas_url"] = canvas_url
    print(f"[1/3] canvas 생성 완료: {canvas_id}")

    # 2) 회의 참여자만 초대(read) => 참여자 열람, 워크스페이스 전체 공개 아님 ----
    if participant_ids:
        slack_call(
            "canvases.access.set",
            {
                "canvas_id": canvas_id,
                "access_level": access_level,
                "user_ids": participant_ids,  # channel_ids 와 동시 전달 불가
            },
            token,
        )
        print(f"[2/3] 참여자 {len(participant_ids)}명 초대({access_level}): {participant_ids}")
    else:
        print("[2/3] 초대할 참여자 ID/이메일이 없어 접근권한 변경 생략(생성자만 열람).")

    # 3) List 등록 -------------------------------------------------------------
    # 실측 확정 컬럼 ID (List F0BDM7J7AV6). 다른 List 면 COL_* 로 덮어쓴다.
    role_map = {
        "name": os.environ.get("COL_NAME", "Col0BCWJ2LEA0"),          # 제목(rich_text)
        "date": os.environ.get("COL_DATE", "Col0BCLJFH0RH"),          # 회의일(date)
        "participants": os.environ.get("COL_PARTICIPANTS", "Col0BD5G8GDE0"),  # 참석자(user)
        "canvas_url": os.environ.get("COL_CANVAS_URL", "Col0BDM7XNCBS"),      # Canvas 링크(link)
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
    print(f"[3/3] List 등록 완료: item {item.get('item', {}).get('id', '?')}")
    print(f"\n완료. Canvas URL: {canvas_url}")


if __name__ == "__main__":
    main()
