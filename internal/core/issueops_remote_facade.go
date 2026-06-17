package core

import (
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
type IssueProviderCreateChildRequest = port.IssueProviderCreateChildRequest
type IssueProviderCreateChildResult = port.IssueProviderCreateChildResult
type IssueProvider = port.IssueProvider

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

// CreateRemotePullRequest opens a pull/merge request through the supplied
// provider, resolved by the caller in the adapter layer.
func CreateRemotePullRequest(req IssueProviderCreatePullRequestRequest, prov IssueProvider) (IssueProviderCreatePullRequestResult, error) {
	if prov == nil {
		return IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreatePullRequest(req)
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
