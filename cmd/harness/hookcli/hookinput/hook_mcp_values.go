package hookinput

import (
	"strconv"
	"strings"
)

func toolNameFromHookObject(obj map[string]any) string {
	for _, key := range []string{"tool_name", "tool", "name"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := values[key].(bool); ok && value {
			return true
		}
	}
	return false
}

func stringListValue(values map[string]any, keys ...string) []string {
	var out []string
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			for _, part := range strings.Split(value, ",") {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, part)
				}
			}
		case []any:
			for _, item := range value {
				switch item := item.(type) {
				case string:
					if strings.TrimSpace(item) != "" {
						out = append(out, strings.TrimSpace(item))
					}
				case float64:
					out = append(out, strconv.FormatFloat(item, 'f', -1, 64))
				case int:
					out = append(out, strconv.Itoa(item))
				}
			}
		case []string:
			for _, item := range value {
				if strings.TrimSpace(item) != "" {
					out = append(out, strings.TrimSpace(item))
				}
			}
		case float64:
			out = append(out, strconv.FormatFloat(value, 'f', -1, 64))
		case int:
			out = append(out, strconv.Itoa(value))
		}
		if len(out) > 0 {
			return out
		}
	}
	return out
}

func shellQuoteArg(value string) string {
	return strconv.Quote(value)
}
