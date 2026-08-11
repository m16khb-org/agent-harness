package issueopsartifact

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestRepositoryUpdatesAndDeletesStagedArtifactMap(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-artifact01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), id, data); err != nil {
		t.Fatal(err)
	}

	repository := Repository{}
	if _, err := repository.Update(
		context.Background(),
		stateRoot,
		id,
		func(_ issueopscontract.IssueOpsRecord, staged map[string]string) (map[string]string, error) {
			staged["plan"] = "# Plan\n"
			return staged, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	staged, err := repository.ReadStaged(context.Background(), stateRoot, id)
	if err != nil || staged["plan"] != "# Plan\n" {
		t.Fatalf("staged map = %v, %v", staged, err)
	}
	if _, err := repository.Update(
		context.Background(),
		stateRoot,
		id,
		func(_ issueopscontract.IssueOpsRecord, staged map[string]string) (map[string]string, error) {
			delete(staged, "plan")
			return staged, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	staged, err = repository.ReadStaged(context.Background(), stateRoot, id)
	if err != nil || len(staged) != 0 {
		t.Fatalf("staged map after delete = %v, %v", staged, err)
	}
}
