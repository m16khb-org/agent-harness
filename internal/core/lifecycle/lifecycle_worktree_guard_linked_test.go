package lifecycle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestWorktreeGuardBlocksSourceEditWhenCycleHasLinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "1-x")
	linked := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, linked); err != nil {
		t.Fatal(err)
	}
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" {
		t.Fatalf("source-checkout edit should block when an exact worktree is linked, got %+v", blocked)
	}
	wtTarget := filepath.Join(linked, "internal", "x.go")
	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{wtTarget}, EnforceWorktree: true,
	})
	if allowed.Decision == "block" {
		t.Fatalf("edit targeting the isolated worktree should pass, got %+v", allowed)
	}
}

func TestWorktreeGuardBlocksSourceEditDuringAISlopClean(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "1-x")
	linked := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsLifecycleIntentForTest(t, id)
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), id, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/1",
		Branch:       "1-x",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, linked); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsLifecycleDesignForTest(t, id)
	writeIssueOpsGuardFileForTest(t, linked, "plans/demo.md", "plan\n")
	if _, err := LinkIssueOpsPlan(IssueOpsStateRoot(), id, filepath.Join(linked, "plans", "demo.md")); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsGuardFileForTest(t, linked, "internal/x.go", "package internal\n")
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(IssueOpsPhaseAISlopClean)); err != nil {
		t.Fatal(err)
	}
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "linked IssueOps worktree") {
		t.Fatalf("source-checkout edit should block during ai-slop-clean, got %+v", blocked)
	}
}

func TestWorktreeGuardBlocksOtherWorktreeWhenCycleHasExactWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "1-x")
	expected := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, expected); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "1-y", "internal", "x.go")
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{other}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "linked IssueOps worktree") {
		t.Fatalf("other worktree edit should block when exact worktree is linked: %+v", blocked)
	}

	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(expected, "internal", "x.go")}, EnforceWorktree: true,
	})
	if allowed.Decision != "allow" {
		t.Fatalf("linked IssueOps worktree edit should pass: %+v", allowed)
	}
}

func TestWorktreeGuardBlocksSourceCheckoutWhenLinkedCycleExists(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhaseProblem)
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, repo, "1-x", IssueOpsPhaseImplement)
	expected := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), rec.ID, expected); err != nil {
		t.Fatal(err)
	}

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, "internal", "x.go")}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "linked IssueOps worktree") {
		t.Fatalf("other branch linked worktree should block source checkout edits: %+v", blocked)
	}

	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(expected, "internal", "x.go")}, EnforceWorktree: true,
	})
	if allowed.Decision != "allow" {
		t.Fatalf("linked issue worktree edit should pass even when source checkout is main: %+v", allowed)
	}
}

func TestWorktreeGuardBlocksOtherWorktreeWhenCurrentBranchCycleIsUnlinked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhaseImplement)
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, repo, "1-x", IssueOpsPhaseImplement)
	expected := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), rec.ID, expected); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "1-y", "internal", "x.go")
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{other}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "requires a linked isolated worktree") {
		t.Fatalf("unlinked current branch cycle should block unrelated worktree edits: %+v", blocked)
	}
}

func TestWorktreeGuardAllowsAnyActiveLinkedIssueOpsWorktreeForRepo(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtures := []linkedIssueOpsWorktreeForTest{
		linkIssueOpsWorktreeForGuardTest(t, repo, "95-tier-matrix-scan-allowance-policy"),
		linkIssueOpsWorktreeForGuardTest(t, repo, "96-integrate-public-seo-rendering"),
		linkIssueOpsWorktreeForGuardTest(t, repo, "97-parallel-worktree-fixture"),
	}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].id < fixtures[j].id
	})
	target := fixtures[len(fixtures)-1]

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            repo,
		Tool:            "Edit",
		Paths:           []string{filepath.Join(repo, "internal", "core", "issueops.go")},
		EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "linked IssueOps worktree") {
		t.Fatalf("source checkout edit should still block, got %+v", res)
	}

	res = BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            repo,
		Tool:            "Edit",
		Paths:           []string{filepath.Join(target.path, "internal", "core", "issueops.go")},
		EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("edit inside any active linked IssueOps worktree should allow; first=%s target=%s got %+v", fixtures[0].path, target.path, res)
	}
}
