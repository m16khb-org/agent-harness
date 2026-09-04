package branchprepare

import (
	"fmt"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func codeProjectStore(t *testing.T, observed string, observeErr error) (Store, *model.IssueOpsRecord) {
	t.Helper()
	stored := &model.IssueOpsRecord{
		OK: true, ID: "io-cross", Repo: t.TempDir(),
		IssueURL: "https://gitlab.example.com/planning/backlog/-/issues/42",
	}
	store := Store{
		Read: func(string, string) (model.IssueOpsRecord, error) { return *stored, nil },
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			*stored = record
			return record, nil
		},
		ValidateIssueURL: func(string) error { return nil },
	}
	if observed != "" || observeErr != nil {
		store.ObserveCodeProjectKey = func(string, string) (string, error) { return observed, observeErr }
	}
	return store, stored
}

func crossProjectRequest(codeProjectKey string) model.IssueOpsBranchPrepareRequest {
	return model.IssueOpsBranchPrepareRequest{
		Provider:       "gitlab",
		IssueURL:       "https://gitlab.example.com/planning/backlog/-/issues/42",
		Branch:         "42-cross",
		BaseBranch:     "main",
		CodeProjectKey: codeProjectKey,
	}
}

func TestPrepareSealsDeclaredCodeProjectKey(t *testing.T) {
	store, stored := codeProjectStore(t, "", nil)
	if _, err := Prepare(store, "state", "io-cross", crossProjectRequest("gitlab.example.com/team/service-a")); err != nil {
		t.Fatal(err)
	}
	if got := stored.BranchPrepare.CodeProjectKey; got != "gitlab.example.com/team/service-a" {
		t.Fatalf("declared code project key must be sealed: %q", got)
	}
}

// 같은 프로젝트면 봉인하지 않는다 — 기존 사이클의 레코드가 그대로 유지된다.
func TestPrepareLeavesCodeProjectKeyEmptyForSameProject(t *testing.T) {
	store, stored := codeProjectStore(t, "", nil)
	if _, err := Prepare(store, "state", "io-cross", crossProjectRequest("gitlab.example.com/planning/backlog")); err != nil {
		t.Fatal(err)
	}
	if got := stored.BranchPrepare.CodeProjectKey; got != "" {
		t.Fatalf("same-project cycle must not seal a code project key: %q", got)
	}
}

func TestPrepareRejectsNonCanonicalCodeProjectKey(t *testing.T) {
	store, _ := codeProjectStore(t, "", nil)
	_, err := Prepare(store, "state", "io-cross", crossProjectRequest("https://gitlab.example.com/team/service-a"))
	if err == nil || !strings.Contains(err.Error(), "code_project_key") {
		t.Fatalf("non-canonical code project key must be rejected: %v", err)
	}
}

// 선언이 없으면 checkout의 origin을 관찰해 봉인한다.
func TestPrepareObservesCodeProjectKeyFromCheckout(t *testing.T) {
	store, stored := codeProjectStore(t, "gitlab.example.com/team/service-a", nil)
	if _, err := Prepare(store, "state", "io-cross", crossProjectRequest("")); err != nil {
		t.Fatal(err)
	}
	if got := stored.BranchPrepare.CodeProjectKey; got != "gitlab.example.com/team/service-a" {
		t.Fatalf("observed code project key must be sealed: %q", got)
	}
}

// 관찰 실패는 branch prepare를 막지 않는다. 이슈 프로젝트로 되돌아갈 뿐이다.
func TestPrepareToleratesUnobservableRemote(t *testing.T) {
	store, stored := codeProjectStore(t, "", fmt.Errorf("no origin"))
	if _, err := Prepare(store, "state", "io-cross", crossProjectRequest("")); err != nil {
		t.Fatalf("an unobservable remote must not block branch preparation: %v", err)
	}
	if got := stored.BranchPrepare.CodeProjectKey; got != "" {
		t.Fatalf("failed observation must leave the key empty: %q", got)
	}
}
