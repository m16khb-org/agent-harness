package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/executioncmd"
	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
	"context"
)

func runIssueOpsExecution(args []string) error {
	return executioncmd.Run(args, executioncmd.Deps{
		StateRoot: core.IssueOpsStateRoot,
		Direct:    gitworktree.New(),
		Orca:      orca.NewExecution(),
		ReadIssue: provider.ReadExecutionIssueSnapshot,
		RemotePR: issueops.RemotePullRequestDependencies{
			Create: func(providerName string, req core.IssueProviderCreatePullRequestRequest) (core.IssueProviderCreatePullRequestResult, error) {
				prov, err := provider.Resolve(providerName)
				if err != nil {
					return core.IssueProviderCreatePullRequestResult{}, err
				}
				return core.CreateRemotePullRequest(req, prov)
			},
			Reconcile: func(providerName string, req core.IssueProviderReconcilePullRequestRequest) (core.IssueProviderReconcilePullRequestResult, error) {
				prov, err := provider.Resolve(providerName)
				if err != nil {
					return core.IssueProviderReconcilePullRequestResult{}, err
				}
				return core.ReconcileRemotePullRequest(req, prov)
			},
			Verify: verifyIssueOpsRemoteArtifactLive,
		},
		SettleOrcaTask: settleOrcaTask,
		PrintJSON:      printJSON,
		PrintError:     printIssueOpsErrorJSON,
	})
}

// settleOrcaTask는 완료된 orca 사이클의 task를 terminal 상태로 옮긴다.
// completed를 쓰는 이유는 operationalhealth 분류기가 completed와 failed를
// 종결로 보고 residue 판정에서 면제하기 때문이며, 정상 완료는 completed다(#130).
func settleOrcaTask(ctx context.Context, taskID string) error {
	return orca.New().UpdateTask(ctx, taskID, orcaTaskStatusCompleted, "")
}

const orcaTaskStatusCompleted = "completed"
