package issueopslease

import (
	"bytes"
	"context"
	"fmt"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

type ReseedRepository struct{ store port.TransactionalRecordStore }

type ReseedInventoryFunc func(context.Context, leasecontract.Record, leasedomain.Actor) (leaseapp.ReseedInventoryReceipt, error)

func (f ReseedInventoryFunc) Observe(ctx context.Context, record leasecontract.Record, actor leasedomain.Actor) (leaseapp.ReseedInventoryReceipt, error) {
	return f(ctx, record, actor)
}

func NewReseedRepository(store port.TransactionalRecordStore) *ReseedRepository {
	return &ReseedRepository{store: store}
}

func (r *ReseedRepository) LoadSnapshot(_ context.Context, id string) (leaseapp.ReseedSnapshot, error) {
	if r == nil || r.store == nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	data, ok, err := r.store.Get(recordBucket, id)
	if err != nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := decodeLeaseRecord(id, data)
	if err != nil {
		return leaseapp.ReseedSnapshot{}, err
	}
	if record.Execution == nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	return leaseapp.ReseedSnapshot{Record: toApplicationRecord(record), Raw: append([]byte(nil), data...)}, nil
}

func (r *ReseedRepository) CommitReseed(ctx context.Context, snapshot leaseapp.ReseedSnapshot, next leaseapp.Record) (leaseapp.RepositoryResult, error) {
	if r == nil || r.store == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	var result leaseapp.RepositoryResult
	var operationErr error
	spanErr := r.store.WithSpan(ctx, func(spanCtx context.Context) error {
		data, ok, err := r.store.Get(recordBucket, snapshot.Record.ID)
		if err != nil {
			operationErr = leasecontract.Fail(leasecontract.FailurePersistence, err)
			return operationErr
		}
		if !ok {
			operationErr = leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", snapshot.Record.ID))
			return operationErr
		}
		if !bytes.Equal(data, snapshot.Raw) {
			operationErr = fmt.Errorf("stale raw record snapshot")
			return operationErr
		}
		current, err := decodeLeaseRecord(snapshot.Record.ID, data)
		if err != nil {
			operationErr = err
			return err
		}
		if current.Execution == nil {
			operationErr = leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
			return operationErr
		}
		if current.Execution.Lease.Generation != snapshot.Record.Lease.Generation {
			operationErr = fmt.Errorf("stale lease generation: current=%d expected=%d", current.Execution.Lease.Generation, snapshot.Record.Lease.Generation)
			return operationErr
		}
		encoded, err := leasecontract.Encode(next.Stable)
		if err != nil {
			operationErr = leasecontract.Fail(leasecontract.FailurePersistence, err)
			return operationErr
		}
		if err := r.store.Apply(spanCtx, []port.RecordMutation{{Bucket: recordBucket, ID: next.ID, Data: encoded}}); err != nil {
			operationErr = leasecontract.Fail(leasecontract.FailurePersistence, err)
			return operationErr
		}
		result = leaseapp.RepositoryResult{Record: next, Execution: *next.Stable.Execution}
		return nil
	})
	if operationErr != nil {
		return leaseapp.RepositoryResult{}, operationErr
	}
	if spanErr != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, spanErr)
	}
	return result, nil
}
