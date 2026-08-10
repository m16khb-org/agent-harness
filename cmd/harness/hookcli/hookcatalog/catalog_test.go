package hookcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCatalogHooksWithInjectedPrinter(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "CONSTITUTION.md"), []byte("# Constitution\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var printed []any
	config := Config{
		ResolveTarget: func(string) string { return repo },
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
	}
	if err := RunPostCompact([]string{"--repo", repo, "--json"}, config); err != nil {
		t.Fatalf("RunPostCompact json returned error: %v", err)
	}
	if err := RunSessionStart([]string{"--repo", repo, "--json"}, config); err != nil {
		t.Fatalf("RunSessionStart json returned error: %v", err)
	}
	if len(printed) != 2 {
		t.Fatalf("expected two printed values, got %#v", printed)
	}
	if fmt.Sprint(printed[0]) == "" || fmt.Sprint(printed[1]) == "" {
		t.Fatalf("printed values should be non-empty: %#v", printed)
	}
}

func TestRunCatalogHooksIgnoreLegacyRuntimeDependencies(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			var printed map[string]any
			config := Config{
				ResolveTarget: func(string) string { return t.TempDir() },
				PrintJSON: func(value any) error {
					printed, _ = value.(map[string]any)
					return nil
				},
			}
			if err := RunSessionStart([]string{"--host", host}, config); err != nil {
				t.Fatalf("RunSessionStart returned error: %v", err)
			}
			if printed == nil {
				t.Fatalf("%s session-start must render a host-compatible response", host)
			}
		})
	}
}

func TestRunCatalogHooksFormatsHostOutputs(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "AGENT_WORKFLOW.md"), []byte("# Agent Workflow\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var printed []any
	config := Config{
		ResolveTarget: func(string) string { return repo },
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
	}
	if err := RunPostCompact([]string{"--repo", repo, "--host", "codex"}, config); err != nil {
		t.Fatalf("RunPostCompact codex returned error: %v", err)
	}
	if err := RunSessionStart([]string{"--repo", repo, "--host", "claude"}, config); err != nil {
		t.Fatalf("RunSessionStart claude returned error: %v", err)
	}
	if len(printed) != 2 {
		t.Fatalf("expected two printed host outputs, got %#v", printed)
	}
	if !strings.Contains(fmt.Sprint(printed[0]), "systemMessage") {
		t.Fatalf("codex output should contain systemMessage shape, got %#v", printed[0])
	}
}

func TestCatalogHostHelpers(t *testing.T) {
	codex := " CODEX "
	if hostOf(&codex) != "codex" {
		t.Fatalf("hostOf = %q", hostOf(&codex))
	}
	for _, host := range []string{"codex", "claude", ""} {
		host := host
		if resolveCatalogHost(&host) == nil {
			t.Fatalf("expected host output for %q", host)
		}
	}
}

func TestCatalogHookFlagErrors(t *testing.T) {
	config := Config{ResolveTarget: func(string) string { return t.TempDir() }, PrintJSON: func(any) error { return nil }}
	if err := RunPostCompact([]string{"--bad"}, config); err == nil {
		t.Fatal("expected post-compact flag error")
	}
	if err := RunSessionStart([]string{"--bad"}, config); err == nil {
		t.Fatal("expected session-start flag error")
	}
}
