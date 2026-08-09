package operationalhealth

import (
	"context"
	"errors"
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

func TestCollectOrcaSharesOneRunInventoryAcrossConcurrentReaders(t *testing.T) {
	orca := newSharedInventoryOrca()
	orca.tasks = []port.OrcaTask{
		{RuntimeID: "runtime", RunID: "run-a", ID: "task-a", Status: "dispatched"},
		{RuntimeID: "runtime", RunID: "run-b", ID: "task-b", Status: "dispatched"},
	}
	orca.dispatched = append([]port.OrcaTask(nil), orca.tasks...)
	orca.dispatches = map[string]port.OrcaDispatch{
		"task-a": {RuntimeID: "runtime", ID: "dispatch-a", TaskID: "task-a", AssigneeHandle: "term-a", Status: "dispatched"},
		"task-b": {RuntimeID: "runtime", ID: "dispatch-b", TaskID: "task-b", AssigneeHandle: "term-b", Status: "dispatched"},
	}

	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, true)

	orca.mu.Lock()
	listRunCalls, fromRunCalls, peak := orca.listRunCalls, orca.fromRunCalls, orca.peak
	orca.mu.Unlock()
	if listRunCalls != 1 || fromRunCalls != 3 || peak != 3 {
		t.Fatalf("shared Run reads: list=%d readers=%d peak=%d", listRunCalls, fromRunCalls, peak)
	}
	if len(snapshot.Dispatches) != 2 || snapshot.Dispatches[0].ID != "dispatch-a" || snapshot.Dispatches[1].ID != "dispatch-b" {
		t.Fatalf("dispatch order = %#v", snapshot.Dispatches)
	}
}

func TestCollectOrcaIsolatesRunInventoryReaders(t *testing.T) {
	orca := &inventoryIsolationOrca{
		sharedInventoryOrca: newSharedInventoryOrca(),
		mutated:             make(chan struct{}),
	}

	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, true)

	orca.mu.Lock()
	dispatchedFirst, gatesFirst := orca.dispatchedFirst, orca.gatesFirst
	orca.mu.Unlock()
	if dispatchedFirst != "run-a" || gatesFirst != "run-a" || orca.inventory.Runs[0].ID != "run-a" {
		t.Fatalf("aliased Run inventory: dispatched=%q gates=%q source=%q", dispatchedFirst, gatesFirst, orca.inventory.Runs[0].ID)
	}
}

func TestCollectOrcaRejectsRunInventoryRuntimeBeforeScopedReads(t *testing.T) {
	orca := newSharedInventoryOrca()
	orca.inventory.RuntimeID = "runtime-other"
	orca.inventory.Runs[0].RuntimeID = "runtime-other"

	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, true)

	orca.mu.Lock()
	fromRunCalls := orca.fromRunCalls
	orca.mu.Unlock()
	if fromRunCalls != 0 || !hasProblemCode(snapshot.InventoryProblems, "orca_run_inventory_runtime_mismatch") {
		t.Fatalf("runtime fence: readers=%d problems=%#v", fromRunCalls, snapshot.InventoryProblems)
	}
}

func TestCollectOrcaKeepsServerFilteredDispatchedCrossCheck(t *testing.T) {
	orca := newSharedInventoryOrca()
	orca.tasks = []port.OrcaTask{{RuntimeID: "runtime", RunID: "run-a", ID: "task-a", Status: "dispatched"}}

	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{Orca: orca}.collectOrca(context.Background(), &snapshot, true)

	if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") {
		t.Fatalf("dispatched cross-check problems = %#v", snapshot.InventoryProblems)
	}
}

func TestCollectOrcaOverlapsIndependentInventoryReads(t *testing.T) {
	orca := newConcurrentInventoryOrca()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{Orca: orca}.collectOrca(ctx, &snapshot, true)

	orca.mu.Lock()
	peak := orca.peak
	orca.mu.Unlock()
	if peak != 7 {
		t.Fatalf("concurrent Orca inventory peak = %d, want 7", peak)
	}
}

