package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	publicationapp "agent-harness/internal/application/issueopspublication"
	"agent-harness/internal/contract/issueops"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestRemotePublicationCreateVerticalMatchesLegacy(t *testing.T) {
	tests := []struct {
		name      string
		confirm   bool
		result    port.IssueProviderCreatePullRequestResult
		createErr func() error
		verifyErr func() error
	}{
		{name: "preview", result: port.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}},
		{name: "success", confirm: true, result: port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/195", Number: "195"}},
		{name: "typed pre invocation failure", confirm: true, createErr: func() error {
			return &port.IssueProviderCreateError{Invoked: false, Err: errors.New("preflight rejected")}
		}},
		{name: "ambiguous failure", confirm: true, createErr: func() error { return errors.New("provider outcome is ambiguous") }},
		{name: "empty URL", confirm: true, result: port.IssueProviderCreatePullRequestResult{OK: true}},
		{name: "known URL verification failure", confirm: true, result: port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/195", Number: "195"}, verifyErr: func() error { return errors.New("live verification failed") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seedRoot := t.TempDir()
			fixture := newRemoteExecutionFixture(t, seedRoot, "195-create-differential")
			request := fixture.request("publication differential")
			request.Body = "Exact differential body."
			request.Confirm = test.confirm
			if !test.confirm {
				request.ExpectedGeneration = 0
				request.Actor = issueops.NativeActor{}
				request.CWD = ""
			}
			legacyRoot := clonePublicationRecord(t, seedRoot, fixture.record.ID)
			verticalRoot := clonePublicationRecord(t, seedRoot, fixture.record.ID)

			legacy := observeLegacyPublicationCreate(t, legacyRoot, request, test.result, errorFrom(test.createErr), errorFrom(test.verifyErr))
			vertical := observeVerticalPublicationCreate(t, verticalRoot, request, test.result, errorFrom(test.createErr), errorFrom(test.verifyErr))
			assertPublicationObservationEqual(t, legacy, vertical)
		})
	}
}

func TestRemotePublicationReconcileVerticalMatchesLegacy(t *testing.T) {
	tests := []struct {
		name             string
		seedNotInvoked   bool
		seedKnownURL     bool
		retryExhausted   bool
		inventory        string
		createResult     port.IssueProviderCreatePullRequestResult
		retryCreateError func() error
		verifyErr        func() error
	}{
		{name: "exact candidate adoption", inventory: "exact"},
		{name: "candidate mismatch", inventory: "mismatch"},
		{name: "multiple candidates", inventory: "multiple"},
		{name: "non authoritative zero", inventory: "zero-ambiguous"},
		{name: "authoritative zero unknown", inventory: "zero-authoritative"},
		{name: "known URL verification failure", seedKnownURL: true, inventory: "exact", verifyErr: func() error { return errors.New("live verification failed") }},
		{name: "retry success", seedNotInvoked: true, inventory: "zero-authoritative", createResult: port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/example/agent-harness/pull/195", Number: "195"}},
		{name: "retry terminal not invoked", seedNotInvoked: true, inventory: "zero-authoritative", retryCreateError: func() error {
			return &port.IssueProviderCreateError{Invoked: false, Err: errors.New("retry preflight rejected")}
		}},
		{name: "retry ambiguous", seedNotInvoked: true, inventory: "zero-authoritative", retryCreateError: func() error { return errors.New("retry transport ambiguous") }},
		{name: "retry exhausted", seedNotInvoked: true, retryExhausted: true, inventory: "zero-authoritative"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seedRoot := t.TempDir()
			fixture := newRemoteExecutionFixture(t, seedRoot, "195-reconcile-differential")
			request := fixture.request("publication reconcile differential")
			request.Body = "Exact reconcile differential body."
			seedPublicationIntent(t, seedRoot, request, test.seedNotInvoked, test.seedKnownURL, test.retryExhausted)
			payload := mustRemotePublicationPayload(t, seedRoot, request.ID)
			inventory := differentialInventory(test.inventory, payload)
			legacyRoot := cloneReconcileState(t, seedRoot, request.ID)
			verticalRoot := cloneReconcileState(t, seedRoot, request.ID)

			legacy := observeLegacyPublicationReconcile(t, legacyRoot, request.ID, inventory, test.createResult, errorFrom(test.retryCreateError), errorFrom(test.verifyErr))
			vertical := observeVerticalPublicationReconcile(t, verticalRoot, request.ID, inventory, test.createResult, errorFrom(test.retryCreateError), errorFrom(test.verifyErr))
			assertPublicationObservationEqual(t, legacy, vertical)
		})
	}
}

