// Package hook provides host-specific hook output adapters. Codex and Claude
// Code expect different JSON shapes for context injection, so the context
// hooks emit through a HostHookOutput instead of inline branch logic.
package hook

// HostHookOutput formats context-hook output for a specific host.
type HostHookOutput interface {
	// FormatContext returns a host-compatible JSON object for injecting
	// additional context during session-start or similar hooks. eventName is
	// the hook event (for example "SessionStart"). additionalContext is the
	// compact, model-facing string; userView is the readable string (may be
	// empty).
	FormatContext(eventName, additionalContext, userView string) map[string]any

	// FormatNoop returns the host-compatible no-op (empty) output.
	FormatNoop() map[string]any
}

// CodexHookOutput formats output compatible with Codex hooks. Codex renders
// additionalContext in its TUI, so the readable user view replaces the compact
// string when present and systemMessage is never emitted.
type CodexHookOutput struct{}

func (CodexHookOutput) FormatContext(eventName, additionalContext, userView string) map[string]any {
	ctx := additionalContext
	if userView != "" {
		ctx = userView
	}
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": ctx,
		},
	}
}

func (CodexHookOutput) FormatNoop() map[string]any {
	return map[string]any{}
}

// ClaudeHookOutput formats output compatible with Claude Code hooks. Claude
// keeps additionalContext model-facing and shows systemMessage to the user.
type ClaudeHookOutput struct{}

func (ClaudeHookOutput) FormatContext(eventName, additionalContext, userView string) map[string]any {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": additionalContext,
		},
	}
	if userView != "" {
		payload["systemMessage"] = userView
	}
	return payload
}

func (ClaudeHookOutput) FormatNoop() map[string]any {
	return map[string]any{}
}
