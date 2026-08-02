package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

type settleCall struct {
	runID  string
	taskID string
}

type fakeTaskSettler struct {
	calls []settleCall
	err   error
}

func (f *fakeTaskSettler) settle(_ context.Context, runID, taskID string) error {
	f.calls = append(f.calls, settleCall{runID: runID, taskID: taskID})
	return f.err
}

// orca 모드 사이클이 정상 완료되면 그 task가 terminal 상태로 종결된다.
// 종결하지 않으면 레코드 삭제 후 소유자 조회가 영구히 0건이 되어
// operational_task_residue로 계속 보고된다(#130 AC-01).
func TestExecutionCompleteSettlesOrcaTask(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newOrcaCompletionFixture(t, stateRoot, "130-orca-settle")
	settler := &fakeTaskSettler{}

	completed, err := CompleteExecution(stateRoot, orcaCompletionRequest(t, fixture), ExecutionCompleteDeps{
		SettleOrcaTask: settler.settle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(settler.calls) != 1 || settler.calls[0].runID != "run-130" || settler.calls[0].taskID != "task-130" {
		t.Fatalf("completion must settle the bound orca task exactly once: %+v", settler.calls)
	}
	if !completed.OrcaTaskSettled || completed.OrcaTaskError != "" {
		t.Fatalf("a successful settlement must be reported without error: settled=%v err=%q",
			completed.OrcaTaskSettled, completed.OrcaTaskError)
	}
}

// 종결이 실패해도 완료 경로 자체는 진행된다. orca 상태 갱신은 best-effort이며
// durable state의 진실성을 훼손해서는 안 된다(#130 AC-02).
func TestExecutionCompleteSurvivesOrcaSettlementFailure(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newOrcaCompletionFixture(t, stateRoot, "130-orca-settle-fail")
	settler := &fakeTaskSettler{err: fmt.Errorf("orca runtime unreachable")}

	completed, err := CompleteExecution(stateRoot, orcaCompletionRequest(t, fixture), ExecutionCompleteDeps{
		SettleOrcaTask: settler.settle,
	})
	if err != nil {
		t.Fatalf("a failed settlement must not fail the completion: %v", err)
	}
	if completed.OrcaTaskSettled {
		t.Fatal("a failed settlement must not be reported as settled")
	}
	if completed.OrcaTaskError == "" {
		t.Fatal("a failed settlement must surface its cause; a silent failure is undiagnosable")
	}
	persisted, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != IssueOpsPhaseDone || persisted.Execution.Completion == nil {
		t.Fatalf("the durable completion must stand on its own: phase=%s completion=%#v",
			persisted.Phase, persisted.Execution.Completion)
	}
}

// direct 모드는 영향이 없다. 종결할 orca task 자체가 없다(#130 AC-03).
func TestExecutionCompleteDoesNotSettleForDirectMode(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "130-direct")
	prepareExecutionCompletionFixture(t, stateRoot, &fixture)
	actor := executionActor("codex", "direct-settle-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	settler := &fakeTaskSettler{}

	completed, err := CompleteExecution(stateRoot, ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree,
		FinalHead:         preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"),
		TuringReportPath:  writeCompletionReport(t, fixture.worktree),
		Verification:      []string{"go test ./... -count=1"},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}, ExecutionCompleteDeps{SettleOrcaTask: settler.settle})
	if err != nil {
		t.Fatal(err)
	}
	if len(settler.calls) != 0 {
		t.Fatalf("direct mode has no orca task to settle: %+v", settler.calls)
	}
	if completed.OrcaTaskSettled || completed.OrcaTaskError != "" {
		t.Fatalf("direct mode must report no settlement at all: settled=%v err=%q",
			completed.OrcaTaskSettled, completed.OrcaTaskError)
	}
}

// 주입점이 없으면 종결을 건너뛴다. 종결 수단이 없다는 사실이 완료를 막아서는
// 안 된다 — orca 상태 갱신은 durable state의 전제 조건이 아니다.
func TestExecutionCompleteWithoutSettlerStillCompletes(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newOrcaCompletionFixture(t, stateRoot, "130-orca-no-settler")

	completed, err := CompleteExecution(stateRoot, orcaCompletionRequest(t, fixture), ExecutionCompleteDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if completed.OrcaTaskSettled {
		t.Fatal("no settler means no settlement")
	}
	if completed.Execution.Completion == nil {
		t.Fatal("the completion receipt must still be persisted")
	}
}

func orcaCompletionRequest(t *testing.T, fixture claimableExecutionFixture) ExecutionCompleteRequest {
	t.Helper()
	return ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1,
		Actor:            orcaCompletionActor(),
		CWD:              fixture.worktree,
		FinalHead:        preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"),
		TuringReportPath: writeCompletionReport(t, fixture.worktree),
		Verification:     []string{"go test ./... -count=1"},

		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}
}

func orcaCompletionActor() issueops.NativeActor {
	return executionActor("claude", "orca-settle-session")
}

func writeCompletionReport(t *testing.T, worktree string) string {
	t.Helper()
	report := filepath.Join(worktree, ".agent-harness", "turing", "report.json")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return report
}

// newOrcaCompletionFixture는 완료 직전 상태의 orca 모드 사이클을 만든다.
// claim 경로는 sealed packet digest를 요구하므로 lease를 직접 active로 써서
// 이 테스트가 검증하려는 지점(종결 호출)만 남긴다.
func newOrcaCompletionFixture(t *testing.T, stateRoot, branch string) claimableExecutionFixture {
	t.Helper()
	fixture := newClaimableExecutionFixture(t, stateRoot, branch)
	prepareExecutionCompletionFixture(t, stateRoot, &fixture)
	record, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	actor := orcaCompletionActor()
	record.Execution.Mode = issueops.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &issueops.OrcaBinding{
		RuntimeID: "runtime-130", RepoID: "repo-130",
		WorktreeID: "worktree-130", TerminalPTYID: "pty-130",
		OwnerHost: "claude", OwnerModel: "claude-opus-5",
		RunID: "run-130", TaskID: "task-130", DispatchID: "dispatch-130",
	}
	record.Execution.Lease = issueops.WriteLease{
		Generation: 1, Status: issueops.LeaseStatusActive,
		Holder:    &actor,
		ClaimedAt: "2026-07-25T00:00:00Z",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	fixture.record = record
	return fixture
}
