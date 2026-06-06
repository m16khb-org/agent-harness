package main

import (
	"fmt"
	"sort"
	"strings"

	"agent-harness/internal/core"
)

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	result := newSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	if strings.TrimSpace(candidateKey) == "" {
		return result, fmt.Errorf("candidate-key is required")
	}
	if maxElapsedRegressionPct < 0 {
		return result, fmt.Errorf("max elapsed regression pct must be non-negative")
	}
	baseline, err := readSelfAugmentStateSnapshot(baselineKey)
	if err != nil {
		return result, fmt.Errorf("read baseline summary: %w", err)
	}
	candidate, err := readSelfAugmentStateSnapshot(candidateKey)
	if err != nil {
		return result, fmt.Errorf("read candidate summary: %w", err)
	}
	return compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate), nil
}

func compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	result := newSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
	result.BaselineSummary = baseline.Summary
	result.CandidateSummary = candidate.Summary
	result.BaselineSnapshotGeneratedAt = baseline.GeneratedAt
	result.CandidateSnapshotGeneratedAt = candidate.GeneratedAt
	result.BaselineSlowestSteps = baseline.Summary.SlowestSteps
	result.CandidateSlowestSteps = candidate.Summary.SlowestSteps
	result.BaselineStepDurationStats = stepDurationStatsForCompare(baseline.Summary)
	result.CandidateStepDurationStats = stepDurationStatsForCompare(candidate.Summary)
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
	result.MissingStepLabels = missingStrings(baseline.Summary.StepLabels, candidate.Summary.StepLabels)
	result.AddedStepLabels = missingStrings(candidate.Summary.StepLabels, baseline.Summary.StepLabels)
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
	result.SlowStepRegressions = compareSlowestStepRegressions(baseline.Summary.SlowestSteps, candidate.Summary.SlowestSteps, maxElapsedRegressionPct)
	for _, regression := range result.SlowStepRegressions {
		result.Regressions = append(result.Regressions, fmt.Sprintf("slow_step:%s_increased_by_%.2f_pct", regression.Label, regression.DeltaPct))
	}
	result.StepBudgetRegressions = compareStepBudgetRegressions(result.BaselineStepDurationStats, result.CandidateStepDurationStats, maxElapsedRegressionPct)
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

func newSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
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

func compareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	baselineByLabel := maxSlowStepDurationByLabel(baseline)
	candidateByLabel := maxSlowStepDurationByLabel(candidate)
	regressions := []SelfAugmentSlowStepRegression{}
	for label, candidateDuration := range candidateByLabel {
		baselineDuration, ok := baselineByLabel[label]
		if !ok || baselineDuration <= 0 {
			continue
		}
		delta := candidateDuration - baselineDuration
		if delta <= 0 {
			continue
		}
		deltaPct := float64(delta) * 100 / float64(baselineDuration)
		if deltaPct <= maxRegressionPct {
			continue
		}
		regressions = append(regressions, SelfAugmentSlowStepRegression{
			Label:               label,
			BaselineDurationMS:  baselineDuration,
			CandidateDurationMS: candidateDuration,
			DeltaMS:             delta,
			DeltaPct:            deltaPct,
		})
	}
	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].DeltaPct != regressions[j].DeltaPct {
			return regressions[i].DeltaPct > regressions[j].DeltaPct
		}
		return regressions[i].Label < regressions[j].Label
	})
	return regressions
}

func compareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	baselineByLabel := stepDurationStatByLabel(baseline)
	candidateByLabel := stepDurationStatByLabel(candidate)
	regressions := []SelfAugmentStepBudgetRegression{}
	for label, candidateStat := range candidateByLabel {
		baselineStat, ok := baselineByLabel[label]
		if !ok || baselineStat.P95DurationMS <= 0 {
			continue
		}
		delta := candidateStat.P95DurationMS - baselineStat.P95DurationMS
		if delta <= 0 {
			continue
		}
		if delta < selfVerifyStepBudgetMinRegressionMS {
			continue
		}
		deltaPct := float64(delta) * 100 / float64(baselineStat.P95DurationMS)
		if deltaPct <= maxRegressionPct {
			continue
		}
		regressions = append(regressions, SelfAugmentStepBudgetRegression{
			Label:               label,
			Metric:              "p95_duration_ms",
			BaselineDurationMS:  baselineStat.P95DurationMS,
			CandidateDurationMS: candidateStat.P95DurationMS,
			DeltaMS:             delta,
			DeltaPct:            deltaPct,
			BaselineCount:       baselineStat.Count,
			CandidateCount:      candidateStat.Count,
		})
	}
	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].DeltaPct != regressions[j].DeltaPct {
			return regressions[i].DeltaPct > regressions[j].DeltaPct
		}
		return regressions[i].Label < regressions[j].Label
	})
	return regressions
}

func missingStrings(want, have []string) []string {
	haveSet := map[string]bool{}
	for _, item := range have {
		haveSet[item] = true
	}
	missing := []string{}
	for _, item := range want {
		if !haveSet[item] {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}
