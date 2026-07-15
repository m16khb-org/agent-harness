package issueops

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/port"
)

type IssueOpsHandoffStartRequest struct {
	ID                    string                 `json:"id"`
	CoordinatorRecipient  string                 `json:"coordinator_recipient,omitempty"`
	Confirm               bool                   `json:"confirm,omitempty"`
	ExpectedContextSHA256 string                 `json:"expected_context_sha256,omitempty"`
	Context               handoff.ContextOptions `json:"context,omitempty"`
	CoordinatorHost       string                 `json:"coordinator_host,omitempty"`
	CoordinatorSessionID  string                 `json:"coordinator_session_id,omitempty"`
	CoordinatorAgentID    string                 `json:"coordinator_agent_id,omitempty"`
	SourceCWD             string                 `json:"source_cwd,omitempty"`
}

type IssueOpsHandoffStartResult struct {
	OK                           bool                  `json:"ok"`
	ID                           string                `json:"id"`
	Preview                      bool                  `json:"preview,omitempty"`
	State                        string                `json:"state"`
	Disposition                  string                `json:"disposition,omitempty"`
	Attempt                      int                   `json:"attempt"`
	ContextSHA256                string                `json:"context_sha256,omitempty"`
	PlanSHA256                   string                `json:"plan_sha256,omitempty"`
	RecoveryCode                 string                `json:"recovery_code,omitempty"`
	CodexHookTrustBypassRequired bool                  `json:"codex_hook_trust_bypass_required"`
	CodexHookTrustBypassAttested bool                  `json:"codex_hook_trust_bypass_attested"`
	Orca                         *IssueOpsOrcaIdentity `json:"orca,omitempty"`
	CoordinatorRecipient         string                `json:"coordinator_recipient,omitempty"`
}

var concreteOrcaTerminalHandlePattern = regexp.MustCompile(`^term_[A-Za-z0-9_-]+$`)

type IssueOpsHandoffStartClock struct {
	Now func() time.Time
}

type issueOpsHandoffStartHooks struct {
	BeforeStage   func(string)
	BeforeJournal func(string)
}

type IssueOpsOrcaDispatchClient interface {
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
	CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error)
	RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error)
	ListTasks(context.Context) ([]port.OrcaTask, error)
	ListDispatchedTasks(context.Context) ([]port.OrcaTask, error)
	CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
	Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
	ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error)
}

func IssueOpsPreDispatchReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := make([]string, 0, len(ready.Missing))
	for _, item := range ready.Missing {
		if item != "handoff_worker_claim" {
			missing = append(missing, item)
		}
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
		missing = append(missing, "handoff_coordinator_preparing")
	}
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil || strings.TrimSpace(record.ExecutionHandoff.Orca.WorktreeID) == "" {
		missing = append(missing, "handoff_orca_worktree")
	}
	ready.Missing = uniqueStrings(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func StartIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffStartRequest, client IssueOpsOrcaDispatchClient, clock IssueOpsHandoffStartClock) (IssueOpsHandoffStartResult, error) {
	return startIssueOpsHandoff(ctx, stateRoot, req, client, clock, issueOpsHandoffStartHooks{})
}

func startIssueOpsHandoff(ctx context.Context, stateRoot string, req IssueOpsHandoffStartRequest, client IssueOpsOrcaDispatchClient, clock IssueOpsHandoffStartClock, hooks issueOpsHandoffStartHooks) (IssueOpsHandoffStartResult, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	if record.ExecutionHandoff == nil {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("execution handoff is required")
	}
	if record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
		return projectHandoffStart(record, false, ""), nil
	}
	if record.ExecutionHandoff.PendingOperation != nil {
		fence := handoffFence(record)
		now := issueOpsHandoffStartNow(clock)
		if err := markHandoffPrepareRecovery(stateRoot, record.ID, fence, "pending_operation_requires_recovery", "start observed an unresolved external mutation", now); err != nil {
			return IssueOpsHandoffStartResult{}, err
		}
		record, err = ReadIssueOps(stateRoot, record.ID)
		return projectHandoffStart(record, false, ""), err
	}
	readiness := IssueOpsPreDispatchReadiness(record)
	if !readiness.Ready {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("handoff pre-dispatch readiness missing: %s", strings.Join(readiness.Missing, ", "))
	}
	coordinatorSession := model.IssueOpsHostSessionIdentity{Host: strings.TrimSpace(req.CoordinatorHost), SessionID: strings.TrimSpace(req.CoordinatorSessionID), AgentID: strings.TrimSpace(req.CoordinatorAgentID)}
	coordinatorRecipient, err := resolveHandoffCoordinatorRecipient(ctx, stateRoot, record, req.CoordinatorRecipient, coordinatorSession, client)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	identityRecord := record
	identityHandoff := *record.ExecutionHandoff
	identityHandoff.CoordinatorSession = &coordinatorSession
	identityRecord.ExecutionHandoff = &identityHandoff
	if !handoff.CoordinatorIdentityMatches(identityRecord, coordinatorSession, req.SourceCWD) {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("handoff start requires authenticated coordinator native session from the exact source checkout")
	}
	contextOptions, err := resolveHandoffContextOptions(record, req.Context)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	contextRecord := record
	contextHandoff := *record.ExecutionHandoff
	contextHandoff.CoordinatorMailboxHandle = coordinatorRecipient
	contextHandoff.CoordinatorSession = &coordinatorSession
	contextRecord.ExecutionHandoff = &contextHandoff
	packet, err := handoff.BuildContext(contextRecord, contextOptions)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	if !req.Confirm {
		result := projectHandoffStart(record, true, packet.PlanSHA256)
		result.ContextSHA256 = packet.SHA256
		result.CoordinatorRecipient = coordinatorRecipient
		result.CodexHookTrustBypassAttested = contextOptions.AllowCodexHookTrustBypass
		return result, nil
	}
	if codexHookTrustBypassRequired(record) && !contextOptions.AllowCodexHookTrustBypass {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("confirmed supervised Codex start requires --allow-codex-hook-trust-bypass after the documented hooks/list attestation")
	}
	if client == nil {
		return IssueOpsHandoffStartResult{}, fmt.Errorf("Orca dispatch dependency is unavailable")
	}
	if err := validateExpectedContextSHA256(req.ExpectedContextSHA256); err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	nextNow := func() string { return issueOpsHandoffStartNow(clock) }
	record, err = persistHandoffContext(stateRoot, record.ID, coordinatorRecipient, coordinatorSession, handoff.CanonicalContextOptions(contextOptions), req.ExpectedContextSHA256, nextNow())
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	fence := handoffFence(record)

	runHandoffStartHook(hooks.BeforeStage, handoff.OperationTerminalCreate)
	record, err = validateHandoffStageCheckpoint(stateRoot, record)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	record, liveHandle, err := ensureHandoffTerminal(ctx, stateRoot, record, fence, client, nextNow, func() {
		runHandoffStartHook(hooks.BeforeJournal, handoff.OperationTerminalCreate)
	})
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	runHandoffStartHook(hooks.BeforeStage, handoff.OperationTaskCreate)
	record, err = validateHandoffStageCheckpoint(stateRoot, record)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	record, err = ensureHandoffTask(ctx, stateRoot, record, fence, client, packet.Markdown, nextNow, func() {
		runHandoffStartHook(hooks.BeforeJournal, handoff.OperationTaskCreate)
	})
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	runHandoffStartHook(hooks.BeforeStage, handoff.OperationDispatch)
	record, err = validateHandoffStageCheckpoint(stateRoot, record)
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	record, err = dispatchHandoff(ctx, stateRoot, record, fence, client, liveHandle, nextNow, func() {
		runHandoffStartHook(hooks.BeforeJournal, handoff.OperationDispatch)
	})
	if err != nil {
		return IssueOpsHandoffStartResult{}, err
	}
	return projectHandoffStart(record, false, packet.PlanSHA256), nil
}

