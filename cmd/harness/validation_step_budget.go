package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type stepBudgetCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult
type stepBudgetSnapshotWriter func(dir, key string, snapshot SelfAugmentStateSnapshot) error

type stepBudgetValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	writeSnapshot stepBudgetSnapshotWriter
	run           stepBudgetCommandRunner
}

func (deps stepBudgetValidationDeps) withDefaults() stepBudgetValidationDeps {
	if deps.makeTempState == nil {
		deps.makeTempState = func(seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("agent-harness-budget-%d-*", seed))
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeSnapshot == nil {
		deps.writeSnapshot = writeSelfAugmentSnapshotRecord
	}
	if deps.run == nil {
		deps.run = runCommandStepEnv
	}
	return deps
}

func validateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return validateStepBudgetBaselineWithDeps(binary, root, seed, stepBudgetValidationDeps{})
}

func validateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps stepBudgetValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempState, err := deps.makeTempState(seed)
	if err != nil {
		return failedStep("step budget baseline", err)
	}
	defer deps.removeAll(tempState)
	baselineKey := fmt.Sprintf("self-verify-budget-baseline-%d", seed)
	candidateKey := fmt.Sprintf("self-verify-budget-candidate-%d", seed)
	baselineSummary, candidateSummary := stepBudgetBaselineSummaries(seed)
	for _, fixture := range []struct {
		key     string
		summary SelfAugmentSummary
	}{
		{key: baselineKey, summary: baselineSummary},
		{key: candidateKey, summary: candidateSummary},
	} {
		if err := deps.writeSnapshot(tempState, fixture.key, stepBudgetStateSnapshot(root, seed, fixture.summary)); err != nil {
			return failedStep("step budget baseline", err)
		}
	}

	env := []string{"HARNESS_STATE_DIR=" + tempState}
	compareStep := deps.run(root, "step budget baseline", 30*time.Second, "", env, binary, "self-verify", "compare", "--baseline-key", baselineKey, "--candidate-key", candidateKey, "--max-elapsed-regression-pct", "5", "--json")
	stdoutParts := []string{compareStep.Stdout}
	commands := []string{compareStep.Command}
	if !compareStep.OK {
		return combineFailedStep("step budget baseline", started, compareStep, stdoutParts, commands)
	}
	var result SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareStep.Stdout), &result); err != nil {
		return assertionStepWithOutput("step budget baseline", started, []string{err.Error()}, stdoutParts, commands)
	}
	errs := stepBudgetValidationErrors(result)
	if len(errs) > 0 {
		return assertionStepWithOutput("step budget baseline", started, errs, stdoutParts, commands)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "step budget baseline",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func stepBudgetBaselineSummaries(seed int64) (SelfAugmentSummary, SelfAugmentSummary) {
	baselineSummary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "docs index smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: seed, Label: "go test", DurationMS: 2000},
		},
		StepDurationStats: []SelfAugmentStepDurationStat{
			{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 100, AverageDurationMS: 95, P95DurationMS: 100},
			{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
		},
	}
	candidateSummary := baselineSummary
	candidateSummary.StepDurationStats = []SelfAugmentStepDurationStat{
		{Label: "docs index smoke", Count: 10, MinDurationMS: 90, MaxDurationMS: 130, AverageDurationMS: 105, P95DurationMS: 130},
		{Label: "go test", Count: 10, MinDurationMS: 1800, MaxDurationMS: 2000, AverageDurationMS: 1900, P95DurationMS: 2000},
	}
	return baselineSummary, candidateSummary
}

func stepBudgetStateSnapshot(root string, seed int64, summary SelfAugmentSummary) SelfAugmentStateSnapshot {
	return SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          selfVerificationSummaryKind,
		LoopKind:      "self_verification",
		KoreanName:    selfVerificationKoreanName,
		OK:            true,
		Iterations:    10,
		BaseSeed:      seed,
		TargetScore:   95,
		ElapsedMS:     1000,
		HarnessRoot:   root,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}
}

func stepBudgetValidationErrors(result SelfAugmentCompareResult) []string {
	errs := []string{}
	if !result.OK || !result.Regressed {
		errs = append(errs, "step budget compare did not report a regression")
	}
	if len(result.SlowStepRegressions) != 0 {
		errs = append(errs, "step budget regression should not depend on slowest_steps top entries")
	}
	if len(result.StepBudgetRegressions) != 1 {
		errs = append(errs, "step budget compare did not report exactly one budget regression")
	} else {
		regression := result.StepBudgetRegressions[0]
		if regression.Label != "docs index smoke" || regression.Metric != "p95_duration_ms" || regression.DeltaMS != 30 || regression.DeltaPct != 30 {
			errs = append(errs, "step budget regression details mismatch")
		}
	}
	if !containsString(result.Regressions, "step_budget:docs index smoke_p95_increased_by_30.00_pct") {
		errs = append(errs, "step budget regression marker missing")
	}
	return errs
}
