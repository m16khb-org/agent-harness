package argmap

import (
	"strconv"
	"strings"
)

func String(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func Set(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	_, ok := args[key]
	return ok
}

func StringDefault(args map[string]any, key, fallback string) string {
	if v := String(args, key); v != "" {
		return v
	}
	return fallback
}

func StringSlice(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return append([]string{}, v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(v)
	default:
		return nil
	}
}

func Bool(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	default:
		return false
	}
}

func Int(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func Int64(args map[string]any, key string, fallback int64) int64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func Float(args map[string]any, key string, fallback float64) float64 {
	if args == nil {
		return fallback
	}
	switch v := args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