func ensureHandoffTerminal(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, now func() string, beforeJournal func()) (IssueOpsRecord, string, error) {
	workerPTYID := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerPTYID)
	workerHandle := strings.TrimSpace(record.ExecutionHandoff.Orca.WorkerTerminalHandle)
	if (workerPTYID == "") != (workerHandle == "") {
		return record, "", fmt.Errorf("persisted Orca terminal checkpoint is incomplete")
	}
	if workerPTYID == "" {
		terminals, err := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
		if err != nil {
			return record, "", persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("list terminals before baseline adoption requires recovery: %w", err), now)
		}
		var baseline *port.OrcaTerminal
		for i := range terminals {
			terminal := &terminals[i]
			if terminal.WorktreeID == record.ExecutionHandoff.Orca.WorktreeID && terminalWorktreePathMatches(*terminal, record.ExecutionHandoff.WorkerRoot) && terminal.Connected && terminal.Writable {
				if baseline != nil {
					return createHandoffTerminal(ctx, stateRoot, record, fence, client, now, beforeJournal)
				}
				baseline = terminal
			}
		}
		if baseline != nil {
			if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, baseline.Handle, now); err != nil {
				return record, "", err
			}
			record, err = persistHandoffAdoptedTerminal(stateRoot, record, *baseline, now())
			if err != nil {
				return record, "", err
			}
			return record, baseline.Handle, nil
		}
		return createHandoffTerminal(ctx, stateRoot, record, fence, client, now, beforeJournal)
	}
	terminal, err := client.RefreshTerminal(ctx, record.ExecutionHandoff.Orca.WorktreeID, workerPTYID)
	if err != nil {
		if isOrcaTerminalNotFound(err) {
			return recoverRuntimeReissuedHandoffTerminal(ctx, stateRoot, record, fence, client, now)
		}
		return record, "", fmt.Errorf("refresh persisted Orca terminal: %w", err)
	}
	if strings.TrimSpace(terminal.RuntimeID) != "" && terminal.RuntimeID != record.ExecutionHandoff.Orca.RuntimeID {
		return recoverRuntimeReissuedHandoffTerminal(ctx, stateRoot, record, fence, client, now)
	}
	if terminal.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || terminal.PTYID != workerPTYID || strings.TrimSpace(terminal.Handle) == "" || !terminal.Connected || !terminal.Writable || !terminalWorktreePathMatches(terminal, record.ExecutionHandoff.WorkerRoot) {
		return record, "", fmt.Errorf("refreshed Orca terminal identity does not match the persisted checkpoint")
	}
	if strings.TrimSpace(terminal.Handle) != workerHandle || terminal.TabID != record.ExecutionHandoff.Orca.WorkerTabID || terminal.LeafID != record.ExecutionHandoff.Orca.WorkerLeafID {
		record, err = persistHandoffLiveTerminalIdentity(stateRoot, record, terminal, now())
		if err != nil {
			return record, "", err
		}
	}
	return record, strings.TrimSpace(terminal.Handle), nil
}

func persistHandoffAdoptedTerminal(stateRoot string, expected IssueOpsRecord, terminal port.OrcaTerminal, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) || current.ExecutionHandoff == nil || current.ExecutionHandoff.Orca == nil {
			return fmt.Errorf("handoff changed before baseline terminal adoption")
		}
		if terminal.WorktreeID != current.ExecutionHandoff.Orca.WorktreeID || strings.TrimSpace(terminal.PTYID) == "" || strings.TrimSpace(terminal.Handle) == "" || !terminal.Connected || !terminal.Writable || !terminalWorktreePathMatches(terminal, current.ExecutionHandoff.WorkerRoot) {
			return fmt.Errorf("baseline terminal does not match prepared worktree authority")
		}
		identity := *current.ExecutionHandoff.Orca
		identity.WorkerPTYID = terminal.PTYID
		identity.WorkerTerminalHandle = terminal.Handle
		identity.WorkerTabID = terminal.TabID
		identity.WorkerLeafID = terminal.LeafID
		current.ExecutionHandoff.Orca = &identity
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, current)
		return err
	})
	return persisted, err
}

func persistHandoffLiveTerminalIdentity(stateRoot string, expected IssueOpsRecord, terminal port.OrcaTerminal, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) || current.ExecutionHandoff == nil || current.ExecutionHandoff.Orca == nil {
			return fmt.Errorf("handoff changed before live terminal identity refresh")
		}
		identity := *current.ExecutionHandoff.Orca
		if terminal.PTYID != identity.WorkerPTYID || terminal.WorktreeID != identity.WorktreeID || !terminalWorktreePathMatches(terminal, current.ExecutionHandoff.WorkerRoot) {
			return fmt.Errorf("live terminal refresh does not match persisted PTY and worktree authority")
		}
		identity.WorkerTerminalHandle = strings.TrimSpace(terminal.Handle)
		identity.WorkerTabID = strings.TrimSpace(terminal.TabID)
		identity.WorkerLeafID = strings.TrimSpace(terminal.LeafID)
		current.ExecutionHandoff.Orca = &identity
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, current)
		return err
	})
	return persisted, err
}

func recoverRuntimeReissuedHandoffTerminal(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, now func() string) (IssueOpsRecord, string, error) {
	startedAt := now()
	journaled, err := beginHandoffOperation(stateRoot, record, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationRuntimeRefresh, StartedAt: startedAt,
	})
	if err != nil {
		return record, "", err
	}
	worktree, terminal, err := reconcileRuntimeReissuedHandoffIdentity(ctx, journaled, client)
	if err != nil {
		transitionAt := now()
		if markErr := markHandoffPrepareRecovery(stateRoot, journaled.ID, fence, "runtime_restart_ambiguous", err.Error(), transitionAt); markErr != nil {
			return journaled, "", fmt.Errorf("runtime restart reconciliation failed: %v; persist recovery: %w", err, markErr)
		}
		return journaled, "", fmt.Errorf("runtime restart requires explicit recovery: %w", err)
	}
	updated, err := completeRuntimeRefreshOperation(stateRoot, journaled, fence, worktree, terminal, now())
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, journaled.ID, fence, "runtime_restart_persist_failed", err.Error(), now())
		return journaled, "", err
	}
	return updated, strings.TrimSpace(terminal.Handle), nil
}

