package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core"
)

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return remoteverify.VerifyChildIssueBeforeLink(childURL)
}

func verifyIssueOpsRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return remoteverify.VerifyRemoteArtifactLive(req)
}

func verifyIssueOpsRemoteArtifactMergedLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) error {
	return remoteverify.VerifyRemoteArtifactMergedLive(artifact)
}

func verifyIssueOpsRemoteArtifactMergedHeadLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
	return remoteverify.VerifyRemoteArtifactMergedHeadLive(artifact)
}
