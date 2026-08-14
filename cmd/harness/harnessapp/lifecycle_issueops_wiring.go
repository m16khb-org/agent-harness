package harnessapp

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	lifecycle "agent-harness/internal/adapter/lifecycle"
)

// lifecycle은 IssueOps 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureLifecycleIssueOps() {
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
		ScanReadableIssueOps:                      issueopscore.ScanReadableIssueOps,
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
