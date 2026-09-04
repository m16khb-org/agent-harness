package state

import (
	"context"
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
	stateport "issueops/internal/port/state"
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

// List/WriteRecord/Prune 조합을 실제 memory store로 잠근다. Prune은
// dry-run에서 삭제를 미루고 confirm에서만 삭제한다.
func TestServiceListWriteRecordAndPrune(t *testing.T) {
	store := &memoryStore{records: map[string][]byte{}}
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	service := NewService(Dependencies{
		StateDir:        func() string { return "/state" },
		StatePath:       func(dir, key string) string { return dir + "/" + key + ".json" },
		OpenStore:       func(string) (stateport.Store, error) { return store, nil },
		ExistingRecords: memoryExistingReader{store: store},
		Now:             func() time.Time { return now },
	})

	// WriteRecord는 UpdatedAt을 호출자가 채운 envelope를 그대로 쓴다.
	// prune 나이 판정은 이 값으로 한다.
	envelope := statecontract.RecordEnvelope{
		SchemaVersion: statecontract.SchemaVersion, Key: "rec-old", Content: "old",
		UpdatedAt: now.Add(-800 * time.Hour).UTC().Format(time.RFC3339Nano),
	}
	envelope.Bytes = len("old")
	if _, err := service.WriteRecord("/state", "rec-old", envelope); err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}
	// envelope key가 쓰기 key와 불일치하면 거부한다.
	mismatch := statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: "different", Content: "x"}
	mismatch.Bytes = 1
	if _, err := service.WriteRecord("/state", "rec-x", mismatch); err == nil {
		t.Fatal("key mismatch must fail")
	}
	// envelope 불변식 위반(schema/bytes)도 거부한다.
	if _, err := service.WriteRecord("/state", "rec-bad", statecontract.RecordEnvelope{Key: "rec-bad", Content: "abc"}); err == nil {
		t.Fatal("invalid envelope invariant must fail")
	}
	listed, err := service.List()
	if err != nil || len(listed.Records) != 1 || listed.Records[0].Key != "rec-old" {
		t.Fatalf("List = %+v err=%v", listed, err)
	}

	// dry-run: 오래된 기록을 골라도 삭제하지 않는다.
	dry, err := service.Prune(720*time.Hour, false)
	if err != nil || !dry.OK || !dry.DryRun || dry.Confirm {
		t.Fatalf("dry-run prune = %+v err=%v", dry, err)
	}
	// dry-run도 삭제 대상을 보고한다(DeletedKeys). 실제 삭제는 confirm에서만.
	if len(dry.Pruned) == 0 || len(dry.DeletedKeys) != 1 {
		t.Fatalf("dry-run must report the selection: %+v", dry)
	}
	afterDry, _ := service.List()
	if len(afterDry.Records) != 1 {
		t.Fatalf("dry-run must not delete: %+v", afterDry)
	}
	// confirm: 실제로 삭제한다.
	confirmed, err := service.Prune(720*time.Hour, true)
	if err != nil || !confirmed.OK || confirmed.DryRun || !confirmed.Confirm {
		t.Fatalf("confirm prune = %+v err=%v", confirmed, err)
	}
	afterConfirm, _ := service.List()
	if len(afterConfirm.Records) != 0 {
		t.Fatalf("confirm must delete: %+v", afterConfirm)
	}
	// 검증: 잘못된 max-age/prefix는 즉시 거부.
	if _, err := service.Prune(0, true); err == nil {
		t.Fatal("zero max-age must fail")
	}
	if _, err := service.PrunePrefix("", time.Hour, 1, true); err == nil {
		t.Fatal("empty prefix must fail")
	}
}
