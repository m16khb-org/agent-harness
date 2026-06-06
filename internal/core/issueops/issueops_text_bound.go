package issueops

import "strings"

func boundedIssueOpsText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "...[truncated]"
	}
	return s
}
