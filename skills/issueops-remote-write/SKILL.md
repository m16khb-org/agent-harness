---
name: issueops-remote-write
description: Apply the shared IssueOps remote write protocol to any governed "issueops remote ..." mutation. Polish the Korean body with fluent-korean, run the bundled Korean artifact gate, preview, confirm the identical request, read the artifact back, and reconcile instead of retrying when the result is ambiguous. Use before creating or editing issues, child tasks, PR or MR bodies, comments, or review replies inside an IssueOps cycle, or when the user says "원격에 써줘", "이슈 본문 올려줘", "PR 본문 갱신".
---

# IssueOps Remote Write

이 스킬의 일은 **원격에 무언가를 쓰기 전후에 지켜야 하는 절차 하나**다. 무엇을
쓸지는 각 단계 스킬이 정하고, 어떻게 안전하게 쓰는지는 여기가 소유한다.

- 이슈 생성: [`issueops-create-issue`](../issueops-create-issue/SKILL.md)
- PR/MR 생성: [`issueops-create-pr`](../issueops-create-pr/SKILL.md)
- 본문 동기화: [`issueops-sync-issue`](../issueops-sync-issue/SKILL.md), [`issueops-sync-pr`](../issueops-sync-pr/SKILL.md)
- 한국어 문체: [`fluent-korean`](../fluent-korean/SKILL.md)

## 여덟 규칙

1. **본문은 `fluent-korean`을 거친다.** 초안을 그대로 원격에 쓰지 않는다. 이 호출은
   권고가 아니라 write의 선행 조건이다.
2. **한국어 게이트를 통과한다.** 번들된 스크립트가 한글 비율을 판정한다. 실패하면
   원격을 건드리지 말고 다시 쓴다.
3. **preview를 먼저 실행한다.** `--confirm` 없는 같은 명령이 무엇이 쓰일지 보여 준다.
4. **confirm은 preview와 완전히 같은 요청에만 붙인다.** 본문 한 글자라도 다르면 그것은
   사용자가 승인한 요청이 아니다.
5. **쓴 뒤 읽는다.** provider가 조용히 무시한 write를 성공으로 보고하지 않으려면
   readback이 필요하다.
6. **결과가 불명확하면 재호출하지 않고 reconcile한다.** timeout이나 전송 실패는 "쓰이지
   않았다"는 뜻이 아니다. 다시 만들면 중복 아티팩트가 생긴다.
7. **secret 원문을 남기지 않는다.** 로그·명령 출력·환경값을 붙일 때는 토큰과 자격
   증명을 지운 뒤 붙인다. 원격 본문은 지워도 이력에 남는다.
8. **label과 concrete assignee 없이 쓰지 않는다.** 스코어 결과의 threshold 이상 label만
   적용하고, assignee는 실제 사용자 이름이어야 한다. `@me`는 사용자 이름이 아니다.

## 절차

```bash
# 1. 본문 초안에 fluent-korean 스킬을 호출해 다듬는다(Skill 도구).
# 2. 한국어 게이트. $SKILL_DIR는 설치된 이 스킬 디렉터리(예: ~/.claude/skills/issueops-remote-write,
#    ~/.codex/skills/issueops-remote-write)다. 대상 repo에는 이 경로가 없으므로 repo-relative 경로는 실패한다.
python3 "$SKILL_DIR/scripts/remote_artifact_gate.py" --kind issue --title "$TITLE" --body-file "$BODY_FILE"

# 3. preview (confirm 없음)
issueops remote <verb> --id "$ISSUEOPS_ID" ... --body-file "$BODY_FILE" --json

# 4. 동일 요청 + --confirm
issueops remote <verb> --id "$ISSUEOPS_ID" ... --body-file "$BODY_FILE" --confirm --json

# 5. readback
issueops remote verify-artifact --id "$ISSUEOPS_ID" --provider "$PROVIDER" \
  --kind pr --url "$URL" --target-branch "$BASE" --label "$LABEL" --assignee "$ASSIGNEE" --json   # PR/MR
# issue·child·본문 갱신은 해당 명령의 응답 readback 필드(hierarchy_verified, url, body sha)를 확인한다.

# 6. 결과가 불명확하면 재호출하지 않는다.
issueops remote reconcile-issue --id "$ISSUEOPS_ID" --json                          # issue create
issueops execution reconcile --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json      # PR/MR create
```

provider CLI(`gh`, `glab`)는 어댑터 내부 구현이다. 이 절차는 provider CLI를 직접
실행하지 않는다. 직접 실행하면 pending intent가 기록되지 않아 모호한 결과를 회복할
방법이 사라진다.

## 본문 품질

한국어 게이트는 한글 비율만 검사한다. 게이트를 통과한 본문에도 AI가 쓴 티는 그대로
남는다. 다음 일곱 가지를 게이트 **전에** 확인한다. 게이트는 최종 방어선이고, 자연스러운
본문은 그 앞에서 만들어진다.

1. 명사구나 목록 항목을 제외하면 완성된 문장으로 쓴다. 서술어와 종결어미를 생략하지
   않는다.
