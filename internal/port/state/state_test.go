package state_test

import (
	"testing"

	stateport "agent-harness/internal/port/state"
)

type memoryStore struct{}

func (memoryStore) Get(string, string) ([]byte, bool, error) { return nil, false, nil }
func (memoryStore) Mutate([]stateport.Mutation) error        { return nil }

func TestTransactionalStoreIsCapabilityMinimal(t *testing.T) {
	var store stateport.TransactionalStore = memoryStore{}
	if _, ok, err := store.Get("bucket", "id"); err != nil || ok {
		t.Fatalf("unexpected read result: ok=%v err=%v", ok, err)
	}
}
