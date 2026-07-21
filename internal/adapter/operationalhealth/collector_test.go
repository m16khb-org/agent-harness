package operationalhealth

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
	corehealth "agent-harness/internal/core/operationalhealth"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestCollectorCollectsCanonicalGitIssueOpsAndBindings(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "17-feature")
	record := startCollectorRecord(t, fixture.repo, "17-feature")
	record.WorktreePath = fixture.worktree
	writeCollectorRecord(t, record)

	otherRepo := filepath.Join(t.TempDir(), "other-repo")
	if err := os.MkdirAll(otherRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	other := startCollectorRecord(t, otherRepo, "18-other")
	if err := issueops.BindIssueOpsSession(record.Repo, record.ID, record.Branch, record.WorktreePath); err != nil {
		t.Fatal(err)
	}
	if err := issueops.BindIssueOpsSession(other.Repo, other.ID, other.Branch, ""); err != nil {
		t.Fatal(err)
	}

	git := &recordingGitRunner{delegate: ExecGitRunner{}}
	snapshot := (Collector{Git: git, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("complete optional-Orca collection problems = %#v", snapshot.InventoryProblems)
	}
	if snapshot.RepoRoot != fixture.repo || snapshot.CanonicalBranch != "main" || snapshot.SourceHead != fixture.mainHead || !snapshot.SourceClean {
		t.Fatalf("source projection = %#v", snapshot)
	}
	if snapshot.OrcaObserved {
		t.Fatal("unavailable unowned Orca must remain optional")
	}
	if len(snapshot.Cycles) != 2 || len(snapshot.Bindings) != 2 {
		t.Fatalf("IssueOps projection cycles=%#v bindings=%#v", snapshot.Cycles, snapshot.Bindings)
	}
	if !hasGitWorktree(snapshot.GitWorktrees, fixture.repo, "main", true) || !hasGitWorktree(snapshot.GitWorktrees, fixture.worktree, "17-feature", false) {
		t.Fatalf("Git worktree projection = %#v", snapshot.GitWorktrees)
	}
	if !hasRef(snapshot.LocalRefs, "17-feature", fixture.branchHead, "local") || !hasRef(snapshot.RemoteRefs, "17-feature", fixture.branchHead, "remote") {
		t.Fatalf("full ref projection local=%#v remote=%#v", snapshot.LocalRefs, snapshot.RemoteRefs)
	}
	wantCommands := [][]string{
		{"symbolic-ref", "--quiet", "--short", "HEAD"},
		{"rev-parse", "--verify", "HEAD"},
		{"status", "--porcelain=v1", "-z"},
		{"worktree", "list", "--porcelain", "-z"},
		{"for-each-ref", "--format=%(refname)%00%(objectname)%00", "refs/heads"},
		{"ls-remote", "--heads", "origin"},
	}
	if !slices.EqualFunc(git.calls, wantCommands, func(left, right []string) bool { return slices.Equal(left, right) }) {
		t.Fatalf("Git argv = %#v, want %#v", git.calls, wantCommands)
	}
}

func TestCollectorDoesNotCreateMissingIssueOpsStore(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "19-read-only")

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("read-only empty inventory problems = %#v", snapshot.InventoryProblems)
	}
	if _, err := os.Stat(issueops.IssueOpsStateRoot()); !os.IsNotExist(err) {
		t.Fatalf("collector created missing IssueOps store: err=%v", err)
	}
}

func TestCollectorDoesNotRepairExistingIssueOpsStore(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "19-existing-read-only")
	record := startCollectorRecord(t, fixture.repo, "19-existing-read-only")
	if err := issueops.BindIssueOpsSession(record.Repo, record.ID, record.Branch, fixture.worktree); err != nil {
		t.Fatal(err)
	}

	stateRoot := issueops.IssueOpsStateRoot()
	paths := []string{
		stateRoot,
		filepath.Join(stateRoot, "harness.db"),
		filepath.Join(stateRoot, "harness.lock.db"),
	}
	for index, path := range paths {
		mode := os.FileMode(0o644)
		if index == 0 {
			mode = 0o755
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if len(snapshot.Cycles) != 1 || len(snapshot.Bindings) != 1 {
		t.Fatalf("existing state projection cycles=%#v bindings=%#v problems=%#v", snapshot.Cycles, snapshot.Bindings, snapshot.InventoryProblems)
	}
	for index, path := range paths {
		want := os.FileMode(0o644)
		if index == 0 {
			want = 0o755
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("collector repaired %s mode to %o, want unchanged %o", path, got, want)
		}
	}
}

func TestCollectorIncludesBindingWithoutSurvivingCycleRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "19-orphan-binding")
	if err := issueops.BindIssueOpsSession(fixture.repo, "io-orphan", "19-orphan-binding", fixture.worktree); err != nil {
		t.Fatal(err)
	}

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if len(snapshot.Cycles) != 0 || len(snapshot.Bindings) != 1 || snapshot.Bindings[0].CycleID != "io-orphan" {
		t.Fatalf("orphan binding was omitted: cycles=%#v bindings=%#v problems=%#v", snapshot.Cycles, snapshot.Bindings, snapshot.InventoryProblems)
	}
}

