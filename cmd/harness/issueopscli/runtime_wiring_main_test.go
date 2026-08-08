package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
)

// 프로덕션에서는 harnessapp이 주입한다. IssueOps CLI 계약 테스트는 실제 사이클
// 저장소를 검증하므로 같은 배선을 재현한다.
func wireIssueOpsRuntimeForTests() {
	ConfigureIssueOpsRuntime2(IssueOpsCLIDeps{
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