type publicationObservation struct {
	resultRaw  []byte
	err        string
	errorShape publicationErrorShape
	recordRaw  []byte
	intentRaw  []byte
}

type publicationErrorShape struct {
	ProviderCreate bool
	Invoked        bool
	Cause          string
}

func observeLegacyPublicationCreate(t *testing.T, stateRoot string, request RemotePullRequestRequest, result port.IssueProviderCreatePullRequestResult, createErr, verifyErr error) publicationObservation {
	t.Helper()
	record, providerRequest, kind, err := prepareRemotePullRequest(stateRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	dependencies := legacyRemotePullRequestDependencies{
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return result, createErr
		},
		Verify: func(issueops.IssueOpsRemoteArtifactVerificationRequest) error { return verifyErr },
		Now:    publicationDifferentialClock,
	}
	var got port.IssueProviderCreatePullRequestResult
	if request.Confirm {
		got, err = legacyCreateRemotePullRequestWithOperationID(
			stateRoot, record, request.Actor, request.CWD, request.ExpectedGeneration, providerRequest,
			request.Provider, kind, legacyPublicationOperationID, dependencies,
		)
	} else {
		got, err = dependencies.Create(request.Provider, providerRequest)
	}
	return observePublicationState(t, stateRoot, request.ID, got, err)
}

func observeVerticalPublicationCreate(t *testing.T, stateRoot string, request RemotePullRequestRequest, result port.IssueProviderCreatePullRequestResult, createErr, verifyErr error) publicationObservation {
	t.Helper()
	repository := &publicationDifferentialRepository{stateRoot: stateRoot}
	provider := &publicationDifferentialProvider{
		create: func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return result, createErr
		},
	}
	verifier := &publicationDifferentialVerifier{stateRoot: stateRoot, liveErr: verifyErr}
	got, err := publicationapp.NewCreateService(repository, provider, verifier).Create(context.Background(), publicationCreateCommand(request))
	return observePublicationState(t, stateRoot, request.ID, got, err)
}

func observeLegacyPublicationReconcile(t *testing.T, stateRoot, id string, inventory port.IssueProviderReconcilePullRequestResult, createResult port.IssueProviderCreatePullRequestResult, createErr, verifyErr error) publicationObservation {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	result, reconcileErr := legacyReconcileRemotePullRequest(context.Background(), stateRoot, record, legacyRemotePullRequestDependencies{
		Reconcile: func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			return inventory, nil
		},
		Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return createResult, createErr
		},
		Verify: func(issueops.IssueOpsRemoteArtifactVerificationRequest) error { return verifyErr },
		Now:    publicationDifferentialClock,
	})
	return observePublicationState(t, stateRoot, id, result, reconcileErr)
}

func observeVerticalPublicationReconcile(t *testing.T, stateRoot, id string, inventory port.IssueProviderReconcilePullRequestResult, createResult port.IssueProviderCreatePullRequestResult, createErr, verifyErr error) publicationObservation {
	t.Helper()
	snapshot, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	repository := &publicationDifferentialRepository{stateRoot: stateRoot}
	provider := &publicationDifferentialProvider{
		create: func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return createResult, createErr
		},
		inspect: func(port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
			return inventory, nil
		},
	}
	verifier := &publicationDifferentialVerifier{stateRoot: stateRoot, liveErr: verifyErr}
	result, reconcileErr := publicationapp.NewReconcileService(repository, provider, verifier).Reconcile(context.Background(), id)
	public := publicationPublicReconcileResult(t, id, snapshot, result, reconcileErr)
	return observePublicationState(t, stateRoot, id, public, reconcileErr)
}

func observePublicationState(t *testing.T, stateRoot, id string, result any, err error) publicationObservation {
	t.Helper()
	resultRaw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	record, readErr := ReadIssueOps(stateRoot, id)
	if readErr != nil {
		t.Fatal(readErr)
	}
	observation := publicationObservation{resultRaw: resultRaw, recordRaw: rawIssueOpsRow(t, stateRoot, id)}
	if err != nil {
		observation.err = err.Error()
		var typed *port.IssueProviderCreateError
		if errors.As(err, &typed) {
			observation.errorShape.ProviderCreate = true
			observation.errorShape.Invoked = typed.Invoked
			if typed.Err != nil {
				observation.errorShape.Cause = typed.Err.Error()
			}
		}
	}
	if record.Execution != nil && record.Execution.Pending != nil {
		observation.intentRaw = readLegacyPublicationRawRowIfPresent(t, stateRoot, externalIntentBucket, record.Execution.Pending.OperationID)
	}
	return observation
}

