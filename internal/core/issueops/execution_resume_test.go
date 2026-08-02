package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

const resumeIssueBody = "## acceptance criteria\n\n- [ ] AC-01: resume owner\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"

func reseededOrcaCycle(t *testing.T) (string, issueops.IssueOpsRecord, ExecutionReplaceResult) {
	return reseededOrcaCycleWithArtifacts(t, nil)
}

func reseededOrcaCycleWithArtifacts(t *testing.T, artifacts map[string]string) (string, issueops.IssueOpsRecord, ExecutionReplaceResult) {
	t.Helper()
	stateRoot, original, _, reader := sealedOrcaCycleWithArtifacts(t, resumeIssueBody, artifacts)
	record, err := ReadIssueOps(stateRoot, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	deps := quiescentOrcaReplaceDeps(reader)
	actor := executionActor("codex", "resume-coordinator")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: record.Execution.Lease.Generation,
		Actor: actor, CWD: record.Execution.Workspace.Root,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	reseededResult, err := reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: record.Execution.Lease.Generation,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "resume test",
		Actor: actor, CWD: record.Execution.Workspace.Root, Confirm: true,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	reseeded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, reseeded, reseededResult
}

func resumeRequest(record issueops.IssueOpsRecord) ExecutionResumeRequest {
	return ExecutionResumeRequest{
		ID: record.ID, ExpectedGeneration: record.Execution.Lease.Generation,
		Actor: executionActor("codex", "resume-coordinator"),
		CWD:   record.Execution.Workspace.Root, Confirm: true,
	}
}

func resumeOrcaFake(t *testing.T, stages *[]port.ExecutionOrcaIntentStage) *executionOrcaFake {
	t.Helper()
	fake := &executionOrcaFake{}
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		*stages = append(*stages, request.Stage)
		switch request.Stage {
		case port.ExecutionOrcaIntentTerminal:
			return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-resume"}, nil
		case port.ExecutionOrcaIntentRun:
			return port.ExecutionOrcaIntentReceipt{RunID: "run-resume"}, nil
		case port.ExecutionOrcaIntentRunBind:
			return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}, nil
		case port.ExecutionOrcaIntentTask:
			return port.ExecutionOrcaIntentReceipt{TaskID: "task-resume"}, nil
		case port.ExecutionOrcaIntentDispatch:
			return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-resume"}, nil
		default:
			t.Fatalf("resume invoked unexpected stage %q", request.Stage)
			return port.ExecutionOrcaIntentReceipt{}, nil
		}
	}
	return fake
}

func TestExecutionResumeRejectsReleasedLeaseBeforeExternalMutation(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.Execution.Lease = issueops.WriteLease{
		Generation: record.Execution.Lease.Generation,
		Status:     issueops.LeaseStatusReleased,
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	var stages []port.ExecutionOrcaIntentStage
	_, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages), OrcaOwner: &executionOrcaOwnerInspectorFake{},
	})
	if err == nil || !strings.Contains(err.Error(), "claimable") {
		t.Fatalf("released resume error = %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("released resume mutated Orca: %v", stages)
	}
}

func TestExecutionReseedAndWriterAbsentPointToResume(t *testing.T) {
	stateRoot, record, reseeded := reseededOrcaCycle(t)
	want := executionResumeCommand(record.ID, record.Execution.Lease.Generation)
	if reseeded.NextCommand != want {
		t.Fatalf("reseed next command = %q, want %q", reseeded.NextCommand, want)
	}
	prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Confirm: true,
		Actor: executionActor("codex", "resume-coordinator"), OwnerHost: record.Execution.Orca.OwnerHost,
	}, ExecutionPrepareDependencies{})
	if err == nil {
		t.Fatalf("holderless Orca generation must not report prepare success: %#v", prepared)
	}
	if prepared.NextCommand != want || !strings.Contains(err.Error(), want) {
		t.Fatalf("writer-absent next command = %q error=%v", prepared.NextCommand, err)
	}
}

