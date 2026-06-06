package validationcli

import "agent-harness/cmd/harness/validationcli/installdryrun"

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return installdryrun.Validate(binary, root, seed)
}
