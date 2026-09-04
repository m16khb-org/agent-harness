package mcpsmoke

import (
	"os"
	"strings"
	"time"

	"issueops/cmd/issueops/commandstep"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult

type MCPValidationDeps struct {
	MkdirTemp         func(string, string) (string, error)
	RemoveAll         func(string) error
	RunCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
	RunSDKSmoke       func(string, string, []string, time.Duration) StepResult
}

func (deps MCPValidationDeps) withDefaults() MCPValidationDeps {
	if deps.MkdirTemp == nil {
		deps.MkdirTemp = os.MkdirTemp
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.RunCommandStepEnv == nil {
		deps.RunCommandStepEnv = runCommandStepEnv
	}
	if deps.RunSDKSmoke == nil {
		deps.RunSDKSmoke = runSDKSmoke
	}
	return deps
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
