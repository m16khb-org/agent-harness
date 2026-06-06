package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	cliadapter "agent-harness/internal/adapter/cli"

	"agent-harness/internal/core"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"
const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024
const selfVerifyStepBudgetMinRegressionMS int64 = 25

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "help", "--help", "-h":
		usage()
	case "version", "--version", "-v":
		fmt.Println("agent-harness", version)
	case "inspect":
		if err := runInspect(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "inspect:", err)
			os.Exit(1)
		}
	case "preflight":
		if err := runPreflight(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "preflight:", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "status:", err)
			os.Exit(1)
		}
	case "doctor":
		if err := runDoctor(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "doctor:", err)
			os.Exit(1)
		}
	case "docs":
		if err := runDocs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "docs:", err)
			os.Exit(1)
		}
	case "policy":
		if err := runPolicy(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "policy:", err)
			if core.IsPolicyDenied(err) {
				os.Exit(3)
			}
			os.Exit(1)
		}
	case "verify-work":
		if err := runVerifyWork(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "verify-work:", err)
			os.Exit(1)
		}
	case "trace":
		if err := runTrace(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "trace:", err)
			os.Exit(1)
		}
	case "guard":
		if err := runGuard(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "guard:", err)
			if core.IsGuardBlocked(err) {
				os.Exit(3)
			}
			os.Exit(1)
		}
	case "self-verify":
		if err := runSelfVerify(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-verify:", err)
			os.Exit(1)
		}
	case "self-augment":
		if err := runSelfAugment(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "self-augment:", err)
			os.Exit(1)
		}
	case "contract":
		if err := runContract(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "contract:", err)
			os.Exit(1)
		}
	case "state":
		if err := runState(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "state:", err)
			os.Exit(1)
		}
	case "issueops":
		if err := runIssueOps(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "issueops:", err)
			os.Exit(1)
		}
	case "api-doc":
		if err := runAPIDoc(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "api-doc:", err)
			os.Exit(1)
		}
	case "hook":
		if err := runHook(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "hook:", err)
			os.Exit(1)
		}
	case "project":
		if err := runProject(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "project:", err)
			os.Exit(1)
		}
	case "install":
		if err := runInstall(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "install:", err)
			os.Exit(1)
		}
	case "install-native":
		if err := runInstallNative(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "install-native:", err)
			os.Exit(1)
		}
	case "update":
		if err := runUpdate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "update:", err)
			os.Exit(1)
		}
	case "bootstrap":
		if err := runBootstrap(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "bootstrap:", err)
			os.Exit(1)
		}
	case "worker":
		if err := runWorker(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "worker:", err)
			os.Exit(1)
		}
	case "daemon":
		if err := runDaemon(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "daemon:", err)
			os.Exit(1)
		}
	case "mcp":
		if err := runMCP(); err != nil {
			fmt.Fprintln(os.Stderr, "mcp:", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fprintUsage(os.Stderr)
}

func fprintUsage(w io.Writer) {
	fprintString(w, cliadapter.Usage(version))
}

func fprintString(w io.Writer, text string) {
	_, _ = fmt.Fprint(w, text)
}

type selfVerifyRunMode struct {
	Full          bool
	Iterations    int
	ContractLabel string
}

func resolveSelfVerifyRunMode(full bool, iterationsFlagSet bool, iterations int) (selfVerifyRunMode, error) {
	if !full {
		if iterationsFlagSet {
			return selfVerifyRunMode{}, fmt.Errorf("--iterations requires --full; default self-verify runs quick one-iteration mode")
		}
		return selfVerifyRunMode{Full: false, Iterations: 1, ContractLabel: "quick one-iteration gate"}, nil
	}
	if iterations < 10 {
		return selfVerifyRunMode{}, fmt.Errorf("full self-verification requires at least 10 iterations; use --full --iterations=10 or higher")
	}
	return selfVerifyRunMode{Full: true, Iterations: iterations, ContractLabel: "full ten-plus-iteration gate"}, nil
}

func flagSetVisited(fs *flag.FlagSet, name string) bool {
	visited := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			visited = true
		}
	})
	return visited
}

func inspectHarness(repoArg string) core.InspectInfo {
	root := harnessRoot()
	target := resolveTarget(repoArg)
	home, _ := os.UserHomeDir()
	return core.InspectHarness(root, target, home, version, skillName)
}

func printJSON(v any) error {
	return printJSONTo(os.Stdout, v)
}
