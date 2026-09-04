---
name: issueops-review
description: Run an adversarial brooks review as a fresh sub-agent on an IssueOps plan or implementation diff, then record the verdict in the durable ledger with the plan digest or change fingerprint sealed. Owns the revise and stop loop rules shared by the plan and verify stages. Use when issueops-plan or issueops-verify needs a review, or when the user says "계획 검토", "구현 리뷰", "brooks 돌려줘", "devil's advocate".
---

# IssueOps Review

이 스킬의 일은 **적대 리뷰 한 번을 실제로 실행하고 그 판정을 원장에 기록하는
것**이다. 계획 단계와 검증 단계가 같은 규칙을 쓰도록 두 곳에서 이 스킬을
호출한다. 대상 파일을 고치는 것은 호출한 단계의 일이다.

- 계획 단계: [`issueops-plan`](../issueops-plan/SKILL.md)
- 검증 단계: [`issueops-verify`](../issueops-verify/SKILL.md)
- 리뷰 렌즈 원문: [`brooks`](../brooks/SKILL.md)
- 원격 반영 프로토콜: [`issueops-remote-write`](../issueops-remote-write/SKILL.md)

## 입력

`--target plan`과 `--target diff` 두 가지다. 대상을 정하고 리뷰어에게 줄 자료를
모은다.

| target | 리뷰 대상 | 함께 주는 자료 |
|---|---|---|
| `plan` | `status --json`의 `plan_path` 파일 전체(링크 전이면 staged plan artifact) | 이슈 본문, intent contract의 성공 기준, design review 본문 |
| `diff` | `git -C "$WORKTREE" diff "$BASE_SHA"` 전체 | plan 파일 전체, 검증 명령과 결과, 변경한 프로젝트 문서 |

리뷰어 모델과 effort는 다음 명령이 돌려준다.

```bash
agent-harness issueops next --id "$ISSUEOPS_ID" --json
# .review.model, .review.effort
```

이 값은 코드가 소유하는 host별 planner 기본값이다. 스킬 본문에 모델 이름 표를
복사하지 않는다 — 복사한 순간 그 표는 코드보다 먼저 낡는다. `review.model`이 비어
있으면 이 호스트의 기본값이 정의돼 있지 않다는 뜻이므로, 진행하지 말고 어떤 모델로
리뷰할지 사용자에게 묻는다.

`--target diff`는 정리와 재검증이 끝난 diff에만 실행한다. 판정이 fingerprint에
묶이므로, 리뷰 뒤 파일을 고치면 그 판정은 무효가 된다.

## 실행

현재 호스트의 delegation 도구로 brooks를 **빈 컨텍스트의 새 세션**에 띄운다.

저자 세션이 brooks의 게이트를 직접 밟아 보는 것은 리뷰가 아니다. 계획을 쓴
에이전트는 매몰 비용을 지고 있고 자기 합리화를 기억하며 자기 설계를 무의식적으로
변호한다. 독립성이 기제 전부다. 계획에 아무 투자도 하지 않은 새 컨텍스트만이 저자가
못 보는 결함을 본다.

프롬프트에 다음을 전부 넣는다. 서브에이전트는 빈 컨텍스트에서 시작하므로 여기 없는
것은 리뷰어에게 존재하지 않는다.

1. 대상 전문(위 표의 리뷰 대상과 함께 주는 자료).
2. 성공 기준과 범위 경계.
3. 관련 ADR과 CAUTIONS 항목의 경로와 제목.
4. 출력 계약: 판정(`pass|revise|stop`), 가장 위험한 결함 한 문장과 그것을 반증할
   가장 싼 실험, gate별 finding, 리뷰어라면 대신 내놓을 더 작은 계획.
5. 코드베이스 존중 렌즈 네 개.

코드베이스 존중 렌즈는 다음을 묻는다.

- **재사용**: 이미 있는 구현으로 됐을 일을 새로 만들지 않았는가. 같은 판정을 두 곳이
  소유하게 되지 않았는가.
- **성능**: 이 변경이 자주 불리는 경로에 관측·조회·할당을 늘리지 않는가. 같은 관측을
  두 번 하지 않는가.
- **하위 호환**: 계약 표면(포트 인터페이스, CLI 플래그, JSON 필드, 온디스크 형식)이
  기존 호출자를 깨지 않는가. 기본값이 종전 동작을 유지하는가.
- **side effect**: 파일·원격·durable state에 남는 변화가 문서화된 것과 일치하는가.
  실패했을 때 남는 상태가 사람이 이어받을 수 있는 모양인가.

