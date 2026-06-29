package issueops

import (
	"agent-harness/internal/core/preflight"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsAdvancePhaseCoversFullLifecycle(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "1-demo"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	// problem completion (intent contract) is required before grill is entered.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseGrill)); err == nil || !strings.Contains(err.Error(), "intent_contract") {
		t.Fatalf("grill entry should require problem completion (intent_contract), got %v", err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	// problem -> grill is a valid forward step once problem is complete.
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
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md")); err != nil {
		t.Fatal(err)
	}
	record = recordIssueOpsPreparedWorktreeToolsForTest(t, stateRoot, record.ID, worktree)
	if record.Phase != IssueOpsPhaseImplement {
		t.Fatalf("prepared worktree tools should move to implement phase: %+v", record)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean)); err == nil || !strings.Contains(err.Error(), "implementation_changes") {
		t.Fatalf("ai-slop-clean without implementation changes should be rejected, got %v", err)
	}
	writeIssueOpsFile(t, worktree, "internal/demo.go", "package demo\n")
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil || record.Phase != IssueOpsPhaseAISlopClean {
		t.Fatalf("expected ai-slop-clean phase, got %+v err=%v", record, err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil || record.Phase != IssueOpsPhaseAISlopClean {
		t.Fatalf("late issue link refresh should not move phase backward, got %+v err=%v", record, err)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "add", "internal/demo.go", "plans/demo.md"); code != 0 {
		t.Fatalf("git add implementation failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "commit", "-q", "-m", "feat: implement issue"); code != 0 {
		t.Fatalf("git commit implementation failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "push", "-q"); code != 0 {
		t.Fatalf("git push implementation failed: %s", stderr)
	}
	remote := strings.TrimSpace(preflight.GitOut(repo, "remote", "get-url", "origin"))
	other := filepath.Join(t.TempDir(), "other")
	if code, _, stderr := preflight.GitCmd(t.TempDir(), "clone", "-q", remote, other); code != 0 {
		t.Fatalf("git clone for remote advance failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Remote"},
		{"config", "user.email", "remote@example.test"},
		{"checkout", "-q", branch},
	} {
		if code, _, stderr := preflight.GitCmd(other, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, other, "REMOTE.md", "remote advance\n")
	if code, _, stderr := preflight.GitCmd(other, "add", "REMOTE.md"); code != 0 {
		t.Fatalf("git add remote advance failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(other, "commit", "-q", "-m", "docs: remote advance"); code != 0 {
		t.Fatalf("git commit remote advance failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(other, "push", "-q"); code != 0 {
		t.Fatalf("git push remote advance failed: %s", stderr)
	}
	if ready := IssueOpsStrictPRReadiness(record); ready.Ready || !containsString(ready.Missing, "upstream_synced") {
		t.Fatalf("strict readiness should fetch and reject stale upstream state: %+v", ready)
	}
	if code, _, stderr := preflight.GitCmd(worktree, "pull", "-q", "--ff-only"); code != 0 {
		t.Fatalf("git pull worktree after remote advance failed: %s", stderr)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil || !strings.Contains(err.Error(), "ai_slop_clean_stale") {
		t.Fatalf("pr phase after post-cleanup changes should require fresh ai-slop-clean, got %v", err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseAISlopClean))
	if err != nil || record.Phase != IssueOpsPhaseAISlopClean || record.AISlopCleanFingerprint == "" {
		t.Fatalf("fresh ai-slop-clean should record the current implementation fingerprint, got %+v err=%v", record, err)
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
	record, err = AddIssueOpsFeedback(stateRoot, record.ID, "review", "late contract change", "contract_change")
	if err != nil {
		t.Fatalf("feedback after pr phase should be recorded, got %v", err)
	}
	if record.Phase != IssueOpsPhaseFeedback {
		t.Fatalf("post-pr feedback should return the loop to feedback phase: %+v", record)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil || !strings.Contains(err.Error(), "contract_feedback_issue_update") {
		t.Fatalf("post-pr contract feedback should block pr until issue update, got %v", err)
	}
	record, err = MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR))
	if err != nil || record.Phase != IssueOpsPhasePR {
		t.Fatalf("pr phase after issue update should succeed, got %+v err=%v", record, err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhaseDone)); err == nil || !strings.Contains(err.Error(), "remote artifact") {
		t.Fatalf("done phase should require remote artifact verification, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/1",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "GitHub pull request URL") {
		t.Fatalf("github remote artifact should reject non-GitHub PR URL, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/not-a-number",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "GitHub pull request URL") {
		t.Fatalf("github remote artifact should reject nonnumeric PR URL, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/other/repo/pull/1",
		Labels:    []string{"bug"},
		Assignees: []string{"habin"},
	}); err == nil || !strings.Contains(err.Error(), "linked issue project") {
		t.Fatalf("github remote artifact should reject PR URL from another repo, got %v", err)
	}
	if _, err := VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"bug"},
		Assignees: []string{"@me"},
	}); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("github remote artifact should reject placeholder assignee, got %v", err)
	}
	record, err = VerifyIssueOpsRemoteArtifact(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/1",
		Labels:    []string{"bug", "bug"},
		Assignees: []string{"habin"},
	})
	if err != nil {
		t.Fatalf("remote artifact verification should succeed: %v", err)
	}
	if record.RemoteArtifact == nil || record.RemoteArtifact.URL == "" || len(record.RemoteArtifact.Labels) != 1 || len(record.RemoteArtifact.Assignees) != 1 {
		t.Fatalf("remote artifact verification should persist URL, labels, and assignees: %+v", record.RemoteArtifact)
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
