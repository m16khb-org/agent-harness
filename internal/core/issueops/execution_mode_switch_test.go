package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/issueops/model"
)

// prepare는 이미 준비된 실행을 다시 준비하지 않는다 — 멱등성이다. 그런데 그
// 분기(execution_prepare.go)가 요청 모드를 보지 않아서, 다른 모드를 달라는 요청도
// 같은 취급을 받는다. ok true와 함께 요청하지 않은 모드가 돌아오고 fallback_code도
// 비어 있다. 폴백이 아니라 요청이 평가조차 되지 않았기 때문이다(이슈 #167).
func TestPrepareRejectsExplicitModeThatDiffersFromThePreparedOne(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: false,
		Actor: executionActor("claude", "switch-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err == nil {
		t.Fatalf("요청하지 않은 모드를 성공으로 돌려주면 사용자는 orca로 실행됐다고 믿는다: %+v", result)
	}
	if result.OK {
		t.Fatalf("모드 불일치는 성공이 아니다: %+v", result)
	}
	if strings.TrimSpace(result.NextCommand) == "" {
		t.Fatalf("거부만 하고 해소 경로를 주지 않으면 사용자가 갇힌다: %+v", result)
	}
	if !strings.Contains(result.NextCommand, "switch-mode") {
		t.Fatalf("다음 명령 %q가 전환 경로를 지목해야 한다", result.NextCommand)
	}
}

// auto는 실행 가능한 모드를 고르는 요청이지 특정 모드를 요구하는 것이 아니다.
// 이미 준비된 실행이 있으면 그것이 실행 가능한 모드이므로 그대로 받아들인다.
// 이 경로까지 거부하면 prepare 멱등성이 깨진다.
func TestPrepareAcceptsAutoAgainstAPreparedExecution(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: false,
		Actor: executionActor("claude", "switch-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("auto는 준비된 모드를 받아들여야 한다: %v", err)
	}
	if !result.OK || result.ResolvedMode != string(model.ExecutionModeDirect) {
		t.Fatalf("auto가 준비된 direct를 그대로 돌려줘야 한다: %+v", result)
	}
}

// 같은 모드를 다시 요청하는 것은 멱등 호출이다. 불일치 검사가 이것까지 잡으면
// 재시도가 실패로 바뀐다.
func TestPrepareAcceptsTheSameExplicitModeAgain(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: false,
		Actor: executionActor("claude", "switch-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("같은 모드 재요청은 멱등이어야 한다: %v", err)
	}
	if !result.OK {
		t.Fatalf("같은 모드 재요청이 실패하면 재시도가 불가능해진다: %+v", result)
	}
}

// 전환은 워크트리와 로컬 브랜치를 지운다. 살아 있는 writer가 그것을 밟고 있으면
// 무엇을 지우는지 알 수 없다. 판정 기준은 상태 이름이 아니라 writer의 유무이며
// cleanupAbandonLeaseHoldsWriter와 같은 기준을 쓴다.
func TestSwitchModeRefusesWhileALeaseHoldsAWriter(t *testing.T) {
	for _, status := range []model.LeaseStatus{model.LeaseStatusActive, model.LeaseStatusRevoking} {
		t.Run(string(status), func(t *testing.T) {
			stateRoot, record := preparedDirectExecutionRecord(t, status)

			result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
				ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
				Actor: executionActor("claude", "switch-session"),
			}, ExecutionSwitchModeDependencies{})
			if err == nil {
				t.Fatalf("writer를 가진 lease에서 전환하면 살아 있는 작업을 밟는다: %+v", result)
			}
			if !containsString(result.Missing, "lease_holds_no_writer") {
				t.Fatalf("어떤 게이트가 막았는지 나와야 한다: %+v", result.Missing)
			}
		})
	}
}

// 같은 모드로의 전환은 지울 이유가 없다. 파괴 조작이 아무 일도 하지 않는 것보다
// 요청 자체를 거부하는 편이 안전하다.
func TestSwitchModeRefusesTheSameMode(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)

	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "direct", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err == nil {
		t.Fatalf("같은 모드 전환은 지울 이유가 없다: %+v", result)
	}
	if !containsString(result.Missing, "mode_actually_changes") {
		t.Fatalf("어떤 게이트가 막았는지 나와야 한다: %+v", result.Missing)
	}
}

