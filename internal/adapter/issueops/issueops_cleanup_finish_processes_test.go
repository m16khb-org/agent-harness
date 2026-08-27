package issueops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// quietCleanupProcesses는 점유가 없는 워크트리 관측이다. 요청자(777)의 계보는
// 항상 관측된다 — 관측 실패는 별도 게이트다.
func quietCleanupProcesses() CleanupProcessDeps {
	return CleanupProcessDeps{
		Observe: func(string) (port.CleanupWorkspaceOccupancy, error) {
			return port.CleanupWorkspaceOccupancy{Ancestry: map[int][]int{777: {700, 1}}}, nil
		},
		Signal:  func(int, syscall.Signal) error { return syscall.ESRCH },
		Sleep:   func(time.Duration) {},
		SelfPID: 777,
		Getenv:  func(string) string { return "" },
	}
}

func worldCleanupProcesses(world *fakeCleanupProcessWorld, env map[string]string) CleanupProcessDeps {
	return CleanupProcessDeps{
		Observe: world.observe, Signal: world.signal, Sleep: func(time.Duration) {}, SelfPID: 777,
		Getenv: func(key string) string { return env[key] },
	}
}

// fakeCleanupOrca는 fail-closed다: 준비되지 않은 경로의 터미널 조회는 테스트를
// 실패시킨다.
type fakeCleanupOrca struct {
	t         *testing.T
	status    port.OrcaStatus
	statusErr error
	all       []port.OrcaTerminal
	byPath    map[string][]port.OrcaTerminal
	byPathErr error
	byPathSeq [][]port.OrcaTerminal
	closed    []string
	closeErr  error
	remaining []port.OrcaTerminal
	trace     *[]string
}

func (f *fakeCleanupOrca) record(event string) {
	if f.trace != nil {
		*f.trace = append(*f.trace, event)
	}
}

func (f *fakeCleanupOrca) Status(context.Context) (port.OrcaStatus, error) {
	f.record("orca:status")
	return f.status, f.statusErr
}

func (f *fakeCleanupOrca) ListAllTerminals(context.Context) ([]port.OrcaTerminal, error) {
	f.record("orca:list-all")
	return f.all, nil
}

func (f *fakeCleanupOrca) ListWorktreeTerminalsByPath(_ context.Context, path string) ([]port.OrcaTerminal, error) {
	f.record("orca:list:" + path)
	if f.byPathErr != nil {
		return nil, f.byPathErr
	}
	if len(f.byPathSeq) > 0 {
		rows := f.byPathSeq[0]
		f.byPathSeq = f.byPathSeq[1:]
		return rows, nil
	}
	rows, ok := f.byPath[path]
	if !ok {
		f.t.Fatalf("unexpected orca terminal list path %s", path)
	}
	return rows, nil
}

func (f *fakeCleanupOrca) CloseTerminal(_ context.Context, handle string) error {
	f.record("orca:close:" + handle)
	if f.closeErr != nil {
		return f.closeErr
	}
	f.closed = append(f.closed, handle)
	for path, rows := range f.byPath {
		kept := rows[:0:0]
		for _, row := range rows {
			if row.Handle != handle {
				kept = append(kept, row)
			}
		}
		if f.remaining != nil {
			kept = f.remaining
		}
		f.byPath[path] = kept
	}
	return nil
}

func codexOccupant() issueops.CleanupWorkspaceProcess {
	return issueops.CleanupWorkspaceProcess{PID: 4321, Command: "codex", StartedAt: "2026-08-27T00:00:01Z", Executable: "codex", Descendants: 1, Collateral: 1}
}

func occupiedWorld(t *testing.T, occupants ...issueops.CleanupWorkspaceProcess) *fakeCleanupProcessWorld {
	world := &fakeCleanupProcessWorld{t: t, occupants: map[int]issueops.CleanupWorkspaceProcess{}, ancestry: map[int][]int{777: {700, 1}}, diesOn: map[int]syscall.Signal{}}
	for _, occupant := range occupants {
		world.occupants[occupant.PID] = occupant
		world.ancestry[occupant.PID] = []int{1}
	}
	return world
}

