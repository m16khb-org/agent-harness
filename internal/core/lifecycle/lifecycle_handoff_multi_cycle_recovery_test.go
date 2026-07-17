package lifecycle

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestHandoffMultiCycleBaselineBlocksAmbiguousSourceWriters(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "apply_patch", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "go test ./...", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "orca terminal send --terminal term-unknown --text guidance --enter --json", EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "block" || !strings.Contains(got.Reason, "ambiguous") && !strings.Contains(got.Reason, "multiple active") {
			t.Fatalf("multi-cycle source writer must remain fail-closed: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleAllowsExactLifecycleAndReadOnlyObservations(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	second := addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + " --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops resume --repo " + repo + " --id " + second.ID + " --json", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "pwd", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "rg -n handoff internal", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "git status --short", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "mcp__filesystem__read_text_file", Paths: []string{repo + "/AGENTS.md"}, EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact lifecycle and proven read-only observations must not deadlock: request=%#v result=%#v", req, got)
		}
	}
}

func TestHandoffMultiCycleKeepsMalformedAndMutatingRequestsBlocked(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	addSecondLifecycleHandoffRecord(t, repo, first)

	tests := []HookToolUseLifecycleRequest{
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + "; pwd", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "bin/agent-harness issueops status --id " + first.ID + " --unknown value", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "go test ./...", EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "apply_patch", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo},
		{Repo: repo, CWD: repo, Tool: "exec_command", Command: "orca terminal send --terminal term-unknown --text guidance --enter --json", EnforceWorktree: true, SourceCheckout: repo},
	}
	for _, req := range tests {
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("malformed and mutating multi-cycle requests must remain fail-closed: request=%#v result=%#v", req, got)
		}
	}
}

func addSecondLifecycleHandoffRecord(t *testing.T, repo string, first IssueOpsRecord) IssueOpsRecord {
	t.Helper()
	second := first
	second.ID = "io-second-cycle"
	second.Branch = "2-demo"
	second.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, second.Branch)
	second.ExecutionHandoff = cloneLifecycleHandoffForTest(t, first.ExecutionHandoff)
	second.ExecutionHandoff.WorkerRoot = second.WorktreePath
	second.ExecutionHandoff.OwnershipEpoch = "epoch-2"
	second.ExecutionHandoff.Orca.WorktreeID = "wt-2"
	second.ExecutionHandoff.Orca.WorktreePath = second.WorktreePath
	if second.ExecutionHandoff.Orca.TaskID != "" {
		second.ExecutionHandoff.Orca.TaskID = "task-2"
		second.ExecutionHandoff.Orca.DispatchID = "dispatch-2"
		second.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-2"
		second.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-2"
	}
	written, err := writeIssueOps(IssueOpsStateRoot(), second)
	if err != nil {
		t.Fatal(err)
	}
	return written
}
