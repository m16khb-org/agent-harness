package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeGuardBlocksSourceEditWhenImplementCycleHasNoLinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-development", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "requires a linked isolated worktree") {
		t.Fatalf("unlinked implement cycle should block source checkout edits, got %+v", res)
	}
}

func TestWorktreeGuardAllowsWorktreeAddBeforeLinkWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-development", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git worktree add ../repo.worktrees/1-development origin/1-development", EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("worktree preparation command should pass before link-worktree, got %+v", res)
	}
}

func TestWorktreeGuardBlocksBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhasePlan)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git switch -c 2-new-work", EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "source ref") || !strings.Contains(res.Reason, "ask the user") {
		t.Fatalf("branch creation without source ref should block with user-source guidance, got %+v", res)
	}
}

func TestWorktreeGuardBlocksWorktreeBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhasePlan)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git worktree add -b 2-new-work ../repo.worktrees/2-new-work", EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "source ref") || !strings.Contains(res.Reason, "ask the user") {
		t.Fatalf("worktree branch creation without source ref should block with user-source guidance, got %+v", res)
	}
}

func TestWorktreeGuardAllowsDynamicBranchWorktreeScript(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhasePlan)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: `BR=2-new-work
git worktree add -b "$BR" ../repo.worktrees/2-new-work origin/main`, EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("dynamic branch variables should not be validated as literal branch names, got %+v", res)
	}
}

func TestWorktreeGuardBlocksLocalCheckoutOfIssueOpsBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-current", IssueOpsPhasePlan)
	if _, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "1-issue-work"}); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, repo, "1-issue-work", IssueOpsPhaseImplement)

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git checkout -b 1-issue-work origin/main", EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "must not be checked out in the source checkout") {
		t.Fatalf("local checkout of IssueOps branch should block: %+v", blocked)
	}
}

func TestWorktreeGuardNoBlockWithoutCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("no cycle for this branch should not block, got %+v", res)
	}
}

func TestWorktreeGuardNoBlockWhenCycleDone(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "1-x")
	markIssueOpsPRPhaseForTest(t, repo, "1-x")
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(IssueOpsPhaseDone)); err != nil {
		t.Fatal(err)
	}
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("done cycle should release the source checkout, got %+v", res)
	}
}

func TestWorktreeGuardNoBlockInPlanningPhase(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseGrill)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("planning phase expects no worktree yet; should not block, got %+v", res)
	}
}

func TestWorktreeGuardIgnoresOtherBranchCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// Active implement cycle for a different branch must not lock edits on main.
	repo := guardRepoWithCycle(t, "2-other", IssueOpsPhaseImplement)
	_ = os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("a cycle for a different branch must not lock the current branch, got %+v", res)
	}
}

func TestWorktreeGuardBlocksMismatchedWorktreeBranchAtLink(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-development", IssueOpsPhasePlan)
	recordID := newIssueOpsID(repo, "1-development")

	worktree := issueOpsGuardWorktreePathForTest(repo, "bugfix-2361")
	gitdir := filepath.Join(repo, ".git", "worktrees", "bugfix-2361")
	if err := os.MkdirAll(filepath.Join(worktree, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(worktree, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/bugfix/2361\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsGuardFileForTest(t, worktree, "docs/plans/2361.md", "plan\n")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-development")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), recordID, worktree); err == nil || !strings.Contains(err.Error(), "does not match IssueOps branch") {
		t.Fatalf("mismatched worktree branch should fail at link-worktree, got %v", err)
	}
}

func TestWorktreeGuardAllowsTempFileBashWrites(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "cat > /tmp/mr-body.md <<EOF\nbody\nEOF", EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("temp file bash writes should not be treated as source checkout edits, got %+v", res)
	}
}

func TestWorktreeGuardBlocksBashCommandThatChangesIntoUnlinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-x", IssueOpsPhaseImplement)
	worktree := issueOpsGuardWorktreePathForTest(repo, "1-x")
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "cd " + worktree + " && printf body > /tmp/mr-body.md", EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "requires a linked isolated worktree") {
		t.Fatalf("bash command scoped to unlinked worktree should block, got %+v", res)
	}
}
