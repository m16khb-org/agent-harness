package hookinput

import (
	"encoding/json"
	"strings"
)

func RepoFromHookInput(input []byte) string {
	if len(strings.TrimSpace(string(input))) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		for _, key := range []string{"repo", "cwd", "workspace", "workspace_root", "project_dir"} {
			if value, ok := nested[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func SourceFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	if value, ok := obj["source"].(string); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func CWDFromHookInput(input []byte) string {
	return hookString(input, "cwd")
}

func SessionIDFromHookInput(input []byte) string {
	return hookString(input, "session_id", "sessionId")
}

func AgentIDFromHookInput(input []byte) string {
	return hookString(input, "agent_id", "agentId", "agent_type", "agentType")
}

func HostFromHookInput(input []byte) string {
	return strings.ToLower(hookString(input, "host"))
}

func hookString(input []byte, keys ...string) string {
	obj := hookInputObject(input)
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Bool(input []byte, key string) bool {
	if v, ok := hookInputObject(input)[key].(bool); ok {
		return v
	}
	return false
}

func hookInputObject(input []byte) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err != nil {
		return map[string]any{}
	}
	if nested, ok := obj["hook_input"].(map[string]any); ok {
		for k, v := range nested {
			if _, exists := obj[k]; !exists {
				obj[k] = v
			}
		}
	}
	return obj
}
