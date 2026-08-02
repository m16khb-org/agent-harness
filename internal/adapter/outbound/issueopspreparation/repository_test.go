package issueopspreparation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	preparationapp "agent-harness/internal/application/issueopspreparation"
	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

func TestDirectRepositoryCommitWritesRecordAndHolderAtomically(t *testing.T) {
	store := newPreparationStore()
	record := repositoryRecord("io-prepare", "/repo", "199-prepare")
	record.Decisions = json.RawMessage(`[{"kind":"preserved"}]`)
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store, func(context.Context) error { return nil })

	snapshot, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repository.CommitDirect(context.Background(), directRepositoryCommit(snapshot))
	if err != nil {
		t.Fatal(err)
	}
	if store.spans != 1 || len(store.applies) != 1 || len(store.applies[0]) != 2 {
		t.Fatalf("spans=%d applies=%+v", store.spans, store.applies)
	}
	mutations := store.applies[0]
	if mutations[0].Bucket != recordBucket || mutations[0].ID != record.ID || mutations[0].RequireAbsent {
		t.Fatalf("record mutation=%+v", mutations[0])
	}
	if mutations[1].Bucket != holderBucket || !mutations[1].RequireAbsent {
		t.Fatalf("holder mutation=%+v", mutations[1])
	}
	persisted, err := leasecontract.Decode(record.ID, store.mustGet(recordBucket, record.ID))
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution == nil || persisted.Execution.Mode != preparationcontract.ModeDirect || persisted.Execution.Lease.Generation != 1 || persisted.Execution.Lease.Status != "active" {
		t.Fatalf("execution=%+v", persisted.Execution)
	}
	if persisted.Execution.Lease.Holder == nil || persisted.Execution.Lease.Holder.SessionProcess == nil || persisted.Execution.Lease.Holder.SessionProcess.StartedAt != "boot:42" {
		t.Fatalf("holder=%+v", persisted.Execution.Lease.Holder)
	}
	if len(persisted.Decisions) == 0 || !result.OK || result.Execution == nil || result.Execution.Lease.Holder == nil {
		t.Fatalf("persisted=%+v result=%+v", persisted, result)
	}
}

func TestDirectRepositoryHolderConflictRollsBackRecord(t *testing.T) {
	store := newPreparationStore()
	record := repositoryRecord("io-prepare", "/repo", "199-prepare")
	store.seedRecord(t, record)
	commit := directRepositoryCommit(preparationcontract.Snapshot{Record: record, RecordRaw: store.mustGet(recordBucket, record.ID)})
	store.rows[holderBucket] = map[string][]byte{holderIndexKey(commit.Command.Actor): []byte(`{"occupied":true}`)}
	repository := NewSQLiteRepository(store, func(context.Context) error { return nil })
	before := append([]byte(nil), store.mustGet(recordBucket, record.ID)...)

	if _, err := repository.CommitDirect(context.Background(), commit); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err=%v", err)
	}
	if after := store.mustGet(recordBucket, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatalf("record changed on holder conflict\nbefore=%s\nafter=%s", before, after)
	}
}

