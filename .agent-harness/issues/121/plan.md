# 121 cleanup/완료 정합성 결함 2건 구현 계획

- 이슈: https://github.com/m16khb/agent-harness/issues/121
- 부모 백로그: https://github.com/m16khb/agent-harness/issues/99
- IssueOps: io-e398a19c16e6 / direct / generation 1
- 브랜치: 121-cleanup-integrity (base main, base head f625e3bc81600d8c8136a249b68cdfd699300097)

## 문제 요약

두 결함은 "정리 경로가 관측을 끝까지 책임지지 않는다"는 같은 실패 형태다.

1. operationalhealth 분류기의 task 루프가 상태와 무관하게 소유자를 요구한다(classifier 소스 257행). cleanup finish가 레코드를 삭제하면 소유자 조회는 영구히 0건이 되므로, 이미 종결된 task도 영원히 `operational_task_residue`로 보고된다. 실측 사례 `task_ca8cb55b687a`는 orca 전역 reset으로 수동 해소해야 했다.
2. cleanup finish의 머지 readback이 merged 여부와 head ref만 관측한다(remoteverify 검증 소스 88-95행). base ref를 보지 않으므로 done 이후 원격 PR base가 바뀌어도 파괴 전에 검출되지 않는다.

## 결함 1 설계: terminal task 소유자 면제

분류기 task 루프에서 `completed`/`failed` 상태를 소유자 요구에서 면제한다.

- `ready`/`dispatched`는 기존과 동일하게 소유자 필수(회귀 금지).
- `ready` + completion metadata 규칙(분류기 소스 253-255행)은 그대로 유지한다.
- 근거: orca CLI에 개별 task 삭제 명령이 없고(`orca orchestration --help` 실측: send/check/reply/inbox/task-create/task-list/task-update/dispatch/dispatch-show/ask/run/run-stop/gate-create/gate-resolve/gate-list/reset), 종결된 task는 워크트리·터미널과 달리 자원을 점유하지 않는 감사 흔적이다.

### RED 테스트

`internal/core/operationalhealth`에 소유자 없는 `completed` task와 `failed` task가 finding을 만들지 않음을 요구하는 테스트를 추가한다. 현재 코드에서는 두 건 모두 `operational_task_residue`가 나와 실패해야 한다.

동반으로 `ready`/`dispatched` task는 여전히 finding이 나오는 회귀 금지 테스트를 둔다.

## 결함 2 설계: base ref를 단일 readback에 편입

#116이 확립한 단일 readback 원칙(검증 소스 62-64행 주석: 두 값이 다른 시점의 관측이면 OID CAS가 무의미해진다)을 유지하며 base ref를 같은 관측에 싣는다.

1. `remoteverify`의 GitHub PR fetch에 `baseRefName`을, GitLab MR fetch에 target branch를 추가하고 `liveRemoteArtifact`에 `BaseRefName` 필드를 둔다.
2. `IssueOpsCleanupRemoteBranchArtifactHead`에 `BaseRefName`을 추가한다. 기존 OID CAS 로직은 이 필드를 읽지 않으므로 remote-branch 경로의 동작은 불변이다.
3. finish CLI는 `VerifyMerged` 대신 이미 존재하는 `VerifyMergedHead`를 사용해 중복 readback 없이 base를 얻고, `CleanupFinishRequest`의 새 필드로 전달한다.
4. `cleanupFinishGates`가 전달된 base와 `BranchPrepare.BaseBranch`를 비교해 불일치 시 missing 코드를 낸다. 값이 비면(관측 불가) 통과가 아니라 거부다.

### fingerprint 규율

`cleanupFinishInventory`(finish 소스 70-79행)는 fingerprint 입력이다. base 관측을 인벤토리에 넣으면 발급된 모든 preview fingerprint가 무효화되고, 네트워크 관측을 fingerprint에 섞으면 일시적 원격 오류가 preview 재발급 루프를 만든다. 따라서 `remote_branch_absent`가 확립한 "관측하되 fingerprint 입력에서 제외" 규율을 따른다.

### RED 테스트

`internal/core/issueops`에 base 불일치 요청이 missing 코드로 차단되는 테스트와, 빈 base 관측이 fail-closed로 거부되는 테스트를 추가한다. 일치하는 경우 기존대로 통과함을 확인하는 테스트도 둔다.

## 수용 기준 매핑

| AC | 검증 |
| --- | --- |
| AC-01 completed/failed task 면제 | operationalhealth 신규 테스트 |
| AC-02 ready/dispatched 회귀 금지 | operationalhealth 회귀 테스트 |
| AC-03 base drift fail-closed | issueops finish 게이트 신규 테스트 |
| AC-04 RED 선행 | 각 테스트를 구현 전 실행해 실패를 확인한 뒤 GREEN 전환 |

## 검증 명령

```bash
go -C /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity test /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity/internal/core/operationalhealth
go -C /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity test /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity/internal/core/issueops
go -C /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity test /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity/cmd/harness/issueopscli/...
go -C /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity test /Users/m16khb/Workspace/agent-harness.worktrees/121-cleanup-integrity/...
```

## 비범위

- raw orca 메시지 명령을 훅 가드 allowlist에 편입하는 것. owner 명령 카탈로그는 execution status/claim, remote create-pr, execution complete, implementation-review record 5개만 정의하며 orca 메시지 전송을 요구하지 않는다.
- core 배선이 없는 dead adapter 코드(`UpdateTask`, `SendWorkerDone`) 삭제. 별도 판단 항목.
- sealed packet digest 수명주기(재봉인 typed 경로, 이슈 편집 선제 차단, claim drift 진단). 후속 사이클.
