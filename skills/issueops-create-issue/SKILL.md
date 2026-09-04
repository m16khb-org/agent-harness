---
name: issueops-create-issue
description: Confirm and create the IssueOps issue that a cycle contracts on. Interview the user only on blocking ambiguity, survey the codebase and background sources into plan-prep evidence, record the intent contract, domain review, split decision, and plan prep, then publish a readable Korean issue through the shared remote write protocol and link it. Use at the start of a cycle, when "issueops next" reports stage none or issue, or when the user says "이슈 만들어줘", "이슈부터 시작하자", "child task 만들어줘".
---

# IssueOps Create Issue

이 스킬의 일은 **1단계 하나**다. 무엇을 할 일인지 확정하고, 그 확정을 원장에
기록하고, 팀이 보는 이슈로 만든다. 브랜치·계획·구현은 하지 않는다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 다음 단계: [`issueops-prepare`](../issueops-prepare/SKILL.md)
- 원격 쓰기 절차: [`issueops-remote-write`](../issueops-remote-write/SKILL.md)
- provider 링크·계층 규칙: [`remote-issue.md`](../issueops/references/remote-issue.md)

## 이 스킬이 맞는지 확인

```bash
agent-harness issueops next --json
```

`stage.key`가 `none`이거나 `issue`면 이 스킬이다. 사용자가 라우터의 "새 사이클
시작"을 골랐으면 `stage.key`와 무관하게 진행하며, 이 저장소에 다른 사이클이 있어도
새 `start`는 허용된다. 그 밖의 값이면 라우터 `## 단계 표`가 지목하는 스킬을 안내하고
멈춘다.

`start`는 **source checkout**에서 실행한다. 워크트리 안에서 실행하면 record의 repo가
워크트리를 가리키고 이후 모든 경로 판정이 어긋난다.

## 입력 세 가지

이슈는 세 곳에서 온 것을 합쳐 만든다. 각 조사 결과가 그대로 plan-prep의 evidence가
되므로, 조사하면서 무엇을 봤는지 문자열로 남긴다.

1. **사용자가 준 정보.** 원문을 그대로 intent contract의 `--raw-request`에 넣는다.
   요약해서 넣으면 나중에 해석이 맞았는지 대조할 원본이 사라진다.
2. **코드베이스 조사.** `.codegraph/`가 있으면 `codegraph explore "<질문>"`으로 관련
   심볼과 호출 경로를 찾고, 없으면 `rg`로 찾는다. 만진 심볼·파일·호출 경로를 evidence
   문자열로 만든다(`--codebase-survey-evidence`).
3. **배경지식과 웹 조사.** 외부 API의 의미나 계약이 걸릴 때만 조사한다
   (`--web-research-evidence`). 조사하지 않았으면 waive하지 말고 왜 필요 없는지를
   evidence로 쓴다. 관련 이슈는 `--related-score-ref`로, 이미 내려진 결정은
   `--decisions-evidence`로 남긴다.

## 질문 규칙

모호함을 세 갈래로 나눠 원장처럼 관리한다.

| 분류 | 뜻 | 처리 |
|---|---|---|
| `resolved` | 조사로 답이 나왔다 | 이슈 본문의 근거 절에 답과 출처를 쓴다 |
| `deferred` | 지금 몰라도 구현 방향이 바뀌지 않는다 | 이슈 본문 "열린 결정" 절에 남긴다 |
| `blocking` | 답에 따라 다른 것을 만들게 된다 | 사용자에게 묻는다 |

- **blocking만 묻는다.** 조사로 답할 수 있는 것을 묻는 것은 조사를 사용자에게 떠넘기는
  것이다.
