package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/candidateexport"

func ExportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	return candidateexport.ExportSelfVerificationCandidates(HarnessRoot())
}

func SaveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return candidateexport.SaveSelfVerificationCandidateExport(result, key)
}

func SelectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return candidateexport.SelectedSelfVerificationCandidateID(candidate)
}

func SelfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return candidateexport.SelfVerificationCandidateCatalog()
}

func SelfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return candidateexport.SelfVerificationCandidateIDsByStatus(candidates, status)
}

func BuildSelfVerificationContract() SelfVerificationContract {
	return selfVerificationContract()
}

func BuildSelfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return selfVerificationCoverage(stepLabels)
}

func SelfVerificationCoverageDefinitions() []SelfVerificationCoverageDefinition {
	definitions := selfVerificationCoverageDefinitions()
	out := make([]SelfVerificationCoverageDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationCoverageDefinition(definition))
	}
	return out
}

func SelfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return selfVerificationFailureClusters(result)
}

func SelfVerificationGoalDefinitions() []SelfVerificationGoalDefinition {
	definitions := selfVerificationGoalDefinitions()
	out := make([]SelfVerificationGoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationGoalDefinition(definition))
	}
	return out
}

func SelfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	return selfVerifyRerunCommands(failedStep, iterations, baseSeed, targetScore)
}

func SelfVerifyStepRerunCommand(label string) (string, bool) {
	return selfVerifyStepRerunCommand(label)
}
