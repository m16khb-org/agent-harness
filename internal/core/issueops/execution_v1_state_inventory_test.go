package issueops

import (
	"context"
	"encoding/json"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func TestListLeaseHolderIndexesV1ReadsAndValidatesExistingRows(t *testing.T) {
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	actor := model.NativeActorV1{Host: "codex", SessionID: "session-v1", AgentID: "agent-v1"}
	index := leaseHolderIndexV1{SchemaVersion: 1, LifecycleID: "io-0123456789ab", Generation: 3, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	key := leaseHolderIndexKey(actor)
	if err := db.Apply(context.Background(), []sqlstore.Mutation{{Bucket: leaseHolderV1Bucket, ID: key, Data: data}}); err != nil {
		t.Fatal(err)
	}
	rows, err := ListLeaseHolderIndexesV1(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != key || rows[0].LifecycleID != index.LifecycleID || rows[0].Generation != 3 || rows[0].Host != actor.Host || rows[0].SessionID != actor.SessionID || rows[0].AgentID != actor.AgentID {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestListLeaseHolderIndexesV1MissingStoreIsEmpty(t *testing.T) {
	rows, err := ListLeaseHolderIndexesV1(t.TempDir() + "/absent")
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows, err = %#v, %v", rows, err)
	}
}

func TestListLeaseHolderIndexesV1RejectsMalformedOrMismatchedRows(t *testing.T) {
	tests := []struct {
		name string
		key  string
		data []byte
	}{
		{name: "malformed", key: "bad", data: []byte("{")},
		{name: "wrong key", key: "wrong", data: mustMarshalLeaseHolderIndexV1(t, leaseHolderIndexV1{SchemaVersion: 1, LifecycleID: "io-0123456789ab", Generation: 1, Host: "codex", SessionID: "session"})},
		{name: "wrong schema", key: leaseHolderIndexKey(model.NativeActorV1{Host: "codex", SessionID: "session"}), data: mustMarshalLeaseHolderIndexV1(t, leaseHolderIndexV1{SchemaVersion: 2, LifecycleID: "io-0123456789ab", Generation: 1, Host: "codex", SessionID: "session"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			db, err := sqlstore.Open(stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Apply(context.Background(), []sqlstore.Mutation{{Bucket: leaseHolderV1Bucket, ID: test.key, Data: test.data}}); err != nil {
				t.Fatal(err)
			}
			if _, err := ListLeaseHolderIndexesV1(stateRoot); err == nil {
				t.Fatal("expected corrupt reverse index to fail closed")
			}
		})
	}
}

func mustMarshalLeaseHolderIndexV1(t *testing.T, value leaseHolderIndexV1) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
