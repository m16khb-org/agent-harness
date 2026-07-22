package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func TestExecutionV1MatrixAllowsObservationAndDeniesSourceOrForeignMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "1-owner-a", IssueOpsPhaseProblem)
	ownerA := linkIssueOpsWorktreeForGuardTest(t, source, "1-owner-a")
	ownerB := linkIssueOpsWorktreeForGuardTest(t, source, "2-owner-b")

	base := HookToolUseLifecycleRequest{
		Repo:             ownerA.path,
		CWD:              ownerA.path,
		Host:             "codex",
		SessionID:        "owner-a-session",
		EnforceWorktree:  true,
		ExpectedWorktree: ownerA.path,
		SourceCheckout:   source,
	}

	for name, testCase := range map[string][2]string{
		"source read":  {"Read", filepath.Join(source, "README.md")},
		"foreign read": {"Read", filepath.Join(ownerB.path, "README.md")},
	} {
		req := base
		req.Tool = testCase[0]
		req.Paths = []string{testCase[1]}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("%s must be observation-first even with two active cycles: %+v", name, got)
		}
	}

	for name, target := range map[string]string{
		"source mutation":  filepath.Join(source, "README.md"),
		"foreign mutation": filepath.Join(ownerB.path, "README.md"),
	} {
		req := base
		req.Tool = "apply_patch"
		req.Paths = []string{target}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("%s must be denied outside the assigned canonical worktree: %+v", name, got)
		}
	}
}

func TestExecutionV1DirectCannotImplementInSourceWithoutWorkspace(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "69-direct", IssueOpsPhaseImplement)

	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            source,
		CWD:             source,
		Host:            "codex",
		SessionID:       "direct-session",
		Tool:            "apply_patch",
		Paths:           []string{filepath.Join(source, "internal", "issueops.go")},
		EnforceWorktree: true,
		SourceCheckout:  source,
	})
	if got.Decision != "block" {
		t.Fatalf("direct execution without a canonical worktree must not mutate source: %+v", got)
	}
}

func TestExecutionV1ParallelCycleObservationsDoNotRequireOwnerSelection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, active, worker := executionV1ActiveLifecycleRecord(t)
	observer, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "70-observer"})
	if err != nil {
		t.Fatal(err)
	}

	for name, command := range map[string]string{
		"exact status": "agent-harness issueops status --id " + observer.ID + " --json",
		"remote score": "agent-harness issueops remote score --input " + filepath.Join(worker, "score-input.json") + " --judge none --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionV1Request(active, worker, "claude", "owner-session", "")
			req.AgentID = "owner-agent"
			req.Tool = "Bash"
			req.Command = command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("observation must ignore unrelated active cycles: %+v", got)
			}
		})
	}
}

func TestExecutionV1ShellReadersAreObservationFirst(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, active, worker := executionV1ActiveLifecycleRecord(t)
	commands := []string{
		"cat " + filepath.Join(repo, "README.md"),
		"head -n 5 " + filepath.Join(repo, "README.md"),
		"tail -n 5 " + filepath.Join(repo, "README.md"),
		"ls -la " + repo,
		"find " + repo + " -maxdepth 1 -type f",
		"stat " + filepath.Join(repo, "README.md"),
		"file " + filepath.Join(repo, "README.md"),
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			req := executionV1Request(active, worker, "claude", "owner-session", "")
			req.AgentID, req.Tool, req.Command = "owner-agent", "Bash", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("bounded shell reader must be allowed before cycle selection: %+v", got)
			}
		})
	}
}