func TestCollectOverlapsGitAndIssueOpsOrcaInventories(t *testing.T) {
	installEmptyIssueOpsDependencies(t)
	tracker := newInventoryOverlapTracker(2)
	collector := Collector{
		Git:  &overlapGitRunner{tracker: tracker},
		Orca: &overlapRootOrca{sharedInventoryOrca: newSharedInventoryOrca(), tracker: tracker},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snapshot := collector.Collect(ctx, "/repo")

	if tracker.peakFetches() != 2 {
		t.Fatalf("concurrent Git and Orca inventory peak = %d, want 2", tracker.peakFetches())
	}
	if snapshot.CanonicalBranch != "main" || snapshot.SourceHead != "1111111111111111111111111111111111111111" || !snapshot.OrcaObserved {
		t.Fatalf("merged inventory snapshot = %#v", snapshot)
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
func (orca statusOnlyOrca) ListRunInventory(context.Context) (port.OrcaRunInventory, error) {
	return port.OrcaRunInventory{RuntimeID: orca.status.RuntimeID, Runs: []port.OrcaRun{{RuntimeID: orca.status.RuntimeID, ID: "run"}}}, nil
}
func (orca statusOnlyOrca) ListAllTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return orca.tasks, nil
}
func (statusOnlyOrca) ListDispatchedTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, nil
}
func (statusOnlyOrca) ListGatesFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaGate, error) {
	return nil, nil
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

type sharedInventoryOrca struct {
	mu             sync.Mutex
	inventory      port.OrcaRunInventory
	tasks          []port.OrcaTask
	dispatched     []port.OrcaTask
	dispatches     map[string]port.OrcaDispatch
	listRunCalls   int
	fromRunCalls   int
	active         int
	peak           int
	readersReady   chan struct{}
	readersRelease sync.Once
}

type inventoryIsolationOrca struct {
	*sharedInventoryOrca
	mutated         chan struct{}
	mu              sync.Mutex
	dispatchedFirst string
	gatesFirst      string
}

func (orca *inventoryIsolationOrca) ListAllTasksFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	inventory.Runs[0].ID = "mutated"
	close(orca.mutated)
	return nil, nil
}

func (orca *inventoryIsolationOrca) ListDispatchedTasksFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	<-orca.mutated
	orca.mu.Lock()
	orca.dispatchedFirst = inventory.Runs[0].ID
	orca.mu.Unlock()
	return nil, nil
}

func (orca *inventoryIsolationOrca) ListGatesFromRuns(_ context.Context, inventory port.OrcaRunInventory) ([]port.OrcaGate, error) {
	<-orca.mutated
	orca.mu.Lock()
	orca.gatesFirst = inventory.Runs[0].ID
	orca.mu.Unlock()
	return nil, nil
}

func newSharedInventoryOrca() *sharedInventoryOrca {
	return &sharedInventoryOrca{
		inventory: port.OrcaRunInventory{RuntimeID: "runtime", Runs: []port.OrcaRun{
			{RuntimeID: "runtime", ID: "run-a"}, {RuntimeID: "runtime", ID: "run-b"},
		}},
		dispatches:   make(map[string]port.OrcaDispatch),
		readersReady: make(chan struct{}),
	}
}

