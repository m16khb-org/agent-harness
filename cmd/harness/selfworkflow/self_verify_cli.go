package selfworkflow

import (
	"agent-harness/cmd/harness/selfworkflow/candidatescmd"
	"agent-harness/cmd/harness/selfworkflow/promotecmd"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type SelfVerifyRunDeps struct {
	LookupEnv           func(string) (string, bool)
	ProgressWriter      io.Writer
	NewProgressReporter func(string, io.Writer) (*SelfVerifyProgressReporter, error)
	Verify              func(int, int64, float64, bool, *SelfVerifyProgressReporter) (SelfAugmentResult, error)
	ApplyLLMEval        func(SelfAugmentResult, SelfVerifyLLMEvalOptions) (SelfAugmentResult, error)
	SaveSummary         func(*SelfAugmentResult, string) error
}

type SelfVerifyRunMode struct {
	Full          bool
	Iterations    int
	ContractLabel string
}

type SelfVerifyPromoteDeps struct {
	Promote func(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error)
}

type SelfVerifyCandidatesDeps struct {
	Export func() SelfVerificationCandidateExportResult
	Save   func(result *SelfVerificationCandidateExportResult, key string) error
}

func ResolveSelfVerifyRunMode(full bool, iterationsFlagSet bool, iterations int) (SelfVerifyRunMode, error) {
	if !full {
		if iterationsFlagSet {
			return SelfVerifyRunMode{}, fmt.Errorf("--iterations requires --full; default self-verify runs quick one-iteration mode")
		}
		return SelfVerifyRunMode{Full: false, Iterations: 1, ContractLabel: "quick one-iteration gate"}, nil
	}
	if iterations < 10 {
		return SelfVerifyRunMode{}, fmt.Errorf("full self-verification requires at least 10 iterations; use --full --iterations=10 or higher")
	}
	return SelfVerifyRunMode{Full: true, Iterations: iterations, ContractLabel: "full ten-plus-iteration gate"}, nil
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
	deps = deps.withDefaults()
	fs := flag.NewFlagSet("self-verify", flag.ContinueOnError)
	full := fs.Bool("full", false, "run the full verification gate; defaults to quick one-iteration mode")
	iterations := fs.Int("iterations", 10, "number of full self-verification loop iterations; requires --full and must be at least 10")
	seed := fs.Int64("seed", time.Now().Unix(), "base seed for randomized checks")
	targetScore := fs.Float64("target-score", 95, "exclusive per-goal score threshold; every concrete goal must score above this value to terminate")
	saveState := fs.Bool("save-state", false, "save compact self-verification summary to harness state")
	stateKey := fs.String("state-key", "self-verify-latest", "state key for --save-state")
	progress := fs.String("progress", "none", "progress output mode: none or jsonl; jsonl writes JSON Lines events to stderr")
	llmEval := fs.Bool("llm-eval", false, "run opt-in agy -p LLM evaluation after deterministic self-verification")
	llmEvalMode := fs.String("llm-eval-mode", "advisory", "LLM evaluation mode: advisory or gate")
	agyCommand := fs.String("agy-command", "agy", "agy executable path for --llm-eval")
	jsonOut := fs.Bool("json", false, "print JSON summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	iterationsFlagSet := flagSetVisited(fs, "iterations")
	runMode, err := ResolveSelfVerifyRunMode(*full, iterationsFlagSet, *iterations)
	if err != nil {
		return err
	}
	llmEvalFlagSet := flagSetVisited(fs, "llm-eval")
	llmEvalModeFlagSet := flagSetVisited(fs, "llm-eval-mode")
	llmEvalConfig, err := ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, *llmEval, *llmEvalMode, llmEvalModeFlagSet, deps.LookupEnv)
	if err != nil {
		return err
	}
	progressReporter, err := deps.NewProgressReporter(*progress, deps.ProgressWriter)
	if err != nil {
		return err
	}
	result, err := deps.Verify(runMode.Iterations, *seed, *targetScore, !*jsonOut, progressReporter)
	if err == nil && llmEvalConfig.Enabled {
		result, err = deps.ApplyLLMEval(result, SelfVerifyLLMEvalOptions{
			Enabled:     true,
			Mode:        llmEvalConfig.Mode,
			AgyCommand:  *agyCommand,
			TargetScore: *targetScore,
		})
	}
	saveErr := error(nil)
	if *saveState {
		saveErr = deps.SaveSummary(&result, *stateKey)
	}
	if *jsonOut {
		_ = printJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
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

func (deps SelfVerifyRunDeps) withDefaults() SelfVerifyRunDeps {
	if deps.LookupEnv == nil {
		deps.LookupEnv = os.LookupEnv
	}
	if deps.ProgressWriter == nil {
		deps.ProgressWriter = os.Stderr
	}
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
	return deps
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

func flagSetVisited(fs *flag.FlagSet, name string) bool {
	visited := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			visited = true
		}
	})
	return visited
}
