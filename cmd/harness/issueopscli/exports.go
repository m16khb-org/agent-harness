package issueopscli

import "agent-harness/internal/core"

type WorktreeToolPrepareResult = issueOpsWorktreeToolPrepareResult

func RunIssueOps(args []string) error {
	return runIssueOps(args)
}

func PrepareWorktreeTools(record core.IssueOpsRecord) (WorktreeToolPrepareResult, error) {
	return prepareIssueOpsWorktreeTools(record)
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
	previous := issueOpsChildIssueVerifier
	issueOpsChildIssueVerifier = verifier
	return previous
}
