package main

import (
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/validationcli"
)

type StepResult = commandstep.StepResult

type candidateExportCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult

type candidateExportValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	run           candidateExportCommandRunner
}

type contractAuditWorkerValidationDeps struct {
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	readFile          func(string) ([]byte, error)
	runCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	runCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

type mcpValidationDeps struct {
	mkdirTemp                   func(string, string) (string, error)
	removeAll                   func(string) error
	runCommandStepEnv           func(string, string, time.Duration, string, []string, string, ...string) StepResult
	runCommandStepEnvWithBudget func(string, string, time.Duration, string, []string, int, string, ...string) StepResult
}

type stepBudgetCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult
type stepBudgetSnapshotWriter func(dir, key string, snapshot SelfAugmentStateSnapshot) error

type stepBudgetValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	writeSnapshot stepBudgetSnapshotWriter
	run           stepBudgetCommandRunner
}

type ClaudeMCPDuplicateWarning = validationcli.ClaudeMCPDuplicateWarning

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return commandstep.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	return commandstep.RunEnvWithBudget(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func mergeEnvOverrides(base []string, overrides []string) []string {
	return commandstep.MergeEnvOverrides(base, overrides)
}

func envEntryKey(entry string) (string, bool) {
	return commandstep.EnvEntryKey(entry)
}

func budgetCommandOutput(s string, budget int) (string, bool, int) {
	return commandstep.BudgetCommandOutput(s, budget)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return commandstep.CombineFailedStep(label, started, child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	return commandstep.AssertionStep(label, started, errs)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return commandstep.AssertionStepWithOutput(label, started, errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func printStep(step StepResult) {
	commandstep.PrintStep(step)
}

func tail(s string, max int) string {
	return commandstep.Tail(s, max)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func indentLines(s string) string {
	return commandstep.IndentLines(s)
}

func validateInspect(binary, root string) StepResult {
	return validationcli.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return validationcli.ValidateDocsIndex(binary, root)
}

func validateCommandPolicy(binary, root string) StepResult {
	return validationcli.ValidateCommandPolicy(binary, root)
}

func validateMCP(binary, root string) StepResult {
	return validationcli.ValidateMCP(binary, root)
}

func validateMCPWithDeps(binary, root string, deps mcpValidationDeps) StepResult {
	return validationcli.ValidateMCPWithDeps(binary, root, validationcli.MCPValidationDeps{
		MkdirTemp:                   deps.mkdirTemp,
		RemoveAll:                   deps.removeAll,
		RunCommandStepEnv:           deps.runCommandStepEnv,
		RunCommandStepEnvWithBudget: deps.runCommandStepEnvWithBudget,
	})
}

func mcpSmokeInput() string {
	return validationcli.MCPSmokeInput()
}

func validateMCPSmokeContract(step *StepResult) {
	validationcli.ValidateMCPSmokeContract(step)
}

func mcpSmokeHasExpectedMarkers(stdout string) bool {
	return validationcli.MCPSmokeHasExpectedMarkers(stdout)
}

func mcpSmokeExpectedMarkers() []string {
	return validationcli.MCPSmokeExpectedMarkers()
}

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStateRoundtrip(binary, root, seed)
}

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return validationcli.ValidateInstallDryRunSmoke(binary, root, seed)
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return validationcli.ValidateParallelTempIsolation(binary, root, seed)
}

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return validationcli.ValidateDaemonRestartResilience(binary, root, seed)
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return validationcli.ValidatePreflightFuzz(binary, root, seed)
}

func validateCommandAudit(binary, root string, seed int64) StepResult {
	return validationcli.ValidateCommandAudit(binary, root, seed)
}

func validateCommandAuditWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateCommandAuditWithDeps(binary, root, seed, contractAuditWorkerDepsForValidationCLI(deps))
}

func validateContractCheck(binary, root string) StepResult {
	return validationcli.ValidateContractCheck(binary, root)
}

func validateContractCheckWithDeps(binary, root string, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateContractCheckWithDeps(binary, root, contractAuditWorkerDepsForValidationCLI(deps))
}

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return validationcli.ValidateWorkerLifecycle(binary, root, seed)
}

func validateWorkerLifecycleWithDeps(binary, root string, seed int64, deps contractAuditWorkerValidationDeps) StepResult {
	return validationcli.ValidateWorkerLifecycleWithDeps(binary, root, seed, contractAuditWorkerDepsForValidationCLI(deps))
}

func contractAuditWorkerDepsForValidationCLI(deps contractAuditWorkerValidationDeps) validationcli.ContractAuditWorkerValidationDeps {
	return validationcli.ContractAuditWorkerValidationDeps{
		MkdirTemp:         deps.mkdirTemp,
		RemoveAll:         deps.removeAll,
		ReadFile:          deps.readFile,
		RunCommandStep:    deps.runCommandStep,
		RunCommandStepEnv: deps.runCommandStepEnv,
	}
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

func validateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStepBudgetBaseline(binary, root, seed)
}

func validateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps stepBudgetValidationDeps) StepResult {
	return validationcli.ValidateStepBudgetBaselineWithDeps(binary, root, seed, validationcli.StepBudgetValidationDeps{
		MakeTempState: deps.makeTempState,
		RemoveAll:     deps.removeAll,
		WriteSnapshot: validationcli.StepBudgetSnapshotWriter(deps.writeSnapshot),
		Run:           validationcli.StepBudgetCommandRunner(deps.run),
	})
}

func stepBudgetBaselineSummaries(seed int64) (SelfAugmentSummary, SelfAugmentSummary) {
	return validationcli.StepBudgetBaselineSummaries(seed)
}

func stepBudgetStateSnapshot(root string, seed int64, summary SelfAugmentSummary) SelfAugmentStateSnapshot {
	return validationcli.StepBudgetStateSnapshot(root, seed, summary)
}

func stepBudgetValidationErrors(result SelfAugmentCompareResult) []string {
	return validationcli.StepBudgetValidationErrors(result)
}

func validateRedactionAudit(root string) StepResult {
	return validationcli.ValidateRedactionAudit(root)
}

func validateQAGate(root string) StepResult {
	return validationcli.ValidateQAGate(root)
}

func validateMermaidDocs(root string) []string {
	return validationcli.ValidateMermaidDocs(root)
}

func lintMermaidBlocks(relPath, text string) []string {
	return validationcli.LintMermaidBlocks(relPath, text)
}

func findUnredactedSecretLike(text string) []string {
	return validationcli.FindUnredactedSecretLike(text)
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return validationcli.ContainsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func forbiddenNameHits(root string) []string {
	return validationcli.ForbiddenNameHits(root)
}

func validateHarnessInvariants(root string) StepResult {
	return validationcli.ValidateHarnessInvariants(root)
}

func validateNativeIntegration(root string) StepResult {
	return validationcli.ValidateNativeIntegration(root)
}

func detectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	return validationcli.DetectClaudeMCPDuplicateWarnings(output)
}

func claudeMCPDuplicateWarningFixture() string {
	return validationcli.ClaudeMCPDuplicateWarningFixture()
}
