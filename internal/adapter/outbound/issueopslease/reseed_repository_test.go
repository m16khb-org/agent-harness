package issueopslease

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/outbound/sqlstore"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/port"
)

func TestReseedRepositoryCommitsOnlyMatchingSnapshot(t *testing.T) {
	db, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := reseedRepositoryRecord()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: data}}); err != nil {
		t.Fatal(err)
	}
	repository := NewReseedRepository(db)
	snapshot, err := repository.LoadSnapshot(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := snapshot.Record
	next.Stable.Execution.Lease.Generation = 2
	next.Stable.Execution.Lease.Status = "claimable"
	next.Stable.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	next.Lease = next.Stable.Execution.Lease
	result, err := repository.CommitReseed(context.Background(), snapshot, next)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if result.Execution.Lease.Generation != 2 || result.Execution.Lease.Status != "claimable" {
		t.Fatalf("execution=%+v", result.Execution.Lease)
	}
}

func TestReseedRepositoryRejectsStaleRawSnapshot(t *testing.T) {
	db, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := reseedRepositoryRecord()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: data}}); err != nil {
		t.Fatal(err)
	}
	repository := NewReseedRepository(db)
	snapshot, err := repository.LoadSnapshot(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	revised := snapshot.Record.Stable
	revised.Phase = "revised"
	revisedData, err := leasecontract.Encode(revised)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: revisedData}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CommitReseed(context.Background(), snapshot, snapshot.Record); err == nil || !strings.Contains(err.Error(), "stale raw record") {
		t.Fatalf("commit error=%v", err)
	}
}

func reseedRepositoryRecord() leasecontract.Record {
	return leasecontract.Record{SchemaVersion: leasecontract.SchemaVersion, ID: "io-reseed-repository", Execution: &leasecontract.Execution{Mode: "direct", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base", Driver: "git", LinkedAt: "2026-07-30T09:00:00Z"}, Lease: leasecontract.Lease{Generation: 1, Status: "released"}}}
}

var _ leaseapp.ReseedRepository = (*ReseedRepository)(nil)
