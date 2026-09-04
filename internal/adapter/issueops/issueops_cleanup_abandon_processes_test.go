package issueops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// abandonResidueFixture는 execution 없이 record가 link한 실제 워크트리·브랜치를
// 가진 레코드다(#342/#433 residue 선례). 프로세스·Orca 표면만 fake로 바꾼다.
func abandonResidueFixture(t *testing.T) (string, issueops.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "477-abandon-processes")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = nil
		rec.WorktreePath = fixture.worktree
	})
	return stateRoot, fixture.record, fixture.worktree
}

// AC-01/AC-06: abandon preview도 점유자와 Orca 터미널을 나열하되 막지 않는다.
func TestCleanupAbandonPreviewListsOccupantsWithoutBlocking(t *testing.T) {
	stateRoot, record, worktree := abandonResidueFixture(t)
	deps := CleanupAbandonDeps{Processes: worldCleanupProcesses(occupiedWorld(t, codexOccupant()), nil), OrcaTerminals: readyOrca(t, worktree, "term_a")}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("occupants must not block abandon preview: %v missing=%v", err, result.Missing)
	}
	if len(result.WorkspaceProcesses) != 1 || result.WorkspaceProcesses[0].PID != 4321 || len(result.OrcaTerminals) != 1 || result.OrcaTerminals[0] != "term_a" {
		t.Fatalf("preview must list what apply will stop: %+v", result)
	}
	quiet, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses(), OrcaTerminals: readyOrca(t, worktree)})
	if err != nil || quiet.Fingerprint == result.Fingerprint {
		t.Fatalf("fingerprint must bind the occupancy: err=%v same=%v", err, quiet.Fingerprint == result.Fingerprint)
	}
}

// AC-03/AC-06: abandon apply는 워크트리를 지우기 전에 터미널과 점유자를 닫는다.
func TestCleanupAbandonApplyStopsOccupantsBeforeWorktreeRemove(t *testing.T) {
	stateRoot, record, worktree := abandonResidueFixture(t)
	world := occupiedWorld(t, codexOccupant())
	world.diesOn[4321] = syscall.SIGHUP
	processes := worldCleanupProcesses(world, nil)
	presentAtSignal := false
	processes.Signal = func(pid int, sig syscall.Signal) error {
		if _, err := os.Stat(worktree); err == nil {
			presentAtSignal = true
		}
		return world.signal(pid, sig)
	}
	deps := CleanupAbandonDeps{Processes: processes, OrcaTerminals: readyOrca(t, worktree, "term_a")}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !applied.RecordDeleted || !applied.WorktreeRemoved {
		t.Fatalf("apply must stop occupants and then remove the worktree: err=%v result=%+v", err, applied)
	}
	if !presentAtSignal {
		t.Fatal("signals must be sent while the worktree still exists (stop runs before worktree remove)")
	}
	if applied.OrcaTerminalsStopped != 1 || len(applied.WorkspaceProcessesStopped) != 1 {
		t.Fatalf("apply must report the stopped inventory: %+v", applied)
	}
	if _, err := os.Stat(worktree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("worktree must be removed after the stop step: %v", err)
	}
}

// AC-04: 점유가 남으면 workspace_processes_stop에서 멈추고 레코드와 워크트리를
// 보존한다. 그 failure receipt는 다음 preview를 cleanup_failure_inventory로 막지 않는다.
func TestCleanupAbandonApplyFailsClosedWithWorkspaceProcessesStop(t *testing.T) {
	stateRoot, record, worktree := abandonResidueFixture(t)
	world := occupiedWorld(t, codexOccupant()) // never dies
	deps := CleanupAbandonDeps{Processes: worldCleanupProcesses(world, nil), OrcaTerminals: readyOrca(t, worktree)}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree must survive a failed stop: %v", err)
	}
	if !strings.Contains(result.NextCommand, "--preview") {
		t.Fatalf("recovery must go through a fresh preview: %q", result.NextCommand)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("failure receipt must name the stop step and stay readable: err=%v failure=%+v", err, kept.CleanupAbandonFailure)
	}
	again, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil || containsString(again.Missing, "cleanup_failure_inventory") {
		t.Fatalf("a stop failure must be re-previewable, not a permanent inventory mismatch: err=%v missing=%v", err, again.Missing)
	}
}

