package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestInstallerName(t *testing.T) {
	if got := NewInstaller().Name(); got != "codex" {
		t.Fatalf("Name() = %q, want %q", got, "codex")
	}
}

func TestCodexInstallerWritesOnlyUserAndHarnessTemplatePaths(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(home, ".codex", "skills", "alpha")); err != nil {
		t.Fatalf("codex user skill link missing: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "[mcp_servers.agent_harness]") || !strings.Contains(string(config), req.BinPath) {
		t.Fatalf("codex config missing harness block:\n%s", string(config))
	}
	hooks, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hooks), "hook user-prompt --host codex") || !strings.Contains(string(hooks), req.BinPath) {
		t.Fatalf("codex hooks missing harness UserPromptSubmit hook:\n%s", string(hooks))
	}
	if !strings.Contains(string(hooks), "hook pre-tool-use --host codex --enforce-worktree") {
		t.Fatalf("codex PreToolUse hook must supply native host identity:\n%s", string(hooks))
	}
	template, err := os.ReadFile(filepath.Join(root, "configs", "codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(template), "hook pre-tool-use --host codex --enforce-worktree") {
		t.Fatalf("codex hook template must supply native host identity:\n%s", string(template))
	}
	if exists(filepath.Join(root, ".claude", "skills", "alpha")) || exists(filepath.Join(root, ".mcp.json")) {
		t.Fatalf("codex installer wrote project-local files")
	}
}

func TestCodexInstallerMergesLifecycleHooksIdempotently(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	hooks, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, subcommand := range []string{"hook user-prompt --host codex", "hook pre-tool-use --host codex --enforce-worktree", "hook post-tool-use", "hook pre-compact", "hook post-compact", "hook stop --enforce-numbered-next-actions"} {
		if count := strings.Count(string(hooks), subcommand); count != 1 {
			t.Fatalf("%s appears %d times, want 1:\n%s", subcommand, count, string(hooks))
		}
	}
	if strings.Contains(string(hooks), "hook pre-tool-use --enforce-search-routing") {
		t.Fatalf("Codex installer must not enable blocking search routing enforcement by default:\n%s", string(hooks))
	}
}

func TestCodexInstallerDropsEmptyHookGroups(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	hooksPath := filepath.Join(home, ".codex", "hooks.json")
	writeFile(t, hooksPath, `{"hooks":{"PostToolUse":[{"matcher":"Write|Edit|Bash","hooks":[]}],"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"echo preserved","timeout":1}]}]}}`)

	if _, err := NewInstaller().Install(req); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	for event, groups := range parsed.Hooks {
		for _, group := range groups {
			hooks, _ := group["hooks"].([]any)
			if len(hooks) == 0 {
				t.Fatalf("installer preserved empty hook group for %s:\n%s", event, string(b))
			}
		}
	}
	if !strings.Contains(string(b), "echo preserved") {
		t.Fatalf("installer dropped non-empty third-party hook group:\n%s", string(b))
	}
}
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
