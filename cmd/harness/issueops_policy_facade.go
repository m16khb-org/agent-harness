package main

import (
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/cmd/harness/policycli"
	"agent-harness/internal/core"
)

type issueOpsWorktreeToolPrepareResult = issueopscli.WorktreeToolPrepareResult

func init() {
	policycli.ResolveTarget = resolveTarget
}

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

func runPolicy(args []string) error {
	return policycli.Run(args)
}

func runPolicyCheck(args []string) error {
	return policycli.RunCheck(args)
}

func runPolicyFakeRun(args []string) error {
	return policycli.RunFakeRun(args)
}

func runPolicyRun(args []string) error {
	return policycli.RunReadOnly(args)
}

func runPolicyAudit(args []string) error {
	return policycli.RunAudit(args)
}

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	return policycli.ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (core.CommandPolicyRequest, bool, bool, error) {
	return policycli.ParseRunFlags(args)
}
