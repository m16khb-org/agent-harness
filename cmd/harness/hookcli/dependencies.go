package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"

	coreinstall "agent-harness/internal/core/install"
)

var ResolveTarget = func(arg string) string {
	if arg == "" {
		if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := os.Getenv("PWD"); env != "" {
			arg = env
		} else if cwd, err := os.Getwd(); err == nil {
			arg = cwd
		} else {
			arg = "."
		}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	return abs
}

var DiagnoseCurrentNativeRuntime = func() (coreinstall.NativeRuntimeDiagnostic, error) {
	executable, err := os.Executable()
	if err != nil {
		return coreinstall.NativeRuntimeDiagnostic{}, err
	}
	return coreinstall.DiagnoseNativeRuntime(executable)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