func completeRuntimeRefreshOperation(stateRoot string, expected IssueOpsRecord, fence handoff.Fence, worktree port.OrcaWorktree, terminal port.OrcaTerminal, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("handoff changed during runtime-refresh inventory")
		}
		if err := validateHandoffContextSource(current); err != nil {
			return err
		}
		if err := validateHandoffCleanExactCheckpoint(current); err != nil {
			return err
		}
		current, err = handoff.CompleteOperation(current, fence, handoff.OperationRuntimeRefresh, now)
		if err != nil {
			return err
		}
		if current.ExecutionHandoff == nil || current.ExecutionHandoff.Orca == nil {
			return fmt.Errorf("runtime-refresh identity checkpoint is unavailable")
		}
		identity := *current.ExecutionHandoff.Orca
		identity.RuntimeID = worktree.RuntimeID
		identity.WorktreeInstanceID = worktree.InstanceID
		identity.ProviderIssueLinkStatus = providerIssueLinkStatus(current, worktree)
		identity.WorkerPTYID = terminal.PTYID
		identity.WorkerTerminalHandle = terminal.Handle
		identity.WorkerTabID = terminal.TabID
		identity.WorkerLeafID = terminal.LeafID
		current.ExecutionHandoff.Orca = &identity
		current.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, current)
		return err
	})
	return persisted, err
}

func reconcileRuntimeReissuedHandoffIdentity(ctx context.Context, record IssueOpsRecord, client IssueOpsOrcaDispatchClient) (port.OrcaWorktree, port.OrcaTerminal, error) {
	worktrees, err := client.ListWorktrees(ctx, record.Repo)
	if err != nil {
		return port.OrcaWorktree{}, port.OrcaTerminal{}, fmt.Errorf("list current-runtime worktrees: %w", err)
	}
	worktree, err := reconcileRuntimeReissuedHandoffWorktree(record, worktrees)
	if err != nil {
		return port.OrcaWorktree{}, port.OrcaTerminal{}, err
	}
	terminals, err := client.ListTerminals(ctx, worktree.ID)
	if err != nil {
		return port.OrcaWorktree{}, port.OrcaTerminal{}, fmt.Errorf("list current-runtime terminals: %w", err)
	}
	terminal, err := reconcileRuntimeReissuedHandoffTerminal(record, worktree, terminals)
	if err != nil {
		return port.OrcaWorktree{}, port.OrcaTerminal{}, err
	}
	return worktree, terminal, nil
}

func reconcileRuntimeReissuedHandoffWorktree(record IssueOpsRecord, rows []port.OrcaWorktree) (port.OrcaWorktree, error) {
	if len(rows) > handoff.MaxBaselineIDs {
		return port.OrcaWorktree{}, fmt.Errorf("runtime restart worktree inventory exceeds %d entries", handoff.MaxBaselineIDs)
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	if err := requireStableInventoryIdentities("worktree", ids); err != nil {
		return port.OrcaWorktree{}, err
	}
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		return port.OrcaWorktree{}, fmt.Errorf("runtime restart worktree identity is unavailable")
	}
	candidates := make([]port.OrcaWorktree, 0, 1)
	for _, row := range rows {
		if row.ID == h.Orca.WorktreeID {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaWorktree{}, fmt.Errorf("runtime restart requires exactly one persisted worktree row; found %d", len(candidates))
	}
	row := candidates[0]
	marker := issueOpsHandoffMarker(record.ID, h.OwnershipEpoch, h.Attempt)
	baseRefMatches := row.BaseRef == h.Orca.BaseRef || h.Orca.WorktreeAdopted && strings.TrimSpace(row.BaseRef) == ""
	if strings.TrimSpace(row.RuntimeID) == "" || row.RuntimeID == h.Orca.RuntimeID || strings.TrimSpace(row.InstanceID) == "" || row.RepoID != h.Orca.RepoID || !baseRefMatches || filepath.Clean(strings.TrimSpace(row.Path)) != filepath.Clean(h.WorkerRoot) || strings.TrimPrefix(strings.TrimSpace(row.Branch), "refs/heads/") != record.Branch || row.Head != h.AttemptBaseHead || row.Comment != marker {
		return port.OrcaWorktree{}, fmt.Errorf("current-runtime worktree does not match exact repo/base/path/branch/head/comment identity")
	}
	if err := validateHandoffWorktreeIssueMetadata(record, row); err != nil {
		return port.OrcaWorktree{}, fmt.Errorf("current-runtime worktree provider issue metadata is invalid: %w", err)
	}
	return row, nil
}

func reconcileRuntimeReissuedHandoffTerminal(record IssueOpsRecord, worktree port.OrcaWorktree, rows []port.OrcaTerminal) (port.OrcaTerminal, error) {
	if len(rows) > handoff.MaxBaselineIDs {
		return port.OrcaTerminal{}, fmt.Errorf("runtime restart terminal inventory exceeds %d entries", handoff.MaxBaselineIDs)
	}
	ptys := make([]string, len(rows))
	handles := make([]string, len(rows))
	for i := range rows {
		ptys[i], handles[i] = rows[i].PTYID, rows[i].Handle
	}
	if err := requireStableInventoryIdentities("terminal", ptys); err != nil {
		return port.OrcaTerminal{}, err
	}
	if err := requireStableInventoryIdentities("terminal", handles); err != nil {
		return port.OrcaTerminal{}, err
	}
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil {
		return port.OrcaTerminal{}, fmt.Errorf("runtime restart terminal identity is unavailable")
	}
	stableObserved := h.Orca.WorkerTabID != "" || h.Orca.WorkerLeafID != ""
	marker := issueOpsHandoffMarker(record.ID, h.OwnershipEpoch, h.Attempt)
	candidates := make([]port.OrcaTerminal, 0, 1)
	for _, row := range rows {
		if stableObserved {
			if row.TabID == h.Orca.WorkerTabID && row.LeafID == h.Orca.WorkerLeafID {
				candidates = append(candidates, row)
			}
		} else if row.StableTabTitle == marker {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTerminal{}, fmt.Errorf("runtime restart requires exactly one stable terminal candidate; found %d", len(candidates))
	}
	row := candidates[0]
	if (row.TabID == "") != (row.LeafID == "") || row.RuntimeID != worktree.RuntimeID || row.WorktreeID != worktree.ID || !terminalWorktreePathMatches(row, h.WorkerRoot) || !row.Connected || !row.Writable {
		return port.OrcaTerminal{}, fmt.Errorf("current-runtime terminal does not match exact stable tab/leaf and worktree identity")
	}
	return row, nil
}

func isOrcaTerminalNotFound(err error) bool {
	var orcaErr *port.OrcaError
	return errors.As(err, &orcaErr) && strings.TrimSpace(orcaErr.Code) == "terminal_not_found"
}

func terminalCreateCapabilityLost(err error) bool {
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) {
		return false
	}
	switch strings.TrimSpace(orcaErr.Code) {
	case "terminal_create_capability_missing", "terminal_create_capability_unavailable":
		return true
	default:
		return false
	}
}

func createHandoffTerminal(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, now func() string, beforeJournal func()) (IssueOpsRecord, string, error) {
	terminals, err := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
	if err != nil {
		return record, "", persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("list terminals before create requires recovery: %w", err), now)
	}
	baseline, err := handoff.CanonicalBaselineIDs("terminal", terminalPTYIDs(terminals))
	if err != nil {
		return record, "", persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("Orca terminal baseline requires recovery: %w", err), now)
	}
	if err := handoff.RequireBaselineDeltaHeadroom("terminal", baseline); err != nil {
		return record, "", persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("Orca terminal baseline requires recovery: %w", err), now)
	}
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, "", now); err != nil {
		return record, "", err
	}
	if beforeJournal != nil {
		beforeJournal()
	}
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, "", now); err != nil {
		return record, "", err
	}
	startedAt := now()
	record, err = beginHandoffOperation(stateRoot, record, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTerminalCreate, StartedAt: startedAt, BaselinePTYIDs: baseline,
	})
	if err != nil {
		return record, "", err
	}
	created, err := client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
		WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
		Agent:      record.ExecutionHandoff.Agent,
		Title:      issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
		AllowCodexHookTrustBypass: record.ExecutionHandoff.ContextOptions != nil &&
			record.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass,
	})
	if err != nil {
		transitionAt := now()
		capabilityLost := terminalCreateCapabilityLost(err)
		if externalMutationNotInvoked(err) && !capabilityLost {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTerminalCreate, transitionAt, nil)
			if clearErr != nil {
				return record, "", fmt.Errorf("clear non-invoked terminal create journal: %w", clearErr)
			}
			return cleared, "", fmt.Errorf("Orca terminal create was not invoked and is safe to retry: %w", err)
		}
		if capabilityLost {
			_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_create_capability_lost", err.Error(), transitionAt)
			return record, "", fmt.Errorf("Orca terminal create capability changed after provisioning and requires recovery: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_create_ambiguous", err.Error(), transitionAt)
		return record, "", fmt.Errorf("Orca terminal create requires recovery: %w", err)
	}
	if created.WorktreeID != record.ExecutionHandoff.Orca.WorktreeID || strings.TrimSpace(created.Handle) == "" {
		err = fmt.Errorf("Orca terminal identity does not match the prepared worktree")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now())
		return record, "", err
	}
	currentTerminals, err := client.ListTerminals(ctx, record.ExecutionHandoff.Orca.WorktreeID)
	if err != nil {
		err = fmt.Errorf("list terminals after create: %w", err)
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now())
		return record, "", err
	}
	terminal, err := ReconcileIssueOpsHandoffTerminal(baseline, record.ExecutionHandoff.Orca.WorktreeID, record.ExecutionHandoff.WorkerRoot, currentTerminals)
	if err != nil {
		err = fmt.Errorf("reconcile created terminal: %w", err)
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now())
		return record, "", err
	}
	if createdPTYID := strings.TrimSpace(created.PTYID); createdPTYID != "" && terminal.PTYID != createdPTYID {
		err = fmt.Errorf("created terminal PTY does not match the exact terminal-list delta")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_identity_mismatch", err.Error(), now())
		return record, "", err
	}
	record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTerminalCreate, now(), func(h *IssueOpsExecutionHandoff) error {
		h.Orca.TerminalBaselinePTYIDs = baseline
		if terminal.RuntimeID != "" {
			h.Orca.RuntimeID = terminal.RuntimeID
		}
		h.Orca.WorkerPTYID = terminal.PTYID
		h.Orca.WorkerTerminalHandle = terminal.Handle
		h.Orca.WorkerTabID = terminal.TabID
		h.Orca.WorkerLeafID = terminal.LeafID
		return nil
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "terminal_persist_failed", err.Error(), now())
		return record, "", err
	}
	return record, terminal.Handle, nil
}

