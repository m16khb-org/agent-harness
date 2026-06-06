package validationcli

import (
	"os"
	"time"
)

type ContractAuditWorkerValidationDeps struct {
	MkdirTemp         func(string, string) (string, error)
	RemoveAll         func(string) error
	ReadFile          func(string) ([]byte, error)
	RunCommandStep    func(string, string, time.Duration, string, string, ...string) StepResult
	RunCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func (deps ContractAuditWorkerValidationDeps) withDefaults() ContractAuditWorkerValidationDeps {
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
		deps.RunCommandStep = runCommandStep
	}
	if deps.RunCommandStepEnv == nil {
		deps.RunCommandStepEnv = runCommandStepEnv
	}
	return deps
}
