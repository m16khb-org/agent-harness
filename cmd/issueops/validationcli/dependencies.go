package validationcli

import (
	"os"
	"strings"
	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/cmd/issueops/selfworkflow"
)

const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

type StepResult = commandstep.StepResult
type SelfAugmentCompareResult = selfworkflow.SelfAugmentCompareResult
type SelfAugmentSlowStep = selfworkflow.SelfAugmentSlowStep
type SelfAugmentSlowStepRegression = selfworkflow.SelfAugmentSlowStepRegression
type SelfAugmentStateSnapshot = selfworkflow.SelfAugmentStateSnapshot
type SelfAugmentStateCheckpoint = selfworkflow.SelfAugmentStateCheckpoint
type SelfAugmentStepDurationStat = selfworkflow.SelfAugmentStepDurationStat
type SelfAugmentStepBudgetRegression = selfworkflow.SelfAugmentStepBudgetRegression
type SelfAugmentSummary = selfworkflow.SelfAugmentSummary
type SelfAugmentPromoteResult = selfworkflow.SelfAugmentPromoteResult
type SelfAugmentHistoryEntry = selfworkflow.SelfAugmentHistoryEntry
type SelfAugmentHistoryResult = selfworkflow.SelfAugmentHistoryResult
type SelfAugmentHistoryRetention = selfworkflow.SelfAugmentHistoryRetention
type SelfVerificationCandidateExportResult = selfworkflow.SelfVerificationCandidateExportResult
type SelfVerificationCandidate = selfworkflow.SelfVerificationCandidate
type SelfVerificationCandidateExportStateSnapshot = selfworkflow.SelfVerificationCandidateExportStateSnapshot

const selfVerificationCandidateExportKind = selfworkflow.SelfVerificationCandidateExportKind
const selfVerificationKoreanName = selfworkflow.SelfVerificationKoreanName
const selfVerificationSummaryKind = selfworkflow.SelfVerificationSummaryKind
const selfAugmentCandidateStatusSatisfied = selfworkflow.SelfAugmentCandidateStatusSatisfied

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return commandstep.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
	return commandstep.RunEnvWithBudget(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return commandstep.CombineFailedStep(label, started, child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return commandstep.AssertionStepWithOutput(label, started, errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) StepResult {
	return commandstep.AssertionStep(label, started, errs)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return selfworkflow.WriteSelfAugmentSnapshotRecord(dir, key, snapshot)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
