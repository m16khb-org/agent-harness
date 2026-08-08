package lifecycle

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 composition root가 주입한다. lifecycle 테스트는 실제 IssueOps
// 상태를 전제로 하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureIssueOps(IssueOpsDeps{
		ActiveIssueOpsCycleForBranch:              issueopscore.ActiveIssueOpsCycleForBranch,
		ActiveIssueOpsLinkedWorktreeCyclesForRepo: issueopscore.ActiveIssueOpsLinkedWorktreeCyclesForRepo,
		AdvanceIssueOpsPhase:                      issueopscore.AdvanceIssueOpsPhase,
		IssueOpsPhaseExpectsWorktree:              issueopscore.IssueOpsPhaseExpectsWorktree,
		IssueOpsStateRoot:                         issueopscore.IssueOpsStateRoot,
		LinkIssueOpsIssue:                         issueopscore.LinkIssueOpsIssue,
		LinkIssueOpsPlan:                          issueopscore.LinkIssueOpsPlan,
		LinkIssueOpsWorktree:                      issueopscore.LinkIssueOpsWorktree,
		ListIssueOpsIDs:                           issueopscore.ListIssueOpsIDs,
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
