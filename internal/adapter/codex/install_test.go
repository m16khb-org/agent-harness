package codex

import (
	"encoding/json"
	"fmt"
	install "issueops/internal/adapter/install"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
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
	if !strings.Contains(string(config), "[mcp_servers.issueops]") || !strings.Contains(string(config), req.BinPath) {
		t.Fatalf("codex config missing harness block:\n%s", string(config))
	}
	hooks, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, subcommand := range []string{"hook session-start --host codex"} {
		if !strings.Contains(string(hooks), subcommand) || !strings.Contains(string(hooks), req.BinPath) {
			t.Fatalf("codex hooks missing context-only command %q:\n%s", subcommand, string(hooks))
		}
	}
	for _, forbidden := range []string{"hook user-prompt", "hook pre-tool-use", "hook post-tool-use", "hook pre-compact", "hook post-compact", "hook stop", "--enforce-", "--relay-next-action-judgement"} {
		if strings.Contains(string(hooks), forbidden) {
			t.Fatalf("codex default hooks must not contain %q:\n%s", forbidden, string(hooks))
		}
	}
	template, err := os.ReadFile(filepath.Join(root, "configs", "codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(template), "hook pre-tool-use") || strings.Contains(string(template), "--enforce-") || strings.Contains(string(template), "hook stop") {
		t.Fatalf("codex hook template must contain only context hooks:\n%s", string(template))
	}
	if exists(filepath.Join(root, ".claude", "skills", "alpha")) || exists(filepath.Join(root, ".mcp.json")) {
		t.Fatalf("codex installer wrote project-local files")
	}
}

func TestCodexInstallerMergesLifecycleHooksIdempotently(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
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
	for _, subcommand := range []string{"hook session-start --host codex"} {
		if count := strings.Count(string(hooks), subcommand); count != 1 {
			t.Fatalf("%s appears %d times, want 1:\n%s", subcommand, count, string(hooks))
		}
	}
	for _, forbidden := range []string{"hook user-prompt", "hook pre-tool-use", "hook post-tool-use", "hook pre-compact", "hook post-compact", "hook stop", "--enforce-", "--relay-next-action-judgement"} {
		if strings.Contains(string(hooks), forbidden) {
			t.Fatalf("Codex installer must remove default hook %q:\n%s", forbidden, string(hooks))
		}
	}
}

func TestMergeHookConfigPreservesCoResidentHookPositions(t *testing.T) {
	agentHarness := map[string]any{"hooks": []any{map[string]any{
		"type": "command", "command": "'/old/bin/issueops' hook pre-tool-use --host codex", "timeout": float64(5),
	}}}
	orca := map[string]any{"hooks": []any{map[string]any{
		"type": "command", "command": "/bin/sh /Users/example/.orca/agent-hooks/codex-hook.sh", "timeout": float64(10),
	}}}
	config := map[string]any{"hooks": map[string]any{"PreToolUse": []any{agentHarness, orca}}}

	merged := mergeHookConfig(config, "/new/bin/issueops")
	groups := merged["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(groups) != 1 {
		t.Fatalf("PreToolUse groups = %d, want only the third-party group: %#v", len(groups), groups)
	}
	command := groups[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
	if command != "/bin/sh /Users/example/.orca/agent-hooks/codex-hook.sh" {
		t.Fatalf("legacy issueops group must be removed while third-party group is preserved: %q", command)
	}
	for _, event := range []string{"SessionStart"} {
		if len(merged["hooks"].(map[string]any)[event].([]any)) != 1 {
			t.Fatalf("%s must contain one replacement context hook: %#v", event, merged)
		}
	}
}

func TestMergeHookConfigReplacesManagedContextGroupsInPlace(t *testing.T) {
	managed := func(command string) map[string]any {
		return map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": command, "timeout": float64(5),
		}}}
	}
	thirdParty := func(command string) map[string]any {
		return map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": command, "timeout": float64(10),
		}}}
	}
	for name, groups := range map[string][]any{
		"managed before third party": {
			managed("'/old/bin/issueops' hook session-start --host codex"),
			thirdParty("orca observe"),
		},
		"managed after third party": {
			thirdParty("codegraph observe"),
			managed("'/old/bin/issueops' hook session-start --host codex"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			merged := mergeHookConfig(map[string]any{"hooks": map[string]any{"SessionStart": groups}}, "/new/bin/issueops")
			got := merged["hooks"].(map[string]any)["SessionStart"].([]any)
			if len(got) != 2 {
				t.Fatalf("SessionStart groups = %d, want 2: %#v", len(got), got)
			}
			for index, group := range groups {
				command := group.(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
				if command == "orca observe" || command == "codegraph observe" {
					gotCommand := got[index].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
					if gotCommand != command {
						t.Fatalf("third-party group moved from index %d: got %q want %q", index, gotCommand, command)
					}
				}
			}
			if count := strings.Count(fmt.Sprint(got), "'/new/bin/issueops' hook session-start --host codex"); count != 1 {
				t.Fatalf("canonical managed group count = %d, want 1: %#v", count, got)
			}
		})
	}
}

func TestCodexInstallerDropsEmptyHookGroups(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
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

func TestCodexInstallerRejectsMalformedHookConfigWithoutWriting(t *testing.T) {
	for name, content := range map[string]string{
		"opaque hooks":          `{"hooks":"opaque"}`,
		"non-array known event": `{"hooks":{"SessionStart":{"owner":"third-party"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			writeAdapterTestSkill(t, root, "alpha")
			path := filepath.Join(home, ".codex", "hooks.json")
			writeFile(t, path, content)
			req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
			req.SkillNames = []string{"alpha"}

			result, err := NewInstaller().Install(req)
			if err == nil || result.OK {
				t.Fatalf("malformed hook config must fail without replacement: result=%+v err=%v", result, err)
			}
			gotBytes, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := string(gotBytes); got != content {
				t.Fatalf("malformed hook config was rewritten:\n got %q\nwant %q", got, content)
			}
		})
	}
}

func TestCodexInstallerReportsStaleHookTarget(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "install", true: "dry-run"}[dryRun], func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			writeAdapterTestSkill(t, root, "alpha")
			hooksPath := filepath.Join(home, ".codex", "hooks.json")
			writeFile(t, hooksPath, `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"'/source.worktrees/completed/bin/issueops' hook pre-tool-use --host codex"}]}]}}`)
			expected := filepath.Join(root, "bin", "issueops")
			req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), expected)
			req.SkillNames = []string{"alpha"}
			req.DryRun = dryRun

			result, err := NewInstaller().Install(req)
			if err != nil {
				t.Fatal(err)
			}
			want := "codex native hook target is stale: observed=/source.worktrees/completed/bin/issueops expected=" + expected + "; reinstall hooks and restart the codex session"
			if countMessage(result.Messages, want) != 1 {
				t.Fatalf("messages = %#v, want exactly one %q", result.Messages, want)
			}
		})
	}
}

func countMessage(messages []string, want string) int {
	count := 0
	for _, message := range messages {
		if message == want {
			count++
		}
	}
	return count
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