func (*sharedInventoryOrca) Available() bool { return true }
func (*sharedInventoryOrca) Status(context.Context) (port.OrcaStatus, error) {
	return port.OrcaStatus{RuntimeID: "runtime", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"}, nil
}
func (*sharedInventoryOrca) ResolveRepo(context.Context, string) (port.OrcaRepo, error) {
	return port.OrcaRepo{RuntimeID: "runtime", ID: "repo", Path: "/repo"}, nil
}
func (*sharedInventoryOrca) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return nil, nil
}
func (*sharedInventoryOrca) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	return nil, nil
}
func (orca *sharedInventoryOrca) ListRunInventory(context.Context) (port.OrcaRunInventory, error) {
	orca.mu.Lock()
	defer orca.mu.Unlock()
	orca.listRunCalls++
	return orca.inventory, nil
}
func (orca *sharedInventoryOrca) ListAllTasksFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaTask, error) {
	if err := orca.awaitReaders(ctx); err != nil {
		return nil, err
	}
	return orca.tasks, nil
}
func (orca *sharedInventoryOrca) ListDispatchedTasksFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaTask, error) {
	if err := orca.awaitReaders(ctx); err != nil {
		return nil, err
	}
	return orca.dispatched, nil
}
func (orca *sharedInventoryOrca) ListGatesFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaGate, error) {
	if err := orca.awaitReaders(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}
func (orca *sharedInventoryOrca) awaitReaders(ctx context.Context) error {
	orca.mu.Lock()
	orca.fromRunCalls++
	orca.active++
	if orca.active > orca.peak {
		orca.peak = orca.active
	}
	if orca.active == 3 {
		orca.readersRelease.Do(func() { close(orca.readersReady) })
	}
	orca.mu.Unlock()
	select {
	case <-orca.readersReady:
	case <-ctx.Done():
		return ctx.Err()
	}
	orca.mu.Lock()
	orca.active--
	orca.mu.Unlock()
	return nil
}
func (orca *sharedInventoryOrca) ShowDispatch(_ context.Context, taskID string) (port.OrcaDispatch, error) {
	return orca.dispatches[taskID], nil
}
func (*sharedInventoryOrca) InboxPresence(context.Context) (port.OrcaInboxPresence, error) {
	return port.OrcaInboxPresence{RuntimeID: "runtime", Count: 0, RowCount: 0, CompleteAbsence: true}, nil
}

// Legacy methods intentionally fail: the operational collector must use the
// frozen Run-scoped reader contract after Orca readiness.
func (*sharedInventoryOrca) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, errors.New("legacy ListAllTasks called")
}
func (*sharedInventoryOrca) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, errors.New("legacy ListDispatchedTasks called")
}
func (*sharedInventoryOrca) ListGates(context.Context) ([]port.OrcaGate, error) {
	return nil, errors.New("legacy ListGates called")
}

type concurrentInventoryOrca struct {
	*sharedInventoryOrca
	mu      sync.Mutex
	active  int
	peak    int
	ready   chan struct{}
	release sync.Once
}

func newConcurrentInventoryOrca() *concurrentInventoryOrca {
	return &concurrentInventoryOrca{sharedInventoryOrca: newSharedInventoryOrca(), ready: make(chan struct{})}
}

func (orca *concurrentInventoryOrca) ResolveRepo(ctx context.Context, _ string) (port.OrcaRepo, error) {
	if err := orca.wait(ctx); err != nil {
		return port.OrcaRepo{}, err
	}
	return port.OrcaRepo{RuntimeID: "runtime", ID: "repo", Path: "/repo"}, nil
}
func (orca *concurrentInventoryOrca) ListWorktrees(ctx context.Context, _ string) ([]port.OrcaWorktree, error) {
	return nil, orca.wait(ctx)
}
func (orca *concurrentInventoryOrca) ListTerminals(ctx context.Context, _ string) ([]port.OrcaTerminal, error) {
	return nil, orca.wait(ctx)
}
func (orca *concurrentInventoryOrca) InboxPresence(ctx context.Context) (port.OrcaInboxPresence, error) {
	if err := orca.wait(ctx); err != nil {
		return port.OrcaInboxPresence{}, err
	}
	return port.OrcaInboxPresence{RuntimeID: "runtime", CompleteAbsence: true}, nil
}
func (orca *concurrentInventoryOrca) ListAllTasksFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, orca.wait(ctx)
}
func (orca *concurrentInventoryOrca) ListDispatchedTasksFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, orca.wait(ctx)
}
func (orca *concurrentInventoryOrca) ListGatesFromRuns(ctx context.Context, _ port.OrcaRunInventory) ([]port.OrcaGate, error) {
	return nil, orca.wait(ctx)
}
func (orca *concurrentInventoryOrca) wait(ctx context.Context) error {
	orca.mu.Lock()
	orca.active++
	if orca.active > orca.peak {
		orca.peak = orca.active
	}
	if orca.active == 7 {
		orca.release.Do(func() { close(orca.ready) })
	}
	orca.mu.Unlock()
	var err error
	select {
	case <-orca.ready:
	case <-ctx.Done():
		err = ctx.Err()
	}
	orca.mu.Lock()
	orca.active--
	orca.mu.Unlock()
	return err
}

