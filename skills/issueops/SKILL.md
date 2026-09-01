---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, AI slop cleanup, feedback loops, and PR/MR drafting.
---

# IssueOps

IssueOps는 문제를 원격 Issue, 계획, 격리 worktree, 검증 증거, PR/MR까지
하나의 durable record로 연결하는 lifecycle router다. 이 파일은 **단계 선택과
공통 불변식만** 설명한다. 현재 단계의 전용 스킬·reference만 읽는다.

## 책임 분리

| 현재 작업 | owner |
|---|---|
| 전체 lifecycle·phase·execution lease | `issueops` |
| parent Issue·provider-native child | [`issueops-create-issue`](../issueops-create-issue/SKILL.md) |
| implement 단계 구현·child 위임·implementation review | [`issueops-implement`](../issueops-implement/SKILL.md) |
| GitHub PR·GitLab MR publication | [`issueops-create-pr`](../issueops-create-pr/SKILL.md) |
| execution completion 기록·generation 반납 | [`issueops-complete`](../issueops-complete/SKILL.md) |
| issue branch·worktree | [`issueops-branch-worktree`](../issueops-branch-worktree/SKILL.md) |
| merge 후 Issue·local worktree·branch 정리 | [`issueops-cleanup`](../issueops-cleanup/SKILL.md) |

Issue 단계에서 MR 스킬을, PR/MR 단계에서 Issue 스킬을 함께 읽지 않는다.
Provider mutation을 raw `gh`/`glab` 명령으로 우회하지 않는다.

## Core contract

사용자 관점 흐름:

```text
problem → grill → issue → plan → compatibility-review → implement
        → ai-slop-clean → feedback → pr → cleanup
```

durable phase 값은 `problem`, `grill`, `plan`, `compatibility-review`,
`implement`, `ai-slop-clean`, `feedback`, `pr`, `done`이다. `issue`는
linkage 단계이고 `cleanup`은 `done` 뒤의 후처리다. `done`은
`issueops execution complete`만 기록한다.

한 cycle은 다음 authority를 가진다.

- exact lifecycle ID
- 하나의 canonical worktree
- 하나의 generation-fenced native holder
- 하나의 linked Issue와 검증된 PR/MR

`agent-harness issueops ... --json`과 MCP `issueops_execution`이 durable
state를 소유한다. hook은 SessionStart에서 project-doc context만 제공한다.
Issue 생성, 파일 수정, 테스트, 대기, branch/worktree 준비, PR/MR publication,
reply, merge, cleanup을 hook에 맡기지 않는다.

## 단계별 라우팅

| 단계 | main agent가 남길 증거 | 읽을 문서·스킬 |
|---|---|---|
| problem | 요청, 성공 기준, 제약, ambiguity, non-goal, intent class | `von-neumann` |
| grill | code/docs/runtime 조사, domain model·용어 검토 | `issue-preflight.md`, 필요 시 `berners-lee` |
| issue | 한국어 body, score, label/assignee, URL readback, split/no-split | `issueops-create-issue` |
| plan | plan-prep 4항목, 설계, 대안, 위험, 검증 | `von-neumann`, `codd`, `dijkstra`, `karpathy` |
| compatibility-review | 호환성, side effect, rollback, blocker, 승인 | `evidence-contract.md` |
| implement | current generation, TDD, focused verification | `issueops-implement`, 필요 시 `hopper`·`turing` |
| ai-slop-clean | diff 기반 cleanup, 전후 품질 지표, 재검증 | `ai-slop-clean.md`, `shannon` |
| feedback | contract change 반영, review thread 검증 | `review-feedback.md` |
| pr | readiness, 한국어 body, actor/branch/URL readback | `issueops-create-pr` |
| pr(완료 기록) | 봉인된 artifact, final head, 보고서, 검증 증거 | `issueops-complete` |
| cleanup | merge evidence, preview/fingerprint, 사용자 승인 | `issueops-cleanup`, `cleanup-state.md` |

