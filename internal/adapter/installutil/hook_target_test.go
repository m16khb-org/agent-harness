package installutil

import (
	"reflect"
	"testing"
)

func TestHookTargetDriftMessages(t *testing.T) {
	expected := "/source/bin/agent-harness"
	tests := []struct {
		name   string
		config map[string]any
		want   []string
	}{
		{
			name: "completed worktree target",
			config: hookTargetTestConfig(
				"'/source.worktrees/completed/bin/agent-harness' hook pre-tool-use --host codex",
			),
			want: []string{"codex native hook target is stale: observed=/source.worktrees/completed/bin/agent-harness expected=/source/bin/agent-harness; reinstall hooks and restart the codex session"},
		},
		{
			name:   "relative legacy target",
			config: hookTargetTestConfig("./bin/agent-harness hook pre-tool-use --host codex"),
			want:   []string{"codex native hook target is stale: observed=./bin/agent-harness expected=/source/bin/agent-harness; reinstall hooks and restart the codex session"},
		},
		{
			name:   "canonical target",
			config: hookTargetTestConfig("'/source/bin/agent-harness' hook pre-tool-use --host codex"),
			want:   nil,
		},
		{
			name: "duplicate stale target is reported once",
			config: map[string]any{"hooks": map[string]any{
				"PreToolUse":  hookTargetTestConfig("'/old/bin/agent-harness' hook pre-tool-use")["hooks"].(map[string]any)["PreToolUse"],
				"PostToolUse": hookTargetTestConfig("'/old/bin/agent-harness' hook post-tool-use")["hooks"].(map[string]any)["PreToolUse"],
			}},
			want: []string{"codex native hook target is stale: observed=/old/bin/agent-harness expected=/source/bin/agent-harness; reinstall hooks and restart the codex session"},
		},
		{
			name: "third party and malformed groups are ignored",
			config: map[string]any{"hooks": map[string]any{"PreToolUse": []any{
				map[string]any{"hooks": []any{map[string]any{"command": "/bin/sh /Users/example/.orca/hook.sh"}}},
				map[string]any{"hooks": "malformed"},
				"malformed",
			}}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HookTargetDriftMessages(tt.config, "codex", expected)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("messages = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestHookCommandTargetDecodesQuotedTargetContainingHookAndEscapedQuote(t *testing.T) {
	want := "/source thin hook workspace/it's/bin/agent-harness"
	command := "'/source thin hook workspace/it'\"'\"'s/bin/agent-harness' hook session-start --host codex"
	got, ok := hookCommandTarget(command)
	if !ok || got != want {
		t.Fatalf("hookCommandTarget(%q) = %q, %t; want %q, true", command, got, ok, want)
	}
}

func hookTargetTestConfig(command string) map[string]any {
	return map[string]any{"hooks": map[string]any{"PreToolUse": []any{
		map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}},
	}}}
}
