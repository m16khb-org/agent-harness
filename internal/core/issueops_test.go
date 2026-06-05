package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if record.ID == "" || record.Phase != IssueOpsPhaseProblem || record.Repo != repo || record.Branch != "1-demo" {
		t.Fatalf("unexpected start record: %+v", record)
	}
	if ready := IssueOpsPRReadiness(record); ready.Ready {
		t.Fatalf("new cycle should not be PR-ready: %+v", ready)
	}

	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhasePlan || record.IssueURL == "" {
		t.Fatalf("issue link should move to plan phase: %+v", record)
	}

	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
	if record.WorktreePath != worktree {
		t.Fatalf("worktree path should be persisted: %+v", record)
	}
	writeIssueOpsFile(t, worktree, "docs/superpowers/plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/superpowers/plans/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement || record.PlanPath != filepath.Join(worktree, "docs/superpowers/plans/demo.md") {
		t.Fatalf("plan link should move to implement phase: %+v", record)
	}

	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "tighten acceptance criteria", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Phase != IssueOpsPhaseImplement || len(record.Feedback) != 1 || record.Feedback[0].Source != "user" {
		t.Fatalf("early feedback should be persisted without entering feedback phase: %+v", record)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ID != record.ID || reloaded.IssueURL != record.IssueURL || reloaded.PlanPath != record.PlanPath || len(reloaded.Feedback) != 1 {
		t.Fatalf("reloaded record mismatch: %+v vs %+v", reloaded, record)
	}
	if ready := IssueOpsPRReadiness(reloaded); ready.Ready || !containsString(ready.Missing, "ai_slop_clean") {
		t.Fatalf("cycle with issue and plan still needs ai-slop-clean before PR drafting: %+v", ready)
	}
	reloaded, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AISlopCleanAt == "" {
		t.Fatalf("ai-slop-clean phase should record completion time: %+v", reloaded)
	}
	reloaded, err = AddIssueOpsFeedback(stateRoot, record.ID, "user", "cleanup passed", "")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Phase != IssueOpsPhaseFeedback || len(reloaded.Feedback) != 2 {
		t.Fatalf("feedback after ai-slop-clean should enter feedback phase: %+v", reloaded)
	}
	if ready := IssueOpsPRReadiness(reloaded); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("cycle with issue, plan, and ai-slop-clean should be PR-ready for drafting: %+v", ready)
	}
}

func TestIssueOpsContractChangeFeedbackBlocksPRUntilIssueUpdateRecorded(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
	writeIssueOpsFile(t, worktree, "docs/superpowers/plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, "docs/superpowers/plans/demo.md")
	if err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "acceptance criteria changed", "contract_change")
	if err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsPRReadiness(record); ready.Ready || !containsString(ready.Missing, "contract_feedback_issue_update") {
		t.Fatalf("contract_change feedback should block PR until issue update is recorded: %+v", ready)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil || !strings.Contains(err.Error(), "contract_feedback_issue_update") {
		t.Fatalf("pr phase should be blocked by unresolved contract feedback, got %v", err)
	}
	record, err = MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Feedback[0].IssueUpdatedAt == "" {
		t.Fatalf("issue update timestamp should be recorded: %+v", record.Feedback)
	}
	if ready := IssueOpsPRReadiness(record); !ready.Ready || containsString(ready.Missing, "contract_feedback_issue_update") {
		t.Fatalf("recorded issue update should unblock PR readiness: %+v", ready)
	}
}

func TestIssueOpsStartRequiresIssueBranch(t *testing.T) {
	stateRoot := t.TempDir()
	for _, branch := range []string{"main", "development", "feature/2387-fix-grpc-ai-dmm-tag-replication-lag"} {
		if _, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: branch}); err == nil || !strings.Contains(err.Error(), "issue number") {
			t.Fatalf("start should reject non-IssueOps branch %q, got %v", branch, err)
		}
	}
	if _, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "2387-fix-grpc-ai-dmm-tag-replication-lag"}); err != nil {
		t.Fatalf("start should accept GitLab-linked IssueOps branch: %v", err)
	}
}

func TestIssueOpsImplementationLinksRequireBranchEvidence(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
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
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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

func TestIssueOpsLinkPlanResolvesRelativePathInsideLinkedWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(filepath.Join(repo, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, repo, "docs/plans/source-only.md", "source plan\n")
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, "docs/plans/source-only.md"); err == nil || !strings.Contains(err.Error(), "plan_path does not exist") {
		t.Fatalf("relative plan path should be resolved inside linked worktree, got %v", err)
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

func TestIssueOpsBranchPrepareRequiresLinkedIssue(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/1",
		Branch:     "1-demo",
		BaseBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "issue must be linked") {
		t.Fatalf("branch prepare before link-issue should fail, got %v", err)
	}
}

func TestIssueOpsGitLabBranchPrepareRequiresIssueNumberPrefix(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "123-provider-linked-branch"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:   "gitlab",
		IssueURL:   record.IssueURL,
		Branch:     "456-provider-linked-branch",
		BaseBranch: "main",
	}); err == nil || !strings.Contains(err.Error(), "123-") {
		t.Fatalf("gitlab branch missing issue number prefix should fail, got %v", err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "gitlab",
		IssueURL:     record.IssueURL,
		Branch:       "123-provider-linked-branch",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatalf("gitlab branch with issue number prefix should pass, got %v", err)
	}
}

