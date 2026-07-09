package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPreToolUseWorktreeGuardBlocksSourceCheckoutMutation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "apply_patch",
		Paths:            []string{filepath.Join(source, ".agent-harness", "OPERATIONS.md")},
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "expected IssueOps worktree") {
		t.Fatalf("expected source checkout mutation to be blocked: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsExpectedWorktreeMutation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             worktree,
		Tool:             "apply_patch",
		Paths:            []string{filepath.Join(worktree, ".agent-harness", "OPERATIONS.md")},
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected worktree mutation to be allowed: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsExpectedWorktreePreparation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "Bash",
		Command:          "git worktree add " + shellQuote(worktree) + " chore-19-docs",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected worktree preparation command to be allowed with explicit expected worktree: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsExpectedWorktreeAbsoluteTargetFromSourceRepo(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "Edit",
		Paths:            []string{filepath.Join(worktree, ".agent-harness", "OPERATIONS.md")},
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected absolute worktree target to be allowed even when repo is source checkout: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsShellRedirectAfterCdIntoExpectedWorktree(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "Bash",
		Command:          "cd " + shellQuote(worktree) + " && printf smoke > .issueops-hook-smoke",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected shell redirect after cd into worktree to be allowed: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardBlocksShellRedirectAfterCdIntoSourceCheckout(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "Bash",
		Command:          "cd " + shellQuote(source) + " && printf smoke > .issueops-hook-smoke",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "expected IssueOps worktree") {
		t.Fatalf("expected shell redirect after cd into source checkout to be blocked: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardDoesNotSpecialCaseExternalSearchMCPTools(t *testing.T) {
	// External code-intelligence MCP tools are the user's own installs; the
	// harness no longer validates or blocks their projectPath.
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "mcp__codegraph__codegraph_search",
		Command:          "BuildLifecyclePreToolUseDecision",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("external search MCP tool should pass through the worktree guard: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardBlocksSourceBoundMCPTools(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	for _, tool := range []string{"mcp__filesystem__read_file", "mcp__plugin_serena_serena__find_symbol"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:             source,
			Tool:             tool,
			Command:          "BuildLifecyclePreToolUseDecision",
			EnforceWorktree:  true,
			ExpectedWorktree: worktree,
			SourceCheckout:   source,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "IssueOps worktree") {
			t.Fatalf("expected %s to be blocked in IssueOps worktree context: %+v", tool, got)
		}
	}
}

func TestPreToolUseWorktreeGuardInfersExpectedWorktreeFromLinkedCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "1-current", IssueOpsPhaseProblem)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: source, Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, source, "1-x", IssueOpsPhaseImplement)
	worktree := makeIssueOpsGuardWorktreeForTest(t, source, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, source, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: source, Tool: "mcp__filesystem__read_file", Command: "BuildLifecyclePreToolUseDecision", EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, worktree) {
		t.Fatalf("linked IssueOps cycle should infer the expected worktree for the MCP guard: %+v", blocked)
	}
}

func TestPreToolUseWorktreeGuardInfersSourceBoundMCPBlockFromLinkedCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "1-current", IssueOpsPhaseProblem)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: source, Branch: "1-x"})
	if err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, source, "1-x", IssueOpsPhaseImplement)
	worktree := makeIssueOpsGuardWorktreeForTest(t, source, "1-x")
	linkIssueOpsBranchEvidenceForTest(t, source, "1-x")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}

	for _, tool := range []string{"mcp__filesystem__read_file", "mcp__plugin_serena_serena__find_symbol"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo: source, Tool: tool, Command: "BuildLifecyclePreToolUseDecision", EnforceWorktree: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "source-root-bound MCP tool") || !strings.Contains(got.Reason, worktree) {
			t.Fatalf("linked IssueOps cycle should block %s without explicit ExpectedWorktree: %+v", tool, got)
		}
	}
}

func TestPreToolUseWorktreeGuardNoopsWithoutExpectedWorktree(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            t.TempDir(),
		Tool:            "apply_patch",
		Paths:           []string{".agent-harness/OPERATIONS.md"},
		EnforceWorktree: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("missing expected worktree should not block: %+v", got)
	}
}
