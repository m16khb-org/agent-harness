package main

import (
	"fmt"
	"os"

	"agent-harness/internal/core"
)

type rootSubcommandRunner func([]string) error

func runRootSubcommand(name string, args []string) (bool, int) {
	switch name {
	case "inspect":
		return true, runRootSubcommandRunner(name, args, runInspect)
	case "preflight":
		return true, runRootSubcommandRunner(name, args, runPreflight)
	case "status":
		return true, runRootSubcommandRunner(name, args, runStatus)
	case "doctor":
		return true, runRootSubcommandRunner(name, args, runDoctor)
	case "docs":
		return true, runRootSubcommandRunner(name, args, runDocs)
	case "policy":
		return true, runRootSubcommandRunner(name, args, runPolicy)
	case "verify-work":
		return true, runRootSubcommandRunner(name, args, runVerifyWork)
	case "trace":
		return true, runRootSubcommandRunner(name, args, runTrace)
	case "guard":
		return true, runRootSubcommandRunner(name, args, runGuard)
	case "self-verify":
		return true, runRootSubcommandRunner(name, args, runSelfVerify)
	case "self-augment":
		return true, runRootSubcommandRunner(name, args, runSelfAugment)
	case "contract":
		return true, runRootSubcommandRunner(name, args, runContract)
	case "state":
		return true, runRootSubcommandRunner(name, args, runState)
	case "issueops":
		return true, runRootSubcommandRunner(name, args, runIssueOps)
	case "api-doc":
		return true, runRootSubcommandRunner(name, args, runAPIDoc)
	case "hook":
		return true, runRootSubcommandRunner(name, args, runHook)
	case "project":
		return true, runRootSubcommandRunner(name, args, runProject)
	case "install":
		return true, runRootSubcommandRunner(name, args, runInstall)
	case "install-native":
		return true, runRootSubcommandRunner(name, args, runInstallNative)
	case "update":
		return true, runRootSubcommandRunner(name, args, runUpdate)
	case "bootstrap":
		return true, runRootSubcommandRunner(name, args, runBootstrap)
	case "worker":
		return true, runRootSubcommandRunner(name, args, runWorker)
	case "daemon":
		return true, runRootSubcommandRunner(name, args, runDaemon)
	case "mcp":
		if err := runMCP(); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			return true, 1
		}
		return true, 0
	default:
		return false, 0
	}
}

func runRootSubcommandRunner(name string, args []string, runner rootSubcommandRunner) int {
	if err := runner(args); err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		return rootSubcommandErrorExitCode(name, err)
	}
	return 0
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
