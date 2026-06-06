package preflightfuzz

import (
	"fmt"
	"os"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/internal/core"
)

type preflightFuzzCommandRunner func(root, label string, timeout time.Duration, input string, command ...string) StepResult
type preflightFuzzGitRunner func(dir string, args ...string) (int, string, string)

type preflightFuzzValidationDeps struct {
	mkdirTemp func(string, string) (string, error)
	removeAll func(string) error
	writeFile func(string, []byte, os.FileMode) error
	git       preflightFuzzGitRunner
	run       preflightFuzzCommandRunner
}

func (deps preflightFuzzValidationDeps) withDefaults() preflightFuzzValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.git == nil {
		deps.git = core.GitCmd
	}
	if deps.run == nil {
		deps.run = runPreflightFuzzCommand
	}
	return deps
}

func runPreflightFuzzCommand(root, label string, timeout time.Duration, input string, command ...string) StepResult {
	if len(command) == 0 {
		return commandstep.FailedStep(label, fmt.Errorf("missing command"))
	}
	return commandstep.Run(root, label, timeout, input, commandOutputBudgetBytes, command[0], command[1:]...)
}
