package harnessapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/issueopscli"
	corefacade "agent-harness/internal/adapter/core"
	publicationinbound "agent-harness/internal/adapter/inbound/issueopspublication"
	"agent-harness/internal/adapter/issueops"
	publicationoutbound "agent-harness/internal/adapter/outbound/issueopspublication"
	"agent-harness/internal/adapter/provider"
	publicationapp "agent-harness/internal/application/issueopspublication"
	issueopscontract "agent-harness/internal/contract/issueops"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/port"
)

type issueOpsPublicationCompositionDeps struct {
	Resolve        func(string) (port.IssueProvider, error)
	VerifyLive     issueops.RemoteArtifactVerifyFunc
	Now            func() time.Time
	NewOperationID func() (string, error)
}

func productionIssueOpsPublicationDeps() issueOpsPublicationCompositionDeps {
	return issueOpsPublicationCompositionDeps{
		Resolve: provider.Resolve, VerifyLive: issueopscli.VerifyRemoteArtifactLive, Now: time.Now,
	}
}

func issueOpsPublicationCreateHandler(ctx context.Context, stateRoot string, request issueops.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return newIssueOpsPublicationHandlers(productionIssueOpsPublicationDeps()).Create(ctx, stateRoot, request)
}

func issueOpsPublicationReconcileHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
	return newIssueOpsPublicationHandlers(productionIssueOpsPublicationDeps()).Reconcile(ctx, stateRoot, request)
}

func newIssueOpsPublicationHandlers(deps issueOpsPublicationCompositionDeps) issueops.RemotePublicationHandlers {
	return issueops.RemotePublicationHandlers{
		Create: func(ctx context.Context, stateRoot string, request issueops.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			create, _ := newIssueOpsPublicationServices(stateRoot, deps)
			return publicationinbound.NewCreateHandler(create)(ctx, stateRoot, request)
		},
		Reconcile: func(ctx context.Context, stateRoot string, request issueops.ExecutionReconcileRequest) (issueops.ExecutionReconcileResult, error) {
			_, reconcile := newIssueOpsPublicationServices(stateRoot, deps)
			return publicationinbound.NewReconcileHandler(reconcile)(ctx, stateRoot, request)
		},
	}
}

func newIssueOpsPublicationServices(stateRoot string, deps issueOpsPublicationCompositionDeps) (*publicationapp.CreateService, *publicationapp.ReconcileService) {
	effects := &corePublicationEffects{stateRoot: stateRoot, deps: deps}
	repository := publicationoutbound.NewRepository(effects)
	gateway := publicationoutbound.NewProviderGateway(effects.create, effects.inspect)
	verifier := publicationoutbound.NewVerifier(effects.verifyCandidate, effects.verifyLive)
	return publicationapp.NewCreateService(repository, gateway, verifier), publicationapp.NewReconcileService(repository, gateway, verifier)
}

type corePublicationEffects struct {
	stateRoot string
	deps      issueOpsPublicationCompositionDeps
}

func (e *corePublicationEffects) PreviewCreate(ctx context.Context, command publicationcontract.CreateCommand) (publicationoutbound.EffectState, error) {
	prepared, err := issueops.PrepareRemotePublication(ctx, e.stateRoot, corePublicationRequest(command))
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationPreparedEffectState(prepared), nil
}

func (e *corePublicationEffects) BeginCreate(ctx context.Context, command publicationcontract.CreateCommand) (publicationoutbound.EffectState, error) {
	prepared, err := issueops.PrepareRemotePublication(ctx, e.stateRoot, corePublicationRequest(command))
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	eligibility := publicationEligibility(prepared)
	state, err := issueops.BeginPreparedRemotePublicationIntent(ctx, e.stateRoot, prepared, issueops.RemotePublicationBridgeDependencies{
		Now: e.deps.Now, NewOperationID: e.deps.NewOperationID,
	})
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationIntentEffectState(state, eligibility), nil
}

func (e *corePublicationEffects) LoadIntent(ctx context.Context, id string) (publicationoutbound.EffectState, error) {
	state, err := issueops.LoadRemotePublicationIntent(ctx, e.stateRoot, id)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationIntentEffectState(state, publicationIntentEligibility(state)), nil
}

func (e *corePublicationEffects) MarkRetry(ctx context.Context, state publicationoutbound.EffectState) (publicationoutbound.EffectState, error) {
	coreState, err := corePublicationIntentState(state)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	next, err := issueops.MarkRemotePublicationRetry(ctx, e.stateRoot, coreState)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationIntentEffectState(next, state.Eligibility), nil
}

func (e *corePublicationEffects) RecordFailure(ctx context.Context, state publicationoutbound.EffectState, invocation publicationcontract.InvocationState, knownURL string, cause error) error {
	coreState, err := corePublicationIntentState(state)
	if err != nil {
		return err
	}
	return issueops.RecordRemotePublicationFailure(ctx, e.stateRoot, coreState, string(invocation), knownURL, cause, e.deps.Now)
}

