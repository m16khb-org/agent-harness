package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/port"
)

const (
	recordBucket = "issueops_v1"
	holderBucket = "lease_holder_v1"
)

type SQLiteRepository struct{ store port.TransactionalRecordStore }

func NewSQLiteRepository(store port.TransactionalRecordStore) *SQLiteRepository {
	return &SQLiteRepository{store: store}
}

func (r *SQLiteRepository) Update(
	ctx context.Context,
	id string,
	validate leaseapp.RecordValidator,
	transition leaseapp.RecordTransition,
) (leaseapp.RepositoryResult, error) {
	if r.store == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	var after leaseapp.RepositoryResult
	var operationErr error
	// record와 reverse index를 같은 span 안에서 읽고 반영해 stale writer를 막는다.
	spanErr := r.store.WithSpan(ctx, func(spanCtx context.Context) error {
		after, operationErr = updateWithinSpan(spanCtx, r.store, id, validate, transition)
		return operationErr
	})
	if operationErr != nil {
		return after, operationErr
	}
	if spanErr != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, spanErr)
	}
	return after, nil
}

func updateWithinSpan(
	ctx context.Context,
	store port.TransactionalRecordStore,
	id string,
	validate leaseapp.RecordValidator,
	transition leaseapp.RecordTransition,
) (leaseapp.RepositoryResult, error) {
	data, ok, err := store.Get(recordBucket, id)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := leasecontract.Decode(id, data)
	if err != nil {
		var unsupported leasecontract.UnsupportedSchemaError
		switch {
		case errors.Is(err, leasecontract.ErrMalformedSchema):
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailureMalformedSchema, err)
		case errors.As(err, &unsupported):
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailureUnsupportedSchema, err)
		default:
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
		}
	}
	if record.Execution == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	before := toApplicationRecord(record)
	// 권한과 canonical CWD는 index 상태보다 먼저 판정해 legacy 공개 오류 우선순위를 보존한다.
	if err := validate(before); err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	indexKey := holderIndexKey(*before.Lease.Holder)
	indexData, indexExists, err := store.Get(holderBucket, indexKey)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	mutations := []port.RecordMutation{}
	if indexExists {
		var index holderIndex
		if err := json.Unmarshal(indexData, &index); err != nil {
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
		}
		if index.LifecycleID != before.ID {
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("refusing to delete another lifecycle's lease-holder index"))
		}
		mutations = append(mutations, port.RecordMutation{Bucket: holderBucket, ID: indexKey, Delete: true})
	}
	after, err := transition(before)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	record.Execution.Lease = after.Lease
	data, err = leasecontract.Encode(record)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	mutations = append([]port.RecordMutation{{Bucket: recordBucket, ID: after.ID, Data: data}}, mutations...)
	if err := store.Apply(ctx, mutations); err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	return leaseapp.RepositoryResult{Record: after, Execution: *record.Execution}, nil
}

type holderIndex struct {
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

func holderIndexKey(actor leasecontract.Actor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}
func toApplicationRecord(record leasecontract.Record) leaseapp.Record {
	return leaseapp.Record{ID: record.ID, CanonicalRoot: record.Execution.Workspace.Root, Lease: record.Execution.Lease}
}
