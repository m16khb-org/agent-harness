package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/summary"

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summary.SummarizeSelfAugment(result)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return summary.SummarizeSelfVerification(result, targetScore)
}

func selfVerificationContract() SelfVerificationContract {
	return summary.SelfVerificationContractValue()
}

func selfVerificationGoalDefinitions() []selfVerificationGoalDefinition {
	return summary.SelfVerificationGoalDefinitions()
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return summary.SelfVerificationCoverageDefinitions()
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return summary.SelfVerificationCoverageForLabels(stepLabels)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return summary.ScoreSelfVerificationGoals(result, targetScore)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summaryValue SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return summary.ClassifySelfVerificationFailure(result, summaryValue)
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return summary.SelfVerificationFailureClusters(result)
}

func stepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return summary.StepDurationStatByLabel(stats)
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return summary.MaxSlowStepDurationByLabel(steps)
}

func buildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return summary.BuildStepDurationStats(durationsByLabel)
}

func stepDurationStatsForCompare(summaryValue SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return summary.StepDurationStatsForCompare(summaryValue)
}
