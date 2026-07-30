package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/executioncmd"
	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
)

func runIssueOpsExecution(args []string) error {
	return runIssueOpsExecutionWithHandlers(args, nil, nil)
}

func runIssueOpsExecutionWithRelease(args []string, release issueops.ExecutionReleaseHandler) error {
	return runIssueOpsExecutionWithHandlers(args, nil, release)
}

func runIssueOpsExecutionWithHandlers(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler) error {
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
		// 완료가 orca task를 종결시킨다(#130).
		SettleOrcaTask: orca.New().SettleTask,
		Claim:          claim,
		Release:        release,
		PrintJSON:      printJSON,
		PrintError:     printIssueOpsErrorJSON,
	})
}
