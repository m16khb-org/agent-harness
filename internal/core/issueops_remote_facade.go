package core

import (
	"context"
	"fmt"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/port"
)

// Remote provider boundary for IssueOps.
//
// internal/core depends only on the port.IssueProvider abstraction. The concrete
// github/gitlab providers are selected in the adapter layer
// (internal/adapter/provider.Resolve) and passed in, so core never imports a
// concrete provider adapter and the hexagonal boundary stays intact.

type IssueProviderCreateIssueRequest = port.IssueProviderCreateIssueRequest
type IssueProviderCreateIssueResult = port.IssueProviderCreateIssueResult
type IssueProviderCreatePullRequestRequest = port.IssueProviderCreatePullRequestRequest
type IssueProviderCreatePullRequestResult = port.IssueProviderCreatePullRequestResult
type IssueProviderCreateError = port.IssueProviderCreateError
type IssueProviderReconcilePullRequestRequest = port.IssueProviderReconcilePullRequestRequest
type IssueProviderReconcilePullRequestResult = port.IssueProviderReconcilePullRequestResult
type IssueProviderReconcilePullRequestCandidate = port.IssueProviderReconcilePullRequestCandidate
type IssueProviderCreateChildRequest = port.IssueProviderCreateChildRequest
type IssueProviderCreateChildResult = port.IssueProviderCreateChildResult
type IssueProviderCloseChildRequest = port.IssueProviderCloseChildRequest
type IssueProviderCloseChildResult = port.IssueProviderCloseChildResult
type IssueProvider = port.IssueProvider
type IssueProviderUpdateIssueBodySectionRequest = port.IssueProviderUpdateIssueBodySectionRequest
type IssueProviderUpdateIssueBodySectionResult = port.IssueProviderUpdateIssueBodySectionResult
type IssueOpsRemotePullRequestRequest = issueops.RemotePullRequestRequest
type IssueOpsRemotePullRequestDependencies = issueops.RemotePullRequestDependencies
type IssueOpsRemotePullRequestCreateFunc = issueops.RemotePullRequestCreateFunc
type IssueOpsRemotePullRequestReconcileFunc = issueops.RemotePullRequestReconcileFunc

func SyncRemoteIssueGraph(record IssueOpsRecord) (map[string]any, error) {
	return issueops.SyncRemoteIssueGraph(record)
}

// CreateRemoteIssue creates an issue through the supplied provider. Callers
// resolve the concrete provider (internal/adapter/provider.Resolve) and pass it
// in, keeping core decoupled from github/gitlab.
func CreateRemoteIssue(req IssueProviderCreateIssueRequest, prov IssueProvider) (IssueProviderCreateIssueResult, error) {
	if prov == nil {
		return IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateIssue(req)
}

// ReflectIssueOpsDevilsAdvocateFindings reflects the recorded devil's-advocate
// findings into the linked issue body through the supplied provider, stamping
// IssueReflectedAt on a confirmed success. The caller resolves the provider.
func ReflectIssueOpsDevilsAdvocateFindings(stateRoot, id string, confirm bool, prov IssueProvider) (IssueOpsRecord, IssueProviderUpdateIssueBodySectionResult, error) {
	return issueops.ReflectDevilsAdvocateFindings(stateRoot, id, confirm, prov)
}

func ReflectIssueOpsDevilsAdvocateFindingsWithActor(stateRoot, id string, confirm bool, prov IssueProvider, actor IssueOpsActor) (IssueOpsRecord, IssueProviderUpdateIssueBodySectionResult, error) {
	return issueops.ReflectDevilsAdvocateFindingsWithActor(stateRoot, id, confirm, prov, actor)
}

// CreateRemotePullRequest opens a pull/merge request through the supplied
// provider, resolved by the caller in the adapter layer.
func CreateRemotePullRequest(req IssueProviderCreatePullRequestRequest, prov IssueProvider) (IssueProviderCreatePullRequestResult, error) {
	if prov == nil {
		return IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreatePullRequest(req)
}

func ReconcileRemotePullRequest(req IssueProviderReconcilePullRequestRequest, prov IssueProvider) (IssueProviderReconcilePullRequestResult, error) {
	reconciler, ok := prov.(port.IssueProviderRemoteCreateReconciler)
	if !ok {
		return IssueProviderReconcilePullRequestResult{}, fmt.Errorf("issue provider does not support remote create reconciliation")
	}
	return reconciler.ReconcilePullRequest(req)
}

func CreateIssueOpsRemotePullRequest(ctx context.Context, stateRoot string, req IssueOpsRemotePullRequestRequest, deps IssueOpsRemotePullRequestDependencies) (IssueProviderCreatePullRequestResult, error) {
	return issueops.CreateRemotePullRequest(ctx, stateRoot, req, deps)
}

// CreateRemoteChild creates and verifies a provider-native child work item
// through the supplied provider. State recording remains explicit at the
// caller boundary via LinkIssueOpsChild so dry-runs never mutate state.
func CreateRemoteChild(req IssueProviderCreateChildRequest, prov IssueProvider) (IssueProviderCreateChildResult, error) {
	if prov == nil {
		return IssueProviderCreateChildResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateChild(req)
}

func CloseRemoteChild(req IssueProviderCloseChildRequest, prov IssueProvider) (IssueProviderCloseChildResult, error) {
	if prov == nil {
		return IssueProviderCloseChildResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CloseChild(req)
}

type IssueProviderCloseIssueResult = port.IssueProviderCloseIssueResult
type IssueProviderCompletionSection = port.IssueProviderCompletionSection
type ExecutionIssueSnapshotRequest = port.ExecutionIssueSnapshotRequest
type ExecutionIssueSnapshot = port.ExecutionIssueSnapshot

const IssueBodyCompletionStartMarker = port.IssueBodyCompletionStartMarker

// ReadRemoteIssueSnapshot readback-reads the linked issue through the provider
// when it supports snapshot reads; cleanup finish uses this for fail-closed
// completion/close verification.
func ReadRemoteIssueSnapshot(ctx context.Context, prov IssueProvider, req ExecutionIssueSnapshotRequest) (ExecutionIssueSnapshot, error) {
	reader, ok := prov.(port.ExecutionIssueSnapshotReader)
	if !ok {
		return ExecutionIssueSnapshot{}, fmt.Errorf("issue provider does not support issue snapshot reads")
	}
	return reader.ReadIssueSnapshot(ctx, req)
}

// ReflectIssueOpsCompletion writes the completion section into the linked
// issue body. merged must carry caller-verified provider merge readback.
func ReflectIssueOpsCompletion(stateRoot, id string, merged, confirm bool, prov IssueProvider) (IssueOpsRecord, IssueProviderUpdateIssueBodySectionResult, error) {
	return issueops.ReflectIssueCompletion(stateRoot, id, merged, confirm, prov)
}

// CloseIssueOpsRemoteIssue closes the linked parent issue after caller-verified
// merge evidence, stamping the local completion cache on verified success.
func CloseIssueOpsRemoteIssue(stateRoot, id string, merged, confirm bool, prov IssueProvider) (IssueOpsRecord, IssueProviderCloseIssueResult, error) {
	return issueops.CloseIssueOpsRemoteIssue(stateRoot, id, merged, confirm, prov)
}
