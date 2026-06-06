package issueops

import "strings"

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsFold(s, needle string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(needle))
}
