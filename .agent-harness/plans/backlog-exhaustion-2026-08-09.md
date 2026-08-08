# 백로그 소진 — 분류와 압축 (2026-08-09)

이 문서는 요청의 세 부분에 대한 최종 산출물이다.

1. 끝났는데 안 닫은 이슈를 닫는다
2. 진짜 남은 작업과 slop을 구분해 **압축**한다
3. 그 압축된 목록을 다 끝낸다

선행 문서는 `.agent-harness/plans/issue-backlog-triage-2026-08-08.md`(첫 triage)이며, 이 문서가 최종 상태다.

## 결과

| | 시작 | 끝 |
|---|---|---|
| 열린 이슈 | 45 | **0** |
| 열린 PR | — | **0** |
| cleanup 미완 lifecycle | 4 | **2** (살아 있는 세션 대기) |

## 1. 분류 — 45건을 세 부류로

### 1.1 이미 해결됨 (닫기만 하면 되는 것)

코드에 수정이 이미 들어가 있는데 이슈만 열려 있던 것들이다. 각각 **코드 위치 또는 머지 PR 번호를 인용**해 닫았다.

`#233 #234 #238 #342 #293 #250` 외 다수. 총 **약 30건**.

이 부류의 위험은 "grep에 없으니 미구현"으로 오판하는 것이다. 실제로 #329/#330/#332/#334를 그렇게 오판했다가, 기존 테스트를 돌려 전부 PASS임을 확인하고 정정했다. hook/guard는 레이어마다 이름이 달라 키워드 부재가 부재의 증거가 아니다.

### 1.2 slop — 하지 않을 것

**#280은 slop이 아니었다.** 처음에 slop으로 닫았다가 `worktree_create`와 `owner_launch` 두 경로로 재현해 되열었고, PR #417로 고쳤다. 이 경험 때문에 이후 slop 판정에는 재현 시도를 요구했다.

최종적으로 slop으로 분류해 닫은 것은 **대상 상태가 소멸한 검증 조건**들이다. 이건 "안 할 일"이 아니라 **"할 수 없는 일"**이라 성격이 다르며, 각각 증거와 함께 기록했다.

| 조건 | 왜 도달 불가 |
|---|---|
| #306 AC-08 | #304의 ref-null 고아가 소멸 (`linkedBranches.totalCount = 0`, 실측) |
| #328 기준 4·5의 `#248 typed recovery` | #248 worktree 제거됨. 그 artifact PR #265는 머지가 아니라 **CLOSED** |

사라진 상태를 인위적으로 되살려 조건을 채우는 것은 검증이 아니라 연출이므로 하지 않았다.

### 1.3 진짜 남은 작업 — 7개 단위로 압축

45건 중 실제 구현이 필요한 것을 **T1~T7**로 묶었다. 같은 결함의 다른 얼굴들을 하나로 합친 결과다.

