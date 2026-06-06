package main

import "agent-harness/cmd/harness/statuscli"

type (
	HarnessStatus              = statuscli.Status
	SelfVerifyStatus           = statuscli.SelfVerificationStatus
	VerifyWorkResult           = statuscli.WorkResult
	VerifyWorkEvidenceItem     = statuscli.WorkEvidenceItem
	VerifyWorkSuggestedCommand = statuscli.WorkSuggestedCommand
)

func init() {
	statuscli.HarnessRoot = harnessRoot
	statuscli.ResolveTarget = resolveTarget
	statuscli.Version = version
	statuscli.InspectHarness = inspectHarness
	statuscli.CheckDaemonStatus = checkDaemonStatus
}

func runStatus(args []string) error {
	return statuscli.RunStatus(args)
}

func buildHarnessStatus(repo string) HarnessStatus {
	return statuscli.BuildStatus(repo)
}

func runVerifyWork(args []string) error {
	return statuscli.RunVerifyWork(args)
}

func buildVerifyWork(repo string, all bool, argv []string) VerifyWorkResult {
	return statuscli.BuildVerifyWork(repo, all, argv)
}
