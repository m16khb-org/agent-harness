package issueopspreparation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	preparationdomain "agent-harness/internal/domain/issueopspreparation"
)

type Service struct {
	repository Repository
	clock      Clock
	operation  OperationID
	direct     DirectWorkspace
	orca       OrcaGateway
	evidence   PreparationEvidence
}

func NewService(repository Repository, clock Clock, operation OperationID, direct DirectWorkspace, orca OrcaGateway, evidence PreparationEvidence) *Service {
	return &Service{repository: repository, clock: clock, operation: operation, direct: direct, orca: orca, evidence: evidence}
}

func (service *Service) Prepare(ctx context.Context, command preparationcontract.Command) (preparationcontract.Result, error) {
	if service.repository == nil {
		return failedResult(command.ID), fmt.Errorf("preparation repository is unavailable")
	}
	if command.Confirm {
		if err := service.repository.RequireMutationAllowed(ctx); err != nil {
			return failedResult(command.ID), err
		}
	}
	snapshot, err := service.repository.Load(ctx, command.ID)
	if err != nil {
		return failedResult(command.ID), err
	}
	command = normalizeOwnerDefaults(command)
	requested, err := preparationdomain.NormalizeMode(command.Mode)
	if err != nil {
		return failedResult(command.ID), err
	}
	command.Mode = requested
	if snapshot.Record.Execution != nil {
		decision, decideErr := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command, Snapshot: snapshot})
		if decideErr != nil {
			return failedResult(command.ID), decideErr
		}
		return service.resolveExisting(snapshot, command, decision)
	}
	if service.evidence == nil {
		return failedResult(command.ID), fmt.Errorf("preparation evidence is unavailable")
	}
	workspace, err := service.evidence.Workspace(snapshot, command.Confirm)
	if err != nil {
		return failedResult(command.ID), err
	}
	snapshot.CanonicalRoot = workspace.Root
	if err := service.repository.EnsureRootUnclaimed(ctx, command.ID, workspace.Root); err != nil {
		return preparationcontract.Result{ID: command.ID, RequestedMode: requested}, err
	}
	readiness := preparationdomain.OrcaReadiness{}
	probeRequest := preparationcontract.ProbeRequest{}
	if requested != preparationcontract.ModeDirect {
		if service.orca == nil {
			readiness.Code = "orca_adapter_unavailable"
		} else {
			if command.OwnerHost != "codex" && command.OwnerHost != "claude" {
				return failedResult(command.ID), fmt.Errorf("Orca owner_host must be codex or claude")
			}
			codec := preparationcontract.IntentCodec{}
			issue, issueErr := codec.PrepareIssueIdentity(snapshot.Record)
			if issueErr != nil {
				return failedResult(command.ID), issueErr
			}
			marker, markerErr := codec.RenderReadinessMarker(snapshot.Record.ID, issue)
			if markerErr != nil {
				return failedResult(command.ID), markerErr
			}
			probeRequest = preparationcontract.ProbeRequest{
				Repo: snapshot.Record.Repo, Host: strings.ToLower(strings.TrimSpace(command.OwnerHost)),
				Model: strings.TrimSpace(command.OwnerModel), Effort: strings.TrimSpace(command.OwnerEffort),
				Provider: issue.Provider, Issue: issue.Issue, Marker: marker, Workspace: workspace,
			}
			probe, probeErr := service.orca.Probe(ctx, probeRequest)
			readiness.Ready = probeErr == nil && probe.Available && probe.Ready
			readiness.Code = strings.TrimSpace(probe.Code)
			if probeErr != nil && readiness.Code == "" {
				readiness.Code = "orca_probe_failed"
			}
			if probeErr != nil && requested == preparationcontract.ModeOrca {
				return failedResult(command.ID), probeErr
			}
		}
	}
	decision, err := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command, Snapshot: snapshot, Orca: readiness})
	if err != nil {
		return preparationcontract.Result{ID: command.ID, RequestedMode: requested}, err
	}
	if decision.ResolvedMode == preparationcontract.ModeOrca {
		return service.prepareOrca(ctx, snapshot, command, workspace, probeRequest, decision)
	}
	return service.prepareDirect(ctx, snapshot, command, workspace, decision)
}

