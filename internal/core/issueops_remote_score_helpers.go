package core

import (
	"sort"
	"strings"
)

func issueOpsRemoteApplyInstructions(provider string, issues, labels []IssueOpsRemoteScoredItem) []string {
	var instructions []string
	if len(issues) > 0 {
		switch provider {
		case "github":
			instructions = append(instructions, "include selected related issues in the issue body; use GitHub issue references such as #123 or full URLs")
		case "gitlab":
			instructions = append(instructions, "attach selected related issues as GitLab linked items, not a body section; create each link with glab api projects/:id/issues/:iid/links -X POST -f target_project_id=... -f target_issue_iid=... -f link_type=relates_to")
		}
	}
	if len(labels) > 0 {
		names := []string{}
		for _, label := range labels {
			names = append(names, label.Name)
		}
		switch provider {
		case "github":
			instructions = append(instructions, "apply selected labels with gh issue create --label or gh issue edit --add-label: "+strings.Join(names, ","))
		case "gitlab":
			instructions = append(instructions, "apply selected labels with the GitLab issue labels field or glab issue create --label: "+strings.Join(names, ","))
		}
	}
	return instructions
}

func issueOpsRemoteIssueApplyHint(provider string, item IssueOpsRemoteScoredItem) string {
	if provider == "gitlab" {
		if ref := firstNonEmpty(item.URL, item.ID); ref != "" {
			return "attach as GitLab linked item via issue links API: " + ref
		}
		return "attach selected related issue as a GitLab linked item via the issue links API"
	}
	if item.URL != "" {
		return "link in issue body: " + item.URL
	}
	if item.ID != "" {
		return "link in issue body: " + item.ID
	}
	return "link selected related issue in issue body"
}

func issueOpsRemoteLabelApplyHint(provider, name string) string {
	if provider == "gitlab" {
		return "apply GitLab label: " + name
	}
	return "apply GitHub label: " + name
}

func sortIssueOpsRemoteScoredItems(items []IssueOpsRemoteScoredItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return firstNonEmpty(items[i].ID, items[i].Name, items[i].Title) < firstNonEmpty(items[j].ID, items[j].Name, items[j].Title)
		}
		return items[i].Score > items[j].Score
	})
}

func scorePtr(score *float64) float64 {
	if score == nil {
		return 0
	}
	return clampScore(*score)
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
