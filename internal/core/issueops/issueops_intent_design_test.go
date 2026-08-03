package issueops

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/adapter/outbound/sqlstore"
)

func TestIssueOpsIntentAndDesignGatePhaseProgression(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-intent-design"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePlan)); err == nil || !strings.Contains(err.Error(), "intent_contract") {
		t.Fatalf("plan phase should require intent contract, got %v", err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatalf("issue link should record remote issue before intent contract: %v", err)
	}
	if record.Phase != IssueOpsPhaseProblem {
		t.Fatalf("issue link before intent should not enter plan phase: %+v", record)
	}
	if _, err := RecordIssueOpsIntent(stateRoot, record.ID, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "IssueOps must understand intent before refactoring",
		InterpretedIntent: "IssueOps must understand intent before refactoring",
		SuccessCriteria:   []string{"intent is interpreted, not copied"},
	}); err == nil || !strings.Contains(err.Error(), "interpreted_intent must differ from raw_request") {
		t.Fatalf("intent interpretation should reject raw request copy, got %v", err)
	}
	if _, err := RecordIssueOpsIntent(stateRoot, record.ID, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "IssueOps must understand intent before refactoring",
		InterpretedIntent: "IssueOps must understand the intent before refactoring.",
		SuccessCriteria:   []string{"intent is interpreted, not copied"},
	}); err == nil || !strings.Contains(err.Error(), "interpreted_intent must materially differ from raw_request") {
		t.Fatalf("intent interpretation should reject near-copy raw request, got %v", err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		Verification:   []string{"go test ./..."},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "intent_contract") {
		t.Fatalf("design review before intent should be rejected, got %v", err)
	}

	record, err = RecordIssueOpsIntent(stateRoot, record.ID, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "IssueOps must understand intent before refactoring",
		InterpretedIntent: "Persist intent evidence and block premature implementation",
		SuccessCriteria:   []string{"plan cannot start without intent", "implementation cannot start without approved design"},
		Constraints:       []string{"keep the cycle auditable"},
		Ambiguities:       []string{"none after user correction"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Intent == nil || len(record.Intent.SuccessCriteria) != 2 {
		t.Fatalf("intent contract should be persisted: %+v", record.Intent)
	}
	recordWithoutIssue, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "2-no-issue"})
	if err != nil {
		t.Fatal(err)
	}
	recordWithoutIssue, err = RecordIssueOpsIntent(stateRoot, recordWithoutIssue.ID, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "IssueOps must still require remote issue before planning",
		InterpretedIntent: "Plan phase must prove the issue contract exists",
		SuccessCriteria:   []string{"plan cannot start without issue_url"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, recordWithoutIssue.ID, string(IssueOpsPhasePlan)); err == nil || !strings.Contains(err.Error(), "issue_url") {
		t.Fatalf("plan phase should require issue_url after intent, got %v", err)
	}
	recordIssueOpsGrillArtifactsForTest(t, stateRoot, record.ID)
	setIssueOpsPlanPrepForTest(t, stateRoot, record.ID)
	record, err = AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePlan))
	if err != nil || record.Phase != IssueOpsPhasePlan {
		t.Fatalf("plan should be allowed after intent contract, got %+v err=%v", record, err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		Verification:   []string{"go test ./..."},
		OpenQuestions:  []string{"which design?"},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "open_questions") {
		t.Fatalf("approved design should reject open questions, got %v", err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		Verification:   []string{"go test ./..."},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "refactor_plan") {
		t.Fatalf("approved design should require a refactor plan, got %v", err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		RefactorPlan:   "Keep IssueOps state changes localized to core and CLI",
		Verification:   []string{"go test ./..."},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "alternatives") {
		t.Fatalf("approved design should require alternatives considered, got %v", err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		RefactorPlan:   "Keep IssueOps state changes localized to core and CLI",
		Alternatives:   []string{"docs-only guidance"},
		Verification:   []string{"go test ./..."},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "risks") {
		t.Fatalf("approved design should require risk review, got %v", err)
	}
	if _, err := RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		RefactorPlan:   "Keep IssueOps state changes localized to core and CLI",
		Alternatives:   []string{"docs-only guidance"},
		Risks:          []string{"existing lifecycle tests need explicit gate setup"},
		Verification:   []string{"go test ./..."},
		Approved:       true,
	}); err == nil || !strings.Contains(err.Error(), "design_review_evidence") {
		t.Fatalf("approved design should require design review evidence, got %v", err)
	}

	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/1",
		Branch:       "1-intent-design",
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", "1-intent-design"); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "1-intent-design")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, "1-intent-design"); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, worktree, "plans/demo.md", "plan\n")
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md")); err == nil || !strings.Contains(err.Error(), "design_review") {
		t.Fatalf("plan link should require approved design review, got %v", err)
	}
	record, err = RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		RefactorPlan:   "Keep IssueOps state changes localized to core and CLI",
		Verification:   []string{"go test ./internal/core/issueops ./cmd/harness/issueopscli"},
		Risks:          []string{"existing lifecycle tests need explicit gate setup"},
		Approved:       false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md")); err == nil || !strings.Contains(err.Error(), "design_approval") {
		t.Fatalf("unapproved design review should not enter implementation, got %v", err)
	}
	record, err = RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "Foldering bug",
		ProposedDesign: "Gate implementation on reviewed design",
		RefactorPlan:   "Keep IssueOps state changes localized to core and CLI",
		Alternatives:   []string{"docs-only guidance"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./internal/core/issueops ./cmd/harness/issueopscli"},
		Risks:          []string{"existing lifecycle tests need explicit gate setup"},
		Approved:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/demo.md"))
	if err != nil || record.Phase != IssueOpsPhasePlan {
		t.Fatalf("approved design should allow plan attachment before tool prep, got %+v err=%v", record, err)
	}
	record = recordIssueOpsPreparedExecutionForTest(t, stateRoot, record.ID, worktree)
	if record.Phase != IssueOpsPhaseImplement {
		t.Fatalf("worktree tool prep should allow implementation entry, got %+v", record)
	}
}

