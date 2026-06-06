package main

import "agent-harness/cmd/harness/validationcli"

func validateCommandPolicy(binary, root string) StepResult {
	return validationcli.ValidateCommandPolicy(binary, root)
}
