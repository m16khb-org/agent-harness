package main

import "agent-harness/cmd/harness/selfworkflow"

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.PlanSelfAugmentation(req)
}

func saveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return selfworkflow.SaveSelfAugmentPlan(result, key)
}

func saveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.SaveSelfAugmentLesson(req)
}

func runSelfAugmentLesson(args []string) error {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.RunSelfAugmentLesson(args)
}

func stateKeySlug(s string) string {
	return selfworkflow.StateKeySlug(s)
}

func selfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return selfworkflow.SelfAugmentCandidateIDsByStatus(candidates, status)
}
