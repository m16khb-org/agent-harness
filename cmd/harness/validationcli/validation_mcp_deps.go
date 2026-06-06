package validationcli

import (
	"os"
	"time"
)

type MCPValidationDeps struct {
	MkdirTemp                   func(string, string) (string, error)
	RemoveAll                   func(string) error
	RunCommandStepEnv           func(string, string, time.Duration, string, []string, string, ...string) StepResult
	RunCommandStepEnvWithBudget func(string, string, time.Duration, string, []string, int, string, ...string) StepResult
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
	if deps.RunCommandStepEnvWithBudget == nil {
		deps.RunCommandStepEnvWithBudget = runCommandStepEnvWithBudget
	}
	return deps
}
