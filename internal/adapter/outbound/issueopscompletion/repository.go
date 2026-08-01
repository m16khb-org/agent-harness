package issueopscompletion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	completionapp "agent-harness/internal/application/issueopscompletion"
	completioncontract "agent-harness/internal/contract/issueopscompletion"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/port"
)

const (
	recordBucket = "issueops_v1"
	holderBucket = "lease_holder_v1"
)

type Repository struct{ store port.TransactionalRecordStore }

func NewRepository(store port.TransactionalRecordStore) *Repository { return &Repository{store: store} }

func (r *Repository) Update(ctx context.Context, id string, transition completionapp.RecordTransition) (completionapp.RepositoryResult, error) {
	if r == nil || r.store == nil {
		return completionapp.RepositoryResult{}, fmt.Errorf("transactional record store is required")
	}
	var result completionapp.RepositoryResult
	var operationErr error
	spanErr := r.store.WithSpan(ctx, func(spanCtx context.Context) error {
		result, operationErr = updateWithinSpan(spanCtx, r.store, id, transition)
		return operationErr
	})
	if operationErr != nil {
		return result, operationErr
	}
	if spanErr != nil {
		return completionapp.RepositoryResult{}, spanErr
	}
	return result, nil
}

func updateWithinSpan(ctx context.Context, store port.TransactionalRecordStore, id string, transition completionapp.RecordTransition) (completionapp.RepositoryResult, error) {
	data, ok, err := store.Get(recordBucket, id)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	if !ok {
		return completionapp.RepositoryResult{}, fmt.Errorf("issueops record %s not found", id)
	}
	record, err := leasecontract.Decode(id, data)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	before, err := snapshot(record)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	after, changed, err := transition(before.Clone())
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	if !changed {
		if record.Execution == nil {
			return completionapp.RepositoryResult{Record: before}, nil
		}
		return completionapp.RepositoryResult{Record: before, Execution: *record.Execution}, nil
	}
	if record.Execution == nil {
		return completionapp.RepositoryResult{}, leasecontract.ErrExecutionNotPrepared
	}
	previousHolder := record.Execution.Lease.Holder
	if previousHolder == nil {
		return completionapp.RepositoryResult{}, fmt.Errorf("active execution lease is missing its holder")
	}
	if err := applySnapshot(&record, after); err != nil {
		return completionapp.RepositoryResult{}, err
	}
	encoded, err := leasecontract.Encode(record)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	mutations := []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: encoded}}
	indexKey := holderIndexKey(*previousHolder)
	indexData, exists, err := store.Get(holderBucket, indexKey)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	if exists {
		var index holderIndex
		if err := json.Unmarshal(indexData, &index); err != nil {
			return completionapp.RepositoryResult{}, fmt.Errorf("decode active lease-holder index: %w", err)
		}
		if index.LifecycleID != record.ID {
			return completionapp.RepositoryResult{}, fmt.Errorf("refusing to delete another lifecycle's lease-holder index")
		}
		mutations = append(mutations, port.RecordMutation{Bucket: holderBucket, ID: indexKey, Delete: true})
	}
	if err := store.Apply(ctx, mutations); err != nil {
		return completionapp.RepositoryResult{}, err
	}
	persisted, err := snapshot(record)
	if err != nil {
		return completionapp.RepositoryResult{}, err
	}
	return completionapp.RepositoryResult{Record: persisted, Execution: *record.Execution}, nil
}

