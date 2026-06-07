package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/rerun"

func selfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	return rerun.SelfVerifyRerunCommands(failedStep, iterations, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return rerun.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return rerun.FormatScore(score)
}
