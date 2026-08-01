package harnessapp

import (
	"context"
	"testing"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/port"
)

type publicationProviderFake struct {
	createCalls int
	request     port.IssueProviderCreatePullRequestRequest
}

func (*publicationProviderFake) Name() string { return "github" }
func (*publicationProviderFake) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (f *publicationProviderFake) CreatePullRequest(request port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	f.createCalls++
	f.request = request
	return port.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}, nil
}
func (*publicationProviderFake) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (*publicationProviderFake) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (*publicationProviderFake) CloseIssue(port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, nil
}
func (*publicationProviderFake) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}
func (*publicationProviderFake) ReconcilePullRequest(port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
	return port.IssueProviderReconcilePullRequestResult{}, nil
}

var _ port.IssueProvider = (*publicationProviderFake)(nil)
var _ port.IssueProviderRemoteCreateReconciler = (*publicationProviderFake)(nil)

func TestIssueOpsPublicationCompositionBuildsBothServicesAndCreatesPreview(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	branch := "195-publication-composition"
	record := issueops.IssueOpsRecord{
		OK: true, SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID: issueops.NewIssueOpsID(repo, branch), Repo: repo, Branch: branch, Phase: issueops.IssueOpsPhasePR,
		IssueURL: "https://github.com/acme/repo/issues/195",
		BranchPrepare: &issueops.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/195",
			Branch: branch, BaseBranch: "117-hexagonal-architecture-migration", LinkVerified: true,
		},
		CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z",
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	fake := &publicationProviderFake{}
	resolveCalls := 0
	verifyCalls := 0
	deps := issueOpsPublicationCompositionDeps{
		Resolve: func(provider string) (port.IssueProvider, error) {
			resolveCalls++
			if provider != "github" {
				t.Fatalf("provider=%q", provider)
			}
			return fake, nil
		},
		VerifyLive: func(issueops.IssueOpsRemoteArtifactVerificationRequest) error {
			verifyCalls++
			return nil
		},
		Now:            func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) },
		NewOperationID: func() (string, error) { return "0123456789abcdef0123456789abcdef", nil },
	}
	create, reconcile := newIssueOpsPublicationServices(stateRoot, deps)
	if create == nil || reconcile == nil {
		t.Fatalf("services create=%#v reconcile=%#v", create, reconcile)
	}
	handlers := newIssueOpsPublicationHandlers(deps)
	result, err := handlers.Create(context.Background(), stateRoot, issueops.RemotePullRequestRequest{
		ID: record.ID, Provider: "github", Title: "Publication preview", Body: "Preview body.",
		Head: branch, Base: record.BranchPrepare.BaseBranch, Labels: []string{"enhancement"},
		Assignees: []string{"maintainer"}, Confirm: false,
	})
	if err != nil || result.Preview != "would create pull request" || resolveCalls != 1 || fake.createCalls != 1 || verifyCalls != 0 {
		t.Fatalf("result=%#v resolve=%d create=%d verify=%d err=%v", result, resolveCalls, fake.createCalls, verifyCalls, err)
	}
	if fake.request.ProjectKey != "github.com/acme/repo" || fake.request.HeadBranch != branch || fake.request.Confirm {
		t.Fatalf("provider request=%#v", fake.request)
	}
}

func TestIssueOpsMCPDependenciesIncludeBothPublicationHandlers(t *testing.T) {
	deps := issueOpsMCPDependencies()
	if deps.Publication.Create == nil || deps.Publication.Reconcile == nil {
		t.Fatalf("publication handlers were not composed for MCP: %#v", deps.Publication)
	}
}
