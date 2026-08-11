package issueopscli

import (
	issueopsartifactinbound "agent-harness/internal/adapter/inbound/issueopsartifact"
	issueopsdecisioninbound "agent-harness/internal/adapter/inbound/issueopsdecision"
	issueopsinventoryinbound "agent-harness/internal/adapter/inbound/issueopsinventory"
	issueopsretentioninbound "agent-harness/internal/adapter/inbound/issueopsretention"
	issueopsroutinginbound "agent-harness/internal/adapter/inbound/issueopsrouting"
	issueopsstatusinbound "agent-harness/internal/adapter/inbound/issueopsstatus"
	issueopscore "agent-harness/internal/adapter/issueops"
	issueopsartifactoutbound "agent-harness/internal/adapter/outbound/issueopsartifact"
	issueopsauthorizationoutbound "agent-harness/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "agent-harness/internal/adapter/outbound/issueopsdecision"
	issueopsinventoryoutbound "agent-harness/internal/adapter/outbound/issueopsinventory"
	issueopsretentionoutbound "agent-harness/internal/adapter/outbound/issueopsretention"
	issueopsroutingoutbound "agent-harness/internal/adapter/outbound/issueopsrouting"
	issueopsstatusoutbound "agent-harness/internal/adapter/outbound/issueopsstatus"
	issueopsartifactapplication "agent-harness/internal/application/issueopsartifact"
	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
	issueopsinventoryapplication "agent-harness/internal/application/issueopsinventory"
	issueopsretentionapplication "agent-harness/internal/application/issueopsretention"
	issueopsroutingapplication "agent-harness/internal/application/issueopsrouting"
	issueopsstatusapplication "agent-harness/internal/application/issueopsstatus"
	issueopsstatusdomain "agent-harness/internal/domain/issueopsstatus"
)

// 프로덕션에서는 harnessapp이 주입한다. IssueOps CLI 계약 테스트는 실제 사이클
// 저장소를 검증하므로 같은 배선을 재현한다.
func wireIssueOpsRuntimeForTests() {
	artifacts := issueopsartifactinbound.NewHandlers(
		issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{}),
	)
	decisions := issueopsdecisioninbound.NewHandlers(issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	))
	inventory := issueopsinventoryapplication.NewService(
		issueopsinventoryoutbound.Repository{},
		issueopsinventoryoutbound.SystemClock{},
		issueopsinventoryoutbound.CleanPath{},
	)
	retention := issueopsretentionapplication.NewService(
		issueopsretentionoutbound.Repository{},
		issueopsretentionoutbound.SystemClock{},
	)
	status := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{},
		issueopsstatusdomain.NewProjector(issueopscore.IssueOpsPhaseCompletion),
	)
	routing := issueopsroutinginbound.NewHandlers(issueopsroutingapplication.NewService(
		issueopsroutingoutbound.Repository{},
		issueopsroutingoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	))
	ConfigureIssueOpsRuntime2(IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:                issueopscore.AcceptIssueOpsChildWithActor,
		AddIssueOpsDecisionWithActor:                decisions.AddWithActor,
		DropIssueOpsChildWithActor:                  issueopscore.DropIssueOpsChildWithActor,
		IssueOpsChildStatusWithActor:                issueopscore.IssueOpsChildStatusWithActor,
		IssueOpsPRReadiness:                         issueopscore.IssueOpsPRReadiness,
		IssueOpsStateRoot:                           issueopscore.IssueOpsStateRoot,
		IssueOpsStatus:                              issueopsstatusinbound.NewStatusHandler(status),
		LinkIssueOpsChildWithActor:                  issueopscore.LinkIssueOpsChildWithActor,
		LinkIssueOpsIssueWithActor:                  issueopscore.LinkIssueOpsIssueWithActor,
		LinkIssueOpsPlanWithActor:                   issueopscore.LinkIssueOpsPlanWithActor,
		LinkIssueOpsRelatedWithActor:                issueopscore.LinkIssueOpsRelatedWithActor,
		LinkIssueOpsWorktreeWithActor:               issueopscore.LinkIssueOpsWorktreeWithActor,
		ListIssueOpsCycles:                          issueopsinventoryinbound.NewListHandler(inventory),
		ObserveNativeProcessAncestry:                issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor:              issueopscore.PrepareIssueOpsBranchWithActor,
		PruneIssueOps:                               issueopsretentioninbound.NewPruneHandler(retention),
		ReadIssueOps:                                issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor:  issueopscore.RecordIssueOpsAISlopCleanEvidenceWithActor,
		RecordIssueOpsCompatibilityReviewWithActor:  issueopscore.RecordIssueOpsCompatibilityReviewWithActor,
		RecordIssueOpsDesignReviewWithActor:         issueopscore.RecordIssueOpsDesignReviewWithActor,
		RecordIssueOpsDevilsAdvocateReviewWithActor: issueopscore.RecordIssueOpsDevilsAdvocateReviewWithActor,
		RecordIssueOpsDomainReviewWithActor:         issueopscore.RecordIssueOpsDomainReviewWithActor,
		RecordIssueOpsImplementationReview:          issueopscore.RecordIssueOpsImplementationReview,
		RecordIssueOpsIntentWithActor:               issueopscore.RecordIssueOpsIntentWithActor,
		RecordIssueOpsPlanPrepWithActor:             issueopscore.RecordIssueOpsPlanPrepWithActor,
		RecordIssueOpsRoutingWithActor:              routing.Record,
		RegressIssueOpsForReplanWithActor:           issueopscore.RegressIssueOpsForReplanWithActor,
		RejectIssueOpsChildWithActor:                issueopscore.RejectIssueOpsChildWithActor,
		ResolveIssueOpsFeedbackWithActor:            issueopscore.ResolveIssueOpsFeedbackWithActor,
		ScoreLiveRoutingFidelity:                    routing.Score,
		StageIssueOpsArtifact:                       artifacts.Stage,
		StagedIssueOpsArtifactNames:                 artifacts.Names,
		StartIssueOps:                               issueopscore.StartIssueOps,
		StartIssueOpsChildWithActor:                 issueopscore.StartIssueOpsChildWithActor,
		UnstageIssueOpsArtifact:                     artifacts.Unstage,
	})
}
