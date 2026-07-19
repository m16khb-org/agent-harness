package basiccli

import (
	"context"
	"encoding/json"
	"os"

	"agent-harness/cmd/harness/daemoncli"
	"agent-harness/internal/core"
	"agent-harness/internal/core/operationalhealth"
)

// Deps holds the host-provided implementations the basic CLI commands depend on.
// The composition root injects real implementations via Configure; standalone
// use and tests fall back to defaults.
type Deps struct {
	HarnessRoot              func() string
	ResolveTarget            func(string) string
	Version                  string
	InspectHarness           func(string) core.InspectInfo
	CheckDaemonStatus        func() daemoncli.Status
	CollectOperationalHealth func(context.Context, string) operationalhealth.Snapshot
}

// deps holds the currently configured dependencies. It is package-private and
// only mutated through Configure/Reset so wiring is explicit rather than an
// import-order-sensitive init() side effect.
var deps = defaultDeps()

// Configure installs host-provided dependencies. The composition root calls this
// once at startup; tests call it with fakes and restore with Reset via t.Cleanup.
func Configure(d Deps) { deps = d }

// Reset restores the standalone defaults. Tests defer this to avoid cross-test
// leakage of injected fakes.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{
		HarnessRoot:       defaultHarnessRoot,
		ResolveTarget:     defaultResolveTarget,
		Version:           "dev",
		InspectHarness:    func(string) core.InspectInfo { return core.InspectInfo{} },
		CheckDaemonStatus: daemoncli.CheckDaemonStatus,
		CollectOperationalHealth: func(_ context.Context, repo string) operationalhealth.Snapshot {
			return operationalhealth.Snapshot{
				RepoRoot: repo,
				InventoryProblems: []operationalhealth.InventoryProblem{{
					Source: "doctor", Code: "operational_collector_unconfigured", Detail: "operational inventory collector is not configured",
				}},
			}
		},
	}
}

func defaultHarnessRoot() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
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
