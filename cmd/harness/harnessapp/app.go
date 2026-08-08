package harnessapp

import (
	"fmt"
	"io"
	"os"

	cliadapter "agent-harness/internal/adapter/cli"
	inspect "agent-harness/internal/adapter/inspect"
	inspectcontract "agent-harness/internal/contract/inspect"
)

const version = "0.1.0"
const skillName = "atomic-commit-push"
const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

func usage() {
	fprintUsage(os.Stderr)
}

func fprintUsage(w io.Writer) {
	fprintString(w, cliadapter.Usage(version))
}

func fprintString(w io.Writer, text string) {
	_, _ = fmt.Fprint(w, text)
}

func inspectHarness(repoArg string) inspectcontract.InspectInfo {
	root := harnessRoot()
	target := resolveTarget(repoArg)
	home, _ := os.UserHomeDir()
	return inspect.InspectHarness(root, target, home, version, skillName)
}

func printJSON(v any) error {
	return printJSONTo(os.Stdout, v)
}
