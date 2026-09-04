---
name: issueops-plan
description: Turn a branch-prepared IssueOps cycle into an implementable contract from the source checkout. Read the operating documents that constrain the change, write the plan, stage it as the sealed plan artifact, approve the design review, run the adversarial review loop until it passes, then hand off with "execution prepare --mode auto" so Orca launches the implementation session when it is available and direct keeps the current session when it is not. Use when "issueops next" reports plan.write, plan.design, plan.review, or plan.handoff, or when the user says "계획 세워줘", "계획 검토해줘", "구현 인계".
---

# IssueOps Plan

이 스킬의 일은 **구현할 수 있는 계약을 만들고 구현 세션에 넘기는 것**이다. 구현은
하지 않는다. 이 단계가 끝나면 워크트리와 구현 세션이 생긴다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 계획 작성: [`von-neumann`](../von-neumann/SKILL.md)
- 적대 리뷰: [`issueops-review`](../issueops-review/SKILL.md)
- 게이트 원장: [`gates-ledger`](../gates-ledger/SKILL.md)
- 프로젝트 문서 갱신 절차: [`project-docs-update`](../project-docs-update/SKILL.md)
- 다음 단계: [`issueops-implement`](../issueops-implement/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
agent-harness issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `plan.write`, `plan.design`, `plan.review`, `plan.handoff` 중 하나면 이
스킬이다. 네 값은 이 스킬 안의 어느 절부터 시작할지를 말한다. `prepare`면
[`issueops-prepare`](../issueops-prepare/SKILL.md)가 먼저다.

## 어디에서 실행하는가

이 단계는 **source checkout의 준비 세션**이 수행한다. 워크트리는 아직 없다. 워크트리를
만드는 것은 이 단계 끝의 `execution prepare`이고, 그것이 direct와 Orca 중 하나를 고른다.

그래서 계획 파일은 **source checkout 밖의 임시 파일**에 쓰고 `artifact stage`로 올린다.
source checkout 안에 계획을 만들면 그 파일이 커밋 대상이 되고, 워크트리가 생긴 뒤에는
두 곳에 같은 계획이 존재한다.

`git worktree add`를 실행하지 않는다.

## 프로젝트 문서 확인

계획을 쓰기 **전에** 이 변경을 제약하는 운영 문서를 읽는다. 계획을 다 쓰고 나서
읽으면 이미 내려진 결정을 되돌리게 된다.

```bash
# MCP: project_docs_route에 이슈 제목과 구현 범위를 넣어 읽을 문서를 고른다.
# route를 쓸 수 없으면 required-doc 목록을 쓴다.
agent-harness docs --json
# MCP: project_docs_read로 각 문서의 현재 내용과 SHA를 읽는다.
```

최소한 다음을 읽는다: `CONSTITUTION.md`, `ARCHITECTURE.md`(해당 모듈),
`CONVENTIONS.md`, `CAUTIONS.md`(색인과 해당 모듈), `ADR.md`(관련 결정), `TESTING.md`.

읽은 결과는 계획의 `## 적용되는 결정과 주의사항` 절에 **문서 경로, 항목 제목, 이 계획에
미치는 제약 한 문장**으로 적는다. 적용되는 항목이 없으면 "대조했으나 없음"과 대조한
문서 목록을 적는다. 읽지 않은 것과 읽었는데 없는 것은 다르다.

이 절은 두 곳으로 흘러간다. design review의 `--risk`에 그 제약이 들어가고,
`issueops-review`의 plan 리뷰 프롬프트에 이 절이 들어가 리뷰어가 "무시된 결정"을
공격한다.

## 계획 작성과 스테이징

[`von-neumann`](../von-neumann/SKILL.md)을 호출해 계획을 쓴다. 저장 위치는
`$(mktemp -d)/plan.md`처럼 source checkout 밖이다.

계획에는 다음 네 절이 반드시 있다.

| 절 | 내용 |
|---|---|
| `## 적용되는 결정과 주의사항` | 위 문서 확인의 결과 |
| `## 재사용하는 기존 구현` | plan-prep의 코드베이스 조사에서 찾은 심볼·패키지·테스트 헬퍼와 재사용 방식. 새로 만드는 것이 있으면 기존 것으로 왜 안 되는지 |
| `## 성능 영향` | hot path 여부, 복잡도 변화, 측정 계획. 알고리즘 선택이 걸리면 [`dijkstra`](../dijkstra/SKILL.md) |
| `## 하위 호환성과 side effect` | CLI JSON·MCP schema·golden·record schema·provider body 계약, 기존 데이터, 롤백 경로 |

이 네 절은 형식이 아니라 판단이다. "재사용할 것이 없다"는 결론도 근거와 함께 적으면
유효하고, 근거 없이 비워 두면 리뷰가 그것을 공격한다.

```bash
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file "$TMP_PLAN" --json
```

`artifact stage`는 actor 플래그를 받지 않는다. `--id`, `--name`, `--file`, `--json`뿐이다.
잘못 올렸으면 `agent-harness issueops artifact unstage --id "$ISSUEOPS_ID" --name plan --json`으로
내린다.

`link-plan`은 여기서 하지 않는다. 워크트리가 없어 `plan_in_worktree`를 만족할 수 없다.
prepare가 스테이징한 계획을 워크트리 안에 materialize하고, 4단계가 그것을 링크한다.

## 게이트 원장

계획의 수용 기준을 [`gates-ledger`](../gates-ledger/SKILL.md)의 형식대로 `G1..Gn`
(CHECK/EXPECT)으로 계획 안에 적어 둔다. 원장 **파일**은 워크트리 안에 있어야 하므로
여기서 만들지 않는다. 4단계가 파일을 만들고 `--write`로 EVIDENCE를 채우며, 5단계가
정리 뒤 다시 채우고, 7단계가 읽기 전용으로 재검사한다.

## 설계 검토

```bash
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "<문제>" --proposed-design "<설계>" --refactor-plan "<리팩터 계획>" \
  --alternative "<기각한 대안과 이유>" --risk "<위 문서 확인에서 온 제약>" \
  --verification "설계 검토로 대안과 위험을 확인했다" --approved $RECORD_ACTOR_FLAGS --json

agent-harness issueops record-routing --id "$ISSUEOPS_ID" --phase plan --skill von-neumann \
  $RECORD_ACTOR_FLAGS --json
```

- `--verification`에는 "설계"와 "검토"가 함께 들어가야 한다(영어면 "design"과 "review").
  그 두 단어가 없으면 `design_review_evidence`가 통과하지 않는다.
- `--refactor-plan`, `--alternative`, `--risk`는 각각 하나 이상 필요하고, open question은
  0이어야 승인된다. 열린 질문이 남았으면 그것을 먼저 닫는다.

## 검토 루프

[`issueops-review`](../issueops-review/SKILL.md)를 `--target plan`으로 호출한다. 루프
절차는 그 스킬이 소유하므로 여기 복사하지 않는다. 이 단계가 알아야 하는 것은 셋이다.

- **입력**: 스테이징한 계획 전체. 위 `## 적용되는 결정과 주의사항` 절이 포함된다.
- **종료 조건**: `pass` 판정이 finding 하나 이상과 함께 기록됐다.
- **stop 판정**: 이 단계가 되돌린다.

```bash
agent-harness issueops regress --id "$ISSUEOPS_ID" --reason "<리뷰 결론>" $RECORD_ACTOR_FLAGS --json
```

`regress`는 사이클을 grill로 되돌린다. 다시 조사하고 이슈 본문을 갱신한 뒤
`phase --to plan`으로 올라와 계획을 다시 쓴다. 판정은 계획 파일의 sha256에 묶이므로,
판정 뒤 계획을 고치면 `devils_advocate_review_stale`이 되어 인계가 막힌다. 고쳤으면
다시 검토한다.

## 인계

```bash
agent-harness issueops execution whoami --json   # ACTOR_FLAGS 원문
agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto \
  --owner-host "$HOST" $ACTOR_FLAGS --json        # preview
# 출력의 next_command(--expected-readiness-fingerprint 포함)를 그대로 실행한다.
```

`--mode auto`가 모드를 고른다. 스킬이 모드를 강제하지 않는다.

- **Orca가 준비돼 있으면** Orca가 브랜치와 워크트리를 만들고 구현 세션을 띄운다. lease는
  claimable로 남고, 그 세션이 자기 프롬프트의 봉인된 claim 명령을 정확히 한 번 실행해
  홀더가 된다. 이 세션의 3단계 임무는 거기서 끝난다.
- **Orca가 없거나 준비되지 않았으면** direct로 내려간다. git이 워크트리를 만들고 이
  세션이 곧바로 generation 1 홀더가 되므로, 같은 세션이 4단계를 이어간다.

어느 쪽인지는 결과의 `resolved_mode`로 확인한다. 추측하지 않는다.

preview가 `Orca prepare needs planner-owned records the owner cannot supply`로 막히면
planner 게이트가 덜 찬 것이다. 메시지가 필요한 명령을 함께 주므로 그것을 실행하고
preview를 다시 한다. `--mode direct`를 강제해 우회하지 않는다.

## 의존성과 로컬 설정

워크트리가 생긴 뒤의 규칙이지만 계획에 함께 적는다.

의존성은 canonical worktree 안에서 저장소가 문서화한 설치 명령으로 준비한다. 크게
생성되는 의존성 디렉터리를 재사용하려면 패키지 매니저·lockfile·런타임·플랫폼·네이티브
모듈 상태가 모두 같은지 확인한 뒤에만 한다. 생성된 의존성 디렉터리나 심볼릭 링크를
커밋하지 않는다.

`.env`, `.env.local`, `.mcp.json`, `dbhub.toml` 같은 로컬 전용 설정은 작업에 필요하고,
원본이 ignore돼 있고, 어떤 secret도 프롬프트·로그·테스트·이슈 본문·PR/MR 본문에 들어가지
않을 때만 링크한다. 추적되는 파일, ignore되지 않은 파일, 설명되지 않은 자격 증명 파일은
링크하기 전에 멈춘다.

## 나쁜 예

| 나쁜 행동 | 왜 나쁜가 | 대신 할 일 |
|---|---|---|
| source checkout 안에 계획 파일을 만든다 | 커밋 대상이 되고 워크트리 생성 뒤 계획이 두 곳에 존재한다 | 임시 디렉터리에 쓰고 `artifact stage` |
| `git worktree add`를 실행한다 | Orca 경로가 이름 충돌로 깨진다 | `execution prepare`가 만들게 둔다 |
| `--mode direct`를 강제한다 | 사용자 결정은 "Orca가 있으면 Orca"다 | `--mode auto`가 probe로 고르게 한다 |
| 리뷰 없이 `--verdict pass`를 기록한다 | 게이트 연극이다 | `issueops-review`로 실제 리뷰를 돌린다 |
| revise 판정을 `--waive`로 닫는다 | 지적이 반영되지 않은 채 구현으로 간다 | 계획을 고치고 다시 검토한다 |
| 판정 뒤 계획을 고치고 재검토를 생략한다 | stale 판정으로 인계가 막히거나, 검토되지 않은 계획이 구현된다 | 다시 검토해 새 판정을 기록한다 |
| staged plan 없이 인계한다 | prepare가 워크트리에 materialize할 계획이 없다 | `artifact stage`를 먼저 한다 |
| Orca 세션이 떴는데 이 세션이 계속 구현한다 | 두 세션이 같은 워크트리를 쓴다 | `resolved_mode`가 orca면 여기서 멈춘다 |

## 검증

- `agent-harness issueops status --id "$ISSUEOPS_ID" --json`의 `design_review.approved`가
  true이고 `devils_advocate_review.verdict`가 `pass`이며 그 digest가 현재 계획과 같다.
- `agent-harness issueops next --id "$ISSUEOPS_ID" --json`의 `stage.key`가 `plan.handoff`
  이거나, 인계 뒤라면 `claim`(Orca) 또는 `implement.enter`(direct)다.
- `git -C "$SOURCE_ROOT" status --short`에 계획 파일이 없다.
- 인계 뒤 `execution prepare` 결과의 `workspace.root`가 실제로 존재하고 그 브랜치가
  record의 브랜치와 같다.