## 기록

리뷰가 **끝난 뒤에만** 기록한다. 기록이 리뷰를 대신하지 않는다.

```bash
# --target plan
agent-harness issueops devils-advocate review --id "$ISSUEOPS_ID" \
  --verdict pass --reviewer-context subagent \
  --finding "<무엇을 공격했고 왜 살아남았는가>" $RECORD_ACTOR_FLAGS --json

# --target diff
agent-harness issueops implementation-review record --id "$ISSUEOPS_ID" \
  --verdict pass --finding "<finding>" --evidence "<evidence>" \
  --reviewer-host "$HOST" --reviewer-model "$REVIEWER_MODEL" --reviewer-effort "$REVIEWER_EFFORT" \
  $RECORD_ACTOR_FLAGS --json
```

`reviewer_context`와 `reviewer_*`는 감사 필드이지 게이트 조건이 아니다. 하네스는
모델의 자기신고를 검증할 수 없으므로 verdict와 finding·evidence의 실질만 게이트한다.
그래서 이 필드를 사실대로 적는 것은 도구가 아니라 실행자의 책임이다.

## 루프 규칙

- 1·2라운드는 대상 전체를 검토한다. 3라운드부터는 직전 판정 이후의 delta만 본다.
  같은 것을 세 번 읽는 리뷰는 새 결함을 찾지 못한다.
- `revise`면 호출한 단계가 대상을 고치고 이 스킬을 다시 실행한다.
- `stop`이면 `--target plan`은 호출자가 `agent-harness issueops regress --id
  "$ISSUEOPS_ID" --reason "<TEXT>"`로 grill까지 되돌려 재조사·재계획한다.
  `--target diff`는 blocker를 보고하고 멈춘다. 진행 판단은 사용자가 한다.
- `pass`는 finding이 하나 이상 있어야 기록된다. 아무것도 공격하지 않은 통과는 리뷰가
  일어나지 않았다는 뜻이다.
- `--waive`는 override이며 `--waiver-rationale`이 필수다. "지적을 반영했다"는 뜻으로
  쓰지 않는다. 반영했으면 다시 리뷰해서 새 판정을 받는다.
- 판정은 plan sha256(`reviewed_plan_digest`) 또는 change fingerprint
  (`reviewed_fingerprint`)에 묶인다. 판정 뒤 대상을 고치면
  `devils_advocate_review_stale`·`implementation_review_stale`이 되어 implement 진입과
  publication이 막히므로 다시 실행해 새 판정을 기록한다.

## 이슈 반영

계획 판정을 팀이 보게 하려면 [`issueops-remote-write`](../issueops-remote-write/SKILL.md)의
절차로 다음을 실행한다.

```bash
agent-harness issueops remote reflect-devils-advocate --id "$ISSUEOPS_ID" --confirm --json
```

`stop` 판정은 반영이 특히 중요하다. 사이클이 뒤로 돌아간 이유가 이슈에 남지 않으면
팀이 보는 진행 상태와 실제가 어긋난다.

## 나쁜 예

- 저자 세션이 인라인으로 게이트를 밟고 `--reviewer-context subagent`로 기록한다.
  기록은 통과하지만 리뷰는 없었다.
- 리뷰를 실행하지 않고 `--verdict pass`를 기록한다. 게이트 연극이다.
- `revise` 판정을 고치는 대신 `--waive`로 닫는다.
- 판정 뒤 계획이나 코드를 고치고 재검토를 생략한다. stale 판정으로 다음 단계에서
  막히고, 막히지 않았다면 검토되지 않은 변경이 게시된 것이다.
- `reviewer_model`이 기록됐으니 planner급 모델이 돌았다고 믿는다. 그 필드는 감사
  기록이지 증명이 아니다.
- `--target diff` 리뷰에 plan을 주지 않는다. 무엇을 하기로 했는지 모르는 리뷰어는
  구현이 계획에서 벗어났는지 판정할 수 없다.

## 검증

- `agent-harness issueops status --id "$ISSUEOPS_ID" --json`의 `devils_advocate_review`
  또는 `implementation_review`에 이번 판정이 있고 digest·fingerprint가 현재 대상과
  같은지 확인한다.
- `agent-harness issueops next --id "$ISSUEOPS_ID" --json`의 `missing`에
  `devils_advocate_review*`·`implementation_review*`가 남아 있지 않은지 확인한다.
- 남아 있으면 그 키가 곧 다음 명령이다. 추측하지 말고 `next_command`를 실행한다.
