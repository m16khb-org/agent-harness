package harnessapp

import (
	"context"
	"os"

	"agent-harness/cmd/harness/issueopscli"
	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// IssueOps CLI는 사이클 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsCLIRuntime() {
	observer := issueOpsRecordObserver(os.Stderr)
	artifacts := issueOpsArtifactHandlers(observer)
	decisions := issueOpsDecisionHandlers(observer)
	routing := issueOpsRoutingHandlers(observer)
	issueopscli.ConfigureIssueOpsRuntime2(issueopscli.IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:   issueopscore.AcceptIssueOpsChildWithActor,
		AddIssueOpsDecisionWithActor:   decisions.AddWithActor,
		DropIssueOpsChildWithActor:     issueopscore.DropIssueOpsChildWithActor,
		IssueOpsChildStatusWithActor:   issueopscore.IssueOpsChildStatusWithActor,
		IssueOpsPRReadiness:            issueopscore.IssueOpsPRReadiness,
		IssueOpsStateRoot:              issueopscore.IssueOpsStateRoot,
		IssueOpsStatus:                 issueOpsStatusHandler(observer),
		LinkIssueOpsChildWithActor:     issueopscore.LinkIssueOpsChildWithActor,
		LinkIssueOpsIssueWithActor:     issueopscore.LinkIssueOpsIssueWithActor,
		LinkIssueOpsPlanWithActor:      issueopscore.LinkIssueOpsPlanWithActor,
		LinkIssueOpsRelatedWithActor:   issueopscore.LinkIssueOpsRelatedWithActor,
		LinkIssueOpsWorktreeWithActor:  issueopscore.LinkIssueOpsWorktreeWithActor,
		ListIssueOpsCycles:             issueOpsInventoryListHandler(observer),
		ObserveNativeProcessAncestry:   issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor: issueopscore.PrepareIssueOpsBranchWithActor,
		AwaitIssueOpsBranchLink: func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error) {
			return issueopscore.AwaitBranchLink(ctx, stateRoot, req, issueopscore.AwaitBranchLinkDeps{
				ObserveLinkedBranches: issueopscore.ObserveGitHubLinkedBranches(issueopscore.LiveProviderCLI),
			})
		},
		PruneIssueOps: issueOpsRetentionPruneHandler(observer),
		ReadIssueOps:  issueopscore.ReadIssueOps,
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
