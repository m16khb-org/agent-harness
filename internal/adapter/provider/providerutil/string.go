package providerutil

import "strings"

func MissingStrings(want, got []string) []string {
	gotSet := make(map[string]bool, len(got))
	for _, value := range got {
		gotSet[strings.TrimSpace(value)] = true
	}
	var missing []string
	for _, value := range want {
		value = strings.TrimSpace(value)
		if value != "" && !gotSet[value] {
			missing = append(missing, value)
		}
	}
	return missing
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
