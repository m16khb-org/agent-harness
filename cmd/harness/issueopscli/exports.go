package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/port"
)

func RunIssueOps(args []string) error {
	return RunIssueOpsWithDependencies(args, Dependencies{})
}

type Dependencies struct {
	Prepare     issueops.ExecutionPrepareHandler
	Orca        port.ExecutionOrcaProvisioner
	OrcaOwner   port.ExecutionOrcaOwnerInspector
	ReadIssue   issueops.ExecutionIssueSnapshotReadFunc
	Claim       issueops.ExecutionClaimHandler
	Release     issueops.ExecutionReleaseHandler
	Reseed      issueops.ExecutionReseedHandler
	Resume      issueops.ExecutionResumeHandler
	Reconcile   issueops.ExecutionReconcileHandler
	Complete    issueops.ExecutionCompleteHandler
	Publication issueops.RemotePublicationHandlers
}

func RunIssueOpsWithDependencies(args []string, deps Dependencies) error {
	return runIssueOpsWithDependencies(args, deps)
}

// RunIssueOpsWithReleaseHandler is the composition-root entry point for the
// production release vertical. Other IssueOps actions retain their existing facade.
func RunIssueOpsWithReleaseHandler(args []string, release issueops.ExecutionReleaseHandler) error {
	return RunIssueOpsWithExecutionHandlers(args, nil, release)
}

func RunIssueOpsWithExecutionHandlers(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseed(args, claim, release, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseed(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler, reseed issueops.ExecutionReseedHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseedAndResume(args, claim, release, reseed, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseedAndResume(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler, reseed issueops.ExecutionReseedHandler, resume issueops.ExecutionResumeHandler) error {
	return RunIssueOpsWithExecutionHandlersAndReseedResumeAndReconcile(args, claim, release, reseed, resume, nil)
}

func RunIssueOpsWithExecutionHandlersAndReseedResumeAndReconcile(args []string, claim issueops.ExecutionClaimHandler, release issueops.ExecutionReleaseHandler, reseed issueops.ExecutionReseedHandler, resume issueops.ExecutionResumeHandler, reconcile issueops.ExecutionReconcileHandler) error {
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
