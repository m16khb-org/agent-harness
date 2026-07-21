package handoff

import "testing"

func TestStateRoleAllowsCurrentContract(t *testing.T) {
	if !OwnerStateAllows("claim", StateOwnershipDispatched) || OwnerStateAllows("claim", StateOwnerActive) {
		t.Fatal("owner claim authority drifted")
	}
	if !OwnerStateAllows("complete", StateOwnerActive) || OwnerStateAllows("complete", StateCleanupPendingHumanDecision) {
		t.Fatal("owner completion authority drifted")
	}
	if !StateRoleAllows(RoleSource, "cleanup-approve", StateCleanupPendingHumanDecision) || StateRoleAllows(RoleOwner, "cleanup-approve", StateCleanupPendingHumanDecision) {
		t.Fatal("source cleanup authority drifted")
	}
	if StateRoleAllows("removed_worker", "finish", "claimed") {
		t.Fatal("removed authority was accepted")
	}
}
