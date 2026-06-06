package contractauditworker

import (
	"os"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult

type ValidationDeps struct {
	MkdirTemp         func(string, string) (string, error)
	RemoveAll         func(string) error
	ReadFile          func(string) ([]byte, error)
	RunCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	RunCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func (deps ValidationDeps) withDefaults() ValidationDeps {
	if deps.MkdirTemp == nil {
		deps.MkdirTemp = os.MkdirTemp
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.ReadFile == nil {
		deps.ReadFile = os.ReadFile
	}
	if deps.RunCommandStep == nil {
		deps.RunCommandStep = func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
			return commandstep.Run(dir, label, timeout, stdin, commandOutputBudgetBytes, name, args...)
		}
	}
	if deps.RunCommandStepEnv == nil {
		deps.RunCommandStepEnv = func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			return commandstep.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}
