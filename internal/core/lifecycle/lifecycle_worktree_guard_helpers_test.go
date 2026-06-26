package lifecycle

import (
	"agent-harness/internal/core/issueops"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func guardRepoWithCycle(t *testing.T, branch string, phase IssueOpsPhase) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: branch}); err != nil {
		t.Fatal(err)
	}
	if phase != IssueOpsPhaseProblem {
		setIssueOpsPhaseForTest(t, repo, branch, phase)
	}
	return repo
}

func setIssueOpsPhaseForTest(t *testing.T, repo, branch string, phase IssueOpsPhase) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = phase
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func linkIssueOpsBranchEvidenceForTest(t *testing.T, repo, branch string) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	before, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/1"
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), id, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), id, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       issueOpsProviderBranchForTest(branch),
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if before.Phase != IssueOpsPhaseProblem && before.Phase != IssueOpsPhasePlan {
		if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(before.Phase)); err != nil {
			t.Fatal(err)
		}
	}
}

func issueOpsProviderBranchForTest(branch string) string {
	if validateIssueOpsIssueBranch(branch) == nil {
		return branch
	}
	slug := strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, branch), "-")
	if slug == "" {
		slug = "branch"
	}
	return "1-" + slug
}

func markIssueOpsPRPhaseForTest(t *testing.T, repo, branch string) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = IssueOpsPhasePR
	record.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
		Provider:   "github",
		Kind:       "pr",
		URL:        "https://github.com/example/repo/pull/1",
		Labels:     []string{"issueops"},
		Assignees:  []string{"habin"},
		VerifiedAt: "2026-06-05T00:00:00Z",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func issueOpsGuardWorktreePathForTest(repo, slug string) string {
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
}

func makeIssueOpsGuardWorktreeForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := issueOpsGuardWorktreePathForTest(repo, slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func writeIssueOpsGuardFileForTest(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type linkedIssueOpsWorktreeForTest struct {
	id   string
	path string
}

func linkIssueOpsWorktreeForGuardTest(t *testing.T, repo, branch string) linkedIssueOpsWorktreeForTest {
	t.Helper()
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/" + strings.SplitN(branch, "-", 2)[0]
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := makeIssueOpsGuardWorktreeForTest(t, repo, branch)
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPhaseForTest(t, repo, branch, IssueOpsPhaseImplement)
	return linkedIssueOpsWorktreeForTest{id: record.ID, path: worktree}
}

func recordIssueOpsWorktreeToolsForGuardTest(t *testing.T, id, worktree string) {
	t.Helper()
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreeTools = &issueops.IssueOpsWorktreeToolPreparation{
		OK:                   true,
		WorktreePath:         worktree,
		CodeGraphProjectPath: worktree,
		CodeGraphChecked:     true,
		CodeGraphReady:       true,
		PreparedAt:           "2026-06-05T00:00:00Z",
	}
	record.CompatibilityReview = &issueops.IssueOpsCompatibilityReview{
		BackwardCompatibility: []string{"guard fixture preserves existing worktree edit boundaries"},
		SideEffects:           []string{"source checkout edits remain blocked after implementation starts"},
		RollbackPlan:          "reset fixture state and rerun lifecycle guard tests",
		Verification:          []string{"go test ./internal/core/lifecycle"},
		Approved:              true,
		ReviewedAt:            "2026-06-26T00:00:00Z",
	}
	record.ExecutionDecision = &issueops.IssueOpsExecutionDecision{
		AutoProceed:       []string{"guard fixture may enter implementation after linked worktree readiness is durable"},
		HookBlocked:       []string{"hooks do not prepare worktrees, create remote artifacts, or choose sub-agents"},
		HumanGates:        []string{"ask before destructive cleanup or unclear product behavior"},
		SubagentUse:       "none",
		SubagentRationale: "main agent owns this focused lifecycle fixture",
		RecordedAt:        "2026-06-23T00:00:00Z",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}
