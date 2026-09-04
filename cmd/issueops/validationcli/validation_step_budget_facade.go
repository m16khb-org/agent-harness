package validationcli

import "issueops/cmd/issueops/validationcli/stepbudget"

type StepBudgetCommandRunner = stepbudget.StepBudgetCommandRunner
type StepBudgetSnapshotWriter = stepbudget.StepBudgetSnapshotWriter
type StepBudgetValidationDeps = stepbudget.StepBudgetValidationDeps

func ValidateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return stepbudget.ValidateStepBudgetBaseline(binary, root, seed)
}

func ValidateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps StepBudgetValidationDeps) StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, deps)
}

func StepBudgetBaselineSummaries(seed int64) (SelfAugmentSummary, SelfAugmentSummary) {
	return stepbudget.StepBudgetBaselineSummaries(seed)
}

func StepBudgetStateSnapshot(root string, seed int64, summary SelfAugmentSummary) SelfAugmentStateSnapshot {
	return stepbudget.StepBudgetStateSnapshot(root, seed, summary)
}

func StepBudgetValidationErrors(result SelfAugmentCompareResult) []string {
	return stepbudget.StepBudgetValidationErrors(result)
}
