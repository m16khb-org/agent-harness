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
	os.Exit(runRootCommand(os.Args[1:]))
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