func readyOrca(t *testing.T, worktree string, handles ...string) *fakeCleanupOrca {
	rows := make([]port.OrcaTerminal, 0, len(handles))
	for _, handle := range handles {
		rows = append(rows, port.OrcaTerminal{Handle: handle, WorktreePath: worktree})
	}
	return &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready", AppPID: 903}, byPath: map[string][]port.OrcaTerminal{worktree: rows}}
}

// AC-01: 점유자가 있다는 사실은 더 이상 preview를 막지 않는다. 대신 receipt와
// Orca 터미널 handle을 결과에 싣는다(#477).
func TestCleanupFinishPreviewListsOccupantsWithoutBlocking(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = worldCleanupProcesses(occupiedWorld(t, codexOccupant()), nil)
	deps.OrcaTerminals = readyOrca(t, worktree, "term_b", "term_a")

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("occupants alone must not block the preview: %v missing=%v", err, result.Missing)
	}
	if containsString(result.Missing, "workspace_processes_quiescent") || len(result.Missing) != 0 {
		t.Fatalf("preview must be ready with occupants listed: %v", result.Missing)
	}
	if len(result.WorkspaceProcesses) != 1 || result.WorkspaceProcesses[0] != codexOccupant() {
		t.Fatalf("occupants must be reported with receipt and descendant counts: %+v", result.WorkspaceProcesses)
	}
	if !reflect.DeepEqual(result.OrcaTerminals, []string{"term_a", "term_b"}) {
		t.Fatalf("orca terminals must be listed sorted: %v", result.OrcaTerminals)
	}
	if result.Fingerprint == "" || !strings.Contains(result.NextCommand, result.Fingerprint) {
		t.Fatalf("preview must issue a fingerprint-bound apply command: %+v", result)
	}
}

// AC-02: fingerprint는 점유 receipt 집합과 터미널 handle 집합에 결속된다.
func TestCleanupFinishFingerprintBindsOccupantReceipts(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	preview := func(occupants []issueops.CleanupWorkspaceProcess, handles ...string) string {
		deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
		deps.Processes = worldCleanupProcesses(occupiedWorld(t, occupants...), nil)
		deps.OrcaTerminals = readyOrca(t, worktree, handles...)
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil {
			t.Fatal(err)
		}
		return result.Fingerprint
	}
	base := preview([]issueops.CleanupWorkspaceProcess{codexOccupant()}, "term_a")
	if again := preview([]issueops.CleanupWorkspaceProcess{codexOccupant()}, "term_a"); again != base {
		t.Fatal("identical occupancy must reproduce the fingerprint")
	}
	reused := codexOccupant()
	reused.StartedAt = "2026-08-27T00:05:00Z"
	if preview([]issueops.CleanupWorkspaceProcess{reused}, "term_a") == base {
		t.Fatal("a changed receipt (pid reuse) must change the fingerprint")
	}
	if preview([]issueops.CleanupWorkspaceProcess{codexOccupant()}, "term_z") == base {
		t.Fatal("a changed orca terminal set must change the fingerprint")
	}
	if preview(nil) == base {
		t.Fatal("quiet occupancy must not share the occupied fingerprint")
	}
}

