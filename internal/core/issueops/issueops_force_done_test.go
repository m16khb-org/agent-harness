package issueops

import (
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