func TestExecutionV1MutationClassCoversBuildGitFilesystemAndUnsafeShell(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionV1ActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"test":      "go test ./... -count=1",
		"build":     "go build ./...",
		"benchmark": "go test -bench=. ./...",
		"git push":  "git push origin HEAD",
	} {
		t.Run(name+" source denied", func(t *testing.T) {
			req := executionV1Request(record, source, "claude", "owner-session", command)
			req.Repo, req.AgentID = source, "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("mutation-class command in source must be denied: %+v", got)
			}
		})
	}

	holderTest := executionV1Request(record, worker, "claude", "owner-session", "go test ./... -count=1")
	holderTest.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holderTest); got.Decision != "allow" {
		t.Fatalf("foreground test in the assigned holder root must be allowed: %+v", got)
	}

	filesystemWrite := executionV1Request(record, source, "claude", "owner-session", "")
	filesystemWrite.Repo, filesystemWrite.AgentID = source, "owner-agent"
	filesystemWrite.Tool = "mcp__filesystem__write_file"
	filesystemWrite.Paths = []string{filepath.Join(source, "generated.txt")}
	filesystemWrite.ToolInput = map[string]any{"path": filesystemWrite.Paths[0], "content": "x"}
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "block" {
		t.Fatalf("filesystem write MCP in source must be denied: %+v", got)
	}
	filesystemWrite.Tool = "mcp__filesystem__append_file"
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "block" {
		t.Fatalf("unenumerated filesystem write MCP in source must fail closed: %+v", got)
	}
	filesystemWrite.Tool = "mcp__filesystem__read_file"
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "allow" {
		t.Fatalf("explicit filesystem reader must remain observation-first: %+v", got)
	}

	for name, command := range map[string]string{
		"background":       "go test ./... &",
		"detached wrapper": "nohup go test ./...",
		"unknown wrapper":  "./scripts/verify.sh",
	} {
		t.Run(name+" denied for holder", func(t *testing.T) {
			req := executionV1Request(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unsafe shell form must be denied even for the current holder: %+v", got)
			}
		})
	}
}

func TestExecutionV1MutationFailsClosedWhenAuthorityStateIsCorrupt(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionV1ActiveLifecycleRecord(t)
	db, err := sqlstore.Open(IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops_v1", "io-aaaaaaaaaaaa", []byte(`{`)); err != nil {
		t.Fatal(err)
	}

	req := executionV1Request(record, worker, "claude", "owner-session", "go test ./... -count=1")
	req.AgentID = "owner-agent"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "authority state") {
		t.Fatalf("corrupt IssueOps v1 authority must fail closed: %+v", got)
	}
}

func TestExecutionV1HolderCannotMutateGitTopology(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionV1ActiveLifecycleRecord(t)
	for name, command := range map[string]string{
		"branch switch":  "git switch other-branch",
		"reset":          "git reset --hard HEAD~1",
		"rebase":         "git rebase origin/main",
		"merge":          "git merge other-branch",
		"force push":     "git push --force origin HEAD",
		"force refspec":  "git push origin +HEAD:refs/heads/main",
		"mirror push":    "git push --mirror origin",
		"remote delete":  "git push origin :refs/heads/obsolete",
		"worktree prune": "git worktree prune",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionV1Request(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("current holder must not change the sealed Git topology: %+v", got)
			}
		})
	}
}

func TestExecutionV1BlocksUnregisteredSiblingWorktreeWithoutHookEnvironment(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, _ := executionV1ActiveLifecycleRecord(t)
	sibling := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "unregistered")
	gitDir := filepath.Join(source, ".git", "worktrees", "unregistered")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := executionV1Request(record, sibling, "claude", "owner-session", "")
	req.Repo = sibling
	req.SourceCheckout = ""
	req.ExpectedWorktree = ""
	req.AgentID = "owner-agent"
	req.Tool = "apply_patch"
	req.Paths = []string{filepath.Join(sibling, "foreign.go")}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("unregistered sibling worktree mutation must be denied without hook environment hints: %+v", got)
	}
}

func TestExecutionV1MutationTargetsSelectAuthorityFromUnrelatedHookCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionV1ActiveLifecycleRecord(t)
	unrelated := t.TempDir()
	sibling := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "foreign-target")
	gitDir := filepath.Join(source, ".git", "worktrees", "foreign-target")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for name, target := range map[string]string{
		"source":    filepath.Join(source, "absolute-source.go"),
		"canonical": filepath.Join(worker, "absolute-owner.go"),
		"sibling":   filepath.Join(sibling, "absolute-foreign.go"),
	} {
		t.Run(name+" mutation", func(t *testing.T) {
			req := HookToolUseLifecycleRequest{
				Repo: unrelated, CWD: unrelated, Host: "codex", SessionID: "unrelated-session",
				Tool: "apply_patch", Paths: []string{target}, EnforceWorktree: true,
			}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("absolute %s mutation target escaped authority selection: %+v", name, got)
			}
		})
	}

	read := HookToolUseLifecycleRequest{
		Repo: unrelated, CWD: unrelated, Tool: "Read", Paths: []string{filepath.Join(source, "README.md")}, EnforceWorktree: true,
	}
	if got := BuildLifecyclePreToolUseDecision(read); got.Decision != "allow" {
		t.Fatalf("absolute read target must remain observation-first: %+v", got)
	}
	_ = record
}

