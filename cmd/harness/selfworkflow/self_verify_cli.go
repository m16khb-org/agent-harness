package selfworkflow

import (
	"agent-harness/cmd/harness/selfworkflow/candidatescmd"
	"agent-harness/cmd/harness/selfworkflow/promotecmd"
	"agent-harness/cmd/harness/selfworkflow/runmode"
	"agent-harness/cmd/harness/selfworkflow/verifycmd"
	"fmt"
)

type SelfVerifyRunDeps = verifycmd.Deps

type SelfVerifyRunMode = runmode.Mode

type SelfVerifyPromoteDeps struct {
	Promote func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error)
}

type SelfVerifyCandidatesDeps struct {
	Export func() SelfVerificationCandidateExportResult
	Save   func(result *SelfVerificationCandidateExportResult, key string) error
}

func ResolveSelfVerifyRunMode(full bool, iterationsFlagSet bool, iterations int) (SelfVerifyRunMode, error) {
	return runmode.Resolve(full, iterationsFlagSet, iterations)
}

func RunSelfVerify(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return RunSelfVerifyHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return RunSelfVerifyCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return RunSelfVerifyPromote(args[1:])
	}
	if len(args) > 0 && args[0] == "candidates" {
		return RunSelfVerifyCandidates(args[1:])
	}
	return RunSelfVerifyWithDeps(args, SelfVerifyRunDeps{})
}

func RunSelfVerifyWithDeps(args []string, deps SelfVerifyRunDeps) error {
	if deps.NewProgressReporter == nil {
		deps.NewProgressReporter = NewSelfVerifyProgressReporter
	}
	if deps.Verify == nil {
		deps.Verify = func(int, int64, float64, bool, *SelfVerifyProgressReporter) (SelfAugmentResult, error) {
			return SelfAugmentResult{}, fmt.Errorf("self-verify runner dependency is required")
		}
	}
	if deps.ApplyLLMEval == nil {
		deps.ApplyLLMEval = ApplySelfVerifyLLMEval
	}
	if deps.SaveSummary == nil {
		deps.SaveSummary = SaveSelfVerificationSummary
	}
	if deps.PrintJSON == nil {
		deps.PrintJSON = printJSON
	}
	return verifycmd.Run(args, deps)
}

func RunSelfVerifyPromote(args []string) error {
	return RunSelfVerifyPromoteWithDeps(args, SelfVerifyPromoteDeps{})
}

func RunSelfVerifyPromoteWithDeps(args []string, deps SelfVerifyPromoteDeps) error {
	deps = deps.withDefaults()
	return promotecmd.Run(args, promotecmd.Deps{
		Promote:   deps.Promote,
		PrintJSON: printJSON,
	})
}

func RunSelfVerifyCandidates(args []string) error {
	return RunSelfVerifyCandidatesWithDeps(args, SelfVerifyCandidatesDeps{})
}

func RunSelfVerifyCandidatesWithDeps(args []string, deps SelfVerifyCandidatesDeps) error {
	deps = deps.withDefaults()
	return candidatescmd.Run(args, candidatescmd.Deps{
		Export:    deps.Export,
		Save:      deps.Save,
		PrintJSON: printJSON,
	})
}

func (deps SelfVerifyPromoteDeps) withDefaults() SelfVerifyPromoteDeps {
	if deps.Promote == nil {
		deps.Promote = PromoteSelfAugmentBaseline
	}
	return deps
}

func (deps SelfVerifyCandidatesDeps) withDefaults() SelfVerifyCandidatesDeps {
	if deps.Export == nil {
		deps.Export = ExportSelfVerificationCandidates
	}
	if deps.Save == nil {
		deps.Save = SaveSelfVerificationCandidateExport
	}
	return deps
}
