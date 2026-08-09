package operationalhealth

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
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

func TestCollectOrcaSharesOneRunSnapshotAndPreservesCrossChecks(t *testing.T) {
	orca := &runInventoryOrca{
		inventory: port.OrcaRunInventory{RuntimeID: "runtime", Runs: []port.OrcaRun{{RuntimeID: "runtime", ID: "run-1"}}},
		allTasks:  []port.OrcaTask{{RuntimeID: "runtime", RunID: "run-1", ID: "task-1", Status: "dispatched"}},
	}
	snapshot := corehealth.Snapshot{RepoRoot: "/repo", Messages: corehealth.MessagePresence{Empty: true}}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, false)

	if orca.inventoryCalls != 1 {
		t.Fatalf("Run inventory calls = %d, want 1", orca.inventoryCalls)
	}
	if len(orca.readerInventories) != 3 {
		t.Fatalf("reader inventories = %#v, want all/dispatched/gates", orca.readerInventories)
	}
	for _, got := range orca.readerInventories {
		if !reflect.DeepEqual(got, orca.inventory) {
			t.Fatalf("reader inventory = %#v, want %#v", got, orca.inventory)
		}
	}
	if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") {
		t.Fatalf("dispatched cross-check problem missing: %#v", snapshot.InventoryProblems)
	}
}

func TestCollectOverlapsGitAndOrcaInventory(t *testing.T) {
	withEmptyIssueOps(t)
	release := make(chan struct{})
	gitStarted := make(chan struct{})
	orcaStarted := make(chan struct{})
	collector := Collector{
		Git:  &overlapGit{gitStarted: gitStarted, orcaStarted: orcaStarted, release: release},
		Orca: &runInventoryOrca{inventory: port.OrcaRunInventory{RuntimeID: "runtime"}, gitStarted: gitStarted, orcaStarted: orcaStarted, release: release},
	}
	done := make(chan corehealth.Snapshot, 1)
	go func() {
		done <- collector.Collect(context.Background(), t.TempDir())
	}()
	defer close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Git and Orca inventory reads did not overlap")
	}
}

func TestCollectOrcaFailsClosedForRunRuntimeMismatch(t *testing.T) {
	orca := &runInventoryOrca{inventory: port.OrcaRunInventory{RuntimeID: "other"}}
	snapshot := corehealth.Snapshot{RepoRoot: "/repo", Messages: corehealth.MessagePresence{Empty: true}}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, false)

	if !hasProblemCode(snapshot.InventoryProblems, "orca_run_runtime_mismatch") {
		t.Fatalf("Run runtime mismatch problem missing: %#v", snapshot.InventoryProblems)
	}
	if len(orca.readerInventories) != 0 {
		t.Fatalf("readers ran after invalid Run snapshot: %#v", orca.readerInventories)
	}
}

