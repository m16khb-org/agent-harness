package summary

import (
	"sort"

	"agent-harness/cmd/harness/selfworkflow/rerun"
	failurecausecontract "agent-harness/internal/contract/failurecause"
)

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return SummarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}

func SummarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	summary := SelfAugmentSummary{
		TotalRuns:            len(result.Runs),
		TargetScore:          targetScore,
		Contract:             SelfVerificationContractValue(),
		StepLabels:           []string{},
		SlowestSteps:         []SelfAugmentSlowStep{},
		StepDurationStats:    []SelfAugmentStepDurationStat{},
		FailureCauseEvidence: []failurecausecontract.Evidence{},
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
	summary.StepDurationStats = BuildStepDurationStats(durationsByLabel)
	if summary.StepDurationStats == nil {
		summary.StepDurationStats = []SelfAugmentStepDurationStat{}
	}
	summary.GoalScores = ScoreSelfVerificationGoals(result, targetScore)
	summary.Coverage, summary.CoverageGaps = SelfVerificationCoverageForLabels(summary.StepLabels)
	if summary.FailedStep != "" {
		summary.RerunCommands = rerun.SelfVerifyRerunCommands(summary.FailedStep, result.BaseSeed, targetScore)
		summary.FailureClass, summary.FailureClassReason, summary.FailureClusters = ClassifySelfVerificationFailure(result, summary)
	}
	evidence := []failurecausecontract.Evidence{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			if !step.OK {
				evidence = append(evidence, step.FailureEvidence...)
			}
		}
	}
	cause := Classify(summary.FailedSteps > 0, evidence)
	summary.FailureCause, summary.FailureCauseReason, summary.FailureCauseEvidence = cause.Cause, cause.Reason, cause.Evidence
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
