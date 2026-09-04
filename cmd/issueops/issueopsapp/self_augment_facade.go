package issueopsapp

import "issueops/cmd/issueops/selfworkflow"

func runSelfAugment(args []string) error {
	selfworkflow.Version = version
	selfworkflow.IssueOpsRoot = issueOpsRoot
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
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.PlanSelfAugmentation(req)
}

func saveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return selfworkflow.SaveSelfAugmentPlan(result, key)
}

func runSelfAugmentLesson(args []string) error {
	selfworkflow.Version = version
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.RunSelfAugmentLesson(args)
}