func TestCollectOrcaReportsReaderFailuresInFixedOrder(t *testing.T) {
	orca := &runInventoryOrca{
		inventory:     port.OrcaRunInventory{RuntimeID: "runtime"},
		allTasksErr:   errors.New("all failed"),
		dispatchedErr: errors.New("dispatched failed"),
		gatesErr:      errors.New("gates failed"),
	}
	snapshot := corehealth.Snapshot{RepoRoot: "/repo", Messages: corehealth.MessagePresence{Empty: true}}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, false)

	var codes []string
	for _, problem := range snapshot.InventoryProblems {
		if strings.HasSuffix(problem.Code, "_failed") {
			codes = append(codes, problem.Code)
		}
	}
	want := []string{"orca_tasks_failed", "orca_dispatched_tasks_failed", "orca_gates_failed"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("problem order = %#v, want %#v", codes, want)
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
func (statusOnlyOrca) ListRunInventory(context.Context) (port.OrcaRunInventory, error) {
	return port.OrcaRunInventory{}, errors.New("unexpected ListRunInventory call")
}
func (statusOnlyOrca) ListAllTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, errors.New("unexpected ListAllTasksFromRuns call")
}
func (statusOnlyOrca) ListDispatchedTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, errors.New("unexpected ListDispatchedTasksFromRuns call")
}
func (statusOnlyOrca) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, errors.New("unexpected ShowDispatch call")
}
func (statusOnlyOrca) ListGatesFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaGate, error) {
	return nil, errors.New("unexpected ListGatesFromRuns call")
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

type runInventoryOrca struct {
	inventory     port.OrcaRunInventory
	allTasks      []port.OrcaTask
	dispatched    []port.OrcaTask
	gates         []port.OrcaGate
	allTasksErr   error
	dispatchedErr error
	gatesErr      error

	gitStarted  <-chan struct{}
	orcaStarted chan<- struct{}
	release     <-chan struct{}

	mu                sync.Mutex
	inventoryCalls    int
	readerInventories []port.OrcaRunInventory
	orcaStartOnce     sync.Once
}

func (*runInventoryOrca) Available() bool { return true }
func (*runInventoryOrca) Status(context.Context) (port.OrcaStatus, error) {
	return port.OrcaStatus{RuntimeID: "runtime", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"}, nil
}
func (*runInventoryOrca) ResolveRepo(_ context.Context, repo string) (port.OrcaRepo, error) {
	return port.OrcaRepo{RuntimeID: "runtime", ID: "repo", Path: repo}, nil
}
func (*runInventoryOrca) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return nil, nil
}
func (*runInventoryOrca) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	return nil, nil
}
func (orca *runInventoryOrca) ListRunInventory(context.Context) (port.OrcaRunInventory, error) {
	if orca.orcaStarted != nil {
		orca.orcaStartOnce.Do(func() { close(orca.orcaStarted) })
		select {
		case <-orca.gitStarted:
		case <-orca.release:
		}
	}
	orca.mu.Lock()
	orca.inventoryCalls++
	inventory := orca.inventory
	orca.mu.Unlock()
	return inventory, nil
}
func (orca *runInventoryOrca) ListAllTasksFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	orca.recordInventory(inventory)
	return orca.allTasks, orca.allTasksErr
}
func (orca *runInventoryOrca) ListDispatchedTasksFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	orca.recordInventory(inventory)
	return orca.dispatched, orca.dispatchedErr
}
func (orca *runInventoryOrca) ListGatesFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaGate, error) {
	orca.recordInventory(inventory)
	return orca.gates, orca.gatesErr
}
func (*runInventoryOrca) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, errors.New("unexpected ShowDispatch call")
}
func (*runInventoryOrca) InboxPresence(context.Context) (port.OrcaInboxPresence, error) {
	return port.OrcaInboxPresence{RuntimeID: "runtime"}, nil
}
func (orca *runInventoryOrca) recordInventory(inventory port.OrcaRunInventory) {
	orca.mu.Lock()
	defer orca.mu.Unlock()
	orca.readerInventories = append(orca.readerInventories, inventory)
}

type overlapGit struct {
	gitStarted  chan<- struct{}
	orcaStarted <-chan struct{}
	release     <-chan struct{}
	once        sync.Once
}

func (git *overlapGit) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	git.once.Do(func() {
		close(git.gitStarted)
		select {
		case <-git.orcaStarted:
		case <-git.release:
		}
	})
	return nil, nil
}

func withEmptyIssueOps(t *testing.T) {
	t.Helper()
	previousRoot := IssueOpsStateRoot
	previousIDs := ListIssueOpsIDs
	previousIndexes := ListLeaseHolderIndexes
	IssueOpsStateRoot = func() string { return t.TempDir() }
	ListIssueOpsIDs = func(string) ([]string, error) { return nil, nil }
	ListLeaseHolderIndexes = func(string) ([]issueopscontract.LeaseHolderIndex, error) { return nil, nil }
	t.Cleanup(func() {
		IssueOpsStateRoot = previousRoot
		ListIssueOpsIDs = previousIDs
		ListLeaseHolderIndexes = previousIndexes
	})
}
