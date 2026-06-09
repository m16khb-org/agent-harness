package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartIssueOpsStoresAbsoluteRepoWhenRelativePathProvided(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: ".", Branch: "12-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Repo != repo {
		t.Fatalf("relative repo should be stored as absolute path, got %q want %q", record.Repo, repo)
	}
	if record.ID != newIssueOpsID(repo, "12-demo") {
		t.Fatalf("relative repo should use absolute path identity, got %q want %q", record.ID, newIssueOpsID(repo, "12-demo"))
	}
}

func TestStartIssueOpsResetsStaleCycleWhenWorktreeDeleted(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	stale := IssueOpsRecord{
		OK:           true,
		ID:           newIssueOpsID(repo, "1-demo"),
		Repo:         repo,
		Branch:       "1-demo",
		Phase:        IssueOpsPhaseImplement,
		PlanPath:     filepath.Join(repo, "PLAN.md"),
		WorktreePath: filepath.Join(t.TempDir(), "deleted-worktree"),
		Intent:       &IssueOpsIntentContract{RawRequest: "old work"},
	}
	if _, err := WriteIssueOps(stateRoot, stale); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Phase != IssueOpsPhaseProblem {
		t.Fatalf("stale cycle with deleted worktree should reset to problem phase, got %q", record.Phase)
	}
	if record.PlanPath != "" || record.Intent != nil {
		t.Fatalf("reset should clear plan/intent, got plan=%q intent=%v", record.PlanPath, record.Intent)
	}
	if record.StaleResetAt == "" {
		t.Fatalf("reset should stamp stale_reset_at for audit, got empty")
	}
	if record.ID != newIssueOpsID(repo, "1-demo") {
		t.Fatalf("reset should keep the repo+branch identity, got %q", record.ID)
	}
}

func TestStartIssueOpsResumesPRPhaseEvenWithDeletedWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	pr := IssueOpsRecord{
		OK:           true,
		ID:           newIssueOpsID(repo, "1-demo"),
		Repo:         repo,
		Branch:       "1-demo",
		Phase:        IssueOpsPhasePR,
		IssueURL:     "https://github.com/example/repo/issues/1",
		WorktreePath: filepath.Join(t.TempDir(), "deleted-worktree"),
		RemoteArtifact: &IssueOpsRemoteArtifactVerification{
			Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/9",
			Labels: []string{"issueops"}, Assignees: []string{"habin"},
		},
	}
	if _, err := WriteIssueOps(stateRoot, pr); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}

	// A pr-phase cycle's durable work product (issue/PR) lives remotely; the
	// deleted worktree must NOT trigger a reset that destroys remote linkage.
	if record.Phase != IssueOpsPhasePR || record.StaleResetAt != "" {
		t.Fatalf("pr-phase cycle with deleted worktree should resume, got phase=%q reset=%q", record.Phase, record.StaleResetAt)
	}
	if record.RemoteArtifact == nil || record.IssueURL == "" {
		t.Fatalf("pr-phase resume must preserve remote linkage, got issueURL=%q artifact=%v", record.IssueURL, record.RemoteArtifact)
	}
}

func TestStartIssueOpsResetPreservesIssueLinkageAnchors(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	stale := IssueOpsRecord{
		OK:           true,
		ID:           newIssueOpsID(repo, "1-demo"),
		Repo:         repo,
		Branch:       "1-demo",
		Phase:        IssueOpsPhaseImplement,
		IssueURL:     "https://github.com/example/repo/issues/1",
		IssueLinks:   []IssueOpsIssueLink{{Type: "child", URL: "https://github.com/example/repo/issues/2"}},
		PlanPath:     filepath.Join(repo, "PLAN.md"),
		WorktreePath: filepath.Join(t.TempDir(), "deleted-worktree"),
		Intent:       &IssueOpsIntentContract{RawRequest: "old"},
	}
	if _, err := WriteIssueOps(stateRoot, stale); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Phase != IssueOpsPhaseProblem {
		t.Fatalf("stale implement cycle should reset to problem, got %q", record.Phase)
	}
	if record.IssueURL == "" || len(record.IssueLinks) != 1 {
		t.Fatalf("reset must preserve issue linkage anchors, got issueURL=%q links=%v", record.IssueURL, record.IssueLinks)
	}
	if record.StaleResetPriorPhase != string(IssueOpsPhaseImplement) {
		t.Fatalf("reset must record prior phase for audit, got %q", record.StaleResetPriorPhase)
	}
	if record.PlanPath != "" || record.Intent != nil {
		t.Fatalf("reset must clear in-worktree artifacts, got plan=%q intent=%v", record.PlanPath, record.Intent)
	}
}

func TestStartIssueOpsResumesCycleWhenWorktreeStillExists(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	worktree := t.TempDir()
	existing := IssueOpsRecord{
		OK:           true,
		ID:           newIssueOpsID(repo, "1-demo"),
		Repo:         repo,
		Branch:       "1-demo",
		Phase:        IssueOpsPhaseImplement,
		PlanPath:     filepath.Join(repo, "PLAN.md"),
		WorktreePath: worktree,
	}
	if _, err := WriteIssueOps(stateRoot, existing); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Phase != IssueOpsPhaseImplement || record.StaleResetAt != "" {
		t.Fatalf("cycle with live worktree should resume unchanged, got phase=%q reset=%q", record.Phase, record.StaleResetAt)
	}
}

func TestStartIssueOpsResumesNonWorktreePhaseWithoutWorktree(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	existing := IssueOpsRecord{
		OK:     true,
		ID:     newIssueOpsID(repo, "1-demo"),
		Repo:   repo,
		Branch: "1-demo",
		Phase:  IssueOpsPhasePlan,
	}
	if _, err := WriteIssueOps(stateRoot, existing); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.Phase != IssueOpsPhasePlan || record.StaleResetAt != "" {
		t.Fatalf("non-worktree phase should resume unchanged, got phase=%q reset=%q", record.Phase, record.StaleResetAt)
	}
}

func TestStartIssueOpsResumesLegacyRelativeRepoRecord(t *testing.T) {
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	legacy := IssueOpsRecord{
		OK:     true,
		ID:     newIssueOpsID(".", "13-demo"),
		Repo:   ".",
		Branch: "13-demo",
		Phase:  IssueOpsPhasePlan,
	}
	if _, err := WriteIssueOps(stateRoot, legacy); err != nil {
		t.Fatal(err)
	}

	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: ".", Branch: "13-demo"})
	if err != nil {
		t.Fatal(err)
	}

	if record.ID != legacy.ID || record.Repo != legacy.Repo || record.Phase != legacy.Phase {
		t.Fatalf("start should resume legacy relative repo record, got %+v want %+v", record, legacy)
	}
}
