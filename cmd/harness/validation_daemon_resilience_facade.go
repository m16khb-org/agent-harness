package main

import "agent-harness/cmd/harness/validationcli"

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return validationcli.ValidateDaemonRestartResilience(binary, root, seed)
}
