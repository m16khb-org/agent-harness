package hookcli

import (
	"os"
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/issueops/loopgate"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func recordIssueOpsHookIntentForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := issueopscore.RecordIssueOpsIntent(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsHookDesignForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := issueopscore.RecordIssueOpsDesignReview(issueopscore.IssueOpsStateRoot(), id, issueopscontract.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve the work contract",
		ProposedDesign: "Gate implementation on a reviewed design contract",
		RefactorPlan:   "Keep hook guard behavior aligned with IssueOps cycle state",
		Alternatives:   []string{"allow source edits without linked-cycle evidence"},
		Risks:          []string{"worktree guard fixtures must model approved design evidence"},
		Verification:   []string{"design review checked worktree guard risks", "go test ./cmd/harness/hookcli"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
}

func activateIssueOpsHookExecution(t *testing.T, id string) issueopscore.IssueOpsActor {
	t.Helper()
	record, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	baseHead := strings.TrimSpace(record.BranchPrepare.BaseSHA)
	if len(baseHead) != 40 {
		baseHead = strings.Repeat("a", 40)
	}
	const now = "2026-07-22T00:00:00Z"
	actor := issueopscore.IssueOpsActor{Host: "codex", SessionID: "hookcli-test", AgentID: "test-agent", CWD: record.WorktreePath}
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
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if _, err := loopgate.AdvancePhaseWithActor(issueopscore.IssueOpsStateRoot(), id, string(issueopscore.IssueOpsPhaseImplement), actor); err != nil {
		t.Fatal(err)
	}
	return actor
}
