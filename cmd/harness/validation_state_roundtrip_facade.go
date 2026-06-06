package main

import "agent-harness/cmd/harness/validationcli"

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return validationcli.ValidateStateRoundtrip(binary, root, seed)
}
