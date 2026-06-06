package main

import (
	"fmt"
	"os"
	"time"

	"agent-harness/internal/core"
)

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
		deps.writeSnapshot = writeSelfAugmentSnapshotRecord
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
