package hook

import "testing"

func TestCodexFormatContextPrefersUserView(t *testing.T) {
	out := CodexHookOutput{}.FormatContext("SessionStart", "compact", "readable")
	hso := out["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "SessionStart" || hso["additionalContext"] != "readable" {
		t.Fatalf("codex context = %#v", out)
	}
	if _, ok := out["systemMessage"]; ok {
		t.Fatalf("codex must not emit systemMessage: %#v", out)
	}
	fallback := CodexHookOutput{}.FormatContext("SessionStart", "compact", "")
	if got := fallback["hookSpecificOutput"].(map[string]any)["additionalContext"]; got != "compact" {
		t.Fatalf("codex empty user view must fall back to compact: %v", got)
	}
}

func TestClaudeFormatContextSplitsModelAndUserChannels(t *testing.T) {
	out := ClaudeHookOutput{}.FormatContext("SessionStart", "compact", "readable")
	hso := out["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "SessionStart" || hso["additionalContext"] != "compact" || out["systemMessage"] != "readable" {
		t.Fatalf("claude context = %#v", out)
	}
	silent := ClaudeHookOutput{}.FormatContext("SessionStart", "compact", "")
	if _, ok := silent["systemMessage"]; ok {
		t.Fatal("claude must omit systemMessage when the user view is empty")
	}
}

func TestFormatNoopIsEmptyForEveryHost(t *testing.T) {
	for _, ho := range []HostHookOutput{CodexHookOutput{}, ClaudeHookOutput{}} {
		if len(ho.FormatNoop()) != 0 {
			t.Fatalf("noop must be empty: %#v", ho.FormatNoop())
		}
	}
}
