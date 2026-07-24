package harnessapp

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow"
)

const selfVerificationCandidateExportKind = selfworkflow.SelfVerificationCandidateExportKind

type SelfVerificationCandidateExportResult = selfworkflow.SelfVerificationCandidateExportResult
type SelfVerificationCandidate = selfworkflow.SelfVerificationCandidate
type SelfVerificationCandidateExportStateSnapshot = selfworkflow.SelfVerificationCandidateExportStateSnapshot
type selfVerifyPlannedStep = selfworkflow.SelfVerifyPlannedStep

type selfVerifyCandidatesDeps struct {
	export func() SelfVerificationCandidateExportResult
	save   func(result *SelfVerificationCandidateExportResult, key string) error
}

type selfVerifyPromoteDeps struct {
	promote func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error)
}

func runSelfVerify(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return runSelfVerifyHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runSelfVerifyCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return runSelfVerifyPromote(args[1:])
	}
	if len(args) > 0 && args[0] == "candidates" {
		return runSelfVerifyCandidates(args[1:])
	}
	return selfworkflow.RunSelfVerifyWithDeps(args, selfworkflow.SelfVerifyRunDeps{
		Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfworkflow.SelfVerifyProgressReporter, collectAll bool) (SelfAugmentResult, error) {
			if progress == nil {
				return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, collectAll)
			}
			return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, &selfVerifyProgressReporter{inner: progress}, collectAll)
		},
	})
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

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	return selfworkflow.CompareSelfAugmentSummaries(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	return selfworkflow.CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate)
}

func newSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return selfworkflow.NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func runSelfVerifyCompare(args []string) error {
	return selfworkflow.RunSelfVerifyCompare(args)
}

func runSelfVerifyHistory(args []string) error {
	return selfworkflow.RunSelfVerifyHistory(args)
}

func selfAugmentHistory(prefix string, limit int, retentionOptions ...selfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	options := []selfworkflow.SelfAugmentHistoryRetentionOptions{}
	for _, option := range retentionOptions {
		options = append(options, selfworkflow.SelfAugmentHistoryRetentionOptions(option))
	}
	return selfworkflow.SelfAugmentHistory(prefix, limit, options...)
}

func runSelfVerifyPromote(args []string) error {
	return selfworkflow.RunSelfVerifyPromote(args)
}

func runSelfVerifyPromoteWithDeps(args []string, deps selfVerifyPromoteDeps) error {
	return selfworkflow.RunSelfVerifyPromoteWithDeps(args, selfworkflow.SelfVerifyPromoteDeps{Promote: deps.promote})
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	return selfworkflow.PromoteSelfAugmentBaseline(fromKey, baselineKey, confirm, allowFailedSource)
}

func readSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return selfworkflow.ReadSelfAugmentStateSnapshot(key)
}

func isSelfVerificationSummaryKind(kind string) bool {
	return selfworkflow.IsSelfVerificationSummaryKind(kind)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return selfworkflow.WriteSelfAugmentSnapshotRecord(dir, key, snapshot)
}

func boolPtr(value bool) *bool {
	return &value
}

func selfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool) (SelfAugmentResult, error) {
	return selfworkflow.SelfVerify(iterations, baseSeed, targetScore, verbose, selfVerifyLoopDeps(false))
}

func selfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfVerifyProgressReporter, collectAll bool) (SelfAugmentResult, error) {
	if progress == nil {
		return selfworkflow.SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, selfVerifyLoopDeps(collectAll))
	}
	return selfworkflow.SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, progress.inner, selfVerifyLoopDeps(collectAll))
}

func selfVerifyLoopDeps(collectAll bool) selfworkflow.SelfVerifyLoopDeps {
	return selfworkflow.SelfVerifyLoopDeps{
		StepDeps:        selfVerifyStepDeps(),
		FailedStep:      failedStep,
		PrintStep:       printStep,
		CollectAllSteps: collectAll,
	}
}

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.NewSelfVerifyLoopResult(iterations, baseSeed, targetScore)
}

func emitSelfVerifyLoopStart(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopStart(progress.inner, loopKind, iterations, seed)
}

func emitSelfVerifyLoopEnd(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopEnd(progress.inner, loopKind, iterations, seed, ok, errorText)
}

func saveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfVerificationSummary(result, key)
}

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfAugmentSummary(result, key)
}

func newSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return selfworkflow.NewSelfVerificationSummarySnapshot(result, generatedAt)
}

func plannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult) []selfVerifyPlannedStep {
	return selfworkflow.PlannedSelfVerifySteps(root, tempBin, seed, goTestStep, selfVerifyStepDeps())
}

func cachedContractGoldenStep(goTestStep StepResult) StepResult {
	return selfworkflow.CachedContractGoldenStep(goTestStep, selfVerifyStepDeps())
}

func selfVerifyStepDeps() selfworkflow.SelfVerifyStepDeps {
	return selfworkflow.SelfVerifyStepDeps{
		HarnessRoot:                     harnessRoot,
		RunCommandStep:                  runCommandStepAdapter,
		ValidateHarnessInvariants:       validateHarnessInvariants,
		ValidateRiskQATier:              validateRiskQATier,
		ValidateResourceAdmission:       validateResourceAdmission,
		ValidateInspect:                 validateInspect,
		ValidateDocsIndex:               validateDocsIndex,
		ValidateSelfVerifyCandidate:     validateSelfVerifyCandidateExport,
		ValidateStepBudgetBaseline:      validateStepBudgetBaseline,
		ValidateInstallDryRunSmoke:      validateInstallDryRunSmoke,
		ValidateCommandPolicy:           validateCommandPolicy,
		ValidateCommandAudit:            validateCommandAudit,
		ValidateContractCheck:           validateContractCheck,
		ValidateToolConformance:         validateToolConformance,
		ValidateWorkerLifecycle:         validateWorkerLifecycle,
		ValidateMCP:                     validateMCP,
		ValidateStateRoundtrip:          validateStateRoundtrip,
		ValidateParallelTempIsolation:   validateParallelTempIsolation,
		ValidateDaemonRestartResilience: validateDaemonRestartResilience,
		ValidatePreflightFuzz:           validatePreflightFuzz,
		ValidateWebFetchBattery:         validateWebFetchBattery,
		ValidateNativeIntegration:       validateNativeIntegration,
		ValidateRedactionAudit:          validateRedactionAudit,
		ValidateQAGate:                  validateQAGate,
	}
}

func runCommandStepAdapter(dir string, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStep(dir, label, timeout, stdin, name, args...)
}
