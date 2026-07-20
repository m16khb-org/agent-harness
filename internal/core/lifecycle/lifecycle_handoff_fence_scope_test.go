package lifecycle

import (
	"strings"
	"testing"
)

// TestFenceScopeNarrowingUnblocksDifferentCycleFromSource proves the Task F2
// scope rule: an ordinary source-root request must not be captured by a
// stranded recovery_required record. Exact-cycle control remains fenced.
func TestFenceScopeNarrowingUnblocksDifferentCycleFromSource(t *testing.T) {
	repo, record, _ := strandedRecoveryRequiredRecord(t)

	// A different, unrelated cycle id (not a supervised handoff record fencing
	// this checkout).
	otherID := "io-unrelated-cycle"

	allowed := []string{
		"agent-harness issueops status --id " + otherID + " --json",
		"agent-harness issueops resume --repo " + repo + " --id " + otherID + " --bind --json",
	}
	for _, command := range allowed {
		req := handoffEditRequest(record, repo, "codex", "any-session", "")
		req.Tool = "Bash"
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("command targeting a different cycle id must not be fenced by the stranded record: command=%q got=%#v", command, got)
		}
	}

	// Companion: an id-less source-root mutation is ordinary source work and is
	// allowed, while targeting the stranded id for mutation stays denied with the
	// recover escape.
	idless := handoffEditRequest(record, repo, "codex", "any-session", repo+"/internal/x.go")
	if got := BuildLifecyclePreToolUseDecision(idless); got.Decision != "allow" {
		t.Fatalf("id-less ordinary source mutation must not be fenced: %#v", got)
	}
	strandedTarget := handoffEditRequest(record, repo, "codex", "any-session", "")
	strandedTarget.Tool = "Bash"
	strandedTarget.Command = "agent-harness issueops heartbeat --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --host codex --session-id any-session --agent-id worker-1"
	if got := BuildLifecyclePreToolUseDecision(strandedTarget); got.Decision != "block" || !strings.Contains(got.Reason, "handoff recover") {
		t.Fatalf("command targeting the stranded id must stay denied with the recover escape: %#v", got)
	}
}

// TestFenceScopeNarrowingSelectionDirect exercises selectSupervisedHandoffRecord
// directly: an explicit different-id command and an id-less source-only
// mutation both select no record.
func TestFenceScopeNarrowingSelectionDirect(t *testing.T) {
	repo, record, _ := strandedRecoveryRequiredRecord(t)
	_ = record

	// Explicit different id -> not fenced.
	diff := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "s", AgentID: "worker-1", Tool: "Bash",
		Command: "agent-harness issueops status --id io-different --json", EnforceWorktree: true, SourceCheckout: repo,
	}
	if _, ok, reason := selectSupervisedHandoffRecord(diff); ok || reason != "" {
		t.Fatalf("different-id source command must select no stranded record: ok=%v reason=%q", ok, reason)
	}

	// Id-less source mutation -> not fenced.
	idless := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "s", AgentID: "worker-1", Tool: "Edit",
		Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo,
	}
	got, ok, reason := selectSupervisedHandoffRecord(idless)
	if ok || reason != "" || got.ExecutionHandoff != nil {
		t.Fatalf("id-less source mutation must select no stranded record: got=%#v ok=%v reason=%q", got, ok, reason)
	}
}