func assertPublicationObservationEqual(t *testing.T, legacy, vertical publicationObservation) {
	t.Helper()
	if !bytes.Equal(legacy.resultRaw, vertical.resultRaw) || legacy.err != vertical.err ||
		legacy.errorShape != vertical.errorShape || !bytes.Equal(legacy.recordRaw, vertical.recordRaw) || !bytes.Equal(legacy.intentRaw, vertical.intentRaw) {
		t.Fatalf("publication differential mismatch\nlegacy result=%s err=%q shape=%+v record=%x intent=%x\nvertical result=%s err=%q shape=%+v record=%x intent=%x",
			legacy.resultRaw, legacy.err, legacy.errorShape, legacy.recordRaw, legacy.intentRaw,
			vertical.resultRaw, vertical.err, vertical.errorShape, vertical.recordRaw, vertical.intentRaw)
	}
}

type publicationDifferentialRepository struct{ stateRoot string }

func (r *publicationDifferentialRepository) PreviewCreate(ctx context.Context, command publicationcontract.CreateCommand) (publicationcontract.PreparedCreate, error) {
	prepared, err := PrepareRemotePublication(ctx, r.stateRoot, publicationCoreRequest(command))
	return publicationcontract.PreparedCreate{Request: publicationContractRequest(prepared.Request), Eligibility: publicationCreateEligibility(prepared)}, err
}

func (r *publicationDifferentialRepository) BeginCreate(ctx context.Context, command publicationcontract.CreateCommand) (publicationcontract.Intent, error) {
	prepared, err := PrepareRemotePublication(ctx, r.stateRoot, publicationCoreRequest(command))
	if err != nil {
		return publicationcontract.Intent{}, err
	}
	eligibility := publicationCreateEligibility(prepared)
	state, err := BeginPreparedRemotePublicationIntent(ctx, r.stateRoot, prepared, RemotePublicationBridgeDependencies{
		Now: publicationDifferentialClock, NewOperationID: func() (string, error) { return legacyPublicationOperationID, nil },
	})
	return publicationContractIntent(state, eligibility), err
}

func (r *publicationDifferentialRepository) LoadIntent(ctx context.Context, id string) (publicationcontract.Intent, error) {
	state, err := LoadRemotePublicationIntent(ctx, r.stateRoot, id)
	return publicationContractIntent(state, publicationcontract.CreateEligibility{}), err
}

func (r *publicationDifferentialRepository) MarkRetry(ctx context.Context, intent publicationcontract.Intent) (publicationcontract.Intent, error) {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return publicationcontract.Intent{}, err
	}
	state, err = MarkRemotePublicationRetry(ctx, r.stateRoot, state)
	return publicationContractIntent(state, intent.Eligibility), err
}

func (r *publicationDifferentialRepository) RecordFailure(ctx context.Context, intent publicationcontract.Intent, invocation publicationcontract.InvocationState, knownURL string, cause error) error {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return err
	}
	return RecordRemotePublicationFailure(ctx, r.stateRoot, state, string(invocation), knownURL, cause, publicationDifferentialClock)
}

func (r *publicationDifferentialRepository) Complete(ctx context.Context, intent publicationcontract.Intent, url string, enforceOriginalGeneration bool) (publicationcontract.RecordSnapshot, error) {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return publicationcontract.RecordSnapshot{}, err
	}
	state, err = CompleteRemotePublication(ctx, r.stateRoot, state, url, enforceOriginalGeneration, publicationDifferentialClock)
	return publicationcontract.RecordSnapshot{ID: state.Record.ID, Raw: append([]byte(nil), state.RecordRaw...)}, err
}

func (r *publicationDifferentialRepository) CompleteNotInvoked(ctx context.Context, intent publicationcontract.Intent, cause error) (publicationcontract.RecordSnapshot, error) {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return publicationcontract.RecordSnapshot{}, err
	}
	state, err = CompleteRemotePublicationNotInvoked(ctx, r.stateRoot, state, cause, publicationDifferentialClock)
	return publicationcontract.RecordSnapshot{ID: state.Record.ID, Raw: append([]byte(nil), state.RecordRaw...)}, err
}

