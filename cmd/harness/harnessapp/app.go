package harnessapp

import (
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

func usage() {
	fprintUsage(os.Stderr)
}

func fprintUsage(w io.Writer) {
	fprintString(w, cliadapter.Usage(version))
}

func fprintString(w io.Writer, text string) {
	_, _ = fmt.Fprint(w, text)
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
