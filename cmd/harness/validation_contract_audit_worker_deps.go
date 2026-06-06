package main

import (
	"os"
	"time"
)

type contractAuditWorkerValidationDeps struct {
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	readFile          func(string) ([]byte, error)
	runCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	runCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func (deps contractAuditWorkerValidationDeps) withDefaults() contractAuditWorkerValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.runCommandStep == nil {
		deps.runCommandStep = runCommandStep
	}
	if deps.runCommandStepEnv == nil {
		deps.runCommandStepEnv = runCommandStepEnv
	}
	return deps
}
