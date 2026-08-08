package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/remoteverify"
	issueopscontract "agent-harness/internal/contract/issueops"
	issueopscore "agent-harness/internal/contract/issueops"
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

func verifyIssueOpsRemoteArtifactMergedHeadLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscore.CleanupRemoteBranchArtifactHead, error) {
	return remoteverify.VerifyRemoteArtifactMergedHeadLive(artifact)
}

func observeIssueOpsRemoteArtifactMergedLive(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (bool, error) {
	return remoteverify.ObserveRemoteArtifactMergedLive(artifact)
}
