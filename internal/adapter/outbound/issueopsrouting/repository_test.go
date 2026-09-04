package issueopsrouting

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestRepositoryUpdatesAndReadsRecordWithinSpan(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops_v1")
	id := "io-routing01"
	record := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            id,
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
	data, err := json.Marshal(record)
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
	updated, err := repository.Update(
		context.Background(),
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
			record.RoutingTrace = append(record.RoutingTrace, issueopscontract.SkillRoutingEntry{
				Phase: "plan",
				Skill: "database-design",
			})
			return record, true, nil
		},
	)
	if err != nil || len(updated.RoutingTrace) != 1 {
		t.Fatalf("update returned %+v, %v", updated, err)
	}
	read, err := repository.Read(context.Background(), stateRoot, id)
	if err != nil || len(read.RoutingTrace) != 1 {
		t.Fatalf("read returned %+v, %v", read, err)
	}
}
