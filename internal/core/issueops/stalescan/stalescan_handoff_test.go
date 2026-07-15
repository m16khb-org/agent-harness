package stalescan

import (
	"testing"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

// TestClassifyFlagsDoneWithNonTerminalHandoff proves the #2581 blind spot is
// closed: a done-phase cycle whose supervised handoff is still non-terminal is
// surfaced as the report-only handoff_nonterminal_on_terminal_phase signal, and
// is never Releasable.
func TestClassifyFlagsDoneWithNonTerminalHandoff(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:    "io-9bab890c4d4f",
		Phase: model.IssueOpsPhaseDone,
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{
			State: handoff.StateRecoveryRequired,
		},
	}
	finding, ok := Classify(record, Probe{}, time.Hour)
	if !ok {
		t.Fatal("done cycle with a non-terminal handoff must be surfaced, not silently ignored")
	}
	if finding.Category != CategoryHandoffInconsistent {
		t.Fatalf("expected handoff-inconsistent category, got %q", finding.Category)
	}
	if finding.Releasable {
		t.Fatal("the inconsistency signal must never be releasable (timeout != absence)")
	}
	foundReason := false
	for _, r := range finding.Reasons {
		if r == "handoff_nonterminal_on_terminal_phase" {
			foundReason = true
		}
	}
	if !foundReason {
		t.Fatalf("finding must carry the exact signal reason: %#v", finding.Reasons)
	}
}

// TestClassifyIgnoresDoneWithTerminalHandoff confirms a done cycle whose handoff
// is closed (or absent) is still never flagged.
func TestClassifyIgnoresDoneWithTerminalHandoff(t *testing.T) {
	for _, h := range []*model.IssueOpsExecutionHandoff{
		nil,
		{State: handoff.StateClosed},
	} {
		record := model.IssueOpsRecord{ID: "io-done", Phase: model.IssueOpsPhaseDone, ExecutionHandoff: h}
		if _, ok := Classify(record, Probe{}, time.Hour); ok {
			t.Fatalf("done cycle with terminal/absent handoff must not be flagged: handoff=%#v", h)
		}
	}
}