func (r *publicationDifferentialRepository) Latest(ctx context.Context, id string) (publicationcontract.RecordSnapshot, error) {
	state, err := LatestRemotePublication(ctx, r.stateRoot, id)
	return publicationcontract.RecordSnapshot{ID: state.Record.ID, Raw: append([]byte(nil), state.RecordRaw...)}, err
}

type publicationDifferentialProvider struct {
	create  func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
	inspect func(port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error)
}

func (p *publicationDifferentialProvider) Create(_ context.Context, _ string, request publicationcontract.ProviderCreateRequest) (publicationcontract.ProviderCreateResult, publicationcontract.InvocationState, error) {
	result, err := p.create(publicationPortRequest(request))
	invocation := publicationcontract.InvocationUnknown
	var typed *port.IssueProviderCreateError
	if errors.As(err, &typed) && !typed.Invoked {
		invocation = publicationcontract.InvocationNotInvokedProven
	}
	return publicationcontract.ProviderCreateResult{OK: result.OK, URL: result.URL, Number: result.Number, Preview: result.Preview}, invocation, err
}

func (p *publicationDifferentialProvider) Inspect(_ context.Context, intent publicationcontract.Intent) (publicationcontract.Inventory, bool, error) {
	result, err := p.inspect(publicationPortReconcileRequest(intent.Request))
	inventory := publicationcontract.Inventory{AuthoritativeZero: result.AuthoritativeZero}
	for _, candidate := range result.Candidates {
		inventory.Candidates = append(inventory.Candidates, publicationContractCandidate(candidate))
	}
	return inventory, true, err
}

type publicationDifferentialVerifier struct {
	stateRoot string
	liveErr   error
}

func (v *publicationDifferentialVerifier) VerifyCandidate(ctx context.Context, intent publicationcontract.Intent, candidate publicationcontract.Candidate) error {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return err
	}
	return VerifyRemotePublicationCandidate(ctx, v.stateRoot, state, publicationPortCandidate(candidate))
}

func (v *publicationDifferentialVerifier) VerifyLive(ctx context.Context, intent publicationcontract.Intent, url string) error {
	state, err := publicationCoreIntent(intent)
	if err != nil {
		return err
	}
	return VerifyRemotePublicationLive(ctx, v.stateRoot, state, url, func(issueops.IssueOpsRemoteArtifactVerificationRequest) error { return v.liveErr })
}