// AC-06: ⑨ orca_resources_absent는 task/dispatch 잔여만 거부한다. 살아 있는
// 터미널은 apply ①′가 닫는다.
func TestCleanupAbandonOrcaTerminalsGates(t *testing.T) {
	run := func(t *testing.T, owner port.ExecutionOrcaOwnerInventory) []string {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "absent-worktree")
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
			rec.Execution.Orca = &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "task-1", DispatchID: "d"}
		})
		deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
		deps.OrcaOwner = &fakeOwnerInspector{inventory: owner}
		deps.Processes = quietCleanupProcesses()
		result, _ := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
		return result.Missing
	}
	if missing := run(t, port.ExecutionOrcaOwnerInventory{TerminalLive: true, TerminalInventoryComplete: true}); !containsString(missing, "orca_resources_absent") {
		t.Fatalf("with the worktree absent, apply ①′ never runs, so a live terminal must still refuse (pr-review #478 F4): %v", missing)
	}
	if missing := run(t, port.ExecutionOrcaOwnerInventory{TaskLive: true, TaskStatus: "dispatched"}); !containsString(missing, "orca_resources_absent") {
		t.Fatalf("live task/dispatch residue must still refuse: %v", missing)
	}
}

// pr-review #478 finding 1: #433이 허용하는 worktree-only 잔여(branch 부재)에서 ①′가
// 실패하면 receipt는 WorktreeHead만 갖는다. 그 receipt가 다음 preview를
// cleanup_failure_inventory로 영구히 막으면 안 된다.
func TestCleanupAbandonStopFailureInWorktreeOnlyResidueIsRePreviewable(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "canonical-worktree")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		rec.WorktreePath = root
	})
	git := &asymmetricAbandonGit{root: root, branch: record.Branch, head: "abc123"}
	deps := CleanupAbandonDeps{
		Processes: worldCleanupProcesses(occupiedWorld(t, codexOccupant()), nil), // never dies
		Git:       git.run, Orca: authoritativeZeroOrca(), OrcaTerminals: readyOrca(t, root),
	}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, "branch ref was removed elsewhere"), deps)
	if err != nil || !preview.WorktreePresent || preview.BranchPresent {
		t.Fatalf("worktree-only residue must preview: err=%v result=%+v", err, preview)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != issueops.CleanupFailureStepWorkspaceProcessesStop {
		t.Fatalf("surviving occupants must fail the stop step: err=%v result=%+v", err, result)
	}
	kept, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.BranchOID != "" || kept.CleanupAbandonFailure.WorktreeHead == "" {
		t.Fatalf("receipt must record the worktree-only shape: err=%v failure=%+v", err, kept.CleanupAbandonFailure)
	}
	again, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, "branch ref was removed elsewhere"), deps)
	if err != nil || containsString(again.Missing, "cleanup_failure_inventory") || again.Fingerprint == "" {
		t.Fatalf("a worktree-only stop failure must be re-previewable: err=%v missing=%v", err, again.Missing)
	}
}

// pr-review #478 F4: ⑨는 TerminalLive를 ①′가 실제로 닫을 수 있을 때(워크트리 존재·
// 터미널 나열)만 통과시킨다. 워크트리가 없거나 런타임이 터미널을 나열하지 못하면
// 살아 있는 터미널은 소유자 없는 자원으로 남으므로 orca_resources_absent로 거부한다.
func TestCleanupAbandonLiveTerminalPassesOnlyWhenStopReachesIt(t *testing.T) {
	liveTerminal := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{TerminalLive: true, TerminalInventoryComplete: true}}
	binding := &issueops.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "task-1", DispatchID: "d"}
	boundFixture := func(t *testing.T) (string, issueops.IssueOpsRecord, string) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		fixture := newClaimableExecutionFixture(t, stateRoot, "477-f4-terminal")
		if err := os.Remove(fixture.tokenPath); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, fixture.record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution.Mode = issueops.ExecutionModeOrca
			rec.Execution.Workspace.Driver = "orca"
			rec.Execution.Orca = binding
		})
		return stateRoot, fixture.record, fixture.worktree
	}
	t.Run("present worktree with listed terminals passes", func(t *testing.T) {
		stateRoot, record, worktree := boundFixture(t)
		deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), OrcaTerminals: readyOrca(t, worktree, "term_live"), Orca: authoritativeZeroOrca(), OrcaOwner: liveTerminal}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
		if err != nil || containsString(result.Missing, "orca_resources_absent") || len(result.OrcaTerminals) != 1 {
			t.Fatalf("a live terminal that apply ①′ will stop must not block: err=%v missing=%v terminals=%v", err, result.Missing, result.OrcaTerminals)
		}
	})
	t.Run("present worktree without a ready runtime refuses", func(t *testing.T) {
		stateRoot, record, _ := boundFixture(t)
		stopped := &fakeCleanupOrca{t: t, status: port.OrcaStatus{RuntimeState: "stopped"}, byPath: map[string][]port.OrcaTerminal{}}
		deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), OrcaTerminals: stopped, Orca: authoritativeZeroOrca(), OrcaOwner: liveTerminal}
		result, _ := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
		if !containsString(result.Missing, "orca_resources_absent") {
			t.Fatalf("a live terminal that ①′ cannot list must refuse: missing=%v", result.Missing)
		}
	})
}
