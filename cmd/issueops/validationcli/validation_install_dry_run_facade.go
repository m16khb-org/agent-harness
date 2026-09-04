package validationcli

import "issueops/cmd/issueops/validationcli/installdryrun"

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return installdryrun.Validate(binary, root, seed)
}
