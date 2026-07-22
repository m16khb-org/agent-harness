package issueops

import (
	"path/filepath"
	"testing"
)

func TestImplementationReadinessRequiresDevilsAdvocateVerdict(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record := IssueOpsRecord{
		OK:                  true,
		Repo:                repo,
		Branch:              "1-demo",
		IssueURL:            "https://github.com/example/repo/issues/1",
		PlanPath:            filepath.Join(worktree, "plans/demo.md"),
		WorktreePath:        worktree,
		Intent:              issueOpsIntentContractForTest(),
		DesignReview:        issueOpsDesignReviewForTest(),
		CompatibilityReview: issueOpsCompatibilityReviewForTest(),
		BranchPrepare:       &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
		Execution:           issueOpsExecutionForTest(repo, worktree, "1-demo"),
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	// No review → blocked.
	if ready := IssueOpsImplementationReadiness(record); ready.Ready || !containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("missing devil's-advocate review must block implement: %+v", ready.Missing)
	}
	// pass → clears the gate.
	record.DevilsAdvocateReview = &IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "t"}
	if ready := IssueOpsImplementationReadiness(record); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("pass verdict should clear the gate: %+v", ready.Missing)
	}
	// unwaived stop → blocked.
	record.DevilsAdvocateReview = &IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"gold-plating"}, RecordedAt: "t"}
	if ready := IssueOpsImplementationReadiness(record); !containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("unwaived stop must block implement: %+v", ready.Missing)
	}
	// waived stop → clears.
	record.DevilsAdvocateReview.Waived = true
	record.DevilsAdvocateReview.WaiverRationale = "filed follow-up issue"
	if ready := IssueOpsImplementationReadiness(record); containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("waived stop should clear the gate: %+v", ready.Missing)
	}
}
