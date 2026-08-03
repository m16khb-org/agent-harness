package issueopslease

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

type ReseedRequest struct {
	ID                   string
	ExpectedGeneration   uint64
	Actor                leasedomain.Actor
	Ancestry             []leasedomain.ProcessReceipt
	CWD                  string
	InventoryFingerprint string
	Reason               string
	Confirm              bool
}

type ReseedResult struct {
	OK        bool
	ID        string
	Receipt   leasecontract.ReseedReceipt
	Execution leasecontract.Execution
}

type ReseedService struct {
	fence      ReseedFence
	repository ReseedRepository
	inventory  ReseedInventory
	artifacts  ReseedArtifacts
	clock      Clock
	inspect    ProcessInspector
	paths      CanonicalPathMatcher
}

func NewReseedService(fence ReseedFence, repository ReseedRepository, inventory ReseedInventory, artifacts ReseedArtifacts, clock Clock, inspect ProcessInspector, paths CanonicalPathMatcher) *ReseedService {
	return &ReseedService{fence: fence, repository: repository, inventory: inventory, artifacts: artifacts, clock: clock, inspect: inspect, paths: paths}
}

func (s *ReseedService) Reseed(ctx context.Context, request ReseedRequest) (ReseedResult, error) {
	if s.fence == nil || s.repository == nil || s.inventory == nil || s.artifacts == nil || s.clock == nil || s.paths == nil {
		return ReseedResult{ID: request.ID}, fmt.Errorf("reseed service dependencies are required")
	}
	var result ReseedResult
	err := s.fence.Within(ctx, request.ID, func(fenceCtx context.Context) error {
		actor, err := resolveActor(fenceCtx, request.Actor, request.Ancestry, s.inspect)
		if err != nil {
			return err
		}
		if !request.Confirm {
			return fmt.Errorf("reseed requires confirm")
		}
		snapshot, err := s.repository.LoadSnapshot(fenceCtx, request.ID)
		if err != nil {
			return err
		}
		before := snapshot.Record
		if before.Stable.Execution == nil {
			return leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
		}
		if cleanupAbandonApplying(before.Stable.CleanupAbandonFailure) {
			return fmt.Errorf("execution reseed is blocked by cleanup abandon apply")
		}
		if err := leasedomain.ValidateReseedGeneration(toDomainLease(before.Lease), request.ExpectedGeneration); err != nil {
			return err
		}
		if before.Stable.Execution.Pending != nil {
			return fmt.Errorf("execution replacement is blocked by a pending external intent; run execution reconcile")
		}
		canonicalCWD := s.paths.Matches(request.CWD, before.SourceRoot) || s.paths.Matches(request.CWD, before.CanonicalRoot)
		if err := leasedomain.ValidateReseed(toDomainLease(before.Lease), leasedomain.ReseedRequest{ExpectedGeneration: request.ExpectedGeneration, CanonicalCWD: canonicalCWD, Reason: request.Reason}); err != nil {
			return err
		}
		observed, err := s.inventory.Observe(fenceCtx, before.Stable, actor)
		if err != nil {
			return err
		}
		if observed.Fingerprint != request.InventoryFingerprint {
			return fmt.Errorf("stale replacement inventory fingerprint")
		}
		stable, err := cloneReseedRecord(before.Stable)
		if err != nil {
			return leasecontract.Fail(leasecontract.FailurePersistence, err)
		}
		next := before
		next.Stable = stable
		if next.Stable.Execution.Orca != nil && strings.TrimSpace(observed.RuntimeID) != "" {
			next.Stable.Execution.Orca.RuntimeID = strings.TrimSpace(observed.RuntimeID)
		}
		outcome := leasedomain.ApplyReseed(s.clock.Now(), toDomainLease(before.Lease), leasedomain.ReseedRequest{ExpectedGeneration: request.ExpectedGeneration, Reason: request.Reason})
		next.Stable.Execution.Lease.Generation = outcome.Generation
		next.Stable.Execution.Lease.Status = outcome.Status
		next.Stable.Execution.Lease.Holder = toContractReseedActor(outcome.Holder)
		next.Stable.Execution.Lease.ClaimTokenSHA256 = outcome.ClaimTokenSHA256
		next.Stable.Execution.Lease.ReplacedAt = outcome.ReplacedAt
		next.Stable.Execution.Lease.ReplacementReason = outcome.ReplacementReason
		prepared, err := s.artifacts.Prepare(fenceCtx, next.Stable)
		if err != nil {
			return err
		}
		next.Stable.Execution.Lease.ClaimTokenSHA256 = prepared.TokenSHA256
		if next.Stable.Execution.Orca != nil {
			next.Stable.Execution.Orca.ArtifactIdentityVersion = leasecontract.OrcaArtifactIdentityVersion
			next.Stable.Execution.Orca.IssueBodySHA256 = prepared.Receipt.IssueBodySHA256
			next.Stable.Execution.Orca.ContextPacketSHA256 = prepared.Receipt.ContextPacketSHA256
			next.Stable.Execution.Orca.OwnerPromptSHA256 = prepared.Receipt.OwnerPromptSHA256
		}
		next.Lease = next.Stable.Execution.Lease
		after, err := s.repository.CommitReseed(fenceCtx, snapshot, next)
		if err != nil {
			if rollbackErr := s.artifacts.Rollback(fenceCtx, prepared); rollbackErr != nil {
				return fmt.Errorf("%w; replacement residue cleanup failed: %v", err, rollbackErr)
			}
			return err
		}
		_ = s.artifacts.CleanupSuperseded(fenceCtx, before.Stable)
		prepared.Receipt.Execution = after.Execution
		result = ReseedResult{OK: true, ID: request.ID, Receipt: prepared.Receipt, Execution: after.Execution}
		return nil
	})
	if err != nil {
		return ReseedResult{ID: request.ID}, err
	}
	return result, nil
}

func cleanupAbandonApplying(raw json.RawMessage) bool {
	var receipt struct {
		Step string `json:"step"`
	}
	return len(raw) > 0 && json.Unmarshal(raw, &receipt) == nil && receipt.Step == "applying"
}

func toContractReseedActor(actor *leasedomain.Actor) *leasecontract.Actor {
	if actor == nil {
		return nil
	}
	result := &leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}

func cloneReseedRecord(record leasecontract.Record) (leasecontract.Record, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return leasecontract.Record{}, err
	}
	var clone leasecontract.Record
	if err := json.Unmarshal(data, &clone); err != nil {
		return leasecontract.Record{}, err
	}
	return clone, nil
}