func ensureHandoffTask(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, contextMarkdown string, now func() string, beforeJournal func()) (IssueOpsRecord, error) {
	if strings.TrimSpace(record.ExecutionHandoff.Orca.TaskID) != "" {
		return record, nil
	}
	tasks, err := client.ListTasks(ctx)
	if err != nil {
		return record, persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("list tasks before create requires recovery: %w", err), now)
	}
	baseline, err := handoff.CanonicalBaselineIDs("task", taskIDs(tasks))
	if err != nil {
		return record, persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("Orca task baseline requires recovery: %w", err), now)
	}
	if err := handoff.RequireBaselineDeltaHeadroom("task", baseline); err != nil {
		return record, persistHandoffSoleWriterInventoryFailure(stateRoot, record, fence, fmt.Errorf("Orca task baseline requires recovery: %w", err), now)
	}
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, record.ExecutionHandoff.Orca.WorkerTerminalHandle, now); err != nil {
		return record, err
	}
	if beforeJournal != nil {
		beforeJournal()
	}
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, record.ExecutionHandoff.Orca.WorkerTerminalHandle, now); err != nil {
		return record, err
	}
	startedAt := now()
	record, err = beginHandoffOperation(stateRoot, record, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, StartedAt: startedAt, BaselineTaskIDs: baseline,
	})
	if err != nil {
		return record, err
	}
	title, displayName, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		return record, err
	}
	task, err := client.CreateTask(ctx, port.OrcaCreateTaskRequest{Spec: contextMarkdown, Title: title, DisplayName: displayName})
	if err != nil {
		transitionAt := now()
		if externalMutationNotInvoked(err) {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTaskCreate, transitionAt, nil)
			if clearErr != nil {
				return record, fmt.Errorf("clear non-invoked task create journal: %w", clearErr)
			}
			return cleared, fmt.Errorf("Orca task create was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_create_ambiguous", err.Error(), transitionAt)
		return record, fmt.Errorf("Orca task create requires recovery: %w", err)
	}
	if strings.TrimSpace(task.ID) == "" || task.Title != title || task.DisplayName != displayName || !validInitialOrcaTaskStatus(task.Status) {
		err = fmt.Errorf("Orca task identity, marker, display name, or initial status does not match the prepared task")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_identity_mismatch", err.Error(), now())
		return record, err
	}
	record, err = completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationTaskCreate, now(), func(h *IssueOpsExecutionHandoff) error {
		h.Orca.TaskID = task.ID
		return nil
	})
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "task_persist_failed", err.Error(), now())
	}
	return record, err
}

