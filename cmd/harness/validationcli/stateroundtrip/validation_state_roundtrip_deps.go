package stateroundtrip

import (
	"fmt"
	"os"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/core"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024
const selfVerificationSummaryKind = selfworkflow.SelfVerificationSummaryKind

type StepResult = commandstep.StepResult
type SelfAugmentCompareResult = selfworkflow.SelfAugmentCompareResult
type SelfAugmentHistoryEntry = selfworkflow.SelfAugmentHistoryEntry
type SelfAugmentHistoryResult = selfworkflow.SelfAugmentHistoryResult
type SelfAugmentHistoryRetention = selfworkflow.SelfAugmentHistoryRetention
type SelfAugmentPromoteResult = selfworkflow.SelfAugmentPromoteResult
type SelfAugmentSlowStep = selfworkflow.SelfAugmentSlowStep
type SelfAugmentStateSnapshot = selfworkflow.SelfAugmentStateSnapshot
type SelfAugmentSummary = selfworkflow.SelfAugmentSummary

type stateRoundtripCommandRunner func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult

type stateRoundtripValidationDeps struct {
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	writeFile     func(string, []byte, os.FileMode) error
	stateRead     func(string) (core.StateResult, error)
	writeSnapshot func(string, string, SelfAugmentStateSnapshot) error
	run           stateRoundtripCommandRunner
}

func (deps stateRoundtripValidationDeps) withDefaults() stateRoundtripValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.stateRead == nil {
		deps.stateRead = core.StateRead
	}
	if deps.writeSnapshot == nil {
		deps.writeSnapshot = selfworkflow.WriteSelfAugmentSnapshotRecord
	}
	if deps.run == nil {
		deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult {
			if len(command) == 0 {
				return failedStep(label, fmt.Errorf("missing command"))
			}
			return runCommandStepEnv(root, label, timeout, input, env, command[0], command[1:]...)
		}
	}
	return deps
}

func runCommandStepEnv(root, label string, timeout time.Duration, input string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(root, label, timeout, input, env, commandOutputBudgetBytes, name, args...)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return commandstep.AssertionStepWithOutput(label, started, errs, stdoutParts, commands, aggregateOutputBudgetBytes)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return commandstep.CombineFailedStep(label, started, child, stdoutParts, commands, aggregateOutputBudgetBytes)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