- 한 번에 한 질문만 한다. 선택지가 있으면 번호로 제시하고 추천안을 먼저 둔다.
- 답을 받으면 그 답이 무엇을 바꾸는지 한 문장으로 되돌려 확인한다.
- 다음을 확인할 때까지 이슈를 만들지 않는다: 사용자에게 보이는 문제와 지금 중요한
  이유, 테스트와 실제 표면으로 검증 가능한 성공 기준, 비목표와 범위 경계, 근거가 필요한
  도메인 용어, 필요한 파일·API·명령·런타임 표면, 구현을 실질적으로 바꿀 열린 결정.

## 기록 순서

이 순서가 곧 grill 완료 조건이다. 항목이 비면 grill 진입이 거부된다.

```bash
agent-harness issueops start --repo "$SOURCE_ROOT" --json      # ISSUEOPS_ID를 받는다

agent-harness issueops intent record --id "$ISSUEOPS_ID" \
  --raw-request "<사용자 원문>" --interpreted-intent "<해석>" \
  --success-criteria "<검증 가능한 기준>" --intent-class trivial|standard \
  $RECORD_ACTOR_FLAGS --json

agent-harness issueops domain-review record --id "$ISSUEOPS_ID" \
  --model-fit "<도메인 모델과 이 변경의 관계>" $RECORD_ACTOR_FLAGS --json

# 분할하지 않는 경우: 그 근거를 결정으로 남긴다.
agent-harness issueops decision add --id "$ISSUEOPS_ID" --kind scope \
  --title "no split" --body "<한 owner·한 리뷰로 끝나는 근거>" $RECORD_ACTOR_FLAGS --json
# 분할하는 경우: remote create-child로 child를 만든다(아래 Parent와 child).

agent-harness issueops plan-prep record --id "$ISSUEOPS_ID" \
  --decisions-evidence "<...>" --related-score-ref "<...>" \
  --web-research-evidence "<...>" --codebase-survey-evidence "<...>" \
  $RECORD_ACTOR_FLAGS --json

agent-harness issueops phase --id "$ISSUEOPS_ID" --to grill $RECORD_ACTOR_FLAGS --json
```

여기까지가 로컬 기록이다. 다음은 원격 write이므로 본문 초안을 사용자에게 보여 주고
승인을 받은 뒤 [`issueops-remote-write`](../issueops-remote-write/SKILL.md)의 절차로
진행한다. 그 스킬이 fluent-korean 호출, 한국어 게이트, preview, 동일 요청 confirm,
readback, 모호할 때의 reconcile을 소유한다.

```bash
agent-harness issueops remote score --input "$SCORE_INPUT" --judge none --json > "$SCORE_FILE"
# → issueops-remote-write 절차로 remote create-issue 실행
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" $RECORD_ACTOR_FLAGS --json
```

`remote create-issue`가 `issue_url`을 record에 이미 넣었으면 `link-issue`는 생략한다.
`status --json`으로 확인하고 없을 때만 실행한다.

마지막 출력은 다음 세 줄이다.

```text
ISSUEOPS_ID: <id>
issue: <url>
다음 단계: issueops-prepare
```

## 먼저 고르는 것

| 요청 | 사용할 형식 | 분리 기준 |
|---|---|---|
| 결함·회귀 | `bug` | 재현 절차와 기대/실제 동작을 숫자 목록으로 쓴다 |
| 사용자 기능 | `feature` | 사용자 가치와 완료 조건을 짧은 checklist로 쓴다 |
| 구조·정책 결정 | `proposal` | 대안 비교가 핵심이면 표와 결정 근거를 쓴다 |
| 바로 실행할 작업 | `implementation_task` | 근거·범위·검증을 중심으로 쓴다 |
| parent의 독립 작업 | `child_task` | scope·의존성·wave를 metadata 표로 쓴다 |

정보를 모두 한 문단에 넣지 않는다. **결론은 위에, 근거는 해당 section에,
실행 명령은 검증 section에** 둔다. 다이어그램은 흐름·상태·경계가 문장보다
빠르게 읽힐 때만 쓴다.

## 템플릿을 정한 근거

공식 문서의 기능 범위와 이 저장소의 운영 계약을 맞췄다.

