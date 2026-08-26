package candidateexport

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/selfworkflow"
	statecontract "agent-harness/internal/contract/state"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024
const selfAugmentCandidateStatusSatisfied = selfworkflow.SelfAugmentCandidateStatusSatisfied
const selfVerificationCandidateExportKind = selfworkflow.SelfVerificationCandidateExportKind

type StepResult = commandstep.StepResult
type SelfAugmentStateCheckpoint = selfworkflow.SelfAugmentStateCheckpoint
type SelfVerificationCandidate = selfworkflow.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = selfworkflow.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = selfworkflow.SelfVerificationCandidateExportStateSnapshot

type CandidateExportCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult

type CandidateExportValidationDeps struct {
	MakeTempState func(seed int64) (string, error)
	RemoveAll     func(path string) error
	Run           CandidateExportCommandRunner
}

func (deps CandidateExportValidationDeps) withDefaults() CandidateExportValidationDeps {
	if deps.MakeTempState == nil {
		deps.MakeTempState = func(seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("agent-harness-candidates-%d-*", seed))
		}
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.Run == nil {
		deps.Run = func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			return commandstep.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}

func ValidateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return ValidateSelfVerifyCandidateExportWithDeps(binary, root, seed, CandidateExportValidationDeps{})
}

func ValidateSelfVerifyCandidateExportWithDeps(binary, root string, seed int64, deps CandidateExportValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempState, err := deps.MakeTempState(seed)
	if err != nil {
		return commandstep.FailedStep("candidate export", err)
	}
	defer func() { _ = deps.RemoveAll(tempState) }()
	key := fmt.Sprintf("self-verify-candidates-%d", seed)
	env := []string{"HARNESS_STATE_DIR=" + tempState}
	stdoutParts := []string{}
	commands := []string{}

	exportStep := deps.Run(root, "candidate export", 30*time.Second, "", env, binary, "self-verify", "candidates", "--save-state", "--state-key", key, "--json")
	stdoutParts = append(stdoutParts, exportStep.Stdout)
	commands = append(commands, exportStep.Command)
	if !exportStep.OK {
		return commandstep.CombineFailedStep("candidate export", started, exportStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	var exportResult SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(exportStep.Stdout), &exportResult); err != nil {
		return commandstep.AssertionStepWithOutput("candidate export", started, []string{err.Error()}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}

	readStep := deps.Run(root, "candidate export state read", 30*time.Second, "", env, binary, "state", "read", "--key", key, "--json")
	stdoutParts = append(stdoutParts, readStep.Stdout)
	commands = append(commands, readStep.Command)
	if !readStep.OK {
		return commandstep.CombineFailedStep("candidate export", started, readStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	var readResult statecontract.StateResult
	if err := json.Unmarshal([]byte(readStep.Stdout), &readResult); err != nil {
		return commandstep.AssertionStepWithOutput("candidate export", started, []string{err.Error()}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	var snapshot SelfVerificationCandidateExportStateSnapshot
	if err := json.Unmarshal([]byte(readResult.Record.Content), &snapshot); err != nil {
		return commandstep.AssertionStepWithOutput("candidate export", started, []string{"candidate export state snapshot parse: " + err.Error()}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}

	errs := CandidateExportValidationErrors(key, exportResult, snapshot)
	if len(errs) > 0 {
		return commandstep.AssertionStepWithOutput("candidate export", started, errs, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := commandstep.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func CandidateExportValidationErrors(key string, exportResult SelfVerificationCandidateExportResult, snapshot SelfVerificationCandidateExportStateSnapshot) []string {
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
