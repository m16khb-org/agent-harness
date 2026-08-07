package commandguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagedCheckDecisionWarnsForBroadBiomeCommands(t *testing.T) {
	repo := t.TempDir()
	writeCommandGuardFile(t, filepath.Join(repo, "package.json"), `{"scripts":{"lint":"biome check apps libs","format":"biome format --staged apps"}}`)

	tests := []struct {
		name       string
		tool       string
		command    string
		wantAction string
	}{
		{name: "direct broad biome check asks", tool: "Bash", command: "biome check apps libs", wantAction: "ask"},
		{name: "scoped biome check is allowed", tool: "Bash", command: "biome check --staged apps libs", wantAction: ""},
		{name: "package script expansion asks", tool: "Bash", command: "npm run lint", wantAction: "ask"},
		{name: "scoped package script is allowed", tool: "Bash", command: "npm run format", wantAction: ""},
		{name: "non shell tools are ignored", tool: "Read", command: "biome check apps libs", wantAction: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, message := StagedCheckDecision(tt.tool, repo, tt.command)
			if action != tt.wantAction {
				t.Fatalf("action=%q, want %q; message=%q", action, tt.wantAction, message)
			}
			if tt.wantAction == "ask" && !strings.Contains(message, "Broad lint/format checks") {
				t.Fatalf("message=%q, want broad-check warning", message)
			}
		})
	}
}

func TestPackageScriptAndBiomeHelpersHandleBoundaries(t *testing.T) {
	repo := t.TempDir()
	writeCommandGuardFile(t, filepath.Join(repo, "package.json"), `{"scripts":{"quoted":"biome check \"apps\" 'libs'","empty":"   "}}`)

	if got := PackageScript(repo, "quoted"); got != `biome check "apps" 'libs'` {
		t.Fatalf("PackageScript returned %q", got)
	}
	if got := PackageScript(repo, "missing"); got != "" {
		t.Fatalf("missing PackageScript returned %q, want empty", got)
	}
	if got := PackageScript("", "quoted"); got != "" {
		t.Fatalf("empty repo PackageScript returned %q, want empty", got)
	}
	if !BroadBiomeCheckCommand(`biome check "apps" 'libs'`) {
		t.Fatalf("quoted broad biome dirs were not detected")
	}
	if BroadBiomeCheckCommand("biome check --since main apps libs") {
		t.Fatalf("scoped biome command should not be broad")
	}
	if BroadBiomeCheckCommand("biome check packages services") {
		t.Fatalf("non-app/lib directories should not count as broad repo dirs")
	}
	if got := PackageScript(repo, "empty"); got != "" {
		t.Fatalf("empty PackageScript returned %q, want empty", got)
	}
}

func writeCommandGuardFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