func TestIssueOpsChildLinkRequiresLinkedParentIssue(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
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

func TestIssueOpsWorktreeLinkRequiresExistingDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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
		if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", name); code != 0 {
			t.Fatalf("git checkout branch %s failed: %s", name, stderr)
		}
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	wrongWorktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", wrongWorktree, otherBranch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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

func TestIssueOpsDoneRequiresPRPhase(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseDone)); err == nil || !strings.Contains(err.Error(), "before pr phase") {
		t.Fatalf("done before pr should fail, got %v", err)
	}
}

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

func TestIssueOpsStrictPRReadinessRequiresCleanSyncedRepo(t *testing.T) {
	repo := initIssueOpsRepo(t)
	record := IssueOpsRecord{
		OK:            true,
		Repo:          repo,
		Branch:        "main",
		IssueURL:      "https://gitlab.example/group/project/-/issues/1",
		PlanPath:      "plans/demo.md",
		WorktreePath:  repo,
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: "https://gitlab.example/group/project/-/issues/1", Branch: "main", BaseBranch: "main", LinkVerified: true},
		AISlopCleanAt: "2026-06-05T00:00:00Z",
	}

	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("clean synced repo should be strict-ready: %+v", ready)
	}

	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ready = IssueOpsStrictPRReadiness(record)
	if ready.Ready || !containsString(ready.Missing, "worktree_clean") {
		t.Fatalf("dirty worktree should block strict readiness: %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessUsesLinkedWorktree(t *testing.T) {
	repo := initIssueOpsRepo(t)
	branch := "12-issue-worktree"
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "issue-worktree")
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(repo, "source-dirty.txt"), []byte("dirty source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := IssueOpsRecord{
		OK:            true,
		Repo:          repo,
		Branch:        branch,
		IssueURL:      "https://gitlab.example/group/project/-/issues/2",
		PlanPath:      "plans/demo.md",
		WorktreePath:  worktree,
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: "https://gitlab.example/group/project/-/issues/2", Branch: branch, BaseBranch: "main", LinkVerified: true},
		AISlopCleanAt: "2026-06-05T00:00:00Z",
	}

	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("strict readiness should inspect the linked worktree, not dirty source checkout: %+v", ready)
	}
}

func TestIssueOpsPlanMustStayInsideLinkedWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "12-issue-worktree"
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "issue-worktree")
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://gitlab.example/group/project/-/issues/12")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
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

