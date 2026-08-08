package harnessapp

import "agent-harness/cmd/harness/selfworkflow"

func runSelfAugment(args []string) error {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.RunSelfAugmentWithDeps(args, selfworkflow.SelfAugmentRunDeps{
		RunLesson: runSelfAugmentLesson,
		RunVerify: runSelfVerify,
		Plan:      planSelfAugmentation,
		SavePlan:  saveSelfAugmentPlan,
		PrintJSON: printJSON,
	})
}

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.PlanSelfAugmentation(req)
}

func saveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return selfworkflow.SaveSelfAugmentPlan(result, key)
}

func runSelfAugmentLesson(args []string) error {
	selfworkflow.Version = version
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.RunSelfAugmentLesson(args)
}
