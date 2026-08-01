package issueopspreparation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	preparationapp "agent-harness/internal/application/issueopspreparation"
	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

const (
	recordBucket = "issueops_v1"
	holderBucket = "lease_holder_v1"
)

type MutationGate func(context.Context) error

type SQLiteRepository struct {
	store port.RecordInventoryStore
	gate  MutationGate
}

func NewSQLiteRepository(store port.RecordInventoryStore, gate MutationGate) *SQLiteRepository {
	return &SQLiteRepository{store: store, gate: gate}
}

func (repository *SQLiteRepository) RequireMutationAllowed(ctx context.Context) error {
	if repository.gate == nil {
		return fmt.Errorf("IssueOps mutation gate is unavailable")
	}
	return repository.gate(ctx)
}

func (repository *SQLiteRepository) Load(_ context.Context, id string) (preparationcontract.Snapshot, error) {
	if repository.store == nil {
		return preparationcontract.Snapshot{}, fmt.Errorf("preparation record store is unavailable")
	}
	data, ok, err := repository.store.Get(recordBucket, id)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	if !ok {
		return preparationcontract.Snapshot{}, fmt.Errorf("issueops record %s not found", id)
	}
	record, err := leasecontract.Decode(id, data)
	if err != nil {
		return preparationcontract.Snapshot{}, err
	}
	return preparationcontract.Snapshot{
		Record: record, RecordRaw: append([]byte(nil), data...), ClaimTokenPath: currentClaimTokenPath(record),
	}, nil
}

func (repository *SQLiteRepository) EnsureRootUnclaimed(_ context.Context, selfID, root string) error {
	if repository.store == nil {
		return fmt.Errorf("preparation record store is unavailable")
	}
	return ensureRootUnclaimed(repository.store, selfID, root)
}

func (repository *SQLiteRepository) CommitDirect(ctx context.Context, commit preparationapp.DirectCommit) (preparationcontract.Result, error) {
	if repository.store == nil {
		return preparationcontract.Result{ID: commit.Command.ID}, fmt.Errorf("preparation record store is unavailable")
	}
	var result preparationcontract.Result
	err := repository.store.WithSpan(ctx, func(spanCtx context.Context) error {
		current, err := repository.Load(spanCtx, commit.Command.ID)
		if err != nil {
			return err
		}
		if current.Record.Execution != nil {
			return fmt.Errorf("IssueOps execution is already prepared")
		}
		if err := ensureRootUnclaimed(repository.store, current.Record.ID, commit.Workspace.Root); err != nil {
			return err
		}
		record := current.Record
		record.WorktreePath = commit.Workspace.Root
		actor := commit.Command.Clone().Actor
		record.Execution = &leasecontract.Execution{
			Mode: preparationcontract.ModeDirect,
			Workspace: leasecontract.Workspace{
				SourceRoot: commit.Workspace.SourceRoot, Root: commit.Workspace.Root,
				Branch: commit.Workspace.Branch, BaseHead: commit.Workspace.BaseHead,
				ParentWorktree: commit.Workspace.ParentWorktree, Driver: "git", LinkedAt: commit.LinkedAt,
			},
			Lease: leasecontract.Lease{
				Generation: 1, Status: "active", Holder: &actor, ClaimedAt: commit.ClaimedAt,
			},
		}
		data, err := leasecontract.Encode(record)
		if err != nil {
			return err
		}
		indexData, err := json.Marshal(holderIndex{
			SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID,
			Generation: 1, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		})
		if err != nil {
			return err
		}
		if err := repository.store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: recordBucket, ID: record.ID, Data: data},
			{Bucket: holderBucket, ID: holderIndexKey(actor), Data: indexData, RequireAbsent: true},
		}); err != nil {
			return err
		}
		result = preparationcontract.Result{
			OK: true, ID: record.ID, RequestedMode: commit.RequestedMode,
			ResolvedMode: preparationcontract.ModeDirect, FallbackCode: commit.FallbackCode,
			Workspace: record.Execution.Workspace, Execution: record.Execution,
		}.Clone()
		return nil
	})
	if err != nil {
		return preparationcontract.Result{ID: commit.Command.ID}, err
	}
	return result, nil
}

func (*SQLiteRepository) BeginIntent(context.Context, preparationapp.OrcaBegin) (preparationapp.IntentState, error) {
	return preparationapp.IntentState{}, fmt.Errorf("Orca preparation intent repository is unavailable")
}

func (*SQLiteRepository) MarkInvoking(context.Context, preparationapp.IntentState) (preparationapp.IntentState, error) {
	return preparationapp.IntentState{}, fmt.Errorf("Orca preparation intent repository is unavailable")
}

func (*SQLiteRepository) RecordFailure(context.Context, preparationapp.IntentState, string, error) error {
	return fmt.Errorf("Orca preparation intent repository is unavailable")
}

func (*SQLiteRepository) ApplyReceipt(context.Context, preparationapp.IntentState, preparationcontract.IntentReceipt) (preparationapp.IntentProgress, error) {
	return preparationapp.IntentProgress{}, fmt.Errorf("Orca preparation intent repository is unavailable")
}

func ensureRootUnclaimed(store port.RecordInventoryStore, selfID, root string) error {
	target := cleanAbsPath(root)
	if target == "" {
		return fmt.Errorf("canonical worktree root is required")
	}
	rows, err := store.GetAll(recordBucket)
	if err != nil {
		return err
	}
	self := strings.TrimSpace(selfID)
	for _, row := range rows {
		if row.ID == self {
			continue
		}
		record, err := leasecontract.Decode(row.ID, row.Data)
		if err != nil {
			return fmt.Errorf("canonical worktree 소유권 스캔이 lifecycle %s 레코드를 읽지 못했다; 손상 레코드를 먼저 해소하라: %w", row.ID, err)
		}
		claims := []string{record.WorktreePath}
		if record.Execution != nil {
			claims = append(claims, record.Execution.Workspace.Root)
		}
		for _, claimed := range claims {
			if cleanAbsPath(claimed) == "" || cleanAbsPath(claimed) != target {
				continue
			}
			return fmt.Errorf(
				"canonical worktree %s는 이미 lifecycle %s(브랜치 %s)가 선점했다; 먼저 그 사이클을 정리하라: agent-harness issueops cleanup finish --id %s --preview --json",
				target, row.ID, strings.TrimSpace(record.Branch), row.ID,
			)
		}
	}
	return nil
}

func cleanAbsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
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

func currentClaimTokenPath(record leasecontract.Record) string {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return ""
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(record.ID)))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}
