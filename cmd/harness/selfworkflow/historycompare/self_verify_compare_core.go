package historycompare

import (
	"fmt"
	"sort"
	"strings"

	"agent-harness/cmd/harness/selfworkflow/stateio"
	"agent-harness/cmd/harness/selfworkflow/summary"
	"agent-harness/internal/adapter/core"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	result := NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	if strings.TrimSpace(candidateKey) == "" {
		return result, fmt.Errorf("candidate-key is required")
	}
	if maxElapsedRegressionPct < 0 {
		return result, fmt.Errorf("max elapsed regression pct must be non-negative")
	}
	baseline, err := stateio.ReadSelfAugmentStateSnapshot(baselineKey)
	if err != nil {
		return result, fmt.Errorf("read baseline summary: %w", err)
	}
	candidate, err := stateio.ReadSelfAugmentStateSnapshot(candidateKey)
	if err != nil {
		return result, fmt.Errorf("read candidate summary: %w", err)
	}
	return CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate), nil
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	result := NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
	stateio.NormalizeSelfAugmentSnapshotFailureCause(&baseline)
	stateio.NormalizeSelfAugmentSnapshotFailureCause(&candidate)
	result.BaselineSummary = baseline.Summary
	result.CandidateSummary = candidate.Summary
	result.BaselineSnapshotGeneratedAt = baseline.GeneratedAt
	result.CandidateSnapshotGeneratedAt = candidate.GeneratedAt
	result.BaselineSlowestSteps = baseline.Summary.SlowestSteps
	result.CandidateSlowestSteps = candidate.Summary.SlowestSteps
	result.BaselineStepDurationStats = summary.StepDurationStatsForCompare(baseline.Summary)
	result.CandidateStepDurationStats = summary.StepDurationStatsForCompare(candidate.Summary)
	result.ElapsedDeltaMS = candidate.ElapsedMS - baseline.ElapsedMS
	result.BaselineMinimumGoalScore = baseline.Summary.MinimumGoalScore
	result.CandidateMinimumGoalScore = candidate.Summary.MinimumGoalScore
	if baseline.ElapsedMS > 0 {
		result.ElapsedDeltaPct = float64(result.ElapsedDeltaMS) * 100 / float64(baseline.ElapsedMS)
	} else if candidate.ElapsedMS > 0 {
		result.Warnings = append(result.Warnings, "baseline_elapsed_zero")
	}
	result.FailedStepsDelta = candidate.Summary.FailedSteps - baseline.Summary.FailedSteps
	result.TotalStepsDelta = candidate.Summary.TotalSteps - baseline.Summary.TotalSteps
	result.MissingStepLabels = MissingStrings(baseline.Summary.StepLabels, candidate.Summary.StepLabels)
	result.AddedStepLabels = MissingStrings(candidate.Summary.StepLabels, baseline.Summary.StepLabels)
	if baseline.OK && !candidate.OK {
		result.Regressions = append(result.Regressions, "candidate_not_ok")
	}
	if baseline.Summary.TerminationEligible && !candidate.Summary.TerminationEligible {
		result.Regressions = append(result.Regressions, "candidate_not_termination_eligible")
	}
	if baseline.Summary.MinimumGoalScore > 0 && candidate.Summary.MinimumGoalScore < baseline.Summary.MinimumGoalScore {
		result.Regressions = append(result.Regressions, fmt.Sprintf("minimum_goal_score_decreased_by_%.2f", baseline.Summary.MinimumGoalScore-candidate.Summary.MinimumGoalScore))
	}
	if result.FailedStepsDelta > 0 {
		result.Regressions = append(result.Regressions, fmt.Sprintf("failed_steps_increased_by_%d", result.FailedStepsDelta))
	}
	if result.ElapsedDeltaPct > maxElapsedRegressionPct {
		result.Regressions = append(result.Regressions, fmt.Sprintf("elapsed_ms_increased_by_%.2f_pct", result.ElapsedDeltaPct))
	}
	if baseline.Summary.FailedSteps > 0 && candidate.Summary.FailedSteps > 0 && baseline.Summary.FailureCause != candidate.Summary.FailureCause {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failure_cause_changed:%s->%s", baseline.Summary.FailureCause, candidate.Summary.FailureCause))
	}
	result.SlowStepRegressions = CompareSlowestStepRegressions(baseline.Summary.SlowestSteps, candidate.Summary.SlowestSteps, maxElapsedRegressionPct)
	for _, regression := range result.SlowStepRegressions {
		result.Regressions = append(result.Regressions, fmt.Sprintf("slow_step:%s_increased_by_%.2f_pct", regression.Label, regression.DeltaPct))
	}
	result.StepBudgetRegressions = CompareStepBudgetRegressions(result.BaselineStepDurationStats, result.CandidateStepDurationStats, maxElapsedRegressionPct)
	for _, regression := range result.StepBudgetRegressions {
		result.Regressions = append(result.Regressions, fmt.Sprintf("step_budget:%s_p95_increased_by_%.2f_pct", regression.Label, regression.DeltaPct))
	}
	for _, label := range result.MissingStepLabels {
		result.Regressions = append(result.Regressions, "missing_step_label:"+label)
	}
	if result.TotalStepsDelta != 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("total_steps_delta_%+d", result.TotalStepsDelta))
	}
	for _, label := range result.AddedStepLabels {
		result.Warnings = append(result.Warnings, "added_step_label:"+label)
	}
	sort.Strings(result.MissingStepLabels)
	sort.Strings(result.AddedStepLabels)
	sort.Strings(result.Regressions)
	sort.Strings(result.Warnings)
	result.Regressed = len(result.Regressions) > 0
	result.OK = true
	return result
}

func NewSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return SelfAugmentCompareResult{
		OK:                      false,
		StateDir:                core.StateDir(),
		BaselineKey:             baselineKey,
		CandidateKey:            candidateKey,
		MaxElapsedRegressionPct: maxElapsedRegressionPct,
		MissingStepLabels:       []string{},
		AddedStepLabels:         []string{},
		Regressions:             []string{},
		Warnings:                []string{},
		SlowStepRegressions:     []SelfAugmentSlowStepRegression{},
		StepBudgetRegressions:   []SelfAugmentStepBudgetRegression{},
	}
}
