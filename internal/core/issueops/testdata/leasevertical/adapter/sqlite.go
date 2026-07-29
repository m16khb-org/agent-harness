package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/testdata/leasevertical/contract"
	"agent-harness/internal/core/issueops/testdata/leasevertical/domain"
	"agent-harness/internal/core/sqlstore"
)

const (
	recordBucket = "issueops_v1"
	holderBucket = "lease_holder_v1"
)

type SQLiteRepository struct {
	root string
}

func NewSQLiteRepository(root string) *SQLiteRepository {
	return &SQLiteRepository{root: root}
}

func (r *SQLiteRepository) Update(
	ctx context.Context,
	id string,
	transition func(domain.Record) (domain.Record, error),
) (domain.Record, error) {
	db, err := sqlstore.Open(r.root)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	var after domain.Record
	var operationErr error
	// 프로세스마다 별도 adapter를 열어도 generation 확인부터 record/index 반영까지
	// 같은 SQLite span에 묶어 stale release가 최신 lease를 덮어쓰지 못하게 한다.
	spanErr := db.WithSpan(ctx, func(spanCtx context.Context) error {
		after, operationErr = updateWithinSpan(spanCtx, db, id, transition)
		return operationErr
	})
	if operationErr != nil {
		return after, operationErr
	}
	if spanErr != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, spanErr)
	}
	return after, nil
}

func updateWithinSpan(
	ctx context.Context,
	db *sqlstore.DB,
	id string,
	transition func(domain.Record) (domain.Record, error),
) (domain.Record, error) {
	data, ok, err := db.Get(recordBucket, id)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	if !ok {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := contract.Decode(id, data)
	if err != nil {
		var unsupported contract.UnsupportedSchemaError
		switch {
		case errors.Is(err, contract.ErrMalformedSchema):
			return domain.Record{}, domain.Deny(domain.DenyMalformedSchema, err)
		case errors.As(err, &unsupported):
			return domain.Record{}, domain.Deny(domain.DenyUnsupportedSchema, err)
		default:
			return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
		}
	}
	if record.Execution == nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("execution is missing"))
	}
	before := toDomain(record)
	after, err := transition(before)
	if err != nil {
		return domain.Record{}, err
	}
	if before.Execution.Lease.Holder == nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("active holder is missing"))
	}
	indexKey := holderIndexKey(*before.Execution.Lease.Holder)
	indexData, ok, err := db.Get(holderBucket, indexKey)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	mutations := []sqlstore.Mutation{}
	if ok {
		var index holderIndex
		if err := json.Unmarshal(indexData, &index); err != nil {
			return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
		}
		if index.LifecycleID != before.ID {
			// 다른 lifecycle의 reverse index를 지우면 두 lease가 동시에 유효해질 수 있다.
			return domain.Record{}, domain.Deny(domain.DenyPersistence, fmt.Errorf("refusing to delete another lifecycle"))
		}
		mutations = append(mutations, sqlstore.Mutation{Bucket: holderBucket, ID: indexKey, Delete: true})
	}
	record.Execution.Lease = fromDomainLease(after.Execution.Lease)
	data, err = contract.Encode(record)
	if err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	// record 갱신과 존재하는 reverse index 삭제는 adapter의 한 critical section과
	// SQLite transaction 안에서만 반영해 중간 상태를 노출하지 않는다.
	mutations = append([]sqlstore.Mutation{{Bucket: recordBucket, ID: after.ID, Data: data}}, mutations...)
	if err := db.Apply(ctx, mutations); err != nil {
		return domain.Record{}, domain.Deny(domain.DenyPersistence, err)
	}
	return after, nil
}

type holderIndex struct {
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

func holderIndexKey(actor domain.Actor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" +
		strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}

func toDomain(record contract.Record) domain.Record {
	execution := record.Execution
	return domain.Record{
		ID: record.ID,
		Execution: domain.Execution{
			Mode: execution.Mode,
			Workspace: domain.Workspace{
				SourceRoot: execution.Workspace.SourceRoot, Root: execution.Workspace.Root,
				Branch: execution.Workspace.Branch, BaseHead: execution.Workspace.BaseHead,
				ParentWorktree: execution.Workspace.ParentWorktree, Driver: execution.Workspace.Driver,
				LinkedAt: execution.Workspace.LinkedAt,
			},
			Lease: toDomainLease(execution.Lease),
		},
	}
}

func toDomainLease(lease contract.Lease) domain.Lease {
	result := domain.Lease{
		Generation: lease.Generation, Status: lease.Status, ClaimTokenSHA256: lease.ClaimTokenSHA256,
		ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt,
		ReplacementReason: lease.ReplacementReason,
	}
	if lease.Holder != nil {
		result.Holder = &domain.Actor{
			Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID,
		}
		if lease.Holder.SessionProcess != nil {
			result.Holder.Process = &domain.ProcessReceipt{
				PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt,
				Executable: lease.Holder.SessionProcess.Executable,
			}
		}
	}
	return result
}

func fromDomainLease(lease domain.Lease) contract.Lease {
	result := contract.Lease{
		Generation: lease.Generation, Status: lease.Status, ClaimTokenSHA256: lease.ClaimTokenSHA256,
		ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt,
		ReplacementReason: lease.ReplacementReason,
	}
	if lease.Holder != nil {
		result.Holder = &contract.Actor{
			Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID,
		}
		if lease.Holder.Process != nil {
			result.Holder.SessionProcess = &contract.ProcessReceipt{
				PID: lease.Holder.Process.PID, StartedAt: lease.Holder.Process.StartedAt,
				Executable: lease.Holder.Process.Executable,
			}
		}
	}
	return result
}
