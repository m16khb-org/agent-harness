package validationcli

import "agent-harness/cmd/harness/validationcli/stateroundtrip"

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return stateroundtrip.Validate(binary, root, seed)
}
