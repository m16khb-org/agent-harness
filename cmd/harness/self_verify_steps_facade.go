package main

import (
	"time"

	"agent-harness/cmd/harness/selfworkflow"
)

type selfVerifyPlannedStep = selfworkflow.SelfVerifyPlannedStep

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
		ValidateInspect:                 validateInspect,
		ValidateDocsIndex:               validateDocsIndex,
		ValidateSelfVerifyCandidate:     validateSelfVerifyCandidateExport,
		ValidateStepBudgetBaseline:      validateStepBudgetBaseline,
		ValidateInstallDryRunSmoke:      validateInstallDryRunSmoke,
		ValidateCommandPolicy:           validateCommandPolicy,
		ValidateCommandAudit:            validateCommandAudit,
		ValidateContractCheck:           validateContractCheck,
		ValidateWorkerLifecycle:         validateWorkerLifecycle,
		ValidateMCP:                     validateMCP,
		ValidateStateRoundtrip:          validateStateRoundtrip,
		ValidateParallelTempIsolation:   validateParallelTempIsolation,
		ValidateDaemonRestartResilience: validateDaemonRestartResilience,
		ValidatePreflightFuzz:           validatePreflightFuzz,
		ValidateNativeIntegration:       validateNativeIntegration,
		ValidateRedactionAudit:          validateRedactionAudit,
		ValidateQAGate:                  validateQAGate,
	}
}

func runCommandStepAdapter(dir string, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStep(dir, label, timeout, stdin, name, args...)
}
