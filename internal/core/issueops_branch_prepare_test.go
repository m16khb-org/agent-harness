package core

import (
	"strings"
	"testing"
)

func TestIssueOpsPrepareBranchRecordsProviderFallbackOrder(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "123-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/123")
	if err != nil {
		t.Fatal(err)
	}

	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:        "gitlab",
		IssueURL:        record.IssueURL,
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

func TestIssueOpsPrepareBranchUsesGitHubDevelopFallback(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "456-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/456")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   record.IssueURL,
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	prepare := record.BranchPrepare
	if prepare == nil || len(prepare.Steps) != 3 {
		t.Fatalf("expected github branch prepare steps: %+v", record)
	}
	if prepare.Steps[0].Strategy != "mcp_unavailable" {
		t.Fatalf("github MCP branch creation is not currently exposed and must be explicit: %+v", prepare.Steps[0])
	}
	if prepare.Steps[1].Strategy != "fallback_api" || len(prepare.Steps[1].Command) < 2 || prepare.Steps[1].Command[0] != "gh" || prepare.Steps[1].Command[1] != "issue" {
		t.Fatalf("github fallback must use gh issue develop: %+v", prepare.Steps[1])
	}
}

func TestIssueOpsPrepareBranchRejectsUnlinkedGitLabBranchName(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "123-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:   "gitlab",
		IssueURL:   record.IssueURL,
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "123-") {
		t.Fatalf("expected GitLab issue prefix rejection, got %v", err)
	}
}
