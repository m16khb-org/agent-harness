package main

import (
	"time"

	"agent-harness/cmd/harness/validationcli"
)

type stepBudgetCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult
type stepBudgetSnapshotWriter func(dir, key string, snapshot SelfAugmentStateSnapshot) error

type stepBudgetValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	writeSnapshot stepBudgetSnapshotWriter
	run           stepBudgetCommandRunner
}

func validateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStepBudgetBaseline(binary, root, seed)
}

func validateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps stepBudgetValidationDeps) StepResult {
	return validationcli.ValidateStepBudgetBaselineWithDeps(binary, root, seed, validationcli.StepBudgetValidationDeps{
		MakeTempState: deps.makeTempState,
		RemoveAll:     deps.removeAll,
		WriteSnapshot: validationcli.StepBudgetSnapshotWriter(deps.writeSnapshot),
		Run:           validationcli.StepBudgetCommandRunner(deps.run),
	})
}

func stepBudgetBaselineSummaries(seed int64) (SelfAugmentSummary, SelfAugmentSummary) {
	return validationcli.StepBudgetBaselineSummaries(seed)
}

func stepBudgetStateSnapshot(root string, seed int64, summary SelfAugmentSummary) SelfAugmentStateSnapshot {
	return validationcli.StepBudgetStateSnapshot(root, seed, summary)
}

func stepBudgetValidationErrors(result SelfAugmentCompareResult) []string {
	return validationcli.StepBudgetValidationErrors(result)
}
