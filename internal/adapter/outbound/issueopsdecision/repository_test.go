package issueopsdecision

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"agent-harness/internal/adapter/outbound/issueopsrecord"
	"agent-harness/internal/adapter/outbound/sqlstore"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestRepositoryPersistsDecisionMutationWithinSpan(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-decision01"
	data, err := json.Marshal(issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
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

	record, err := (Repository{}).Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error) {
			record.Decisions = append(record.Decisions, issueopscontract.IssueOpsDecision{
				Title: "Boundary",
			})
			return record, nil
		},
	)
	if err != nil || len(record.Decisions) != 1 {
		t.Fatalf("update returned %+v, %v", record, err)
	}
}
