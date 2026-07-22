package lifecycle

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
)

func TestPausedOwnershipLedgerReleasesSourceButQuarantinesHistoricalOwner(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	paused, err := issueops.ForceReleaseIssueOps(IssueOpsStateRoot(), record.ID, "operator paused stranded owner")
	if err != nil {
		t.Fatal(err)
	}
	if paused.CycleState != issueops.IssueOpsCyclePaused || paused.Ownership.ActiveAttempt != 0 || paused.Phase != record.Phase {
		t.Fatalf("unexpected paused ledger: %+v", paused)
	}

	source := handoffEditRequest(paused, repo, "codex", "fresh-source", filepath.Join(repo, "internal", "ordinary.go"))
	source.AgentID = "source-agent"
	if got := BuildLifecyclePreToolUseDecision(source); got.Decision != "allow" {
		t.Fatalf("paused cycle retained ordinary source fence: %#v", got)
	}

	historicalOwner := handoffEditRequest(paused, worker, "claude", "owner-session", filepath.Join(worker, "internal", "owner.go"))
	historicalOwner.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(historicalOwner); got.Decision != "block" {
		t.Fatalf("historical owner regained worker mutation authority: %#v", got)
	}

	historicalOrca := handoffEditRequest(paused, repo, "claude", "coordinator-session", "")
	historicalOrca.AgentID = "coordinator-agent"
	historicalOrca.Tool = "Bash"
	historicalOrca.Command = "orca orchestration task-update --id task-1 --status completed --json"
	if got := BuildLifecyclePreToolUseDecision(historicalOrca); got.Decision != "block" {
		t.Fatalf("paused historical Orca identity was not quarantined: %#v", got)
	}
}