// AC-03: apply는 fingerprinted terminal close → HUP+TERM → (KILL) → 재관측 순서로 점유를
// 없앤 뒤에야 git worktree remove로 넘어간다.
func TestCleanupFinishApplyStopsOccupantsInOrder(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	trace := []string{}
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGHUP
	processes := worldCleanupProcesses(world, nil)
	processes.Signal = func(pid int, sig syscall.Signal) error {
		trace = append(trace, fmt.Sprintf("sig:%d:%s", pid, sig))
		return world.signal(pid, sig)
	}
	orca := readyOrca(t, worktree, "term_a")
	orca.trace = &trace
	deps := finishDeps(git)
	deps.Processes = processes
	deps.OrcaTerminals = orca
	deps.Git = func(dir string, args ...string) (int, string) {
		trace = append(trace, "git:"+args[0])
		return git.run(dir, args...)
	}

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !result.OK || !result.RecordDeleted {
		t.Fatalf("apply must converge: err=%v result=%+v", err, result)
	}
	if result.OrcaTerminalsStopped != 1 || len(result.WorkspaceProcessesStopped) != 1 || result.WorkspaceProcessesStopped[0].PID != 4321 {
		t.Fatalf("apply must report what it stopped: %+v", result)
	}
	stop, hup, remove := slices.Index(trace, "orca:close:term_a"), slices.Index(trace, "sig:4321:hangup"), slices.Index(trace, "git:worktree")
	if stop < 0 || hup < 0 || remove < 0 || !(stop < hup && hup < remove) {
		t.Fatalf("order must be orca stop → signals → git worktree remove: %v", trace)
	}
	if slices.Contains(trace, "sig:4321:killed") {
		t.Fatalf("a process released by HUP must not be SIGKILLed: %v", trace)
	}
}

// AC-04: 점유가 남으면 레코드를 보존한 채 workspace_processes_stop에서 멈추고,
// 그 failure receipt는 codec을 통과해 다시 읽힌다.
func TestCleanupFinishApplyFailsClosedWithWorkspaceProcessesStop(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	git := &fakeFinishGit{branchOID: "abc123"}
	world := occupiedWorld(t, codexOccupant()) // never dies
	deps := finishDeps(git)
	deps.Processes = worldCleanupProcesses(world, nil)
	deps.OrcaTerminals = readyOrca(t, worktree)

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	if git.removedWorktree || git.deletedBranch {
		t.Fatalf("git steps must not run after a failed stop: %+v", git)
	}
	if !strings.Contains(result.NextCommand, "--preview") {
		t.Fatalf("recovery must go through a fresh preview: %q", result.NextCommand)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatalf("record with the new failure step must stay readable: %v", err)
	}
	if kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("failure receipt must name the stop step: %+v", kept.CleanupFinishFailure)
	}
}