func TestExecGitRunnerUsesNoninteractiveReadOnlyEnvironment(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join(binDir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s|%s|%s|%s' \"$GIT_OPTIONAL_LOCKS\" \"$GIT_TERMINAL_PROMPT\" \"$GCM_INTERACTIVE\" \"$SSH_ASKPASS_REQUIRE\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GCM_INTERACTIVE", "Always")
	t.Setenv("SSH_ASKPASS_REQUIRE", "force")

	output, err := (ExecGitRunner{}).Run(context.Background(), t.TempDir(), "probe")
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "0|0|Never|never" {
		t.Fatalf("Git inventory environment = %q", output)
	}
}

func TestExecGitRunnerAppliesLocalDeadline(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join(binDir, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	started := time.Now()

	_, err := (ExecGitRunner{timeout: 25 * time.Millisecond}).Run(context.Background(), t.TempDir(), "probe")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Git inventory timeout error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Git inventory ignored local deadline: %s", elapsed)
	}
}

func TestCanonicalInventoryPathResolvesExistingSymlinkAncestors(t *testing.T) {
	realRoot := t.TempDir()
	resolvedRoot, err := filepath.EvalSymlinks(realRoot)
	if err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Fatal(err)
	}

	got := canonicalInventoryPath(filepath.Join(linkRoot, "missing", "child"))
	want := filepath.Join(resolvedRoot, "missing", "child")
	if got != want {
		t.Fatalf("canonical path = %q, want %q", got, want)
	}
}

func TestCollectorReportsUnreadableIssueOpsWithoutSkippingValidRows(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "20-readable")
	valid := startCollectorRecord(t, fixture.repo, "20-readable")
	db, err := sqlstore.Open(issueops.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "io-corrupt", []byte(`{"id":"io-corrupt"`)); err != nil {
		t.Fatal(err)
	}

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if len(snapshot.Cycles) != 1 || snapshot.Cycles[0].ID != valid.ID {
		t.Fatalf("valid rows were skipped after corruption: %#v", snapshot.Cycles)
	}
	if !hasInventoryProblem(snapshot.InventoryProblems, "issueops_read_failed") {
		t.Fatalf("unreadable row was hidden: %#v", snapshot.InventoryProblems)
	}
}

func TestCollectorTreatsUnavailableOrcaAsOptionalOnlyWithoutOwnedRecords(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "21-orca-owned")
	record := startCollectorRecord(t, fixture.repo, "21-orca-owned")
	attachPreparingOrcaIdentity(t, &record, fixture.mainHead)
	writeCollectorRecord(t, record)

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: &fakeOrcaInventory{}}).Collect(context.Background(), fixture.repo)

	if !hasInventoryProblem(snapshot.InventoryProblems, "orca_unavailable") {
		t.Fatalf("Orca-owned record without Orca was accepted: %#v", snapshot.InventoryProblems)
	}
}