func TestExecutionWriterAbsentCurrentOrcaGenerationStillPointsToResume(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.Execution.Orca.LeaseGeneration = record.Execution.Lease.Generation
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	want := executionResumeCommand(record.ID, record.Execution.Lease.Generation)
	prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Execution.Workspace.Root, Confirm: true,
		Actor: executionActor("codex", "resume-coordinator"), OwnerHost: record.Execution.Orca.OwnerHost,
	}, ExecutionPrepareDependencies{})
	if err == nil {
		t.Fatalf("claimable Orca generation must not report prepare success: %#v", prepared)
	}
	if prepared.NextCommand != want || !strings.Contains(err.Error(), want) {
		t.Fatalf("current Orca generation next command = %q error=%v", prepared.NextCommand, err)
	}
}

func TestExecutionResumeRejectsPreviousLiveTaskFromAnotherGeneration(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	var stages []port.ExecutionOrcaIntentStage
	_, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages),
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			TerminalLive: true, TaskLive: true, TerminalID: record.Execution.Orca.TerminalPTYID,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "previous Orca owner task is still live") {
		t.Fatalf("live previous owner error = %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("live previous owner caused mutation: %v", stages)
	}
}

func TestExecutionResumeRejectsLiveTaskWithoutLiveTerminal(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.Execution.Orca.LeaseGeneration = record.Execution.Lease.Generation
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	var stages []port.ExecutionOrcaIntentStage
	_, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages),
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			TaskLive: true,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "live task without a live terminal") {
		t.Fatalf("contradictory owner error = %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("contradictory owner caused mutation: %v", stages)
	}
}

func TestExecutionResumeReusesLiveTerminalWhenThePreviousTaskSettled(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	before := record.Execution.Lease
	var stages []port.ExecutionOrcaIntentStage
	resumed, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages),
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			TerminalLive: true, TerminalID: record.Execution.Orca.TerminalPTYID,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Execution.Lease != before {
		t.Fatalf("resume changed the sealed lease: before=%#v after=%#v", before, resumed.Execution.Lease)
	}
	if resumed.Execution.Orca.TerminalPTYID != record.Execution.Orca.TerminalPTYID ||
		resumed.Execution.Orca.RunID != "run-resume" ||
		resumed.Execution.Orca.TaskID != "task-resume" ||
		resumed.Execution.Orca.DispatchID != "dispatch-resume" ||
		resumed.Execution.Orca.LeaseGeneration != before.Generation {
		t.Fatalf("resumed owner binding = %#v", resumed.Execution.Orca)
	}
	if len(stages) != 4 || stages[0] != port.ExecutionOrcaIntentRun ||
		stages[1] != port.ExecutionOrcaIntentRunBind ||
		stages[2] != port.ExecutionOrcaIntentTask ||
		stages[3] != port.ExecutionOrcaIntentDispatch {
		t.Fatalf("resume stages = %v", stages)
	}
}

func TestExecutionResumeRejectsLiveTerminalWithChangedIdentity(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	var stages []port.ExecutionOrcaIntentStage
	_, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages),
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			TerminalLive: true, TerminalID: "pty-other",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "terminal identity changed") {
		t.Fatalf("changed terminal identity error = %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("changed terminal identity caused mutation: %v", stages)
	}
}

func TestExecutionResumeReturnsExistingLiveBindingForTheSameGeneration(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.Execution.Orca.LeaseGeneration = record.Execution.Lease.Generation
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	var stages []port.ExecutionOrcaIntentStage
	resumed, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages),
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			TerminalLive: true, TaskLive: true, TerminalID: record.Execution.Orca.TerminalPTYID,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Execution.Orca.TaskID != record.Execution.Orca.TaskID || len(stages) != 0 {
		t.Fatalf("idempotent resume changed owner: result=%#v stages=%v", resumed.Execution.Orca, stages)
	}
}