func snapshot(record leasecontract.Record) (completioncontract.RecordSnapshot, error) {
	result := completioncontract.RecordSnapshot{ID: record.ID, Prepared: record.Execution != nil, Phase: record.Phase, IssueURL: record.IssueURL}
	if len(record.BranchPrepare) > 0 {
		var branch struct {
			BaseBranch string `json:"base_branch"`
		}
		if err := json.Unmarshal(record.BranchPrepare, &branch); err != nil {
			return completioncontract.RecordSnapshot{}, err
		}
		result.BaseBranch = branch.BaseBranch
	}
	if len(record.RemoteArtifact) > 0 {
		var artifact struct {
			Provider     string   `json:"provider"`
			Kind         string   `json:"kind"`
			URL          string   `json:"url"`
			Labels       []string `json:"labels"`
			Assignees    []string `json:"assignees"`
			VerifiedAt   string   `json:"verified_at"`
			TargetBranch string   `json:"target_branch"`
		}
		if err := json.Unmarshal(record.RemoteArtifact, &artifact); err != nil {
			return completioncontract.RecordSnapshot{}, err
		}
		result.Artifact = &completioncontract.RemoteArtifact{Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL, Labels: append([]string(nil), artifact.Labels...), Assignees: append([]string(nil), artifact.Assignees...), VerifiedAt: artifact.VerifiedAt, TargetBranch: artifact.TargetBranch}
	}
	if len(record.PhaseLedger) > 0 {
		var persisted map[string]persistedLedgerEntry
		if err := json.Unmarshal(record.PhaseLedger, &persisted); err != nil {
			return completioncontract.RecordSnapshot{}, err
		}
		result.Ledger = make(map[string]completioncontract.LedgerEntry, len(persisted))
		for phase, entry := range persisted {
			result.Ledger[phase] = completioncontract.LedgerEntry{Phase: entry.Phase, EnteredAt: entry.EnteredAt, CompletedAt: entry.CompletedAt, Artifacts: append([]string(nil), entry.Artifacts...), Missing: append([]string(nil), entry.Missing...), Notes: append([]string(nil), entry.Notes...)}
		}
	}
	if result.Ledger == nil {
		result.Ledger = map[string]completioncontract.LedgerEntry{}
	}
	if record.Execution == nil {
		return result, nil
	}
	execution := record.Execution
	result.CanonicalRoot = execution.Workspace.Root
	result.Mode = execution.Mode
	result.Lease = completionLease(execution.Lease)
	if execution.Completion != nil {
		result.Completion = &completioncontract.Completion{FinalHead: execution.Completion.FinalHead, TuringReportPath: execution.Completion.TuringReportPath, Verification: append([]string(nil), execution.Completion.Verification...), RemoteArtifactURL: execution.Completion.RemoteArtifactURL, CompletedAt: execution.Completion.CompletedAt}
	}
	if execution.Orca != nil {
		result.Orca = &completioncontract.OrcaBinding{RunID: execution.Orca.RunID, TaskID: execution.Orca.TaskID}
	}
	return result, nil
}

func applySnapshot(record *leasecontract.Record, snapshot completioncontract.RecordSnapshot) error {
	if record.Execution == nil {
		return leasecontract.ErrExecutionNotPrepared
	}
	record.Phase = snapshot.Phase
	record.Execution.Lease = leaseCompletion(snapshot.Lease)
	if snapshot.Completion == nil {
		record.Execution.Completion = nil
	} else {
		record.Execution.Completion = &leasecontract.Completion{FinalHead: snapshot.Completion.FinalHead, TuringReportPath: snapshot.Completion.TuringReportPath, Verification: append([]string(nil), snapshot.Completion.Verification...), RemoteArtifactURL: snapshot.Completion.RemoteArtifactURL, CompletedAt: snapshot.Completion.CompletedAt}
	}
	persisted := make(map[string]persistedLedgerEntry, len(snapshot.Ledger))
	for phase, entry := range snapshot.Ledger {
		persisted[phase] = persistedLedgerEntry{Phase: entry.Phase, EnteredAt: entry.EnteredAt, CompletedAt: entry.CompletedAt, Artifacts: append([]string(nil), entry.Artifacts...), Missing: append([]string(nil), entry.Missing...), Notes: append([]string(nil), entry.Notes...)}
	}
	ledger, err := json.Marshal(persisted)
	if err != nil {
		return err
	}
	record.PhaseLedger = ledger
	return nil
}

func completionLease(lease leasecontract.Lease) completioncontract.Lease {
	result := completioncontract.Lease{Generation: lease.Generation, Status: lease.Status, ClaimTokenSHA256: lease.ClaimTokenSHA256, ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt, ReplacementReason: lease.ReplacementReason}
	if lease.Holder != nil {
		result.Holder = completionActor(*lease.Holder)
	}
	return result
}

func completionActor(actor leasecontract.Actor) *completioncontract.Actor {
	result := &completioncontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &completioncontract.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func leaseCompletion(lease completioncontract.Lease) leasecontract.Lease {
	result := leasecontract.Lease{Generation: lease.Generation, Status: lease.Status, ClaimTokenSHA256: lease.ClaimTokenSHA256, ClaimedAt: lease.ClaimedAt, ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt, ReplacementReason: lease.ReplacementReason}
	if lease.Holder != nil {
		result.Holder = &leasecontract.Actor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.Process != nil {
			result.Holder.SessionProcess = &leasecontract.ProcessReceipt{PID: lease.Holder.Process.PID, StartedAt: lease.Holder.Process.StartedAt, Executable: lease.Holder.Process.Executable}
		}
	}
	return result
}

type holderIndex struct {
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

type persistedLedgerEntry struct {
	Phase       string   `json:"phase"`
	EnteredAt   string   `json:"entered_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

func holderIndexKey(actor leasecontract.Actor) string {
	identity := strings.ToLower(strings.TrimSpace(actor.Host)) + "\x00" + strings.TrimSpace(actor.SessionID) + "\x00" + strings.TrimSpace(actor.AgentID)
	sum := sha256.Sum256([]byte(identity))
	return "lease-holder-" + hex.EncodeToString(sum[:])
}

var _ completionapp.Repository = (*Repository)(nil)