func TestCollectorCollectsRegisteredGlobalOrcaInventory(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "22-orca-live")
	record := startCollectorRecord(t, fixture.repo, "22-orca-live")
	attachPreparingOrcaIdentity(t, &record, fixture.mainHead)
	writeCollectorRecord(t, record)
	worker := record.ExecutionHandoff.WorkerRoot

	orca := &fakeOrcaInventory{
		available: true,
		status:    port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
		repo:      port.OrcaRepo{RuntimeID: "runtime-1", ID: "repo-1", Path: fixture.repo, RemoteName: "origin"},
		worktrees: []port.OrcaWorktree{
			{RuntimeID: "runtime-1", ID: "wt-main", InstanceID: "instance-main", RepoID: "repo-1", Path: fixture.repo, Branch: "main", Head: fixture.mainHead},
			{RuntimeID: "runtime-1", ID: "wt-1", InstanceID: "instance-1", RepoID: "repo-1", Path: worker, Branch: record.Branch, Head: fixture.mainHead},
		},
		terminals: []port.OrcaTerminal{{RuntimeID: "runtime-1", Handle: "term-global", PTYID: "pty-global", TabID: "tab-global", LeafID: "leaf-global", WorktreeID: "wt-1", WorktreePath: worker, Connected: true, Writable: true}},
		tasks: []port.OrcaTask{
			{RuntimeID: "runtime-1", ID: "task-completed", Status: "completed", CompletedAt: "2026-07-19T01:02:03Z", HasResult: true},
			{RuntimeID: "runtime-1", ID: "task-dispatched", Status: "dispatched"},
			{RuntimeID: "runtime-1", ID: "task-completed", Status: "failed"},
		},
		dispatchedTasks: []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-dispatched", Status: "dispatched"}},
		dispatches:      map[string]port.OrcaDispatch{"task-dispatched": {RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-dispatched", AssigneeHandle: "term-global", Status: "dispatched"}},
		gates:           []port.OrcaGate{{RuntimeID: "runtime-1", ID: "gate-1", TaskID: "task-dispatched", Status: "pending"}},
		inbox:           port.OrcaInboxPresence{RuntimeID: "runtime-1", Count: 1, RowCount: 1},
	}

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: orca}).Collect(context.Background(), fixture.repo)

	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("registered Orca collection problems = %#v", snapshot.InventoryProblems)
	}
	if !snapshot.OrcaObserved || snapshot.OrcaRuntimeID != "runtime-1" || snapshot.OrcaRepoID != "repo-1" || len(snapshot.OrcaWorktrees) != 2 || len(snapshot.Terminals) != 1 || len(snapshot.Tasks) != 3 || len(snapshot.Dispatches) != 1 || len(snapshot.Gates) != 1 {
		t.Fatalf("Orca projection = %#v", snapshot)
	}
	if snapshot.Tasks[0].CompletedAt.IsZero() || !hasOperationalTask(snapshot.Tasks, "task-dispatched", "dispatch-1") {
		t.Fatalf("task/dispatch projection = %#v", snapshot.Tasks)
	}
	if snapshot.Messages.Count != 1 || snapshot.Messages.Empty || snapshot.Messages.CompleteAbsence {
		t.Fatalf("message presence projection = %#v", snapshot.Messages)
	}
	if got := countOperationalTasks(snapshot.Tasks, "task-completed"); got != 2 {
		t.Fatalf("duplicate task IDs were collapsed: %#v", snapshot.Tasks)
	}
	for _, call := range []string{"status", "resolve-repo", "list-worktrees", "list-terminals", "list-all-tasks", "list-dispatched-tasks", "show-dispatch:task-dispatched", "list-gates", "inbox-presence"} {
		if !slices.Contains(orca.calls, call) {
			t.Fatalf("missing Orca read %q in %#v", call, orca.calls)
		}
	}
}

func TestCycleFromRecordPreservesCompleteDurableOrcaIdentity(t *testing.T) {
	record := issueops.IssueOpsRecord{
		ID: "io-sealed", Repo: "/repo", Branch: "1-sealed", Phase: issueops.IssueOpsPhaseImplement,
		ExecutionHandoff: &issueops.IssueOpsExecutionHandoff{
			State: handoff.StateOwnerActive,
			Orca: &issueops.IssueOpsOrcaIdentity{
				RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "wt-1", WorktreeInstanceID: "instance-1",
				WorktreePath: "/repo.wt/1-sealed", WorkerTerminalHandle: "term-1", WorkerPTYID: "pty-1",
				WorkerTabID: "tab-1", WorkerLeafID: "leaf-1", TaskID: "task-1", DispatchID: "dispatch-1",
			},
		},
	}

	cycle, problems := cycleFromRecord(record)

	if len(problems) != 0 || cycle.OrcaRuntimeID != "runtime-1" || cycle.OrcaRepoID != "repo-1" || cycle.TerminalTabID != "tab-1" || cycle.TerminalLeafID != "leaf-1" {
		t.Fatalf("durable Orca identity projection cycle=%#v problems=%#v", cycle, problems)
	}
}

