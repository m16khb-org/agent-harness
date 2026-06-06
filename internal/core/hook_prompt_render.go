package core

import "strings"

func renderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []DocUpkeepEvent, profile *ProjectProfile, catalog string) string {
	groups := map[string][]HookUserPromptHint{}
	for _, h := range hints {
		priority := h.Priority
		if priority == "" {
			priority = fallbackHintPriority(h)
		}
		groups[priority] = append(groups[priority], h)
	}

	parts := []string{"[agent-harness] routing hint"}
	if catalog != "" {
		parts = append(parts, catalog)
	}
	appendCompactHintGroup(&parts, "docs", groups[hintPriorityRoute])
	appendCompactHintGroup(&parts, "actions", groups[hintPriorityAction])
	appendCompactProjectProfile(&parts, profile)
	appendCompactPendingUpkeep(&parts, pendingUpkeep)
	appendSecondaryHints(&parts, groups[hintPrioritySecondary])
	parts = append(parts, nextActionPolicyHint)
	parts = append(parts, draftWikiPolicyHint)
	parts = append(parts, "rule: verify with repo/tool evidence before changing files")
	return strings.Join(parts, " | ")
}

func fallbackHintPriority(h HookUserPromptHint) string {
	switch {
	case strings.HasSuffix(h.Tool, ".md"):
		return hintPriorityConsider
	case h.Tool == "CodeGraph" || h.Tool == "LLM Wiki" || h.Tool == "claude-mem" || h.Tool == "agy -p":
		return hintPrioritySecondary
	case h.Tool == "project_docs_route":
		return hintPriorityRoute
	default:
		return hintPriorityAction
	}
}

func appendCompactHintGroup(parts *[]string, title string, hints []HookUserPromptHint) {
	if len(hints) == 0 {
		return
	}
	labels := make([]string, 0, len(hints))
	seen := map[string]bool{}
	for _, h := range hints {
		label := compactHintLabel(h)
		if seen[label] {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return
	}
	*parts = append(*parts, title+": "+strings.Join(labels, ", "))
}

func appendSecondaryHints(parts *[]string, hints []HookUserPromptHint) {
	if len(hints) == 1 && hints[0].Tool == "claude-mem" {
		*parts = append(*parts, "memory: use claude-mem only for previous-session/repeated-work recall")
		return
	}
	appendCompactHintGroup(parts, "secondary", hints)
}

func compactHintLabel(h HookUserPromptHint) string {
	switch h.Tool {
	case "project_docs_route":
		return "use project docs only when repo-specific context matters"
	case "project_docs_read/project_docs_update":
		return "refresh project docs only if evidence changed"
	case "project_docs_record":
		if strings.Contains(h.Reason, "kind=caution") {
			return "record reusable caution"
		}
		if strings.Contains(h.Reason, "kind=adr") {
			return "record ADR decision"
		}
		return "record durable project note"
	case "api_doc_static_check":
		return "check OpenAPI gaps"
	case "api_doc_review":
		return "review API error contract"
	case "issueops":
		return "issueops issue-driven workflow; hooks must not create issues or PRs"
	case "vcs_remote_auth":
		return "VCS remote work: use authenticated CLI first; on missing token or auth/permission error use MCP fallback; do not print tokens"
	case "CodeGraph":
		return "CodeGraph for structural lookup; rg for exact strings"
	case "LLM Wiki":
		return "LLM Wiki for explicit wiki/research work"
	case "claude-mem":
		return "claude-mem only for previous-session/repeated-work recall"
	case "agy -p":
		return "agy -p for LLM second-pass review"
	default:
		return h.Tool
	}
}

func appendCompactPendingUpkeep(parts *[]string, events []DocUpkeepEvent) {
	if len(events) == 0 {
		return
	}
	items := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		item := event.Summary
		if len(event.TargetDocs) > 0 {
			item = strings.Join(event.TargetDocs, ",") + " " + item
		}
		if seen[item] {
			continue
		}
		seen[item] = true
		items = append(items, item)
	}
	if len(items) == 0 {
		return
	}
	*parts = append(*parts, "pending upkeep: "+strings.Join(items, "; "))
}

func appendCompactProjectProfile(parts *[]string, profile *ProjectProfile) {
	if profile == nil {
		return
	}
	items := []string{}
	if profile.VCS.Provider != "" && profile.VCS.Provider != "none" {
		vcs := profile.VCS.Provider
		if profile.VCS.Hosting != "" && profile.VCS.Hosting != "unknown" {
			vcs += "/" + profile.VCS.Hosting
		}
		if profile.VCS.RemoteHost != "" {
			vcs += "@" + profile.VCS.RemoteHost
		}
		items = append(items, vcs)
	}
	if len(profile.Languages) > 0 {
		items = append(items, strings.Join(profile.Languages, "+"))
	}
	if len(profile.ProjectTypes) > 0 {
		items = append(items, strings.Join(profile.ProjectTypes, "+"))
	}
	if len(profile.Frameworks) > 0 {
		frameworks := profile.Frameworks
		if len(frameworks) > 4 {
			frameworks = frameworks[:4]
		}
		items = append(items, strings.Join(frameworks, "+"))
	}
	if len(items) == 0 {
		return
	}
	*parts = append(*parts, "profile: "+strings.Join(items, ", "))
}
