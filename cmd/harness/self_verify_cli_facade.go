package main

import "agent-harness/cmd/harness/selfworkflow"

func runSelfVerify(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return runSelfVerifyHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runSelfVerifyCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return runSelfVerifyPromote(args[1:])
	}
	if len(args) > 0 && args[0] == "candidates" {
		return runSelfVerifyCandidates(args[1:])
	}
	return selfworkflow.RunSelfVerifyWithDeps(args, selfworkflow.SelfVerifyRunDeps{
		Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfworkflow.SelfVerifyProgressReporter) (SelfAugmentResult, error) {
			if progress == nil {
				return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil)
			}
			return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, &selfVerifyProgressReporter{inner: progress})
		},
	})
}