func TestExecutionResumeCreatesFreshBindingAndPreservesLeaseAudit(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	before := record.Execution.Lease
	var stages []port.ExecutionOrcaIntentStage
	resumed, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages), OrcaOwner: &executionOrcaOwnerInspectorFake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Execution.Lease != before {
		t.Fatalf("resume changed the sealed lease: before=%#v after=%#v", before, resumed.Execution.Lease)
	}
	if resumed.Execution.Orca.LeaseGeneration != before.Generation ||
		resumed.Execution.Orca.RunID != "run-resume" ||
		resumed.Execution.Orca.TaskID != "task-resume" ||
		resumed.Execution.Orca.DispatchID != "dispatch-resume" ||
		resumed.Execution.Orca.TerminalPTYID != "pty-resume" {
		t.Fatalf("resume binding = %#v", resumed.Execution.Orca)
	}
	if len(stages) != 5 || stages[0] != port.ExecutionOrcaIntentTerminal ||
		stages[1] != port.ExecutionOrcaIntentRun ||
		stages[2] != port.ExecutionOrcaIntentRunBind {
		t.Fatalf("resume stages = %v", stages)
	}
}

func TestExecutionResumePromotesLegacyZeroGenerationBinding(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.Execution.Orca.LeaseGeneration = 0
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	var stages []port.ExecutionOrcaIntentStage
	resumed, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: resumeOrcaFake(t, &stages), OrcaOwner: &executionOrcaOwnerInspectorFake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Execution.Orca.LeaseGeneration != record.Execution.Lease.Generation {
		t.Fatalf("legacy binding was not promoted: %#v", resumed.Execution.Orca)
	}
}

func TestExecutionResumeAmbiguousDispatchRemainsReconcileable(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	before := record.Execution.Lease
	var stages []port.ExecutionOrcaIntentStage
	fake := resumeOrcaFake(t, &stages)
	originalInvoke := fake.invoke
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		if request.Stage == port.ExecutionOrcaIntentDispatch {
			stages = append(stages, request.Stage)
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
		}
		return originalInvoke(request)
	}
	_, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, resumeRequest(record), ExecutionResumeDependencies{
		Orca: fake, OrcaOwner: &executionOrcaOwnerInspectorFake{},
	})
	if err == nil || !strings.Contains(err.Error(), "requires execution reconcile") {
		t.Fatalf("ambiguous dispatch error = %v", err)
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution.Pending == nil || pending.Execution.Pending.Kind != "dispatch" || pending.Execution.Lease != before {
		t.Fatalf("ambiguous dispatch changed authority: %#v", pending.Execution)
	}
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{
			TaskID: request.TaskID, DispatchID: "dispatch-resume",
		}}}, nil
	}
	reconciled, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("codex", "resume-reconciler"),
		CWD: record.Execution.Workspace.Root,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Execution.Lease != before || reconciled.Execution.Orca.LeaseGeneration != before.Generation ||
		reconciled.Execution.Orca.TaskID != "task-resume" || reconciled.Execution.Orca.DispatchID != "dispatch-resume" {
		t.Fatalf("reconciled resume = %#v", reconciled.Execution)
	}
}

func TestExecuteExecutionRoutesResumeThroughSharedAction(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	calls := 0
	raw, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{Action: ExecutionActionResume, ID: record.ID, ExpectedGeneration: record.Execution.Lease.Generation, Actor: executionActor("codex", "resume-api"), CWD: record.Execution.Workspace.Root, Confirm: true}, ExecutionActionDependencies{Resume: func(_ context.Context, gotRoot string, request ExecutionResumeRequest) (ExecutionResumeResult, error) {
		calls++
		if gotRoot != stateRoot || request.ID != record.ID || request.ExpectedGeneration != record.Execution.Lease.Generation || request.CWD != record.Execution.Workspace.Root || !request.Confirm {
			t.Fatalf("resume handler request=%+v root=%q", request, gotRoot)
		}
		return executionResumeResult(record, executionResumeArtifacts{}), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	resumed, ok := raw.(ExecutionResumeResult)
	if !ok || resumed.ID != record.ID || calls != 1 {
		t.Fatalf("shared resume route = %#v calls=%d", raw, calls)
	}
}
