package hookprompt

import (
	"strings"
	"time"
)

type HookUserPromptRequest struct {
	Prompt         string `json:"prompt"`
	Repo           string `json:"repo,omitempty"`
	EnableAgyHints bool   `json:"enable_agy_hints,omitempty"`
}

const (
	hintPriorityRequired  = PriorityRequired
	hintPriorityConsider  = PriorityConsider
	hintPriorityRoute     = PriorityRoute
	hintPriorityAction    = PriorityAction
	hintPrioritySecondary = PrioritySecondary
)

// nextActionPolicyHint carries the next-action / auto-proceed policy to the main
// agent every turn via UserPromptSubmit. This replaces the external-LLM auto-proceed
// gate (too slow at ~13-25s for a Stop hook): the Stop hook only parses choices
// and applies hard guards, then asks the main agent to use conversation context for
// the final proceed-or-ask judgement. Keep it one concise line.
const nextActionPolicyHint = "next-action: end only user-decision turns with a '선택지:' block — exactly 3 numbered options and exactly one '(추천)'. Mark '(추천)' only when the main agent judges that option safe, reversible, and aligned with user intent from current context. The Stop hook may re-enter the main agent to re-check that recommendation; proceed only if that judgement still holds, otherwise ask for user confirmation. The main agent must make exactly one decision when re-entered from Stop hook choices: auto-proceed or no-auto-proceed, never both in the same answer. State the auto-proceed or no-auto-proceed rationale before acting or stopping. If the recommended option is continued implementation or verification and current user instructions already authorize automatic continuation, first state the safety/reversibility/alignment judgement and then execute when that judgement supports auto-proceed. A no-auto-proceed judgement is sticky across automated goal continuation; do not resume the same action unless there is an explicit user choice or a new user instruction. Auto-proceed result reports must still end with choices. No-auto-proceed judgements must stop without adding another choices block. Do not mark destructive, irreversible, or uncertain actions as '(추천)'."
const draftWikiPolicyHint = "draft-wiki: main agent must judge whether current work produced reusable long-term knowledge; heuristics must not queue it. If yes, explicitly run `agent-harness project draft-wiki queue --repo <repo> --stdin <<'EOF'` (or `--input <file>`); otherwise do nothing."

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
		if rule.Matches(prompt, lower, req.EnableAgyHints) {
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