func (e *corePublicationEffects) Complete(ctx context.Context, state publicationoutbound.EffectState, url string, enforceOriginalGeneration bool) (publicationoutbound.EffectState, error) {
	coreState, err := corePublicationIntentState(state)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	next, err := issueops.CompleteRemotePublication(ctx, e.stateRoot, coreState, url, enforceOriginalGeneration, e.deps.Now)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationIntentEffectState(next, state.Eligibility), nil
}

func (e *corePublicationEffects) CompleteNotInvoked(ctx context.Context, state publicationoutbound.EffectState, cause error) (publicationoutbound.EffectState, error) {
	coreState, err := corePublicationIntentState(state)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	next, err := issueops.CompleteRemotePublicationNotInvoked(ctx, e.stateRoot, coreState, cause, e.deps.Now)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationIntentEffectState(next, state.Eligibility), nil
}

func (e *corePublicationEffects) Latest(ctx context.Context, id string) (publicationoutbound.EffectState, error) {
	state, err := issueops.LatestRemotePublication(ctx, e.stateRoot, id)
	if err != nil {
		return publicationoutbound.EffectState{}, err
	}
	return publicationoutbound.EffectState{RecordID: state.Record.ID, RecordRaw: append([]byte(nil), state.RecordRaw...)}, nil
}

func (e *corePublicationEffects) create(_ context.Context, providerName string, request publicationcontract.ProviderCreateRequest) (publicationcontract.ProviderCreateResult, error) {
	if e.deps.Resolve == nil {
		return publicationcontract.ProviderCreateResult{}, fmt.Errorf("publication provider resolver is required")
	}
	resolved, err := e.deps.Resolve(providerName)
	if err != nil {
		return publicationcontract.ProviderCreateResult{}, err
	}
	result, err := corefacade.CreateRemotePullRequest(portPublicationRequest(request), resolved)
	return publicationcontract.ProviderCreateResult{
		OK: result.OK, URL: result.URL, Number: result.Number, Preview: result.Preview,
	}, err
}

func (e *corePublicationEffects) inspect(_ context.Context, intent publicationcontract.Intent) (publicationcontract.Inventory, bool, error) {
	if e.deps.Resolve == nil {
		return publicationcontract.Inventory{}, false, fmt.Errorf("publication provider resolver is required")
	}
	resolved, err := e.deps.Resolve(intent.Provider)
	if err != nil {
		return publicationcontract.Inventory{}, false, err
	}
	result, err := corefacade.ReconcileRemotePullRequest(publicationReconcileRequest(intent.Request), resolved)
	inventory := publicationcontract.Inventory{AuthoritativeZero: result.AuthoritativeZero}
	if result.Candidates != nil {
		inventory.Candidates = make([]publicationcontract.Candidate, len(result.Candidates))
		for index, candidate := range result.Candidates {
			inventory.Candidates[index] = publicationCandidate(candidate)
		}
	}
	return inventory, true, err
}

func (e *corePublicationEffects) verifyCandidate(ctx context.Context, intent publicationcontract.Intent, candidate publicationcontract.Candidate) error {
	state, err := corePublicationIntentState(publicationEffectState(intent))
	if err != nil {
		return err
	}
	return issueops.VerifyRemotePublicationCandidate(ctx, e.stateRoot, state, portPublicationCandidate(candidate))
}

func (e *corePublicationEffects) verifyLive(ctx context.Context, intent publicationcontract.Intent, url string) error {
	state, err := corePublicationIntentState(publicationEffectState(intent))
	if err != nil {
		return err
	}
	return issueops.VerifyRemotePublicationLive(ctx, e.stateRoot, state, url, e.deps.VerifyLive)
}

func corePublicationRequest(command publicationcontract.CreateCommand) issueops.RemotePullRequestRequest {
	return issueops.RemotePullRequestRequest{
		ID: command.ID, Provider: command.Provider, Title: command.Title, Body: command.Body,
		Head: command.Head, Base: command.Base, Labels: clonePublicationStrings(command.Labels),
		Assignees: clonePublicationStrings(command.Assignees), ExpectedGeneration: command.ExpectedGeneration,
		Actor: corePublicationActor(command.Actor), CWD: command.CWD, Confirm: command.Confirm,
	}
}

func corePublicationActor(actor publicationcontract.Actor) issueopscontract.NativeActor {
	result := issueopscontract.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &issueopscontract.NativeProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	if actor.ProcessAncestry != nil {
		result.ProcessAncestry = make([]issueopscontract.NativeProcessReceipt, len(actor.ProcessAncestry))
		for index, receipt := range actor.ProcessAncestry {
			result.ProcessAncestry[index] = issueopscontract.NativeProcessReceipt{
				PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
			}
		}
	}
	return result
}

func publicationPreparedEffectState(prepared issueops.RemotePublicationPreparedState) publicationoutbound.EffectState {
	return publicationoutbound.EffectState{
		RecordID: prepared.Record.ID, Provider: strings.ToLower(strings.TrimSpace(prepared.Provider)), Kind: prepared.Kind,
		Request: publicationRequest(prepared.Request), Eligibility: publicationEligibility(prepared),
	}
}