func TestIssueOpsIntentAndDesignRedactSecretLikeFreeform(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-redaction"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	record, err = RecordIssueOpsIntent(stateRoot, record.ID, issueops.IssueOpsIntentRecordRequest{
		RawRequest:        "token=secret-value",
		InterpretedIntent: "api_key=secret-value",
		SuccessCriteria:   []string{"password=secret-value"},
		Constraints:       []string{"keep audit trail"},
		Ambiguities:       []string{"secret=secret-value"},
		NonGoals:          []string{"do not leak token=secret-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	setIssueOpsPlanPrepForTest(t, stateRoot, record.ID)
	record, err = RecordIssueOpsDesignReview(stateRoot, record.ID, issueops.IssueOpsDesignReviewRequest{
		ProblemSummary: "token=secret-value",
		ProposedDesign: "Keep design evidence but redact api_key=secret-value",
		RefactorPlan:   "password=secret-value",
		Alternatives:   []string{"secret=secret-value"},
		Risks:          []string{"token=secret-value"},
		Verification:   []string{"design review checked redaction risk", "go test ./..."},
		Approved:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	returnedRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(returnedRecord), "secret-value") {
		t.Fatalf("returned record should redact secret-like values: %s", returnedRecord)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	stateFile, ok, err := db.Get("issueops_v1", record.ID)
	if err != nil || !ok {
		t.Fatalf("read persisted record: ok=%v err=%v", ok, err)
	}
	stateText := string(stateFile)
	if strings.Contains(stateText, "secret-value") || (!strings.Contains(stateText, "<redacted>") && !strings.Contains(stateText, `\u003credacted\u003e`)) {
		t.Fatalf("persisted state should redact secret-like values:\n%s", stateFile)
	}
}
