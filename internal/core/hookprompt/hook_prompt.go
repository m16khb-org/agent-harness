package hookprompt

import (
	"strings"
	"time"
)

type HookUserPromptRequest struct {
	Prompt         string `json:"prompt"`
	Repo           string `json:"repo,omitempty"`
	EnableLLMHints bool   `json:"enable_llm_hints,omitempty"`
}

const (
	hintPriorityRequired  = PriorityRequired
	hintPriorityConsider  = PriorityConsider
	hintPriorityRoute     = PriorityRoute
	hintPriorityAction    = PriorityAction
	hintPrioritySecondary = PrioritySecondary
)

// UserPromptSubmit appears in every prompt transcript, so keep reminders short.
// The Stop hook relay carries the full auto/no-auto decision contract when it is
// actually needed.
const nextActionPolicyHint = "next-action: decision turns need 3 choices/1 recommendation; recommend only safe/reversible/aligned options; Stop hook relays full decision details"
const draftWikiPolicyHint = "draft-wiki: queue reusable project knowledge by main-agent judgement only"

type HookUserPromptResult struct {
	OK                bool                     `json:"ok"`
	Kind              string                   `json:"kind"`
	GeneratedAt       string                   `json:"generated_at"`
	ShouldInject      bool                     `json:"should_inject"`
	AdditionalContext string                   `json:"additional_context,omitempty"`
	Hints             []HookUserPromptHint     `json:"hints,omitempty"`
	ProjectDocs       []ProjectDocCatalogEntry `json:"project_docs,omitempty"`
}

type HookUserPromptHint = Hint

func BuildUserPromptMCPHints(req HookUserPromptRequest) HookUserPromptResult {
	prompt := strings.TrimSpace(req.Prompt)
	result := HookUserPromptResult{
		OK:          true,
		Kind:        "hook_user_prompt",
		GeneratedAt: time.Now().Format(time.RFC3339),
		Hints:       []HookUserPromptHint{},
	}
	if prompt == "" {
		return result
	}
	lower := strings.ToLower(prompt)
	addPriority := func(tool, reason, priority string) {
		for _, h := range result.Hints {
			if h.Tool == tool && h.Reason == reason && h.Priority == priority {
				return
			}
		}
		result.Hints = append(result.Hints, HookUserPromptHint{Tool: tool, Reason: reason, Priority: priority})
	}
	addPriority("project_docs_route", "Use when the prompt is broad, ambiguous, or needs repo-specific document selection beyond static hook hints.", hintPriorityRoute)

	// Doc selection is no longer prescribed by keyword matching: the project-doc
	// catalog injected below presents every doc and what it contains, and the
	// main agent decides which to read. Keyword rules now only route to MCP
	// tools/actions, never to a required/consider doc verdict.
	for _, rule := range hookRoutingRules {
		if rule.Matches(prompt, lower, req.EnableLLMHints) {
			addPriority(rule.Tool, rule.Reason, rule.Priority)
		}
	}

	pendingUpkeep := []DocUpkeepEvent{}
	var repoProfile *ProjectProfile
	if strings.TrimSpace(req.Repo) != "" {
		if state, err := ResolveProjectLifecycleState(req.Repo); err == nil && state.Exists && state.NamespaceValid && state.Profile != nil {
			repoProfile = state.Profile.Metadata
		}
		if events, _, err := ReadPendingDocUpkeepEvents(req.Repo, 5); err == nil && len(events) > 0 {
			pendingUpkeep = events
			addPriority("project_docs_read/project_docs_update", "Pending lifecycle state indicates shared .agent-harness docs may need an evidence-preserving refresh.", hintPriorityAction)
		}
	}

	// The stable project-doc catalog is no longer injected per turn here; it is
	// established once via the SessionStart hook and re-established on PostCompact.
	// UserPromptSubmit now carries only the dynamic, per-turn signals (routing,
	// actions, profile, pending upkeep, rule).
	if len(result.Hints) == 0 && len(pendingUpkeep) == 0 && repoProfile == nil {
		return result
	}
	result.ShouldInject = true
	result.AdditionalContext = renderHookMCPHintContext(result.Hints, pendingUpkeep, repoProfile, "")
	return result
}
