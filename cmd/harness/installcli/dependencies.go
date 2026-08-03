package installcli

import (
	"encoding/json"
	"os"

	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

// Deps holds host-provided dependencies for the install CLI. The composition
// root injects implementations via Configure; defaults support standalone
// use/tests.
type Deps struct {
	HarnessRoot        func() string
	ExecutablePath     func() (string, error)
	ActivationBackend  activationport.Backend
	ActivationReadback activationport.ReadbackVerifier
	HostInstallers     []port.HostInstaller
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) {
	if d.ExecutablePath == nil {
		d.ExecutablePath = os.Executable
	}
	deps = d
}

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{HarnessRoot: defaultHarnessRoot, ExecutablePath: os.Executable}
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
