// Package hook provides host-specific hook output adapters.
// Hosts (Codex, Claude, Reasonix) expect different JSON shapes for
// hook output. This package abstracts those differences behind a
// HostHookOutput interface so hook command implementations can emit
// host-compatible output without inline branch logic.
package hook

// Host identifies a hook host.
type Host string

const (
	HostCodex   Host = "codex"
	HostClaude  Host = "claude"
	HostReasonix Host = "reasonix"
)

// HostHookOutput formats hook output for a specific host.
type HostHookOutput interface {
	// FormatBlock returns a host-compatible JSON object for a blocking
	// ("block") decision in pre-tool-use hooks. reason explains the block.
	// Codex uses {"decision":"block","reason":"..."}; Claude uses
	// hookSpecificOutput with permissionDecision="deny".
	FormatBlock(reason string) map[string]any

	// FormatAsk returns a host-compatible JSON object for an "ask"
	// decision in pre-tool-use hooks. All hosts use hookSpecificOutput
	// with permissionDecision="ask" since no host has a native "ask" concept.
	FormatAsk(reason string) map[string]any

	// FormatContext returns a host-compatible JSON object for injecting
	// additional context during session-start, post-compact, or similar
	// hooks. eventName is the hook event (e.g. "SessionStart", "PostCompact").
	// additionalContext is the compact/Claude-formatted context string.
	// userView is the Codex-formatted context string (may be empty).
	FormatContext(eventName, additionalContext, userView string) map[string]any

	// FormatStopBlock returns a host-compatible JSON object for a stop-hook
	// block that continues the agent in-turn. reason explains the block.
	FormatStopBlock(reason string) map[string]any

	// FormatNoop returns the host-compatible no-op (empty) output.
	FormatNoop() map[string]any
}

// Resolve returns the HostHookOutput for the given host string.
// Empty or unknown hosts default to Codex-compatible output.
func Resolve(host string) HostHookOutput {
	switch Host(host) {
	case HostClaude:
		return ClaudeHookOutput{}
	case HostReasonix:
		return ReasonixHookOutput{}
	default:
		return CodexHookOutput{}
	}
}

// CodexHookOutput formats output compatible with Codex (OpenCode) hooks.
type CodexHookOutput struct{}

func (CodexHookOutput) FormatBlock(reason string) map[string]any {
	return map[string]any{
		"decision": "block",
		"reason":   reason,
	}
}

func (CodexHookOutput) FormatAsk(reason string) map[string]any {
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "ask",
			"permissionDecisionReason": reason,
		},
	}
}

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

func (CodexHookOutput) FormatStopBlock(reason string) map[string]any {
	return map[string]any{
		"continue": true,
		"decision": "block",
		"reason":   reason,
	}
}

func (CodexHookOutput) FormatNoop() map[string]any {
	return map[string]any{}
}

// ClaudeHookOutput formats output compatible with Claude Code hooks.
type ClaudeHookOutput struct{}

func (ClaudeHookOutput) FormatBlock(reason string) map[string]any {
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}
}

func (ClaudeHookOutput) FormatAsk(reason string) map[string]any {
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "ask",
			"permissionDecisionReason": reason,
		},
	}
}

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

func (ClaudeHookOutput) FormatStopBlock(reason string) map[string]any {
	return map[string]any{
		"continue": true,
		"decision": "block",
		"reason":   reason,
	}
}

func (ClaudeHookOutput) FormatNoop() map[string]any {
	return map[string]any{}
}

// ReasonixHookOutput formats output compatible with Reasonix hooks.
// It uses the same shapes as Claude for now.
type ReasonixHookOutput struct{}

func (ReasonixHookOutput) FormatBlock(reason string) map[string]any {
	return ClaudeHookOutput{}.FormatBlock(reason)
}

func (ReasonixHookOutput) FormatAsk(reason string) map[string]any {
	return ClaudeHookOutput{}.FormatAsk(reason)
}

func (ReasonixHookOutput) FormatContext(eventName, additionalContext, userView string) map[string]any {
	return ClaudeHookOutput{}.FormatContext(eventName, additionalContext, userView)
}

func (ReasonixHookOutput) FormatStopBlock(reason string) map[string]any {
	return ClaudeHookOutput{}.FormatStopBlock(reason)
}

func (ReasonixHookOutput) FormatNoop() map[string]any {
	return map[string]any{}
}
