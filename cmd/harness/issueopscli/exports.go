package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	"agent-harness/cmd/harness/issueopscli/worktreetools"
	"agent-harness/internal/core"
)

type WorktreeToolPrepareResult = worktreetools.PrepareResult

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
	return remoteverify.SetChildIssueVerifier(verifier)
}
