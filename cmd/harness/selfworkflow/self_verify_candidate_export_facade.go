package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/candidateexport"

const SelfVerificationCandidateExportKind = candidateexport.SelfVerificationCandidateExportKind

type SelfVerificationCandidate = candidateexport.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = candidateexport.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = candidateexport.SelfVerificationCandidateExportStateSnapshot

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
