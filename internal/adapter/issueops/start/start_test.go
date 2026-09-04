package start

import (
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
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

func TestStartCanonicalizesLinkedWorktreeRepoBeforeIDAndWrite(t *testing.T) {
	const (
		worktree = "/repo.worktrees/69-v1"
		source   = "/repo"
	)
	var idRepo string
	var written model.IssueOpsRecord
	store := Store{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{}, errors.New("not found")
		},
		Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			written = record
			return record, nil
		},
		NewID: func(repo, _ string) string {
			idRepo = repo
			return "io-v1"
		},
		ValidateBranch: func(string) error { return nil },
		NormalizeRepo: func(repo string) string {
			if repo != worktree {
				t.Fatalf("NormalizeRepo input=%q, want %q", repo, worktree)
			}
			return source
		},
	}

	if _, err := Start(store, t.TempDir(), model.IssueOpsStartRequest{Repo: worktree, Branch: "69-v1"}); err != nil {
		t.Fatal(err)
	}
	if idRepo != source || written.Repo != source {
		t.Fatalf("canonical repo mismatch: id repo=%q record repo=%q want=%q", idRepo, written.Repo, source)
	}
}
