package validationcli

import "issueops/cmd/issueops/validationcli/daemonresilience"

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return daemonresilience.Validate(binary, root, seed)
}
