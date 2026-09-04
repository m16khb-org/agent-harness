package statuscli

import (
	"encoding/json"
	preflightcontract "issueops/internal/contract/preflight"
	projectdocdomain "issueops/internal/domain/projectdoc"
	"os"

	"issueops/cmd/issueops/daemoncli"
	inspect "issueops/internal/contract/inspect"
)

// Deps holds host-provided dependencies for the status CLI. The composition root
// injects implementations via Configure; defaults support standalone use/tests.
type Deps struct {
	// AnalyzeProjectSignals는 composition root가 주입한다.
	AnalyzeProjectSignals func(root string) projectdocdomain.ProjectSignals
	IssueOpsRoot          func() string
	ResolveTarget         func(string) string
	Version               string
	InspectHarness        func(string) inspect.InspectInfo
	CheckDaemonStatus     func() daemoncli.Status
	// GitPreflight는 composition root가 주입한다.
	GitPreflight func(target, issueOpsRoot string) preflightcontract.PreflightResult
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) { deps = d }

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{
		IssueOpsRoot:      defaultIssueOpsRoot,
		ResolveTarget:     defaultResolveTarget,
		Version:           "dev",
		InspectHarness:    func(string) inspect.InspectInfo { return inspect.InspectInfo{} },
		CheckDaemonStatus: daemoncli.CheckDaemonStatus,
	}
}

func defaultIssueOpsRoot() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func defaultResolveTarget(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
