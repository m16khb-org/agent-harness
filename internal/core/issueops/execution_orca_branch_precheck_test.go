package issueops

import (
	"context"
	"os"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

// readyOrcaFake는 성공하는 Orca를 흉내낸다. 워크스페이스 루트를 실제로
// 만드는 것까지가 성공의 정의다 — core가 그 뒤에 그 경로를 쓴다.
func readyOrcaFake() *executionOrcaFake {
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}
	return fake
}

// createLocalBranch는 provider-linked 브랜치가 이미 만들어진 상태를 재현한다.
// IssueOps는 `gh issue develop`으로 브랜치를 먼저 만들도록 요구하므로 이것이
// 정식 순서를 따랐을 때의 실제 상태다.
func createLocalBranch(t *testing.T, repo, branch string) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(repo, "branch", branch); code != 0 {
		t.Fatalf("create local branch fixture: %s", stderr)
	}
}

// Orca는 언제나 새 브랜치를 만든다. 대상 브랜치가 이미 있으면 Orca가 접미사를
// 붙여 만들고 agent-harness가 그것을 worktree_branch_mismatch로 거부하는데, 그
// 실패는 Invoked라 pending intent와 Orca 워크트리를 남긴다. 실측에서 수동 회수
// 없이는 abandon도 막혔다(#149).
//
// mutation 이전에 막으면 잔여물이 생기지 않는다.
func TestOrcaPrepareRejectsExistingBranchBeforeMutation(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createLocalBranch(t, record.Repo, record.Branch)
	orca := readyOrcaFake()

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, ReadIssue: executionIssueSnapshotReader})

	if err == nil {
		t.Fatal("an existing branch must block Orca prepare: Orca would create a suffixed branch instead")
	}
	if !strings.Contains(err.Error(), record.Branch) {
		t.Fatalf("the error %q must name the conflicting branch", err)
	}
	if orca.prepareCalls != 0 {
		t.Fatalf("the precheck must run before any Orca mutation: prepareCalls=%d", orca.prepareCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution != nil && persisted.Execution.Pending != nil {
		t.Fatalf("a blocked precheck must not leave a pending intent: %#v", persisted.Execution.Pending)
	}
}

// 오류는 원인과 다음 행동을 알려야 한다. 운영자는 왜 막혔는지와 무엇을 할지
// 알아야 하며, Orca가 무엇을 했는지 추측하게 두어서는 안 된다.
func TestOrcaPrepareBranchConflictExplainsTheCause(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createLocalBranch(t, record.Repo, record.Branch)

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), ReadIssue: executionIssueSnapshotReader})

	if err == nil {
		t.Fatal("expected the branch conflict to block prepare")
	}
	for _, want := range []string{"Orca", "direct"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the error %q must mention %s so the operator knows the cause and the way forward", err, want)
		}
	}
}

// preview는 mutation이 아니므로 막지 않는다. 운영자가 무엇이 일어날지 볼 수
// 있어야 하고, 그 시점에는 브랜치 충돌이 아직 손해를 만들지 않는다.
func TestOrcaPreparePreviewSurvivesExistingBranch(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createLocalBranch(t, record.Repo, record.Branch)

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("preview must still describe the planned workspace: %v", err)
	}
	if !result.Preview || result.ResolvedMode != "orca" {
		t.Fatalf("unexpected preview result: %+v", result)
	}
}

// 브랜치가 없으면 Orca 경로는 종전대로 진행한다. 이 검사가 정상 경로를
// 막아서는 안 된다.
func TestOrcaPrepareProceedsWithoutExistingBranch(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	orca := readyOrcaFake()

	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, ReadIssue: executionIssueSnapshotReader}); err != nil {
		t.Fatalf("a free branch name must not be blocked: %v", err)
	}
	if orca.prepareCalls != 1 {
		t.Fatalf("Orca must still be invoked once: prepareCalls=%d", orca.prepareCalls)
	}
}

// direct 모드는 기존 브랜치를 쓰는 것이 정상 경로다. 이 검사가 거기까지
// 번지면 지금 동작하는 모든 direct 사이클이 깨진다.
func TestDirectPrepareStillAllowsExistingBranch(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createLocalBranch(t, record.Repo, record.Branch)

	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", CWD: record.Repo,
		Actor: executionActor("codex", "direct-session"),
	}, ExecutionPrepareDependencies{Direct: gitworktree.New()}); err != nil {
		t.Fatalf("direct preview must not be blocked by an existing branch: %v", err)
	}
}
