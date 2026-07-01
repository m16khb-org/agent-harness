package cleanupchildren

import (
	"fmt"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestCloseChildrenRequiresMergeEvidence(t *testing.T) {
	store := newCloseChildrenStoreForTest(model.IssueOpsRecord{
		ID:       "io-close-children",
		IssueURL: "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueOpsIssueLink{{
			Type:     "child",
			URL:      "https://github.com/acme/repo/issues/2",
			Provider: "github",
		}},
	})

	result, err := ByID(store.store(), t.TempDir(), "io-close-children", model.IssueOpsCloseChildrenRequest{Merged: false})
	if err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("expected merge evidence failure, got result=%+v err=%v", result, err)
	}
	if len(store.provider.calls) != 0 {
		t.Fatalf("provider must not be called without merge evidence: %+v", store.provider.calls)
	}
}

func TestCloseChildrenDryRunDoesNotMutateState(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-close-children",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueOpsIssueLink{{
			Type:     "child",
			URL:      "https://github.com/acme/repo/issues/2",
			Provider: "github",
		}},
	}
	store := newCloseChildrenStoreForTest(record)

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{Merged: true, Confirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.ClosedCount != 0 || len(result.Children) != 1 {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if len(store.provider.calls) != 1 || store.provider.calls[0].Confirm {
		t.Fatalf("dry-run should call provider preview only: %+v", store.provider.calls)
	}
	if got := store.records[record.ID].IssueLinks[0].CloseVerifiedAt; got != "" {
		t.Fatalf("dry-run mutated close evidence: %+v", store.records[record.ID].IssueLinks[0])
	}
}

func TestCloseChildrenConfirmRecordsVerifiedEvidence(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-close-children",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/1",
		IssueLinks: []model.IssueOpsIssueLink{{
			Type:      "child",
			URL:       "https://github.com/acme/repo/issues/2",
			Provider:  "github",
			CreatedAt: "2026-06-17T00:00:00Z",
		}},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.result = port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          "https://github.com/acme/repo/issues/2",
		HierarchyVerified: true,
		Closed:            true,
		State:             "closed",
	}

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{Merged: true, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.DryRun || result.ClosedCount != 1 {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
	link := store.records[record.ID].IssueLinks[0]
	if link.ClosedAt == "" || link.CloseVerifiedAt == "" || link.CloseReason != "completed" {
		t.Fatalf("verified close evidence not recorded: %+v", link)
	}
}

type closeChildrenStoreForTest struct {
	records  map[string]model.IssueOpsRecord
	provider *fakeCloseChildProvider
}

func newCloseChildrenStoreForTest(records ...model.IssueOpsRecord) *closeChildrenStoreForTest {
	store := &closeChildrenStoreForTest{
		records:  map[string]model.IssueOpsRecord{},
		provider: &fakeCloseChildProvider{result: port.IssueProviderCloseChildResult{OK: true, Provider: "github", HierarchyVerified: true, Closed: true, State: "closed"}},
	}
	for _, record := range records {
		store.records[record.ID] = record
	}
	return store
}

func (s *closeChildrenStoreForTest) store() Store {
	return Store{
		Read:       s.read,
		TouchWrite: s.touchWrite,
		Provider: func(string) (port.IssueProvider, error) {
			return s.provider, nil
		},
	}
}

func (s *closeChildrenStoreForTest) read(_ string, id string) (model.IssueOpsRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return model.IssueOpsRecord{}, fmt.Errorf("missing record")
	}
	return record, nil
}

func (s *closeChildrenStoreForTest) touchWrite(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	s.records[record.ID] = record
	return record, nil
}

type fakeCloseChildProvider struct {
	result port.IssueProviderCloseChildResult
	calls  []port.IssueProviderCloseChildRequest
}

func (p *fakeCloseChildProvider) Name() string { return "github" }

func (p *fakeCloseChildProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}

func (p *fakeCloseChildProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}

func (p *fakeCloseChildProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}

func (p *fakeCloseChildProvider) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}

func (p *fakeCloseChildProvider) CloseChild(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	p.calls = append(p.calls, req)
	result := p.result
	result.ChildURL = req.ChildURL
	if !req.Confirm {
		result.Closed = false
		result.Preview = "[dry-run] close " + req.ChildURL
	}
	return result, nil
}
