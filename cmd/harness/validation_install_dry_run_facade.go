package main

import "agent-harness/cmd/harness/validationcli"

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return validationcli.ValidateInstallDryRunSmoke(binary, root, seed)
}
