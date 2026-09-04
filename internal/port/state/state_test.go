package state_test

import (
	"testing"

	stateport "issueops/internal/port/state"
)

type memoryStore struct{}

func (memoryStore) Get(string, string) ([]byte, bool, error) { return nil, false, nil }
func (memoryStore) Mutate([]stateport.Mutation) error        { return nil }

type memoryExistingReader struct{}

func (memoryExistingReader) GetExisting(string, string, string) ([]byte, bool, error) {
	return nil, false, nil
}

func TestTransactionalStoreIsCapabilityMinimal(t *testing.T) {
	var store stateport.TransactionalStore = memoryStore{}
	if _, ok, err := store.Get("bucket", "id"); err != nil || ok {
		t.Fatalf("unexpected read result: ok=%v err=%v", ok, err)
	}
}

func TestExistingReaderIsCapabilityMinimal(t *testing.T) {
	var reader stateport.ExistingReader = memoryExistingReader{}
	if _, ok, err := reader.GetExisting("/state", "bucket", "id"); err != nil || ok {
		t.Fatalf("unexpected existing read result: ok=%v err=%v", ok, err)
	}
}
