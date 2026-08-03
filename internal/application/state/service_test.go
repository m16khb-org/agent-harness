package state

import (
	"context"
	"testing"
	"time"

	statecontract "agent-harness/internal/contract/state"
	stateport "agent-harness/internal/port/state"
)

func TestServiceOwnsStateReadWriteAndUpdateOrchestration(t *testing.T) {
	store := &memoryStore{records: map[string][]byte{}}
	service := NewService(Dependencies{
		StateDir:        func() string { return "/state" },
		StatePath:       func(dir, key string) string { return dir + "/" + key + ".json" },
		OpenStore:       func(string) (stateport.Store, error) { return store, nil },
		ExistingRecords: memoryExistingReader{store: store},
		Now:             func() time.Time { return time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC) },
	})

	written, err := service.Write("counter", "1")
	if err != nil || !written.OK || written.Record.UpdatedAt != "2026-08-04T01:02:03Z" {
		t.Fatalf("Write() = %+v, %v", written, err)
	}
	updated, err := service.Update("counter", func(current statecontract.RecordEnvelope) (statecontract.RecordEnvelope, error) {
		current.Content = "12"
		current.Bytes = 2
		return current, nil
	})
	if err != nil || updated.Record.Content != "12" || store.spanCalls != 2 {
		t.Fatalf("Update() = %+v, %v; spans=%d", updated, err, store.spanCalls)
	}
	read, err := service.Read("counter")
	if err != nil || read.Record.Content != "12" {
		t.Fatalf("Read() = %+v, %v", read, err)
	}
}

type memoryExistingReader struct {
	store *memoryStore
}

func (r memoryExistingReader) GetExisting(_ string, bucket, id string) ([]byte, bool, error) {
	return r.store.Get(bucket, id)
}

var _ stateport.ExistingReader = memoryExistingReader{}

type memoryStore struct {
	records   map[string][]byte
	spanCalls int
}

func (s *memoryStore) Get(bucket, id string) ([]byte, bool, error) {
	value, ok := s.records[bucket+"/"+id]
	return append([]byte(nil), value...), ok, nil
}

func (s *memoryStore) Mutate(mutations []stateport.Mutation) error {
	for _, mutation := range mutations {
		key := mutation.Bucket + "/" + mutation.ID
		if mutation.Delete {
			delete(s.records, key)
			continue
		}
		s.records[key] = append([]byte(nil), mutation.Data...)
	}
	return nil
}

func (s *memoryStore) List(bucket string) ([]string, error) {
	var ids []string
	for key := range s.records {
		if len(key) > len(bucket)+1 && key[:len(bucket)+1] == bucket+"/" {
			ids = append(ids, key[len(bucket)+1:])
		}
	}
	return ids, nil
}

func (s *memoryStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	s.spanCalls++
	return fn(ctx)
}