| | 단위 | 결과 |
|---|---|---|
| T1 | Codex command-only hook payload의 owner mutation 도달성 | 완료 |
| T2 | active lease read-only reader grammar 확장 | 완료 |
| T3 | exact IssueOps command 분류 보강 | 완료 |
| T4 | provider 계약 정합 (#306 포함) | 완료 |
| T5 | cleanup 잔여 계약 | 완료 |
| T6 | Orca 실행 계약 (#319·#325 포함) | 완료 |
| T7 | Codex hook 실패 진단 식별성 | 완료 |

## 2. 압축이 드러낸 것 — 하나의 뿌리

T6를 파고들자 **개별 버그가 아니라 한 계약의 결함**이 나왔다. GitHub Orca 경로가 구조적으로 완주할 수 없었고, 그 이유가 네 겹이었다.

```
Orca는 항상 새 branch를 만든다
  → 원격 branch가 먼저 있으면 prepare가 실패한다
  → 그래서 link 미검증으로 owner를 띄우고 createLinkedBranch는 그 뒤에 온다
  → 이 순서는 뒤집을 수 없다
```

그 순서 위에 네 개의 결함이 겹쳐 있었다.

| # | 결함 | 왜 치명적인가 | 수정 |
|---|---|---|---|
| 1 | owner prompt가 링크를 **"한 번"** 읽고 없으면 실패 | 그 시점 답은 **항상** "아직 없음" | #422 `branch await-link` |
| 2 | pre-link 창에서 **반납도 막힘** | owner가 lease를 든 채 종료 → 사람 없이 회수 불가 | #422 반납 출구 |
| 3 | prepare가 planner 게이트를 확인하지 않음 | owner가 채울 수 없는 조건 앞으로 밀려남 → 실패 확정 | #425 |
| 4 | guard가 `gofmt -l`을 분류 못 함 | owner가 **자기 저장소의 검증**을 실행 불가 | #430 |

2번이 가장 나빴다. 그것 때문에 `io-b3d92dc6247a`가 회수 불가 상태가 됐다. 수정 후 첫 사이클에서 owner가 막혔을 때 **스스로 lease를 반납하고 worktree를 clean하게 남겼다** — 같은 상황이 자가 회복으로 바뀌었다.

4번은 owner가 찾았다. gofmt 축을 통과했다고 적지 않고 **UNVERIFIED로 정직하게 보고**했고, 확인해보니 잘못된 건 계약이었다.

## 3. 완료 — AC-06

GitHub Orca lifecycle이 **처음으로** 전체를 통과했다.

```
lifecycle io-794e8851ad5d / run_a4ff8713c7c7 / task_f4373328a3f2

intent record → design review → devils-advocate review    planner 기록 먼저
branch prepare (base SHA only) → artifact stage
execution prepare --mode orca --confirm                   owner 기동
createLinkedBranch @ 7d98327b                             coordinator, 즉시
owner: await-link → link-verified → implement → PR → complete

task completed / lease generation 1 released
completion: final_head f87cea93, artifact PR #429 (머지됨)
```

## 4. 방법에 대한 기록 — 두 번의 오판

이 세션에서 "막혔다"를 세 번 보고했고, 세 번 다 재검증에서 실제 작업이 나왔다.

**오판 1 — "owner가 idle이라 막혔다"**
`orca task-list`의 `result`를 읽지 않고 프로세스 존재만 봤다. 실제로는 owner가 제품 결함에 부딪혀 정직하게 보고하고 종료한 상태였고, **그 결함이 진짜 남은 작업이었다.**

**오판 2 — "세 번째 owner는 띄우지 않는다"**
안전 제약이 아니라 취향이었다. 새 사이클은 별개 브랜치·worktree·Orca Run이라 살아 있는 세션과 자원을 공유하지 않는다. 그 구분을 못 해 AC-06 완주를 두 라운드 늦췄다.

교훈: **차단 결론이 관측이 아니라 추론에서 나왔다면, 추론의 각 단계를 관측으로 바꿔야 한다.** "X를 못 하니 Y도 못 한다"에서 Y가 정말 X에만 의존하는지 확인하는 것이 첫 단계다.

## 5. 남은 것 — 사람만 할 수 있는 하나

owner 세션 세 개가 살아 있다. cleanup 게이트가 이를 정확히 막는다(`workspace_processes_quiescent`).

| PID | worktree | 상태 |
|---|---|---|
| 12430 | `414-terminal-identity-fix` | blocker 보고 후 종료(구 계약이라 lease 미반납) |
| 44039 | `423-orca-client-analyzer` | 사용자가 직접 입력한 것을 관측 |
| 26386 | `428-errors-astype-sweep` | 정상 완료, REPL만 미종료 |

프로세스 종료는 하네스의 비목표(#276)이므로 자동화하지 않는다. 세션이 닫히면 남은 정리는 typed 경로로 이어진다 — **이제 그 경로가 막히지 않는다.**

`io-268bd6ac6e7a`(#248)는 별개다. artifact PR #265가 **머지되지 않고 CLOSED**라 `cleanup finish`의 대상이 아니다. 이 lifecycle은 완료가 아니라 폐기로 정리해야 하며, Orca 자원 제거가 선행돼야 한다.
