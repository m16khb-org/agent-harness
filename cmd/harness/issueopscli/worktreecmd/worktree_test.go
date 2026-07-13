package worktreecmd

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/testsupport"
)

func TestRunWorktreeCommandsWithInjectedDeps(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := worktreeIssueOpsRecord(t)
	var printed []any
	var errorsPrinted []error
	deps := Deps{
		ParseFlags: parseWorktreeFlags,
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintError: func(err error) error {
			errorsPrinted = append(errorsPrinted, err)
			return nil
		},
		PrepareHandoff: func(_ context.Context, _ string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
			return core.IssueOpsHandoffPrepareResult{OK: true, ID: req.ID, Repo: record.Repo, Branch: record.Branch, BaseBranch: "main", WorktreePath: record.Repo + ".worktrees/" + record.Branch, Command: []string{"git", "worktree", "add"}}, nil
		},
	}
	if err := Run([]string{"prepare", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("prepare returned error: %v", err)
	}
	if err := Run([]string{"verify", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
	if err := Run([]string{"cleanup-readiness", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("cleanup-readiness returned error: %v", err)
	}
	if err := Run([]string{"prepare", "--id", record.ID}, deps); err != nil {
		t.Fatalf("prepare text returned error: %v", err)
	}
	if err := Run([]string{"verify", "--id", record.ID}, deps); err != nil {
		t.Fatalf("verify text returned error: %v", err)
	}
	if err := Run([]string{"cleanup-readiness", "--id", record.ID}, deps); err != nil {
		t.Fatalf("cleanup-readiness text returned error: %v", err)
	}
	if len(printed) != 3 {
		t.Fatalf("expected 3 JSON outputs, got %d", len(printed))
	}
	if err := Run([]string{"verify", "--id", "missing", "--json"}, deps); err == nil {
		t.Fatal("expected missing verify error")
	}
	if len(errorsPrinted) != 1 {
		t.Fatalf("expected printed error, got %d", len(errorsPrinted))
	}
}

func TestWorktreePrepareCLIForwardsOrchestratorAgentAndConfirmation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := worktreeIssueOpsRecord(t)
	var captured core.IssueOpsHandoffPrepareRequest
	var printed any
	deps := Deps{
		ParseFlags: parseWorktreeFlags,
		PrintJSON:  func(value any) error { printed = value; return nil },
		PrintError: func(error) error { return nil },
		PrepareHandoff: func(_ context.Context, _ string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
			captured = req
			return core.IssueOpsHandoffPrepareResult{OK: true, ID: req.ID, RequestedMode: req.Orchestrator, ResolvedMode: "orca", Preview: !req.Confirm}, nil
		},
	}
	if err := Run([]string{"prepare", "--id", record.ID, "--orchestrator", "inline", "--inline-reason", "user-requested", "--agent", "claude", "--confirm", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if captured.ID != record.ID || captured.Orchestrator != "inline" || captured.InlineReason != "user-requested" || captured.Agent != "claude" || !captured.Confirm {
		t.Fatalf("flags not forwarded: %#v", captured)
	}
	if _, ok := printed.(core.IssueOpsHandoffPrepareResult); !ok {
		t.Fatalf("expected typed result, got %T", printed)
	}
}

func TestWorktreePrepareCLIRejectsInlineWithoutAuthorizationBeforeDependency(t *testing.T) {
	called := false
	deps := Deps{
		ParseFlags: parseWorktreeFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		PrepareHandoff: func(context.Context, string, core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
			called = true
			return core.IssueOpsHandoffPrepareResult{}, nil
		},
	}
	err := Run([]string{"prepare", "--id", "io-demo", "--orchestrator", "inline", "--json"}, deps)
	if err == nil || err.Error() != "explicit inline requires --inline-reason user-requested|recovery" {
		t.Fatalf("missing inline authorization error = %v", err)
	}
	if called {
		t.Fatal("missing inline authorization reached prepare dependency")
	}
}

func TestWorktreePrepareCLIRejectsInvalidInlineAuthorization(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := worktreeIssueOpsRecord(t)
	err := Run([]string{"prepare", "--id", record.ID, "--orchestrator", "inline", "--inline-reason", "simpler", "--json"}, Deps{
		ParseFlags: parseWorktreeFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		PrepareHandoff: func(ctx context.Context, stateRoot string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
			return core.PrepareIssueOpsHandoffWorktree(ctx, stateRoot, req, nil, core.IssueOpsHandoffPrepareClock{})
		},
	})
	if err == nil || err.Error() != "inline reason must be user-requested or recovery" {
		t.Fatalf("invalid inline authorization error = %v", err)
	}
}

func TestWorktreePrepareHumanOutputPrintsWarnings(t *testing.T) {
	const warning = "orca_gitlab_native_metadata_unavailable"
	deps := Deps{
		ParseFlags: parseWorktreeFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		PrepareHandoff: func(context.Context, string, core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
			return core.IssueOpsHandoffPrepareResult{
				OK: true, Branch: "16-demo", BaseBranch: "main", WorktreePath: "/repo.worktrees/16-demo",
				ResolvedMode: "orca", State: "coordinator_preparing",
				Warnings: []string{warning},
			}, nil
		},
	}
	out := testsupport.CaptureStdout(t, func() error {
		return Run([]string{"prepare", "--id", "io-demo"}, deps)
	})
	if !strings.Contains(out, "warning: "+warning+"\n") {
		t.Fatalf("human prepare output omitted warning:\n%s", out)
	}
}

func TestWorktreePrepareAutoFallbackHumanOutputIsByteExactLegacy(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := worktreeIssueOpsRecord(t)
	worktreePath := filepath.Join(record.Repo+".worktrees", strings.ReplaceAll(record.Branch, "/", "-"))
	legacy := core.IssueOpsHandoffPrepareResult{
		OK: true, ID: record.ID, Repo: record.Repo, Branch: record.Branch, BaseBranch: "main",
		WorktreePath: worktreePath, Command: []string{"git", "worktree", "add", worktreePath, record.Branch},
	}
	run := func(prepare func(context.Context, string, core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error)) string {
		return testsupport.CaptureStdout(t, func() error {
			return Run([]string{"prepare", "--id", record.ID, "--orchestrator", "auto", "--confirm"}, Deps{
				ParseFlags:     parseWorktreeFlags,
				PrintJSON:      func(any) error { return nil },
				PrintError:     func(error) error { return nil },
				PrepareHandoff: prepare,
			})
		})
	}
	actual := func(ctx context.Context, stateRoot string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
		return core.PrepareIssueOpsHandoffWorktree(ctx, stateRoot, req, nil, core.IssueOpsHandoffPrepareClock{})
	}
	expected := func(context.Context, string, core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
		return legacy, nil
	}
	if got, want := run(actual), run(expected); got != want {
		t.Fatalf("auto fallback text changed legacy bytes:\n got: %q\nwant: %q", got, want)
	}
}

func TestPrepareWorktreeToolsSuccessAndErrors(t *testing.T) {
	tmp := t.TempDir()
	worktree := filepath.Join(tmp, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := PrepareWorktreeTools(core.IssueOpsRecord{ID: "io-1", WorktreePath: worktree})
	if err != nil {
		t.Fatalf("PrepareWorktreeTools returned error: %v", err)
	}
	if !result.OK || result.WorktreePath != worktree {
		t.Fatalf("unexpected prepare tools result: %#v", result)
	}
	if want := "export HARNESS_EXPECTED_WORKTREE=" + worktree; result.Guidance != want {
		t.Fatalf("expected guidance %q, got %q", want, result.Guidance)
	}
	if _, err := PrepareWorktreeTools(core.IssueOpsRecord{}); err == nil || !strings.Contains(err.Error(), "worktree_path is required") {
		t.Fatalf("expected missing worktree error, got %v", err)
	}
	if _, err := PrepareWorktreeTools(core.IssueOpsRecord{WorktreePath: filepath.Join(tmp, "missing")}); err == nil {
		t.Fatal("expected missing directory error")
	}
}

func TestRunPrepareToolsAndBoundaries(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := worktreeIssueOpsRecord(t)
	worktree := filepath.Join(record.Repo+".worktrees", record.Branch)
	initWorktreeGitRepo(t, record.Repo)
	runWorktreeGit(t, record.Repo, "worktree", "add", "-b", record.Branch, worktree)
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1234"); err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/acme/repo/issues/1234",
		Branch:       record.Branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatalf("PrepareIssueOpsBranch: %v", err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatalf("LinkIssueOpsWorktree: %v", err)
	}
	var printed []any
	deps := Deps{ParseFlags: parseWorktreeFlags, PrintJSON: func(value any) error {
		printed = append(printed, value)
		return nil
	}, PrintError: func(error) error { return nil }, PrepareHandoff: func(_ context.Context, _ string, req core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error) {
		return core.IssueOpsHandoffPrepareResult{OK: true, ID: req.ID, Repo: record.Repo, Branch: record.Branch, BaseBranch: "main", WorktreePath: worktree}, nil
	}}
	if err := Run([]string{"prepare-tools", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("prepare-tools returned error: %v", err)
	}
	if err := Run([]string{"prepare-tools", "--id", record.ID}, deps); err != nil {
		t.Fatalf("prepare-tools text returned error: %v", err)
	}
	if len(printed) != 1 {
		t.Fatalf("expected prepare-tools JSON output, got %d", len(printed))
	}
	if result, ok := printed[0].(PrepareResult); !ok {
		t.Fatalf("expected PrepareResult JSON output, got %T", printed[0])
	} else if want := "export HARNESS_EXPECTED_WORKTREE=" + worktree; result.Guidance != want {
		t.Fatalf("expected guidance %q, got %q", want, result.Guidance)
	}
	if err := Run(nil, deps); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if err := Run([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops worktree") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
	if err := Run([]string{"prepare", "--bad"}, deps); err == nil {
		t.Fatal("expected flag parse error")
	}
}

func worktreeIssueOpsRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "1234-worktree-cmd"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	return record
}

func initWorktreeGitRepo(t *testing.T, repo string) {
	t.Helper()
	runWorktreeGit(t, repo, "init")
	runWorktreeGit(t, repo, "config", "user.email", "test@example.com")
	runWorktreeGit(t, repo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runWorktreeGit(t, repo, "add", "README.md")
	runWorktreeGit(t, repo, "commit", "-m", "initial")
}

func runWorktreeGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
}

func parseWorktreeFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
