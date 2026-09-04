# 149 — orca 워크스페이스 생성 전 브랜치 충돌 사전 확인

이슈: https://github.com/m16khb-org/issueops/issues/149
사이클: io-aa325be0c36b
브랜치: `149-orca-branch-precheck` (base `main` @ 6e693e9)

## 문제

IssueOps는 "linked branch first"를 요구해 `gh issue develop`으로 브랜치를 먼저 만든다.
그 다음 orca 모드 `execution prepare`가 `orca worktree create`로 **같은 이름 브랜치를
새로 만들려다** 충돌해 orca가 `-2`를 붙인다.

```
worktree_branch_mismatch: created branch "147-dispatch-structural-signals-2"
does not match requested branch "147-dispatch-structural-signals"
```

실패가 `Invoked: true`라 pending intent와 orca 워크트리가 남고, **수동 회수 없이는
abandon도 막힌다**(실측: `pending_intent_safe`가 "Orca worktree receipt does not match
the canonical workspace identity"로 차단).

## 채택은 불가능하다

`AdoptWorktree`는 `orca worktree set` — 이미 orca가 관리하는 워크트리의 메타데이터를
갱신할 뿐이다. orca CLI에 채택 명령이 없고, `worktree create`에도 기존 브랜치를
체크아웃하는 옵션이 없다(`--base-branch`는 시작 ref, `--name`이 새 브랜치).

`CanonicalizeWorktreeBranch`(client.go:298)는 `<namespace>/<branch>` 형태만 복구하고
`<branch>-N`은 처리하지 않는다.

## 변경

core의 orca 경로가 `worktree create` 이전에 대상 브랜치의 **로컬 존재**를 확인한다.
이미 있으면 mutation을 시도하지 않고 실패하며, 메시지가 원인과 다음 행동을 지시한다.

direct 모드는 검사하지 않는다 — 기존 브랜치 사용이 정상 경로다.

## 비범위

- **채택**: orca CLI에 수단이 없다.
- **접미사 정규화**: orca 메타데이터와 git 상태의 사후 보정이며 orca 규칙 변경에 취약하다.
- **IssueOps 순서 변경**: 계약 근간이라 별도 결정이 필요하다. 이 사전 확인의 실패
  메시지가 그 필요성을 드러내게 한다.
- **원격 전용 브랜치**: orca 동작을 확인하지 못했다. 로컬 기준으로 검사하고 범위를
  주석에 남긴다.

## 구현

`ensureOrcaBranchIsFree`(execution_prepare.go)를 `prepareOrcaExecution`의 confirm 직후,
`beginOrcaExecutionIntent` 이전에 둔다. `git rev-parse --verify --quiet refs/heads/<branch>`가
성공하면 mutation 없이 실패한다.

재진입은 이 경로에 닿지 않는다 — `PrepareExecution`이 `record.Execution != nil`이면
81-89행에서 먼저 반환하므로, orca 경로는 언제나 최초 생성이고 기존 브랜치는 늘 충돌이다.

## 검증

```bash
go test ./internal/core/issueops/ -run "OrcaPrepare|DirectPrepareStillAllows" -count=1
go test ./... -count=1
```

RED → GREEN 확인:

- RED: 차단 두 건만 실패(`prepareCalls=1`, 즉 mutation이 일어남).
  preview·정상경로·direct 세 건은 그때도 통과 — 이 검사가 막지 말아야 할 것들이다.
- GREEN: 다섯 건 모두 통과.
- 전체 회귀 `go test ./... -count=1` 통과.

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
