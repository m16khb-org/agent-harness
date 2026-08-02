package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	statecontract "agent-harness/internal/contract/state"
	leasedomain "agent-harness/internal/domain/issueopslease"
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

func (r *SQLiteRepository) Claim(ctx context.Context, request leaseapp.ClaimRepositoryRequest) (leaseapp.RepositoryResult, error) {
	if r.store == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	var result leaseapp.RepositoryResult
	var operationErr error
	spanErr := r.store.WithSpan(ctx, func(spanCtx context.Context) error {
		result, operationErr = claimWithinSpan(spanCtx, r.store, request)
		return operationErr
	})
	if operationErr != nil {
		return result, operationErr
	}
	if spanErr != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, spanErr)
	}
	return result, nil
}

func claimWithinSpan(ctx context.Context, store port.TransactionalRecordStore, request leaseapp.ClaimRepositoryRequest) (leaseapp.RepositoryResult, error) {
	data, ok, err := store.Get(recordBucket, request.ID)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", request.ID))
	}
	record, err := decodeLeaseRecord(request.ID, data)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	if record.Execution == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	before := toApplicationRecord(record)
	if leasedomain.IsClaimRetry(toDomainLease(before.Lease), request.Generation, request.Actor) {
		return leaseapp.RepositoryResult{Record: before, Execution: *record.Execution}, nil
	}
	canonicalCWD := (FilesystemPathMatcher{}).Matches(request.CWD, before.CanonicalRoot)
	if err := leasedomain.ValidateClaim(toDomainLease(before.Lease), leasedomain.ClaimRequest{
		Generation: request.Generation, Actor: request.Actor, AuthorityVerified: true, CanonicalCWD: canonicalCWD, TokenVerified: true,
	}); err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	if request.ValidateRecord == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("claim record validator is required"))
	}
	if err := request.ValidateRecord(before); err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	token, err := readCurrentClaimToken(record, request.TokenFile)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	if claimTokenSHA256(token) != before.Lease.ClaimTokenSHA256 {
		return leaseapp.RepositoryResult{}, leasedomain.ValidateClaim(toDomainLease(before.Lease), leasedomain.ClaimRequest{
			Generation: request.Generation, Actor: request.Actor, AuthorityVerified: true, CanonicalCWD: canonicalCWD, TokenVerified: false,
		})
	}
	if request.Clock == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("claim clock is required"))
	}
	outcome := leasedomain.ApplyClaim(request.Clock.Now(), request.Actor)
	record.Execution.Lease.Status = outcome.Status
	record.Execution.Lease.Holder = toContractActor(*outcome.Holder)
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Lease.ClaimedAt = outcome.ClaimedAt
	record.Execution.Lease.ReleasedAt = outcome.ReleasedAt
	encoded, err := leasecontract.Encode(record)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	indexData, err := json.Marshal(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Generation: record.Execution.Lease.Generation, Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID})
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	indexKey := holderIndexKey(*record.Execution.Lease.Holder)
	existing, exists, err := store.Get(holderBucket, indexKey)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	mutations := []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: encoded}}
	if exists {
		var index holderIndex
		if err := json.Unmarshal(existing, &index); err != nil {
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("decode active lease-holder index: %w", err))
		}
		if index.SchemaVersion != leasecontract.SchemaVersion {
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("unsupported lease-holder index schema %d", index.SchemaVersion))
		}
		if index.LifecycleID != record.ID || index.Generation != record.Execution.Lease.Generation {
			return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("native session already holds active lease %s generation %d", index.LifecycleID, index.Generation))
		}
	} else {
		mutations = append(mutations, port.RecordMutation{Bucket: holderBucket, ID: indexKey, Data: indexData, RequireAbsent: true})
	}
	if err := store.Apply(ctx, mutations); err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	_ = os.Remove(request.TokenFile)
	after := toApplicationRecord(record)
	return leaseapp.RepositoryResult{Record: after, Execution: *record.Execution}, nil
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
	record, err := decodeLeaseRecord(id, data)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
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
	return leaseapp.Record{ID: record.ID, SourceRoot: record.Execution.Workspace.SourceRoot, CanonicalRoot: record.Execution.Workspace.Root, Lease: record.Execution.Lease, Stable: record}
}

func decodeLeaseRecord(id string, data []byte) (leasecontract.Record, error) {
	record, err := leasecontract.Decode(id, data)
	if err == nil {
		return record, nil
	}
	if errors.Is(err, statecontract.ErrInvalidState) {
		return leasecontract.Record{}, leasecontract.Fail(leasecontract.FailureInvalidState, err)
	}
	return leasecontract.Record{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
}

func toDomainLease(lease leasecontract.Lease) leasedomain.Lease {
	result := leasedomain.Lease{Generation: lease.Generation, Status: lease.Status}
	if lease.Holder != nil {
		actor := leasedomain.Actor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			actor.Process = &leasedomain.ProcessReceipt{PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt, Executable: lease.Holder.SessionProcess.Executable}
		}
		result.Holder = &actor
	}
	return result
}

func toContractActor(actor leasedomain.Actor) *leasecontract.Actor {
	result := &leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}
