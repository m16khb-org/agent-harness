package lifecycle

import (
	"agent-harness/internal/core/issueops"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeGuardAllowsSourceEditWhenImplementCycleHasNoLinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-development", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("unlinked implement cycle must not block ordinary source checkout edits, got %+v", res)
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

func TestWorktreeGuardAllowsIssueShapedBranchEditWithoutCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/2403-fix-dmm-fanza-account-merge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("issue-shaped branch without a cycle should remain ordinary source work, got %+v", res)
	}
}

func TestWorktreeGuardAllowsIssueOpsBootstrapOnIssueShapedBranchWithoutCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/2403-fix-dmm-fanza-account-merge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "agent-harness issueops start --repo . --branch 2403-fix-dmm-fanza-account-merge", EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("issueops bootstrap command should not be blocked on an issue-shaped branch without a cycle, got %+v", res)
	}
}

func TestWorktreeGuardBlocksIssueBranchCreationWithSourceRefWithoutCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git checkout -b 2403-fix-dmm-fanza-account-merge origin/main", EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "must be started through IssueOps before checking it out in the source checkout") {
		t.Fatalf("issue branch creation with source ref should block without an active cycle, got %+v", res)
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

func TestWorktreeGuardAllowsSourceEditWithoutParallelWorktreePathCollision(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "2518-main-maintenance", IssueOpsPhasePlan)
	linkIssueOpsWorktreeForGuardTest(t, repo, "2519-vertex-cache")

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, ".gitkeep")}, EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("source-only edit should remain allow when a parallel worktree lacks the path, got %+v", res)
	}
}

func TestWorktreeGuardAllowsSourceEditWithParallelWorktreePathCollision(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "2518-main-maintenance", IssueOpsPhasePlan)
	cycle := linkIssueOpsWorktreeForGuardTest(t, repo, "2519-vertex-cache")
	writeIssueOpsGuardFileForTest(t, cycle.path, ".gitkeep", "\n")

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, ".gitkeep")}, EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("ordinary source edit must remain available despite a parallel path collision: cycle=%s result=%+v", cycle.id, res)
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

func TestWorktreeGuardAllowsSessionBoundMirrorFileEditInSourceCheckout(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := sourceCheckoutRepoForMisdirectTest(t)
	cycle := linkIssueOpsWorktreeForGuardTest(t, repo, "2519-test-quality-comprehensive")
	writeIssueOpsGuardFileForTest(t, cycle.path, "src/a.ts", "export const a = 1;\n")
	if err := issueops.BindIssueOpsSession(repo, cycle.id, "2519-test-quality-comprehensive", cycle.path); err != nil {
		t.Fatal(err)
	}

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            repo,
		Tool:            "apply_patch",
		Paths:           []string{filepath.Join(repo, "src", "a.ts")},
		EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("session-bound mirror file edit must remain source-only work: cycle=%s result=%+v", cycle.id, res)
	}
}

func TestWorktreeGuardMirrorAskSkipsFalseCases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := sourceCheckoutRepoForMisdirectTest(t)
	cycle := linkIssueOpsWorktreeForGuardTest(t, repo, "2519-test-quality-comprehensive")

	t.Run("no session binding", func(t *testing.T) {
		res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:            repo,
			Tool:            "apply_patch",
			Paths:           []string{filepath.Join(repo, "src", "a.ts")},
			EnforceWorktree: true,
		})
		if res.Decision != "allow" {
			t.Fatalf("unbound source edit should remain allow, got %+v", res)
		}
	})

	if err := issueops.BindIssueOpsSession(repo, cycle.id, "2519-test-quality-comprehensive", cycle.path); err != nil {
		t.Fatal(err)
	}
	t.Run("missing mirror file", func(t *testing.T) {
		res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:            repo,
			Tool:            "apply_patch",
			Paths:           []string{filepath.Join(repo, "src", "new.ts")},
			EnforceWorktree: true,
		})
		if res.Decision != "allow" {
			t.Fatalf("source-only new file should remain allow, got %+v", res)
		}
	})

	setIssueOpsPhaseForTest(t, repo, "2519-test-quality-comprehensive", IssueOpsPhasePlan)
	writeIssueOpsGuardFileForTest(t, cycle.path, "src/plan.ts", "export const plan = true;\n")
	t.Run("binding cycle phase does not expect worktree", func(t *testing.T) {
		res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:            repo,
			Tool:            "apply_patch",
			Paths:           []string{filepath.Join(repo, "src", "plan.ts")},
			EnforceWorktree: true,
		})
		if res.Decision != "allow" {
			t.Fatalf("non-worktree phase should remain allow, got %+v", res)
		}
	})
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
