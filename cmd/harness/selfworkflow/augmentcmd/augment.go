package augmentcmd

import (
	"flag"
	"fmt"
	"io"
	"os"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
)

type Deps struct {
	Output              io.Writer
	RunLesson           func([]string) error
	RunVerify           func([]string) error
	Plan                func(model.SelfAugmentPlanRequest) model.SelfAugmentPlanResult
	SavePlan            func(*model.SelfAugmentPlanResult, string) error
	PrintJSON           func(any) error
	SelectedCandidateID func(*model.SelfAugmentCandidate) string
	DefaultTargetScore  float64
}

func Run(args []string, deps Deps) error {
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
	targetScore := fs.Float64("target-score", deps.DefaultTargetScore, "exclusive per-goal score threshold; every concrete goal must score above this value before termination")
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
	result := deps.Plan(model.SelfAugmentPlanRequest{Cycles: *cycles, TargetScore: *targetScore})
	if *saveState {
		if err := deps.SavePlan(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	_, _ = fmt.Fprintf(deps.Output, "%s plan: %d candidate(s), selected=%s, termination_eligible=%v\n", result.KoreanName, len(result.Candidates), deps.SelectedCandidateID(result.SelectedCandidate), result.TerminationEligible)
	for _, goal := range result.Goals {
		_, _ = fmt.Fprintf(deps.Output, "- %s score=%.1f target>%.1f passed=%v\n", goal.KoreanName, goal.Score, goal.TargetScore, goal.Passed)
	}
	if result.SelectedCandidate != nil {
		_, _ = fmt.Fprintf(deps.Output, "selected: %s \u2014 %s\n", result.SelectedCandidate.ID, result.SelectedCandidate.Title)
	}
	return nil
}

func (deps Deps) withDefaults() Deps {
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	if deps.RunLesson == nil {
		deps.RunLesson = func([]string) error {
			return fmt.Errorf("self-augment lesson runner dependency is required")
		}
	}
	if deps.RunVerify == nil {
		deps.RunVerify = func([]string) error {
			return fmt.Errorf("self-verify runner dependency is required")
		}
	}
	if deps.Plan == nil {
		deps.Plan = func(model.SelfAugmentPlanRequest) model.SelfAugmentPlanResult {
			return model.SelfAugmentPlanResult{OK: false}
		}
	}
	if deps.SavePlan == nil {
		deps.SavePlan = func(*model.SelfAugmentPlanResult, string) error {
			return fmt.Errorf("self-augment state dependency is required")
		}
	}
	if deps.PrintJSON == nil {
		deps.PrintJSON = func(any) error {
			return fmt.Errorf("JSON printer dependency is required")
		}
	}
	if deps.SelectedCandidateID == nil {
		deps.SelectedCandidateID = augmentcatalog.SelectedCandidateID
	}
	if deps.DefaultTargetScore == 0 {
		deps.DefaultTargetScore = model.DefaultLoopTargetScoreExclusive
	}
	return deps
}