func dispatchHandoff(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, liveHandle string, now func() string, beforeJournal func()) (IssueOpsRecord, error) {
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, liveHandle, now); err != nil {
		return record, err
	}
	if beforeJournal != nil {
		beforeJournal()
	}
	if err := attestHandoffSoleWriterWithRecovery(ctx, stateRoot, record, fence, client, liveHandle, now); err != nil {
		return record, err
	}
	startedAt := now()
	record, err := beginHandoffOperation(stateRoot, record, fence, model.IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationDispatch, StartedAt: startedAt, ExpectedAssigneeHandle: strings.TrimSpace(liveHandle), DeliveryMode: "inject",
	})
	if err != nil {
		return record, err
	}
	dispatched, err := client.Dispatch(ctx, port.OrcaDispatchRequest{
		TaskID: record.ExecutionHandoff.Orca.TaskID, ToHandle: liveHandle, FromHandle: record.ExecutionHandoff.CoordinatorMailboxHandle, Inject: true, ReturnPreamble: true,
	})
	if err != nil {
		transitionAt := now()
		if externalMutationNotInvoked(err) {
			cleared, clearErr := completeHandoffOperation(stateRoot, record.ID, fence, handoff.OperationDispatch, transitionAt, nil)
			if clearErr != nil {
				return record, fmt.Errorf("clear non-invoked dispatch journal: %w", clearErr)
			}
			return cleared, fmt.Errorf("Orca dispatch was not invoked and is safe to retry: %w", err)
		}
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_ambiguous", err.Error(), transitionAt)
		return record, fmt.Errorf("Orca dispatch requires recovery: %w", err)
	}
	if dispatched.ID == "" || dispatched.TaskID != record.ExecutionHandoff.Orca.TaskID || dispatched.AssigneeHandle != liveHandle || dispatched.Status != "dispatched" || !dispatched.Injected {
		err = fmt.Errorf("Orca dispatch identity, status, or injected delivery does not match the persisted task and worker mailbox")
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_identity_mismatch", err.Error(), now())
		return record, err
	}
	if err := validateHandoffDispatchPreamble(dispatched.Preamble, record.ExecutionHandoff.CoordinatorMailboxHandle, dispatched.TaskID, dispatched.ID); err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_preamble_mismatch", err.Error(), now())
		return record, err
	}
	record, err = finalizeHandoffDispatch(stateRoot, record.ID, fence, dispatched.ID, dispatched.AssigneeHandle, now())
	if err != nil {
		_ = markHandoffPrepareRecovery(stateRoot, record.ID, fence, "dispatch_persist_failed", err.Error(), now())
	}
	return record, err
}

type handoffSoleWriterRecoveryError struct{ cause error }

func (e handoffSoleWriterRecoveryError) Error() string { return e.cause.Error() }
func (e handoffSoleWriterRecoveryError) Unwrap() error { return e.cause }

func soleWriterRecoveryError(format string, args ...any) error {
	return handoffSoleWriterRecoveryError{cause: errors.New(soleWriterRecoveryDiagnostic(fmt.Sprintf(format, args...)))}
}

type handoffSoleWriterConflictError struct{ cause error }

func (e handoffSoleWriterConflictError) Error() string { return e.cause.Error() }
func (e handoffSoleWriterConflictError) Unwrap() error { return e.cause }

func soleWriterConflictError(format string, args ...any) error {
	return handoffSoleWriterConflictError{cause: errors.New(soleWriterRecoveryDiagnostic(fmt.Sprintf(format, args...)))}
}

func soleWriterRecoveryDiagnostic(value string) string {
	value = strings.TrimSpace(policy.RedactFreeform(strings.TrimSpace(value)))
	if len(value) > publicationDiagnosticLimit {
		value = value[:publicationDiagnosticLimit]
	}
	return value
}

func persistHandoffSoleWriterInventoryFailure(stateRoot string, record IssueOpsRecord, fence handoff.Fence, cause error, now func() string) error {
	err := soleWriterRecoveryError("%v", cause)
	if markErr := markHandoffSoleWriterRecovery(stateRoot, record, fence, "sole_writer_inventory_ambiguous", err.Error(), now()); markErr != nil {
		return fmt.Errorf("%v; persist sole writer recovery: %w", err, markErr)
	}
	return err
}

func attestHandoffSoleWriterWithRecovery(ctx context.Context, stateRoot string, record IssueOpsRecord, fence handoff.Fence, client IssueOpsOrcaDispatchClient, allowedHandle string, now func() string) error {
	err := attestHandoffSoleWriter(ctx, record, client, allowedHandle)
	if err == nil {
		return nil
	}
	var recoveryErr handoffSoleWriterRecoveryError
	var conflictErr handoffSoleWriterConflictError
	if !errors.As(err, &recoveryErr) && !errors.As(err, &conflictErr) {
		return err
	}
	code := "sole_writer_inventory_ambiguous"
	if errors.As(err, &conflictErr) {
		code = "sole_writer_conflict"
	}
	if markErr := markHandoffSoleWriterRecovery(stateRoot, record, fence, code, err.Error(), now()); markErr != nil {
		return fmt.Errorf("%v; persist sole writer recovery: %w", err, markErr)
	}
	return err
}