최신 사용자 지시가 이전 계획과 충돌하면 최신 지시를 반영하고 durable
contract를 갱신한다. 충돌하지 않는 기존 목표는 유지한다.

## 시작 순서

1. `issueops list --repo "$PWD" --json`으로 동일 repo의 cycle을 확인한다.
2. 없으면 `issueops start`; 있으면 exact ID로 `status`를 읽는다.
3. `agent-harness issueops intent record`로 요청 계약을 기록한다.
4. `issueops plan-prep record`로 다음 네 항목을 evidence 또는 waive reason과
   함께 기록한다.
   - decisions
   - related issue/label score
   - web research
   - codebase survey
5. Issue는 `issueops-create-issue`로 생성·연결한다.
6. Issue 번호로 시작하는 branch 이름을 정한다. `feature/123-*`가 아니라
   `123-*` 형식이다.
7. base SHA를 고정하고 plan을 stage한 뒤
   `agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto`를
   preview한다. 출력된 readiness fingerprint가 든 `next_command`로 confirm한다.

`--mode direct`는 예외이며 `--direct-reason`이 필요하다. Orca는 owner/workspace
adapter일 뿐 두 번째 workflow authority가 아니다. GitHub Orca의 sealed
base-SHA linked-branch 순서는 다음과 같다.