type inventoryOverlapTracker struct {
	mu       sync.Mutex
	expected int
	active   int
	peak     int
	ready    chan struct{}
	release  sync.Once
}

func newInventoryOverlapTracker(expected int) *inventoryOverlapTracker {
	return &inventoryOverlapTracker{expected: expected, ready: make(chan struct{})}
}

func (tracker *inventoryOverlapTracker) wait(ctx context.Context) error {
	tracker.mu.Lock()
	tracker.active++
	if tracker.active > tracker.peak {
		tracker.peak = tracker.active
	}
	if tracker.active == tracker.expected {
		tracker.release.Do(func() { close(tracker.ready) })
	}
	tracker.mu.Unlock()
	var err error
	select {
	case <-tracker.ready:
	case <-ctx.Done():
		err = ctx.Err()
	}
	tracker.mu.Lock()
	tracker.active--
	tracker.mu.Unlock()
	return err
}

func (tracker *inventoryOverlapTracker) peakFetches() int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	return tracker.peak
}

type overlapGitRunner struct {
	tracker *inventoryOverlapTracker
	once    sync.Once
}

func (runner *overlapGitRunner) Run(ctx context.Context, _ string, args ...string) ([]byte, error) {
	var waitErr error
	runner.once.Do(func() { waitErr = runner.tracker.wait(ctx) })
	if waitErr != nil {
		return nil, waitErr
	}
	switch args[0] {
	case "symbolic-ref":
		return []byte("main\n"), nil
	case "rev-parse":
		return []byte("1111111111111111111111111111111111111111\n"), nil
	case "status":
		return nil, nil
	case "worktree":
		return []byte("worktree /repo\x00HEAD 1111111111111111111111111111111111111111\x00branch refs/heads/main\x00\x00"), nil
	case "for-each-ref":
		return []byte("refs/heads/main\x001111111111111111111111111111111111111111\x00"), nil
	case "ls-remote":
		return []byte("1111111111111111111111111111111111111111\trefs/heads/main\n"), nil
	default:
		return nil, errors.New("unexpected Git command")
	}
}

type overlapRootOrca struct {
	*sharedInventoryOrca
	tracker *inventoryOverlapTracker
}

func (orca *overlapRootOrca) Status(ctx context.Context) (port.OrcaStatus, error) {
	if err := orca.tracker.wait(ctx); err != nil {
		return port.OrcaStatus{}, err
	}
	return orca.sharedInventoryOrca.Status(ctx)
}

func installEmptyIssueOpsDependencies(t *testing.T) {
	t.Helper()
	oldRoot := IssueOpsStateRoot
	oldList := ListIssueOpsIDs
	oldIndexes := ListLeaseHolderIndexes
	oldRead := ReadIssueOpsExisting
	IssueOpsStateRoot = func() string { return t.TempDir() }
	ListIssueOpsIDs = func(string) ([]string, error) { return nil, nil }
	ListLeaseHolderIndexes = func(string) ([]issueopscontract.LeaseHolderIndex, error) { return nil, nil }
	ReadIssueOpsExisting = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{}, errors.New("unexpected IssueOps record read")
	}
	t.Cleanup(func() {
		IssueOpsStateRoot = oldRoot
		ListIssueOpsIDs = oldList
		ListLeaseHolderIndexes = oldIndexes
		ReadIssueOpsExisting = oldRead
	})
}

func hasProblemCode(problems []corehealth.InventoryProblem, code string) bool {
	for _, problem := range problems {
		if problem.Code == code {
			return true
		}
	}
	return false
}
