package issueopscli

import (
	"context"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// cleanup CLI는 정리 구현과 의존 조립을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func wireCleanupForTests() {
	feedbackcleanup.ConfigureCleanup(feedbackcleanup.CleanupDeps{
		AddIssueOpsFeedbackWithActor: issueopscore.AddIssueOpsFeedbackWithActor,
		CleanupAbandon: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupAbandonResult, error) {
			return issueopscore.CleanupAbandon(ctx, stateRoot, req, issueopscore.CleanupAbandonDeps{Orca: d.OrcaIntent, OrcaOwner: d.OrcaOwner})
		},
		CleanupFinish: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			return issueopscore.CleanupFinish(ctx, stateRoot, req, issueopscore.CleanupFinishDeps{
				Git:                d.CleanupFinishGit,
				Processes:          issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses},
				RemoveOrcaWorktree: d.RemoveOrcaWorktree,
				ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error {
					return issueopscore.ReflectCleanupAudit(issueopscore.IssueOpsStateRoot(), rec, completion, audit, prov)
				},
			})
		},
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
			return issueopscore.CleanupRemoteBranch(ctx, stateRoot, req, issueopscore.CleanupRemoteBranchDeps{
				VerifyMergedArtifact: d.VerifyMergedHead,
				ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error {
					return issueopscore.ReflectCleanupAudit(issueopscore.IssueOpsStateRoot(), rec, completion, audit, prov)
				},
			})
		},
		CloseIssueOpsChildren:                             issueopscore.CloseIssueOpsChildren,
		FinalizeIssueOpsCleanupStatus:                     issueopscore.FinalizeIssueOpsCleanupStatus,
		IssueOpsCleanupStatusForRecord:                    issueopscore.IssueOpsCleanupStatusForRecord,
		IssueOpsRemoteArtifactMissing:                     issueopscore.IssueOpsRemoteArtifactMissing,
		IssueOpsStateRoot:                                 issueopscore.IssueOpsStateRoot,
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: issueopscore.MarkIssueOpsContractFeedbackIssueUpdatedWithActor,
		ObserveNativeProcessAncestry:                      issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                                      issueopscore.ReadIssueOps,
		ReadRemoteIssueSnapshot:                           issueopscore.ReadRemoteIssueSnapshot,
		ReflectCleanupAudit:                               issueopscore.ReflectCleanupAudit,
		ResolveRecordProvider:                             issueopscore.ResolveRecordProvider,
	})
}
