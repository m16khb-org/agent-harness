package issueops

import (
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"os"
	"path/filepath"
	"testing"
)

func initIssueOpsRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	remote := t.TempDir()
	if code, _, stderr := preflight.GitCmd(remote, "init", "--bare", "-q"); code != 0 {
		t.Fatalf("git init bare failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Test"},
		{"config", "user.email", "issueops@example.test"},
		{"remote", "add", "origin", remote},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeIssueOpsFile(t, repo, "README.md", "readme\n")
	writeIssueOpsFile(t, repo, "plans/demo.md", "plan\n")
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md", "plans/demo.md"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "initial"); code != 0 {
		t.Fatalf("git commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", "main"); code != 0 {
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

func recordIssueOpsIntentForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsIntent(stateRoot, id, IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func setIssueOpsPlanPrepForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	rec, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	waived := model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "legacy lifecycle test"}
	rec.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
	}
	if _, err := writeIssueOps(stateRoot, rec); err != nil {
		t.Fatal(err)
	}
}

// recordIssueOpsGrillArtifactsForTest satisfies the grill completion gate
// (split_decision + domain_review) so legacy tests can still advance past the
// grill->plan boundary. split_decision is recorded as a no-split scope decision.
func recordIssueOpsGrillArtifactsForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := AddIssueOpsDecision(stateRoot, id, IssueOpsDecisionRecordRequest{
		Title: "no split",
		Body:  "single focused work item; no provider-native child tasks needed",
		Kind:  "scope",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsDomainReview(stateRoot, id, IssueOpsDomainReviewRequest{
		ModelFit:          "change fits the existing IssueOps phase model",
		Terminology:       []string{"phase ledger", "completion gate"},
		OpenUncertainties: []string{"none blocking"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsApprovedDesignForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	recordIssueOpsGrillArtifactsForTest(t, stateRoot, id)
	setIssueOpsPlanPrepForTest(t, stateRoot, id)
	if _, err := RecordIssueOpsDesignReview(stateRoot, id, IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:   []string{"documentation-only guidance"},
		Risks:          []string{"legacy tests must create explicit design evidence"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./internal/core/issueops"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsPreparedWorktreeToolsForTest(t *testing.T, stateRoot, id, worktree string) IssueOpsRecord {
	t.Helper()
	recordIssueOpsCompatibilityReviewForTest(t, stateRoot, id)
	recordIssueOpsExecutionDecisionForTest(t, stateRoot, id)
	record, err := RecordIssueOpsWorktreeTools(stateRoot, id, IssueOpsWorktreeToolPreparation{
		OK:                   true,
		WorktreePath:         worktree,
		CodeGraphProjectPath: worktree,
		CodeGraphChecked:     true,
		CodeGraphReady:       true,
		Messages:             []string{"test prepared IssueOps worktree tools"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func recordIssueOpsExecutionDecisionForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsExecutionDecision(stateRoot, id, IssueOpsExecutionDecisionRecordRequest{
		AutoProceed:       []string{"implement after durable readiness gates are present"},
		HookBlocked:       []string{"hooks do not create issues, prepare worktrees, or choose sub-agents"},
		HumanGates:        []string{"ask before destructive cleanup or unclear product behavior"},
		SubagentUse:       "none",
		SubagentRationale: "main agent directly owns this focused implementation",
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCompatibilityReviewForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	if _, err := RecordIssueOpsCompatibilityReview(stateRoot, id, IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps state records remain readable"},
		SideEffects:           []string{"phase order changes are limited to IssueOps lifecycle readiness"},
		RollbackPlan:          "Revert the compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects", "go test ./internal/core/issueops"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	// The devil's-advocate verdict is a fail-closed implement-entry gate, so bring
	// the cycle to implement-readiness with a pass verdict alongside compatibility.
	if _, err := RecordIssueOpsDevilsAdvocateReview(stateRoot, id, IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
}

func issueOpsIntentContractForTest() *IssueOpsIntentContract {
	return &IssueOpsIntentContract{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
		RecordedAt:        "2026-06-05T00:00:00Z",
	}
}

func issueOpsCompatibilityReviewForTest() *IssueOpsCompatibilityReview {
	return &IssueOpsCompatibilityReview{
		BackwardCompatibility: []string{"existing IssueOps state records remain readable"},
		SideEffects:           []string{"phase order changes are limited to IssueOps lifecycle readiness"},
		RollbackPlan:          "Revert the compatibility-review phase and readiness gate.",
		Verification:          []string{"compatibility review checked backward compatibility and side effects", "go test ./internal/core/issueops"},
		Approved:              true,
		ReviewedAt:            "2026-06-26T00:00:00Z",
	}
}

func issueOpsDevilsAdvocateReviewForTest() *IssueOpsDevilsAdvocateReview {
	return &IssueOpsDevilsAdvocateReview{
		Verdict:         "pass",
		ReviewerPattern: "devils-advocate-review",
		RecordedAt:      "2026-06-29T00:00:00Z",
	}
}

func issueOpsExecutionDecisionForTest() *IssueOpsExecutionDecision {
	return &IssueOpsExecutionDecision{
		AutoProceed:       []string{"implement after durable readiness gates are present"},
		HookBlocked:       []string{"hooks do not create issues, prepare worktrees, or choose sub-agents"},
		HumanGates:        []string{"ask before destructive cleanup or unclear product behavior"},
		SubagentUse:       "none",
		SubagentRationale: "main agent directly owns this focused implementation",
		RecordedAt:        "2026-06-23T00:00:00Z",
	}
}

func issueOpsDesignReviewForTest() *IssueOpsDesignReview {
	return &IssueOpsDesignReview{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:   []string{"documentation-only guidance"},
		Risks:          []string{"legacy tests must create explicit design evidence"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./internal/core/issueops"},
		Approved:       true,
		ReviewedAt:     "2026-06-05T00:00:00Z",
	}
}

func issueOpsWeakApprovedDesignReviewForTest() *IssueOpsDesignReview {
	return &IssueOpsDesignReview{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		Verification:   []string{"go test ./internal/core/issueops"},
		Approved:       true,
		ReviewedAt:     "2026-06-05T00:00:00Z",
	}
}
