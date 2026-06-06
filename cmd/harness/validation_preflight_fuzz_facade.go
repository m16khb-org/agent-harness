package main

import "agent-harness/cmd/harness/validationcli"

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return validationcli.ValidatePreflightFuzz(binary, root, seed)
}