func (service *Service) prepareOrca(
	ctx context.Context,
	snapshot preparationcontract.Snapshot,
	command preparationcontract.Command,
	workspace preparationcontract.WorkspaceRequest,
	probe preparationcontract.ProbeRequest,
	decision preparationdomain.Decision,
) (preparationcontract.Result, error) {
	if service.orca == nil {
		return failedResult(command.ID), fmt.Errorf("Orca provisioner is unavailable")
	}
	workspace.CWD = command.CWD
	probe.Workspace = workspace
	if command.Confirm {
		actor, err := normalizeActor(command.Actor)
		if err != nil {
			return failedResult(command.ID), err
		}
		command.Actor = actor
	}
	owner, err := service.evidence.ReadOwner(ctx, snapshot, command)
	if err != nil {
		return failedResult(command.ID), err
	}
	if owner.Provider == "" {
		owner.Provider = probe.Provider
	}
	if owner.Issue == 0 {
		owner.Issue = probe.Issue
	}
	if owner.Provider != probe.Provider || owner.Issue != probe.Issue {
		return failedResult(command.ID), fmt.Errorf("owner issue identity changed before Orca intent persistence")
	}
	preview := preparationcontract.Result{
		OK: true, ID: command.ID, Preview: !command.Confirm,
		RequestedMode: decision.RequestedMode, ResolvedMode: preparationcontract.ModeOrca,
		FallbackCode: decision.FallbackCode, Workspace: workspaceResult(workspace, "orca", ""),
	}
	if !command.Confirm {
		return preview, nil
	}
	if service.operation == nil {
		return failedResult(command.ID), fmt.Errorf("preparation operation ID generator is unavailable")
	}
	operationID, err := service.operation.New()
	if err != nil {
		return failedResult(command.ID), err
	}
	if service.clock == nil {
		return failedResult(command.ID), fmt.Errorf("preparation clock is unavailable")
	}
	state, err := service.repository.BeginIntent(ctx, OrcaBegin{
		Snapshot: snapshot, Command: command, Workspace: workspace, Probe: probe,
		Owner: owner, OperationID: operationID, StartedAt: formatTime(service.clock.Now()),
	})
	if err != nil {
		return failedResult(command.ID), err
	}
	state.Command, state.Owner, state.Pending = command.Clone(), owner, true
	progress := IntentProgress{State: state, Pending: state.Pending}
	for step := 0; progress.Pending; step++ {
		if step >= 6 {
			return failedResult(command.ID), fmt.Errorf("Orca prepare exceeded the fixed external intent stage count")
		}
		progress, err = service.advanceOrca(ctx, progress.State)
		if err != nil {
			return failedResult(command.ID), err
		}
	}
	result := progress.Result.Clone()
	result.RequestedMode = decision.RequestedMode
	result.ResolvedMode = preparationcontract.ModeOrca
	result.FallbackCode = decision.FallbackCode
	return result, nil
}

