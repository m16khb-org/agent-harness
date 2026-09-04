package statuscli

type (
	Status                 = HarnessStatus
	SelfVerificationStatus = SelfVerifyStatus
	WorkResult             = VerifyWorkResult
	WorkEvidenceItem       = VerifyWorkEvidenceItem
	WorkSuggestedCommand   = VerifyWorkSuggestedCommand
)

func RunStatus(args []string) error {
	return runStatus(args)
}

func BuildStatus(repo string) Status {
	return buildHarnessStatus(repo)
}

func RunVerifyWork(args []string) error {
	return runVerifyWork(args)
}

func BuildVerifyWork(repo string, all bool, argv []string) WorkResult {
	return buildVerifyWork(repo, all, argv)
}
