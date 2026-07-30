# IssueOps Archived Child Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 이미 정리된 자식 cycle을 부모의 기존 ref와 명시적 증거로 안전하게 사후 승인한다.

**Architecture:** 정상 자식 검증 경로는 그대로 유지한다. `fs.ErrNotExist`일 때만 부모 record의 기존 child ref를 잠금 안에서 찾아 승인 영수증을 갱신하며, ref가 없으면 fail-closed한다.

**Tech Stack:** Go 1.26, IssueOps SQLite state store, Go 표준 `errors`/`io/fs`, `testing`

## Global Constraints

- 자식 record가 존재하면 기존 `phase=done` 검증을 우회하지 않는다.
- 삭제된 record의 cycle ID는 부모 `ChildCycles`에 이미 존재해야 한다.
- validation evidence는 비어 있을 수 없다.
- 새 child ref나 삭제된 child record를 생성하지 않는다.
- `reject`와 `drop` 동작은 변경하지 않는다.

---

### Task 1: 정리된 자식 사후 승인

**Files:**
- Modify: `internal/core/issueops/issueops_delegation.go`
- Test: `internal/core/issueops/issueops_delegation_status_test.go`

**Interfaces:**
- Consumes: `AcceptIssueOpsChildWithActor(stateRoot, parentID, childID, evidence, actor)`
- Produces: 삭제된 child record에 대해 기존 parent ref만 갱신하는 사후 승인 동작

- [ ] **Step 1: 실패하는 회귀 테스트 작성**

```go
func TestAcceptIssueOpsChildAfterCleanupRequiresIndexedParentRef(t *testing.T) {
    // done child를 삭제한 뒤 parent ref와 evidence로 accept가 성공해야 한다.
    // 부모에 없는 missing cycle ID는 evidence가 있어도 실패해야 한다.
}
```

- [ ] **Step 2: 테스트가 현재 결함으로 실패하는지 확인**

Run:

```bash
go test ./internal/core/issueops -run TestAcceptIssueOpsChildAfterCleanupRequiresIndexedParentRef -count=1
```

Expected: `issueops record ...: file does not exist` 때문에 FAIL.

- [ ] **Step 3: 최소 구현 추가**

```go
if errors.Is(err, fs.ErrNotExist) {
    return acceptArchivedIssueOpsChild(stateRoot, parentID, childID, evidence, actor)
}
```

`acceptArchivedIssueOpsChild`는 부모 lock 안에서 actor를 검증하고 기존 ref만
찾아 `accepted`, evidence, timestamp를 기록한다. ref가 없으면
`child_not_indexed` 오류를 반환한다.

- [ ] **Step 4: 관련 테스트 통과 확인**

Run:

```bash
go test ./internal/core/issueops -run 'TestAcceptIssueOpsChild(RequiresDonePhaseAndEvidence|AfterCleanupRequiresIndexedParentRef)' -count=1
go test ./cmd/harness/issueopscli -run 'TestIssueOpsChild' -count=1
```

Expected: 모두 PASS.

- [ ] **Step 5: 실제 #191 복구 명령으로 통합 확인**

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
agent-harness issueops child accept --parent io-06edaddc2980 --child io-5bdee93886b2 --evidence '검증된 병합 및 정리 증거' --json
```

Expected: `ok=true`, `validation_verdict=accepted`.

- [ ] **Step 6: 변경을 원자적으로 커밋하고 push**

`atomic-commit-push` 계약에 따라 설계, 계획, 회귀 테스트와 구현만 stage하고
관련 검증 결과를 Lore body에 기록한다.
