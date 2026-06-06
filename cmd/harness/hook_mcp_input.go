package main

import (
	"strings"
)

func mcpRemoteArtifactCommandFromHookObject(obj map[string]any, toolInput map[string]any) string {
	tool := strings.ToLower(toolNameFromHookObject(obj))
	if tool == "" || (!strings.Contains(tool, "issue") && !strings.Contains(tool, "merge") && !strings.Contains(tool, "pull") && !strings.Contains(tool, "_mr") && !strings.Contains(tool, "_pr")) {
		return ""
	}
	toolInput = mergeMCPToolFlags(toolInput)
	cli := ""
	switch {
	case strings.Contains(tool, "gitlab") || strings.Contains(tool, "glab"):
		cli = "glab"
	case strings.Contains(tool, "github") || strings.Contains(tool, "gh"):
		cli = "gh"
	default:
		return ""
	}
	kind := ""
	switch {
	case strings.Contains(tool, "merge_request") || strings.Contains(tool, "merge-request") || strings.Contains(tool, "_mr") || strings.HasSuffix(tool, "mr"):
		kind = "mr"
	case strings.Contains(tool, "pull_request") || strings.Contains(tool, "pull-request") || strings.Contains(tool, "_pr") || strings.HasSuffix(tool, "pr"):
		kind = "pr"
	case strings.Contains(tool, "issue"):
		kind = "issue"
	default:
		return ""
	}
	action := ""
	switch {
	case strings.HasSuffix(tool, "_for") || strings.Contains(tool, "create_for") || strings.Contains(tool, "create-for"):
		action = "for"
	case strings.Contains(tool, "create") || strings.Contains(tool, "open"):
		action = "create"
	case strings.Contains(tool, "update"):
		action = "update"
	case strings.Contains(tool, "edit"):
		action = "edit"
	default:
		return ""
	}
	var args []string
	args = append(args, cli, kind, action)
	for _, positional := range stringListValue(toolInput, "args", "arguments") {
		args = append(args, shellQuoteArg(positional))
	}
	if action == "for" && len(stringListValue(toolInput, "args", "arguments")) == 0 {
		for _, issue := range stringListValue(toolInput, "issue", "issue_iid", "issueIid") {
			args = append(args, shellQuoteArg(issue))
		}
	}
	if title := firstStringValue(toolInput, "title", "name", "subject"); title != "" {
		args = append(args, "--title", shellQuoteArg(title))
	}
	if body := firstStringValue(toolInput, "body", "description", "content", "markdown"); body != "" {
		if cli == "glab" {
			args = append(args, "--description", shellQuoteArg(body))
		} else {
			args = append(args, "--body", shellQuoteArg(body))
		}
	}
	for _, label := range stringListValue(toolInput, "label", "labels", "add_label", "add_labels") {
		args = append(args, "--label", shellQuoteArg(label))
	}
	if boolValue(toolInput, "copy_issue_labels", "copyIssueLabels", "copy_labels", "copyLabels") {
		args = append(args, "--copy-issue-labels")
	}
	if boolValue(toolInput, "with_labels", "withLabels") {
		args = append(args, "--with-labels")
	}
	if relatedIssue := firstStringValue(toolInput, "related_issue", "relatedIssue", "issue", "issue_iid", "issueIid"); relatedIssue != "" && cli == "glab" && kind == "mr" && action != "for" {
		args = append(args, "--related-issue", shellQuoteArg(relatedIssue))
	}
	for _, assignee := range stringListValue(toolInput, "assignee", "assignees", "add_assignee", "add_assignees") {
		args = append(args, "--assignee", shellQuoteArg(assignee))
	}
	for _, assigneeID := range stringListValue(toolInput, "assignee_id", "assignee_ids", "assigneeId", "assigneeIds") {
		args = append(args, "--assignee-id", shellQuoteArg(assigneeID))
	}
	return strings.Join(args, " ")
}

func mergeMCPToolFlags(toolInput map[string]any) map[string]any {
	flags, ok := toolInput["flags"].(map[string]any)
	if !ok || len(flags) == 0 {
		return toolInput
	}
	merged := make(map[string]any, len(flags)+len(toolInput))
	for key, value := range flags {
		merged[key] = value
	}
	for key, value := range toolInput {
		if key != "flags" {
			merged[key] = value
		}
	}
	return merged
}