func TestRepositoryRootScanRejectsClaimAndCorruption(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, *preparationStore)
		want string
	}{
		{name: "claimed worktree_path", seed: func(t *testing.T, store *preparationStore) {
			record := repositoryRecord("io-other", "/repo", "199/prepare")
			record.WorktreePath = "/repo.worktrees/199-prepare"
			store.seedRecord(t, record)
		}, want: "io-other"},
		{name: "corrupt inventory", seed: func(_ *testing.T, store *preparationStore) {
			store.rows[recordBucket] = map[string][]byte{"io-broken": []byte(`{"schema_version":999,"id":"io-broken"}`)}
		}, want: "손상 레코드"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newPreparationStore()
			test.seed(t, store)
			repository := NewSQLiteRepository(store, func(context.Context) error { return nil })
			err := repository.EnsureRootUnclaimed(context.Background(), "io-self", "/repo.worktrees/199-prepare")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRepositoryMutationGateFailsClosed(t *testing.T) {
	want := errors.New("hook mutation denied")
	repository := NewSQLiteRepository(newPreparationStore(), func(context.Context) error { return want })
	if err := repository.RequireMutationAllowed(context.Background()); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	repository = NewSQLiteRepository(newPreparationStore(), nil)
	if err := repository.RequireMutationAllowed(context.Background()); err == nil || !strings.Contains(err.Error(), "mutation gate") {
		t.Fatalf("err=%v", err)
	}
}

func directRepositoryCommit(snapshot preparationcontract.Snapshot) preparationapp.DirectCommit {
	actor := leasecontract.Actor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "boot:42", Executable: "/bin/codex"}}
	return preparationapp.DirectCommit{
		Snapshot:      snapshot,
		Command:       preparationcontract.Command{ID: snapshot.Record.ID, Mode: preparationcontract.ModeDirect, Actor: actor, Confirm: true},
		Workspace:     preparationcontract.WorkspaceReceipt{SourceRoot: snapshot.Record.Repo, Root: "/repo.worktrees/199-prepare", Branch: snapshot.Record.Branch, BaseHead: "base", Driver: "git", Exists: true},
		RequestedMode: preparationcontract.ModeDirect,
		LinkedAt:      "2026-08-02T00:00:00Z",
		ClaimedAt:     "2026-08-02T00:00:01Z",
	}
}

func repositoryRecord(id, repo, branch string) leasecontract.Record {
	return leasecontract.Record{OK: true, SchemaVersion: 1, ID: id, Repo: repo, Branch: branch, Phase: "implement", CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z"}
}

type preparationStore struct {
	rows    map[string]map[string][]byte
	spans   int
	cas     int
	applies [][]port.RecordMutation
}

func newPreparationStore() *preparationStore {
	return &preparationStore{rows: map[string]map[string][]byte{}}
}

func (store *preparationStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	store.spans++
	return fn(ctx)
}

func (store *preparationStore) Get(bucket, id string) ([]byte, bool, error) {
	data, ok := store.rows[bucket][id]
	return append([]byte(nil), data...), ok, nil
}

func (store *preparationStore) GetAll(bucket string) ([]port.RecordRow, error) {
	rows := make([]port.RecordRow, 0, len(store.rows[bucket]))
	for id, data := range store.rows[bucket] {
		rows = append(rows, port.RecordRow{ID: id, Data: append([]byte(nil), data...)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func (store *preparationStore) Apply(_ context.Context, mutations []port.RecordMutation) error {
	for _, mutation := range mutations {
		if mutation.RequireAbsent {
			if _, exists := store.rows[mutation.Bucket][mutation.ID]; exists {
				return errors.New("sqlstore precondition failed: row already exists")
			}
		}
	}
	cloned := make([]port.RecordMutation, len(mutations))
	copy(cloned, mutations)
	store.applies = append(store.applies, cloned)
	for _, mutation := range mutations {
		if store.rows[mutation.Bucket] == nil {
			store.rows[mutation.Bucket] = map[string][]byte{}
		}
		if mutation.Delete {
			delete(store.rows[mutation.Bucket], mutation.ID)
			continue
		}
		store.rows[mutation.Bucket][mutation.ID] = append([]byte(nil), mutation.Data...)
	}
	return nil
}

func (store *preparationStore) CompareAndApply(ctx context.Context, expected []port.ExpectedRecord, mutations []port.RecordMutation) error {
	store.cas++
	for _, item := range expected {
		data, ok, err := store.Get(item.Bucket, item.ID)
		if err != nil {
			return err
		}
		if !ok || !reflect.DeepEqual(data, item.Data) {
			return errors.New("stale raw record snapshot")
		}
	}
	return store.Apply(ctx, mutations)
}

func (store *preparationStore) seedRecord(t *testing.T, record leasecontract.Record) {
	t.Helper()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if store.rows[recordBucket] == nil {
		store.rows[recordBucket] = map[string][]byte{}
	}
	store.rows[recordBucket][record.ID] = data
}

func (store *preparationStore) mustGet(bucket, id string) []byte {
	return append([]byte(nil), store.rows[bucket][id]...)
}
