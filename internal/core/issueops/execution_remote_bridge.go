package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/port"
)

type RemotePublicationBridgeDependencies struct {
	Now            func() time.Time
	NewOperationID func() (string, error)
}

type RemotePublicationPreparedState struct {
	Record             issueops.IssueOpsRecord
	Actor              issueops.NativeActor
	CWD                string
	ExpectedGeneration uint64
	Provider           string
	Kind               string
	Request            port.IssueProviderCreatePullRequestRequest
}

type RemotePublicationIntentState struct {
	Record      issueops.IssueOpsRecord
	RecordRaw   []byte
	IntentRaw   []byte
	OperationID string
	Generation  uint64
	Provider    string
	Kind        string
	Request     port.IssueProviderCreatePullRequestRequest

	InvocationState string
	RetryCount      int
	KnownURL        string
}

func PrepareRemotePublication(_ context.Context, stateRoot string, req RemotePullRequestRequest) (RemotePublicationPreparedState, error) {
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return RemotePublicationPreparedState{}, err
		}
		req.Actor = actor
	}
	record, providerReq, kind, err := prepareRemotePullRequest(stateRoot, req)
	if err != nil {
		return RemotePublicationPreparedState{}, err
	}
	return RemotePublicationPreparedState{
		Record: record, Actor: req.Actor, CWD: req.CWD, ExpectedGeneration: req.ExpectedGeneration,
		Provider: req.Provider, Kind: kind, Request: cloneRemotePublicationRequest(providerReq),
	}, nil
}

func BeginPreparedRemotePublicationIntent(ctx context.Context, stateRoot string, prepared RemotePublicationPreparedState, deps RemotePublicationBridgeDependencies) (RemotePublicationIntentState, error) {
	return BeginRemotePublicationIntent(
		ctx, stateRoot, prepared.Record, prepared.Actor, prepared.CWD, prepared.ExpectedGeneration,
		prepared.Request, prepared.Provider, prepared.Kind, deps,
	)
}

func BeginRemotePublicationIntent(
	_ context.Context,
	stateRoot string,
	expected issueops.IssueOpsRecord,
	actor issueops.NativeActor,
	cwd string,
	expectedGeneration uint64,
	providerReq port.IssueProviderCreatePullRequestRequest,
	provider string,
	kind string,
	deps RemotePublicationBridgeDependencies,
) (RemotePublicationIntentState, error) {
	newOperationID := deps.NewOperationID
	if newOperationID == nil {
		newOperationID = newExecutionOperationID
	}
	operationID, err := newOperationID()
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	pending, payload, err := beginRemotePullRequestIntentWithOperationID(
		stateRoot, expected, actor, cwd, expectedGeneration, providerReq, provider, kind, operationID, deps.Now,
	)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return loadRemotePublicationState(stateRoot, pending, payload)
}

func LoadRemotePublicationIntent(_ context.Context, stateRoot, id string) (RemotePublicationIntentState, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.Kind != externalIntentRemotePR {
		return RemotePublicationIntentState{}, fmt.Errorf("remote publication intent is not pending")
	}
	payload, err := readExternalRemotePRPayload(stateRoot, record.Execution.Pending.OperationID)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return loadRemotePublicationState(stateRoot, record, payload)
}

func MarkRemotePublicationRetry(_ context.Context, stateRoot string, state RemotePublicationIntentState) (RemotePublicationIntentState, error) {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	payload, err = markRemotePullRequestRetry(stateRoot, state.Record.ID, payload)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	record, err := ReadIssueOps(stateRoot, state.Record.ID)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return loadRemotePublicationState(stateRoot, record, payload)
}

func RecordRemotePublicationFailure(_ context.Context, stateRoot string, state RemotePublicationIntentState, invocation, knownURL string, cause error, now func() time.Time) error {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return err
	}
	return recordRemotePullRequestFailure(stateRoot, state.Record.ID, payload.OperationID, invocation, payload.RetryCount, knownURL, cause, now)
}

func CompleteRemotePublication(_ context.Context, stateRoot string, state RemotePublicationIntentState, url string, enforceOriginalGeneration bool, now func() time.Time) (RemotePublicationIntentState, error) {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	record, err := finishRemotePullRequestIntent(stateRoot, state.Record.ID, payload, url, enforceOriginalGeneration, now)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return loadCompletedRemotePublicationState(stateRoot, record, payload)
}

