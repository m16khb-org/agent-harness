package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestImplementationReadinessRequiresDevilsAdvocateVerdict(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "1-demo")
	record := issueops.IssueOpsRecord{
		OK:                  true,
		Repo:                repo,
		Branch:              "1-demo",
		IssueURL:            "https://github.com/example/repo/issues/1",
		PlanPath:            filepath.Join(worktree, "plans/demo.md"),
		WorktreePath:        worktree,
		Intent:              issueOpsIntentContractForTest(),
		DesignReview:        issueOpsDesignReviewForTest(),
		CompatibilityReview: issueOpsCompatibilityReviewForTest(),
		BranchPrepare:       &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "1-demo", BaseBranch: "main", LinkVerified: true},
		Execution:           issueOpsExecutionForTest(repo, worktree, "1-demo"),
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")

	// No review → blocked.
	if ready := IssueOpsImplementationReadiness(record); ready.Ready || !containsString(ready.Missing, "devils_advocate_review") {
		t.Fatalf("missing devil's-advocate review must block implement: %+v", ready.Missing)
	}
	// pass → clears the gate.
	bound := digestExecutionOwnerBytes([]byte("plan\n"))
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{Verdict: "pass", Findings: []string{"attacked gate 3"}, ReviewerContext: "subagent", ReviewedPlanDigest: bound, RecordedAt: "t"}
	if ready := IssueOpsImplementationReadiness(record); !ready.Ready || len(ready.Missing) != 0 {
		t.Fatalf("pass verdict should clear the gate: %+v", ready.Missing)
	}
	// unwaived stop → blocked.
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"gold-plating"}, ReviewerContext: "subagent", ReviewedPlanDigest: bound, RecordedAt: "t"}
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

func issueOpsPlanBoundRecordForTest(t *testing.T) (issueops.IssueOpsRecord, string) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "2-bound")
	planPath := filepath.Join(worktree, "plans/bound.md")
	writeIssueOpsFile(t, worktree, "plans/bound.md", "# plan v1\n")
	record := issueops.IssueOpsRecord{
		OK:                  true,
		Repo:                repo,
		Branch:              "2-bound",
		IssueURL:            "https://github.com/example/repo/issues/2",
		PlanPath:            planPath,
		WorktreePath:        worktree,
		Intent:              issueOpsIntentContractForTest(),
		DesignReview:        issueOpsDesignReviewForTest(),
		CompatibilityReview: issueOpsCompatibilityReviewForTest(),
		BranchPrepare:       &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/2", Branch: "2-bound", BaseBranch: "main", LinkVerified: true},
		Execution:           issueOpsExecutionForTest(repo, worktree, "2-bound"),
	}
	return record, planPath
}

func TestImplementationReadinessRejectsStaleDevilsAdvocateReview(t *testing.T) {
	record, planPath := issueOpsPlanBoundRecordForTest(t)
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: []string{"attacked gate 3"}, ReviewerContext: "subagent",
		ReviewedPlanDigest: digestExecutionOwnerBytes([]byte("# plan v1\n")), RecordedAt: "t",
	}
	if ready := IssueOpsImplementationReadiness(record); !ready.Ready {
		t.Fatalf("pass bound to the current plan must clear the gate: %+v", ready.Missing)
	}
	// The plan changed after the review → the verdict no longer describes it.
	if err := os.WriteFile(planPath, []byte("# plan v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsImplementationReadiness(record); ready.Ready || !containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("edited plan must report devils_advocate_review_stale: %+v", ready.Missing)
	}
	// A plan that can no longer be identified is not the reviewed plan either:
	// an empty file or a symlink (identity uses Lstat) must fail closed, even
	// though plan_exists (Stat) would still pass.
	record.DevilsAdvocateReview.ReviewedPlanDigest = digestExecutionOwnerBytes([]byte("# plan v1\n"))
	if err := os.WriteFile(planPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsImplementationReadiness(record); !containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("empty plan file must be stale: %+v", ready.Missing)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "plans/other.md", "# plan v1\n")
	if err := os.Symlink(filepath.Join(record.WorktreePath, "plans/other.md"), planPath); err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsImplementationReadiness(record); !containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("symlinked plan must be stale even when its target has the reviewed content: %+v", ready.Missing)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "plans/bound.md", "# plan v2\n")
	// A legacy review with no digest is stale as well (fail closed).
	record.DevilsAdvocateReview.ReviewedPlanDigest = ""
	if ready := IssueOpsImplementationReadiness(record); !containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("review without a plan digest must be stale: %+v", ready.Missing)
	}
	// A delegated child inherits the parent verdict by policy; it is exempt.
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{Verdict: "pass", Waived: true, WaiverRationale: "delegated:io-parent parent DA verdict pass", ReviewerPattern: "delegated-parent-review", RecordedAt: "t"}
	if ready := IssueOpsImplementationReadiness(record); containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("delegated parent review must be exempt from plan binding: %+v", ready.Missing)
	}
}

func TestAISlopCleanReadinessDoesNotCheckPlanBinding(t *testing.T) {
	record, planPath := issueOpsPlanBoundRecordForTest(t)
	record.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: []string{"attacked gate 3"}, ReviewerContext: "subagent",
		ReviewedPlanDigest: digestExecutionOwnerBytes([]byte("# plan v1\n")), RecordedAt: "t",
	}
	if err := os.WriteFile(planPath, []byte("# plan v1\n- [x] T1 done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ready := IssueOpsAISlopCleanReadiness(record); containsString(ready.Missing, "devils_advocate_review_stale") {
		t.Fatalf("plan edits during implementation must not block ai-slop-clean entry: %+v", ready.Missing)
	}
}

func TestRecordDevilsAdvocateReviewBindsStagedPlanWhenNoFileIsLinked(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	request := issueops.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"attacked gate 3"}}
	if _, err := RecordIssueOpsDevilsAdvocateReview(stateRoot, record.ID, request); err == nil || !strings.Contains(err.Error(), "link-plan") {
		t.Fatalf("a cycle with neither a linked nor a staged plan has nothing to review, got %v", err)
	}
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# staged plan\n")); err != nil {
		t.Fatal(err)
	}
	bound, err := RecordIssueOpsDevilsAdvocateReview(stateRoot, record.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := bound.DevilsAdvocateReview.ReviewedPlanDigest, digestExecutionOwnerBytes([]byte("# staged plan\n")); got != want {
		t.Fatalf("verdict must bind to the staged plan artifact: got %q want %q", got, want)
	}
	if _, err := RequireStagedExecutionOwnerPlan(stateRoot, bound); err != nil {
		t.Fatalf("a verdict bound to the staged plan must pass the owner preflight: %v", err)
	}
}