// AC-05: 요청자 보호는 거부다. 자기 조상이 점유하거나, 요청자 터미널이 대상
// 워크트리에 매여 있거나, env가 있는데 터미널을 확정할 수 없으면 preview가 막는다.
func TestCleanupFinishRequesterGatesRefuse(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	shell := issueops.CleanupWorkspaceProcess{PID: 700, Command: "zsh", StartedAt: "2026-08-27T00:00:00Z", Executable: "zsh"}
	run := func(world *fakeCleanupProcessWorld, env map[string]string, orca *fakeCleanupOrca) []string {
		deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
		deps.Processes = worldCleanupProcesses(world, env)
		deps.OrcaTerminals = orca
		result, _ := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		return result.Missing
	}
	t.Run("requester ancestor occupies", func(t *testing.T) {
		if missing := run(occupiedWorld(t, shell), nil, readyOrca(t, worktree)); !containsString(missing, "requester_occupies_worktree") {
			t.Fatalf("own shell occupancy must refuse, not exclude: %v", missing)
		}
	})
	env := map[string]string{"ORCA_PANE_KEY": "tab-1:leaf-1", "ORCA_TERMINAL_HANDLE": "term_self"}
	t.Run("requester terminal bound to target", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{{Handle: "term_self", TabID: "tab-1", LeafID: "leaf-1", WorktreePath: worktree}}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_outside_worktree") {
			t.Fatalf("a requester terminal bound to the target worktree must refuse: %v", missing)
		}
	})
	t.Run("requester terminal elsewhere passes", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{{Handle: "term_self", TabID: "tab-1", LeafID: "leaf-1", WorktreePath: record.Repo}}
		if missing := run(occupiedWorld(t), env, orca); len(missing) != 0 {
			t.Fatalf("a requester terminal on another worktree must pass: %v", missing)
		}
	})
	t.Run("env set but unmatched", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{{Handle: "term_other", TabID: "tab-9", LeafID: "leaf-9", WorktreePath: record.Repo}}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_unresolved") {
			t.Fatalf("an unmatched env handle must fail closed: %v", missing)
		}
	})
	t.Run("matched requester terminal without worktree path", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{{Handle: "term_self", TabID: "tab-1", LeafID: "leaf-1"}}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_unresolved") {
			t.Fatalf("a requester terminal with an unknown worktree must fail closed: %v", missing)
		}
	})
	t.Run("duplicate requester pane identity", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{
			{Handle: "term_first", TabID: "tab-1", LeafID: "leaf-1", WorktreePath: record.Repo},
			{Handle: "term_self", TabID: "tab-1", LeafID: "leaf-1", WorktreePath: worktree},
		}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_unresolved") {
			t.Fatalf("duplicate requester identity must fail closed: %v", missing)
		}
	})
	t.Run("matched requester terminal with unresolvable worktree path", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.all = []port.OrcaTerminal{{Handle: "term_self", TabID: "tab-1", LeafID: "leaf-1", WorktreePath: filepath.Join(t.TempDir(), "missing")}}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_unresolved") {
			t.Fatalf("an unresolvable requester worktree must fail closed: %v", missing)
		}
	})
	t.Run("env set but runtime not ready", func(t *testing.T) {
		orca := &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeState: "starting"}, byPath: map[string][]port.OrcaTerminal{}}
		if missing := run(occupiedWorld(t), env, orca); !containsString(missing, "requester_terminal_unresolved") {
			t.Fatalf("an Orca-hosted requester cannot be verified without the runtime: %v", missing)
		}
	})
}

func TestCleanupFinishRequesterGatesRefuseSourceCheckout(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	// codec은 Execution.Workspace.Root == SourceRoot를 저장 단계에서 거부하므로,
	// 이 게이트에 닿는 것은 legacy WorktreePath만 가진 레코드다.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = nil
		rec.WorktreePath = rec.Repo
	})
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.OrcaTerminals = readyOrca(t, record.Repo)
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err == nil || !containsString(result.Missing, "worktree_is_source_checkout") {
		t.Fatalf("the source checkout is never a cleanup target: err=%v missing=%v", err, result.Missing)
	}
}

func TestCleanupWorkspaceGatesRejectSourceCheckoutSymlinkAlias(t *testing.T) {
	sourceParent := t.TempDir()
	source := filepath.Join(sourceParent, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	aliasParent := t.TempDir()
	aliasLink := filepath.Join(aliasParent, "workspace")
	if err := os.Symlink(sourceParent, aliasLink); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasLink, "source")
	record := issueops.IssueOpsRecord{Repo: source}
	_, missing := cleanupWorkspaceGatesForRecord(
		context.Background(),
		record,
		alias,
		quietCleanupProcesses(),
		readyOrca(t, alias),
	)
	if !containsString(missing, "worktree_is_source_checkout") {
		t.Fatalf("a symlink alias of the source checkout must refuse: %v", missing)
	}
}

func TestCleanupWorkspaceGatesRejectUnresolvedSourceIdentity(t *testing.T) {
	source := t.TempDir()
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	record := issueops.IssueOpsRecord{Repo: source}
	_, missing := cleanupWorkspaceGatesForRecord(
		context.Background(),
		record,
		loop,
		quietCleanupProcesses(),
		readyOrca(t, loop),
	)
	if !containsString(missing, "worktree_is_source_checkout") {
		t.Fatalf("an unresolvable source identity must fail closed: %v", missing)
	}
}

