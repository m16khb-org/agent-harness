package mcpcli

import (
	"testing"
	"time"

	"agent-harness/internal/core"
)

// TestMCPIssueOpsCleanupStalePruneDoneArg verifies the issueops_cleanup_stale MCP
// handler threads the prune_done argument into IssueOpsStaleScanRequest.PruneDoneAge
// (default 720h, matching the CLI --prune-done flag), so the tool can prune done
// cycles instead of leaving PruneDoneAge unset.
func TestMCPIssueOpsCleanupStalePruneDoneArg(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	stateRoot := core.IssueOpsStateRoot()

	// writeDoneCycle seeds an 800h-old done cycle (past the 720h default) and
	// returns its id.
	writeDoneCycle := func(branch string) string {
		t.Helper()
		ts := time.Now().Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano)
		id := core.NewIssueOpsID(repo, branch)
		if _, err := core.WriteIssueOps(stateRoot, core.IssueOpsRecord{
			OK:        true,
			ID:        id,
			Repo:      repo,
			Branch:    branch,
			Phase:     core.IssueOpsPhaseDone,
			CreatedAt: ts,
			UpdatedAt: ts,
		}); err != nil {
			t.Fatalf("WriteIssueOps: %v", err)
		}
		return id
	}

	scan := func(args map[string]any) core.IssueOpsStaleScanResult {
		t.Helper()
		outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_cleanup_stale", Arguments: args})
		if outcome.Err != nil {
			t.Fatalf("cleanup_stale outcome error: %#v", outcome.Err)
		}
		result, ok := outcome.Payload.(core.IssueOpsStaleScanResult)
		if !ok {
			t.Fatalf("cleanup_stale payload type = %T", outcome.Payload)
		}
		return result
	}

	// Default prune_done (720h): the 800h-old done cycle is past the retention
	// window and is pruned. This fails if the handler left PruneDoneAge unset
	// (the bug), which would prune nothing.
	writeDoneCycle("prune-default")
	if res := scan(map[string]any{"repo": repo, "apply": true}); res.PrunedDone != 1 {
		t.Fatalf("default prune_done should prune the 800h-old done cycle, got PrunedDone=%d (errors=%v)", res.PrunedDone, res.Errors)
	}

	// Explicit prune_done well beyond the cycle age keeps it: this proves the arg
	// value (not the default) is threaded into PruneDoneAge.
	writeDoneCycle("prune-keep")
	if res := scan(map[string]any{"repo": repo, "apply": true, "prune_done": "100000h"}); res.PrunedDone != 0 {
		t.Fatalf("large prune_done should keep the 800h-old done cycle, got PrunedDone=%d (errors=%v)", res.PrunedDone, res.Errors)
	}

	// An invalid duration is a validation error, mirroring the CLI.
	bad := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_cleanup_stale", Arguments: map[string]any{"repo": repo, "prune_done": "not-a-duration"}})
	if !bad.IsError {
		t.Fatalf("invalid prune_done should be a normalized error result, got %#v", bad)
	}
}
