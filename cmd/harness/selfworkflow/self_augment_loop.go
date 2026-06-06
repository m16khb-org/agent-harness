package selfworkflow

import (
	"flag"
	"fmt"
	"io"
	"os"
)

type SelfAugmentRunDeps struct {
	Output    io.Writer
	RunLesson func([]string) error
	RunVerify func([]string) error
	Plan      func(SelfAugmentPlanRequest) SelfAugmentPlanResult
	SavePlan  func(*SelfAugmentPlanResult, string) error
	PrintJSON func(any) error
}

func RunSelfAugment(args []string) error {
	return RunSelfAugmentWithDeps(args, SelfAugmentRunDeps{})
}

func RunSelfAugmentWithDeps(args []string, deps SelfAugmentRunDeps) error {
	deps = deps.withDefaults()
	if len(args) > 0 {
		switch args[0] {
		case "lesson":
			return deps.RunLesson(args[1:])
		case "verify":
			return deps.RunVerify(args[1:])
		case "history", "compare", "promote":
			return deps.RunVerify(args)
		}
	}
	fs := flag.NewFlagSet("self-augment", flag.ContinueOnError)
	cycles := fs.Int("cycles", 1, "number of autonomous improvement cycles to plan")
	targetScore := fs.Float64("target-score", defaultLoopTargetScoreExclusive, "exclusive per-goal score threshold; every concrete goal must score above this value before termination")
	saveState := fs.Bool("save-state", false, "save compact self-augmentation plan to harness state")
	stateKey := fs.String("state-key", "self-augment-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cycles < 1 {
		return fmt.Errorf("cycles must be positive")
	}
	if *targetScore < 0 || *targetScore >= 100 {
		return fmt.Errorf("target-score must be >= 0 and < 100")
	}
	result := deps.Plan(SelfAugmentPlanRequest{Cycles: *cycles, TargetScore: *targetScore})
	if *saveState {
		if err := deps.SavePlan(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	_, _ = fmt.Fprintf(deps.Output, "%s plan: %d candidate(s), selected=%s, termination_eligible=%v\n", result.KoreanName, len(result.Candidates), SelectedCandidateID(result.SelectedCandidate), result.TerminationEligible)
	for _, goal := range result.Goals {
		_, _ = fmt.Fprintf(deps.Output, "- %s score=%.1f target>%.1f passed=%v\n", goal.KoreanName, goal.Score, goal.TargetScore, goal.Passed)
	}
	if result.SelectedCandidate != nil {
		_, _ = fmt.Fprintf(deps.Output, "selected: %s — %s\n", result.SelectedCandidate.ID, result.SelectedCandidate.Title)
	}
	return nil
}

func (deps SelfAugmentRunDeps) withDefaults() SelfAugmentRunDeps {
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	if deps.RunLesson == nil {
		deps.RunLesson = RunSelfAugmentLesson
	}
	if deps.RunVerify == nil {
		deps.RunVerify = func([]string) error {
			return fmt.Errorf("self-verify runner dependency is required")
		}
	}
	if deps.Plan == nil {
		deps.Plan = PlanSelfAugmentation
	}
	if deps.SavePlan == nil {
		deps.SavePlan = SaveSelfAugmentPlan
	}
	if deps.PrintJSON == nil {
		deps.PrintJSON = printJSON
	}
	return deps
}
