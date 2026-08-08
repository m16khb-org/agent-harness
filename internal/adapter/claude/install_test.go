package claude

import (
	install "agent-harness/internal/adapter/install"
	hook "agent-harness/internal/domain/hook"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeInstallerDefaultsToUserScopeOnly(t *testing.T) {
	if got := NewInstaller().Name(); got != "claude" {
		t.Fatalf("installer name = %q, want claude", got)
	}
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
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
	for _, needle := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "hook user-prompt", "hook pre-tool-use", "hook post-tool-use", "hook stop", req.BinPath} {
		if !strings.Contains(settings, needle) {
			t.Fatalf("claude settings missing %q:\n%s", needle, settings)
		}
	}
	mcp := readClaudeTestFile(t, filepath.Join(home, ".claude.json"))
	if !strings.Contains(mcp, `"agent_harness"`) || !strings.Contains(mcp, req.BinPath) || !strings.Contains(mcp, `"HARNESS_ROOT"`) {
		t.Fatalf("claude user MCP config missing exact harness server:\n%s", mcp)
	}
	if strings.Contains(mcp, `"agent-harness"`) {
		t.Fatalf("claude user MCP config retained obsolete alias:\n%s", mcp)
	}
	for _, path := range []string{filepath.Join(root, ".claude", "skills", "alpha"), filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".mcp.json")} {
		if exists(path) {
			t.Fatalf("default installer wrote unexpected path %s", path)
		}
	}
}

func TestOwnershipTransferClaudeHostPreservesPermissionDecisionMeaning(t *testing.T) {
	output := hook.ClaudeHookOutput{}.FormatBlock("worker ownership is sealed")
	specific, ok := output["hookSpecificOutput"].(map[string]any)
	if !ok || specific["permissionDecision"] != "deny" || specific["permissionDecisionReason"] != "worker ownership is sealed" {
		t.Fatalf("Claude ownership block must preserve permissionDecision deny semantics: %#v", output)
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
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
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
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
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
	for _, event := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PreCompact", "PostCompact", "Stop"} {
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
	preToolUse := hooks["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
	if !strings.Contains(preToolUse, "hook pre-tool-use --host claude --enforce-worktree") {
		t.Fatalf("PreToolUse should preserve Claude host schema: %s", preToolUse)
	}
	if strings.Contains(preToolUse, "--enforce-search-routing") {
		t.Fatalf("PreToolUse must not enable blocking search routing enforcement by default: %s", preToolUse)
	}
	stop := hooks["Stop"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
	if !strings.Contains(stop, "hook stop --host claude --enforce-numbered-next-actions") {
		t.Fatalf("Stop should be strict-ready for numbered next actions: %s", stop)
	}
}

func TestClaudeInstallerReportsInvalidExistingSettings(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	writeClaudeTestFile(t, filepath.Join(home, ".claude", "settings.json"), "{")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "harness"))
	req.SkillNames = []string{"alpha"}

	result, err := NewInstaller().Install(req)
	if err == nil {
		t.Fatalf("invalid existing settings should fail")
	}
	if result.OK {
		t.Fatalf("invalid settings result should be not OK: %+v", result)
	}
	if !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("invalid settings error = %v", err)
	}
}

func TestClaudeInstallerReportsStaleHookTarget(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "install", true: "dry-run"}[dryRun], func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			writeAdapterTestSkill(t, root, "alpha")
			settingsPath := filepath.Join(home, ".claude", "settings.json")
			writeClaudeTestFile(t, settingsPath, `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"'/source.worktrees/completed/bin/agent-harness' hook pre-tool-use --host claude"}]}]}}`)
			expected := filepath.Join(root, "bin", "agent-harness")
			req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), expected)
			req.SkillNames = []string{"alpha"}
			req.DryRun = dryRun

			result, err := NewInstaller().Install(req)
			if err != nil {
				t.Fatal(err)
			}
			want := "claude native hook target is stale: observed=/source.worktrees/completed/bin/agent-harness expected=" + expected + "; reinstall hooks and restart the claude session"
			if countClaudeMessage(result.Messages, want) != 1 {
				t.Fatalf("messages = %#v, want exactly one %q", result.Messages, want)
			}
		})
	}
}

func countClaudeMessage(messages []string, want string) int {
	count := 0
	for _, message := range messages {
		if message == want {
			count++
		}
	}
	return count
}

func TestClaudeInstallHelpersCoverQuoting(t *testing.T) {
	if got := shellQuote(""); got != "''" {
		t.Fatalf("empty shellQuote = %q", got)
	}
	if got := shellQuote("it's/bin"); got != `'it'"'"'s/bin'` {
		t.Fatalf("quoted shellQuote = %q", got)
	}
}
