package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

// TestTerminalPhaseHandoffGuardRejectsNonTerminalHandoff pins the Task F3
// write-time guard for every handoff state.
func TestTerminalPhaseHandoffGuardRejectsNonTerminalHandoff(t *testing.T) {
	base := IssueOpsRecord{ID: "io-guard", Repo: "/tmp/repo"}
	for _, state := range []string{handoff.StateOwnershipDispatching, handoff.StateOwnershipDispatched, handoff.StateOwnerOrienting, handoff.StateOwnerActive, handoff.StateCleanupPendingHumanDecision, handoff.StateCleanupExecuting, handoff.StateRecoveryRequired} {
		r := base
		r.ExecutionHandoff = &model.IssueOpsExecutionHandoff{State: state}
		err := issueOpsTerminalPhaseHandoffGuard(r, IssueOpsPhaseDone)
		if err == nil {
			t.Fatalf("done transition must be rejected while handoff state=%s", state)
		}
		if !strings.Contains(err.Error(), "handoff recover") || !strings.Contains(err.Error(), r.ID) {
			t.Fatalf("guard error must name the recover escape and id for state=%s: %v", state, err)
		}
	}
	// closed handoff, nil handoff, and non-terminal target phase are all allowed.
	for _, tc := range []struct {
		name  string
		rec   IssueOpsRecord
		phase IssueOpsPhase
	}{
		{"closed handoff", IssueOpsRecord{ID: "io-x", ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: handoff.StateClosed}}, IssueOpsPhaseDone},
		{"no handoff", IssueOpsRecord{ID: "io-x"}, IssueOpsPhaseDone},
		{"non-terminal phase", IssueOpsRecord{ID: "io-x", ExecutionHandoff: &model.IssueOpsExecutionHandoff{State: handoff.StateRecoveryRequired}}, IssueOpsPhasePR},
	} {
		if err := issueOpsTerminalPhaseHandoffGuard(tc.rec, tc.phase); err != nil {
			t.Fatalf("%s must be allowed: %v", tc.name, err)
		}
	}
}
