package hookprompt

import (
	"strings"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func renderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []lifecyclecontract.DocUpkeepEvent, profile *ProjectProfile, catalog string) string {
	groups := map[string][]HookUserPromptHint{}
	for _, h := range hints {
		priority := h.Priority
		if priority == "" {
			priority = fallbackHintPriority(h)
		}
		groups[priority] = append(groups[priority], h)
	}

	parts := []string{"[agent-harness]"}
	if catalog != "" {
		appendContextLine(&parts, "catalog", catalog)
	}
	appendCompactHintGroup(&parts, "required", groups[hintPriorityRequired])
	appendCompactHintGroup(&parts, "docs", groups[hintPriorityRoute])
	appendCompactHintGroup(&parts, "actions", groups[hintPriorityAction])
	appendCompactProjectProfile(&parts, profile)
	appendCompactPendingUpkeep(&parts, pendingUpkeep)
	appendSecondaryHints(&parts, groups[hintPrioritySecondary])
	appendContextLine(&parts, "next-action", strings.TrimPrefix(nextActionPolicyHint, "next-action: "))
	appendContextLine(&parts, "rule", "verify with repo/tool evidence before changing files")
	return strings.Join(parts, "\n")
}

func appendCompactWorktreeReminder(parts *[]string, repo string) {
	appendContextLine(parts, "worktree", activeWorktreeReminderValue(repo))
	for _, line := range strings.Split(orchestrationReminderValue(repo), "\n") {
		appendContextLine(parts, "orchestration", line)
	}
}

func RenderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []lifecyclecontract.DocUpkeepEvent, profile *ProjectProfile, catalog string) string {
	return renderHookMCPHintContext(hints, pendingUpkeep, profile, catalog)
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
	appendContextLine(parts, title, strings.Join(labels, ", "))
}

func appendSecondaryHints(parts *[]string, hints []HookUserPromptHint) {
	appendCompactHintGroup(parts, "secondary", hints)
}

func appendCompactPendingUpkeep(parts *[]string, events []lifecyclecontract.DocUpkeepEvent) {
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
	appendContextLine(parts, "pending upkeep", strings.Join(items, "; "))
}

func AppendCompactPendingUpkeep(parts *[]string, events []lifecyclecontract.DocUpkeepEvent) {
	appendCompactPendingUpkeep(parts, events)
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
	appendContextLine(parts, "profile", strings.Join(items, ", "))
}

func appendContextLine(parts *[]string, title, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*parts = append(*parts, "- "+title+": "+value)
}
