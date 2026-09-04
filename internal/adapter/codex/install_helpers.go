package codex

import "strings"

func shellQuote(value string) string {
	if value == "" {
		return "issueops"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
