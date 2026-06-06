package main

import (
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func runRootCommand(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}

	switch args[0] {
	case "help", "--help", "-h":
		usage()
		return 0
	case "version", "--version", "-v":
		fmt.Println("agent-harness", version)
		return 0
	case "inspect":
		if err := runInspect(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "inspect:", err)
			return 1
		}
	case "preflight":
		if err := runPreflight(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "preflight:", err)
			return 1
		}
	case "status":
		if err := runStatus(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "status:", err)
			return 1
		}
	case "doctor":
		if err := runDoctor(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "doctor:", err)
			return 1
		}
	case "docs":
		if err := runDocs(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "docs:", err)
			return 1
		}
	case "policy":
		if err := runPolicy(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "policy:", err)
			if core.IsPolicyDenied(err) {
				return 3
			}
			return 1
		}
	case "verify-work":
		if err := runVerifyWork(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "verify-work:", err)
			return 1
		}
	case "trace":
		if err := runTrace(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "trace:", err)
			return 1
		}
	case "guard":
		if err := runGuard(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "guard:", err)
			if core.IsGuardBlocked(err) {
				return 3
			}
			return 1
		}
	case "self-verify":
		if err := runSelfVerify(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-verify:", err)
			return 1
		}
	case "self-augment":
		if err := runSelfAugment(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-augment:", err)
			return 1
		}
	case "contract":
		if err := runContract(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "contract:", err)
			return 1
		}
	case "state":
		if err := runState(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "state:", err)
			return 1
		}
	case "issueops":
		if err := runIssueOps(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "issueops:", err)
			return 1
		}
	case "api-doc":
		if err := runAPIDoc(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "api-doc:", err)
			return 1
		}
	case "hook":
		if err := runHook(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "hook:", err)
			return 1
		}
	case "project":
		if err := runProject(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "project:", err)
			return 1
		}
	case "install":
		if err := runInstall(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "install:", err)
			return 1
		}
	case "install-native":
		if err := runInstallNative(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "install-native:", err)
			return 1
		}
	case "update":
		if err := runUpdate(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "update:", err)
			return 1
		}
	case "bootstrap":
		if err := runBootstrap(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "bootstrap:", err)
			return 1
		}
	case "worker":
		if err := runWorker(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "worker:", err)
			return 1
		}
	case "daemon":
		if err := runDaemon(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "daemon:", err)
			return 1
		}
	case "mcp":
		if err := runMCP(); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			return 1
		}
	default:
		usage()
		return 2
	}
	return 0
}
