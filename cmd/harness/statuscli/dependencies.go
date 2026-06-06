package statuscli

import (
	"encoding/json"
	"os"

	"agent-harness/cmd/harness/daemoncli"
	"agent-harness/internal/core"
)

var HarnessRoot = func() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

var ResolveTarget = func(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

var Version = "dev"

var InspectHarness = func(repo string) core.InspectInfo {
	return core.InspectInfo{}
}

var CheckDaemonStatus = daemoncli.CheckDaemonStatus

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
