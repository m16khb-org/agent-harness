package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/cmd/harness/policycli"
	"agent-harness/internal/adapter/issueops"
	basesyncoutbound "agent-harness/internal/adapter/outbound/issueopsbasesync"
	provenanceadapter "agent-harness/internal/adapter/outbound/issueopsprovenance"
	issueopscontract "agent-harness/internal/contract/issueops"
	policy "agent-harness/internal/contract/policy"
)

func wirePolicyCLIDeps() {
	policycli.Configure(policycli.Deps{ResolveTarget: resolveTarget})
}

func runIssueOps(args []string) error {
	execution := productionIssueOpsExecutionDependencies()
	return issueopscli.RunIssueOpsWithDependencies(args, issueopscli.Dependencies{
		Prepare: execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner,
		BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit), ReadIssue: execution.ReadIssue,
		Claim: issueops.ExecutionClaimHandler(issueOpsClaimHandler), Release: issueops.ExecutionReleaseHandler(issueOpsReleaseHandler),
		Reseed: issueops.ExecutionReseedHandler(issueOpsReseedHandler), Resume: issueops.ExecutionResumeHandler(issueOpsResumeHandler),
		Reconcile: issueops.ExecutionReconcileHandler(issueOpsReconcileHandler), Complete: issueops.ExecutionCompleteHandler(issueOpsCompleteHandler),
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: remotecmd.PublicationHandlers{
			Create:    issueops.RemotePullRequestCreateHandler(issueOpsPublicationCreateHandler),
			Reconcile: issueops.RemotePullRequestReconcileHandler(issueOpsPublicationReconcileHandler),
		},
	})
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return issueopscli.CleanupMerged(id, requested)
}

func verifyIssueOpsRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return issueopscli.VerifyRemoteArtifactLive(req)
}

func runPolicy(args []string) error {
	return policycli.Run(args)
}

func parseCommandPolicyFlags(name string, args []string) (policy.CommandPolicyRequest, bool, error) {
	return policycli.ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (policy.CommandPolicyRequest, bool, bool, error) {
	return policycli.ParseRunFlags(args)
}
