package issueopscompletion

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	completionapp "issueops/internal/application/issueopscompletion"
	completioncontract "issueops/internal/contract/issueopscompletion"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

func TestRepositoryAtomicallyPersistsCompletionAndDeletesHolderIndex(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	record.Execution.CompletionHistory = []leasecontract.CompletionHistoryEntry{{
		Generation: 4,
		Completion: leasecontract.Completion{FinalHead: "d6d8c6a5a98fcca6bca33edf9e7965636429ce28", VerificationReportPath: ".issueops/verified-execution/old.json", Verification: []string{"old verification"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198", CompletedAt: "2026-08-01T00:00:00Z"},
		Reason:     "functional HEAD moved",
		ReopenedAt: "2026-08-02T00:00:00Z",
	}}
	record.Execution.Lease.Generation = 5
	raw, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	indexRaw, err := json.Marshal(holderIndex{SchemaVersion: 1, LifecycleID: record.ID, Generation: 1, Host: actor.Host, SessionID: actor.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	store := newMemoryStore(map[string][]byte{
		recordKey(recordBucket, record.ID):             raw,
		recordKey(holderBucket, holderIndexKey(actor)): indexRaw,
	})

	result, err := NewRepository(store).Update(context.Background(), record.ID, func(before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		before.Phase = "done"
		before.Lease.Status = "released"
		before.Lease.Holder = nil
		before.Lease.ReleasedAt = "2026-08-02T01:02:03.000000004Z"
		before.Completion = &completioncontract.Completion{Generation: 5, FinalHead: strings.Repeat("a", 40), VerificationReportPath: "/worktree/report.json", Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198", CompletedAt: "2026-08-02T01:02:03.000000004Z"}
		before.Ledger["pr"] = completioncontract.LedgerEntry{Phase: "pr", EnteredAt: "2026-08-02T00:00:03Z", CompletedAt: "2026-08-02T01:02:03.000000004Z", Artifacts: []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}}
		before.Ledger["done"] = completioncontract.LedgerEntry{Phase: "done", EnteredAt: "2026-08-02T01:02:03.000000004Z"}
		return before, true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Record.Prepared || result.Execution.Completion == nil || result.Execution.Lease.Status != "released" {
		t.Fatalf("result = %+v", result)
	}
	if _, ok := store.records[recordKey(holderBucket, holderIndexKey(actor))]; ok {
		t.Fatal("holder index still exists")
	}
	persisted, err := leasecontract.Decode(record.ID, store.records[recordKey(recordBucket, record.ID)])
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != "done" || persisted.Execution.Completion == nil || persisted.Execution.Completion.Generation != 5 || persisted.Execution.Lease.Status != "released" {
		t.Fatalf("persisted = %+v", persisted)
	}
	if len(persisted.Execution.CompletionHistory) != 1 || persisted.Execution.CompletionHistory[0].Completion.FinalHead != "d6d8c6a5a98fcca6bca33edf9e7965636429ce28" || persisted.Execution.CompletionHistory[0].Completion.Verification[0] != "old verification" {
		t.Fatalf("completion history lost: %+v", persisted.Execution.CompletionHistory)
	}
	var ledger map[string]completioncontract.LedgerEntry
	if err := json.Unmarshal(persisted.PhaseLedger, &ledger); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ledger["pr"].Artifacts, []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}) {
		t.Fatalf("ledger = %+v", ledger)
	}
}

func TestRepositoryNoChangeDoesNotApplyOrDeleteIndex(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	raw, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	store := newMemoryStore(map[string][]byte{recordKey(recordBucket, record.ID): raw})

	_, err = NewRepository(store).Update(context.Background(), record.ID, func(before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		return before, false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.applyCalls != 0 {
		t.Fatalf("Apply calls = %d", store.applyCalls)
	}
	_ = actor
}

func completionRepositoryRecord(t *testing.T) (leasecontract.Record, leasecontract.Actor) {
	t.Helper()
	actor := leasecontract.Actor{Host: "codex", SessionID: "session-198", SessionProcess: &leasecontract.ProcessReceipt{PID: 198, StartedAt: "2026-08-02T00:00:00Z", Executable: "/bin/codex"}}
	branchPrepare, _ := json.Marshal(map[string]any{"provider": "github", "issue_url": "https://github.com/acme/repo/issues/198", "branch": "198", "base_branch": "main", "link_verified": true, "steps": []any{}, "created_at": "2026-08-02T00:00:00Z"})
	artifact, _ := json.Marshal(map[string]any{"provider": "github", "kind": "pr", "url": "https://github.com/acme/repo/pull/198", "labels": []string{"enhancement"}, "assignees": []string{"m16khb"}, "verified_at": "2026-08-02T00:00:02Z", "target_branch": "main"})
	ledger, _ := json.Marshal(map[string]any{"pr": map[string]any{"phase": "pr", "entered_at": "2026-08-02T00:00:03Z"}})
	return leasecontract.Record{OK: true, SchemaVersion: 1, ID: "io-198", Repo: "/source", Branch: "198", Phase: "pr", IssueURL: "https://github.com/acme/repo/issues/198", BranchPrepare: branchPrepare, RemoteArtifact: artifact, PhaseLedger: ledger, CreatedAt: "2026-08-02T00:00:00Z", UpdatedAt: "2026-08-02T00:00:00Z", Execution: &leasecontract.Execution{Mode: "direct", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "198", BaseHead: strings.Repeat("b", 40), Driver: "git", LinkedAt: "2026-08-02T00:00:01Z"}, Lease: leasecontract.Lease{Generation: 1, Status: "active", Holder: &actor, ClaimedAt: "2026-08-02T00:00:01Z"}}}, actor
}

type memoryStore struct {
	records    map[string][]byte
	applyCalls int
}

func newMemoryStore(records map[string][]byte) *memoryStore {
	result := &memoryStore{records: map[string][]byte{}}
	for key, value := range records {
		result.records[key] = append([]byte(nil), value...)
	}
	return result
}

func (s *memoryStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *memoryStore) Get(bucket, id string) ([]byte, bool, error) {
	data, ok := s.records[recordKey(bucket, id)]
	return append([]byte(nil), data...), ok, nil
}
func (s *memoryStore) Apply(_ context.Context, mutations []port.RecordMutation) error {
	s.applyCalls++
	for _, mutation := range mutations {
		key := recordKey(mutation.Bucket, mutation.ID)
		if mutation.Delete {
			delete(s.records, key)
		} else {
			s.records[key] = append([]byte(nil), mutation.Data...)
		}
	}
	return nil
}

func recordKey(bucket, id string) string { return bucket + "\x00" + id }

var _ port.TransactionalRecordStore = (*memoryStore)(nil)
var _ completionapp.Repository = (*Repository)(nil)