func TestIssueOpsAdvancePhaseCoversFullLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "1-demo"
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	// problem -> grill is a valid forward step.
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseGrill))
	if err != nil || record.Phase != IssueOpsPhaseGrill {
		t.Fatalf("expected grill phase, got %+v err=%v", record, err)
	}
	// An unknown phase must be rejected.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, "nonsense"); err == nil {
		t.Fatalf("expected unknown phase rejection")
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean)); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ai-slop-clean without issue/plan/worktree should be rejected, got %v", err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "cannot enter implement phase") {
		t.Fatalf("implement phase without issue/plan/worktree should be rejected, got %v", err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseFeedback)); err == nil || !strings.Contains(err.Error(), "before ai-slop-clean") {
		t.Fatalf("feedback phase before ai-slop-clean should be rejected, got %v", err)
	}
	// pr phase requires issue + plan + ai-slop-clean evidence (readiness gate).
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil {
		t.Fatalf("pr phase without readiness should be rejected")
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/1",
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean)); err == nil || !strings.Contains(err.Error(), "worktree_path") {
		t.Fatalf("ai-slop-clean without worktree should be rejected, got %v", err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md")); err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil || record.Phase != IssueOpsPhaseAISlopClean {
		t.Fatalf("expected ai-slop-clean phase, got %+v err=%v", record, err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil || record.Phase != IssueOpsPhaseAISlopClean {
		t.Fatalf("late issue link refresh should not move phase backward, got %+v err=%v", record, err)
	}
	remote := strings.TrimSpace(GitOut(repo, "remote", "get-url", "origin"))
	other := filepath.Join(t.TempDir(), "other")
	if code, _, stderr := GitCmd(t.TempDir(), "clone", "-q", remote, other); code != 0 {
		t.Fatalf("git clone for remote advance failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Remote"},
		{"config", "user.email", "remote@example.test"},
		{"checkout", "-q", branch},
	} {
		if code, _, stderr := GitCmd(other, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, other, "REMOTE.md", "remote advance\n")
	if code, _, stderr := GitCmd(other, "add", "REMOTE.md"); code != 0 {
		t.Fatalf("git add remote advance failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(other, "commit", "-q", "-m", "docs: remote advance"); code != 0 {
		t.Fatalf("git commit remote advance failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(other, "push", "-q"); code != 0 {
		t.Fatalf("git push remote advance failed: %s", stderr)
	}
	if ready := IssueOpsStrictPRReadiness(record); ready.Ready || !containsString(ready.Missing, "upstream_synced") {
		t.Fatalf("strict readiness should fetch and reject stale upstream state: %+v", ready)
	}
	if code, _, stderr := GitCmd(worktree, "pull", "-q", "--ff-only"); code != 0 {
		t.Fatalf("git pull worktree after remote advance failed: %s", stderr)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR))
	if err != nil || record.Phase != IssueOpsPhasePR {
		t.Fatalf("pr phase with strict readiness should succeed, got %+v err=%v", record, err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "cannot move issueops phase backward") {
		t.Fatalf("pr phase should not move backward to implement, got %v", err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseFeedback)); err == nil || !strings.Contains(err.Error(), "cannot move issueops phase backward") {
		t.Fatalf("pr phase should not move backward to feedback, got %v", err)
	}
	if _, err := AddIssueOpsFeedback(stateRoot, record.ID, "review", "late contract change", "contract_change"); err == nil || !strings.Contains(err.Error(), "after pr phase") {
		t.Fatalf("feedback after pr phase should be rejected, got %v", err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseDone))
	if err != nil || record.Phase != IssueOpsPhaseDone {
		t.Fatalf("done phase should succeed, got %+v err=%v", record, err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseDone))
	if err != nil || record.Phase != IssueOpsPhaseDone {
		t.Fatalf("done phase should be idempotent, got %+v err=%v", record, err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "cannot leave done phase") {
		t.Fatalf("done phase should be terminal, got %v", err)
	}
	if _, err := AddIssueOpsFeedback(stateRoot, record.ID, "review", "too late", "defect"); err == nil || !strings.Contains(err.Error(), "after done phase") {
		t.Fatalf("feedback after done phase should be rejected, got %v", err)
	}
}

func TestIssueOpsFeedbackRecordsClassification(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "scope change請求", "contract_change")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Feedback) != 1 || record.Feedback[0].Classification != "contract_change" {
		t.Fatalf("expected classification persisted, got %+v", record.Feedback)
	}
	// classification is optional and defaults to empty.
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "ci", "flaky test", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Feedback[1].Classification != "" {
		t.Fatalf("expected empty classification default, got %+v", record.Feedback[1])
	}
}

func TestIssueOpsChildLinksPersistProviderNeutralGraph(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example", Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/10")
	if err != nil {
		t.Fatal(err)
	}

	record, err = LinkIssueOpsChild(stateRoot, record.ID, "https://github.com/example/repo/issues/11", "write child graph tests")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.IssueLinks) != 1 {
		t.Fatalf("expected one child issue link, got %+v", record.IssueLinks)
	}
	link := record.IssueLinks[0]
	if link.Type != "child" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "write child graph tests" || link.Provider != "github" {
		t.Fatalf("unexpected child issue link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("child issue link should record created_at: %+v", link)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.IssueLinks) != 1 || reloaded.IssueLinks[0].URL != link.URL {
		t.Fatalf("reloaded child issue links mismatch: %+v", reloaded.IssueLinks)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, link.URL, "duplicate"); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate child link rejection, got %v", err)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child"); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("generic child under GitHub parent should be rejected as provider mismatch, got %v", err)
	}
	generic, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/generic", Branch: "10-generic"})
	if err != nil {
		t.Fatal(err)
	}
	generic, err = LinkIssueOpsIssue(stateRoot, generic.ID, "https://tracker.example/acme/repo/issues/10")
	if err != nil {
		t.Fatal(err)
	}
	generic, err = LinkIssueOpsChild(stateRoot, generic.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child")
	if err != nil {
		t.Fatal(err)
	}
	if got := generic.IssueLinks[0].Provider; got != "" {
		t.Fatalf("generic issue URL should not infer a provider, got %q", got)
	}
	if _, err := LinkIssueOpsChild(stateRoot, record.ID, "not-a-url", "bad"); err == nil || !strings.Contains(err.Error(), "child_url") {
		t.Fatalf("expected child URL validation error, got %v", err)
	}
}

func TestIssueOpsRejectsUnsafeInputs(t *testing.T) {
	stateRoot := t.TempDir()
	if _, err := StartIssueOps(stateRoot, IssueOpsStartRequest{}); err == nil || !strings.Contains(err.Error(), "repo") {
		t.Fatalf("expected repo validation error, got %v", err)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: "/repo/example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "TOKEN=secret-value"); err == nil || !strings.Contains(err.Error(), "issue_url") {
		t.Fatalf("expected issue URL validation error, got %v", err)
	}
}

func initIssueOpsRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	remote := t.TempDir()
	if code, _, stderr := GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, repo, "README.md", "readme\n")
	writeIssueOpsFile(t, repo, "plans/demo.md", "plan\n")
	if code, _, stderr := GitCmd(repo, "add", "README.md", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	return repo
}

func issueOpsWorktreePathForTest(repo, slug string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
}

func makeIssueOpsWorktreeDirForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := issueOpsWorktreePathForTest(repo, slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func writeIssueOpsFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
