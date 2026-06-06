package lifecycle

import "agent-harness/internal/core/commandguard"

func gitOpsKubectlDecision(tool, command string) (string, string) {
	return commandguard.GitOpsKubectlDecision(tool, command)
}

func stagedCheckDecision(req HookToolUseLifecycleRequest) (string, string) {
	return commandguard.StagedCheckDecision(req.Tool, req.Repo, req.Command)
}

func expandPackageScriptCommands(repo, command string) []string {
	return commandguard.ExpandPackageScriptCommands(repo, command)
}

func packageScript(repo, scriptName string) string {
	return commandguard.PackageScript(repo, scriptName)
}

func broadBiomeCheckCommand(command string) bool {
	return commandguard.BroadBiomeCheckCommand(command)
}

func biomeArgsAreScoped(args []string) bool {
	return commandguard.BiomeArgsAreScoped(args)
}

func biomeArgsIncludeBroadRepoDirs(args []string) bool {
	return commandguard.BiomeArgsIncludeBroadRepoDirs(args)
}

func kubectlFlagTakesValue(flag string) bool {
	return commandguard.KubectlFlagTakesValue(flag)
}