func markHandoffSoleWriterRecovery(stateRoot string, expected IssueOpsRecord, fence handoff.Fence, code, message, now string) error {
	message = soleWriterRecoveryDiagnostic(message)
	return withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, expected) {
			return fmt.Errorf("handoff changed during sole writer attestation")
		}
		record, err = handoff.BeginOperation(record, fence, model.IssueOpsExecutionHandoffPendingOperation{
			Kind: handoff.OperationLeaseAttestation, StartedAt: now,
		})
		if err != nil {
			return err
		}
		record, err = handoff.MarkRecoveryRequired(record, fence, model.IssueOpsExecutionHandoffFailure{
			Code: code, Message: message, At: now,
		})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func attestHandoffSoleWriter(ctx context.Context, record IssueOpsRecord, client IssueOpsOrcaDispatchClient, allowedHandle string) error {
	h := record.ExecutionHandoff
	if h == nil || h.Orca == nil || strings.TrimSpace(h.Orca.WorktreeID) == "" {
		return soleWriterRecoveryError("sole writer attestation requires exact Orca worktree authority")
	}
	terminals, err := client.ListTerminals(ctx, h.Orca.WorktreeID)
	if err != nil {
		return soleWriterRecoveryError("sole writer terminal inventory requires recovery: %v", err)
	}
	if len(terminals) > handoff.MaxBaselineIDs {
		return soleWriterRecoveryError("sole writer terminal inventory requires recovery: exceeds %d entries", handoff.MaxBaselineIDs)
	}
	ptyIDs := make([]string, len(terminals))
	handles := make([]string, len(terminals))
	exactHandles := make(map[string]struct{}, len(terminals))
	for _, handle := range []string{h.Orca.WorkerTerminalHandle, h.Orca.WorkerMailboxHandle} {
		if handle = strings.TrimSpace(handle); handle != "" {
			exactHandles[handle] = struct{}{}
		}
	}
	allowedMatches := 0
	for i, terminal := range terminals {
		ptyIDs[i], handles[i] = terminal.PTYID, terminal.Handle
	}
	if err := requireStableInventoryIdentities("terminal", ptyIDs); err != nil {
		return soleWriterRecoveryError("sole writer terminal inventory requires recovery: %v", err)
	}
	if err := requireStableInventoryIdentities("terminal", handles); err != nil {
		return soleWriterRecoveryError("sole writer terminal inventory requires recovery: %v", err)
	}
	for _, terminal := range terminals {
		if terminal.WorktreeID != h.Orca.WorktreeID || !terminalWorktreePathMatches(terminal, h.WorkerRoot) {
			return soleWriterRecoveryError("sole writer terminal inventory requires recovery: row does not match the exact worktree")
		}
		exactHandles[terminal.Handle] = struct{}{}
		if terminal.Connected || terminal.Writable {
			if terminal.Handle != strings.TrimSpace(allowedHandle) {
				return soleWriterConflictError("sole writer attestation found a competing connected or writable terminal")
			}
			if !terminal.Connected || !terminal.Writable {
				return soleWriterRecoveryError("sole writer designated terminal has inconsistent connected/writable state")
			}
			allowedMatches++
		}
	}
	if allowedHandle != "" && allowedMatches != 1 {
		return soleWriterRecoveryError("sole writer attestation requires exactly one connected writable designated worker terminal; found %d", allowedMatches)
	}
	dispatchedTasks, err := client.ListDispatchedTasks(ctx)
	if err != nil {
		return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: %v", err)
	}
	if len(dispatchedTasks) > handoff.MaxBaselineIDs {
		return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: exceeds %d entries", handoff.MaxBaselineIDs)
	}
	taskIDs := make([]string, len(dispatchedTasks))
	for i, task := range dispatchedTasks {
		taskIDs[i] = task.ID
	}
	if err := requireStableInventoryIdentities("task", taskIDs); err != nil {
		return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: %v", err)
	}
	for _, task := range dispatchedTasks {
		if strings.TrimSpace(task.Status) != "dispatched" {
			return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: row has a non-dispatched status")
		}
		dispatch, showErr := client.ShowDispatch(ctx, task.ID)
		if showErr != nil {
			return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: dispatch inspection failed")
		}
		if requireStableInventoryIdentities("task", []string{dispatch.ID}) != nil || requireStableInventoryIdentities("task", []string{dispatch.TaskID}) != nil || requireStableInventoryIdentities("terminal", []string{dispatch.AssigneeHandle}) != nil {
			return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: dispatch identity is not stable and bounded")
		}
		if dispatch.TaskID != task.ID || dispatch.Status != "dispatched" {
			return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: row has incomplete dispatch identity")
		}
		if _, assignedHere := exactHandles[dispatch.AssigneeHandle]; assignedHere {
			return soleWriterConflictError("sole writer attestation found a dispatched task assigned to the exact worktree")
		}
		return soleWriterRecoveryError("sole writer dispatched task inventory requires recovery: assignee terminal is absent from the exact worktree inventory")
	}
	return nil
}

func persistHandoffContext(stateRoot, id, coordinatorRecipient string, coordinatorSession model.IssueOpsHostSessionIdentity, options model.IssueOpsExecutionHandoffContextOptions, expectedContextSHA256 string, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := validateHandoffCleanExactCheckpoint(record); err != nil {
			return err
		}
		if sealed := strings.TrimSpace(record.ExecutionHandoff.CoordinatorMailboxHandle); sealed != "" && sealed != coordinatorRecipient {
			return fmt.Errorf("coordinator recipient differs from sealed handoff authority")
		}
		record.ExecutionHandoff.CoordinatorMailboxHandle = coordinatorRecipient
		if record.ExecutionHandoff.CoordinatorSession != nil && !reflect.DeepEqual(*record.ExecutionHandoff.CoordinatorSession, coordinatorSession) {
			return fmt.Errorf("coordinator native session differs from sealed handoff authority")
		}
		record.ExecutionHandoff.CoordinatorSession = &coordinatorSession
		packet, err := handoff.BuildContext(record, handoff.ContextOptionsFromModel(options))
		if err != nil {
			return fmt.Errorf("re-render handoff context before persist: %w", err)
		}
		if packet.SHA256 != expectedContextSHA256 {
			return fmt.Errorf("expected_context_sha256 does not match freshly recomputed sealed context")
		}
		if record.ExecutionHandoff.ContextSHA256 != "" {
			persistedOptions := model.IssueOpsExecutionHandoffContextOptions{}
			if record.ExecutionHandoff.ContextOptions != nil {
				persistedOptions = handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(*record.ExecutionHandoff.ContextOptions))
			}
			if record.ExecutionHandoff.ContextSHA256 != packet.SHA256 || record.ExecutionHandoff.ContextSourceSHA256 != packet.SourceSHA256 || record.ExecutionHandoff.ContextVersion != packet.Version || record.ExecutionHandoff.ContextOptions == nil || !reflect.DeepEqual(persistedOptions, handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(options))) {
				return fmt.Errorf("persisted handoff context differs from current context")
			}
			persisted = record
			return nil
		}
		record, err = handoff.SetContext(record, handoffFence(record), packet.Version, packet.SHA256, packet.SourceSHA256, options, now)
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateExpectedContextSHA256(expected string) error {
	if strings.TrimSpace(expected) == "" {
		return fmt.Errorf("expected_context_sha256 is required for confirmed supervised start")
	}
	if len(expected) != 64 || expected != strings.ToLower(expected) {
		return fmt.Errorf("expected_context_sha256 must be a lowercase SHA-256 hex digest")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("expected_context_sha256 must be a valid SHA-256 hex digest")
	}
	return nil
}

func resolveHandoffCoordinatorRecipient(ctx context.Context, stateRoot string, record IssueOpsRecord, supplied string, session model.IssueOpsHostSessionIdentity, client IssueOpsOrcaDispatchClient) (string, error) {
	supplied = strings.TrimSpace(supplied)
	sealed := ""
	if record.ExecutionHandoff != nil {
		sealed = strings.TrimSpace(record.ExecutionHandoff.CoordinatorMailboxHandle)
	}
	if supplied == "" {
		supplied = sealed
	}
	if supplied == "" {
		if strings.TrimSpace(session.Host) == "" || strings.TrimSpace(session.SessionID) == "" || client == nil {
			return "", fmt.Errorf("coordinator recipient must be a concrete bounded Orca terminal handle")
		}
		worktrees, err := client.ListWorktrees(ctx, record.Repo)
		if err != nil {
			return "", fmt.Errorf("resolve source coordinator worktree: %w", err)
		}
		var source *port.OrcaWorktree
		for i := range worktrees {
			if terminalWorktreePathMatches(port.OrcaTerminal{WorktreePath: worktrees[i].Path}, record.Repo) {
				if source != nil {
					return "", fmt.Errorf("coordinator recipient requires exactly one source Orca worktree")
				}
				source = &worktrees[i]
			}
		}
		if source == nil || strings.TrimSpace(source.ID) == "" {
			return "", fmt.Errorf("coordinator recipient requires an exact source Orca worktree")
		}
		terminals, err := client.ListTerminals(ctx, source.ID)
		if err != nil {
			return "", fmt.Errorf("resolve source coordinator terminal: %w", err)
		}
		for _, terminal := range terminals {
			if terminal.WorktreeID != source.ID || !terminalWorktreePathMatches(terminal, record.Repo) || !terminal.Connected || !terminal.Writable {
				continue
			}
			if supplied != "" {
				return "", fmt.Errorf("coordinator recipient requires exactly one connected writable source terminal")
			}
			supplied = terminal.Handle
		}
	}
	if !concreteOrcaTerminalHandlePattern.MatchString(supplied) || len(supplied) > 256 {
		return "", fmt.Errorf("coordinator recipient must be a concrete bounded Orca terminal handle")
	}
	if sealed != "" && supplied != sealed {
		return "", fmt.Errorf("coordinator recipient differs from sealed handoff authority")
	}
	if sealed == "" && strings.TrimSpace(stateRoot) != "" {
		claimed, err := handoffCoordinatorRecipientClaimed(stateRoot, record.ID, record.Repo, supplied)
		if err != nil {
			return "", err
		}
		if claimed {
			return "", fmt.Errorf("coordinator recipient is sealed by another active handoff")
		}
	}
	return supplied, nil
}