// AC-06: Orca 바인딩 사이클은 런타임 없이 터미널을 죽이지 않는다. 터미널 목록
// 관측 실패는 자기 슬러그를 갖고, 비바인딩 사이클은 signal 폴백으로 진행한다.
func TestCleanupFinishOrcaTerminalsGates(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	run := func(t *testing.T, bound bool, orca *fakeCleanupOrca) ([]string, issueops.CleanupFinishResult) {
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			if bound {
				rec.Execution.Mode = issueops.ExecutionModeOrca
				rec.Execution.Workspace.Driver = "orca"
				rec.Execution.Orca = &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
			} else {
				rec.Execution.Mode = issueops.ExecutionModeDirect
				rec.Execution.Workspace.Driver = "git"
				rec.Execution.Orca = nil
			}
		})
		deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
		deps.Processes = worldCleanupProcesses(occupiedWorld(t, codexOccupant()), nil)
		deps.OrcaTerminals = orca
		result, _ := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		return result.Missing, result
	}
	t.Run("bound cycle needs the runtime before killing anything", func(t *testing.T) {
		missing, _ := run(t, true, &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeState: "stopped"}, byPath: map[string][]port.OrcaTerminal{}})
		if !containsString(missing, "orca_runtime_ready") {
			t.Fatalf("bound cycle with occupants and no runtime must refuse: %v", missing)
		}
	})
	t.Run("bound cycle needs the runtime without native occupants", func(t *testing.T) {
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution.Mode = issueops.ExecutionModeOrca
			rec.Execution.Workspace.Driver = "orca"
			rec.Execution.Orca = &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
		})
		deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
		deps.Processes = quietCleanupProcesses()
		deps.OrcaTerminals = &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeState: "stopped"}, byPath: map[string][]port.OrcaTerminal{}}
		result, _ := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if !containsString(result.Missing, "orca_runtime_ready") {
			t.Fatalf("bound cycle must require the runtime even without native occupants: %v", result.Missing)
		}
	})
	t.Run("bound cycle needs a reachable runtime", func(t *testing.T) {
		orca := &fakeCleanupOrca{
			t: t,
			status: port.OrcaStatus{
				RuntimeState: "ready",
				GraphState:   "ready",
			},
			byPath: map[string][]port.OrcaTerminal{worktree: nil},
		}
		missing, _ := run(t, true, orca)
		if !containsString(missing, "orca_runtime_ready") {
			t.Fatalf("bound cycle must reject an unreachable runtime: %v", missing)
		}
	})
	t.Run("bound cycle needs a ready graph", func(t *testing.T) {
		orca := &fakeCleanupOrca{
			t: t,
			status: port.OrcaStatus{
				RuntimeReachable: true,
				RuntimeState:     "ready",
				GraphState:       "starting",
			},
			byPath: map[string][]port.OrcaTerminal{worktree: nil},
		}
		missing, _ := run(t, true, orca)
		if !containsString(missing, "orca_runtime_ready") {
			t.Fatalf("bound cycle must reject a graph that is not ready: %v", missing)
		}
	})
	t.Run("terminal inventory failure has its own slug", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.byPathErr = errors.New("orca runtime hiccup")
		missing, _ := run(t, false, orca)
		if !containsString(missing, "orca_terminals_observable") {
			t.Fatalf("terminal inventory failure must be reported as unobservable: %v", missing)
		}
	})
	t.Run("malformed terminal inventory has its own slug", func(t *testing.T) {
		orca := readyOrca(t, worktree)
		orca.byPath[worktree] = []port.OrcaTerminal{{WorktreePath: worktree}}
		missing, _ := run(t, false, orca)
		if !containsString(missing, "orca_terminals_observable") {
			t.Fatalf("a terminal without a handle must be unobservable: %v", missing)
		}
	})
	t.Run("unbound cycle falls back to signals without the runtime", func(t *testing.T) {
		missing, result := run(t, false, &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeState: "stopped"}, byPath: map[string][]port.OrcaTerminal{}})
		if len(missing) != 0 || len(result.OrcaTerminals) != 0 || len(result.WorkspaceProcesses) != 1 {
			t.Fatalf("unbound cycle must proceed on the signal path: missing=%v result=%+v", missing, result)
		}
	})
}

