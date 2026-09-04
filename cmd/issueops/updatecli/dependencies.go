package updatecli

import "os"

// Deps holds host-provided dependencies for the update CLI. The composition root
// injects implementations via Configure; defaults support standalone use/tests.
type Deps struct {
	IssueOpsRoot func() string
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) { deps = d }

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{IssueOpsRoot: defaultIssueOpsRoot}
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