func publicationIntentEffectState(state issueops.RemotePublicationIntentState, eligibility publicationcontract.CreateEligibility) publicationoutbound.EffectState {
	return publicationoutbound.EffectState{
		RecordID: state.Record.ID, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Generation: state.Generation, Provider: state.Provider, Kind: state.Kind,
		Request: publicationRequest(state.Request), Eligibility: eligibility,
		InvocationState: publicationcontract.InvocationState(state.InvocationState), RetryCount: state.RetryCount, KnownURL: state.KnownURL,
	}
}

func publicationEligibility(prepared issueops.RemotePublicationPreparedState) publicationcontract.CreateEligibility {
	record := prepared.Record
	executionActive := record.Execution != nil && record.Execution.Lease.Status == issueopscontract.LeaseStatusActive
	noPending := record.Execution == nil || record.Execution.Pending == nil
	return publicationcontract.CreateEligibility{
		Provider: strings.ToLower(strings.TrimSpace(prepared.Provider)), Kind: prepared.Kind, Confirm: prepared.Request.Confirm,
		PhasePR: record.Phase == issueopscontract.IssueOpsPhasePR, ExecutionActive: executionActive, NoPending: noPending,
		NoArtifact: record.RemoteArtifact == nil, BranchAuthority: true,
		CanonicalLabelsAssignees: len(prepared.Request.Labels) > 0 && len(prepared.Request.Assignees) > 0,
	}
}

func publicationIntentEligibility(state issueops.RemotePublicationIntentState) publicationcontract.CreateEligibility {
	return publicationcontract.CreateEligibility{
		Provider: strings.ToLower(strings.TrimSpace(state.Provider)), Kind: state.Kind, Confirm: state.Request.Confirm,
		PhasePR:         state.Record.Phase == issueopscontract.IssueOpsPhasePR,
		ExecutionActive: state.Record.Execution != nil && state.Record.Execution.Lease.Status == issueopscontract.LeaseStatusActive,
		NoPending:       false, NoArtifact: state.Record.RemoteArtifact == nil, BranchAuthority: true,
		CanonicalLabelsAssignees: len(state.Request.Labels) > 0 && len(state.Request.Assignees) > 0,
	}
}

func corePublicationIntentState(state publicationoutbound.EffectState) (issueops.RemotePublicationIntentState, error) {
	if len(state.RecordRaw) == 0 {
		return issueops.RemotePublicationIntentState{}, fmt.Errorf("publication record raw bytes are required")
	}
	var record issueopscontract.IssueOpsRecord
	if err := json.Unmarshal(state.RecordRaw, &record); err != nil {
		return issueops.RemotePublicationIntentState{}, fmt.Errorf("decode publication record: %w", err)
	}
	return issueops.RemotePublicationIntentState{
		Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
		OperationID: state.OperationID, Generation: state.Generation, Provider: state.Provider, Kind: state.Kind,
		Request: portPublicationRequest(state.Request), InvocationState: string(state.InvocationState),
		RetryCount: state.RetryCount, KnownURL: state.KnownURL,
	}, nil
}

func publicationEffectState(intent publicationcontract.Intent) publicationoutbound.EffectState {
	return publicationoutbound.EffectState{
		RecordID: intent.Record.ID, RecordRaw: append([]byte(nil), intent.Record.Raw...), IntentRaw: append([]byte(nil), intent.Raw...),
		OperationID: intent.OperationID, Generation: intent.Generation, Provider: intent.Provider, Kind: intent.Kind,
		Request: intent.Request.Clone(), Eligibility: intent.Eligibility, InvocationState: intent.InvocationState,
		RetryCount: intent.RetryCount, KnownURL: intent.KnownURL,
	}
}

func publicationRequest(request port.IssueProviderCreatePullRequestRequest) publicationcontract.ProviderCreateRequest {
	return publicationcontract.ProviderCreateRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func portPublicationRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderCreatePullRequestRequest {
	return port.IssueProviderCreatePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func publicationReconcileRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderReconcilePullRequestRequest {
	sum := sha256.Sum256([]byte(request.Body))
	return port.IssueProviderReconcilePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		ExpectedHeadSHA: request.ExpectedHeadSHA, Title: request.Title, BodySHA256: hex.EncodeToString(sum[:]),
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees), Draft: request.Draft,
	}
}

func publicationCandidate(candidate port.IssueProviderReconcilePullRequestCandidate) publicationcontract.Candidate {
	return publicationcontract.Candidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256,
		Labels: clonePublicationStrings(candidate.Labels), Assignees: clonePublicationStrings(candidate.Assignees),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func portPublicationCandidate(candidate publicationcontract.Candidate) port.IssueProviderReconcilePullRequestCandidate {
	return port.IssueProviderReconcilePullRequestCandidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256,
		Labels: clonePublicationStrings(candidate.Labels), Assignees: clonePublicationStrings(candidate.Assignees),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func clonePublicationStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
