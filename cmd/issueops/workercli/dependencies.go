package workercli

import (
	"encoding/json"
	"os"
	"strings"
)

// Deps holds host-provided dependencies for the worker CLI. The composition root
// injects implementations via Configure; defaults support standalone use/tests.
type Deps struct {
	ResolveTarget func(string) string
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) { deps = d }

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{ResolveTarget: defaultResolveTarget}
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

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
