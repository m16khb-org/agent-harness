package validationcli

import (
	"fmt"
	"os"
	"time"
)

type daemonResilienceValidationDeps struct {
	mkdirTemp func(string, string) (string, error)
	removeAll func(string) error
	writeFile func(string, []byte, os.FileMode) error
	chtimes   func(string, time.Time, time.Time) error
	stat      func(string) (os.FileInfo, error)
	exists    func(string) bool
	run       daemonResilienceCommandRunner
}

func (deps daemonResilienceValidationDeps) withDefaults() daemonResilienceValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.chtimes == nil {
		deps.chtimes = os.Chtimes
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = runDaemonResilienceCommand
	}
	return deps
}

func runDaemonResilienceCommand(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult {
	if len(command) == 0 {
		return failedStep(label, fmt.Errorf("missing command"))
	}
	return runCommandStepEnv(root, label, timeout, input, env, command[0], command[1:]...)
}
