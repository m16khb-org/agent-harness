package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/augmentlesson"

func runSelfAugmentLesson(args []string) error {
	return augmentlesson.RunSelfAugmentLesson(args, selfAugmentLessonDeps())
}

func saveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	return augmentlesson.SaveSelfAugmentLesson(req, selfAugmentLessonDeps())
}

func stateKeySlug(s string) string {
	return augmentlesson.StateKeySlug(s)
}

func selfAugmentLessonDeps() augmentlesson.Deps {
	return augmentlesson.Deps{
		HarnessRoot: HarnessRoot,
		PrintJSON:   printJSON,
		SelectCandidate: func() *SelfAugmentCandidate {
			plan := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: defaultLoopTargetScoreExclusive})
			return plan.SelectedCandidate
		},
	}
}
