# 이슈 #102 — execution prepare canonical root 충돌 fail-closed 가드

이슈: https://github.com/m16khb/agent-harness/issues/102

## 문제

`executionWorkspaceRequest`(`internal/core/issueops/execution_prepare.go:298-312`)의 leaf 파생 `strings.ReplaceAll(branch, "/", "-")`이 비단사라 `72/fix`↔`72-fix`가 동일 root로 매핑되고, lifecycle ID는 branch로 해시되어 서로 다른 레코드가 된다 — 두 lifecycle이 같은 canonical worktree를 소유하는 불변식 위반. `PrepareExecution`(:56-110)에 레코드 간 root 충돌 검사가 없다.

## 변경 (brooks devil's-advocate revise 반영)

1. 가드 함수 `ensureExecutionRootUnclaimed(stateRoot, selfID, root)`: 전 레코드 스캔에서 **`record.WorktreePath`와 `Execution.Workspace.Root`의 합집합**을 CleanAbs 동치 비교(LinkWorktree만 설정된 Execution-부재 레코드도 root를 점유한다 — linking 경로는 cross-record 유일성 검증이 없음). **done/released 포함 전 레코드 무예외** — cleanup finish가 worktree 제거 후에야 레코드를 삭제하므로 레코드 존재가 곧 root 소유권. 자기 ID 제외. 리더는 prune 선례의 `readIssueOpsUnchecked`를 쓰되 **읽기 오류는 fail-closed**(손상 레코드의 무음 통과 금지). 오류에 선점 lifecycle ID·브랜치와 다음 행동(`issueops cleanup finish --id <선점ID>` 안내)을 에코.
2. 호출 3곳:
   - `PrepareExecution`의 workspaceReq 산출(:89) 직후 — preview UX용(락 밖, 멱등 재-prepare 경로는 :80-88에서 선반환되므로 자기충돌 없음).
   - `prepareDirectExecution`의 영속 콜백 내부 — `withIssueOpsLock`이 state-root 전역 span이므로 임계구역 내 재검사가 TOCTOU 봉합.
   - `beginOrcaExecutionIntent`의 영속 콜백 내부 — 기존 검사는 자기 ID의 Execution만 보므로 orca 경로의 레코드 수준 레이스는 이 재검사 없이는 미봉합(필수).
3. 인코딩·기존 레코드 하위 호환 불변.

## git 계층과의 역할 분담 (brooks 지적 명시)

confirm 시점의 물리 충돌 자체는 gitworktree `inspectExisting`(브랜치 불일치 거부)과 `git worktree add` 실패가 이미 fail-closed로 막는다. 이 가드의 고유 가치: **preview 시점 검출, 선점 lifecycle ID·브랜치 에코(불친절한 git 오류 대체), orca 경로 커버, "레코드는 root를 주장하나 디렉터리 부재" 케이스**.

## TDD 순서

1. RED: 브랜치 `72/fix`로 execution 준비된 레코드가 있는 상태에서 브랜치 `72-fix` 레코드의 prepare preview가 현재 충돌 없이 통과해버림을 재현 → 충돌 오류 기대 단언 실패.
2. GREEN: 가드+3 호출 지점 추가 → preview·confirm 거부(ID·브랜치·next action 에코), WorktreePath-만 설정된 레코드와의 충돌 거부, 같은 lifecycle 재-prepare 멱등·무충돌 브랜치 정상, 손상 레코드 읽기 오류 fail-closed.
3. 회귀: issueops 패키지·전체 모듈 green.

## 비범위

- leaf 인코딩 변경, branch prepare 명명 규칙, orphan cleanup(프룬이 남긴 고아 디렉터리는 git 계층이 confirm에서 차단 — preview false-pass는 수용, 위험 절 기록).

## 위험

- O(N) 레코드 스캔 — prepare 저빈도·executionGuardRecords 선례 규모로 수용.
- PruneIssueOps는 worktree 제거 없이 레코드만 삭제하므로 레코드 스캔이 못 보는 고아 디렉터리 가능 — confirm은 git 계층이 차단, preview만 false-pass(비범위 orphan cleanup과 정합).

## 역할 분담

- 계획·리뷰 설계: Fable 5 메인 세션. 구현안: Opus 5 서브에이전트(적용은 lease holder인 메인 세션).
