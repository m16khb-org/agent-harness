package issueopslease

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
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

func TestReseedRepositoryPersistsCompletedReseedHistoryFixtures(t *testing.T) {
	for _, test := range []struct {
		name                 string
		leaseGeneration      uint64
		completionGeneration uint64
		oldHead              string
		newHead              string
		completedAt          string
		replacedAt           string
	}{
		{name: "issue 261", leaseGeneration: 5, completionGeneration: 4, oldHead: "d6d8c6a5a98fcca6bca33edf9e7965636429ce28", newHead: "ff27b34520e4e253d8ebfd523e4e4352bf93e8d8", completedAt: "2026-08-03T17:41:13.488177Z", replacedAt: "2026-08-03T17:58:40.077656Z"},
		{name: "issue 237", leaseGeneration: 2, completionGeneration: 1, oldHead: "fcd84227e5ed67d951d02f866bb6c23f1ecb0b27", newHead: "9c8db06313cfce39d17a53123f84da1fc4bc7b34", completedAt: "2026-08-03T17:25:16.676991Z", replacedAt: "2026-08-03T18:02:00.939902Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := sqlstore.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			record := reseedRepositoryRecord()
			record.Phase = "done"
			record.Execution.Lease.Generation = test.leaseGeneration
			record.Execution.Lease.ReplacedAt = test.replacedAt
			record.Execution.Completion = &leasecontract.Completion{FinalHead: test.oldHead, VerificationReportPath: ".issueops/verified-execution/old.json", Verification: []string{"old verification"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: test.completedAt}
			record.Execution.SyncBaseEvents = []leasecontract.SyncBaseEvent{{Mode: "apply", BaseBranch: "main", BaseOID: strings.Repeat("a", 40), MergeCommit: strings.Repeat("b", 40), Actor: "codex", At: "2026-08-03T01:00:00Z"}}
			record.PhaseLedger = json.RawMessage(`{"implement":{"phase":"implement","completed_at":"old"}}`)
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
			oldCompletion := *next.Stable.Execution.Completion
			oldCompletion.Verification = append([]string(nil), oldCompletion.Verification...)
			next.Stable.Execution.CompletionHistory = append(next.Stable.Execution.CompletionHistory, leasecontract.CompletionHistoryEntry{Generation: test.completionGeneration, Completion: oldCompletion, Reason: "functional HEAD moved to " + test.newHead, ReopenedAt: "2026-08-04T00:00:00Z"})
			next.Stable.Execution.Completion = nil
			next.Stable.Execution.Lease.Generation = test.leaseGeneration + 1
			next.Stable.Execution.Lease.Status = "claimable"
			next.Stable.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("d", 64)
			next.Stable.Phase = "implement"
			next.Lease = next.Stable.Execution.Lease
			if _, err := repository.CommitReseed(context.Background(), snapshot, next); err != nil {
				t.Fatal(err)
			}
			persisted, ok, err := db.Get(recordBucket, record.ID)
			if err != nil || !ok {
				t.Fatalf("get persisted record ok=%v err=%v", ok, err)
			}
			decoded, err := leasecontract.Decode(record.ID, persisted)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Phase != "implement" || decoded.Execution.Completion != nil || len(decoded.Execution.CompletionHistory) != 1 || decoded.Execution.CompletionHistory[0].Generation != test.completionGeneration || decoded.Execution.CompletionHistory[0].Completion.FinalHead != test.oldHead || decoded.Execution.CompletionHistory[0].Reason != "functional HEAD moved to "+test.newHead {
				t.Fatalf("persisted reopen=%+v", decoded.Execution)
			}
			if len(decoded.Execution.SyncBaseEvents) != 1 {
				t.Fatalf("sync-base history lost: %+v", decoded.Execution.SyncBaseEvents)
			}
		})
	}
}

func TestReseedRepositoryRejectedCASLeavesRecordAndHolderIndexUntouched(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*leaseapp.ReseedSnapshot, *leasecontract.Record)
	}{
		{name: "raw snapshot", mutate: func(_ *leaseapp.ReseedSnapshot, current *leasecontract.Record) { current.Phase = "revised" }},
		{name: "generation", mutate: func(snapshot *leaseapp.ReseedSnapshot, _ *leasecontract.Record) {
			snapshot.Record.Lease.Generation = 99
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := sqlstore.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			record := reseedRepositoryRecord()
			data, err := leasecontract.Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			holderData := []byte(`{"sentinel":"keep"}`)
			if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: data}, {Bucket: holderBucket, ID: "sentinel", Data: holderData}}); err != nil {
				t.Fatal(err)
			}
			repository := NewReseedRepository(db)
			snapshot, err := repository.LoadSnapshot(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			current := record
			test.mutate(&snapshot, &current)
			if current.Phase != record.Phase {
				currentData, err := leasecontract.Encode(current)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: currentData}}); err != nil {
					t.Fatal(err)
				}
				data = currentData
			}
			if _, err := repository.CommitReseed(context.Background(), snapshot, snapshot.Record); err == nil {
				t.Fatal("rejected CAS unexpectedly succeeded")
			}
			persisted, _, err := db.Get(recordBucket, record.ID)
			if err != nil || !bytes.Equal(persisted, data) {
				t.Fatalf("record changed after rejected CAS err=%v", err)
			}
			persistedHolder, _, err := db.Get(holderBucket, "sentinel")
			if err != nil || !bytes.Equal(persistedHolder, holderData) {
				t.Fatalf("holder index changed after rejected CAS err=%v", err)
			}
		})
	}
}

func reseedRepositoryRecord() leasecontract.Record {
	return leasecontract.Record{SchemaVersion: leasecontract.SchemaVersion, ID: "io-reseed-repository", Execution: &leasecontract.Execution{Mode: "direct", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base", Driver: "git", LinkedAt: "2026-07-30T09:00:00Z"}, Lease: leasecontract.Lease{Generation: 1, Status: "released"}}}
}

var _ leaseapp.ReseedRepository = (*ReseedRepository)(nil)
