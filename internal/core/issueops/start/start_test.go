package start

import (
	"errors"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestStartCreatesSchemaOneRecord(t *testing.T) {
	writes := 0
	store := Store{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{}, errors.New("not found")
		},
		Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return record, nil
		},
		NewID:          func(string, string) string { return "io-v1" },
		ValidateBranch: func(string) error { return nil },
	}

	got, err := Start(store, t.TempDir(), model.IssueOpsStartRequest{Repo: ".", Branch: "69-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.ID != "io-v1" || got.Phase != model.IssueOpsPhaseProblem || writes != 1 {
		t.Fatalf("unexpected new v1 record: %+v writes=%d", got, writes)
	}
}

func TestStartReturnsExistingRecordWithoutRewrite(t *testing.T) {
	existing := model.IssueOpsRecord{OK: true, SchemaVersion: 1, ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: model.IssueOpsPhasePlan}
	store := Store{
		Read: func(string, string) (model.IssueOpsRecord, error) { return existing, nil },
		Write: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("existing record must not be rewritten")
			return model.IssueOpsRecord{}, nil
		},
		NewID:          func(string, string) string { return existing.ID },
		ValidateBranch: func(string) error { return nil },
	}

	got, err := Start(store, t.TempDir(), model.IssueOpsStartRequest{Repo: existing.Repo, Branch: existing.Branch})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != existing.ID || got.Phase != existing.Phase {
		t.Fatalf("existing record changed: %+v", got)
	}
}