- [GitHub issue form 문법](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms):
  field type과 required validation을 사용한다.
- [GitHub PR template](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository):
  본문에 반복되는 리뷰 정보를 미리 제공한다.
- [GitLab description templates](https://docs.gitlab.com/user/project/description_templates/):
  Issue와 MR 모두 Markdown template을 사용한다.

그래서 GitHub Issue는 입력 오류를 앞에서 막는 form으로, GitLab Issue와
PR/MR은 같은 읽기 순서를 유지하는 Markdown으로 관리한다. 필수 항목은
많이 넣기보다 완료·검증에 실제로 필요한 항목만 남긴다.

## Parent와 child

기본값은 `no split`이다. 한 owner가 한 번의 검토 가능한 변경으로 끝낼 수
있다면 다음 근거를 parent에 남기고 child를 만들지 않는다.

```text
Large Issue Breakdown Gate: no split
- 독립적인 acceptance와 rollback 경계가 없다.
- 한 owner와 한 MR로 검토할 수 있다.
- 이번 범위는 <파일/모듈> 안에 머문다.
```

다음 중 하나가 있을 때만 split한다.

- 한 Issue에 두면 독립된 delivery·rollback·review가 숨겨진다.
- 사용자가 병렬 ownership 또는 assignee 분리를 명시했다.

| 항목 | 규칙 |
|---|---|
| 실행 class | `[p]` 기본. `[s]`는 이름 있는 hard dependency가 있을 때만 |
| `[p]` | prerequisite `none`, 독립 검증, 보통 wave 1 |
| `[s]` | 선행 child URL/산출물과 순서를 반드시 명시 |
| 생성 | ordinary sibling이 아닌 `remote create-child` |
| parent 기록 | `## 하위 Task` 아래 URL·scope·wave·prerequisite를 본문에 기록 |

parent body를 안전하게 갱신할 IssueOps 경계가 없으면 raw `gh`/`glab`로
우회하지 말고 중지한다. 댓글만 남기는 것은 완료가 아니다.

## 읽기 좋은 body

본문의 문장 규칙과 원격 쓰기 전 다듬기는
[`issueops-remote-write`](../issueops-remote-write/SKILL.md)가 소유한다. 여기서는 어떤
절을 어떤 순서로 두는지만 정한다.

### Implementation Issue 좋은 예

```markdown
## 문제
Issue와 PR/MR 생성 절차가 한 스킬에 섞여 있어 필요한 지침을 찾기 어렵다.

## 현재 근거
`skills/issueops/SKILL.md`가 lifecycle과 publication 규칙을 함께 안내한다.

## 관련 이슈/라벨 판단
threshold 0.70; 선택 `enhancement`; 거절 `documentation`; override 없음.

## 완료 기준
- [ ] Issue와 PR/MR 전용 스킬이 각각 독립 검증된다.
- [ ] 기존 lifecycle 라우팅과 provider contract가 유지된다.

## 비목표
provider API나 전체 IssueOps lifecycle을 재설계하지 않는다.

## 구현 범위
두 SKILL.md, router 링크, remote 입력 validation만 수정한다.

## 검증
`python3 scripts/validate-skill.py skills/issueops-create-issue`

## 위험과 트레이드오프
라우팅 누락 가능성은 focused skill validation과 router readback으로 줄인다.

## 피드백 기록
정보량보다 한국어 독자의 첫 읽기 순서를 우선했다.
```

### Bug Issue 좋은 예

```markdown
## 문제
`--label " bug "`가 공백을 제거하지 않고 provider에 전달된다.

## 재현 절차
1. label을 앞뒤 공백과 함께 두 번 전달한다.
2. `create-issue --confirm`을 실행한다.
3. provider request를 확인한다.

## 기대 동작
`bug` 한 번만 전달된다.

## 실제 동작
공백 label과 중복 label이 request에 남는다.

## 현재 근거
`cmd/harness/issueopscli/remotecmd/remote.go`의 repeated flag 경계.
```

로그는 secret을 제거한 짧은 code block으로만 붙인다. 긴 로그 전체나
스크린샷 대신 재현에 필요한 줄과 파일·명령을 적는다.

### 나쁜 예

| 나쁜 입력 | 왜 나쁜가 | 고치는 방법 |
|---|---|---|
| `버그 고쳐주세요` | 재현·완료 기준이 없다 | bug template의 재현/기대/실제 작성 |
| 파일 20개 목록 | 문제와 연결되지 않는다 | scope와 non-goals를 한 문단씩 작성 |
| `나중에 테스트` | 검증이 실행 가능하지 않다 | 명령과 기대 결과를 명시 |
| `task: 작업` | parent·class·wave가 없다 | `[p]`/`[s]`, prerequisite, wave 작성 |
| closed #18에 새 child 부착 | 종료된 umbrella 재사용 | 활성 parent를 확인하거나 새 parent 준비 |
| token이 든 로그 첨부 | secret이 durable artifact에 남는다 | redaction 후 최소 재현 출력만 첨부 |
| 워크트리 안에서 `start` 실행 | record의 repo가 워크트리를 가리켜 이후 경로 판정이 어긋난다 | source checkout에서 실행한다 |
| blocking이 아닌 질문을 연달아 던짐 | 조사로 답할 것을 사용자에게 떠넘긴다 | 조사로 답하고 blocking만 묻는다 |
| plan-prep 네 항목을 waive로 채움 | 계획의 근거가 비어 있는 채로 다음 단계가 진행된다 | 조사 결과를 evidence로 넣고, 필요 없으면 그 이유를 evidence로 쓴다 |

## Canonical publication

score 결과를 보존하고 body file을 위 형식으로 먼저 읽어 본다. preview와 confirm의
규율은 [`issueops-remote-write`](../issueops-remote-write/SKILL.md)가 소유한다.

```bash
agent-harness issueops remote score \
  --input "$SCORE_INPUT" --judge none --json > "$SCORE_FILE"
agent-harness issueops remote create-issue \
  --id "$ISSUEOPS_ID" --provider "$PROVIDER" \
  --title "[enhancement] IssueOps 생성 경계를 분리한다" \
  --template implementation_task --body-file "$BODY_FILE" \
  --score-file "$SCORE_FILE" \
  --label enhancement --assignee "$ASSIGNEE" --json
```

child는 parent URL이 record에 연결되고 umbrella branch gate가 통과한 뒤 만든다.

```bash
agent-harness issueops remote create-child \
  --id "$ISSUEOPS_ID" --title "[p] Issue body 계약을 검증한다" \
  --template child_task --body-file "$CHILD_BODY" \
  --label enhancement --assignee "$ASSIGNEE" \
  --host "$HOST" --session-id "$SESSION_ID" --cwd "$WORKER_PATH" --json
```

child body에는 parent URL, scope, `[p]`/`[s]`, prerequisite, wave, acceptance,
verification, merge condition, cleanup을 넣는다. confirmed child 결과의
`hierarchy_verified`, type, URL, labels, assignee와 parent body readback을
확인한다.

## 품질·성능 게이트

- 품질: template critical validation 0, 한국어 body, 원격 write 전
  `fluent-korean` 호출, score 기록, secret redaction,
  hierarchy/label/assignee readback.
- 성능: issue 단계에서만 이 스킬을 로드한다. PR/MR reference를 함께
  중복 로드하지 않는다. 변경 전후 byte 수와 focused 검증 시간을 기록하되
  측정 없는 성능 개선을 주장하지 않는다.
- 검증:

```bash
python3 scripts/validate-skill.py skills/issueops-create-issue
python3 scripts/verify-skill-shell.py skills/issueops-create-issue
wc -c skills/issueops-create-issue/SKILL.md
```

provider, project authority, owner, body, label/assignee, hierarchy, 또는
durable intent가 모호하면 쓰지 않는다.
