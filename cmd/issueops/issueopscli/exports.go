package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/remotecmd"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

func RunIssueOps(args []string) error {
	return RunIssueOpsWithDependencies(args, Dependencies{})
}

type Dependencies struct {
	Prepare     issueopscontract.ExecutionPrepareHandler
	Orca        port.ExecutionOrcaProvisioner
	OrcaOwner   port.ExecutionOrcaOwnerInspector
	BaseSync    basesyncport.Inspector
	ReadIssue   executionissue.ExecutionIssueSnapshotReadFunc
	Claim       issueopscontract.ExecutionClaimHandler
	Release     issueopscontract.ExecutionReleaseHandler
	Reseed      issueopscontract.ExecutionReseedHandler
	Resume      issueopscontract.ExecutionResumeHandler
	Reconcile   port.ExecutionReconcileHandler
	Complete    issueopscontract.ExecutionCompleteHandler
	Publication remotecmd.PublicationHandlers
	Provenance  provenanceport.Observer
}

func RunIssueOpsWithDependencies(args []string, deps Dependencies) error {
	return runIssueOpsWithDependencies(args, deps)
}

// RunIssueOpsWithReleaseHandler is the composition-root entry point for the
// production release vertical. Other IssueOps actions retain their existing facade.
func RunIssueOpsWithReleaseHandler(args []string, release issueopscontract.ExecutionReleaseHandler) error {
	return RunIssueOpsWithExecutionHandlers(args, nil, release)
}

func RunIssueOpsWithExecutionHandlers(args []string, claim issueopscontract.ExecutionClaimHandler, release issueopscontract.ExecutionReleaseHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseed(args, claim, release, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseed(args []string, claim issueopscontract.ExecutionClaimHandler, release issueopscontract.ExecutionReleaseHandler, reseed issueopscontract.ExecutionReseedHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseedAndResume(args, claim, release, reseed, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseedAndResume(args []string, claim issueopscontract.ExecutionClaimHandler, release issueopscontract.ExecutionReleaseHandler, reseed issueopscontract.ExecutionReseedHandler, resume issueopscontract.ExecutionResumeHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseedResumeAndReconcile(args, claim, release, reseed, resume, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseedResumeAndReconcile(args []string, claim issueopscontract.ExecutionClaimHandler, release issueopscontract.ExecutionReleaseHandler, reseed issueopscontract.ExecutionReseedHandler, resume issueopscontract.ExecutionResumeHandler, reconcile port.ExecutionReconcileHandler) error {
	return RunIssueOpsWithDependencies(args, Dependencies{
		Claim: claim, Release: release, Reseed: reseed, Resume: resume, Reconcile: reconcile,
	})
}

func VerifyChildIssueBeforeLink(childURL string) error {
	return verifyIssueOpsChildIssueBeforeLink(childURL)
}

func CleanupMerged(id string, requested bool) bool {
	return issueOpsCleanupMerged(id, requested)
}

func VerifyRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return verifyIssueOpsRemoteArtifactLive(req)
}

func SetChildIssueVerifier(verifier func(string) error) func(string) error {
	return remoteverify.SetChildIssueVerifier(verifier)
}
