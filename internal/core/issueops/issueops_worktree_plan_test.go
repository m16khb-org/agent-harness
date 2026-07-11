package issueops

import (
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkPlanAfterCoordinatorCommitAdvancesAttemptBase(t *testing.T) {
	stateRoot, record := coordinatorPlanningRecordForTest(t)
	planPath := filepath.Join(record.WorktreePath, ".agent-harness", "plans", record.ID+"-live-e2e.md")
	writeIssueOpsFile(t, record.WorktreePath, filepath.ToSlash(strings.TrimPrefix(planPath, record.WorktreePath+string(filepath.Separator))), "# current cycle plan\n")
	commitIssueOpsPlanForTest(t, record.WorktreePath, planPath)
	wantHead := strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))

	got, err := LinkIssueOpsPlan(stateRoot, record.ID, planPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanPath != planPath {
		t.Fatalf("PlanPath=%q, want %q", got.PlanPath, planPath)
	}
	if got.ExecutionHandoff.AttemptBaseHead != wantHead {
		t.Fatalf("AttemptBaseHead=%q, want coordinator plan commit %q", got.ExecutionHandoff.AttemptBaseHead, wantHead)
	}
}

func TestLinkPlanRequiresCleanPlanOnlyCoordinatorCommit(t *testing.T) {
	t.Run("dirty plan", func(t *testing.T) {
		stateRoot, record := coordinatorPlanningRecordForTest(t)
		planPath := filepath.Join(record.WorktreePath, ".agent-harness", "plans", record.ID+"-live-e2e.md")
		writeIssueOpsFile(t, record.WorktreePath, filepath.ToSlash(strings.TrimPrefix(planPath, record.WorktreePath+string(filepath.Separator))), "# dirty plan\n")
		if _, err := LinkIssueOpsPlan(stateRoot, record.ID, planPath); err == nil || !strings.Contains(err.Error(), "clean coordinator plan commit") {
			t.Fatalf("dirty coordinator plan must fail before state mutation, got %v", err)
		}
	})

	t.Run("extra committed file", func(t *testing.T) {
		stateRoot, record := coordinatorPlanningRecordForTest(t)
		planPath := filepath.Join(record.WorktreePath, ".agent-harness", "plans", record.ID+"-live-e2e.md")
		writeIssueOpsFile(t, record.WorktreePath, filepath.ToSlash(strings.TrimPrefix(planPath, record.WorktreePath+string(filepath.Separator))), "# current cycle plan\n")
		writeIssueOpsFile(t, record.WorktreePath, "internal/unrelated.go", "package internal\n")
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, "add", planPath, "internal/unrelated.go"); code != 0 {
			t.Fatalf("git add: %s", stderr)
		}
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, "commit", "-q", "-m", "test: mixed coordinator commit"); code != 0 {
			t.Fatalf("git commit: %s", stderr)
		}
		if _, err := LinkIssueOpsPlan(stateRoot, record.ID, planPath); err == nil || !strings.Contains(err.Error(), "only the current cycle plan") {
			t.Fatalf("mixed coordinator commit must fail before state mutation, got %v", err)
		}
	})
}

func coordinatorPlanningRecordForTest(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	record.WorktreePath = worktree
	record.PlanPath = filepath.Join(worktree, "README.md")
	record.DesignReview = issueOpsDesignReviewForTest()
	record.ExecutionHandoff = &IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion,
		State:           handoff.StateCoordinatorPreparing,
		Attempt:         1,
		OwnershipEpoch:  "epoch-plan",
		AttemptBaseHead: record.BranchPrepare.BaseSHA,
		Driver:          "orca",
		Agent:           "codex",
		CoordinatorRoot: record.Repo,
		WorkerRoot:      worktree,
		Orca: &IssueOpsOrcaIdentity{
			RuntimeID: "runtime-plan", RepoID: "repo-plan", BaseRef: "refs/remotes/origin/" + record.Branch,
			WorktreeID: "worktree-plan", WorktreeInstanceID: "instance-plan", WorktreePath: worktree,
		},
	}
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, got
}

func commitIssueOpsPlanForTest(t *testing.T, worktree, planPath string) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(worktree, "add", "--", planPath); code != 0 {
		t.Fatalf("git add plan: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "docs: record current cycle plan"); code != 0 {
		t.Fatalf("git commit plan: %s", stderr)
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
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
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
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
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
