package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type selfVerifyRunDeps struct {
	lookupEnv           func(string) (string, bool)
	progressWriter      io.Writer
	newProgressReporter func(string, io.Writer) (*selfVerifyProgressReporter, error)
	verify              func(int, int64, float64, bool, *selfVerifyProgressReporter) (SelfAugmentResult, error)
	applyLLMEval        func(SelfAugmentResult, SelfVerifyLLMEvalOptions) (SelfAugmentResult, error)
	saveSummary         func(*SelfAugmentResult, string) error
}

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
	return runSelfVerifyWithDeps(args, selfVerifyRunDeps{})
}

func runSelfVerifyWithDeps(args []string, deps selfVerifyRunDeps) error {
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
	runMode, err := resolveSelfVerifyRunMode(*full, iterationsFlagSet, *iterations)
	if err != nil {
		return err
	}
	llmEvalFlagSet := flagSetVisited(fs, "llm-eval")
	llmEvalModeFlagSet := flagSetVisited(fs, "llm-eval-mode")
	llmEvalConfig, err := resolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, *llmEval, *llmEvalMode, llmEvalModeFlagSet, deps.lookupEnv)
	if err != nil {
		return err
	}
	progressReporter, err := deps.newProgressReporter(*progress, deps.progressWriter)
	if err != nil {
		return err
	}
	result, err := deps.verify(runMode.Iterations, *seed, *targetScore, !*jsonOut, progressReporter)
	if err == nil && llmEvalConfig.Enabled {
		result, err = deps.applyLLMEval(result, SelfVerifyLLMEvalOptions{
			Enabled:     true,
			Mode:        llmEvalConfig.Mode,
			AgyCommand:  *agyCommand,
			TargetScore: *targetScore,
		})
	}
	saveErr := error(nil)
	if *saveState {
		saveErr = deps.saveSummary(&result, *stateKey)
	}
	if *jsonOut {
		_ = printJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
}

func (deps selfVerifyRunDeps) withDefaults() selfVerifyRunDeps {
	if deps.lookupEnv == nil {
		deps.lookupEnv = os.LookupEnv
	}
	if deps.progressWriter == nil {
		deps.progressWriter = os.Stderr
	}
	if deps.newProgressReporter == nil {
		deps.newProgressReporter = newSelfVerifyProgressReporter
	}
	if deps.verify == nil {
		deps.verify = selfVerifyWithProgress
	}
	if deps.applyLLMEval == nil {
		deps.applyLLMEval = applySelfVerifyLLMEval
	}
	if deps.saveSummary == nil {
		deps.saveSummary = saveSelfVerificationSummary
	}
	return deps
}
