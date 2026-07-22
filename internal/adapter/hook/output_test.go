package hook

import (
	"testing"
)

func TestCodexFormatBlock(t *testing.T) {
	out := CodexHookOutput{}.FormatBlock("test reason")
	if out["decision"] != "block" {
		t.Errorf("expected decision=block, got %v", out["decision"])
	}
	if out["reason"] != "test reason" {
		t.Errorf("expected reason, got %v", out["reason"])
	}
}

func TestClaudeFormatBlock(t *testing.T) {
	out := ClaudeHookOutput{}.FormatBlock("test reason")
	hso, ok := out["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput")
	}
	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("expected PreToolUse, got %v", hso["hookEventName"])
	}
	if hso["permissionDecision"] != "deny" {
		t.Errorf("expected deny, got %v", hso["permissionDecision"])
	}
	if hso["permissionDecisionReason"] != "test reason" {
		t.Errorf("expected reason, got %v", hso["permissionDecisionReason"])
	}
}

func TestCodexFormatContextWithUserView(t *testing.T) {
	out := CodexHookOutput{}.FormatContext("SessionStart", "ctx", "user view")
	hso, ok := out["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput")
	}
	if hso["additionalContext"] != "user view" {
		t.Errorf("expected additionalContext=user view, got %v", hso["additionalContext"])
	}
	if _, ok := out["systemMessage"]; ok {
		t.Errorf("Codex must not emit systemMessage in FormatContext")
	}
}

func TestCodexFormatContextEmptyUserView(t *testing.T) {
	out := CodexHookOutput{}.FormatContext("SessionStart", "ctx", "")
	hso, ok := out["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput")
	}
	if hso["hookEventName"] != "SessionStart" {
		t.Errorf("expected hookEventName, got %v", hso["hookEventName"])
	}
	if hso["additionalContext"] != "ctx" {
		t.Errorf("expected additionalContext=ctx, got %v", hso["additionalContext"])
	}
}

func TestClaudeFormatContext(t *testing.T) {
	out := ClaudeHookOutput{}.FormatContext("SessionStart", "additional ctx", "user view")
	hso, ok := out["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput")
	}
	if hso["hookEventName"] != "SessionStart" {
		t.Errorf("expected SessionStart, got %v", hso["hookEventName"])
	}
	if hso["additionalContext"] != "additional ctx" {
		t.Errorf("expected additional ctx, got %v", hso["additionalContext"])
	}
	if out["systemMessage"] != "user view" {
		t.Errorf("expected systemMessage, got %v", out["systemMessage"])
	}
}

func TestClaudeFormatContextEmptyUserView(t *testing.T) {
	out := ClaudeHookOutput{}.FormatContext("SessionStart", "additional ctx", "")
	if _, ok := out["systemMessage"]; ok {
		t.Errorf("expected no systemMessage when userView is empty")
	}
}

func TestResolveKnownHosts(t *testing.T) {
	if _, ok := Resolve("codex").(CodexHookOutput); !ok {
		t.Error("expected CodexHookOutput for codex")
	}
	if _, ok := Resolve("claude").(ClaudeHookOutput); !ok {
		t.Error("expected ClaudeHookOutput for claude")
	}
}

func TestResolveUnknownDefaultsToCodex(t *testing.T) {
	if _, ok := Resolve("unknown").(CodexHookOutput); !ok {
		t.Error("expected CodexHookOutput for unknown host")
	}
	if _, ok := Resolve("").(CodexHookOutput); !ok {
		t.Error("expected CodexHookOutput for empty host")
	}
}

func TestFormatStopBlock(t *testing.T) {
	for _, h := range []HostHookOutput{CodexHookOutput{}, ClaudeHookOutput{}} {
		out := h.FormatStopBlock("reason")
		if out["continue"] != true {
			t.Errorf("%T: expected continue=true", h)
		}
		if out["decision"] != "block" {
			t.Errorf("%T: expected decision=block", h)
		}
		if out["reason"] != "reason" {
			t.Errorf("%T: expected reason", h)
		}
	}
}

func TestFormatNoop(t *testing.T) {
	for _, h := range []HostHookOutput{CodexHookOutput{}, ClaudeHookOutput{}} {
		out := h.FormatNoop()
		if len(out) != 0 {
			t.Errorf("%T: expected empty noop, got %+v", h, out)
		}
	}
}

func TestCodexFormatAskFallsBackToBlock(t *testing.T) {
	out := CodexHookOutput{}.FormatAsk("please confirm")
	if out["decision"] != "block" {
		t.Fatalf("Codex ask fallback must use block decision, got %+v", out)
	}
	if out["reason"] != "please confirm" {
		t.Fatalf("expected reason, got %+v", out)
	}
	if _, ok := out["hookSpecificOutput"]; ok {
		t.Fatalf("Codex ask fallback must not emit unsupported hookSpecificOutput, got %+v", out)
	}
}

func TestFormatAskForHostsWithNativeAsk(t *testing.T) {
	for _, h := range []HostHookOutput{ClaudeHookOutput{}} {
		out := h.FormatAsk("please confirm")
		hso := out["hookSpecificOutput"].(map[string]any)
		if hso["permissionDecision"] != "ask" {
			t.Errorf("%T: expected ask, got %v", h, hso["permissionDecision"])
		}
		if hso["permissionDecisionReason"] != "please confirm" {
			t.Errorf("%T: expected reason, got %v", h, hso["permissionDecisionReason"])
		}
	}
}
