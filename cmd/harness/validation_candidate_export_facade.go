package main

import (
	"time"

	"agent-harness/cmd/harness/validationcli"
)

type candidateExportCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult

type candidateExportValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	run           candidateExportCommandRunner
}

func validateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return validationcli.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func validateSelfVerifyCandidateExportWithDeps(binary, root string, seed int64, deps candidateExportValidationDeps) StepResult {
	return validationcli.ValidateSelfVerifyCandidateExportWithDeps(binary, root, seed, validationcli.CandidateExportValidationDeps{
		MakeTempState: deps.makeTempState,
		RemoveAll:     deps.removeAll,
		Run:           validationcli.CandidateExportCommandRunner(deps.run),
	})
}

func candidateExportValidationErrors(key string, exportResult SelfVerificationCandidateExportResult, snapshot SelfVerificationCandidateExportStateSnapshot) []string {
	return validationcli.CandidateExportValidationErrors(key, exportResult, snapshot)
}
