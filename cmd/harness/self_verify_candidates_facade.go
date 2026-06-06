package main

import "agent-harness/cmd/harness/selfworkflow"

const selfVerificationCandidateExportKind = selfworkflow.SelfVerificationCandidateExportKind

type SelfVerificationCandidateExportResult = selfworkflow.SelfVerificationCandidateExportResult
type SelfVerificationCandidate = selfworkflow.SelfVerificationCandidate
type SelfVerificationCandidateExportStateSnapshot = selfworkflow.SelfVerificationCandidateExportStateSnapshot

type selfVerifyCandidatesDeps struct {
	export func() SelfVerificationCandidateExportResult
	save   func(result *SelfVerificationCandidateExportResult, key string) error
}

func exportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.ExportSelfVerificationCandidates()
}

func selfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return selfworkflow.SelfVerificationCandidateCatalog()
}

func selfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return selfworkflow.SelfVerificationCandidateIDsByStatus(candidates, status)
}

func selectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return selfworkflow.SelectedSelfVerificationCandidateID(candidate)
}

func runSelfVerifyCandidates(args []string) error {
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.RunSelfVerifyCandidates(args)
}

func runSelfVerifyCandidatesWithDeps(args []string, deps selfVerifyCandidatesDeps) error {
	return selfworkflow.RunSelfVerifyCandidatesWithDeps(args, selfworkflow.SelfVerifyCandidatesDeps{
		Export: deps.export,
		Save:   deps.save,
	})
}

func saveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return selfworkflow.SaveSelfVerificationCandidateExport(result, key)
}
