package issueops

import (
	"strings"
	"testing"
)

func TestExecutionReconcileIntentStateRejectsTypedAndRawDrift(t *testing.T) {
	stateRoot, record, payload := legacyPrepareIntentFixture(t)
	if _, err := markOrcaIntentInvoking(stateRoot, record.ID, payload); err != nil {
		t.Fatal(err)
	}

	state, err := executionReconcileIntentStateFromPayload(stateRoot, record, payload)
	if err == nil || !strings.Contains(err.Error(), "snapshot changed") || state.Record.ID != record.ID {
		t.Fatalf("state=%#v err=%v", state, err)
	}
}

func TestCanonicalizeExecutionReconcileIntentRejectsRouterSnapshotDrift(t *testing.T) {
	stateRoot, record, _ := pendingOrcaIntentFixture(t)
	snapshot, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	drifted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	drifted.Execution.Workspace.Root += "-drift"
	if _, err := WriteIssueOps(stateRoot, drifted); err != nil {
		t.Fatal(err)
	}
	_, err = CanonicalizeExecutionReconcileIntent(stateRoot, record.ID, &snapshot)
	if err == nil || !strings.Contains(err.Error(), "legacy_intent_upgrade_unsafe") {
		t.Fatalf("snapshot drift error=%v", err)
	}
}
