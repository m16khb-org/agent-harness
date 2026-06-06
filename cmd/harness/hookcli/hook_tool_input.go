package hookcli

import (
	"strings"
)

func toolNameFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"tool_name", "tool", "name"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func commandFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"command", "cmd"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if toolInput, ok := obj["tool_input"].(map[string]any); ok {
		for _, key := range []string{"command", "cmd"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
		if command := mcpRemoteArtifactCommandFromHookObject(obj, toolInput); command != "" {
			return command
		}
		for _, key := range []string{"query", "pattern", "symbol", "text", "q"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func projectPathFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	if toolInput, ok := obj["tool_input"].(map[string]any); ok {
		for _, key := range []string{"projectPath", "project_path"} {
			if value, ok := toolInput[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	for _, key := range []string{"projectPath", "project_path"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
