package artifactverify

import (
	"os"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

type artifactStoreForTest struct {
	records map[string]model.IssueOpsRecord
}

func newArtifactStoreForTest(record model.IssueOpsRecord) (*artifactStoreForTest, Store) {
	store := &artifactStoreForTest{records: map[string]model.IssueOpsRecord{record.ID: record}}
	return store, Store{Read: store.read, TouchWrite: store.touchWrite}
}

func (s *artifactStoreForTest) read(_ string, id string) (model.IssueOpsRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return model.IssueOpsRecord{OK: false, ID: id}, os.ErrNotExist
	}
	record.OK = true
	return record, nil
}

func (s *artifactStoreForTest) touchWrite(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	record.OK = true
	s.records[record.ID] = record
	return record, nil
}

func TestValidateChecksWithoutPersisting(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	fake, store := newArtifactStoreForTest(record)

	got, err := Validate(store, t.TempDir(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pull_request",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{" enhancement ", "issueops"},
		Assignees: []string{" habin "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != record.ID || got.RemoteArtifact != nil {
		t.Fatalf("validate should return original record without persisting artifact: %+v", got)
	}
	if persisted := fake.records[record.ID]; persisted.RemoteArtifact != nil {
		t.Fatalf("validate should not persist artifact, got %+v", persisted.RemoteArtifact)
	}
}

func TestValidateRejectsInvalidRecord(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    model.IssueOpsPhaseFeedback,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	_, store := newArtifactStoreForTest(record)

	got, err := Validate(store, t.TempDir(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{"issueops"},
		Assignees: []string{"habin"},
	})
	if err == nil || !strings.Contains(err.Error(), "before pr phase") {
		t.Fatalf("expected phase validation error, got record %+v err %v", got, err)
	}
	if got.OK {
		t.Fatalf("invalid validation should return ok=false record: %+v", got)
	}
}

func TestVerifyRemoteArtifactURLMatchesProvider(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "2-gitlab-mr",
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://gitlab.example/group/project/-/issues/2",
	}
	fake, store := newArtifactStoreForTest(record)

	if _, err := Verify(store, t.TempDir(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/9",
		Labels:    []string{"issueops"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "match linked issue provider") {
		t.Fatalf("expected provider mismatch error, got %v", err)
	}
	if _, err := Verify(store, t.TempDir(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "pr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/4",
		Labels:    []string{"issueops"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "gitlab remote artifact kind must be mr") {
		t.Fatalf("expected gitlab kind error, got %v", err)
	}
	got, err := Verify(store, t.TempDir(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "merge_request",
		URL:       "https://gitlab.example/group/project/-/merge_requests/4",
		Labels:    []string{"issueops"},
		Assignees: []string{"habin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteArtifact == nil || got.RemoteArtifact.Provider != "gitlab" || got.RemoteArtifact.Kind != "mr" {
		t.Fatalf("expected gitlab mr artifact, got %+v", got.RemoteArtifact)
	}
	if persisted := fake.records[record.ID]; persisted.RemoteArtifact == nil || persisted.RemoteArtifact.Provider != "gitlab" {
		t.Fatalf("expected persisted gitlab artifact, got %+v", persisted.RemoteArtifact)
	}
}
