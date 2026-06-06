package core

import "agent-harness/internal/core/hookprompt"

type HookUserPromptRequest = hookprompt.HookUserPromptRequest
type HookUserPromptHint = hookprompt.HookUserPromptHint
type HookUserPromptResult = hookprompt.HookUserPromptResult
type HookRoutingRule = hookprompt.HookRoutingRule
type ProjectDocCatalogContext = hookprompt.ProjectDocCatalogContext

const (
	hintPriorityRequired  = hookprompt.PriorityRequired
	hintPriorityConsider  = hookprompt.PriorityConsider
	hintPriorityRoute     = hookprompt.PriorityRoute
	hintPriorityAction    = hookprompt.PriorityAction
	hintPrioritySecondary = hookprompt.PrioritySecondary
)

func BuildUserPromptMCPHints(req HookUserPromptRequest) HookUserPromptResult {
	return hookprompt.BuildUserPromptMCPHints(req)
}

func BuildProjectDocCatalogContext(repo string) ProjectDocCatalogContext {
	return hookprompt.BuildProjectDocCatalogContext(repo)
}

func RenderUserPromptUserView(result HookUserPromptResult) string {
	return hookprompt.RenderUserPromptUserView(result)
}

func RenderUserPromptCodexContext(result HookUserPromptResult) string {
	return hookprompt.RenderUserPromptCodexContext(result)
}

func renderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []DocUpkeepEvent, profile *ProjectProfile, catalog string) string {
	return hookprompt.RenderHookMCPHintContext(hints, pendingUpkeep, profile, catalog)
}

func appendCompactPendingUpkeep(parts *[]string, events []DocUpkeepEvent) {
	hookprompt.AppendCompactPendingUpkeep(parts, events)
}

func fallbackHintPriority(h HookUserPromptHint) string {
	return hookprompt.FallbackHintPriority(h)
}

func compactHintLabel(h HookUserPromptHint) string {
	return hookprompt.CompactHintLabel(h)
}

func containsAnySlice(s string, needles []string) bool {
	return hookprompt.ContainsAnySlice(s, needles)
}

func containsAny(s string, needles ...string) bool {
	return hookprompt.ContainsAny(s, needles...)
}
