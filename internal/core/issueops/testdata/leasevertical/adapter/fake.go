package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"agent-harness/internal/core/issueops/testdata/leasevertical/contract"
	"agent-harness/internal/core/issueops/testdata/leasevertical/domain"
)

type FakeRepository struct {
	mu          sync.Mutex
	id          string
	record      []byte
	holderIndex map[string]holderIndex
	nextSaveErr error
}

func NewFakeRepository(id string, data []byte) (*FakeRepository, error) {
	record, err := contract.Decode(id, data)
	if err != nil {
		return nil, err
	}
	indexes := map[string]holderIndex{}
	if record.Execution != nil && record.Execution.Lease.Holder != nil {
		holder := toDomainLease(record.Execution.Lease).Holder
		indexes[holderIndexKey(*holder)] = holderIndex{
			SchemaVersion: contract.SchemaVersion,
			LifecycleID:   id,
			Generation:    record.Execution.Lease.Generation,
			Host:          holder.Host,
			SessionID:     holder.SessionID,
			AgentID:       holder.AgentID,
		}
	}
	return &FakeRepository{
		id: id, record: append([]byte(nil), data...), holderIndex: indexes,
	}, nil
}

func (r *FakeRepository) Update(
	_ context.Context,
	id string,
	transition func(domain.Record) (domain.Record, error),
) (domain.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.id {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := contract.Decode(id, r.record)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	if record.Execution == nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("execution is missing"))
	}
	before := toDomain(record)
	after, err := transition(before)
	if err != nil {
		return domain.Record{}, err
	}
	if r.nextSaveErr != nil {
		err := r.nextSaveErr
		r.nextSaveErr = nil
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	if before.Execution.Lease.Holder == nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("active holder is missing"))
	}
	indexKey := holderIndexKey(*before.Execution.Lease.Holder)
	index, ok := r.holderIndex[indexKey]
	if ok && index.LifecycleID != before.ID {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("active holder index is unavailable"))
	}
	record.Execution.Lease = fromDomainLease(after.Execution.Lease)
	data, err := contract.Encode(record)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	r.record = data
	if ok {
		delete(r.holderIndex, indexKey)
	}
	return after, nil
}

func (r *FakeRepository) FailNextSave(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSaveErr = err
}

func (r *FakeRepository) StateBytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	var snapshot bytes.Buffer
	snapshot.Write(r.record)
	keys := make([]string, 0, len(r.holderIndex))
	for key := range r.holderIndex {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		data, _ := json.Marshal(r.holderIndex[key])
		snapshot.WriteByte(0)
		snapshot.WriteString(key)
		snapshot.WriteByte(0)
		snapshot.Write(data)
	}
	return snapshot.Bytes()
}
