package lifecycle

import "testing"

func TestStopNextActionRelayRecordsDuplicatesAndClears(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repoRoot := t.TempDir()
	trigger := NextActionJudgementTriggerResult{
		RecommendedIndex: 1,
		RecommendedText:  "continue safely",
		Candidates: []NextActionCandidate{{
			Index:       1,
			Text:        "continue safely",
			Recommended: true,
		}},
	}

	recorded := RecordStopNextActionRelay(repoRoot, trigger)
	if !recorded.OK || !recorded.ShouldRelay || recorded.Reason != "recorded_next_action_relay" {
		t.Fatalf("expected first relay record to be relayed: %+v", recorded)
	}

	duplicate := RecordStopNextActionRelay(repoRoot, trigger)
	if !duplicate.OK || duplicate.ShouldRelay || duplicate.Reason != "duplicate_next_action_relay" {
		t.Fatalf("expected duplicate relay suppression: %+v", duplicate)
	}

	changed := trigger
	changed.Candidates[0].Text = "continue with another slice"
	pending := RecordStopNextActionRelay(repoRoot, changed)
	if !pending.OK || pending.ShouldRelay || pending.Reason != "pending_next_action_relay" {
		t.Fatalf("expected pending relay suppression for changed fingerprint: %+v", pending)
	}

	cleared := ClearStopNextActionRelay(repoRoot)
	if !cleared.OK || cleared.Reason != "cleared_next_action_relay" {
		t.Fatalf("expected relay record to clear: %+v", cleared)
	}
	none := ClearStopNextActionRelay(repoRoot)
	if !none.OK || none.Reason != "no_next_action_relay" {
		t.Fatalf("expected second clear to be a no-op: %+v", none)
	}
	empty := RecordStopNextActionRelay(repoRoot, NextActionJudgementTriggerResult{})
	if !empty.OK || empty.ShouldRelay || empty.Reason != "no_next_action_fingerprint" {
		t.Fatalf("expected empty trigger to skip relay: %+v", empty)
	}
}
