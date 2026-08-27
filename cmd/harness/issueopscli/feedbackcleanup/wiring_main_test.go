package feedbackcleanup

import (
	"context"
	"os"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// 프로덕션에서는 harnessapp이 주입한다. cleanup CLI 테스트는 실제 정리 경로를
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureCleanup(CleanupDeps{
		AddIssueOpsFeedbackWithActor: issueopscore.AddIssueOpsFeedbackWithActor,
		CleanupAbandon: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, d Deps) (issueopscontract.CleanupAbandonResult, error) {
			return issueopscore.CleanupAbandon(ctx, stateRoot, req, issueopscore.CleanupAbandonDeps{Orca: d.OrcaIntent, OrcaOwner: d.OrcaOwner})
		},
		CleanupFinish: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			return issueopscore.CleanupFinish(ctx, stateRoot, req, issueopscore.CleanupFinishDeps{
				Git:                d.CleanupFinishGit,
				Processes:          issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses},
				RemoveOrcaWorktree: d.RemoveOrcaWorktree,
				ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error {
					return issueopscore.ReflectCleanupAudit(issueopscore.IssueOpsStateRoot(), rec, completion, audit, prov)
				},
			})
		},
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
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
	os.Exit(m.Run())
}
