---
name: issueops
description: Route an issue-driven work cycle across its ten user-facing stages. Run "issueops next" to decide which stage a cycle is in, hand the work to that stage's skill, and hold the invariants every stage shares. Use when the user asks to start, resume, or continue an issue cycle, when the current stage is unclear, or when they say "이슈옵스", "이슈 작업 시작", "이어서 진행".
---

# IssueOps

IssueOps는 이슈, 브랜치, 계획, 실행 lease, 검증 증거, PR/MR을 durable record
하나로 묶는다. 이 스킬은 **라우터**다. 단계의 일은 단계 스킬이 하고, 이 파일은
어느 단계인지 정하는 방법과 모든 단계가 공유하는 불변식만 소유한다.

## 먼저 실행

```bash
issueops next --json
```

이 명령은 읽기 전용이다. record와 로컬 관측만 쓰고 fetch도 provider 호출도 하지
않는다. 출력을 이렇게 읽는다.

1. `stage.key`를 아래 `## 단계 표`로 스킬과 한국어 label로 바꾼다.
2. `lease`, `missing`, `next_command`, `exits`를 사용자에게 그대로 보여 준다.
   `next_command_kind`가 `template`이면 `<...>`와 `$ACTOR_FLAGS`에 채울 값을 함께
   보여 주고, 실행은 사용자가 값을 확인한 뒤에 한다.
3. 아래 선택지 네 줄을 묻는다.

```text
1. 이어서 진행: <스킬> (<label>) (추천)
2. 다른 단계 지정: 원하는 단계 번호를 말해 주세요
3. 중단: issueops-abandon (일시 중단·폐기)
4. 새 사이클 시작: issueops-create-issue (이 저장소에 다른 사이클이 있어도 새로 시작한다)
```

- `blocked.*`면 진행하지 않는다. 1번을 `1. 대기: <next_command>로 상태를 다시
  읽습니다`로 바꾼다.
- `ambiguous`면 `candidates`를 보여 주고 어느 ID인지 고르게 한다. 추측해서 고르지
  않는다.
- `none`이면 1번과 4번이 같으므로 4번을 생략한다.

## 10단계와 스킬

| 단계 | 스킬 | 함께 쓰는 스킬 |
|---|---|---|
| 1 이슈 확정·생성 | [`issueops-create-issue`](../issueops-create-issue/SKILL.md) | `implementation-planning`(인터뷰), `web-research`(외부 조사) |
| 2 브랜치 준비 | [`issueops-prepare`](../issueops-prepare/SKILL.md) | — |
| 3 문서 확인·계획·검토·인계 | [`issueops-plan`](../issueops-plan/SKILL.md) | `implementation-planning`, `design-review`, `database-design`, `algorithm-optimization`, `prompt-engineering` |
| 4 구현 | [`issueops-implement`](../issueops-implement/SKILL.md) | `issueops-debugging`, `verified-execution`, `code-quality-metrics` |
| 5 AI slop 정리 | [`issueops-clean`](../issueops-clean/SKILL.md) | `code-quality-metrics`, `verified-execution` |
| 6 프로젝트 문서 반영 | [`issueops-docs`](../issueops-docs/SKILL.md) | `project-docs-update` |
| 7 검증 | [`issueops-verify`](../issueops-verify/SKILL.md) | `database-design`, `verified-execution` |
| 8 커밋·푸시 | [`atomic-commit-push`](../atomic-commit-push/SKILL.md) | `git-operations`(history 수술) |
| 9 PR/MR 발행·완료 | [`issueops-create-pr`](../issueops-create-pr/SKILL.md), [`issueops-complete`](../issueops-complete/SKILL.md) | — |
| 10 머지 후 정리 | [`issueops-cleanup`](../issueops-cleanup/SKILL.md) | — |
| 탈출(어느 단계든) | [`issueops-abandon`](../issueops-abandon/SKILL.md) | — |
| 공용: 적대 리뷰 | [`issueops-review`](../issueops-review/SKILL.md) | 3·7단계 |
| 공용: 게이트 원장 | [`gates-ledger`](../gates-ledger/SKILL.md) | 3·4·5·7단계 |
| 공용: 원격 쓰기 | [`issueops-remote-write`](../issueops-remote-write/SKILL.md) | 1·9·10단계, 본문 동기화 |

이미 만든 본문이 사이클보다 낡으면 [`issueops-sync-issue`](../issueops-sync-issue/SKILL.md)와
[`issueops-sync-pr`](../issueops-sync-pr/SKILL.md)이 관리 블록을 보존한 채 교체한다.

