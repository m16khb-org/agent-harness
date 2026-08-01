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
	return runIssueOpsExecutionWithDependencies(args, Dependencies{})
}

func runIssueOpsExecutionWithRelease(args []string, release issueops.ExecutionReleaseHandler) error {
	return runIssueOpsExecutionWithDependencies(args, Dependencies{Release: release})
}

func runIssueOpsExecutionWithHandlers(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler) error {
	return runIssueOpsExecutionWithDependencies(args, Dependencies{Claim: claim, Release: release})
}

func runIssueOpsExecutionWithDependencies(args []string, deps Dependencies) error {
	return executioncmd.Run(args, issueOpsExecutionDeps(deps))
}

func issueOpsExecutionDeps(deps Dependencies) executioncmd.Deps {
	return executioncmd.Deps{
		StateRoot: core.IssueOpsStateRoot,
		Direct:    gitworktree.New(),
		Orca:      orca.NewExecution(),
		ReadIssue: provider.ReadExecutionIssueSnapshot,
		// 완료가 orca task를 종결시킨다(#130).
		SettleOrcaTask: orca.New().SettleTask,
		Claim:          deps.Claim,
		Release:        deps.Release,
		Reseed:         deps.Reseed,
		Resume:         deps.Resume,
		Reconcile:      deps.Reconcile,
		Publication:    deps.Publication,
		PrintJSON:      printJSON,
		PrintError:     printIssueOpsErrorJSON,
	}
}
