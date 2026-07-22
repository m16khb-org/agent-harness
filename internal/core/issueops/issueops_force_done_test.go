package issueops

import (
	"strings"
	"testing"
)

func TestForceDoneStampsForceReleasedAtWhenSkippingVerification(t *testing.T) {
	stateRoot := t.TempDir()
	seed := IssueOpsRecord{
		ID:    "io-force-done",
		Phase: IssueOpsPhasePR,
		// No RemoteArtifact: remote artifact verification is missing, so
		// force-done must bypass it and record the bypass.
	}
	if _, err := WriteIssueOps(stateRoot, seed); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	got, err := ForceDoneIssueOps(stateRoot, seed.ID)
	if err != nil {
		t.Fatalf("force-done failed: %v", err)
	}
	if got.Phase != IssueOpsPhaseDone {
		t.Fatalf("expected done phase, got %s", got.Phase)
	}
	if got.ForceReleaseReason == "" {
		t.Error("expected force-release reason to be recorded")
	}
	if got.ForceReleasedAt == "" {
		t.Error("expected ForceReleasedAt to be stamped when verification is skipped")
	}
}

func TestForceDoneRejectsNonPRPhase(t *testing.T) {
	stateRoot := t.TempDir()
	seed := IssueOpsRecord{ID: "io-force-done-bad", Phase: IssueOpsPhaseImplement}
	if _, err := WriteIssueOps(stateRoot, seed); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}
	if _, err := ForceDoneIssueOps(stateRoot, seed.ID); err == nil {
		t.Fatal("expected error when force-done is called outside PR phase")
	}
}

func TestForceDoneParentRecordsActiveChildrenAudit(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := StartIssueOpsChild(stateRoot, IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-force-done",
		Title:              "force done child",
		TaskScope:          "prove force-done audits active children",
		AcceptanceCriteria: []string{"active child id is visible in force audit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent.Phase = IssueOpsPhasePR
	writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)

	done, err := ForceDoneIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Phase != IssueOpsPhaseDone || !strings.Contains(done.ForceReleaseReason, started.Child.ID) {
		t.Fatalf("force-done should audit active child ids, got phase=%s reason=%q", done.Phase, done.ForceReleaseReason)
	}
	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := childStatusEntryByID(status.Children, started.Child.ID)
	if !ok || entry.ParentClosedState != "parent_closed" {
		t.Fatalf("child status should mark active child under done parent as parent_closed, got %#v", entry)
	}
}

func TestForceReleaseParentRecordsActiveChildrenAudit(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := StartIssueOpsChild(stateRoot, IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-force-release",
		Title:              "force release child",
		TaskScope:          "prove force-release audits active children",
		AcceptanceCriteria: []string{"active child id is visible in force audit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	released, err := ForceReleaseIssueOps(stateRoot, parent.ID, "manual recovery after external merge")
	if err != nil {
		t.Fatal(err)
	}
	if released.Phase != parent.Phase || released.CycleState != IssueOpsCyclePaused || !strings.Contains(released.ForceReleaseReason, started.Child.ID) {
		t.Fatalf("force-release should audit active child ids, got phase=%s reason=%q", released.Phase, released.ForceReleaseReason)
	}
}
