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
	for _, subcommand := range []string{"hook user-prompt --host codex", "hook pre-tool-use --enforce-worktree", "hook post-tool-use", "hook pre-compact", "hook post-compact", "hook stop --enforce-numbered-next-actions"} {
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

func TestCodexInstallerPatchesUnsupportedPluginHookSuppressOutput(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}

	llmWikiPath := filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki-marketplace", "llm-wiki", "0.2.1", "hooks", "llm-wiki-hook.cjs")
	writeFile(t, llmWikiPath, "process.stdout.write(JSON.stringify({\n  hookSpecificOutput: {\n    hookEventName: eventName,\n    additionalContext: text\n    },\n    suppressOutput: true\n  }));\n")

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	b, err := os.ReadFile(llmWikiPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "suppressOutput") {
		t.Fatalf("unsupported Codex hook field was not patched in %s:\n%s", llmWikiPath, string(b))
	}
	if !exists(llmWikiPath + ".harness.bak") {
		t.Fatalf("backup missing for patched plugin file: %s", llmWikiPath)
	}
	if !containsMessage(result.Messages, "patched Codex plugin hook compatibility") {
		t.Fatalf("patch message missing: %+v", result.Messages)
	}
}

func TestCodexInstallerPatchesClaudeMemCodexHookOutputs(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}

	workerServicePath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.4.0", "scripts", "worker-service.cjs")
	writeFile(t, workerServicePath, `function fZ(t,e){return{continue:!0,suppressOutput:!0,status:t,...e&&{message:e}}}
Fh(s,{continue:!0,suppressOutput:!0})
return{continue:!0,suppressOutput:!0,exitCode:Ke.SUCCESS}
`)
	workerCLIPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.4.0", "scripts", "worker-cli.js")
	writeFile(t, workerCLIPath, `var O='{"continue": true, "suppressOutput": true}';`)
	codexHooksPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.4.0", "hooks", "codex-hooks.json")
	writeFile(t, codexHooksPath, `{"hooks":{"PostToolUse":[{"hooks":[{"command":"node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation"}]}]}}`)

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	for _, path := range []string{workerServicePath, workerCLIPath, codexHooksPath} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, unsupported := range []string{"suppressOutput", "status:t", "message:e"} {
			if strings.Contains(text, unsupported) {
				t.Fatalf("unsupported Codex hook output field %q was not patched in %s:\n%s", unsupported, path, text)
			}
		}
		if path == codexHooksPath && !strings.Contains(text, `hook codex observation || true`) {
			t.Fatalf("Codex claude-mem hook command was not made non-blocking in %s:\n%s", path, text)
		}
		if !exists(path + ".harness.bak") {
			t.Fatalf("backup missing for patched plugin file: %s", path)
		}
	}
	if !containsMessage(result.Messages, "patched Codex plugin hook compatibility") {
		t.Fatalf("patch message missing: %+v", result.Messages)
	}
}

func TestCodexInstallerPatchesClaudeMemCodexHooksIdempotently(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}

	codexHooksPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.4.0", "hooks", "codex-hooks.json")
	writeFile(t, codexHooksPath, `{"hooks":{"PostToolUse":[{"hooks":[{"command":"node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation"}]}]}}`)

	first, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !containsMessage(first.Messages, "patched Codex plugin hook compatibility") {
		t.Fatalf("first install patch message missing: %+v", first.Messages)
	}
	second, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if containsMessage(second.Messages, "patched Codex plugin hook compatibility") {
		t.Fatalf("second install should not patch already-compatible hooks: %+v", second.Messages)
	}

	b, err := os.ReadFile(codexHooksPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if count := strings.Count(text, `hook codex observation || true`); count != 1 {
		t.Fatalf("expected one non-blocking hook command, got %d:\n%s", count, text)
	}
	if strings.Contains(text, `|| true || true`) {
		t.Fatalf("hook command accumulated duplicate non-blocking suffix:\n%s", text)
	}
}

func TestCodexInstallerDryRunPlansPluginHookCompatibilityPatch(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), "", filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	req.DryRun = true

	llmWikiPath := filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki-marketplace", "llm-wiki", "0.2.1", "hooks", "llm-wiki-hook.cjs")
	original := "process.stdout.write(JSON.stringify({\n  hookSpecificOutput: {\n    hookEventName: eventName,\n    additionalContext: text\n    },\n    suppressOutput: true\n  }));\n"
	writeFile(t, llmWikiPath, original)

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(llmWikiPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != original {
		t.Fatalf("dry-run modified plugin file:\n%s", string(b))
	}
	if exists(llmWikiPath + ".harness.bak") {
		t.Fatalf("dry-run wrote backup: %s", llmWikiPath+".harness.bak")
	}
	if !containsMessage(result.Messages, "dry-run: would patch Codex plugin hook compatibility") {
		t.Fatalf("dry-run patch message missing: %+v", result.Messages)
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

func containsMessage(messages []string, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
