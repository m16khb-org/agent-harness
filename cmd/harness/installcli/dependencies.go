package installcli

import (
	"encoding/json"
	"os"

	activationport "agent-harness/internal/port/nativeactivation"
)

// Deps holds host-provided dependencies for the install CLI. The composition
// root injects implementations via Configure; defaults support standalone
// use/tests.
type Deps struct {
	HarnessRoot       func() string
	ActivationBackend activationport.Backend
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) { deps = d }

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{HarnessRoot: defaultHarnessRoot}
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

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
