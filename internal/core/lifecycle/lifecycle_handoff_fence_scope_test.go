package lifecycle

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

// TestFenceScopeNarrowingUnblocksDifferentCycleFromSource proves the Task F2
// allow-delta: a stranded recovery_required record that fences the source
// checkout must not capture a command that explicitly names a *different* cycle
// id. Provably-unrelated work (resume/status/start for another id) proceeds,
// while id-less mutation and stranded-id targeting stay denied.
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

	// Companion: id-less mutation from the source checkout stays denied
	// (default-deny preserved), and targeting the stranded id for mutation stays
	// denied with the recover escape.
	idless := handoffEditRequest(record, repo, "codex", "any-session", repo+"/internal/x.go")
	if got := BuildLifecyclePreToolUseDecision(idless); got.Decision != "block" {
		t.Fatalf("id-less mutation from source must stay denied: %#v", got)
	}
	strandedTarget := handoffEditRequest(record, repo, "codex", "any-session", "")
	strandedTarget.Tool = "Bash"
	strandedTarget.Command = "agent-harness issueops heartbeat --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --host codex --session-id any-session --agent-id worker-1"
	if got := BuildLifecyclePreToolUseDecision(strandedTarget); got.Decision != "block" || !strings.Contains(got.Reason, "handoff recover") {
		t.Fatalf("command targeting the stranded id must stay denied with the recover escape: %#v", got)
	}
}

// TestFenceScopeNarrowingSelectionDirect exercises selectSupervisedHandoffRecord
// directly: an explicit different-id command from source selects no record; an
// id-less request still selects the stranded record.
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

	// Id-less mutation -> still selects the stranded record (fenced).
	idless := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "s", AgentID: "worker-1", Tool: "Edit",
		Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true, SourceCheckout: repo,
	}
	got, ok, _ := selectSupervisedHandoffRecord(idless)
	if !ok || got.ExecutionHandoff == nil || got.ExecutionHandoff.State != handoff.StateRecoveryRequired {
		t.Fatalf("id-less mutation from source must still select the stranded record")
	}
	_ = issueopsmodel.IssueOpsPhaseDone
}
