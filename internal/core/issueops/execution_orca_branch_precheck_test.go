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

// createLocalBranch는 로컬 브랜치를 만들어 둔 상태를 재현한다. 이미 체크아웃해
// 작업하던 브랜치로 돌아온 경우가 여기에 해당한다.
func createLocalBranch(t *testing.T, repo, branch string) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(repo, "branch", branch); code != 0 {
		t.Fatalf("create local branch fixture: %s", stderr)
	}
}

// orcaPrepareRecord는 Orca 경로가 **실제로 진행할 수 있는** 준비 상태를 만든다.
//
// 기본 픽스처(executionPrepareRecord)는 `refs/remotes/origin/<branch>`를 만들어
// 두는데, 그것이 IssueOps 정식 순서(`gh issue develop` → `branch prepare`)를 따랐을
// 때의 실제 상태이기 때문이다. 그런데 Orca는 언제나 새 브랜치를 만들므로 그 이름이
// 원격에 있으면 접미사를 붙인다 — 즉 **정식 순서를 따르면 Orca 모드를 쓸 수 없다.**
//
// 그 충돌 자체는 #152가 다룬다. Orca 경로의 다른 계약(intent 봉인, CAS, reconcile)을
// 검증하는 테스트는 이름이 비어 있는 상태를 전제해야 하므로 여기서 그 ref를 지운다.
func orcaPrepareRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := executionPrepareRecord(t)
	if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref", "-d", "refs/remotes/origin/"+record.Branch); code != 0 {
		t.Fatalf("drop the remote fixture ref: %s", stderr)
	}
	return stateRoot, record
}

// createRemoteOnlyBranch는 IssueOps 정식 순서를 따랐을 때의 실제 상태를 재현한다.
//
// `gh issue develop`은 provider가 이슈에 연결할 브랜치를 **원격에** 만들고, 로컬
// refs/heads에는 아무것도 만들지 않는다. #149의 사전 확인은 로컬만 봤기 때문에
// 이 경우를 통과시켰고, 실환경에서 Orca가 접미사 브랜치를 만들었다(#154).
func createRemoteOnlyBranch(t *testing.T, repo, branch string) {
	t.Helper()
	head := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	if code, _, stderr := preflight.GitCmd(repo, "update-ref", "refs/remotes/origin/"+branch, head); code != 0 {
		t.Fatalf("create remote-only branch fixture: %s", stderr)
	}
	if code, _, _ := preflight.GitCmd(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); code == 0 {
		t.Fatal("이 픽스처는 로컬 브랜치가 없는 상태를 재현해야 한다")
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

// 로컬에도 원격에도 브랜치가 없어야 Orca 경로가 진행한다. 이 검사가 그
// 정상 경로를 막아서는 안 된다.
func TestOrcaPrepareProceedsWhenTheNameIsFreeEverywhere(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	// 기본 픽스처는 원격 브랜치를 만들어 둔다 — 그것이 IssueOps 정식 순서의
	// 상태이기 때문이다. 이 테스트는 이름이 어디에도 없는 경우를 보므로
	// 그 ref를 지운다.
	if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref", "-d", "refs/remotes/origin/"+record.Branch); code != 0 {
		t.Fatalf("drop the remote fixture ref: %s", stderr)
	}
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

// #149의 사전 확인은 로컬 refs만 봤다. 그런데 `gh issue develop`은 원격에만
// 브랜치를 만들므로, IssueOps 정식 순서를 따르면 검사가 **언제나** 통과했다.
// 실환경 dogfood에서 Orca가 `154-blocking-diagnostics-2`를 만들었고 pending
// intent와 워크트리가 남아 수동 회수를 요구했다(#154).
//
// Orca는 원격 브랜치를 본다. 사전 확인의 시야가 Orca의 시야와 같아야 한다.
func TestOrcaPrepareRejectsRemoteOnlyBranchBeforeMutation(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createRemoteOnlyBranch(t, record.Repo, record.Branch)
	orca := readyOrcaFake()

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, ReadIssue: executionIssueSnapshotReader})

	if err == nil {
		t.Fatal("원격 전용 브랜치도 Orca 이름 충돌을 만든다. 로컬만 보는 검사는 정식 순서를 통과시킨다")
	}
	if !strings.Contains(err.Error(), record.Branch) {
		t.Fatalf("오류 %q가 충돌 브랜치를 지목해야 한다", err)
	}
	if orca.prepareCalls != 0 {
		t.Fatalf("사전 확인은 외부 mutation 이전에 끝나야 한다: prepareCalls=%d", orca.prepareCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution != nil && persisted.Execution.Pending != nil {
		t.Fatalf("차단된 사전 확인은 pending intent를 남기지 않는다: %#v", persisted.Execution.Pending)
	}
}

// 원격 전용 충돌에서도 메시지가 다음 행동을 지시한다. 로컬 브랜치가 없으므로
// "로컬 브랜치를 지워라"는 안내는 이 경우에 쓸모가 없다.
func TestOrcaRemoteOnlyConflictExplainsWhereTheBranchLives(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	createRemoteOnlyBranch(t, record.Repo, record.Branch)

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "precheck-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), ReadIssue: executionIssueSnapshotReader})
	if err == nil {
		t.Fatal("expected the remote-only branch to block prepare")
	}
	for _, want := range []string{"remote", "direct"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("오류 %q가 %s를 언급해야 운영자가 어디를 봐야 할지 안다", err, want)
		}
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
