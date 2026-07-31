package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
)

func RunIssueOps(args []string) error {
	return runIssueOps(args)
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
	if len(args) > 0 && args[0] == "execution" {
		return runIssueOpsExecutionWithHandlersAndReseed(args[1:], claim, release, reseed, resume)
	}
	return runIssueOps(args)
}

func VerifyChildIssueBeforeLink(childURL string) error {
	return verifyIssueOpsChildIssueBeforeLink(childURL)
}

func CleanupMerged(id string, requested bool) bool {
	return issueOpsCleanupMerged(id, requested)
}

func VerifyRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
	return verifyIssueOpsRemoteArtifactLive(req)
}

func SetChildIssueVerifier(verifier func(string) error) func(string) error {
	return remoteverify.SetChildIssueVerifier(verifier)
}