func TestExecutionV1HolderCannotEscapeCanonicalRootThroughSymlinkTarget(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionV1ActiveLifecycleRecord(t)
	escape := filepath.Join(worker, "escape-to-source")
	if err := os.Symlink(source, escape); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"existing file":    filepath.Join(escape, "README.md"),
		"nonexistent leaf": filepath.Join(escape, "new", "generated.go"),
	} {
		t.Run(name, func(t *testing.T) {
			req := executionV1Request(record, worker, "claude", "owner-session", "")
			req.AgentID, req.Tool, req.Paths = "owner-agent", "apply_patch", []string{target}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("symlink escape target was allowed: %+v", got)
			}
		})
	}

	req := executionV1Request(record, worker, "claude", "owner-session", "")
	req.AgentID, req.Tool, req.Paths = "owner-agent", "apply_patch", []string{filepath.Join(worker, "safe", "generated.go")}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("ordinary nonexistent leaf inside canonical root was denied: %+v", got)
	}
}

func TestExecutionV1HolderOutputAndDetachedFlagsFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionV1ActiveLifecycleRecord(t)
	commands := map[string]string{
		"go coverprofile":  "go test -coverprofile=" + filepath.Join(source, "coverage.out") + " ./...",
		"go trace":         "go test -trace=" + filepath.Join(source, "trace.out") + " ./...",
		"go cpuprofile":    "go test -cpuprofile=" + filepath.Join(source, "cpu.out") + " ./...",
		"go memprofile":    "go test -memprofile=" + filepath.Join(source, "mem.out") + " ./...",
		"go blockprofile":  "go test -blockprofile=" + filepath.Join(source, "block.out") + " ./...",
		"go mutexprofile":  "go test -mutexprofile=" + filepath.Join(source, "mutex.out") + " ./...",
		"go outputdir":     "go test -outputdir=" + source + " ./...",
		"target directory": "cp --target-directory=" + source + " local.txt",
		"detach equals":    "go run ./cmd/server --detach=true",
		"daemon equals":    "go run ./cmd/server --daemon=true",
		"daemonize flag":   "go run ./cmd/server --daemonize",
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			req := executionV1Request(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unsafe output/detached command was allowed: %q => %+v", command, got)
			}
		})
	}

	safe := executionV1Request(record, worker, "claude", "owner-session", "go test -coverprofile="+filepath.Join(worker, "coverage.out")+" ./...")
	safe.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(safe); got.Decision != "allow" {
		t.Fatalf("foreground output wholly inside canonical root was denied: %+v", got)
	}
}

func executionV1Request(record IssueOpsRecord, cwd, host, sessionID, command string) HookToolUseLifecycleRequest {
	repo := record.Repo
	var processAncestry []issueopsmodel.NativeProcessReceiptV1
	if record.Execution != nil {
		repo = record.Execution.Workspace.Root
		if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
			processAncestry = []issueopsmodel.NativeProcessReceiptV1{*holder.SessionProcess}
		}
	}
	return HookToolUseLifecycleRequest{
		Repo: repo, CWD: cwd, SourceCheckout: record.Repo,
		Host: host, SessionID: sessionID, Tool: "Bash", Command: command,
		EnforceWorktree: true, NativeProcessAncestry: processAncestry,
	}
}

