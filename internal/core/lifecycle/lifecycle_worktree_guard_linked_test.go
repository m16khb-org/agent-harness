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

func TestWorktreeGuardAllowsSourceEditWhenLinkedWorktreeDirIsDeleted(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "1-x")
	linked := makeIssueOpsGuardWorktreeForTest(t, repo, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, linked); err != nil {
		t.Fatal(err)
	}

	// The isolated worktree directory is removed without releasing the cycle
	// (e.g. a prior session deleted it and a new session checked the branch out
	// in the source checkout). The stale cycle must not deadlock source edits.
	if err := os.RemoveAll(linked); err != nil {
		t.Fatal(err)
	}

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("source edit must not deadlock when the linked worktree dir was deleted, got %+v", res)
	}
}

func TestWorktreeGuardBlockMessageNamesForceReleaseEscape(t *testing.T) {
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
		t.Fatalf("source edit with a live linked worktree should block, got %+v", blocked)
	}
	if !strings.Contains(blocked.Reason, "force-release") {
		t.Fatalf("block reason must name the working escape command (force-release), got %q", blocked.Reason)
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
	recordIssueOpsWorktreeToolsForGuardTest(t, id, linked)
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

// TestWorktreeGuardBlockNamesAllParallelWorktreeCyclesDeterministically guards
// SCENARIO 1: when several parallel IssueOps cycles each hold a worktree and the
// user edits a shared source-checkout file, the block message must be
// deterministic and must NOT single out one arbitrary cycle as the force-release
// target (that cycle may be unrelated/live, and releasing it would not unblock
// the edit while the other worktrees remain — the non-working-escape trap).
func TestWorktreeGuardBlockNamesAllParallelWorktreeCyclesDeterministically(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// Current branch carries a non-worktree-phase (problem) cycle; two OTHER
	// branches each hold a live linked worktree.
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhaseProblem)
	cycleB := linkIssueOpsWorktreeForGuardTest(t, repo, "2-bravo")
	cycleC := linkIssueOpsWorktreeForGuardTest(t, repo, "3-charlie")

	first := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, "internal", "x.go")}, EnforceWorktree: true,
	})
	if first.Decision != "block" {
		t.Fatalf("source edit must block while parallel worktree cycles are active, got %+v", first)
	}
	// Both holders are named (no arbitrary single-cycle force-release target).
	if !strings.Contains(first.Reason, cycleB.id) || !strings.Contains(first.Reason, cycleC.id) {
		t.Fatalf("block reason must name every active worktree cycle (%s, %s), got %q", cycleB.id, cycleC.id, first.Reason)
	}
	// Branch-sorted order: "2-bravo" before "3-charlie".
	if strings.Index(first.Reason, cycleB.id) > strings.Index(first.Reason, cycleC.id) {
		t.Fatalf("block reason must order cycles deterministically by branch, got %q", first.Reason)
	}
	if !strings.Contains(first.Reason, "linked IssueOps worktree") || !strings.Contains(first.Reason, "force-release") {
		t.Fatalf("block reason must keep the worktree+force-release vocabulary, got %q", first.Reason)
	}
	// Deterministic across repeated evaluation (no os.ReadDir order dependence).
	second := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, "internal", "x.go")}, EnforceWorktree: true,
	})
	if first.Reason != second.Reason {
		t.Fatalf("block reason must be deterministic across calls:\n  first=%q\n  second=%q", first.Reason, second.Reason)
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
	if res.Decision != "allow" {
		t.Fatalf("source checkout edit without an active cycle on current branch should allow (no cycle-scoped blocking), got %+v", res)
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
