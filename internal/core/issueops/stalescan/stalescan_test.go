package stalescan

import (
	"slices"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
)

func baseProbe() Probe {
	return Probe{
		WorktreeDirExists:  func(string) bool { return true },
		WorktreeHeadBranch: func(string) string { return "1-x" },
		RemoteBranchExists: func(model.IssueOpsRecord) (bool, bool) { return true, true },
		Now:                func() time.Time { return time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC) },
	}
}

func TestClassifyConfirmedStaleWhenWorktreeDeleted(t *testing.T) {
	p := baseProbe()
	p.WorktreeDirExists = func(string) bool { return false }
	rec := model.IssueOpsRecord{ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/gone"}

	f, ok := Classify(rec, p, 14*24*time.Hour)
	if !ok || f.Category != CategoryConfirmedStale || !f.Releasable {
		t.Fatalf("deleted worktree should be confirmed-stale+releasable, got %+v ok=%v", f, ok)
	}
	if !slices.Contains(f.Reasons, "worktree_deleted") {
		t.Fatalf("expected worktree_deleted reason, got %v", f.Reasons)
	}
}

func TestClassifyNeedsReviewWhenWorktreeBranchMismatch(t *testing.T) {
	p := baseProbe()
	p.WorktreeHeadBranch = func(string) string { return "other-branch" }
	rec := model.IssueOpsRecord{ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/wt"}

	f, ok := Classify(rec, p, 14*24*time.Hour)
	if !ok || f.Category != CategoryNeedsReview || f.Releasable {
		t.Fatalf("branch-mismatch worktree should be needs-review and NOT releasable, got %+v ok=%v", f, ok)
	}
	if !slices.Contains(f.Reasons, "worktree_branch_mismatch") {
		t.Fatalf("expected worktree_branch_mismatch reason, got %v", f.Reasons)
	}
}

func TestClassifyConfirmedStaleWhenWorktreeNotGit(t *testing.T) {
	p := baseProbe()
	p.WorktreeHeadBranch = func(string) string { return "" }
	rec := model.IssueOpsRecord{ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/wt"}

	f, ok := Classify(rec, p, 14*24*time.Hour)
	if !ok || f.Category != CategoryConfirmedStale || !slices.Contains(f.Reasons, "worktree_not_git") {
		t.Fatalf("non-git worktree should be confirmed-stale, got %+v ok=%v", f, ok)
	}
}

func TestClassifyLikelyDoneWhenRemoteBranchAbsent(t *testing.T) {
	p := baseProbe()
	p.RemoteBranchExists = func(model.IssueOpsRecord) (bool, bool) { return false, true }
	rec := model.IssueOpsRecord{
		ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhasePR, WorktreePath: "/wt",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/x/r/pull/1"},
	}

	f, ok := Classify(rec, p, 14*24*time.Hour)
	if !ok || f.Category != CategoryLikelyDone || !f.Releasable || !slices.Contains(f.Reasons, "remote_branch_absent") {
		t.Fatalf("merged/absent remote branch should be likely-done+releasable, got %+v ok=%v", f, ok)
	}
}

func TestClassifyNeedsReviewWhenOnlyStaleByAge(t *testing.T) {
	p := baseProbe()
	rec := model.IssueOpsRecord{
		ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhasePlan,
		UpdatedAt: "2026-05-01T00:00:00Z", // > 14 days before now (2026-06-09)
	}

	f, ok := Classify(rec, p, 14*24*time.Hour)
	if !ok || f.Category != CategoryNeedsReview || f.Releasable {
		t.Fatalf("age-only stale should be needs-review and NOT releasable, got %+v ok=%v", f, ok)
	}
}

func TestClassifySkipsHealthyCycle(t *testing.T) {
	p := baseProbe()
	rec := model.IssueOpsRecord{
		ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/wt",
		UpdatedAt: "2026-06-08T00:00:00Z", // fresh
	}

	if _, ok := Classify(rec, p, 14*24*time.Hour); ok {
		t.Fatalf("a live worktree cycle on its branch, recently updated, must not be flagged")
	}
}

func TestClassifySkipsDoneCycle(t *testing.T) {
	p := baseProbe()
	p.WorktreeDirExists = func(string) bool { return false }
	rec := model.IssueOpsRecord{ID: "io-1", Branch: "1-x", Phase: model.IssueOpsPhaseDone, WorktreePath: "/gone"}

	if _, ok := Classify(rec, p, 14*24*time.Hour); ok {
		t.Fatalf("done cycles must never be flagged")
	}
}
