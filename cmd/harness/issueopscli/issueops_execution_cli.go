package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/executioncmd"
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
		StateRoot:   core.IssueOpsStateRoot,
		Prepare:     deps.Prepare,
		Orca:        deps.Orca,
		OrcaOwner:   deps.OrcaOwner,
		BaseSync:    deps.BaseSync,
		ReadIssue:   deps.ReadIssue,
		Claim:       deps.Claim,
		Release:     deps.Release,
		Reseed:      deps.Reseed,
		Resume:      deps.Resume,
		Reconcile:   deps.Reconcile,
		Complete:    deps.Complete,
		Publication: deps.Publication,
		Provenance:  deps.Provenance,
		PrintJSON:   printJSON,
		PrintError:  printIssueOpsErrorJSON,
	}
}