func CompleteRemotePublicationNotInvoked(_ context.Context, stateRoot string, state RemotePublicationIntentState, cause error, now func() time.Time) (RemotePublicationIntentState, error) {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	record, err := finishRemotePullRequestPreInvocationFailure(stateRoot, state.Record.ID, payload, cause, now)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return loadCompletedRemotePublicationState(stateRoot, record, payload)
}

func LatestRemotePublication(_ context.Context, stateRoot, id string) (RemotePublicationIntentState, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	recordRaw, err := readRemotePublicationRaw(stateRoot, issueOpsBucket, id)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return RemotePublicationIntentState{Record: record, RecordRaw: recordRaw}, nil
}

func VerifyRemotePublicationCandidate(_ context.Context, stateRoot string, state RemotePublicationIntentState, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return err
	}
	record, err := ReadIssueOps(stateRoot, state.Record.ID)
	if err != nil {
		return err
	}
	return validateRemotePullRequestCandidate(record, payload, candidate)
}

func VerifyRemotePublicationLive(_ context.Context, stateRoot string, state RemotePublicationIntentState, url string, verify RemoteArtifactVerifyFunc) error {
	payload, err := remotePublicationPayload(state)
	if err != nil {
		return err
	}
	record, err := ReadIssueOps(stateRoot, state.Record.ID)
	if err != nil {
		return err
	}
	return verifyRemotePullRequestResult(record, payload, url, verify)
}

func loadRemotePublicationState(stateRoot string, record issueops.IssueOpsRecord, payload externalRemotePRPayload) (RemotePublicationIntentState, error) {
	recordRaw, err := readRemotePublicationRaw(stateRoot, issueOpsBucket, record.ID)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	intentRaw, err := readRemotePublicationRaw(stateRoot, externalIntentBucket, payload.OperationID)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return remotePublicationState(record, recordRaw, intentRaw, payload), nil
}

func loadCompletedRemotePublicationState(stateRoot string, record issueops.IssueOpsRecord, payload externalRemotePRPayload) (RemotePublicationIntentState, error) {
	recordRaw, err := readRemotePublicationRaw(stateRoot, issueOpsBucket, record.ID)
	if err != nil {
		return RemotePublicationIntentState{}, err
	}
	return remotePublicationState(record, recordRaw, nil, payload), nil
}

func remotePublicationState(record issueops.IssueOpsRecord, recordRaw, intentRaw []byte, payload externalRemotePRPayload) RemotePublicationIntentState {
	return RemotePublicationIntentState{
		Record: record, RecordRaw: append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...),
		OperationID: payload.OperationID, Generation: payload.Generation, Provider: payload.Provider, Kind: payload.Kind,
		Request: cloneRemotePublicationRequest(payload.Request), InvocationState: payload.InvocationState,
		RetryCount: payload.RetryCount, KnownURL: payload.KnownURL,
	}
}

func remotePublicationPayload(state RemotePublicationIntentState) (externalRemotePRPayload, error) {
	if len(state.IntentRaw) == 0 {
		return externalRemotePRPayload{}, fmt.Errorf("remote publication intent raw bytes are required")
	}
	var payload externalRemotePRPayload
	if err := json.Unmarshal(state.IntentRaw, &payload); err != nil {
		return externalRemotePRPayload{}, fmt.Errorf("decode remote publication intent: %w", err)
	}
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID == "" || payload.OperationID != state.OperationID || payload.Generation == 0 {
		return externalRemotePRPayload{}, fmt.Errorf("remote publication intent state is invalid")
	}
	return payload, nil
}

func readRemotePublicationRaw(stateRoot, bucket, id string) ([]byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("remote publication raw row %s/%s is missing", bucket, id)
	}
	return append([]byte(nil), raw...), nil
}

func cloneRemotePublicationRequest(request port.IssueProviderCreatePullRequestRequest) port.IssueProviderCreatePullRequestRequest {
	cloned := request
	cloned.Labels = append([]string(nil), request.Labels...)
	cloned.Assignees = append([]string(nil), request.Assignees...)
	return cloned
}
