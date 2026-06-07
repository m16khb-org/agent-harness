package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/implementation"
	"agent-harness/internal/core/preflight"
)

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

func TestIssueOpsImplementationReadinessRejectsPersistedWeakDesignReview(t *testing.T) {
	record := IssueOpsRecord{
		OK:            true,
		Repo:          "/repo/example",
		Branch:        "1-demo",
		IssueURL:      "https://github.com/example/repo/issues/1",
		PlanPath:      "plans/demo.md",
		WorktreePath:  "/repo/example.worktrees/1-demo",
		Intent:        issueOpsIntentContractForTest(),
		DesignReview:  issueOpsWeakApprovedDesignReviewForTest(),
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
	}

	ready := IssueOpsImplementationReadiness(record)
	if ready.Ready {
		t.Fatalf("weak persisted approved design review should not be implementation-ready: %+v", ready)
	}
	for _, want := range []string{"refactor_plan", "alternatives", "risks"} {
		if !containsString(ready.Missing, want) {
			t.Fatalf("implementation readiness should report %s, got %+v", want, ready)
		}
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
		Intent:        issueOpsIntentContractForTest(),
		DesignReview:  issueOpsDesignReviewForTest(),
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
		Intent:        issueOpsIntentContractForTest(),
		DesignReview:  issueOpsDesignReviewForTest(),
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: "https://gitlab.example/group/project/-/issues/2", Branch: branch, BaseBranch: "main", LinkVerified: true},
		AISlopCleanAt: "2026-06-05T00:00:00Z",
	}

	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("strict readiness should inspect the linked worktree, not dirty source checkout: %+v", ready)
	}
}

func TestIssueOpsStrictPRReadinessDetectsStaleAISlopCleanAfterImplementationChange(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "12-stale-ai-slop"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "stale-ai-slop")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/12")
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
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\nconst Value = 1\n")
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(record.AISlopCleanFingerprint) == "" {
		t.Fatalf("ai-slop-clean should record changed-file fingerprint: %+v", record)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "add", "internal/demo.go", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "feat: implement after clean"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push failed: %s", stderr)
	}
	if ready := IssueOpsStrictPRReadiness(record); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("committing the same cleaned content should remain ready, got %+v", ready)
	}

	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\nconst Value = 2\n")
	if code, _, stderr := preflight.GitCmd(worktree, "add", "internal/demo.go"); code != 0 {
		t.Fatalf("git add stale change failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "feat: change after clean"); code != 0 {
		t.Fatalf("git commit stale change failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push stale change failed: %s", stderr)
	}
	if ready := IssueOpsStrictPRReadiness(record); ready.Ready || !containsString(ready.Missing, "ai_slop_clean_stale") {
		t.Fatalf("implementation changes after ai-slop-clean should stale PR readiness, got %+v", ready)
	}
}

func TestIssueOpsImplementationEvidenceHelpersParsePorcelainAndIgnorePlanPath(t *testing.T) {
	worktree := t.TempDir()
	plan := filepath.Join(worktree, "plans", "demo.md")
	record := IssueOpsRecord{PlanPath: plan}

	for line, want := range map[string]string{
		" M internal/demo.go":                    "internal/demo.go",
		"?? docs/new.md":                         "docs/new.md",
		"R  old/path.go -> internal/new-path.go": "internal/new-path.go",
		"   ":                                    "",
		" M \"quoted path.md\"":                  "quoted path.md",
	} {
		if got := implementation.PorcelainPath(line); got != want {
			t.Fatalf("PorcelainPath(%q)=%q want %q", line, got, want)
		}
	}
	if !implementation.PathMatchesPlan(record, worktree, plan) {
		t.Fatalf("absolute plan path should match itself")
	}
	if !implementation.PathMatchesPlan(IssueOpsRecord{PlanPath: "plans/demo.md"}, worktree, "plans/demo.md") {
		t.Fatalf("relative plan path should match relative porcelain path")
	}
	if implementation.PathMatchesPlan(record, worktree, filepath.Join(worktree, "internal", "demo.go")) {
		t.Fatalf("implementation path must not be treated as plan path")
	}
}