func publicationCreateCommand(request RemotePullRequestRequest) publicationcontract.CreateCommand {
	command := publicationcontract.CreateCommand{
		ID: request.ID, Provider: request.Provider, Title: request.Title, Body: request.Body,
		Head: request.Head, Base: request.Base, Labels: append([]string(nil), request.Labels...), Assignees: append([]string(nil), request.Assignees...),
		ExpectedGeneration: request.ExpectedGeneration, CWD: request.CWD, Confirm: request.Confirm,
		Actor: publicationcontract.Actor{Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID},
	}
	if request.Actor.SessionProcess != nil {
		command.Actor.SessionProcess = &publicationcontract.ProcessReceipt{PID: request.Actor.SessionProcess.PID, StartedAt: request.Actor.SessionProcess.StartedAt, Executable: request.Actor.SessionProcess.Executable}
	}
	for _, receipt := range request.Actor.ProcessAncestry {
		command.Actor.ProcessAncestry = append(command.Actor.ProcessAncestry, publicationcontract.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return command
}

func publicationCoreRequest(command publicationcontract.CreateCommand) RemotePullRequestRequest {
	request := RemotePullRequestRequest{
		ID: command.ID, Provider: command.Provider, Title: command.Title, Body: command.Body,
		Head: command.Head, Base: command.Base, Labels: append([]string(nil), command.Labels...), Assignees: append([]string(nil), command.Assignees...),
		ExpectedGeneration: command.ExpectedGeneration, CWD: command.CWD, Confirm: command.Confirm,
		Actor: issueops.NativeActor{Host: command.Actor.Host, SessionID: command.Actor.SessionID, AgentID: command.Actor.AgentID},
	}
	if command.Actor.SessionProcess != nil {
		request.Actor.SessionProcess = &issueops.NativeProcessReceipt{PID: command.Actor.SessionProcess.PID, StartedAt: command.Actor.SessionProcess.StartedAt, Executable: command.Actor.SessionProcess.Executable}
	}
	for _, receipt := range command.Actor.ProcessAncestry {
		request.Actor.ProcessAncestry = append(request.Actor.ProcessAncestry, issueops.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return request
}

func publicationCreateEligibility(prepared RemotePublicationPreparedState) publicationcontract.CreateEligibility {
	record := prepared.Record
	return publicationcontract.CreateEligibility{
		Provider: prepared.Provider, Kind: prepared.Kind, Confirm: prepared.Request.Confirm,
		PhasePR:         record.Phase == issueops.IssueOpsPhasePR,
		ExecutionActive: record.Execution != nil && record.Execution.Lease.Status == issueops.LeaseStatusActive,
		NoPending:       record.Execution == nil || record.Execution.Pending == nil,
		NoArtifact:      record.RemoteArtifact == nil, BranchAuthority: true,
		CanonicalLabelsAssignees: len(prepared.Request.Labels) > 0 && len(prepared.Request.Assignees) > 0,
	}
}

func publicationContractIntent(state RemotePublicationIntentState, eligibility publicationcontract.CreateEligibility) publicationcontract.Intent {
	return publicationcontract.Intent{
		Record:      publicationcontract.RecordSnapshot{ID: state.Record.ID, Raw: append([]byte(nil), state.RecordRaw...)},
		OperationID: state.OperationID, Generation: state.Generation, Provider: state.Provider, Kind: state.Kind,
		Request: publicationContractRequest(state.Request), InvocationState: publicationcontract.InvocationState(state.InvocationState),
		RetryCount: state.RetryCount, KnownURL: state.KnownURL, Eligibility: eligibility, Raw: append([]byte(nil), state.IntentRaw...),
	}
}

func publicationCoreIntent(intent publicationcontract.Intent) (RemotePublicationIntentState, error) {
	var record issueops.IssueOpsRecord
	if err := json.Unmarshal(intent.Record.Raw, &record); err != nil {
		return RemotePublicationIntentState{}, err
	}
	return RemotePublicationIntentState{
		Record: record, RecordRaw: append([]byte(nil), intent.Record.Raw...), IntentRaw: append([]byte(nil), intent.Raw...),
		OperationID: intent.OperationID, Generation: intent.Generation, Provider: intent.Provider, Kind: intent.Kind,
		Request: publicationPortRequest(intent.Request), InvocationState: string(intent.InvocationState), RetryCount: intent.RetryCount, KnownURL: intent.KnownURL,
	}, nil
}

func publicationContractRequest(request port.IssueProviderCreatePullRequestRequest) publicationcontract.ProviderCreateRequest {
	return publicationcontract.ProviderCreateRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch, Labels: append([]string(nil), request.Labels...), Assignees: append([]string(nil), request.Assignees...),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func publicationPortRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderCreatePullRequestRequest {
	return port.IssueProviderCreatePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch, Labels: append([]string(nil), request.Labels...), Assignees: append([]string(nil), request.Assignees...),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func publicationPortReconcileRequest(request publicationcontract.ProviderCreateRequest) port.IssueProviderReconcilePullRequestRequest {
	sum := sha256.Sum256([]byte(request.Body))
	return port.IssueProviderReconcilePullRequestRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		ExpectedHeadSHA: request.ExpectedHeadSHA, Title: request.Title, BodySHA256: hex.EncodeToString(sum[:]),
		Labels: append([]string(nil), request.Labels...), Assignees: append([]string(nil), request.Assignees...), Draft: request.Draft,
	}
}

func publicationContractCandidate(candidate port.IssueProviderReconcilePullRequestCandidate) publicationcontract.Candidate {
	return publicationcontract.Candidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256, Labels: append([]string(nil), candidate.Labels...), Assignees: append([]string(nil), candidate.Assignees...),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func publicationPortCandidate(candidate publicationcontract.Candidate) port.IssueProviderReconcilePullRequestCandidate {
	return port.IssueProviderReconcilePullRequestCandidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256, Labels: append([]string(nil), candidate.Labels...), Assignees: append([]string(nil), candidate.Assignees...),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func publicationPublicReconcileResult(t *testing.T, id string, snapshot issueops.IssueOpsRecord, result publicationcontract.ReconcileResult, serviceErr error) ExecutionReconcileResult {
	t.Helper()
	public := ExecutionReconcileResult{OK: serviceErr == nil || result.Reconciled, ID: id, Reconciled: result.Reconciled, Code: result.Code, ExternalStateInspected: result.ExternalStateInspected}
	var record issueops.IssueOpsRecord
	if serviceErr == nil || result.Reconciled {
		if len(result.Record.Raw) == 0 {
			return public
		}
		if err := json.Unmarshal(result.Record.Raw, &record); err != nil {
			t.Fatal(err)
		}
	} else {
		record = snapshot
	}
	public.ID = record.ID
	if record.Execution != nil {
		public.Execution = *record.Execution
		public.Pending = record.Execution.Pending
	}
	return public
}

func seedPublicationIntent(t *testing.T, stateRoot string, request RemotePullRequestRequest, notInvoked, knownURL, retryExhausted bool) {
	t.Helper()
	record, providerRequest, kind, err := prepareRemotePullRequest(stateRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	result := port.IssueProviderCreatePullRequestResult{}
	createErr := errors.New("provider outcome is ambiguous")
	if notInvoked {
		createErr = &port.IssueProviderCreateError{Invoked: false, Err: errors.New("preflight rejected")}
	}
	if knownURL {
		result.URL = "https://github.com/example/agent-harness/pull/195"
	}
	_, err = legacyCreateRemotePullRequestWithOperationID(
		stateRoot, record, request.Actor, request.CWD, request.ExpectedGeneration, providerRequest,
		request.Provider, kind, legacyPublicationOperationID,
		legacyRemotePullRequestDependencies{Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return result, createErr
		}, Now: publicationDifferentialClock},
	)
	if err == nil {
		t.Fatal("seed publication create unexpectedly succeeded")
	}
	if retryExhausted {
		state, err := LoadRemotePublicationIntent(context.Background(), stateRoot, request.ID)
		if err != nil {
			t.Fatal(err)
		}
		state, err = MarkRemotePublicationRetry(context.Background(), stateRoot, state)
		if err != nil {
			t.Fatal(err)
		}
		if err := RecordRemotePublicationFailure(context.Background(), stateRoot, state, remoteInvocationNotInvoked, state.KnownURL, errors.New("retry was not invoked"), publicationDifferentialClock); err != nil {
			t.Fatal(err)
		}
	}
}

func differentialInventory(kind string, payload externalRemotePRPayload) port.IssueProviderReconcilePullRequestResult {
	expected := remotePullRequestReconcileRequest(payload)
	candidate := port.IssueProviderReconcilePullRequestCandidate{
		URL: "https://github.com/example/agent-harness/pull/195", ProjectKey: expected.ProjectKey, SourceProjectKey: expected.ProjectKey,
		HeadBranch: expected.HeadBranch, BaseBranch: expected.BaseBranch, HeadSHA: expected.ExpectedHeadSHA,
		Title: expected.Title, BodySHA256: expected.BodySHA256, Labels: expected.Labels, Assignees: expected.Assignees, Draft: expected.Draft,
	}
	switch kind {
	case "exact":
		return port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{candidate}}
	case "mismatch":
		candidate.Title += " drift"
		return port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{candidate}}
	case "multiple":
		second := candidate
		second.URL = "https://github.com/example/agent-harness/pull/196"
		return port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{candidate, second}}
	case "zero-authoritative":
		return port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}
	default:
		return port.IssueProviderReconcilePullRequestResult{}
	}
}

func mustRemotePublicationPayload(t *testing.T, stateRoot, id string) externalRemotePRPayload {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil || record.Execution == nil || record.Execution.Pending == nil {
		t.Fatalf("pending publication intent missing: record=%#v err=%v", record, err)
	}
	payload, err := readExternalRemotePRPayload(stateRoot, record.Execution.Pending.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func clonePublicationRecord(t *testing.T, stateRoot, id string) string {
	t.Helper()
	source, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := source.Get(issueOpsBucket, id)
	if err != nil || !ok {
		t.Fatalf("read publication record ok=%t err=%v", ok, err)
	}
	destinationRoot := t.TempDir()
	destination, err := sqlstore.Open(destinationRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.Put(issueOpsBucket, id, raw); err != nil {
		t.Fatal(err)
	}
	return destinationRoot
}

func errorFrom(factory func() error) error {
	if factory == nil {
		return nil
	}
	return factory()
}

func publicationDifferentialClock() time.Time {
	return time.Date(2026, time.August, 1, 16, 0, 0, 123, time.UTC)
}
