package verifycmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"agent-harness/cmd/harness/selfworkflow/llmeval"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
	"agent-harness/cmd/harness/selfworkflow/verifyloop"
)

type Deps struct {
	LookupEnv           func(string) (string, bool)
	ProgressWriter      io.Writer
	NewProgressReporter func(string, io.Writer) (*progress.SelfVerifyProgressReporter, error)
	Verify              func(verifyloop.Request) (model.SelfAugmentResult, error)
	ApplyLLMEval        func(model.SelfAugmentResult, llmeval.SelfVerifyLLMEvalOptions) (model.SelfAugmentResult, error)
	SaveSummary         func(*model.SelfAugmentResult, string) error
	PrintJSON           func(any) error
}

func Run(args []string, deps Deps) error {
	deps = deps.withDefaults()
	fs := flag.NewFlagSet("self-verify", flag.ContinueOnError)
	seed := fs.Int64("seed", time.Now().Unix(), "seed for randomized checks")
	targetScore := fs.Float64("target-score", 95, "exclusive per-goal score threshold; every concrete goal must score above this value to terminate")
	saveState := fs.Bool("save-state", false, "save compact self-verification summary to harness state")
	stateKey := fs.String("state-key", "self-verify-latest", "state key for --save-state")
	progressMode := fs.String("progress", "none", "progress output mode: none or jsonl; jsonl writes JSON Lines events to stderr")
	llmEval := fs.Bool("llm-eval", false, "run opt-in host-agent judgement prompt after deterministic self-verification")
	llmEvalMode := fs.String("llm-eval-mode", "advisory", "LLM evaluation mode: advisory or gate")
	jsonOut := fs.Bool("json", false, "print JSON summary")
	collectAll := fs.Bool("collect-all-steps", false, "run every gate in the evidence pass and surface ALL failures (concurrent regression diagnosis); default fail-fast. Never weakens the gate — any failure still fails self-verify")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	llmEvalFlagSet := flagSetVisited(fs, "llm-eval")
	llmEvalModeFlagSet := flagSetVisited(fs, "llm-eval-mode")
	llmEvalConfig, err := llmeval.ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, *llmEval, *llmEvalMode, llmEvalModeFlagSet, deps.LookupEnv)
	if err != nil {
		return err
	}
	reporter, err := deps.NewProgressReporter(*progressMode, deps.ProgressWriter)
	if err != nil {
		return err
	}
	result, err := deps.Verify(verifyloop.Request{
		BaseSeed:        *seed,
		TargetScore:     *targetScore,
		Verbose:         !*jsonOut,
		Reporter:        reporter,
		CollectAllSteps: *collectAll,
	})
	if err == nil && llmEvalConfig.Enabled {
		result, err = deps.ApplyLLMEval(result, llmeval.SelfVerifyLLMEvalOptions{
			Enabled:     true,
			Mode:        llmEvalConfig.Mode,
			TargetScore: *targetScore,
		})
	}
	saveErr := error(nil)
	if *saveState {
		saveErr = deps.SaveSummary(&result, *stateKey)
	}
	if *jsonOut {
		_ = deps.PrintJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
}

func (deps Deps) withDefaults() Deps {
	if deps.LookupEnv == nil {
		deps.LookupEnv = os.LookupEnv
	}
	if deps.ProgressWriter == nil {
		deps.ProgressWriter = os.Stderr
	}
	if deps.NewProgressReporter == nil {
		deps.NewProgressReporter = progress.NewSelfVerifyProgressReporter
	}
	if deps.Verify == nil {
		deps.Verify = func(verifyloop.Request) (model.SelfAugmentResult, error) {
			return model.SelfAugmentResult{}, fmt.Errorf("self-verify runner dependency is required")
		}
	}
	if deps.ApplyLLMEval == nil {
		deps.ApplyLLMEval = llmeval.ApplySelfVerifyLLMEval
	}
	if deps.SaveSummary == nil {
		deps.SaveSummary = func(*model.SelfAugmentResult, string) error {
			return fmt.Errorf("self-verify state dependency is required")
		}
	}
	if deps.PrintJSON == nil {
		deps.PrintJSON = func(any) error {
			return fmt.Errorf("JSON printer dependency is required")
		}
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
