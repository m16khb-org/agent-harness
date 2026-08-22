package mcp

// ChannelTools는 세션 간 메시지 채널 도구군이다. CLI channel 명령과 같은
// contract DTO를 공유한다.
func ChannelTools() []Tool {
	return []Tool{
		{
			Name:        "channel_send",
			Description: "Append a durable message to a named session-to-session channel backed by the shared harness state. Codex, Claude Code, and Omo sessions sharing the state see it. Use for cross-session coordination such as front/server contract negotiation.",
			InputSchema: map[string]any{"type": "object", "required": []string{"channel", "from", "body"}, "properties": map[string]any{
				"channel": map[string]any{"type": "string", "description": "Channel name (shared mailbox key), for example contract or front-server."},
				"from":    map[string]any{"type": "string", "description": "Sending session identity, for example server, front, or codex:abc."},
				"body":    map[string]any{"type": "string", "description": "Message body."},
			}},
		},
		{
			Name:        "channel_recv",
			Description: "Read messages from a named channel after a given message id (exclusive). With wait=true, blocks until a new message arrives or the timeout (default 300s) — the receiving side of a front/server session coordination flow.",
			InputSchema: map[string]any{"type": "object", "required": []string{"channel"}, "properties": map[string]any{
				"channel":         map[string]any{"type": "string", "description": "Channel name."},
				"since_id":        map[string]any{"type": "string", "description": "Return messages after this message id (exclusive). Omit to read from the beginning."},
				"wait":            map[string]any{"type": "boolean", "description": "Block until a new message arrives or timeout. Default false."},
				"timeout_seconds": map[string]any{"type": "integer", "description": "Wait timeout in seconds when wait=true. Default 300."},
				"limit":           map[string]any{"type": "integer", "description": "Maximum messages to return. 0 returns all."},
			}},
		},
	}
}
