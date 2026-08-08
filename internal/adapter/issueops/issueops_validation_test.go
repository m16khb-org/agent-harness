package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/contract/issueops"
)

func TestIssueOpsStartRequiresIssueBranch(t *testing.T) {
	stateRoot := t.TempDir()
	for _, branch := range []string{"main", "development", "feature/2387-fix-grpc-ai-dmm-tag-replication-lag"} {
		if _, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: branch}); err == nil || !strings.Contains(err.Error(), "issue number") {
			t.Fatalf("start should reject non-IssueOps branch %q, got %v", branch, err)
		}
	}
	if _, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "2387-fix-grpc-ai-dmm-tag-replication-lag"}); err != nil {
		t.Fatalf("start should accept GitLab-linked IssueOps branch: %v", err)
	}
}

func TestIssueOpsImplementationLinksRequireBranchEvidence(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "plans/demo.md"); err == nil || !strings.Contains(err.Error(), "branch evidence") {
		t.Fatalf("plan link before issue and branch evidence should fail, got %v", err)
	}
	worktree := t.TempDir()
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err == nil || !strings.Contains(err.Error(), "branch evidence") {
		t.Fatalf("worktree link before issue and branch evidence should fail, got %v", err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "plans/demo.md"); err == nil || !strings.Contains(err.Error(), "branch_prepare") {
		t.Fatalf("plan link before branch prepare should fail, got %v", err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   record.IssueURL,
		Branch:     "1-demo",
		BaseBranch: "main",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "plans/demo.md"); err == nil || !strings.Contains(err.Error(), "branch_link_verified") {
		t.Fatalf("plan link before verified branch should fail, got %v", err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "plans/demo.md"); err == nil || !strings.Contains(err.Error(), "linked worktree") {
		t.Fatalf("plan link before linked worktree should fail, got %v", err)
	}
}

func TestIssueOpsBranchPrepareRequiresLinkedIssue(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/1",
		Branch:     "1-demo",
		BaseBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "issue must be linked") {
		t.Fatalf("branch prepare before link-issue should fail, got %v", err)
	}
}

func TestIssueOpsBranchPrepareRequiresResolvableLocalBaseCommit(t *testing.T) {
	repo := t.TempDir()
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q"); code != 0 {
		t.Fatalf("git init: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(
		repo,
		"-c",
		"user.name=IssueOps Test",
		"-c",
		"user.email=issueops@example.invalid",
		"commit",
		"--allow-empty",
		"-q",
		"-m",
		"initial",
	); code != 0 {
		t.Fatalf("git commit: %s", stderr)
	}
	head := preflight.GitOut(repo, "rev-parse", "HEAD")
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "194-base-commit"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/194")
	if err != nil {
		t.Fatal(err)
	}

	_, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   record.IssueURL,
		Branch:     record.Branch,
		BaseBranch: "main",
		BaseSHA:    strings.Repeat("f", 40),
	})
	if err == nil || !strings.Contains(err.Error(), "does not resolve to a local commit") {
		t.Fatalf("unknown base commit must fail closed: %v", err)
	}
	unchanged, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || unchanged.BranchPrepare != nil {
		t.Fatalf("failed prepare must not persist branch metadata: record=%+v err=%v", unchanged.BranchPrepare, readErr)
	}

	prepared, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   record.IssueURL,
		Branch:     record.Branch,
		BaseBranch: "main",
		BaseSHA:    head[:12],
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.BranchPrepare == nil || prepared.BranchPrepare.BaseSHA != head {
		t.Fatalf("base commit must be canonicalized: got=%+v want=%s", prepared.BranchPrepare, head)
	}
}

func TestIssueOpsBranchPrepareRequiresLinkedIssueNumberPrefix(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "123-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "gitlab",
		IssueURL:   record.IssueURL,
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	}); err == nil || !strings.Contains(err.Error(), "123-") {
		t.Fatalf("gitlab branch missing issue number prefix should fail, got %v", err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "gitlab",
		IssueURL:     record.IssueURL,
		Branch:       "123-provider-linked-branch",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatalf("gitlab branch with issue number prefix should pass, got %v", err)
	}

	record, err = StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "456-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   record.IssueURL,
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	}); err == nil || !strings.Contains(err.Error(), "123-") {
		t.Fatalf("github branch missing issue number prefix should fail, got %v", err)
	}
}

func TestIssueOpsChildLinkRequiresLinkedParentIssue(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = LinkIssueOpsChild(stateRoot, record.ID, "https://github.com/example/repo/issues/2", "child")
	if err == nil || !strings.Contains(err.Error(), "parent issue") {
		t.Fatalf("child link before parent issue should fail, got %v", err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/2", "child"); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("child link with provider mismatch should fail, got %v", err)
	}
}

func TestIssueOpsRejectsUnsafeInputs(t *testing.T) {
	stateRoot := t.TempDir()
	if _, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{}); err == nil || !strings.Contains(err.Error(), "repo") {
		t.Fatalf("expected repo validation error, got %v", err)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "TOKEN=secret-value"); err == nil || !strings.Contains(err.Error(), "issue_url") {
		t.Fatalf("expected issue URL validation error, got %v", err)
	}
}
