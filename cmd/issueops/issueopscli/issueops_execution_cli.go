package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/executioncmd"
)

func runIssueOpsExecution(args []string) error {
	return runIssueOpsExecutionWithDependencies(args, Dependencies{})
}

func runIssueOpsExecutionWithDependencies(args []string, deps Dependencies) error {
	return executioncmd.Run(args, issueOpsExecutionDeps(deps))
}

func issueOpsExecutionDeps(deps Dependencies) executioncmd.Deps {
	return executioncmd.Deps{
		StateRoot:   issueOpsCLIDeps.IssueOpsStateRoot,
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