// AC-07: 결과와 감사 라인에 무엇을 종료했는지 남는다.
func TestCleanupFinishReportsStoppedProcessesAndAudit(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGTERM
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = worldCleanupProcesses(world, nil)
	deps.OrcaTerminals = readyOrca(t, worktree, "term_a")
	audit := ""
	deps.ReflectAudit = func(_ issueops.IssueOpsRecord, _ portCompletionSection, line string) error {
		audit = line
		return nil
	}
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !result.AuditReflected {
		t.Fatalf("apply must succeed and reflect the audit line: err=%v result=%+v", err, result)
	}
	if result.OrcaTerminalsStopped != 1 || len(result.WorkspaceProcessesStopped) != 1 {
		t.Fatalf("stopped inventory must be reported: %+v", result)
	}
	if !strings.Contains(audit, "stopped=1") || !strings.Contains(audit, "terminals=1") {
		t.Fatalf("audit line must record the stop counts: %q", audit)
	}
}

// fagan #478 finding 2: preview가 나열한 Orca 터미널을 apply가 닫지 못하면
// 시그널 경로로 넘어가지 말고 workspace_processes_stop에서 멈춰야 한다. apply 직전
// Orca 상태가 바뀐 경우는 fingerprint가 잡는다.
func TestCleanupFinishApplyFailsClosedWhenOrcaTerminalStopFails(t *testing.T) {
	newCase := func(t *testing.T) (string, issueops.IssueOpsRecord, *fakeFinishGit, *fakeCleanupProcessWorld, *fakeCleanupOrca, CleanupFinishDeps) {
		stateRoot, record, worktree := finishTestRecord(t, true)
		git := &fakeFinishGit{branchOID: "abc123"}
		world := occupiedWorld(t, codexOccupant())
		world.diesOn[4321] = syscall.SIGHUP
		orca := readyOrca(t, worktree, "term_a")
		deps := finishDeps(git)
		deps.Processes = worldCleanupProcesses(world, nil)
		deps.OrcaTerminals = orca
		return stateRoot, record, git, world, orca, deps
	}
	t.Run("terminal close error", func(t *testing.T) {
		stateRoot, record, git, world, orca, deps := newCase(t)
		preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil || len(preview.OrcaTerminals) != 1 {
			t.Fatalf("preview must list the terminal: err=%v result=%+v", err, preview)
		}
		orca.closeErr = errors.New("orca terminal close failed")
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
		if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop || result.OrcaTerminalsStopped != 0 {
			t.Fatalf("a failed Orca terminal close must fail the stop step: err=%v result=%+v", err, result)
		}
		if len(world.signals) != 0 || git.removedWorktree || git.deletedBranch {
			t.Fatalf("nothing may be signalled or removed after a failed terminal close: signals=%v git=%+v", world.signals, git)
		}
		kept, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil || kept.CleanupFinishFailure == nil || kept.CleanupFinishFailure.Step != issueops.CleanupFailureStepWorkspaceProcessesStop {
			t.Fatalf("failure receipt must name the stop step: err=%v failure=%+v", err, kept.CleanupFinishFailure)
		}
	})
	t.Run("terminal substitution closes only previewed handle", func(t *testing.T) {
		stateRoot, record, git, world, orca, deps := newCase(t)
		worktree := record.Execution.Workspace.Root
		previewed := port.OrcaTerminal{Handle: "term_a", WorktreePath: worktree}
		replacement := port.OrcaTerminal{Handle: "term_b", WorktreePath: worktree}
		orca.byPathSeq = [][]port.OrcaTerminal{{previewed}, {previewed}, {replacement}}
		preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil {
			t.Fatal(err)
		}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
		if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
			t.Fatalf("a substituted terminal must fail closed: err=%v result=%+v", err, result)
		}
		if !reflect.DeepEqual(orca.closed, []string{"term_a"}) {
			t.Fatalf("only the fingerprinted terminal may be closed: %v", orca.closed)
		}
		if len(world.signals) != 0 || git.removedWorktree {
			t.Fatalf("terminal substitution must not signal or remove anything: signals=%v git=%+v", world.signals, git)
		}
	})
	t.Run("terminal remains after successful stop", func(t *testing.T) {
		stateRoot, record, git, world, orca, deps := newCase(t)
		preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil {
			t.Fatal(err)
		}
		orca.remaining = []port.OrcaTerminal{{Handle: "term_survivor", WorktreePath: preview.WorktreePath}}
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
		if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
			t.Fatalf("a surviving terminal must fail closed: err=%v result=%+v", err, result)
		}
		if len(world.signals) != 0 || git.removedWorktree {
			t.Fatalf("surviving terminal must not signal or remove anything: signals=%v git=%+v", world.signals, git)
		}
	})
	t.Run("runtime lost after preview", func(t *testing.T) {
		stateRoot, record, git, world, orca, deps := newCase(t)
		preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
		if err != nil {
			t.Fatal(err)
		}
		orca.statusErr = errors.New("orca status unavailable")
		result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
		if err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") || result.OK {
			t.Fatalf("a lost runtime must invalidate the preview instead of falling back to signals: err=%v result=%+v", err, result)
		}
		if len(world.signals) != 0 || git.removedWorktree {
			t.Fatalf("stale apply must not signal or remove anything: signals=%v git=%+v", world.signals, git)
		}
	})
}

