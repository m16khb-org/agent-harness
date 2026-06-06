package main

import (
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/internal/core"
)

type issueOpsWorktreeToolPrepareResult = issueopscli.WorktreeToolPrepareResult

func runIssueOps(args []string) error {
	return issueopscli.RunIssueOps(args)
}

func prepareIssueOpsWorktreeTools(record core.IssueOpsRecord) (issueOpsWorktreeToolPrepareResult, error) {
	return issueopscli.PrepareWorktreeTools(record)
}

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return issueopscli.VerifyChildIssueBeforeLink(childURL)
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return issueopscli.CleanupMerged(id, requested)
}

func verifyIssueOpsRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
	return issueopscli.VerifyRemoteArtifactLive(req)
}
