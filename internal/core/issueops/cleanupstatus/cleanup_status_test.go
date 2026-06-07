package cleanupstatus

import (
	"fmt"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestByIDReadsRecordAndReportsMissingEvidence(t *testing.T) {
	store := newCleanupStatusTestStore(model.IssueOpsRecord{
		ID:     "io-123456789abc",
		Repo:   t.TempDir(),
		Branch: "1-cleanup",
		Phase:  model.IssueOpsPhasePR,
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
			Provider:  "github",
			Kind:      "pr",
			URL:       "https://github.com/example/repo/pull/1",
			Labels:    []string{"issueops"},
			Assignees: []string{"habin"},
		},
	})

	status, err := ByID(store.issueOpsStore(), t.TempDir(), "io-123456789abc", model.IssueOpsCleanupStatusRequest{Merged: false})
	if err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.ID != "io-123456789abc" || status.Ready {
		t.Fatalf("unexpected cleanup status: %+v", status)
	}
	for _, want := range []string{"remote_artifact_merged", "worktree_path"} {
		if !containsString(status.Missing, want) {
			t.Fatalf("cleanup status missing %q not found in %+v", want, status.Missing)
		}
	}
	if len(status.Choices) != 3 || !strings.Contains(status.Choices[0], "차단 해소") {
		t.Fatalf("blocked cleanup choices should guide remediation: %+v", status.Choices)
	}
}

func TestByIDReturnsReadError(t *testing.T) {
	store := newCleanupStatusTestStore(model.IssueOpsRecord{})
	status, err := ByID(store.issueOpsStore(), t.TempDir(), "io-123456789abc", model.IssueOpsCleanupStatusRequest{})
	if err == nil {
		t.Fatal("ByID(missing) error = nil, want error")
	}
	if status.OK || status.ID != "io-123456789abc" {
		t.Fatalf("missing status = %+v, want ok=false with id", status)
	}
}

type cleanupStatusTestStore struct {
	record model.IssueOpsRecord
}

func newCleanupStatusTestStore(record model.IssueOpsRecord) *cleanupStatusTestStore {
	return &cleanupStatusTestStore{record: record}
}

func (s *cleanupStatusTestStore) issueOpsStore() Store {
	return Store{
		Read: func(_ string, id string) (model.IssueOpsRecord, error) {
			if s.record.ID != id {
				return model.IssueOpsRecord{}, fmt.Errorf("missing record")
			}
			return s.record, nil
		},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