func handoffCoordinatorRecipientClaimed(stateRoot, currentID, repo, handle string) (bool, error) {
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id == currentID {
			continue
		}
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return false, err
		}
		if filepath.Clean(record.Repo) != filepath.Clean(repo) || record.ExecutionHandoff == nil || record.ExecutionHandoff.State == handoff.StateClosed {
			continue
		}
		if strings.TrimSpace(record.ExecutionHandoff.CoordinatorMailboxHandle) == handle {
			return true, nil
		}
	}
	return false, nil
}

func validateHandoffDispatchPreamble(preamble, coordinatorRecipient, taskID, dispatchID string) error {
	if len(preamble) == 0 || len(preamble) > handoff.MaxRenderedContextBytes {
		return fmt.Errorf("Orca dispatch preamble is missing or exceeds the bounded context limit")
	}
	const coordinatorLabel = "Your coordinator's terminal handle is: "
	const taskLabel = "Your task ID is: "
	coordinatorLine := coordinatorLabel + coordinatorRecipient
	taskLine := taskLabel + taskID
	coordinatorCount, taskCount, dispatchCount := 0, 0, 0
	foundCoordinator, foundTask, foundDispatch := false, false, false
	for _, line := range strings.Split(preamble, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, coordinatorLabel) {
			coordinatorCount++
			foundCoordinator = foundCoordinator || line == coordinatorLine
		}
		if strings.HasPrefix(line, taskLabel) {
			taskCount++
			foundTask = foundTask || line == taskLine
		}
		fields := strings.Fields(line)
		for index, field := range fields {
			switch {
			case field == "--dispatch-id":
				dispatchCount++
				if index+1 < len(fields) && fields[index+1] == dispatchID {
					foundDispatch = true
				}
			case strings.HasPrefix(field, "--dispatch-id="):
				dispatchCount++
			}
		}
	}
	if coordinatorRecipient == "" || coordinatorCount != 1 || !foundCoordinator {
		return fmt.Errorf("Orca dispatch preamble must contain exactly one official coordinator recipient line")
	}
	if taskID == "" || taskCount != 1 || !foundTask {
		return fmt.Errorf("Orca dispatch preamble must contain exactly one official task id line")
	}
	if dispatchID == "" || dispatchCount != 1 || !foundDispatch {
		return fmt.Errorf("Orca dispatch preamble must contain exactly one exact --dispatch-id token")
	}
	return nil
}

func resolveHandoffContextOptions(record IssueOpsRecord, supplied handoff.ContextOptions) (handoff.ContextOptions, error) {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.ContextOptions == nil {
		return supplied, nil
	}
	persisted := *record.ExecutionHandoff.ContextOptions
	if handoff.ContextOptionsEmpty(supplied) {
		return handoff.ContextOptionsFromModel(persisted), nil
	}
	if record.ExecutionHandoff.ContextSHA256 == "" && !persisted.AllowCodexHookTrustBypass && supplied.AllowCodexHookTrustBypass {
		expected := handoff.ContextOptionsFromModel(persisted)
		expected.AllowCodexHookTrustBypass = true
		withoutAttestation := supplied
		withoutAttestation.AllowCodexHookTrustBypass = false
		if handoff.ContextOptionsEmpty(withoutAttestation) || reflect.DeepEqual(handoff.CanonicalContextOptions(supplied), handoff.CanonicalContextOptions(expected)) {
			return expected, nil
		}
	}
	if !reflect.DeepEqual(handoff.CanonicalContextOptions(supplied), handoff.CanonicalContextOptions(handoff.ContextOptionsFromModel(persisted))) {
		return handoff.ContextOptions{}, fmt.Errorf("supplied handoff context options do not match the sealed delivery contract")
	}
	return handoff.ContextOptionsFromModel(persisted), nil
}

func beginHandoffOperation(stateRoot string, expected IssueOpsRecord, fence handoff.Fence, pending model.IssueOpsExecutionHandoffPendingOperation) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, expected) {
			return fmt.Errorf("handoff changed before %s journal", pending.Kind)
		}
		if err := validateHandoffContextSource(record); err != nil {
			return err
		}
		if err := validateHandoffCleanExactCheckpoint(record); err != nil {
			return err
		}
		record, err = handoff.BeginOperation(record, fence, pending)
		if err != nil {
			return err
		}
		record.UpdatedAt = pending.StartedAt
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateHandoffStageCheckpoint(stateRoot string, expected IssueOpsRecord) (IssueOpsRecord, error) {
	var validated IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, expected) {
			return fmt.Errorf("handoff changed before the next Orca stage")
		}
		if err := validateHandoffContextSource(record); err != nil {
			return err
		}
		if err := validateHandoffCleanExactCheckpoint(record); err != nil {
			return err
		}
		validated = record
		return nil
	})
	return validated, err
}

func runHandoffStartHook(hook func(string), stage string) {
	if hook != nil {
		hook(stage)
	}
}

