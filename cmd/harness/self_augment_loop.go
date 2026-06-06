package main

import (
	"flag"
	"fmt"
)

func runSelfAugment(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "lesson":
			return runSelfAugmentLesson(args[1:])
		case "verify":
			return runSelfVerify(args[1:])
		case "history", "compare", "promote":
			return runSelfVerify(args)
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
	result := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: *cycles, TargetScore: *targetScore})
	if *saveState {
		if err := saveSelfAugmentPlan(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s plan: %d candidate(s), selected=%s, termination_eligible=%v\n", result.KoreanName, len(result.Candidates), selectedCandidateID(result.SelectedCandidate), result.TerminationEligible)
	for _, goal := range result.Goals {
		fmt.Printf("- %s score=%.1f target>%.1f passed=%v\n", goal.KoreanName, goal.Score, goal.TargetScore, goal.Passed)
	}
	if result.SelectedCandidate != nil {
		fmt.Printf("selected: %s — %s\n", result.SelectedCandidate.ID, result.SelectedCandidate.Title)
	}
	return nil
}