// preview는 무엇이 지워지는지 보여주고 fingerprint를 준다. 그것 없이 apply를
// 허용하면 사용자가 확인하지 않은 상태를 지우게 된다(cleanup abandon과 같은 계약).
func TestSwitchModePreviewNamesWhatItWouldRemove(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)
	dropRemoteBranchFixture(t, record.Repo, record.Branch)

	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatalf("preview는 게이트를 통과하면 성공해야 한다: %v", err)
	}
	if !result.Preview {
		t.Fatalf("apply 없이 부르면 preview여야 한다: %+v", result)
	}
	if strings.TrimSpace(result.Fingerprint) == "" {
		t.Fatalf("fingerprint 없이는 TOCTOU를 막을 수 없다: %+v", result)
	}
	if strings.TrimSpace(result.WorktreeRoot) == "" {
		t.Fatalf("지워질 워크트리를 이름으로 보여줘야 한다: %+v", result)
	}
	if !strings.Contains(result.NextCommand, "--apply") {
		t.Fatalf("다음 명령 %q가 apply 단계를 지목해야 한다", result.NextCommand)
	}
}

// preview 이후 상태가 바뀌었다면 그 preview는 다른 상태를 승인한 것이다.
func TestSwitchModeApplyRejectsAStaleFingerprint(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)
	dropRemoteBranchFixture(t, record.Repo, record.Branch)

	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Apply: true, Confirm: true,
		Fingerprint: "0000000000000000000000000000000000000000000000000000000000000000",
		Actor:       executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err == nil {
		t.Fatalf("낡은 fingerprint로 지우면 확인하지 않은 상태를 지운다: %+v", result)
	}
}

// --apply만으로 워크트리가 사라져서는 안 된다. cleanup abandon과 같은 3단이다.
func TestSwitchModeApplyRequiresConfirm(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)
	dropRemoteBranchFixture(t, record.Repo, record.Branch)

	preview, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Apply: true,
		Fingerprint: preview.Fingerprint, Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err == nil {
		t.Fatalf("--confirm 없이 파괴 조작을 실행하면 3단 확인이 무의미하다: %+v", result)
	}
}

// 전환이 성공하면 execution record가 새 모드로 교체되고 워크스페이스 링크가
// 사라진다. 그래야 다음 prepare가 새 모드로 워크스페이스를 만든다.
func TestSwitchModeApplyReplacesTheExecutionRecord(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)
	dropRemoteBranchFixture(t, record.Repo, record.Branch)

	preview, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Apply: true, Confirm: true,
		Fingerprint: preview.Fingerprint, Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatalf("게이트를 통과한 전환은 성공해야 한다: %v", err)
	}
	if !result.OK || result.Preview {
		t.Fatalf("apply 결과가 확정이어야 한다: %+v", result)
	}
	after, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Execution != nil {
		t.Fatalf("execution record가 남아 있으면 다음 prepare가 다시 같은 모드로 막힌다: %+v", after.Execution)
	}
	if strings.TrimSpace(after.WorktreePath) != "" {
		t.Fatalf("워크트리 링크가 남아 있으면 새 모드의 워크스페이스와 어긋난다: %q", after.WorktreePath)
	}
}

// Orca는 언제나 새 브랜치를 만들고 이름이 쓰이고 있으면 접미사를 붙인다
// (#149·#154). 그 상태를 통과시키면 전환은 성공하는데 바로 다음 prepare가
// 막힌다 — 워크트리만 잃고 제자리다. IssueOps 정식 순서(`gh issue develop` →
// `branch prepare`)를 따르면 원격에 이름이 있으므로 이것이 기본 상태다.
func TestSwitchModeToOrcaRefusesWhileTheBranchNameIsTaken(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)

	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err == nil {
		t.Fatalf("이름이 쓰이고 있으면 전환해봐야 다음 prepare가 막힌다: %+v", result)
	}
	if !containsString(result.Missing, "orca_branch_name_free") {
		t.Fatalf("어떤 게이트가 막았는지 나와야 한다: %+v", result.Missing)
	}
	if strings.TrimSpace(result.BranchFreeError) == "" {
		t.Fatalf("이름이 로컬에 있는지 원격에 있는지 알려줘야 한다: %+v", result)
	}
	after, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Execution == nil {
		t.Fatal("게이트에 막힌 전환이 record를 지우면 안 된다")
	}
}

// 전환 직후 prepare가 요청한 모드로 실행을 준비할 수 있어야 한다. 그러지 못하면
// 전환은 record만 지우고 사용자를 같은 자리에 남긴다.
func TestPrepareAfterSwitchModeUsesTheRequestedMode(t *testing.T) {
	stateRoot, record := preparedDirectExecutionRecord(t, model.LeaseStatusClaimable)
	dropRemoteBranchFixture(t, record.Repo, record.Branch)

	preview, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root,
		Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Apply: true, Confirm: true,
		Fingerprint: preview.Fingerprint, Actor: executionActor("claude", "switch-session"),
	}, ExecutionSwitchModeDependencies{}); err != nil {
		t.Fatal(err)
	}

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: false,
		Actor: executionActor("claude", "switch-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("전환 뒤 prepare가 막히면 전환이 아무것도 해결하지 못한다: %v", err)
	}
	if result.ResolvedMode != string(model.ExecutionModeOrca) {
		t.Fatalf("전환한 모드로 준비돼야 한다: %+v", result)
	}
}
