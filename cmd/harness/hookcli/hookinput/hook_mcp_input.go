package hookinput

import (
	"sort"
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
	// PR/MR의 base·head는 IssueOps PR target guard가 검사하는 값이다. connector가
	// 표준 필드와 호환 별칭 어느 쪽으로 보내든 명령 표현에 보존해야 한다 —
	// 누락되면 올바른 대상 branch를 전달해도 base 없음으로 오탐 차단된다(#263).
	// branch 개념이 없는 issue에는 지어내지 않는다.
	if kind == "pr" || kind == "mr" {
		baseFlag, headFlag := "--base", "--head"
		if cli == "glab" {
			baseFlag, headFlag = "--target-branch", "--source-branch"
		}
		if base := firstStringValue(toolInput, "base", "base_branch", "baseBranch", "target_branch", "targetBranch"); base != "" {
			args = append(args, baseFlag, shellQuoteArg(base))
		}
		if head := firstStringValue(toolInput, "head", "head_branch", "headBranch", "source_branch", "sourceBranch"); head != "" {
			args = append(args, headFlag, shellQuoteArg(head))
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

func mcpGlabAPICommandFromHookObject(obj map[string]any, toolInput map[string]any) string {
	tool := strings.ToLower(toolNameFromHookObject(obj))
	if !strings.Contains(tool, "glab") || !strings.Contains(tool, "api") {
		return ""
	}
	toolInput = mergeMCPToolFlags(toolInput)
	endpoint := firstStringValue(toolInput, "endpoint", "path", "api_path", "apiPath", "resource")
	if endpoint == "" {
		return ""
	}
	args := []string{"glab", "api", shellQuoteArg(endpoint)}
	if method := firstStringValue(toolInput, "method", "http_method", "httpMethod"); method != "" {
		args = append(args, "-X", shellQuoteArg(method))
	}
	keys := make([]string, 0, len(toolInput))
	for key := range toolInput {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if skipGlabAPIFieldKey(key) {
			continue
		}
		for _, value := range stringListValue(toolInput, key) {
			args = append(args, "-f", shellQuoteArg(key+"="+value))
		}
		if boolValue(toolInput, key) {
			args = append(args, "-f", shellQuoteArg(key+"=true"))
		}
	}
	return strings.Join(args, " ")
}

func skipGlabAPIFieldKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return true
	}
	switch lower {
	case "endpoint", "path", "api_path", "apipath", "resource", "method", "http_method", "httpmethod", "flags":
		return true
	}
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "credential")
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
