package worktreecmd

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-harness/internal/core"
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

func TestPrepareWorktreeToolsSuccessAndErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake codegraph shell script is POSIX-specific")
	}
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	codegraph := filepath.Join(fakeBin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	worktree := filepath.Join(tmp, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := PrepareWorktreeTools(core.IssueOpsRecord{ID: "io-1", WorktreePath: worktree})
	if err != nil {
		t.Fatalf("PrepareWorktreeTools returned error: %v", err)
	}
	if !result.OK || !result.CodeGraphChecked || !result.CodeGraphReady || result.WorktreePath != worktree {
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
	if runtime.GOOS == "windows" {
		t.Skip("fake codegraph shell script is POSIX-specific")
	}
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "codegraph"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	}, PrintError: func(error) error { return nil }}
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