2. 단정 회피 어미를 쓰지 않는다. 불확실성이 실제로 있으면 근거를 함께 쓴다.
   - 나쁜 예: `해당 변경이 호환성 문제를 일으키지 않을 것으로 보여집니다.`
   - 좋은 예: `기존 클라이언트는 응답 필드를 추가로 무시하므로 호환됩니다.`
3. 서론 선언으로 시작하지 않는다. 첫 문장이 문제 정의나 요구 사항이다.
   - 나쁜 예: `이 이슈에서는 배포 실패 문제를 다루고자 합니다.`
   - 좋은 예: `v2.3 배포가 시크릿 누락으로 실패합니다.`
4. 수락 기준은 검증 가능한 동작으로 쓴다. "적절히 처리된다" 같은 판단 불가능한 문구를
   쓰지 않는다.
   - 나쁜 예: `잘못된 입력이 적절하게 처리됩니다.`
   - 좋은 예: `만료된 토큰으로 요청하면 401과 재발급 안내를 반환합니다.`
5. 정보가 없는 수식어(`효율적인`, `전반적인`, `개선된`) 대신 무엇이 어떻게 바뀌는지 쓴다.
6. 같은 개념에 하나의 용어를 유지한다. 본문 안에서 워커와 worker, 데몬과 daemon을 섞어
   쓰지 않는다.
7. 명령어, 코드 식별자, 경로, URL은 영어 원문을 유지하고, 나머지 서술은 위 규칙대로
   한국어로 쓴다.

## 한국어 게이트

IssueOps가 원격에 생성하거나 수정하는 issue와 PR/MR의 제목·본문은 한글 중심이어야 한다.
명령어, 코드 식별자, 파일 경로, URL, upstream·project 이름은 영어 원문을 유지할 수 있다.

```bash
python3 "$SKILL_DIR/scripts/remote_artifact_gate.py" --kind issue --title "$TITLE" --body-file "$BODY_FILE"
python3 "$SKILL_DIR/scripts/remote_artifact_gate.py" --kind pr --title "$TITLE" --body-file "$BODY_FILE"
```

- 제목과 본문을 임시 파일이나 heredoc으로 준비한 뒤 게이트를 실행한다.
- 게이트가 실패하면 원격 아티팩트를 만들거나 고치지 말고 한글 중심으로 다시 쓴다.
- 영어 섹션 라벨, 명령 출력, 코드 식별자, URL, 외부 프로젝트 이름이 들어 있어도 게이트는
  반드시 실행한다.
- 이 게이트를 실행하는 훅은 없다. 2026-08-27에 PreToolUse 훅이 제거됐으므로 실행 책임은
  이 절차에 있다.

원격 issue 본문에는 repo-local plan 경로를 넣지 않는다. plan 파일은 추적되지 않을 수
있으므로 링크는 `issueops link-plan` state와 PR/MR 본문에서만 다룬다.

## body-of-record

원격 issue 본문이 IssueOps 범위의 SSOT다. 사용자 피드백, 리뷰 피드백, QA, CI 증거,
에이전트 분석이 문제 정의·수락 기준·비목표·검증·구현 범위·관련 이슈 링크·label 중
하나라도 바꾸면 계속하기 전에 본문을 갱신한다. 스레드 댓글은 논의를 남길 뿐 계약이
아니다.

```bash
# 본문 갱신은 issueops-sync-issue가 소유한다. 갱신한 뒤 그 접합점을 기록한다.
issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" $RECORD_ACTOR_FLAGS --json
```

이 기록 전까지 `issueops pr-readiness --strict`는 `contract_feedback_issue_update`로
막힌 채 남는다. 그것이 의도다 — 계약이 바뀌었는데 팀이 보는 이슈가 그대로면, 진행
상태를 이슈로 공유한다는 전제가 깨진다.

## 나쁜 예

- raw `gh`·`glab`로 본문을 고친다. pending intent가 없으니 모호한 결과를 회복할 수 없다.
- preview 없이 `--confirm`을 붙인다.
- preview 뒤 본문을 고치고 같은 `--confirm`을 붙인다. 승인된 것과 다른 것이 쓰인다.
- timeout 뒤 create를 다시 실행한다. 중복 이슈나 중복 PR이 생기고, 그것을 정리하는 일이
  원래 작업보다 커진다.
- 실패 로그를 그대로 붙인다. 토큰과 자격 증명이 원격 이력에 영구히 남는다.
- `fluent-korean` 호출을 생략한다. 게이트는 한글 비율만 보므로 통과하고, 팀은 AI가 쓴
  본문을 읽는다.
- label과 assignee 없이 쓴다. 아무도 그 아티팩트를 자기 것으로 보지 않는다.

## 검증

- write 뒤 readback 응답의 URL·상태·본문 해시가 방금 쓴 것과 같은지 확인한다.
- `issueops status --id "$ISSUEOPS_ID" --json`의 `body_syncs`에 이번 write의
  baseline이 남았는지 확인한다.
- 모호한 결과를 만났다면 reconcile 결과가 정확히 하나의 아티팩트를 지목했는지 확인한다.
  둘 이상이면 사람이 판단할 때까지 멈춘다.
