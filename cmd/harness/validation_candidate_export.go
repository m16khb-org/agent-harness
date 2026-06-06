package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type candidateExportCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult

type candidateExportValidationDeps struct {
	makeTempState func(seed int64) (string, error)
	removeAll     func(path string) error
	run           candidateExportCommandRunner
}

func (deps candidateExportValidationDeps) withDefaults() candidateExportValidationDeps {
	if deps.makeTempState == nil {
		deps.makeTempState = func(seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("agent-harness-candidates-%d-*", seed))
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.run == nil {
		deps.run = runCommandStepEnv
	}
	return deps
}

func validateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return validateSelfVerifyCandidateExportWithDeps(binary, root, seed, candidateExportValidationDeps{})
}

func validateSelfVerifyCandidateExportWithDeps(binary, root string, seed int64, deps candidateExportValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempState, err := deps.makeTempState(seed)
	if err != nil {
		return failedStep("candidate export", err)
	}
	defer deps.removeAll(tempState)
	key := fmt.Sprintf("self-verify-candidates-%d", seed)
	env := []string{"HARNESS_STATE_DIR=" + tempState}
	stdoutParts := []string{}
	commands := []string{}

	exportStep := deps.run(root, "candidate export", 30*time.Second, "", env, binary, "self-verify", "candidates", "--save-state", "--state-key", key, "--json")
	stdoutParts = append(stdoutParts, exportStep.Stdout)
	commands = append(commands, exportStep.Command)
	if !exportStep.OK {
		return combineFailedStep("candidate export", started, exportStep, stdoutParts, commands)
	}
	var exportResult SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(exportStep.Stdout), &exportResult); err != nil {
		return assertionStepWithOutput("candidate export", started, []string{err.Error()}, stdoutParts, commands)
	}

	readStep := deps.run(root, "candidate export state read", 30*time.Second, "", env, binary, "state", "read", "--key", key, "--json")
	stdoutParts = append(stdoutParts, readStep.Stdout)
	commands = append(commands, readStep.Command)
	if !readStep.OK {
		return combineFailedStep("candidate export", started, readStep, stdoutParts, commands)
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(readStep.Stdout), &readResult); err != nil {
		return assertionStepWithOutput("candidate export", started, []string{err.Error()}, stdoutParts, commands)
	}
	var snapshot SelfVerificationCandidateExportStateSnapshot
	if err := json.Unmarshal([]byte(readResult.Record.Content), &snapshot); err != nil {
		return assertionStepWithOutput("candidate export", started, []string{"candidate export state snapshot parse: " + err.Error()}, stdoutParts, commands)
	}

	errs := candidateExportValidationErrors(key, exportResult, snapshot)
	if len(errs) > 0 {
		return assertionStepWithOutput("candidate export", started, errs, stdoutParts, commands)
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "candidate export",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func candidateExportValidationErrors(key string, exportResult SelfVerificationCandidateExportResult, snapshot SelfVerificationCandidateExportStateSnapshot) []string {
	errs := []string{}
	if !exportResult.OK || exportResult.Kind != selfVerificationCandidateExportKind || exportResult.LoopKind != "self_verification" {
		errs = append(errs, "candidate export identity mismatch")
	}
	if exportResult.CandidateCount < 10 || len(exportResult.Candidates) != exportResult.CandidateCount {
		errs = append(errs, "candidate export did not include the candidate curriculum")
	}
	if exportResult.SelectedCandidate != nil || len(exportResult.OpenCandidateIDs) != 0 || !containsString(exportResult.SatisfiedCandidateIDs, "completion-evidence-audit") {
		errs = append(errs, "candidate export did not mark completion evidence candidate satisfied")
	}
	if containsString(exportResult.OpenCandidateIDs, "self-verify-candidate-export") || !containsString(exportResult.SatisfiedCandidateIDs, "self-verify-candidate-export") || containsString(exportResult.OpenCandidateIDs, "self-verify-step-budget-baseline") || !containsString(exportResult.SatisfiedCandidateIDs, "self-verify-step-budget-baseline") || containsString(exportResult.OpenCandidateIDs, "self-verify-install-dry-run-smoke") || !containsString(exportResult.SatisfiedCandidateIDs, "self-verify-install-dry-run-smoke") {
		errs = append(errs, "candidate export did not mark implemented candidates satisfied")
	}
	if exportResult.StateCheckpoint == nil || !exportResult.StateCheckpoint.OK || exportResult.StateCheckpoint.Key != key {
		errs = append(errs, "candidate export did not save the requested state checkpoint")
	}
	if snapshot.Kind != selfVerificationCandidateExportKind || snapshot.CandidateCount != exportResult.CandidateCount {
		errs = append(errs, "candidate export state snapshot mismatch")
	}
	if snapshot.SelectedCandidate != nil || len(snapshot.OpenCandidateIDs) != 0 || !containsString(snapshot.SatisfiedCandidateIDs, "completion-evidence-audit") {
		errs = append(errs, "candidate export state satisfied candidate mismatch")
	}
	return errs
}