Issue 단계에서 PR/MR 스킬을, PR/MR 단계에서 Issue 스킬을 함께 읽지 않는다.

## 세션 경계

- **1·2단계는 source checkout의 준비 세션**이 수행한다. 워크트리는 아직 없다.
- **3단계**도 같은 세션이 수행하고, 그 끝의 `execution prepare --mode auto`가 워크트리와
  구현 세션을 만든다. Orca가 준비돼 있으면 Orca가, 아니면 direct가 고른다.
- **4단계 이후는 구현 세션**이 canonical worktree에서 수행한다. Orca 모드면 prepare가
  띄운 세션이 봉인된 claim으로 홀더가 되고, direct 모드면 3단계 세션이 그대로 이어간다.
- lease는 3단계의 인계에서 생긴다. 다른 세션이나 다른 호스트에서 이어받는 경로는
  [`issueops-abandon`](../issueops-abandon/SKILL.md)의 재개·인수 절이 설명한다.

## 공통 불변식

단계 스킬은 이 절을 링크하고 복사하지 않는다.

**(a) 단계 판별.** 모든 단계 스킬은 `issueops next --json`으로 시작하고
`stage.key`가 자기 단계인지 확인한다. 아니면 표가 지목하는 스킬로 안내한다. `blocked.*`
면 중단한다. phase를 추정하지 않는다.

**(b) actor 플래그.** `issueops execution whoami --json`이 돌려주는
`record_actor_flags`와 `claim_actor_flags`를 그대로 쓴다. 손으로 조립하지 않는다.

**(c) lease fencing.** durable mutation(phase 전이, record 기록, artifact stage) 전마다
exact lifecycle ID·generation·native actor·canonical cwd를 현재 record와 대조한다.
record 기록은 `RECORD_ACTOR_FLAGS`, lease 전이와 publication은 `ACTOR_FLAGS`를 쓴다.
두 축약의 정의는 `issueops --help`의 legend가 소유한다. 불일치는 stop이다. 사용자의
"그 세션은 내가 껐어"는 quiescence 증거가 아니다.

**(d) 편집 대상 확인.** 편집 배치마다 셸 프롬프트를 믿지 말고 실측한다.

```bash
pwd
git branch --show-current
git rev-parse HEAD
test "$PWD" = "$EXPECTED_WORKTREE"
git -C "$SOURCE_CHECKOUT" status --short
git status --short
```

patch·edit·생성 도구의 루트를 `$EXPECTED_WORKTREE`로 둔다. 도구가 다른 체크아웃에
썼으면 멈추고, 자기 변경만 canonical worktree로 옮긴 뒤 두 status를 다시 확인한다.
worker 프롬프트에는 exact lifecycle ID, generation, branch, worktree, 허용 경로,
수락 기준, 중단 규칙을 넣는다.

**(e) mutation 전 대조.** durable mutation 직전에 ID·generation·actor·cwd를 다시 읽고
record와 다르면 실행하지 않는다. 모호한 결과는 재실행이 아니라 reconcile이 처분한다.

**(f) 코드베이스 존중.** 새로 만들기보다 기존 구현의 확장과 재사용을 먼저 본다. 계약
표면의 하위 호환성, 성능 영향, 파일·원격·상태의 side effect를 세 곳에서 명시한다:
계획의 필수 절 세 개(`## 재사용하는 기존 구현`, `## 성능 영향`,
`## 하위 호환성과 side effect`), 구현 루프의 네 규칙, 리뷰의 네 렌즈. 근거는
`AGENTS.md` §2 Simplicity First와 §3 Surgical Changes다.

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

`issueops ... --json`과 MCP `issueops_execution`이 durable
state를 소유한다. hook은 SessionStart에서 project-doc context만 제공한다.
Issue 생성, 파일 수정, 테스트, 대기, branch/worktree 준비, PR/MR publication,
reply, merge, cleanup을 hook에 맡기지 않는다.

## 단계 표

`stage.key`를 스킬과 label로 바꾸는 유일한 표다. CLI는 이 표를 모른다.

