package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli"
	issueopscore "agent-harness/internal/adapter/issueops"
)

// IssueOps CLI는 사이클 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsCLIRuntime() {
	issueopscli.ConfigureIssueOpsRuntime2(issueopscli.IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:                issueopscore.AcceptIssueOpsChildWithActor,
		AddIssueOpsDecisionWithActor:                issueopscore.AddIssueOpsDecisionWithActor,
		DropIssueOpsChildWithActor:                  issueopscore.DropIssueOpsChildWithActor,
		IssueOpsChildStatusWithActor:                issueopscore.IssueOpsChildStatusWithActor,
		IssueOpsPRReadiness:                         issueopscore.IssueOpsPRReadiness,
		IssueOpsStateRoot:                           issueopscore.IssueOpsStateRoot,
		IssueOpsStatus:                              issueopscore.IssueOpsStatus,
		LinkIssueOpsChildWithActor:                  issueopscore.LinkIssueOpsChildWithActor,
		LinkIssueOpsIssueWithActor:                  issueopscore.LinkIssueOpsIssueWithActor,
		LinkIssueOpsPlanWithActor:                   issueopscore.LinkIssueOpsPlanWithActor,
		LinkIssueOpsRelatedWithActor:                issueopscore.LinkIssueOpsRelatedWithActor,
		LinkIssueOpsWorktreeWithActor:               issueopscore.LinkIssueOpsWorktreeWithActor,
		ListIssueOpsCycles:                          issueopscore.ListIssueOpsCycles,
		ObserveNativeProcessAncestry:                issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor:              issueopscore.PrepareIssueOpsBranchWithActor,
		PruneIssueOps:                               issueopscore.PruneIssueOps,
		ReadIssueOps:                                issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor:  issueopscore.RecordIssueOpsAISlopCleanEvidenceWithActor,
		RecordIssueOpsCompatibilityReviewWithActor:  issueopscore.RecordIssueOpsCompatibilityReviewWithActor,
		RecordIssueOpsDesignReviewWithActor:         issueopscore.RecordIssueOpsDesignReviewWithActor,
		RecordIssueOpsDevilsAdvocateReviewWithActor: issueopscore.RecordIssueOpsDevilsAdvocateReviewWithActor,
		RecordIssueOpsDomainReviewWithActor:         issueopscore.RecordIssueOpsDomainReviewWithActor,
		RecordIssueOpsImplementationReview:          issueopscore.RecordIssueOpsImplementationReview,
		RecordIssueOpsIntentWithActor:               issueopscore.RecordIssueOpsIntentWithActor,
		RecordIssueOpsPlanPrepWithActor:             issueopscore.RecordIssueOpsPlanPrepWithActor,
		RecordIssueOpsRoutingWithActor:              issueopscore.RecordIssueOpsRoutingWithActor,
		RegressIssueOpsForReplanWithActor:           issueopscore.RegressIssueOpsForReplanWithActor,
		RejectIssueOpsChildWithActor:                issueopscore.RejectIssueOpsChildWithActor,
		ResolveIssueOpsFeedbackWithActor:            issueopscore.ResolveIssueOpsFeedbackWithActor,
		ScoreLiveRoutingFidelity:                    issueopscore.ScoreLiveRoutingFidelity,
		StageIssueOpsArtifact:                       issueopscore.StageIssueOpsArtifact,
		StagedIssueOpsArtifactNames:                 issueopscore.StagedIssueOpsArtifactNames,
		StartIssueOps:                               issueopscore.StartIssueOps,
		StartIssueOpsChildWithActor:                 issueopscore.StartIssueOpsChildWithActor,
		UnstageIssueOpsArtifact:                     issueopscore.UnstageIssueOpsArtifact,
	})
}
