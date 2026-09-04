package feedbackcleanup

import (
	"context"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func wireAbandonCapture(t *testing.T) (*[]issueopscontract.CleanupAbandonRequest, *int) {
	t.Helper()
	requests := &[]issueopscontract.CleanupAbandonRequest{}
	providerCalls := new(int)
	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	wired.IssueOpsStateRoot = issueopscore.IssueOpsStateRoot
	wired.ReadIssueOps = issueopscore.ReadIssueOps
	wired.ResolveRecordProvider = issueopscore.ResolveRecordProvider
	wired.CleanupAbandon = func(_ context.Context, _ string, req issueopscontract.CleanupAbandonRequest, _ Deps, _ port.IssueProvider) (issueopscontract.CleanupAbandonResult, error) {
		*requests = append(*requests, req)
		return issueopscontract.CleanupAbandonResult{OK: true, ID: req.ID, RemoteEffects: []string{"close_issue"}}, nil
	}
	ConfigureCleanup(wired)
	_ = providerCalls
	return requests, providerCalls
}

// 세 플래그가 요청으로 그대로 전달돼야 어댑터의 게이트가 의미를 갖는다.
func TestRunCleanupAbandonForwardsRemoteEffectFlags(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, false, true)
	requests, providerCalls := wireAbandonCapture(t)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		Provider: func(name string) (port.IssueProvider, error) {
			*providerCalls++
			return &fakeCleanupAbandonProvider{}, nil
		},
	}
	err := RunCleanup([]string{
		"abandon", "--id", record.ID, "--reason", "폐기 검증",
		"--close-pr", "--close-issue", "--delete-remote-branch", "--preview", "--json",
	}, deps)
	if err != nil {
		t.Fatalf("abandon preview: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one abandon request, got %d", len(*requests))
	}
	got := (*requests)[0]
	if !got.ClosePR || !got.CloseIssue || !got.DeleteRemoteBranch {
		t.Fatalf("remote effect flags did not reach the adapter: %#v", got)
	}
	if *providerCalls != 1 {
		t.Fatalf("a remote effect must resolve exactly one provider, got %d", *providerCalls)
	}
}

// 플래그가 없으면 provider를 해석하지 않는다. 원격 정체가 없는 사이클도
// 폐기할 수 있어야 한다.
func TestRunCleanupAbandonWithoutFlagsNeedsNoProvider(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	requests, providerCalls := wireAbandonCapture(t)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		Provider: func(string) (port.IssueProvider, error) {
			*providerCalls++
			return nil, context.DeadlineExceeded
		},
	}
	if err := RunCleanup([]string{"abandon", "--id", record.ID, "--reason", "플래그 없는 폐기", "--preview", "--json"}, deps); err != nil {
		t.Fatalf("abandon preview: %v", err)
	}
	got := (*requests)[0]
	if got.ClosePR || got.CloseIssue || got.DeleteRemoteBranch {
		t.Fatalf("no flag must mean no remote effect: %#v", got)
	}
	if *providerCalls != 0 {
		t.Fatalf("no flag must resolve no provider, got %d", *providerCalls)
	}
}

type fakeCleanupAbandonProvider struct{}

func (fakeCleanupAbandonProvider) Name() string { return "github" }
func (fakeCleanupAbandonProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (fakeCleanupAbandonProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (fakeCleanupAbandonProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (fakeCleanupAbandonProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (fakeCleanupAbandonProvider) CloseIssue(port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, nil
}
func (fakeCleanupAbandonProvider) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}