func TestCollectorFailsClosedOnOrcaResolutionOrListFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		method string
		code   string
	}{
		{name: "repo", method: "resolve-repo", code: "orca_repo_failed"},
		{name: "tasks", method: "list-all-tasks", code: "orca_tasks_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			fixture := newCollectorGitFixture(t, "23-orca-error")
			orca := healthyEmptyOrca(fixture.repo)
			orca.errors = map[string]error{test.method: errors.New("external failure with secret=must-not-escape")}

			snapshot := (Collector{Git: ExecGitRunner{}, Orca: orca}).Collect(context.Background(), fixture.repo)

			if !hasInventoryProblem(snapshot.InventoryProblems, test.code) {
				t.Fatalf("%s failure was hidden: %#v", test.method, snapshot.InventoryProblems)
			}
			for _, problem := range snapshot.InventoryProblems {
				if strings.Contains(problem.Detail, "must-not-escape") {
					t.Fatalf("inventory problem leaked external detail: %#v", problem)
				}
			}
		})
	}
}

func TestCollectorFailsClosedOnDispatchedTaskInventoryMismatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "24-dispatch-mismatch")
	for _, test := range []struct {
		name            string
		allStatus       string
		includeFiltered bool
	}{
		{name: "status mismatch", allStatus: "ready", includeFiltered: true},
		{name: "missing filtered row", allStatus: "dispatched"},
	} {
		t.Run(test.name, func(t *testing.T) {
			orca := healthyEmptyOrca(fixture.repo)
			orca.tasks = []port.OrcaTask{{ID: "task-dispatched", Status: test.allStatus}}
			if test.includeFiltered {
				orca.dispatchedTasks = []port.OrcaTask{{ID: "task-dispatched", Status: "dispatched"}}
				orca.dispatches = map[string]port.OrcaDispatch{
					"task-dispatched": {ID: "dispatch-1", TaskID: "task-dispatched", AssigneeHandle: "term-1", Status: "dispatched"},
				}
			}

			snapshot := (Collector{Git: ExecGitRunner{}, Orca: orca}).Collect(context.Background(), fixture.repo)

			if !hasInventoryProblem(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") {
				t.Fatalf("inconsistent dispatched-task projections were accepted: %#v", snapshot.InventoryProblems)
			}
		})
	}
}

func TestCollectorFailsClosedOnDispatchedTaskRuntimeDrift(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newCollectorGitFixture(t, "24-dispatch-runtime")
	orca := healthyEmptyOrca(fixture.repo)
	orca.tasks = []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-dispatched", Status: "dispatched"}}
	orca.dispatchedTasks = []port.OrcaTask{{RuntimeID: "runtime-other", ID: "task-dispatched", Status: "dispatched"}}
	orca.dispatches = map[string]port.OrcaDispatch{
		"task-dispatched": {RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-dispatched", AssigneeHandle: "term-1", Status: "dispatched"},
	}

	snapshot := (Collector{Git: ExecGitRunner{}, Orca: orca}).Collect(context.Background(), fixture.repo)

	if !hasInventoryProblem(snapshot.InventoryProblems, "orca_dispatched_task_runtime_mismatch") {
		t.Fatalf("dispatched task runtime drift was hidden: %#v", snapshot.InventoryProblems)
	}
}

func TestCycleFromRecordRejectsConflictingWorktreePaths(t *testing.T) {
	record := issueops.IssueOpsRecord{
		ID: "io-conflict", Repo: "/repo", Branch: "25-conflict", Phase: issueops.IssueOpsPhasePlan,
		WorktreePath: "/repo.wt/from-record",
		ExecutionHandoff: &issueops.IssueOpsExecutionHandoff{
			WorkerRoot: "/repo.wt/from-handoff",
			Orca:       &issueops.IssueOpsOrcaIdentity{WorktreePath: "/repo.wt/from-handoff"},
		},
	}

	_, problems := cycleFromRecord(record)

	if !hasInventoryProblem(problems, "issueops_worktree_identity_mismatch") {
		t.Fatalf("conflicting durable paths were accepted: %#v", problems)
	}
}

func TestSortSnapshotIsDeterministicForDuplicateIdentities(t *testing.T) {
	earlier := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Minute)
	left := corehealth.Snapshot{
		Cycles:       []corehealth.Cycle{{ID: "io-duplicate", Repo: "/z"}, {ID: "io-duplicate", Repo: "/a"}},
		GitWorktrees: []corehealth.GitWorktree{{Path: "/same", Branch: "z"}, {Path: "/same", Branch: "a"}},
		OrcaWorktrees: []corehealth.OrcaWorktree{
			{ID: "wt-duplicate", InstanceID: "instance", Path: "/same", Branch: "z"},
			{ID: "wt-duplicate", InstanceID: "instance", Path: "/same", Branch: "a"},
		},
		Terminals:  []corehealth.OrcaTerminal{{Handle: "term", PTYID: "pty", WorktreeID: "z"}, {Handle: "term", PTYID: "pty", WorktreeID: "a"}},
		Tasks:      []corehealth.OrcaTask{{ID: "task", Status: "failed", CompletedAt: later}, {ID: "task", Status: "failed", CompletedAt: earlier}},
		Dispatches: []corehealth.OrcaDispatch{{ID: "dispatch", TaskID: "z"}, {ID: "dispatch", TaskID: "a"}},
		Gates:      []corehealth.OrcaGate{{ID: "gate", TaskID: "z"}, {ID: "gate", TaskID: "a"}},
	}
	right := corehealth.Snapshot{
		Cycles:       []corehealth.Cycle{{ID: "io-duplicate", Repo: "/a"}, {ID: "io-duplicate", Repo: "/z"}},
		GitWorktrees: []corehealth.GitWorktree{{Path: "/same", Branch: "a"}, {Path: "/same", Branch: "z"}},
		OrcaWorktrees: []corehealth.OrcaWorktree{
			{ID: "wt-duplicate", InstanceID: "instance", Path: "/same", Branch: "a"},
			{ID: "wt-duplicate", InstanceID: "instance", Path: "/same", Branch: "z"},
		},
		Terminals:  []corehealth.OrcaTerminal{{Handle: "term", PTYID: "pty", WorktreeID: "a"}, {Handle: "term", PTYID: "pty", WorktreeID: "z"}},
		Tasks:      []corehealth.OrcaTask{{ID: "task", Status: "failed", CompletedAt: earlier}, {ID: "task", Status: "failed", CompletedAt: later}},
		Dispatches: []corehealth.OrcaDispatch{{ID: "dispatch", TaskID: "a"}, {ID: "dispatch", TaskID: "z"}},
		Gates:      []corehealth.OrcaGate{{ID: "gate", TaskID: "a"}, {ID: "gate", TaskID: "z"}},
	}

	sortSnapshot(&left)
	sortSnapshot(&right)

	if !reflect.DeepEqual(left, right) {
		t.Fatalf("duplicate identity ordering depends on input order:\nleft=%#v\nright=%#v", left, right)
	}
}

