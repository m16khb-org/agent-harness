package main

import (
	"encoding/json"
	"os"
	"strings"
)

func lastAssistantMessageFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"last_assistant_message", "lastAssistantMessage", "assistant_message", "assistantMessage", "response", "final_response"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func transcriptPathFromHookInput(input []byte) string {
	obj := hookInputObject(input)
	for _, key := range []string{"transcript_path", "transcriptPath"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readLastAssistantMessageFromTranscript(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(strings.ToLower(line), "assistant") {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if text := assistantTextFromTranscriptObject(obj); text != "" {
			return text
		}
	}
	return ""
}

func assistantTextFromTranscriptObject(value any) string {
	switch v := value.(type) {
	case map[string]any:
		role := ""
		if r, ok := v["role"].(string); ok {
			role = strings.ToLower(strings.TrimSpace(r))
		}
		if msg, ok := v["message"].(map[string]any); ok {
			if r, ok := msg["role"].(string); ok && role == "" {
				role = strings.ToLower(strings.TrimSpace(r))
			}
		}
		if typ, ok := v["type"].(string); ok && role == "" {
			role = strings.ToLower(strings.TrimSpace(typ))
		}
		if role != "" && role != "assistant" {
			return ""
		}
		for _, key := range []string{"last_assistant_message", "text", "content", "message"} {
			if text := transcriptTextValue(v[key]); text != "" {
				return text
			}
		}
	case []any:
		return transcriptTextValue(v)
	}
	return ""
}

func transcriptTextValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		var parts []string
		for _, item := range v {
			if text := transcriptTextValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		if typ, ok := v["type"].(string); ok && strings.EqualFold(strings.TrimSpace(typ), "tool_use") {
			return ""
		}
		for _, key := range []string{"text", "content"} {
			if text := transcriptTextValue(v[key]); text != "" {
				return text
			}
		}
	}
	return ""
}
