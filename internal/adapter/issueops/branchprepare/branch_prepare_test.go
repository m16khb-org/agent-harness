package branchprepare

import (
	"fmt"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestPrepareRecordsProviderFallbackOrder(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-1",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "123-provider-linked-branch",
		IssueURL: "https://gitlab.example/group/project/-/issues/123",
	})

	record, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-1", model.IssueOpsBranchPrepareRequest{
		Provider:        "gitlab",
		IssueURL:        "https://gitlab.example/group/project/-/issues/123",
		Branch:          "123-provider-linked-branch",
		BaseBranch:      "main",
		BaseSHA:         "abc123",
		LinkVerified:    true,
		RemoteBranchURL: "https://gitlab.example/group/project/-/tree/123-provider-linked-branch",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Branch != "123-provider-linked-branch" || record.BranchPrepare == nil {
		t.Fatalf("branch prepare should update record branch and state: %+v", record)
	}
	prepare := record.BranchPrepare
	if prepare.Provider != "gitlab" || prepare.BaseBranch != "main" || prepare.BaseSHA != "abc123" || !prepare.LinkVerified {
		t.Fatalf("unexpected branch prepare metadata: %+v", prepare)
	}
	if len(prepare.Steps) != 3 {
		t.Fatalf("expected mcp, fallback, failure steps: %+v", prepare.Steps)
	}
	if prepare.Steps[0].Strategy != "mcp" || prepare.Steps[0].Tool != "mcp__glab.glab_api" {
		t.Fatalf("first step must use GitLab MCP API: %+v", prepare.Steps[0])
	}
	if prepare.Steps[1].Strategy != "fallback_api" || len(prepare.Steps[1].Command) == 0 || prepare.Steps[1].Command[0] != "glab" {
		t.Fatalf("second step must be glab API fallback: %+v", prepare.Steps[1])
	}
	if prepare.Steps[2].Strategy != "fail" {
		t.Fatalf("third step must fail closed: %+v", prepare.Steps[2])
	}
}

func TestPrepareUsesGitHubDevelopFallback(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-2",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "456-provider-linked-branch",
		IssueURL: "https://github.com/example/repo/issues/456",
	})

	record, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-2", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/456",
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	prepare := record.BranchPrepare
	// #306: base SHA 없는 경로도 생성 뒤 두 readback 단계를 갖는다.
	if prepare == nil || len(prepare.Steps) != 5 {
		t.Fatalf("expected github branch prepare steps: %+v", record)
	}
	if prepare.Steps[0].Strategy != "mcp_unavailable" {
		t.Fatalf("github MCP branch creation is not currently exposed and must be explicit: %+v", prepare.Steps[0])
	}
	if prepare.Steps[1].Strategy != "fallback_api" || len(prepare.Steps[1].Command) < 2 || prepare.Steps[1].Command[0] != "gh" || prepare.Steps[1].Command[1] != "issue" {
		t.Fatalf("github fallback must use gh issue develop: %+v", prepare.Steps[1])
	}
}

func TestPreparePersistsExplicitParentWorktree(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-4",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "456-provider-linked-branch",
		IssueURL: "https://github.com/example/repo/issues/456",
	})

	record, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-4", model.IssueOpsBranchPrepareRequest{
		Provider:       "github",
		IssueURL:       "https://github.com/example/repo/issues/456",
		Branch:         "456-provider-linked-branch",
		BaseBranch:     "117-umbrella",
		ParentWorktree: "/repo/example.worktrees/117-umbrella",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := record.BranchPrepare.ParentWorktree; got != "/repo/example.worktrees/117-umbrella" {
		t.Fatalf("parent worktree = %q", got)
	}
}

func TestPrepareRejectsRelativeParentWorktree(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-5",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "456-provider-linked-branch",
		IssueURL: "https://github.com/example/repo/issues/456",
	})

	_, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-5", model.IssueOpsBranchPrepareRequest{
		Provider:       "github",
		IssueURL:       "https://github.com/example/repo/issues/456",
		Branch:         "456-provider-linked-branch",
		BaseBranch:     "117-umbrella",
		ParentWorktree: "../117-umbrella",
	})
	if err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("relative parent worktree must fail closed: %v", err)
	}
}

func TestPrepareRejectsUnlinkedGitLabBranchName(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-3",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "123-provider-linked-branch",
		IssueURL: "https://gitlab.example/group/project/-/issues/123",
	})

	_, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-3", model.IssueOpsBranchPrepareRequest{
		Provider:   "gitlab",
		IssueURL:   "https://gitlab.example/group/project/-/issues/123",
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "123-") {
		t.Fatalf("expected GitLab issue prefix rejection, got %v", err)
	}
}

func TestPrepareRejectsBaseSHAThatDoesNotResolveToCommit(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-invalid-base",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "123-invalid-base",
		IssueURL: "https://github.com/example/repo/issues/123",
	})
	deps := store.issueOpsStore()
	deps.ResolveBaseCommit = func(repo, revision string) (string, error) {
		if repo != "/repo/example" || revision != "deadbeef" {
			t.Fatalf("unexpected base resolution request: repo=%q revision=%q", repo, revision)
		}
		return "", fmt.Errorf("commit does not exist")
	}

	_, err := Prepare(deps, t.TempDir(), "io-invalid-base", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/123",
		Branch:     "123-invalid-base",
		BaseBranch: "main",
		BaseSHA:    "deadbeef",
	})
	if err == nil || !strings.Contains(err.Error(), "base_sha") || !strings.Contains(err.Error(), "commit does not exist") {
		t.Fatalf("unresolvable base_sha must fail closed: %v", err)
	}
	if store.record.BranchPrepare != nil {
		t.Fatalf("failed base resolution must not mutate the record: %+v", store.record.BranchPrepare)
	}
}

type branchPrepareTestStore struct {
	record model.IssueOpsRecord
}

func newBranchPrepareTestStore(record model.IssueOpsRecord) *branchPrepareTestStore {
	return &branchPrepareTestStore{record: record}
}

func (s *branchPrepareTestStore) issueOpsStore() Store {
	return Store{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return s.record, nil
		},
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			s.record = record
			return record, nil
		},
		ValidateIssueURL: func(issueURL string) error {
			if strings.TrimSpace(issueURL) == "" {
				return fmt.Errorf("issue_url is required")
			}
			return nil
		},
		ResolveBaseCommit: func(_ string, revision string) (string, error) {
			return revision, nil
		},
	}
}

func TestValidateBranchErrorUsesNeutralExamples(t *testing.T) {
	err := ValidateBranch("dogfood-lifecycle")
	if err == nil {
		t.Fatal("expected branch without issue-number prefix to be rejected")
	}
	if !strings.Contains(err.Error(), "issue number followed by a hyphen") {
		t.Fatalf("expected rule text, got %v", err)
	}
	// 안내 문구의 예시는 이 저장소가 아닌 외부 프로젝트 이슈를 참조해선 안 된다.
	// dogfooding 중 실제로 혼란을 일으킨 회귀 방지(#dogfood-whoami-record-flags).
	for _, foreign := range []string{"fanza", "grpc-ai"} {
		if strings.Contains(err.Error(), foreign) {
			t.Fatalf("branch error must not reference unrelated projects: %v", err)
		}
	}
}
