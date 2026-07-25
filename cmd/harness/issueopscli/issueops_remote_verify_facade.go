package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	"agent-harness/internal/core"
)

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return remoteverify.VerifyChildIssueBeforeLink(childURL)
}

func verifyIssueOpsRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
	return remoteverify.VerifyRemoteArtifactLive(req)
}

func verifyIssueOpsRemoteArtifactMergedLive(artifact core.IssueOpsRemoteArtifactVerification) error {
	return remoteverify.VerifyRemoteArtifactMergedLive(artifact)
}

func verifyIssueOpsRemoteArtifactMergedHeadLive(artifact core.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
	return remoteverify.VerifyRemoteArtifactMergedHeadLive(artifact)
}
