package main

import (
	"time"

	"agent-harness/cmd/harness/validationcli"
)

type mcpValidationDeps struct {
	mkdirTemp                   func(string, string) (string, error)
	removeAll                   func(string) error
	runCommandStepEnv           func(string, string, time.Duration, string, []string, string, ...string) StepResult
	runCommandStepEnvWithBudget func(string, string, time.Duration, string, []string, int, string, ...string) StepResult
}

func validateMCP(binary, root string) StepResult {
	return validationcli.ValidateMCP(binary, root)
}

func validateMCPWithDeps(binary, root string, deps mcpValidationDeps) StepResult {
	return validationcli.ValidateMCPWithDeps(binary, root, validationcli.MCPValidationDeps{
		MkdirTemp:                   deps.mkdirTemp,
		RemoveAll:                   deps.removeAll,
		RunCommandStepEnv:           deps.runCommandStepEnv,
		RunCommandStepEnvWithBudget: deps.runCommandStepEnvWithBudget,
	})
}

func mcpSmokeInput() string {
	return validationcli.MCPSmokeInput()
}

func validateMCPSmokeContract(step *StepResult) {
	validationcli.ValidateMCPSmokeContract(step)
}

func mcpSmokeHasExpectedMarkers(stdout string) bool {
	return validationcli.MCPSmokeHasExpectedMarkers(stdout)
}

func mcpSmokeExpectedMarkers() []string {
	return validationcli.MCPSmokeExpectedMarkers()
}
