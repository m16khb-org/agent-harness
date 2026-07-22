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

const leaseHolderV1Bucket = "lease_holder_v1"

type LeaseHolderIndexV1 struct {
	Key           string `json:"-"`
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

type leaseHolderIndexV1 = LeaseHolderIndexV1

// ListLeaseHolderIndexesV1 reads the reverse index without creating or
// repairing state. Every row is validated against its content-derived key.
func ListLeaseHolderIndexesV1(stateRoot string) ([]LeaseHolderIndexV1, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, leaseHolderV1Bucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []LeaseHolderIndexV1{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]LeaseHolderIndexV1, 0, len(rows))
	for _, row := range rows {
		var index LeaseHolderIndexV1
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
		actor := model.NativeActorV1{Host: index.Host, SessionID: index.SessionID, AgentID: index.AgentID}
		if expected := leaseHolderIndexKey(actor); row.ID != expected {
			return nil, fmt.Errorf("active lease-holder index %s key does not match its actor identity", row.ID)
		}
		index.Key = row.ID
		result = append(result, index)
	}
	return result, nil
}

func persistExecutionTransition(stateRoot string, record IssueOpsRecord, previousHolder *model.NativeActorV1) (IssueOpsRecord, error) {
	return persistExecutionTransitionWithMutations(stateRoot, record, previousHolder, nil)
}

func persistExecutionTransitionWithMutations(stateRoot string, record IssueOpsRecord, previousHolder *model.NativeActorV1, extra []sqlstore.Mutation) (IssueOpsRecord, error) {
	if err := RequireIssueOpsV1MutationAllowed(stateRoot); err != nil {
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
		index := leaseHolderIndexV1{
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
				Bucket: leaseHolderV1Bucket, ID: leaseHolderIndexKey(*holder), Data: indexData, RequireAbsent: true,
			})
		}
	}
	mutations = append(mutations, extra...)
	if err := db.Apply(context.Background(), mutations); err != nil {
		return IssueOpsRecord{OK: false, ID: encoded.ID}, err
	}
	return encoded, nil
}

func requireLeaseIndexAvailable(db *sqlstore.DB, lifecycleID string, generation uint64, actor model.NativeActorV1) (bool, error) {
	data, ok, err := db.Get(leaseHolderV1Bucket, leaseHolderIndexKey(actor))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	var existing leaseHolderIndexV1
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

func leaseIndexDeleteMutation(db *sqlstore.DB, lifecycleID string, actor model.NativeActorV1) (*sqlstore.Mutation, error) {
	key := leaseHolderIndexKey(actor)
	data, ok, err := db.Get(leaseHolderV1Bucket, key)
	if err != nil || !ok {
		return nil, err
	}
	var existing leaseHolderIndexV1
	if err := json.Unmarshal(data, &existing); err != nil {
		return nil, fmt.Errorf("decode active lease-holder index: %w", err)
	}
	if existing.LifecycleID != lifecycleID {
		return nil, fmt.Errorf("refusing to delete another lifecycle's lease-holder index")
	}
	return &sqlstore.Mutation{Bucket: leaseHolderV1Bucket, ID: key, Delete: true}, nil
}

func leaseHolderIndexKey(actor model.NativeActorV1) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}
