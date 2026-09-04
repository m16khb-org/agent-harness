package augmentlesson

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/selfworkflow/model"
)

func RunSelfAugmentLesson(args []string, deps Deps) error {
	fs := flag.NewFlagSet("self-augment lesson", flag.ContinueOnError)
	candidateID := fs.String("candidate", "", "candidate id this lesson belongs to; defaults to the current selected open candidate")
	lesson := fs.String("lesson", "", "Reflexion-style lesson learned from a failure, QA issue, or design concern")
	nextAction := fs.String("next-action", "", "specific next action that should use this lesson")
	source := fs.String("source", "self-augment", "source that produced the lesson")
	severity := fs.String("severity", "info", "lesson severity: info, warning, or error")
	stateKey := fs.String("state-key", "", "state key; defaults to self-augment-lesson-<candidate>-<timestamp>")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := SaveSelfAugmentLesson(model.SelfAugmentLessonRequest{
		CandidateID: *candidateID,
		Lesson:      *lesson,
		NextAction:  *nextAction,
		Source:      *source,
		Severity:    *severity,
		StateKey:    *stateKey,
	}, deps)
	if err != nil {
		return err
	}
	if *jsonOut {
		if deps.PrintJSON != nil {
			return deps.PrintJSON(result)
		}
		return fmt.Errorf("print JSON dependency is required")
	}
	fmt.Printf("self-augment lesson saved: candidate=%s key=%s\n", result.CandidateID, result.StateCheckpoint.Key)
	return nil
}
