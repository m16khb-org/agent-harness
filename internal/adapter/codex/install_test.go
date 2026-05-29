package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestCodexInstallerWritesOnlyUserAndHarnessTemplatePaths(t *testing.T) {
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
	if !strings.Contains(string(hooks), "hook user-prompt") || !strings.Contains(string(hooks), req.BinPath) {
		t.Fatalf("codex hooks missing harness UserPromptSubmit hook:\n%s", string(hooks))
	}
	if exists(filepath.Join(root, ".claude", "skills", "alpha")) || exists(filepath.Join(root, ".mcp.json")) {
		t.Fatalf("codex installer wrote project-local files")
	}
}

func TestCodexInstallerPatchesUnsupportedPluginHookSuppressOutput(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}

	llmWikiPath := filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki-marketplace", "llm-wiki", "0.2.1", "hooks", "llm-wiki-hook.cjs")
	writeFile(t, llmWikiPath, "process.stdout.write(JSON.stringify({\n  hookSpecificOutput: {\n    hookEventName: eventName,\n    additionalContext: text\n    },\n    suppressOutput: true\n  }));\n")
	claudeMemCodexPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.3.0", "scripts", "worker-service.cjs")
	writeFile(t, claudeMemCodexPath, claudeMemUnsupportedHookOutput)
	claudeMemCodexHooksPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.3.0", "hooks", "hooks.json")
	writeFile(t, claudeMemCodexHooksPath, claudeMemUnsupportedHookJSON)
	claudeMemHomePath := filepath.Join(home, ".claude", "plugins", "marketplaces", "thedotmack", "plugin", "scripts", "worker-service.cjs")
	writeFile(t, claudeMemHomePath, claudeMemUnsupportedHookOutput)
	claudeMemHomeHooksPath := filepath.Join(home, ".claude", "plugins", "marketplaces", "thedotmack", "plugin", "hooks", "hooks.json")
	writeFile(t, claudeMemHomeHooksPath, claudeMemUnsupportedHookJSON)

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	for _, path := range []string{llmWikiPath, claudeMemCodexPath, claudeMemCodexHooksPath, claudeMemHomePath, claudeMemHomeHooksPath} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, unsupported := range []string{"suppressOutput", "systemMessage", "continue:!0", "status:t"} {
			if strings.Contains(string(b), unsupported) {
				t.Fatalf("unsupported Codex SessionStart hook field %q was not patched in %s:\n%s", unsupported, path, string(b))
			}
		}
		if !exists(path + ".harness.bak") {
			t.Fatalf("backup missing for patched plugin file: %s", path)
		}
	}
	if !containsMessage(result.Messages, "patched Codex plugin hook compatibility") {
		t.Fatalf("patch message missing: %+v", result.Messages)
	}
}

func TestCodexInstallerDryRunPlansPluginHookCompatibilityPatch(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := core.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}
	req.DryRun = true

	claudeMemPath := filepath.Join(req.CodexHome, "plugins", "cache", "claude-mem-local", "claude-mem", "13.3.0", "scripts", "worker-service.cjs")
	original := claudeMemUnsupportedHookOutput
	writeFile(t, claudeMemPath, original)

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(claudeMemPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != original {
		t.Fatalf("dry-run modified plugin file:\n%s", string(b))
	}
	if exists(claudeMemPath + ".harness.bak") {
		t.Fatalf("dry-run wrote backup: %s", claudeMemPath+".harness.bak")
	}
	if !containsMessage(result.Messages, "dry-run: would patch Codex plugin hook compatibility") {
		t.Fatalf("dry-run patch message missing: %+v", result.Messages)
	}
}

const claudeMemUnsupportedHookOutput = `function CBe(t,e){return{continue:!0,suppressOutput:!0,status:t,...e&&{message:e}}}formatOutput(t){let e=t??{};if(e.hookSpecificOutput){let n={hookSpecificOutput:t.hookSpecificOutput};return e.systemMessage&&(n.systemMessage=e.systemMessage),n}let r={};return e.systemMessage&&(r.systemMessage=e.systemMessage),r}function gqt(t){let e={};return t.continue!==void 0&&(e.continue=t.continue),t.suppressOutput!==void 0&&(e.suppressOutput=t.suppressOutput),t.systemMessage&&(e.systemMessage=t.systemMessage),t.decision==="block"&&(e.decision="block"),t.reason&&(e.reason=t.reason),e}
`

const claudeMemUnsupportedHookJSON = `{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "command": "node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" start; echo '{\"continue\":true,\"suppressOutput\":true}'"
          }
        ]
      }
    ]
  }
}
`

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
