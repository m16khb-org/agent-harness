package harnessapp

import (
	"os"

	"agent-harness/cmd/harness/rootcmd"
	"agent-harness/internal/core"
)

func RunRootCommand(args []string) int {
	return rootCommand().Run(args)
}

func rootCommand() rootcmd.Command {
	return rootcmd.Command{
		Version: version,
		Usage:   usage,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Runners: map[string]rootcmd.Runner{
			"inspect":        runInspect,
			"preflight":      runPreflight,
			"status":         runStatus,
			"doctor":         runDoctor,
			"docs":           runDocs,
			"policy":         runPolicy,
			"verify-work":    runVerifyWork,
			"trace":          runTrace,
			"guard":          runGuard,
			"quality":        runQuality,
			"self-verify":    runSelfVerify,
			"self-augment":   runSelfAugment,
			"contract":       runContract,
			"state":          runState,
			"issueops":       runIssueOps,
			"api-doc":        runAPIDoc,
			"hook":           runHook,
			"project":        runProject,
			"install":        runInstall,
			"install-native": runInstallNative,
			"update":         runUpdate,
			"bootstrap":      runBootstrap,
			"worker":         runWorker,
			"daemon":         runDaemon,
			"mcp":            runMCPNoArgs,
		},
		ErrorExitCode: rootSubcommandErrorExitCode,
	}
}

func runMCPNoArgs(_ []string) error {
	return runMCP()
}

func rootSubcommandErrorExitCode(name string, err error) int {
	switch name {
	case "policy":
		if core.IsPolicyDenied(err) {
			return 3
		}
	case "guard":
		if core.IsGuardBlocked(err) {
			return 3
		}
	}
	return 1
}
