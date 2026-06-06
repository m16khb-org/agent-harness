package main

import "time"

type selfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

func plannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult) []selfVerifyPlannedStep {
	return []selfVerifyPlannedStep{
		{Label: "harness invariants", Run: func() StepResult { return validateHarnessInvariants(root) }},
		{Label: "go test", Run: func() StepResult {
			*goTestStep = runCommandStep(root, "go test", 120*time.Second, "", "go", "test", "./...", "-count=1")
			return *goTestStep
		}},
		{Label: "contract golden tests", Run: func() StepResult {
			return cachedContractGoldenStep(*goTestStep)
		}},
		{Label: "risk QA tier", Run: func() StepResult { return validateRiskQATier(root) }},
		{Label: "go build", Run: func() StepResult {
			return runCommandStep(root, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/harness")
		}},
		{Label: "inspect smoke", Run: func() StepResult { return validateInspect(tempBin, root) }},
		{Label: "docs index smoke", Run: func() StepResult { return validateDocsIndex(tempBin, root) }},
		{Label: "candidate export", Run: func() StepResult { return validateSelfVerifyCandidateExport(tempBin, root, seed) }},
		{Label: "step budget baseline", Run: func() StepResult { return validateStepBudgetBaseline(tempBin, root, seed) }},
		{Label: "install dry-run smoke", Run: func() StepResult { return validateInstallDryRunSmoke(tempBin, root, seed) }},
		{Label: "command policy smoke", Run: func() StepResult { return validateCommandPolicy(tempBin, root) }},
		{Label: "command audit smoke", Run: func() StepResult { return validateCommandAudit(tempBin, root, seed) }},
		{Label: "contract check", Run: func() StepResult { return validateContractCheck(tempBin, root) }},
		{Label: "worker lifecycle smoke", Run: func() StepResult { return validateWorkerLifecycle(tempBin, root, seed) }},
		{Label: "MCP smoke", Run: func() StepResult { return validateMCP(tempBin, root) }},
		{Label: "state roundtrip", Run: func() StepResult { return validateStateRoundtrip(tempBin, root, seed) }},
		{Label: "parallel isolation", Run: func() StepResult { return validateParallelTempIsolation(tempBin, root, seed) }},
		{Label: "daemon resilience", Run: func() StepResult { return validateDaemonRestartResilience(tempBin, root, seed) }},
		{Label: "preflight fuzz", Run: func() StepResult { return validatePreflightFuzz(tempBin, root, seed) }},
		{Label: "native integration", Run: func() StepResult { return validateNativeIntegration(root) }},
		{Label: "redaction audit", Run: func() StepResult { return validateRedactionAudit(root) }},
		{Label: "QA gate", Run: func() StepResult { return validateQAGate(root) }},
	}
}

func cachedContractGoldenStep(goTestStep StepResult) StepResult {
	if goTestStep.OK {
		return StepResult{
			Label:      "contract golden tests",
			Command:    "covered by go test ./... -count=1",
			OK:         true,
			DurationMS: 0,
			Stdout:     "contract golden tests already executed by full go test suite",
		}
	}
	return runCommandStep(harnessRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/harness", "-run", "Golden", "-count=1")
}
