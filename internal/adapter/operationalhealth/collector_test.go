package operationalhealth

import (
	"context"
	"errors"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	corehealth "agent-harness/internal/core/operationalhealth"
	"agent-harness/internal/port"
)

func TestCycleFromRecordProjectsExecutionV1(t *testing.T) {
	record := issueops.IssueOpsRecord{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: model.IssueOpsPhaseImplement,
		Execution: &model.ExecutionV1{
			Mode:      model.ExecutionModeOrca,
			Workspace: model.WorkspaceV1{Root: "/repo.worktrees/69-v1"},
			Lease: model.WriteLeaseV1{
				Generation: 3, Status: model.LeaseStatusActive,
				Holder: &model.NativeActorV1{Host: "codex", SessionID: "session-v1", AgentID: "agent-v1", SessionProcess: &model.NativeProcessReceiptV1{
					PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/../opt/codex",
				}},
			},
			Orca: &model.OrcaBindingV1{
				RuntimeID: "runtime", RepoID: "repo-id", WorktreeID: "worktree-id", WorktreeInstanceID: "instance-id", OwnerHost: "codex",
				TaskID: "task-id", DispatchID: "dispatch-id", TerminalPTYID: "pty-id",
			},
		},
	}
	cycle, problems := cycleFromRecord(record, func(receipt model.NativeProcessReceiptV1) (string, model.NativeProcessReceiptV1, error) {
		return corehealth.ProcessStatusLive, receipt, nil
	})
	if len(problems) != 0 {
		t.Fatalf("problems = %#v", problems)
	}
	if cycle.LeaseStatus != "active" || cycle.ExecutionMode != "orca" || cycle.Generation != 3 {
		t.Fatalf("lease projection = %#v", cycle)
	}
	if cycle.HolderHost != "codex" || cycle.HolderSessionID != "session-v1" || cycle.HolderAgentID != "agent-v1" ||
		cycle.HolderPID != 123 || cycle.HolderStartedAt != "2026-07-22T00:00:00Z" || cycle.HolderExecutable != "/opt/../opt/codex" || cycle.HolderProcessStatus != corehealth.ProcessStatusLive ||
		cycle.WorktreePath != "/repo.worktrees/69-v1" {
		t.Fatalf("holder/workspace projection = %#v", cycle)
	}
	if cycle.OrcaRuntimeID != "runtime" || cycle.OrcaWorktreeID != "worktree-id" || cycle.OrcaOwnerHost != "codex" || cycle.TaskID != "task-id" || cycle.DispatchID != "dispatch-id" || cycle.TerminalPTYID != "pty-id" {
		t.Fatalf("Orca projection = %#v", cycle)
	}
}

func TestCycleFromRecordReportsNativeProcessProbeFailure(t *testing.T) {
	record := issueops.IssueOpsRecord{ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: model.IssueOpsPhaseImplement, Execution: &model.ExecutionV1{
		Mode: model.ExecutionModeDirect, Workspace: model.WorkspaceV1{Root: "/repo.worktrees/69-v1"},
		Lease: model.WriteLeaseV1{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActorV1{
			Host: "codex", SessionID: "session", SessionProcess: &model.NativeProcessReceiptV1{PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/codex"},
		}},
	}}
	cycle, problems := cycleFromRecord(record, func(model.NativeProcessReceiptV1) (string, model.NativeProcessReceiptV1, error) {
		return corehealth.ProcessStatusUnknown, model.NativeProcessReceiptV1{}, errors.New("probe failed")
	})
	if cycle.HolderProcessStatus != corehealth.ProcessStatusUnknown || len(problems) != 1 || problems[0].Code != "issueops_process_probe_failed" {
		t.Fatalf("cycle/problems = %#v / %#v", cycle, problems)
	}
}

func TestRecordOwnsOrcaUsesOnlyExecutionV1Binding(t *testing.T) {
	if recordOwnsOrca(issueops.IssueOpsRecord{}) {
		t.Fatal("record without execution must not own Orca")
	}
	record := issueops.IssueOpsRecord{Execution: &model.ExecutionV1{Mode: model.ExecutionModeOrca}}
	if !recordOwnsOrca(record) {
		t.Fatal("Orca execution without a completed binding was not detected")
	}
}

func TestCollectOrcaTreatsUnreadyRuntimeAsOptionalForDirectExecution(t *testing.T) {
	collector := Collector{Orca: statusOnlyOrca{available: true, status: port.OrcaStatus{RuntimeID: "runtime", RuntimeReachable: true, RuntimeState: "starting", GraphState: "ready"}}}
	snapshot := corehealth.Snapshot{}
	collector.collectOrca(context.Background(), &snapshot, false)
	if snapshot.OrcaObserved || !snapshot.Messages.CompleteAbsence || len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("optional unready Orca projection = %#v", snapshot)
	}
}

func TestCollectOrcaRejectsUnreadyRuntimeForOrcaExecution(t *testing.T) {
	collector := Collector{Orca: statusOnlyOrca{available: true, status: port.OrcaStatus{RuntimeID: "runtime", RuntimeReachable: true, RuntimeState: "starting", GraphState: "ready"}}}
	snapshot := corehealth.Snapshot{}
	collector.collectOrca(context.Background(), &snapshot, true)
	if !hasProblemCode(snapshot.InventoryProblems, "orca_runtime_unready") {
		t.Fatalf("owned unready Orca problems = %#v", snapshot.InventoryProblems)
	}
}

type statusOnlyOrca struct {
	available bool
	status    port.OrcaStatus
}

func (orca statusOnlyOrca) Available() bool { return orca.available }
func (orca statusOnlyOrca) Status(context.Context) (port.OrcaStatus, error) {
	return orca.status, nil
}
func (statusOnlyOrca) ResolveRepo(context.Context, string) (port.OrcaRepo, error) {
	return port.OrcaRepo{}, errors.New("unexpected ResolveRepo call")
}
func (statusOnlyOrca) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return nil, errors.New("unexpected ListWorktrees call")
}
func (statusOnlyOrca) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	return nil, errors.New("unexpected ListTerminals call")
}
func (statusOnlyOrca) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, errors.New("unexpected ListAllTasks call")
}
func (statusOnlyOrca) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, errors.New("unexpected ListDispatchedTasks call")
}
func (statusOnlyOrca) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, errors.New("unexpected ShowDispatch call")
}
func (statusOnlyOrca) ListGates(context.Context) ([]port.OrcaGate, error) {
	return nil, errors.New("unexpected ListGates call")
}
func (statusOnlyOrca) InboxPresence(context.Context) (port.OrcaInboxPresence, error) {
	return port.OrcaInboxPresence{}, errors.New("unexpected InboxPresence call")
}

func hasProblemCode(problems []corehealth.InventoryProblem, code string) bool {
	for _, problem := range problems {
		if problem.Code == code {
			return true
		}
	}
	return false
}
