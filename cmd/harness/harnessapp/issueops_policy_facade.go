package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/cmd/harness/policycli"
	basesyncoutbound "agent-harness/internal/adapter/outbound/issueopsbasesync"
	provenanceadapter "agent-harness/internal/adapter/outbound/issueopsprovenance"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
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
		Publication: issueops.RemotePublicationHandlers{
			Create:    issueops.RemotePullRequestCreateHandler(issueOpsPublicationCreateHandler),
			Reconcile: issueops.RemotePullRequestReconcileHandler(issueOpsPublicationReconcileHandler),
		},
	})
}

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return issueopscli.VerifyChildIssueBeforeLink(childURL)
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

func runPolicyCheck(args []string) error {
	return policycli.RunCheck(args)
}

func runPolicyFakeRun(args []string) error {
	return policycli.RunFakeRun(args)
}

func runPolicyRun(args []string) error {
	return policycli.RunReadOnly(args)
}

func runPolicyAudit(args []string) error {
	return policycli.RunAudit(args)
}

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	return policycli.ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (core.CommandPolicyRequest, bool, bool, error) {
	return policycli.ParseRunFlags(args)
}
