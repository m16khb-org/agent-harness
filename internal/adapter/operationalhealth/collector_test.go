package operationalhealth

import (
	"context"
	"errors"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	corehealth "agent-harness/internal/domain/operationalhealth"
	"agent-harness/internal/port"
)

func TestCycleFromRecordProjectsExecution(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: issueopscontract.IssueOpsPhaseImplement,
		Execution: &issueopscontract.Execution{
			Mode:      issueopscontract.ExecutionModeOrca,
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/69-v1"},
			Lease: issueopscontract.WriteLease{
				Generation: 3, Status: issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: "session-v1", AgentID: "agent-v1", SessionProcess: &issueopscontract.NativeProcessReceipt{
					PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/../opt/codex",
				}},
			},
			Orca: &issueopscontract.OrcaBinding{
				RuntimeID: "runtime", RepoID: "repo-id", WorktreeID: "worktree-id", WorktreeInstanceID: "instance-id", OwnerHost: "codex",
				TaskID: "task-id", DispatchID: "dispatch-id", TerminalPTYID: "pty-id",
			},
		},
	}
	cycle, problems := cycleFromRecord(record, func(receipt issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error) {
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
	record := issueopscontract.IssueOpsRecord{ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: issueopscontract.IssueOpsPhaseImplement, Execution: &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect, Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/69-v1"},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &issueopscontract.NativeActor{
			Host: "codex", SessionID: "session", SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/codex"},
		}},
	}}
	cycle, problems := cycleFromRecord(record, func(issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error) {
		return corehealth.ProcessStatusUnknown, issueopscontract.NativeProcessReceipt{}, errors.New("probe failed")
	})
	if cycle.HolderProcessStatus != corehealth.ProcessStatusUnknown || len(problems) != 1 || problems[0].Code != "issueops_process_probe_failed" {
		t.Fatalf("cycle/problems = %#v / %#v", cycle, problems)
	}
}

func TestRecordOwnsOrcaUsesOnlyExecutionBinding(t *testing.T) {
	if recordOwnsOrca(issueopscontract.IssueOpsRecord{}) {
		t.Fatal("record without execution must not own Orca")
	}
	record := issueopscontract.IssueOpsRecord{Execution: &issueopscontract.Execution{Mode: issueopscontract.ExecutionModeOrca}}
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

func TestCollectOrcaProjectsTaskCompletionTimestamps(t *testing.T) {
	tests := []struct {
		name        string
		completedAt string
		want        time.Time
		invalid     bool
	}{
		{
			name:        "RFC3339Nano",
			completedAt: "2026-08-03T22:35:17.123456789Z",
			want:        time.Date(2026, time.August, 3, 22, 35, 17, 123456789, time.UTC),
		},
		{
			name:        "legacy UTC",
			completedAt: "2026-08-03 22:35:17",
			want:        time.Date(2026, time.August, 3, 22, 35, 17, 0, time.UTC),
		},
		{
			name:        "malformed slash-separated",
			completedAt: "2026/08/03 22:35:17",
			invalid:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collector := Collector{Orca: statusOnlyOrca{
				available: true,
				status:    port.OrcaStatus{RuntimeID: "runtime", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
				tasks:     []port.OrcaTask{{RuntimeID: "runtime", ID: "task", CompletedAt: test.completedAt}},
			}}
			snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
			collector.collectOrca(context.Background(), &snapshot, false)

			if len(snapshot.Tasks) != 1 {
				t.Fatalf("tasks = %#v", snapshot.Tasks)
			}
			if test.invalid {
				if !hasProblemCode(snapshot.InventoryProblems, "orca_task_timestamp_invalid") {
					t.Fatalf("problems = %#v", snapshot.InventoryProblems)
				}
				return
			}
			if !snapshot.Tasks[0].CompletedAt.Equal(test.want) {
				t.Fatalf("CompletedAt = %s, want %s", snapshot.Tasks[0].CompletedAt, test.want)
			}
			if hasProblemCode(snapshot.InventoryProblems, "orca_task_timestamp_invalid") {
				t.Fatalf("problems = %#v", snapshot.InventoryProblems)
			}
		})
	}
}

type statusOnlyOrca struct {
	available bool
	status    port.OrcaStatus
	tasks     []port.OrcaTask
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
func (orca statusOnlyOrca) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	return orca.tasks, nil
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