type collectorGitFixture struct {
	repo       string
	worktree   string
	mainHead   string
	branchHead string
}

func newCollectorGitFixture(t *testing.T, branch string) collectorGitFixture {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "source")
	remote := filepath.Join(root, "origin.git")
	worktree := repo + ".worktrees" + string(filepath.Separator) + branch
	runCollectorGit(t, "", "init", "--bare", remote)
	runCollectorGit(t, "", "init", "-b", "main", repo)
	runCollectorGit(t, repo, "config", "user.name", "Collector Test")
	runCollectorGit(t, repo, "config", "user.email", "collector@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("collector fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCollectorGit(t, repo, "add", "README.md")
	runCollectorGit(t, repo, "commit", "-m", "fixture")
	runCollectorGit(t, repo, "remote", "add", "origin", remote)
	runCollectorGit(t, repo, "push", "-u", "origin", "main")
	runCollectorGit(t, repo, "branch", branch)
	runCollectorGit(t, repo, "worktree", "add", worktree, branch)
	runCollectorGit(t, repo, "push", "origin", branch)
	return collectorGitFixture{
		repo:       canonicalInventoryPath(repo),
		worktree:   canonicalInventoryPath(worktree),
		mainHead:   strings.TrimSpace(runCollectorGit(t, repo, "rev-parse", "HEAD")),
		branchHead: strings.TrimSpace(runCollectorGit(t, repo, "rev-parse", branch)),
	}
}