func (service *Service) advanceOrca(ctx context.Context, state IntentState) (IntentProgress, error) {
	request := intentRequest(state.Intent)
	inventory, err := service.orca.Inspect(ctx, request)
	if err != nil {
		_ = service.recordOrcaFailure(ctx, state, state.Intent.InvocationState, err)
		return IntentProgress{State: state, Pending: true}, fmt.Errorf("Orca intent inventory is ambiguous; intent retained: %w", err)
	}
	if len(inventory.Candidates) > 1 {
		cause := fmt.Errorf("Orca intent inventory found multiple candidates; intent retained")
		_ = service.recordOrcaFailure(ctx, state, state.Intent.InvocationState, cause)
		return IntentProgress{State: state, Pending: true}, cause
	}
	if len(inventory.Candidates) == 1 {
		return service.applyOrcaReceipt(ctx, state, inventory.Candidates[0])
	}
	if !inventory.AuthoritativeZero {
		cause := fmt.Errorf("Orca intent inventory returned a non-authoritative zero; intent retained")
		_ = service.recordOrcaFailure(ctx, state, state.Intent.InvocationState, cause)
		return IntentProgress{State: state, Pending: true}, cause
	}
	if state.Intent.InvocationState != preparationcontract.InvocationNotInvoked && state.Intent.Stage != preparationcontract.IntentStageRunBind {
		cause := fmt.Errorf("authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained")
		_ = service.recordOrcaFailure(ctx, state, state.Intent.InvocationState, cause)
		return IntentProgress{State: state, Pending: true}, cause
	}
	if state.Intent.InvocationAttempts >= preparationcontract.MaxInvocationAttempts {
		cause := fmt.Errorf("Orca intent retry is exhausted; intent retained")
		_ = service.recordOrcaFailure(ctx, state, state.Intent.InvocationState, cause)
		return IntentProgress{State: state, Pending: true}, cause
	}
	marked, err := service.repository.MarkInvoking(ctx, state)
	if err != nil {
		return IntentProgress{State: state, Pending: true}, err
	}
	receipt, err := service.orca.Invoke(ctx, request)
	if err != nil {
		invocation := preparationcontract.InvocationUnknown
		var typed *preparationcontract.InvocationError
		if errors.As(err, &typed) && typed.State != "" {
			invocation = typed.State
		}
		_ = service.recordOrcaFailure(ctx, marked, invocation, err)
		return IntentProgress{State: marked, Pending: true}, fmt.Errorf("Orca mutation outcome requires execution reconcile; mutation was not repeated: %w", err)
	}
	return service.applyOrcaReceipt(ctx, marked, receipt)
}

func (service *Service) applyOrcaReceipt(ctx context.Context, state IntentState, receipt preparationcontract.IntentReceipt) (IntentProgress, error) {
	if state.Intent.Stage == preparationcontract.IntentStageWorktree {
		artifacts, err := service.evidence.PrepareOwner(ctx, state.Snapshot, state.Command, state.Intent, receipt)
		if err != nil {
			_ = service.recordOrcaFailure(ctx, state, preparationcontract.InvocationUnknown, err)
			return IntentProgress{State: state, Pending: true}, err
		}
		state.OwnerArtifacts = artifacts
	}
	progress, err := service.repository.ApplyReceipt(ctx, state, receipt)
	if err != nil {
		_ = service.recordOrcaFailure(ctx, state, preparationcontract.InvocationUnknown, err)
		return IntentProgress{State: state, Pending: true}, err
	}
	return progress, nil
}

func (service *Service) recordOrcaFailure(ctx context.Context, state IntentState, invocation string, cause error) error {
	if service.clock == nil {
		return fmt.Errorf("preparation clock is unavailable")
	}
	state.FailureAt = formatTime(service.clock.Now())
	return service.repository.RecordFailure(ctx, state, invocation, cause)
}

func intentRequest(intent preparationcontract.Intent) preparationcontract.IntentRequest {
	request := preparationcontract.IntentRequest{
		Stage: intent.Stage, Marker: intent.Marker, Workspace: intent.Workspace,
		Probe: intent.Probe, Prepared: intent.Prepared, TerminalPTYID: intent.TerminalPTYID,
		RunID: intent.RunID, RunBound: intent.RunBound, TaskID: intent.TaskID,
	}
	if intent.Launch != nil {
		request.Launch = &preparationcontract.LaunchRequest{
			PromptPath: intent.Launch.PromptPath, PromptSHA256: intent.Launch.PromptSHA256,
			ContextPacketPath: intent.Launch.ContextPacketPath, ContextPacketSHA256: intent.Launch.ContextPacketSHA256,
		}
	}
	return request
}

