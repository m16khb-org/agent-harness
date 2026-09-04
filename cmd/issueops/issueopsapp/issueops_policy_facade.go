package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	"issueops/cmd/issueops/policycli"
	"issueops/internal/adapter/issueops"
	basesyncoutbound "issueops/internal/adapter/outbound/issueopsbasesync"
	provenanceadapter "issueops/internal/adapter/outbound/issueopsprovenance"
	issueopscontract "issueops/internal/contract/issueops"
	policy "issueops/internal/contract/policy"
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
