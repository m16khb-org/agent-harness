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

func TestProtocolStateRoleAuthorityMatrix(t *testing.T) {
	cases := []struct {
		protocol int
		role     string
		action   string
		state    string
		want     bool
	}{
		{ProtocolVersion, RoleLegacyCoordinator, "phase", StateCoordinatorPreparing, true},
		{ProtocolVersion, RoleLegacyWorker, "claim", StateDispatched, true},
		{ProtocolVersion, RoleLegacyWorker, "mutate", StateClaimed, true},
		{ProtocolVersion, RoleLegacyWorker, "mutate", StateDispatched, false},
		{OwnershipTransferProtocolVersion, RoleSourceOwnerTransfer, "status", StateOwnerActive, true},
		{OwnershipTransferProtocolVersion, RoleSourceOwnerTransfer, "mutate", StateOwnerActive, false},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "claim", StateOwnershipDispatched, true},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "acknowledge-context", StateOwnerOrienting, true},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "heartbeat", StateOwnerOrienting, true},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "mutate", StateOwnerActive, true},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "mutate", StateOwnerOrienting, false},
		{OwnershipTransferProtocolVersion, RoleTransferredOwner, "publish", StateOwnerActive, false},
		{99, RoleTransferredOwner, "mutate", StateOwnerActive, false},
		{OwnershipTransferProtocolVersion, "unknown", "mutate", StateOwnerActive, false},
	}
	for _, tc := range cases {
		if got := ProtocolStateRoleAllows(tc.protocol, tc.role, tc.action, tc.state, ""); got != tc.want {
			t.Fatalf("protocol=%d role=%s action=%s state=%s: got %t want %t", tc.protocol, tc.role, tc.action, tc.state, got, tc.want)
		}
	}
}