func (service *Service) prepareDirect(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, workspace preparationcontract.WorkspaceRequest, decision preparationdomain.Decision) (preparationcontract.Result, error) {
	actor, err := normalizeActor(command.Actor)
	if err != nil {
		return failedResult(command.ID), err
	}
	command.Actor = actor
	workspace.CWD = command.CWD
	if service.direct == nil {
		return failedResult(command.ID), fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	if command.Confirm {
		access, err := service.direct.ProbeAccess(ctx, workspace, actor.Host)
		if err != nil {
			return failedResult(command.ID), err
		}
		if !access.Allowed {
			return preparationcontract.Result{
				ID: command.ID, RequestedMode: decision.RequestedMode, ResolvedMode: decision.ResolvedMode,
				Workspace: workspaceResult(workspace, "git", ""), NextCommand: access.RelaunchCommand,
			}, fmt.Errorf("canonical worktree base is not accessible; relaunch with: %s", access.RelaunchCommand)
		}
	}
	receipt, err := service.direct.Prepare(ctx, workspace)
	if err != nil {
		return failedResult(command.ID), err
	}
	if service.clock == nil {
		return failedResult(command.ID), fmt.Errorf("preparation clock is unavailable")
	}
	linkedAt := formatTime(service.clock.Now())
	result := preparationcontract.Result{
		OK: true, ID: command.ID, Preview: !command.Confirm,
		RequestedMode: decision.RequestedMode, ResolvedMode: decision.ResolvedMode, FallbackCode: decision.FallbackCode,
		Workspace: workspaceResultFromReceipt(receipt, linkedAt),
	}
	if !command.Confirm {
		return result, nil
	}
	claimedAt := formatTime(service.clock.Now())
	if err := service.evidence.MaterializeDirect(ctx, snapshot, receipt); err != nil {
		return failedResult(command.ID), err
	}
	persisted, err := service.repository.CommitDirect(ctx, DirectCommit{
		Snapshot: snapshot, Command: command, Workspace: receipt,
		RequestedMode: decision.RequestedMode, FallbackCode: decision.FallbackCode,
		LinkedAt: linkedAt, ClaimedAt: claimedAt,
	})
	if err != nil {
		return failedResult(command.ID), err
	}
	return persisted, nil
}

func (service *Service) resolveExisting(snapshot preparationcontract.Snapshot, command preparationcontract.Command, decision preparationdomain.Decision) (preparationcontract.Result, error) {
	result := preparedResult(snapshot.Record, decision.RequestedMode, decision.FallbackCode)
	switch decision.Code {
	case preparationdomain.CodeExisting:
		return result, nil
	case preparationdomain.CodePendingReconcile:
		result.OK = false
		result.NextCommand = "agent-harness issueops execution reconcile --id " + snapshot.Record.ID + " --preview ACTOR_FLAGS"
		return result, fmt.Errorf("IssueOps execution has a pending external intent; run %s", result.NextCommand)
	case preparationdomain.CodeModeMismatch:
		result.OK = false
		result.NextCommand = fmt.Sprintf("agent-harness issueops execution switch-mode --id %s --mode %s --json", snapshot.Record.ID, decision.RequestedMode)
		return result, fmt.Errorf("IssueOps execution is already prepared as %s; switching to %s removes the canonical worktree, so run %s", decision.ResolvedMode, decision.RequestedMode, result.NextCommand)
	case preparationdomain.CodeWriterless:
		result.OK = false
		result.NextCommand = writerlessNextCommand(snapshot)
		return result, writerlessError(snapshot.Record, result.NextCommand)
	default:
		return failedResult(command.ID), fmt.Errorf("unsupported preparation decision %q", decision.Code)
	}
}

func normalizeOwnerDefaults(command preparationcontract.Command) preparationcontract.Command {
	command.OwnerHost = strings.ToLower(strings.TrimSpace(command.OwnerHost))
	command.OwnerModel = strings.TrimSpace(command.OwnerModel)
	command.OwnerEffort = strings.TrimSpace(command.OwnerEffort)
	if model, effort, ok := preparationcontract.ImplementerDefaults(command.OwnerHost); ok {
		if command.OwnerModel == "" {
			command.OwnerModel = model
		}
		if command.OwnerEffort == "" {
			command.OwnerEffort = effort
		}
	}
	return command
}

func normalizeActor(actor preparationcontract.Actor) (preparationcontract.Actor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		process.StartedAt = strings.TrimSpace(process.StartedAt)
		process.Executable = strings.TrimSpace(process.Executable)
		actor.SessionProcess = &process
	}
	if actor.Host != "codex" && actor.Host != "claude" {
		return actor, fmt.Errorf("native actor host must be codex or claude")
	}
	if actor.SessionID == "" {
		return actor, fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return actor, fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	return actor, nil
}

func preparedResult(record preparationcontract.Record, requested, fallback string) preparationcontract.Result {
	return preparationcontract.Result{
		OK: true, ID: record.ID, RequestedMode: requested, ResolvedMode: record.Execution.Mode,
		FallbackCode: fallback, Workspace: record.Execution.Workspace, Execution: cloneExecution(record.Execution),
	}
}

func cloneExecution(execution *preparationcontract.Execution) *preparationcontract.Execution {
	if execution == nil {
		return nil
	}
	return preparationcontract.Result{Execution: execution}.Clone().Execution
}

func workspaceResult(request preparationcontract.WorkspaceRequest, driver, linkedAt string) preparationcontract.Workspace {
	return preparationcontract.Workspace{SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch, BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: driver, LinkedAt: linkedAt}
}

func workspaceResultFromReceipt(receipt preparationcontract.WorkspaceReceipt, linkedAt string) preparationcontract.Workspace {
	return preparationcontract.Workspace{SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver, LinkedAt: linkedAt}
}

func writerlessNextCommand(snapshot preparationcontract.Snapshot) string {
	record := snapshot.Record
	execution := record.Execution
	generation := execution.Lease.Generation
	switch execution.Lease.Status {
	case "claimable":
		if execution.Mode == preparationcontract.ModeOrca {
			return "agent-harness issueops execution resume --id " + quoteArg(record.ID) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --confirm"
		}
		return "agent-harness issueops execution claim --id " + quoteArg(record.ID) + " --generation " + strconv.FormatUint(generation, 10) + " --claim-token-file " + quoteArg(snapshot.ClaimTokenPath)
	case "released":
		return "agent-harness issueops execution replace --id " + quoteArg(record.ID) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
	case "revoking":
		return "agent-harness issueops execution replace --id " + quoteArg(record.ID) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --finalize-preview"
	default:
		return ""
	}
}

func writerlessError(record preparationcontract.Record, next string) error {
	lease := record.Execution.Lease
	switch lease.Status {
	case "claimable":
		if record.Execution.Mode == preparationcontract.ModeOrca && (record.Execution.Orca == nil || record.Execution.Orca.LeaseGeneration != lease.Generation) {
			return fmt.Errorf("IssueOps execution is prepared but Orca generation %d has no current owner; run %s", lease.Generation, next)
		}
		return fmt.Errorf("IssueOps execution is prepared but generation %d is claimable and has no writer; run %s", lease.Generation, next)
	case "released":
		return fmt.Errorf("IssueOps execution is prepared but generation %d was released and has no writer; preview resealing with %s", lease.Generation, next)
	case "revoking":
		return fmt.Errorf("IssueOps execution generation %d is revoking and has no writer; finalize the revocation with %s", lease.Generation, next)
	default:
		return errors.New("IssueOps execution has no writer")
	}
}

func quoteArg(value string) string                      { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func formatTime(value time.Time) string                 { return value.UTC().Format(time.RFC3339Nano) }
func failedResult(id string) preparationcontract.Result { return preparationcontract.Result{ID: id} }
