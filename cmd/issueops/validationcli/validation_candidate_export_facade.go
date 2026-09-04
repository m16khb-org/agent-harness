package validationcli

import "issueops/cmd/issueops/validationcli/candidateexport"

type CandidateExportCommandRunner = candidateexport.CandidateExportCommandRunner
type CandidateExportValidationDeps = candidateexport.CandidateExportValidationDeps

func ValidateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return candidateexport.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func ValidateSelfVerifyCandidateExportWithDeps(binary, root string, seed int64, deps CandidateExportValidationDeps) StepResult {
	return candidateexport.ValidateSelfVerifyCandidateExportWithDeps(binary, root, seed, deps)
}

func CandidateExportValidationErrors(key string, exportResult SelfVerificationCandidateExportResult, snapshot SelfVerificationCandidateExportStateSnapshot) []string {
	return candidateexport.CandidateExportValidationErrors(key, exportResult, snapshot)
}
