package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestClaudeInstallerDefaultsToUserScopeOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	if !exists(filepath.Join(home, ".claude", "skills", "alpha")) {
		t.Fatalf("claude user skill link missing")
	}
	settings := readClaudeTestFile(t, filepath.Join(home, ".claude", "settings.json"))
	for _, needle := range []string{"UserPromptSubmit", "PostToolUse", "Stop", "hook user-prompt", "hook post-tool-use", "hook stop", req.BinPath} {
		if !strings.Contains(settings, needle) {
			t.Fatalf("claude settings missing %q:\n%s", needle, settings)
		}
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".mcp.json")} {
		if exists(path) {
			t.Fatalf("default installer wrote unexpected path %s", path)
		}
	}
}

func readClaudeTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeClaudeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeInstallerProjectLocalIsExplicit(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	req.ProjectLocal = true
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".mcp.json")} {
		if !exists(path) {
			t.Fatalf("project-local installer did not write %s", path)
		}
	}
	if exists(filepath.Join(root, ".claude", "settings.json")) {
		t.Fatalf("project-local installer should not write repo-local Claude settings")
	}
}

func TestClaudeInstallerMergesLifecycleHooksIdempotently(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeClaudeTestFile(t, settingsPath, `{
  "theme": "dark",
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo keep"
          }
        ]
      }
    ]
  }
}
`)
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(readClaudeTestFile(t, settingsPath)), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["theme"] != "dark" {
		t.Fatalf("existing setting was not preserved: %+v", settings)
	}
	hooks := settings["hooks"].(map[string]any)
	for _, event := range []string{"UserPromptSubmit", "PostToolUse", "PreCompact", "PostCompact", "Stop"} {
		groups := hooks[event].([]any)
		count := 0
		for _, group := range groups {
			for _, hook := range group.(map[string]any)["hooks"].([]any) {
				cmd, _ := hook.(map[string]any)["command"].(string)
				if strings.Contains(cmd, "agent-harness") || (strings.Contains(cmd, "harness") && strings.Contains(cmd, " hook ")) {
					count++
				}
			}
		}
		if count != 1 {
			t.Fatalf("event %s has %d harness hooks, want 1: %+v", event, count, groups)
		}
	}
}
