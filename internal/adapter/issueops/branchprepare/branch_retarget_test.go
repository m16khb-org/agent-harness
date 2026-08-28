package branchprepare

import (
	"fmt"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func retargetTestRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID:       "io-1",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "2819-child",
		IssueURL: "https://gitlab.example/group/project/-/issues/2819",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "gitlab", Branch: "2819-child", BaseBranch: "release/stg", BaseSHA: "abc123",
		},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			Provider: "gitlab", Kind: "mr", URL: "https://gitlab.example/group/project/-/merge_requests/5606",
			TargetBranch: "release/stg",
		},
	}
}

func retargetTestStore(record model.IssueOpsRecord, observedTarget string, remotePresent bool) (*branchPrepareTestStore, Store) {
	s := newBranchPrepareTestStore(record)
	store := s.issueOpsStore()
	store.ObserveArtifactTargetBranch = func(artifact model.IssueOpsRemoteArtifactVerification) (string, error) {
		if artifact.URL != record.RemoteArtifact.URL {
			return "", fmt.Errorf("unexpected artifact %q", artifact.URL)
		}
		return observedTarget, nil
	}
	store.RemoteBranchPresent = func(repo, branch string) (bool, error) {
		return remotePresent, nil
	}
	return s, store
}

func TestRetargetRecordsProviderObservedBaseChange(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)

	record, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "child MR retargeted to the umbrella branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	prepare := record.BranchPrepare
	if prepare.BaseBranch != "2803-umbrella" {
		t.Fatalf("base_branch must follow the observed retarget: %+v", prepare)
	}
	if prepare.BaseSHA != "abc123" {
		t.Fatalf("sealed base sha is the fork point and must survive a retarget: %+v", prepare)
	}
	if record.RemoteArtifact.TargetBranch != "2803-umbrella" {
		t.Fatalf("remote artifact target must stay in step with the prepared base: %+v", record.RemoteArtifact)
	}
	if len(prepare.Retargets) != 1 {
		t.Fatalf("expected one retarget entry: %+v", prepare.Retargets)
	}
	entry := prepare.Retargets[0]
	if entry.FromBase != "release/stg" || entry.ToBase != "2803-umbrella" || entry.Reason == "" ||
		entry.ArtifactURL != record.RemoteArtifact.URL || entry.ObservedAt == "" {
		t.Fatalf("retarget entry must carry from/to/reason/artifact/observed_at: %+v", entry)
	}
}

func TestRetargetRejectsBaseTheProviderDoesNotShow(t *testing.T) {
	s, store := retargetTestStore(retargetTestRecord(), "release/stg", true)

	_, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "wishful",
	})
	if err == nil || !strings.Contains(err.Error(), "release/stg") {
		t.Fatalf("a base the provider does not show must be rejected with the observed target: %v", err)
	}
	if s.record.BranchPrepare.BaseBranch != "release/stg" || len(s.record.BranchPrepare.Retargets) != 0 {
		t.Fatalf("rejected retarget must not touch the record: %+v", s.record.BranchPrepare)
	}
}

func TestRetargetRejectsBaseAbsentFromRemote(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", false)

	_, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "gone",
	})
	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("a base missing from origin must be rejected: %v", err)
	}
}

func TestRetargetFailsClosedWhenObservationFails(t *testing.T) {
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	store.ObserveArtifactTargetBranch = func(model.IssueOpsRemoteArtifactVerification) (string, error) {
		return "", fmt.Errorf("network down")
	}

	_, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("observation failure must reject, not pass: %v", err)
	}
}

func TestRetargetRequiresRemoteArtifactAndReason(t *testing.T) {
	record := retargetTestRecord()
	record.RemoteArtifact = nil
	_, store := retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	store.Read = func(string, string) (model.IssueOpsRecord, error) { return record, nil }
	if _, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella", Reason: "x"}); err == nil || !strings.Contains(err.Error(), "remote artifact") {
		t.Fatalf("retarget without a verified remote artifact must be rejected: %v", err)
	}

	_, store = retargetTestStore(retargetTestRecord(), "2803-umbrella", true)
	if _, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "2803-umbrella"}); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("retarget without a reason must be rejected: %v", err)
	}
	if _, err := Retarget(store, t.TempDir(), "io-1", model.IssueOpsBranchRetargetRequest{BaseBranch: "release/stg", Reason: "same"}); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("retarget to the current base must be rejected: %v", err)
	}
}
