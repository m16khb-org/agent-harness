package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/verifyloop"

var ErrSelfVerificationGateFailed = verifyloop.ErrSelfVerificationGateFailed

type SelfVerifyLoopDeps = verifyloop.Deps

func SelfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	return SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, deps)
}

func SelfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *SelfVerifyProgressReporter, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	if deps.HarnessRoot == nil {
		deps.HarnessRoot = HarnessRoot
	}
	return verifyloop.SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, progress, deps)
}
