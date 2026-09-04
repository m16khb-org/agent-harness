package issueops

import (
	"context"
	"encoding/json"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestListLeaseHolderIndexesReadsAndValidatesExistingRows(t *testing.T) {
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	actor := issueops.NativeActor{Host: "codex", SessionID: "session-v1", AgentID: "agent-v1"}
	index := leaseHolderIndex{SchemaVersion: 1, LifecycleID: "io-0123456789ab", Generation: 3, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	key := leaseHolderIndexKey(actor)
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: leaseHolderBucket, ID: key, Data: data}}); err != nil {
		t.Fatal(err)
	}
	rows, err := ListLeaseHolderIndexes(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != key || rows[0].LifecycleID != index.LifecycleID || rows[0].Generation != 3 || rows[0].Host != actor.Host || rows[0].SessionID != actor.SessionID || rows[0].AgentID != actor.AgentID {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestListLeaseHolderIndexesAcceptsOmoHolder(t *testing.T) {
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	actor := issueops.NativeActor{Host: "omo", SessionID: "omo-session"}
	index := leaseHolderIndex{
		SchemaVersion: 1, LifecycleID: "io-0123456789ab", Generation: 1,
		Host: actor.Host, SessionID: actor.SessionID,
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	key := leaseHolderIndexKey(actor)
	if err := db.Apply(context.Background(), []port.RecordMutation{{
		Bucket: leaseHolderBucket, ID: key, Data: data,
	}}); err != nil {
		t.Fatal(err)
	}
	rows, err := ListLeaseHolderIndexes(stateRoot)
	if err != nil {
		t.Fatalf("Omo holder index must be readable: %v", err)
	}
	if len(rows) != 1 || rows[0].Host != "omo" || rows[0].SessionID != "omo-session" {
		t.Fatalf("Omo holder rows=%#v", rows)
	}
}

func TestListLeaseHolderIndexesMissingStoreIsEmpty(t *testing.T) {
	rows, err := ListLeaseHolderIndexes(t.TempDir() + "/absent")
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows, err = %#v, %v", rows, err)
	}
}

func TestListLeaseHolderIndexesRejectsMalformedOrMismatchedRows(t *testing.T) {
	tests := []struct {
		name string
		key  string
		data []byte
	}{
		{name: "malformed", key: "bad", data: []byte("{")},
		{name: "wrong key", key: "wrong", data: mustMarshalLeaseHolderIndex(t, leaseHolderIndex{SchemaVersion: 1, LifecycleID: "io-0123456789ab", Generation: 1, Host: "codex", SessionID: "session"})},
		{name: "wrong schema", key: leaseHolderIndexKey(issueops.NativeActor{Host: "codex", SessionID: "session"}), data: mustMarshalLeaseHolderIndex(t, leaseHolderIndex{SchemaVersion: 2, LifecycleID: "io-0123456789ab", Generation: 1, Host: "codex", SessionID: "session"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			db, err := sqlstore.Open(stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: leaseHolderBucket, ID: test.key, Data: test.data}}); err != nil {
				t.Fatal(err)
			}
			if _, err := ListLeaseHolderIndexes(stateRoot); err == nil {
				t.Fatal("expected corrupt reverse index to fail closed")
			}
		})
	}
}

func mustMarshalLeaseHolderIndex(t *testing.T, value leaseHolderIndex) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
