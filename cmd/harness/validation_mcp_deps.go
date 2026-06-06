package main

import (
	"os"
	"time"
)

type mcpValidationDeps struct {
	mkdirTemp                   func(string, string) (string, error)
	removeAll                   func(string) error
	runCommandStepEnv           func(string, string, time.Duration, string, []string, string, ...string) StepResult
	runCommandStepEnvWithBudget func(string, string, time.Duration, string, []string, int, string, ...string) StepResult
}

func (deps mcpValidationDeps) withDefaults() mcpValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.runCommandStepEnv == nil {
		deps.runCommandStepEnv = runCommandStepEnv
	}
	if deps.runCommandStepEnvWithBudget == nil {
		deps.runCommandStepEnvWithBudget = runCommandStepEnvWithBudget
	}
	return deps
}
