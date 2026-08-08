package draftmeta

import "strings"

func ParseFrontmatter(content string) map[string]string {
	meta := map[string]string{}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			return meta
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "title", "source", "target_wiki", "target_type", "summary":
			meta[key] = value
		}
	}
	return meta
}
