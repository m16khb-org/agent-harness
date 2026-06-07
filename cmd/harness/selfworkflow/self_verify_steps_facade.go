package selfworkflow

import (
	"agent-harness/cmd/harness/selfworkflow/steps"
)

type SelfVerifyPlannedStep = steps.SelfVerifyPlannedStep
type SelfVerifyStepDeps = steps.SelfVerifyStepDeps

func PlannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	return steps.PlannedSelfVerifySteps(root, tempBin, seed, goTestStep, deps)
}

func CachedContractGoldenStep(goTestStep StepResult, deps SelfVerifyStepDeps) StepResult {
	return steps.CachedContractGoldenStep(goTestStep, deps)
}
