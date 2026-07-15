package handoff

import "testing"

// legacyCoordinatorLifecycleStateAllows is the exact pre-consolidation switch
// (Task A). The characterization test asserts the declarative table reproduces
// it for every (command × state × disposition) cell, so the move is provably
// behavior-preserving.
func legacyCoordinatorLifecycleStateAllows(path, state, closedDisposition string) bool {
	switch path {
	case "phase":
		return state == StateCoordinatorPreparing
	case "handoff accept":
		return state == StateSubmitted || state == StateClosed
	case "handoff recover":
		return state == StateRecoveryRequired || state == StateSubmitted || state == StateClosed || state == StateClaimed
	case "handoff publish":
		return state == StateClosed && closedDisposition == DispositionAccepted
	case "handoff start":
		return true
	default:
		return state == StateCoordinatorPreparing
	}
}

func TestCoordinatorCommandStateAllowsMatchesLegacy(t *testing.T) {
	paths := []string{
		"phase", "handoff accept", "handoff recover", "handoff publish", "handoff start",
		"link-plan", "compatibility review", "execution decide", "devils-advocate review",
		"worktree prepare", "worktree prepare-tools",
		"unknown command path",
	}
	states := []string{
		StateCoordinatorPreparing, StateDispatched, StateClaimed, StateSubmitted, StateClosed, StateRecoveryRequired,
		"", "bogus-state",
	}
	dispositions := []string{"", DispositionAccepted, DispositionWorkerFailed, DispositionCancelled}
	for _, path := range paths {
		for _, state := range states {
			for _, disp := range dispositions {
				got := CoordinatorCommandStateAllows(path, state, disp)
				want := legacyCoordinatorLifecycleStateAllows(path, state, disp)
				if got != want {
					t.Fatalf("CoordinatorCommandStateAllows(%q,%q,%q)=%v want %v", path, state, disp, got, want)
				}
			}
		}
	}
}
