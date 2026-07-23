package hookcli

import (
	"os"
	"strings"
	"testing"

	"agent-harness/internal/core"
	issueopscore "agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func recordIssueOpsHookIntentForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), id, core.IssueOpsIntentRecordRequest{
		RawRequest:        "refactor issueops flow",
		InterpretedIntent: "keep intent and design evidence before implementation",
		SuccessCriteria:   []string{"intent is recorded", "design is reviewed"},
	}); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsHookDesignForTest(t *testing.T, id string) {
	t.Helper()
	if _, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), id, core.IssueOpsDesignReviewRequest{
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

func activateIssueOpsHookExecution(t *testing.T, id string) core.IssueOpsActor {
	t.Helper()
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	baseHead := strings.TrimSpace(record.BranchPrepare.BaseSHA)
	if len(baseHead) != 40 {
		baseHead = strings.Repeat("a", 40)
	}
	const now = "2026-07-22T00:00:00Z"
	actor := core.IssueOpsActor{Host: "codex", SessionID: "hookcli-test", AgentID: "test-agent", CWD: record.WorktreePath}
	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actor.NativeProcessAncestry = []model.NativeProcessReceipt{receipt}
	record.Execution = &model.Execution{
		Mode: model.ExecutionModeDirect,
		Workspace: model.Workspace{
			SourceRoot: record.Repo, Root: record.WorktreePath, Branch: record.Branch,
			BaseHead: baseHead, Driver: "git", LinkedAt: now,
		},
		Lease: model.WriteLease{
			Generation: 1, Status: model.LeaseStatusActive, ClaimedAt: now,
			Holder: &model.NativeActor{
				Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
				SessionProcess: &receipt,
			},
		},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if _, err := core.AdvanceIssueOpsPhaseWithActor(core.IssueOpsStateRoot(), id, string(core.IssueOpsPhaseImplement), actor); err != nil {
		t.Fatal(err)
	}
	return actor
}