// fagan #478 finding 3 / AC-01: 런타임이 ready면 점유·바인딩·호스팅과 무관하게
// 워크트리 터미널을 나열하고 apply가 닫는다. cwd를 옮긴 셸은 점유자가 아니지만
// Orca 레지스트리에는 워크트리 터미널로 남아 있다.
func TestCleanupFinishListsAndStopsOrcaTerminalsWithoutOccupants(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Mode = issueops.ExecutionModeDirect
		rec.Execution.Workspace.Driver = "git"
		rec.Execution.Orca = nil
	})
	trace := []string{}
	orca := readyOrca(t, worktree, "term_a")
	orca.trace = &trace
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes = quietCleanupProcesses()
	deps.OrcaTerminals = orca
	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil || len(preview.OrcaTerminals) != 1 || preview.OrcaTerminals[0] != "term_a" {
		t.Fatalf("ready runtime must list worktree terminals even without occupants: err=%v result=%+v", err, preview)
	}
	applied, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !applied.RecordDeleted || applied.OrcaTerminalsStopped != 1 {
		t.Fatalf("apply must stop the listed terminal: err=%v result=%+v", err, applied)
	}
	if !containsString(trace, "orca:close:term_a") {
		t.Fatalf("apply must close the fingerprinted terminal handle: trace=%v", trace)
	}
}

func TestCleanupFinishFinalTerminalObservationBlocksLateTerminal(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Mode = issueops.ExecutionModeDirect
		rec.Execution.Workspace.Driver = "git"
		rec.Execution.Orca = nil
	})
	late := port.OrcaTerminal{Handle: "term_late", WorktreePath: worktree}
	orca := readyOrca(t, worktree)
	orca.byPathSeq = [][]port.OrcaTerminal{nil, nil, {late}}
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	deps.Processes = quietCleanupProcesses()
	deps.OrcaTerminals = orca

	preview, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("a terminal appearing before deletion must fail closed: err=%v result=%+v", err, result)
	}
	if git.removedWorktree || git.deletedBranch {
		t.Fatalf("late terminal must block Git deletion: %+v", git)
	}
}