`branch prepare` (base SHA only) → `artifact stage --name plan` → `execution prepare --mode orca` → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified`

status의 selection에서 `requested_mode`, `resolved_mode`, readiness fingerprint를
다시 읽는다. 상세 복구 명령은 `execution.md`만 소유한다.

## Gate map

readiness 오류가 나오면 숨은 override를 추측하지 말고 해당 state owner를
실행한다.

| missing state | owner command/reference |
|---|---|
| `intent_contract` | `issueops intent record` |
| `plan_prep_*` | `issueops plan-prep record` |
| Issue URL·label·assignee·hierarchy | `issueops-create-issue` |
| `design_review` | `agent-harness issueops design review` |
| `compatibility_review` | `issueops compatibility review` |
| `devils_advocate_review` | `issueops devils-advocate review` |
| branch·worktree·plan·execution lease | `issueops-branch-worktree`, `execution.md` |
| `implementation_review`·`implementation_review_stale` | `issueops-implement` |
| `ai_slop_clean` | `issueops ai-slop-clean record` |
| contract feedback Issue 반영 | `issueops feedback mark-issue-updated` |
| PR/MR body·target·actor·readback | `issueops-create-pr` |
| `execution completion requires pr phase`·`final_head must match`·Turing report 경로 | `issueops-complete` |
| cleanup readiness | `issueops-cleanup`, `cleanup-state.md` |

승인된 design review에는 refactor plan, 대안, 위험, 검증이 있어야 하고 open
question은 없어야 한다. compatibility review는 blocker가 없어야 한다.
plan이 바뀌면 plan hash에 묶인 devil's-advocate·implementation review freshness를
다시 확인한다.

## Child와 delegation

원격 parent/child 분리는 `issueops-create-issue`가 소유한다. 기본은 no split,
child는 `[p]`가 기본이고 `[s]`는 이름 있는 hard dependency가 있을 때만 쓴다.

실행 중 delegated child cycle은 별도 개념이다. 다음 조건이 모두 맞을 때만
`issueops child start`를 사용한다.

- parent가 `implement` phase다.
- design·compatibility·devil's-advocate gate가 승인 또는 명시적으로 waive됐다.
- plan이 sub-agent pattern, scope, acceptance, verification, fallback,
  tradeoff를 기록한다.

parent는 child record를 대신 수정하지 않는다. parent가 소유하는 것은
`accept`, `reject`, `drop` verdict뿐이다. 자세한 prompt와 scope-drift 규칙은
`orchestration.md`를 따른다.

## 구현·검증 규칙

- behavior change는 focused failing test에서 시작해
  `RED→GREEN→SURFACE→CLEAN` 순서로 검증한다.
- 작업은 canonical worktree에서만 한다. source checkout에 구현하지 않는다.
- API/DTO/OpenAPI 변경은 `.agent-harness/OPEN_API_SPEC.md` gate를 적용한다.
- live runtime, review reply, completion hygiene가 요청 범위라면 테스트 통과만으로
  완료를 선언하지 않는다.
- ai-slop-clean은 실제 diff가 생긴 뒤 실행하고, cleanup 후 관련 검증을 다시
  실행한다.
- Git staging/push는 `atomic-commit-push`, 고급 history 작업은 `torvalds`가
  소유한다. 사용자 지시 없이 commit하거나 push하지 않는다.
- destructive cleanup은 exact target과 fingerprint를 preview한 뒤 별도 사용자
  승인을 받는다.

## Remote write 공통 게이트

Issue와 PR/MR publication의 body·예시·명령은 전용 생성 스킬이 소유한다.
공통 규칙만 유지한다.

- score 결과를 join한 뒤 threshold 이상 label만 적용한다.
- label과 concrete assignee가 없으면 쓰지 않는다.
- title/body는 한국어 중심이며 secret 원문을 포함하지 않는다.
- preview와 동일한 요청에만 `--confirm`을 추가한다.
- provider 호출 결과가 불명확하면 자동 retry하지 않고 reconcile한다.
- contract-changing feedback가 생기면 원격 Issue body를 갱신하고 durable
  state에 반영 사실을 기록한다.

## Reference map

현재 단계에 해당하는 파일만 읽는다.

| reference | 책임 |
|---|---|
| `references/issue-preflight.md` | ambiguity 감소, ideal Issue prompt |
| `references/remote-issue.md` | provider relation·hierarchy·한국어 공통 규칙 |
| `references/evidence-contract.md` | domain/API/live/review/completion evidence |
| `references/worktree-context.md` | branch·worktree·local config |
| `references/execution.md` | direct/Orca, generation, claim/recovery/publication |
| `references/orchestration.md` | delegated child contract |
| `references/ai-slop-clean.md` | diff cleanup |
| `references/review-feedback.md` | feedback·thread resolution |
| `references/cleanup-state.md` | post-merge cleanup |
| `references/operational-start.md` | start/resume command sequence |

## Stop conditions

다음 조건이면 다음 phase나 remote write로 진행하지 않는다.

- provider, credentials, project, Issue owner, target branch가 모호하다.
- intent·success criteria·domain term 해석이 구현을 바꿀 만큼 갈린다.
- design open question, compatibility blocker, stale review가 남아 있다.
- branch/worktree/plan/generation/actor가 current record와 맞지 않는다.
- strict PR readiness가 Issue, branch link, plan, worktree, upstream,
  ai-slop-clean, contract feedback를 누락했다고 보고한다.
- label·assignee·한국어 body·target branch·live readback이 검증되지 않았다.
- merge evidence 없이 cleanup을 요청한다.

## IssueOps benchmark artifact contract

Benchmark 응답에는 의도나 계획만 쓰지 말고 다음 labeled evidence를 넣는다.

```text
Durable state record: <IssueOps id, phase, readiness gates, state path/tool output>
Phase routing: <problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup>
Flow evidence: <Issue, plan, TDD, sub-agent decision, feedback, PR/MR artifacts>
Hook boundary: <what hooks may suggest and what only the main agent/CLI owns>
Cleanup/readiness evidence: <strict readiness, merge/cleanup status, remaining choices>
```

semantic judge는 artifact 작성자가 아닌 fresh-context host agent가 맡는다.
deterministic pass를 먼저 실행하고, JSON-only judge map을 `--judge file`로
strict-decode한다. 외부 judge는 read-only이며 workspace나 remote를 수정하지
않는다.

## Execution ownership

active holder만 canonical worktree에서 구현·검증·publication하고
`issueops execution complete`를 호출한다. 이 명령은 `done`을 기록하고 lease를
해제할 뿐 merge나 resource 삭제를 하지 않는다. merge와 cleanup은 별도 단계다.
