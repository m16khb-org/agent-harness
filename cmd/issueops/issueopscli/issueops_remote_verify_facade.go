package issueopscli

import (
	"context"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	issueopscontract "issueops/internal/contract/issueops"
	issueopscore "issueops/internal/contract/issueops"
)

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return remoteverify.VerifyChildIssueBeforeLink(childURL)
}

func verifyIssueOpsRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return remoteverify.VerifyRemoteArtifactLive(req)
}

func verifyIssueOpsRemoteArtifactLiveContext(ctx context.Context, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return remoteverify.VerifyRemoteArtifactLiveContext(ctx, req)
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