| stage.key | 스킬 | label |
|---|---|---|
| `none`, `issue` | `issueops-create-issue` | 이슈 확정·생성 |
| `prepare` | `issueops-prepare` | 브랜치 준비 |
| `plan.write`, `plan.design`, `plan.review`, `plan.handoff` | `issueops-plan` | 문서 확인·계획·검토·인계 |
| `claim` | 스킬 없음. Orca가 띄운 세션은 자기 프롬프트의 봉인된 claim을 정확히 한 번 실행하고, 그 밖의 세션은 `next_command`가 돌려주는 체인을 lease가 active(self)가 될 때까지 따라간 뒤 `next`를 다시 실행한다 | 현재 index의 label |
| `implement.enter`, `implement` | `issueops-implement` | 구현 |
| `clean` | `issueops-clean` | AI slop 정리 |
| `docs` | `issueops-docs` | 프로젝트 문서 반영 |
| `verify` | `issueops-verify` | 검증 |
| `commit-push` | `atomic-commit-push` | 커밋·푸시 |
| `pr.create` | `issueops-create-pr` | PR/MR 발행·완료 |
| `pr.complete` | `issueops-complete` | PR/MR 발행·완료 |
| `done` | `issueops-cleanup` | 머지 후 정리 |
| `takeover` | 스킬 없음. `next_command`를 실행하고 결과가 돌려주는 `next_command`를 따라간다. 죽은 홀더 인수는 `issueops-abandon`이 설명한다 | 현재 index의 label |
| `blocked.pending`, `blocked.holder_live` | 없음. `next_command`로 상태를 다시 읽는다 | 현재 index의 label |
| `blocked.root_conflict` | 충돌 사이클을 `issueops-cleanup`(머지됨) 또는 `issueops-abandon`(미머지)으로 먼저 정리한다 | 현재 index의 label |
| `unknown`, `invalid` | 없음. `next_command`로 record를 읽고 `warnings`의 missing 키를 사용자에게 보여 준다 | 현재 index의 label |
| `ambiguous` | 사용자에게 `candidates` 중 ID 선택 또는 새 사이클 시작을 요청 | 없음 |

`missing` 키를 어떤 명령이 해소하는지는 `issueops next`의 `next_command`가 렌더한다.
그 대응표는 CLI가 소유하므로 여기 복사하지 않는다.

## 구현·검증 규칙

- behavior change는 focused failing test에서 시작해
  `RED→GREEN→SURFACE→CLEAN` 순서로 검증한다.
- 작업은 canonical worktree에서만 한다. source checkout에 구현하지 않는다.
- API/DTO/OpenAPI 변경은 `.issueops/OPEN_API_SPEC.md` gate를 적용한다.
- live runtime, review reply, completion hygiene가 요청 범위라면 테스트 통과만으로
  완료를 선언하지 않는다.
- ai-slop-clean은 실제 diff가 생긴 뒤 실행하고, cleanup 후 관련 검증을 다시
  실행한다.
- publication 전에 구현 diff를 project docs와 양방향으로 대조한다. CONSTITUTION·
  CONVENTIONS·ARCHITECTURE를 어겼으면 구현을 고치고, CAUTIONS에 남길 재발 함정이나
  ADR에 남길 결정이 생겼으면 문서를 먼저 고친 뒤 기록한다. 남길 것이 없으면 확인
  근거와 함께 `no-change`로 기록한다.
- 변경 집합에 마이그레이션·엔티티·SQL 스키마 파일이 있으면 실제 데이터베이스에서
  인덱스 현황과 대상 테이블 row 수를 관찰해 관찰값과 출처를 기록한다.
- Git staging/push는 `atomic-commit-push`, 고급 history 작업은 `git-operations`가
  소유한다. 사용자 지시 없이 commit하거나 push하지 않는다.
- destructive cleanup은 exact target과 fingerprint를 preview한 뒤 별도 사용자
  승인을 받는다.

## 원격 쓰기

원격 본문 쓰기의 공통 프로토콜은
[`issueops-remote-write`](../issueops-remote-write/SKILL.md)가 소유한다. 명령별 본문
형식은 각 생성·동기화 스킬이 소유한다.

## Reference map

현재 단계에 해당하는 파일만 읽는다.

| reference | 책임 |
|---|---|
| `references/remote-issue.md` | provider relation과 hierarchy |
| `references/evidence-contract.md` | domain/API/live/review/completion evidence |
| `references/execution.md` | direct/Orca, generation, claim/recovery/publication |
| `references/orchestration.md` | delegated child contract |
| `references/review-feedback.md` | feedback·thread resolution |
| `references/cleanup-state.md` | post-merge cleanup |

## Stop conditions

다음 조건이면 다음 phase나 remote write로 진행하지 않는다.

- provider, credentials, project, Issue owner, target branch가 모호하다.
- intent·success criteria·domain term 해석이 구현을 바꿀 만큼 갈린다.
- design open question, compatibility blocker, stale review가 남아 있다.
- branch/worktree/plan/generation/actor가 current record와 맞지 않는다.
- strict PR readiness가 Issue, branch link, plan, worktree, upstream,
  ai-slop-clean, project-doc 반영 판정, 스키마 실측 근거, contract feedback를
  누락했다고 보고한다.
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
