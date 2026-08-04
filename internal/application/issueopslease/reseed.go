package issueopslease

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

type ReseedRequest struct {
	ID                   string
	ExpectedGeneration   uint64
	CompletionGeneration uint64
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
	baseSync   basesyncport.Inspector
	artifacts  ReseedArtifacts
	clock      Clock
	inspect    ProcessInspector
	paths      CanonicalPathMatcher
}

func NewReseedService(fence ReseedFence, repository ReseedRepository, inventory ReseedInventory, baseSync basesyncport.Inspector, artifacts ReseedArtifacts, clock Clock, inspect ProcessInspector, paths CanonicalPathMatcher) *ReseedService {
	return &ReseedService{fence: fence, repository: repository, inventory: inventory, baseSync: baseSync, artifacts: artifacts, clock: clock, inspect: inspect, paths: paths}
}

func (s *ReseedService) Reseed(ctx context.Context, request ReseedRequest) (ReseedResult, error) {
	if s.fence == nil || s.repository == nil || s.inventory == nil || s.baseSync == nil || s.artifacts == nil || s.clock == nil || s.paths == nil {
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
		completionGeneration, err := resolveCompletionGeneration(before.Stable.Execution, request.CompletionGeneration)
		if err != nil {
			return err
		}
		if err := s.observeCompletedBase(fenceCtx, before.Stable, completionGeneration); err != nil {
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
		reseededAt := s.clock.Now()
		outcome := leasedomain.ApplyReseed(reseededAt, toDomainLease(before.Lease), leasedomain.ReseedRequest{ExpectedGeneration: request.ExpectedGeneration, Reason: request.Reason})
		next.Stable.Execution.Lease.Generation = outcome.Generation
		next.Stable.Execution.Lease.Status = outcome.Status
		next.Stable.Execution.Lease.Holder = toContractReseedActor(outcome.Holder)
		next.Stable.Execution.Lease.ClaimTokenSHA256 = outcome.ClaimTokenSHA256
		next.Stable.Execution.Lease.ReplacedAt = outcome.ReplacedAt
		next.Stable.Execution.Lease.ReplacementReason = outcome.ReplacementReason
		if err := reopenCompletedExecution(&next.Stable, completionGeneration, before.Lease.Generation, outcome.Generation, outcome.ReplacementReason, outcome.ReplacedAt); err != nil {
			return leasecontract.Fail(leasecontract.FailurePersistence, err)
		}
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

func (s *ReseedService) observeCompletedBase(ctx context.Context, record leasecontract.Record, completionGeneration uint64) error {
	if record.Execution == nil || record.Execution.Completion == nil ||
		(record.Execution.Lease.Status != "released" && record.Execution.Lease.Status != "claimable") {
		return nil
	}
	var branchPrepare struct {
		BaseBranch string `json:"base_branch"`
	}
	if len(record.BranchPrepare) == 0 {
		return fmt.Errorf("completed reseed requires branch_prepare.base_branch")
	}
	if err := json.Unmarshal(record.BranchPrepare, &branchPrepare); err != nil {
		return fmt.Errorf("decode branch_prepare for completed reseed: %w", err)
	}
	if strings.TrimSpace(branchPrepare.BaseBranch) == "" {
		return fmt.Errorf("completed reseed requires branch_prepare.base_branch")
	}
	receipt, err := s.baseSync.Observe(ctx, basesyncport.Request{
		Worktree: record.Execution.Workspace.Root, BaseBranch: branchPrepare.BaseBranch,
	})
	if err != nil {
		return fmt.Errorf("observe completed execution base: %w", err)
	}
	if receipt.SyncRequired {
		return issueopscontract.NewBaseSyncRequiredError(record.ID, completionGeneration)
	}
	return nil
}

func resolveCompletionGeneration(execution *leasecontract.Execution, selected uint64) (uint64, error) {
	if execution == nil || execution.Completion == nil {
		if selected != 0 {
			return 0, fmt.Errorf("completion_generation requires a current completion")
		}
		return 0, nil
	}
	stamped := execution.Completion.Generation
	if stamped == 0 {
		return 0, fmt.Errorf("invalid or missing stamped completion generation")
	}
	if selected != 0 && selected != stamped {
		return 0, fmt.Errorf("completion_generation conflicts with stamped completion generation %d", stamped)
	}
	return stamped, nil
}

func reopenCompletedExecution(record *leasecontract.Record, completionGeneration, previousGeneration, nextGeneration uint64, reason, reopenedAt string) error {
	if record.Execution == nil || record.Execution.Completion == nil {
		return nil
	}
	completion := *record.Execution.Completion
	completion.Verification = append([]string(nil), record.Execution.Completion.Verification...)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "completed execution reseed"
	}
	record.Execution.CompletionHistory = append(record.Execution.CompletionHistory, leasecontract.CompletionHistoryEntry{
		Generation: completionGeneration,
		Completion: completion,
		Reason:     reason,
		ReopenedAt: reopenedAt,
	})
	record.Execution.Completion = nil
	record.Phase = "implement"
	record.AISlopCleanAt = ""
	record.AISlopCleanHead = ""
	record.AISlopCleanFingerprint = ""
	record.AISlopCleanCategories = nil
	record.AISlopCleanVerification = nil
	record.ImplementationReview = nil
	record.RemoteCompletion = nil
	return staleCompletedReseedLedger(record, previousGeneration, nextGeneration)
}

func staleCompletedReseedLedger(record *leasecontract.Record, previousGeneration, nextGeneration uint64) error {
	ledger := issueopscontract.IssueOpsPhaseLedger{}
	if len(record.PhaseLedger) > 0 {
		if err := json.Unmarshal(record.PhaseLedger, &ledger); err != nil {
			return fmt.Errorf("decode phase ledger: %w", err)
		}
	}
	note := fmt.Sprintf("stale: completed execution reseed (%d -> %d)", previousGeneration, nextGeneration)
	for _, phase := range []issueopscontract.IssueOpsPhase{
		issueopscontract.IssueOpsPhaseImplement,
		issueopscontract.IssueOpsPhaseAISlopClean,
		issueopscontract.IssueOpsPhaseFeedback,
		issueopscontract.IssueOpsPhasePR,
		issueopscontract.IssueOpsPhaseDone,
	} {
		entry := ledger[phase]
		entry.Phase = phase
		entry.CompletedAt = ""
		if !containsLedgerNote(entry.Notes, note) {
			entry.Notes = append(entry.Notes, note)
		}
		ledger[phase] = entry
	}
	encoded, err := json.Marshal(ledger)
	if err != nil {
		return fmt.Errorf("encode phase ledger: %w", err)
	}
	record.PhaseLedger = encoded
	return nil
}

func containsLedgerNote(notes []string, want string) bool {
	for _, note := range notes {
		if note == want {
			return true
		}
	}
	return false
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
