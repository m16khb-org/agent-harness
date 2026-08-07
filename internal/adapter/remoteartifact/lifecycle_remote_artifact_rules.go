package remoteartifact

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	planLinkHeadingRe             = regexp.MustCompile(`(?mi)^\s*(?:#{1,6}\s*)?(Plan Link|Plan link|계획\s*링크)\s*:?\s*$`)
	relatedHeadingRe              = regexp.MustCompile(`(?mi)^\s*(?:#{1,6}\s*)?(Related Issues|Related issues|관련\s*이슈)\s*:?\s*$`)
	childHierarchyLinkCommandRe   = regexp.MustCompile(`(?is)(?:^|\s)(?:\S*/)?agent-harness\s+issueops\s+link-related\b|(?:^|\s)glab\s+issue\s+link\b|(?:^|\s)glab\s+api\b.*\bissues/[^ \t\n]+/links\b`)
	childHierarchyIntentKeywordRe = regexp.MustCompile(`(?i)\b(child|children|sub[- ]?issue|sub[- ]?task|umbrella|parent)\b|하위|자식|상위|부모|우산`)
)

func VCSIssueLinkingBlockReason(tool, command, repo string) string {
	if !remoteArtifactGateAppliesToTool(tool) {
		return ""
	}
	if reason := childHierarchyLinkedIssueBlockReason(command); reason != "" {
		return reason
	}
	artifact, ok := parseGHRemoteArtifactCommand(command, repo)
	if !ok {
		return ""
	}
	body := artifact.body
	if strings.TrimSpace(body) != "" {
		if planLinkHeadingRe.MatchString(body) {
			return fmt.Sprintf("IssueOps issue body must not contain a Plan Link section before %s %s %s; plan tracking lives in issueops link-plan state and the PR/MR body, not the issue body", artifact.provider, artifact.kind, artifact.action)
		}
		if artifact.provider == "gitlab" && (artifact.kind == "issue") && relatedHeadingRe.MatchString(body) {
			return fmt.Sprintf("GitLab related issues must be attached as native linked items, not a body Related Issues section, before glab %s %s; use glab api projects/:id/issues/:iid/links with link_type=relates_to", artifact.kind, artifact.action)
		}
	}
	if artifact.action == "create" && len(artifact.labels) == 0 {
		return fmt.Sprintf("IssueOps remote %s create must include labels before %s %s create; copy the linked issue labels or pass an explicit manual label flag", artifact.kind, artifact.provider, artifact.kind)
	}
	if artifact.action == "create" && len(artifact.assignees) == 0 {
		return fmt.Sprintf("IssueOps remote %s create must include an assignee before %s %s create; assign the artifact to the currently authenticated user and verify the remote assignee list before reporting readiness", artifact.kind, artifact.provider, artifact.kind)
	}
	if artifact.action == "create" {
		if reason := remoteAssigneePlaceholderBlockReason(artifact); reason != "" {
			return reason
		}
	}
	if artifact.action == "create" && artifact.provider == "gitlab" {
		if reason := gitLabRemoteAssigneeBlockReason(artifact); reason != "" {
			return reason
		}
	}
	return ""
}

func childHierarchyLinkedIssueBlockReason(command string) string {
	if !childHierarchyLinkCommandRe.MatchString(command) || !childHierarchyIntentKeywordRe.MatchString(command) {
		return ""
	}
	return "GitLab linked items and child items are different: the issue links API (`/issues/:iid/links`, `relates_to`, `blocks`, `is_blocked_by`) is only for related/non-hierarchical links. IssueOps child-task breakdown must use `agent-harness issueops remote create-child` for new provider-native child Tasks, or `agent-harness issueops link-child` only for an already verified child item"
}

func remoteAssigneePlaceholderBlockReason(artifact remoteArtifactCommand) string {
	for _, assignee := range artifact.assignees {
		value := strings.TrimSpace(strings.ToLower(assignee))
		switch value {
		case "@me", "me", "self", "current_user", "current-user", "currentuser":
			return fmt.Sprintf("IssueOps %s remote create must use a concrete current-user username or numeric assignee id, not a placeholder such as @me; resolve the current provider user first, then verify the remote assignee list", artifact.provider)
		}
	}
	return ""
}

func gitLabRemoteAssigneeBlockReason(artifact remoteArtifactCommand) string {
	for _, assignee := range artifact.assignees {
		value := strings.TrimSpace(strings.ToLower(assignee))
		if artifact.kind == "mr" && artifact.createFromIssue && !allDigits(value) {
			return "IssueOps GitLab issue-based MR create (`glab mr for`) must pass numeric assignee IDs because this glab command does not accept usernames; resolve the current user id with glab api user --jq .id and verify the remote assignee list"
		}
	}
	return ""
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
