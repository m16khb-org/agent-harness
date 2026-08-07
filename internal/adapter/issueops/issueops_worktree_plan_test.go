package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

func TestIssueOpsLinkPlanResolvesRelativePathInsideLinkedWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(filepath.Join(repo, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, repo, "docs/plans/source-only.md", "source plan\n")
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "docs/plans/source-only.md"); err == nil || !strings.Contains(err.Error(), "plan_path does not exist") {
		t.Fatalf("relative plan path should be resolved inside linked worktree, got %v", err)
	}
	externalPlan := filepath.Join(t.TempDir(), "external.md")
	if err := os.WriteFile(externalPlan, []byte("external plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(worktree, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPlan := filepath.Join(worktree, "docs", "plans", "external-link.md")
	if err := os.Symlink(externalPlan, symlinkPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "docs/plans/external-link.md"); err == nil || !strings.Contains(err.Error(), "inside linked worktree") {
		t.Fatalf("relative symlink plan should resolve inside linked worktree, got %v", err)
	}
	writeIssueOpsFile(t, worktree, "docs/plans/worktree.md", "worktree plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/plans/worktree.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(worktree, "docs", "plans", "worktree.md")
	if record.PlanPath != want {
		t.Fatalf("relative plan path should persist as linked-worktree path, got %q want %q", record.PlanPath, want)
	}
}

func TestIssueOpsWorktreeLinkRequiresExistingDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-worktree")
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, missing); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing worktree path should fail, got %v", err)
	}
}

func TestIssueOpsWorktreeLinkRequiresSiblingIsolation(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-demo",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, repo); err == nil || !strings.Contains(err.Error(), "source checkout") {
		t.Fatalf("source checkout as worktree should fail, got %v", err)
	}
	symlinkParent := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees")
	if err := os.MkdirAll(symlinkParent, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkWorktree := filepath.Join(symlinkParent, "1-demo-symlink")
	if err := os.Symlink(repo, symlinkWorktree); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, symlinkWorktree); err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("source checkout symlink as worktree should fail, got %v", err)
	}
	adHoc := filepath.Join(t.TempDir(), "1-demo")
	if err := os.MkdirAll(adHoc, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, adHoc); err == nil || !strings.Contains(err.Error(), "sibling worktree") {
		t.Fatalf("ad hoc worktree path should fail, got %v", err)
	}
	expected := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, expected); err != nil {
		t.Fatalf("sibling worktree should be accepted, got %v", err)
	}
}

func TestIssueOpsWorktreeLinkRequiresIssueBranch(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "1-demo"
	otherBranch := "2-other"
	for _, name := range []string{branch, otherBranch} {
		if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", name); code != 0 {
			t.Fatalf("git checkout branch %s failed: %s", name, stderr)
		}
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	wrongWorktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", wrongWorktree, otherBranch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, wrongWorktree); err == nil || !strings.Contains(err.Error(), "does not match IssueOps branch") {
		t.Fatalf("wrong branch worktree should fail, got %v", err)
	}
}

func TestIssueOpsPlanMustStayInsideLinkedWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "12-issue-worktree"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "issue-worktree")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/12")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "gitlab",
		IssueURL:     record.IssueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	sourcePlan := filepath.Join(repo, "plans", "demo.md")
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, sourcePlan); err == nil || !strings.Contains(err.Error(), "inside linked worktree") {
		t.Fatalf("source checkout plan should not link after worktree, got %v", err)
	}

	record.PlanPath = sourcePlan
	record.AISlopCleanAt = "2026-06-05T00:00:00Z"
	ready := IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "plan_in_worktree") {
		t.Fatalf("source checkout plan should block strict readiness: %+v", ready)
	}
}
