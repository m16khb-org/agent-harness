package main

import "sort"

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
