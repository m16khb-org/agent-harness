package validationcli

import "issueops/cmd/issueops/validationcli/stateroundtrip"

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return stateroundtrip.Validate(binary, root, seed)
}
