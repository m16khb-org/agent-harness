package codex

import "strings"

func shellQuote(value string) string {
	if value == "" {
		return "harness"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