func executionV1ActiveLifecycleRecord(t *testing.T) (string, IssueOpsRecord, string) {
	t.Helper()
	repo := guardRepoWithCycle(t, "69-v1-observation", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, repo, "69-v1-observation")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.ExecutionV1{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.WorkspaceV1{
			SourceRoot: repo, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLeaseV1{
			Generation: 1, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopsmodel.NativeActorV1{
				Host: "claude", SessionID: "owner-session", AgentID: "owner-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceiptV1{PID: 1234, StartedAt: "2026-07-22T00:00:00Z", Executable: "claude"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return repo, record, linked.path
}

func TestExecutionV1LeaseAllowsOnlyCurrentHolderInCanonicalRoot(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "68-source", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, source, "69-v1-holder")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.ExecutionV1{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.WorkspaceV1{
			SourceRoot: source, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLeaseV1{
			Generation: 4, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopsmodel.NativeActorV1{
				Host: "codex", SessionID: "holder-session", AgentID: "holder-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceiptV1{PID: 1234, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	holder := HookToolUseLifecycleRequest{
		Repo: linked.path, CWD: linked.path, SourceCheckout: source,
		Host: "codex", SessionID: "holder-session", AgentID: "holder-agent",
		Tool: "apply_patch", Paths: []string{filepath.Join(linked.path, "internal", "v1.go")},
		EnforceWorktree:       true,
		NativeProcessAncestry: []issueopsmodel.NativeProcessReceiptV1{*record.Execution.Lease.Holder.SessionProcess},
	}
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("current holder mutation in canonical root was denied: %+v", got)
	}

	wrongSession := holder
	wrongSession.SessionID = "other-session"
	if got := BuildLifecyclePreToolUseDecision(wrongSession); got.Decision != "block" || !strings.Contains(got.Reason, "current write lease") {
		t.Fatalf("non-holder mutation was not denied by the v1 lease: %+v", got)
	} else if got.Deny == nil || got.Deny.Code != "write_lease_required" || got.Deny.LifecycleID != linked.id ||
		!sameExecutionV1Path(got.Deny.ExpectedRoot, linked.path) || got.Deny.CurrentGeneration != 4 ||
		!strings.Contains(got.Deny.NextCommand, "issueops execution status --id "+linked.id) {
		t.Fatalf("non-holder deny did not expose the structured v1 escape contract: %+v", got)
	}

	reusedSession := holder
	reusedSession.NativeProcessAncestry = []issueopsmodel.NativeProcessReceiptV1{{
		PID: 1234, StartedAt: "2026-07-22T00:00:01Z", Executable: "codex",
	}}
	if got := BuildLifecyclePreToolUseDecision(reusedSession); got.Decision != "block" || !strings.Contains(got.Reason, "current write lease") {
		t.Fatalf("reused session id from a different process identity was not denied: %+v", got)
	}

	missingProcess := holder
	missingProcess.NativeProcessAncestry = nil
	if got := BuildLifecyclePreToolUseDecision(missingProcess); got.Decision != "block" || !strings.Contains(got.Reason, "current write lease") {
		t.Fatalf("holder mutation without locally observed process identity was not denied: %+v", got)
	}

	sourceMutation := holder
	sourceMutation.Repo, sourceMutation.CWD = source, source
	sourceMutation.Paths = []string{filepath.Join(source, "internal", "v1.go")}
	if got := BuildLifecyclePreToolUseDecision(sourceMutation); got.Decision != "block" || !strings.Contains(got.Reason, linked.path) {
		t.Fatalf("holder source mutation was not redirected to canonical root: %+v", got)
	}
}

func TestExecutionV1RevokingLeaseDeniesOldHolderWithFiniteNextCommand(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "68-source", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, source, "69-v1-revoking")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.ExecutionV1{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.WorkspaceV1{
			SourceRoot: source, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLeaseV1{
			Generation: 5, Status: issueopsmodel.LeaseStatusRevoking,
			Holder: &issueopsmodel.NativeActorV1{
				Host: "codex", SessionID: "old-session",
				SessionProcess: &issueopsmodel.NativeProcessReceiptV1{PID: 999999, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	req := HookToolUseLifecycleRequest{
		Repo: linked.path, CWD: linked.path, SourceCheckout: source,
		Host: "codex", SessionID: "old-session", Tool: "apply_patch",
		Paths: []string{filepath.Join(linked.path, "internal", "late.go")}, EnforceWorktree: true,
	}
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "--finalize-preview") || !strings.Contains(got.Reason, linked.id) {
		t.Fatalf("revoking lease did not return one finite next command: %+v", got)
	}
	if got.Deny == nil || got.Deny.Code != "lease_revoking" || got.Deny.LifecycleID != linked.id ||
		!sameExecutionV1Path(got.Deny.ExpectedRoot, linked.path) || got.Deny.CurrentGeneration != 5 ||
		!strings.Contains(got.Deny.NextCommand, "--finalize-preview") {
		t.Fatalf("revoking lease did not expose the structured v1 escape contract: %+v", got)
	}

	req.Tool = "Bash"
	req.Paths = nil
	req.Repo, req.CWD = source, source
	req.Command = "agent-harness issueops execution replace --id " + linked.id + " --finalize-preview --expected-generation 5 --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact finalize-preview must be reachable from a fresh source session: %+v", got)
	}
}
