package handoff

import "slices"

const (
	RoleSource = "source"
	RoleOwner  = "owner"
)

// StateRoleAllows is the single state/role authority table for handoff work.
// Identity, fence, canonical-root, and target-scope checks remain at callers.
func StateRoleAllows(role, action, state string) bool {
	switch role {
	case RoleSource:
		switch action {
		case "status", "resume":
			return slices.Contains([]string{StateOwnershipDispatching, StateOwnershipDispatched, StateOwnerOrienting, StateOwnerActive, StateCleanupPendingHumanDecision, StateCleanupExecuting, StateClosed, StateRecoveryRequired}, state)
		case "cleanup-preview", "cleanup-approve", "cleanup-record":
			return slices.Contains([]string{StateCleanupPendingHumanDecision, StateCleanupExecuting}, state)
		}
	case RoleOwner:
		switch action {
		case "claim":
			return state == StateOwnershipDispatched
		case "acknowledge-context":
			return state == StateOwnerOrienting
		case "heartbeat":
			return state == StateOwnerOrienting || state == StateOwnerActive
		case "mutate", "publish", "remote-create", "complete":
			return state == StateOwnerActive
		}
	}
	return false
}

func OwnerStateAllows(action, state string) bool {
	return StateRoleAllows(RoleOwner, action, state)
}
