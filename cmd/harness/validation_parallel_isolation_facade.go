package main

import "agent-harness/cmd/harness/validationcli"

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return validationcli.ValidateParallelTempIsolation(binary, root, seed)
}
