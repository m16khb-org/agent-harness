package selfworkflow

import (
	"sort"
)

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	summary := SelfAugmentSummary{
		TotalRuns:         len(result.Runs),
		TargetScore:       targetScore,
		Contract:          selfVerificationContract(),
		StepLabels:        []string{},
		SlowestSteps:      []SelfAugmentSlowStep{},
		StepDurationStats: []SelfAugmentStepDurationStat{},
	}
	seenLabels := map[string]bool{}
	durationsByLabel := map[string][]int64{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			summary.TotalSteps++
			if step.OK {
				summary.PassedSteps++
			} else {
				summary.FailedSteps++
				if summary.FailedStep == "" {
					summary.FailedIteration = run.Iteration
					summary.FailedSeed = run.Seed
					summary.FailedStep = step.Label
				}
			}
			if !seenLabels[step.Label] {
				seenLabels[step.Label] = true
				summary.StepLabels = append(summary.StepLabels, step.Label)
			}
			summary.SlowestSteps = append(summary.SlowestSteps, SelfAugmentSlowStep{
				Iteration:  run.Iteration,
				Seed:       run.Seed,
				Label:      step.Label,
				DurationMS: step.DurationMS,
			})
			durationsByLabel[step.Label] = append(durationsByLabel[step.Label], step.DurationMS)
		}
	}
	sort.Slice(summary.SlowestSteps, func(i, j int) bool {
		if summary.SlowestSteps[i].DurationMS != summary.SlowestSteps[j].DurationMS {
			return summary.SlowestSteps[i].DurationMS > summary.SlowestSteps[j].DurationMS
		}
		if summary.SlowestSteps[i].Iteration != summary.SlowestSteps[j].Iteration {
			return summary.SlowestSteps[i].Iteration < summary.SlowestSteps[j].Iteration
		}
		return summary.SlowestSteps[i].Label < summary.SlowestSteps[j].Label
	})
	if len(summary.SlowestSteps) > 5 {
		summary.SlowestSteps = summary.SlowestSteps[:5]
	}
	if summary.StepLabels == nil {
		summary.StepLabels = []string{}
	}
	if summary.SlowestSteps == nil {
		summary.SlowestSteps = []SelfAugmentSlowStep{}
	}
	summary.StepDurationStats = buildStepDurationStats(durationsByLabel)
	if summary.StepDurationStats == nil {
		summary.StepDurationStats = []SelfAugmentStepDurationStat{}
	}
	summary.GoalScores = scoreSelfVerificationGoals(result, targetScore)
	summary.Coverage, summary.CoverageGaps = selfVerificationCoverage(summary.StepLabels)
	if summary.FailedStep != "" {
		summary.RerunCommands = selfVerifyRerunCommands(summary.FailedStep, result.Iterations, result.BaseSeed, targetScore)
		summary.FailureClass, summary.FailureClassReason, summary.FailureClusters = classifySelfVerificationFailure(result, summary)
	}
	summary.MinimumGoalScore = 100
	if len(summary.GoalScores) == 0 {
		summary.MinimumGoalScore = 0
	}
	summary.TerminationEligible = result.OK
	for _, goal := range summary.GoalScores {
		if goal.Score < summary.MinimumGoalScore {
			summary.MinimumGoalScore = goal.Score
		}
		if !goal.Passed {
			summary.TerminationEligible = false
		}
	}
	return summary
}
