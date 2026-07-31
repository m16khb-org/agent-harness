package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

var leaseHolderBucket = fmt.Sprintf("lease_holder_v%d", model.IssueOpsSchemaVersion)

type LeaseHolderIndex struct {
	Key           string `json:"-"`
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

type leaseHolderIndex = LeaseHolderIndex

// ListLeaseHolderIndexes는 state를 생성하거나 복구하지 않고 reverse index를
// 읽는다. 모든 row는 content에서 파생한 key와 대조해 검증한다.
func ListLeaseHolderIndexes(stateRoot string) ([]LeaseHolderIndex, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, leaseHolderBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []LeaseHolderIndex{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]LeaseHolderIndex, 0, len(rows))
	for _, row := range rows {
		var index LeaseHolderIndex
		if err := json.Unmarshal(row.Data, &index); err != nil {
			return nil, fmt.Errorf("decode active lease-holder index %s: %w", row.ID, err)
		}
		if index.SchemaVersion != model.IssueOpsSchemaVersion || index.Generation == 0 || strings.TrimSpace(index.LifecycleID) == "" ||
			(index.Host != "codex" && index.Host != "claude") || strings.TrimSpace(index.SessionID) == "" {
			return nil, fmt.Errorf("active lease-holder index %s is invalid", row.ID)
		}
		if _, err := normalizeIssueOpsID(index.LifecycleID); err != nil {
			return nil, fmt.Errorf("active lease-holder index %s lifecycle identity is invalid: %w", row.ID, err)
		}
		actor := model.NativeActor{Host: index.Host, SessionID: index.SessionID, AgentID: index.AgentID}
		if expected := leaseHolderIndexKey(actor); row.ID != expected {
			return nil, fmt.Errorf("active lease-holder index %s key does not match its actor identity", row.ID)
		}
		index.Key = row.ID
		result = append(result, index)
	}
	return result, nil
}

func persistExecutionTransition(stateRoot string, record IssueOpsRecord, previousHolder *model.NativeActor) (IssueOpsRecord, error) {
	return persistExecutionTransitionWithMutations(stateRoot, record, previousHolder, nil)
}

func persistExecutionTransitionWithMutations(stateRoot string, record IssueOpsRecord, previousHolder *model.NativeActor, extra []sqlstore.Mutation) (IssueOpsRecord, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, err
	}
	encoded, data, err := encodeIssueOpsRecord(record)
	if err != nil {
		return encoded, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, err
	}
	mutations := []sqlstore.Mutation{{Bucket: issueOpsBucket, ID: encoded.ID, Data: data}}
	if previousHolder != nil {
		mutation, err := leaseIndexDeleteMutation(db, encoded.ID, *previousHolder)
		if err != nil {
			return IssueOpsRecord{OK: false, ID: encoded.ID}, err
		}
		if mutation != nil {
			mutations = append(mutations, *mutation)
		}
	}
	if encoded.Execution != nil && encoded.Execution.Lease.Status == model.LeaseStatusActive {
		holder := encoded.Execution.Lease.Holder
		if holder == nil {
			return IssueOpsRecord{OK: false, ID: encoded.ID}, fmt.Errorf("active execution lease is missing its holder")
		}
		alreadyCurrent, err := requireLeaseIndexAvailable(db, encoded.ID, encoded.Execution.Lease.Generation, *holder)
		if err != nil {
			return IssueOpsRecord{OK: false, ID: encoded.ID}, err
		}
		index := leaseHolderIndex{
			SchemaVersion: model.IssueOpsSchemaVersion,
			LifecycleID:   encoded.ID,
			Generation:    encoded.Execution.Lease.Generation,
			Host:          holder.Host,
			SessionID:     holder.SessionID,
			AgentID:       holder.AgentID,
		}
		indexData, err := json.Marshal(index)
		if err != nil {
			return IssueOpsRecord{OK: false, ID: encoded.ID}, err
		}
		if !alreadyCurrent {
			mutations = append(mutations, sqlstore.Mutation{
				Bucket: leaseHolderBucket, ID: leaseHolderIndexKey(*holder), Data: indexData, RequireAbsent: true,
			})
		}
	}
	mutations = append(mutations, extra...)
	if err := db.Apply(context.Background(), mutations); err != nil {
		return IssueOpsRecord{OK: false, ID: encoded.ID}, err
	}
	return encoded, nil
}

// persistExecutionTransitionWithRawCAS는 resume stage가 읽은 record와 intent를
// 같은 SQLite transaction에서 다시 대조한 뒤 저장한다. resume lease는
// holderless claimable 상태를 유지하므로 lease-holder reverse index transition을
// 여기로 옮기지 않는다.
func persistExecutionTransitionWithRawCAS(stateRoot string, record IssueOpsRecord, expected []sqlstore.ExpectedRecord, extra []sqlstore.Mutation) (IssueOpsRecord, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, err
	}
	var encoded IssueOpsRecord
	if err := db.CompareAndApplyFunc(context.Background(), expected, func() ([]sqlstore.Mutation, error) {
		var data []byte
		var encodeErr error
		encoded, data, encodeErr = encodeIssueOpsRecord(record)
		if encodeErr != nil {
			return nil, encodeErr
		}
		mutations := append([]sqlstore.Mutation{{Bucket: issueOpsBucket, ID: encoded.ID, Data: data}}, extra...)
		return mutations, nil
	}); err != nil {
		var stale *sqlstore.RawCASError
		if errors.As(err, &stale) {
			if stale.Bucket == issueOpsBucket {
				return IssueOpsRecord{OK: false, ID: record.ID}, fmt.Errorf("stale raw record snapshot")
			}
			return IssueOpsRecord{OK: false, ID: record.ID}, fmt.Errorf("stale raw intent snapshot")
		}
		return IssueOpsRecord{OK: false, ID: record.ID}, err
	}
	return encoded, nil
}

func requireLeaseIndexAvailable(db *sqlstore.DB, lifecycleID string, generation uint64, actor model.NativeActor) (bool, error) {
	data, ok, err := db.Get(leaseHolderBucket, leaseHolderIndexKey(actor))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	var existing leaseHolderIndex
	if err := json.Unmarshal(data, &existing); err != nil {
		return false, fmt.Errorf("decode active lease-holder index: %w", err)
	}
	if existing.SchemaVersion != model.IssueOpsSchemaVersion {
		return false, fmt.Errorf("unsupported lease-holder index schema %d", existing.SchemaVersion)
	}
	if existing.LifecycleID == lifecycleID && existing.Generation == generation {
		return true, nil
	}
	return false, fmt.Errorf("native session already holds active lease %s generation %d", existing.LifecycleID, existing.Generation)
}

func leaseIndexDeleteMutation(db *sqlstore.DB, lifecycleID string, actor model.NativeActor) (*sqlstore.Mutation, error) {
	key := leaseHolderIndexKey(actor)
	data, ok, err := db.Get(leaseHolderBucket, key)
	if err != nil || !ok {
		return nil, err
	}
	var existing leaseHolderIndex
	if err := json.Unmarshal(data, &existing); err != nil {
		return nil, fmt.Errorf("decode active lease-holder index: %w", err)
	}
	if existing.LifecycleID != lifecycleID {
		return nil, fmt.Errorf("refusing to delete another lifecycle's lease-holder index")
	}
	return &sqlstore.Mutation{Bucket: leaseHolderBucket, ID: key, Delete: true}, nil
}

func leaseHolderIndexKey(actor model.NativeActor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}
