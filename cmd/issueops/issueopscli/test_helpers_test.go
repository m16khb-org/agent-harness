package issueopscli

import (
	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
	"os"
	"strings"
	"testing"
)

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func captureProjectCLIStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStderrAndError(t, fn)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func recordIssueOpsCLIIntentForTest(t *testing.T, id string) {
	t.Helper()
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"intent", "record",
			"--id", id,
			"--raw-request", "refactor issueops flow",
			"--interpreted-intent", "keep intent and design evidence before implementation",
			"--success-criteria", "intent is recorded",
			"--success-criteria", "design is reviewed",
			"--json",
		})
	})
}

func recordIssueOpsCLIDesignForTest(t *testing.T, id string) {
	t.Helper()
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{
			"design", "review",
			"--id", id,
			"--problem-summary", "IssueOps must preserve the work contract",
			"--proposed-design", "Gate implementation on a reviewed design contract",
			"--refactor-plan", "Keep IssueOps state and adapter changes scoped to the active cycle",
			"--alternative", "documentation-only guidance",
			"--risk", "legacy tests must create explicit design evidence",
			"--verification", "design review checked alternatives and risks",
			"--verification", "go test ./cmd/issueops/issueopscli",
			"--approved",
			"--json",
		})
	})
}

func recordIssueOpsCoreIntentForCLITest(t *testing.T, id string) {
	t.Helper()
	if _, err := issueopscore.RecordIssueOpsIntent(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCLIPlanPrepForTest(t *testing.T, id string) {
	t.Helper()
	waived := issueopscontract.IssueOpsPlanPrepItemRequest{WaiveReason: "cli lifecycle test"}
	if _, err := issueopscore.RecordIssueOpsPlanPrep(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsPlanPrepRequest{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
		CodebaseSurvey: waived,
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsCoreDesignForCLITest(t *testing.T, id string) {
	t.Helper()
	if _, err := issueopscore.RecordIssueOpsDesignReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep IssueOps state and adapter changes scoped to the active cycle",
		Alternatives:   []string{"documentation-only guidance"},
		Risks:          []string{"legacy tests must create explicit design evidence"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./cmd/issueops/issueopscli"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}

func seedIssueOpsCLIExecution(t *testing.T, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, issueopscore.IssueOpsActor) {
	t.Helper()
	baseHead := strings.TrimSpace(record.BranchPrepare.BaseSHA)
	if len(baseHead) != 40 {
		baseHead = strings.Repeat("a", 40)
	}
	const now = "2026-07-22T00:00:00Z"
	actor := issueopscore.IssueOpsActor{Host: "codex", SessionID: "issueops-cli-test", AgentID: "test-agent", CWD: record.WorktreePath}
	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actor.NativeProcessAncestry = []issueopscontract.NativeProcessReceipt{receipt}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: record.Repo, Root: record.WorktreePath, Branch: record.Branch,
			BaseHead: baseHead, Driver: "git", LinkedAt: now,
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: now,
			Holder: &issueopscontract.NativeActor{
				Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
				SessionProcess: &receipt,
			},
		},
	}
	written, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written, actor
}

func withIssueOpsCLIActor(args []string, actor issueopscore.IssueOpsActor) []string {
	return append(args,
		"--host", actor.Host,
		"--session-id", actor.SessionID,
		"--agent-id", actor.AgentID,
		"--cwd", actor.CWD,
	)
}
