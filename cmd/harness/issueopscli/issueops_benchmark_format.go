package issueopscli

import (
	"fmt"
	"strings"
)

func issueOpsBenchmarkBullets(items []string) string {
	if len(items) == 0 {
		return "- 해당 fixture의 추가 요구사항 없음"
	}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, "- "+item)
	}
	if len(out) == 0 {
		return "- 해당 fixture의 추가 요구사항 없음"
	}
	return strings.Join(out, "\n")
}

func issueOpsBenchmarkOwnedTasks(items []string) string {
	if len(items) == 0 {
		return "- Worker Fixture owns verification that this fixture has no additional task requirements."
	}
	var out []string
	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, fmt.Sprintf("- Worker Fixture-%d owns %s and reports test evidence for that task.", i+1, item))
	}
	if len(out) == 0 {
		return "- Worker Fixture owns verification that this fixture has no additional task requirements."
	}
	return strings.Join(out, "\n")
}
