package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	lifecycle "agent-harness/internal/adapter/lifecycle"
)

// 프로덕션에서는 harnessapp이 주입한다. IssueOps CLI 테스트는 lifecycle 훅 경로도
// 함께 검증하므로 같은 배선을 재현한다.
func wireLifecycleIssueOpsForTests() {
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
		NewIssueOpsID:                             issueopscore.NewIssueOpsID,
		PrepareIssueOpsBranch:                     issueopscore.PrepareIssueOpsBranch,
		ReadIssueOps:                              issueopscore.ReadIssueOps,
		RecordIssueOpsDesignReview:                issueopscore.RecordIssueOpsDesignReview,
		RecordIssueOpsIntent:                      issueopscore.RecordIssueOpsIntent,
		SealedOwnerContextPacketPath:              issueopscore.SealedOwnerContextPacketPath,
		StartIssueOps:                             issueopscore.StartIssueOps,
		WriteIssueOps:                             issueopscore.WriteIssueOps,
	})
}
