package qualitycli

import (
	"encoding/json"
	"fmt"
	statecontract "issueops/internal/contract/state"
	"os"
)

// Deps holds host-provided dependencies for the quality CLI. The composition
// root injects implementations via Configure; defaults support standalone
// use/tests.
type Deps struct {
	IssueOpsRoot func() string
	Version      string
	PrintJSON    func(any) error

	// StateRead와 StateWrite는 composition root가 주입한다. default를 두면 이
	// package가 concrete state store를 알게 되므로 비워 둔다.
	StateRead  func(key string) (statecontract.StateResult, error)
	StateWrite func(key, content string) (statecontract.StateResult, error)
}

// hostDeps holds the host-provided dependencies. It is named distinctly from the
// per-call InspectDeps parameter used elsewhere in this package to avoid
// shadowing.
var hostDeps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) { hostDeps = d }

// Reset restores standalone defaults.
func Reset() { hostDeps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{
		IssueOpsRoot: defaultIssueOpsRoot,
		Version:      "dev",
		PrintJSON:    defaultPrintJSON,
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

func defaultPrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(b))
	return err
}
