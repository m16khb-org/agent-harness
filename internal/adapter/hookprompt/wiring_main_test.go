package hookprompt

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	"os"
	"testing"
)

// 프로덕션에서는 composition root가 주입한다. 이 테스트는 bootstrap이 실제로
// lifecycle namespace를 만든 뒤의 프롬프트를 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	ConfigureLifecycle(realLifecycleDeps())
	lifecycle.ConfigureIssueOps(lifecycle.IssueOpsDeps{
		ActiveIssueOpsCycleForBranch:              issueopscore.ActiveIssueOpsCycleForBranch,
		ActiveIssueOpsLinkedWorktreeCyclesForRepo: issueopscore.ActiveIssueOpsLinkedWorktreeCyclesForRepo,
		AdvanceIssueOpsPhase:                      issueopscore.AdvanceIssueOpsPhase,
		IssueOpsPhaseExpectsWorktree:              issueopscore.IssueOpsPhaseExpectsWorktree,
		IssueOpsStateRoot:                         issueopscore.IssueOpsStateRoot,
		LinkIssueOpsIssue:                         issueopscore.LinkIssueOpsIssue,
		LinkIssueOpsPlan:                          issueopscore.LinkIssueOpsPlan,
		LinkIssueOpsWorktree:                      issueopscore.LinkIssueOpsWorktree,
		ListIssueOpsIDs:                           issueopscore.ListIssueOpsIDs,
		ScanIssueOps:                              issueopscore.ScanIssueOps,
		NewIssueOpsID:                             issueopscore.NewIssueOpsID,
		PrepareIssueOpsBranch:                     issueopscore.PrepareIssueOpsBranch,
		ReadIssueOps:                              issueopscore.ReadIssueOps,
		RecordIssueOpsDesignReview:                issueopscore.RecordIssueOpsDesignReview,
		RecordIssueOpsIntent:                      issueopscore.RecordIssueOpsIntent,
		SealedOwnerContextPacketPath:              issueopscore.SealedOwnerContextPacketPath,
		StartIssueOps:                             issueopscore.StartIssueOps,
		WriteIssueOps:                             issueopscore.WriteIssueOps,
	})
	os.Exit(m.Run())
}