func completeHandoffOperation(stateRoot, id string, fence handoff.Fence, kind, now string, update func(*IssueOpsExecutionHandoff) error) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
			return fmt.Errorf("handoff is no longer coordinator_preparing")
		}
		if update != nil {
			if err := update(record.ExecutionHandoff); err != nil {
				return err
			}
		}
		record, err = handoff.CompleteOperation(record, fence, kind, now)
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func finalizeHandoffDispatch(stateRoot, id string, fence handoff.Fence, dispatchID, assigneeHandle, now string) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil || record.ExecutionHandoff.State != handoff.StateCoordinatorPreparing {
			return fmt.Errorf("handoff is no longer coordinator_preparing")
		}
		identity := *record.ExecutionHandoff.Orca
		if sealed := strings.TrimSpace(identity.WorkerMailboxHandle); identity.DispatchID != "" && sealed != "" && sealed != strings.TrimSpace(assigneeHandle) {
			return fmt.Errorf("worker mailbox recipient differs from sealed dispatch authority")
		}
		identity.DispatchID = dispatchID
		identity.WorkerMailboxHandle = strings.TrimSpace(assigneeHandle)
		identity.WorkerTerminalHandle = strings.TrimSpace(assigneeHandle)
		record, err = handoff.CompleteOperation(record, fence, handoff.OperationDispatch, now)
		if err != nil {
			return err
		}
		record, err = handoff.Dispatch(record, fence, identity, now)
		if err != nil {
			return err
		}
		record.ExecutionHandoff.DeliveryMode = "inject"
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func projectHandoffStart(record IssueOpsRecord, preview bool, planSHA string) IssueOpsHandoffStartResult {
	result := IssueOpsHandoffStartResult{OK: true, ID: record.ID, Preview: preview, PlanSHA256: planSHA}
	if record.ExecutionHandoff == nil {
		return result
	}
	result.State = record.ExecutionHandoff.State
	result.Disposition = record.ExecutionHandoff.ClosedDisposition
	result.Attempt = record.ExecutionHandoff.Attempt
	result.ContextSHA256 = record.ExecutionHandoff.ContextSHA256
	result.CodexHookTrustBypassRequired = codexHookTrustBypassRequired(record)
	result.CodexHookTrustBypassAttested = record.ExecutionHandoff.ContextOptions != nil && record.ExecutionHandoff.ContextOptions.AllowCodexHookTrustBypass
	result.Orca = record.ExecutionHandoff.Orca
	result.CoordinatorRecipient = record.ExecutionHandoff.CoordinatorMailboxHandle
	if record.ExecutionHandoff.State == handoff.StateRecoveryRequired {
		result.RecoveryCode = "explicit_reconcile_required"
		if record.ExecutionHandoff.Failure != nil && record.ExecutionHandoff.Failure.Code != "" {
			result.RecoveryCode = record.ExecutionHandoff.Failure.Code
		}
	}
	return result
}

func codexHookTrustBypassRequired(record IssueOpsRecord) bool {
	return record.ExecutionHandoff != nil && record.ExecutionHandoff.Driver == "orca" && record.ExecutionHandoff.Agent == "codex"
}

func ReconcileIssueOpsHandoffTerminal(baseline []string, worktreeID, workerRoot string, rows []port.OrcaTerminal) (port.OrcaTerminal, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTerminal, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.PTYID]; existed || row.PTYID == "" {
			continue
		}
		if row.WorktreeID == strings.TrimSpace(worktreeID) && strings.TrimSpace(row.Handle) != "" && row.Connected && row.Writable && terminalWorktreePathMatches(row, workerRoot) {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTerminal{}, fmt.Errorf("terminal recovery requires exactly one PTY delta; found %d", len(candidates))
	}
	return candidates[0], nil
}

func terminalWorktreePathMatches(terminal port.OrcaTerminal, workerRoot string) bool {
	path := strings.TrimSpace(terminal.WorktreePath)
	return path != "" && filepath.Clean(path) == filepath.Clean(strings.TrimSpace(workerRoot))
}

func ReconcileIssueOpsHandoffTask(baseline []string, marker, displayName string, rows []port.OrcaTask) (port.OrcaTask, error) {
	before := make(map[string]struct{}, len(baseline))
	for _, id := range baseline {
		before[id] = struct{}{}
	}
	candidates := make([]port.OrcaTask, 0, 1)
	for _, row := range rows {
		if _, existed := before[row.ID]; existed || row.ID == "" {
			continue
		}
		if row.Title == marker && row.DisplayName == displayName && validInitialOrcaTaskStatus(row.Status) {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) != 1 {
		return port.OrcaTask{}, fmt.Errorf("task recovery requires exactly one marker candidate; found %d", len(candidates))
	}
	return candidates[0], nil
}

func validInitialOrcaTaskStatus(status string) bool {
	return status == "ready"
}

const (
	orcaTaskTitleMaxLength   = 80
	orcaDisplayNameMaxLength = 160
)

func issueOpsHandoffTaskIdentity(id, epoch string, attempt int) (string, string, error) {
	title := "ah id=" + strings.TrimSpace(id) + " a=" + strconv.Itoa(attempt) + " e=" + strings.TrimSpace(epoch)
	display := "ah " + strings.TrimSpace(id) + " attempt=" + strconv.Itoa(attempt)
	if len(title) == 0 || len(title) > orcaTaskTitleMaxLength || len(display) == 0 || len(display) > orcaDisplayNameMaxLength {
		return "", "", fmt.Errorf("Orca task identity exceeds title/display limits %d/%d", orcaTaskTitleMaxLength, orcaDisplayNameMaxLength)
	}
	return title, display, nil
}

func ReconcileIssueOpsHandoffDispatch(ctx context.Context, taskID, assigneeHandle, deliveryMode string, client interface {
	ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error)
}, coordinatorRecipient string) (port.OrcaDispatch, error) {
	taskID = strings.TrimSpace(taskID)
	assigneeHandle = strings.TrimSpace(assigneeHandle)
	if taskID == "" || assigneeHandle == "" || deliveryMode != "inject" {
		return port.OrcaDispatch{}, fmt.Errorf("persisted task id, expected assignee, and inject delivery are required for dispatch recovery")
	}
	var err error
	coordinatorRecipient, err = resolveHandoffCoordinatorRecipient(context.Background(), "", IssueOpsRecord{}, coordinatorRecipient, model.IssueOpsHostSessionIdentity{}, nil)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	dispatch, err := client.ShowDispatchFrom(ctx, taskID, coordinatorRecipient)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if dispatch.ID == "" || dispatch.TaskID != taskID || strings.TrimSpace(dispatch.AssigneeHandle) != assigneeHandle || dispatch.Status != "dispatched" {
		return port.OrcaDispatch{}, fmt.Errorf("dispatch recovery result does not match persisted task, assignee, and dispatched status")
	}
	if err := validateHandoffDispatchPreamble(dispatch.Preamble, coordinatorRecipient, taskID, dispatch.ID); err != nil {
		return port.OrcaDispatch{}, err
	}
	return dispatch, nil
}

func handoffFence(record IssueOpsRecord) handoff.Fence {
	if record.ExecutionHandoff == nil {
		return handoff.Fence{}
	}
	return handoff.Fence{Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch, ContextSHA256: record.ExecutionHandoff.ContextSHA256}
}

func issueOpsHandoffStartNow(clock IssueOpsHandoffStartClock) string {
	if clock.Now != nil {
		return clock.Now().UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func terminalPTYIDs(rows []port.OrcaTerminal) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PTYID != "" {
			values = append(values, row.PTYID)
		}
	}
	sort.Strings(values)
	return values
}

func taskIDs(rows []port.OrcaTask) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Status == "ready" && row.ID != "" {
			values = append(values, row.ID)
		}
	}
	sort.Strings(values)
	return values
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
