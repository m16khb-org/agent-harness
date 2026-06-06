package validationcli

import "agent-harness/cmd/harness/validationcli/daemonresilience"

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return daemonresilience.Validate(binary, root, seed)
}