func runCollectorGit(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func startCollectorRecord(t *testing.T, repo, branch string) issueops.IssueOpsRecord {
	t.Helper()
	record, err := issueops.StartIssueOps(issueops.IssueOpsStateRoot(), issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func writeCollectorRecord(t *testing.T, record issueops.IssueOpsRecord) {
	t.Helper()
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func attachPreparingOrcaIdentity(t *testing.T, record *issueops.IssueOpsRecord, head string) {
	t.Helper()
	worker := record.Repo + ".worktrees" + string(filepath.Separator) + record.Branch
	record.WorktreePath = worker
	record.ExecutionHandoff = &issueops.IssueOpsExecutionHandoff{
		State: handoff.StateOwnershipDispatching, Attempt: 1, OwnershipEpoch: "epoch-1",
		WorkspaceEpoch: "workspace-1", WorkspaceSHA256: strings.Repeat("a", 64), AttemptBaseHead: head,
		Driver: "orca", Agent: "codex", CoordinatorRoot: record.Repo, WorkerRoot: worker,
		Orca: &issueops.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/" + record.Branch,
			WorktreeID: "wt-1", WorktreeInstanceID: "instance-1", WorktreePath: worker,
		},
	}
	record.ExecutionWorkspace = &issueops.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "workspace-1", Driver: "orca", Agent: "codex", CoordinatorRoot: record.Repo, WorkerRoot: worker,
		PreparationSession: &issueops.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "source-session"}, BaseHead: head,
		Orca: record.ExecutionHandoff.Orca,
	}
}

func hasGitWorktree(values []corehealth.GitWorktree, path, branch string, canonical bool) bool {
	for _, value := range values {
		if value.Path == path && value.Branch == branch && value.Canonical == canonical {
			return true
		}
	}
	return false
}

func hasRef(values []corehealth.GitRef, branch, oid, location string) bool {
	for _, value := range values {
		if value.Branch == branch && value.OID == oid && value.Location == location {
			return true
		}
	}
	return false
}

func hasInventoryProblem(values []corehealth.InventoryProblem, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func hasOperationalTask(values []corehealth.OrcaTask, id, dispatchID string) bool {
	for _, value := range values {
		if value.ID == id && value.DispatchID == dispatchID {
			return true
		}
	}
	return false
}

func countOperationalTasks(values []corehealth.OrcaTask, id string) int {
	count := 0
	for _, value := range values {
		if value.ID == id {
			count++
		}
	}
	return count
}

type recordingGitRunner struct {
	delegate GitRunner
	calls    [][]string
}

func (r *recordingGitRunner) Run(ctx context.Context, repo string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return r.delegate.Run(ctx, repo, args...)
}

type fakeOrcaInventory struct {
	available       bool
	status          port.OrcaStatus
	repo            port.OrcaRepo
	worktrees       []port.OrcaWorktree
	terminals       []port.OrcaTerminal
	tasks           []port.OrcaTask
	dispatchedTasks []port.OrcaTask
	dispatches      map[string]port.OrcaDispatch
	gates           []port.OrcaGate
	inbox           port.OrcaInboxPresence
	errors          map[string]error
	calls           []string
}

func healthyEmptyOrca(repo string) *fakeOrcaInventory {
	return &fakeOrcaInventory{
		available: true,
		status:    port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
		repo:      port.OrcaRepo{RuntimeID: "runtime-1", ID: "repo-1", Path: repo},
		inbox:     port.OrcaInboxPresence{RuntimeID: "runtime-1", CompleteAbsence: true},
	}
}

func (f *fakeOrcaInventory) Available() bool { return f.available }

func (f *fakeOrcaInventory) result(method string) error {
	f.calls = append(f.calls, method)
	return f.errors[method]
}

func (f *fakeOrcaInventory) Status(context.Context) (port.OrcaStatus, error) {
	return f.status, f.result("status")
}

func (f *fakeOrcaInventory) ResolveRepo(context.Context, string) (port.OrcaRepo, error) {
	return f.repo, f.result("resolve-repo")
}

func (f *fakeOrcaInventory) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return append([]port.OrcaWorktree(nil), f.worktrees...), f.result("list-worktrees")
}

func (f *fakeOrcaInventory) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	return append([]port.OrcaTerminal(nil), f.terminals...), f.result("list-terminals")
}

func (f *fakeOrcaInventory) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	return append([]port.OrcaTask(nil), f.tasks...), f.result("list-all-tasks")
}

func (f *fakeOrcaInventory) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	return append([]port.OrcaTask(nil), f.dispatchedTasks...), f.result("list-dispatched-tasks")
}

func (f *fakeOrcaInventory) ShowDispatch(_ context.Context, taskID string) (port.OrcaDispatch, error) {
	return f.dispatches[taskID], f.result("show-dispatch:" + taskID)
}

func (f *fakeOrcaInventory) ListGates(context.Context) ([]port.OrcaGate, error) {
	return append([]port.OrcaGate(nil), f.gates...), f.result("list-gates")
}

func (f *fakeOrcaInventory) InboxPresence(context.Context) (port.OrcaInboxPresence, error) {
	return f.inbox, f.result("inbox-presence")
}

var _ OrcaInventory = (*fakeOrcaInventory)(nil)
