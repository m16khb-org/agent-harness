package selfworkflow

import (
	"fmt"
	"os"

	"agent-harness/cmd/harness/selfworkflow/augmentcmd"
)

type SelfAugmentRunDeps = augmentcmd.Deps

func RunSelfAugment(args []string) error {
	return RunSelfAugmentWithDeps(args, SelfAugmentRunDeps{})
}

func RunSelfAugmentWithDeps(args []string, deps SelfAugmentRunDeps) error {
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
	if deps.SelectedCandidateID == nil {
		deps.SelectedCandidateID = SelectedCandidateID
	}
	if deps.DefaultTargetScore == 0 {
		deps.DefaultTargetScore = defaultLoopTargetScoreExclusive
	}
	return augmentcmd.Run(args, deps)
}
