package hookprompt

import (
	"strings"
	"time"
)

type HookUserPromptRequest struct {
	Prompt               string `json:"prompt"`
	Repo                 string `json:"repo,omitempty"`
	Host                 string `json:"host,omitempty"`
	SessionID            string `json:"session_id,omitempty"`
	EnableLLMHints       bool   `json:"enable_llm_hints,omitempty"`
	DisableKarpathyFirst bool   `json:"disable_karpathy_first,omitempty"`
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
	KarpathyFirst     bool                     `json:"karpathy_first,omitempty"`
	UserNotice        string                   `json:"user_notice,omitempty"`
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
	if handled, context := ApproveCodexKubectlLiveAccess(req.Repo, req.Host, req.SessionID, prompt); handled {
		result.ShouldInject = true
		result.AdditionalContext = context
		return result
	}
	karpathyFirst, cleaned := karpathyFirstDecision(prompt)
	optedOut := strings.HasPrefix(prompt, karpathyOptOutPrefix)
	if req.DisableKarpathyFirst {
		karpathyFirst = false
	}
	prompt = cleaned
	if prompt == "" {
		return result
	}
	karpathyLine := ""
	karpathyNotice := ""
	if karpathyFirst {
		karpathyLine = "- karpathy-first: " + karpathyFirstDirective
		karpathyNotice = KarpathyFirstUserNotice
	} else if !req.DisableKarpathyFirst && !optedOut {
		// A bare choice reply carries no content, but the Stop next-action
		// relay still knows what the numbers meant: expand the chosen option
		// back into an augmentable request.
		if index, text, ok := resolveChoiceExpansion(prompt, req.Repo); ok {
			karpathyFirst = true
			karpathyLine = karpathyChoiceContextLine(index, text)
			karpathyNotice = karpathyChoiceUserNotice(index)
		}
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
	if repoProfile != nil && strings.EqualFold(repoProfile.VCS.Provider, "gitlab") && promptLooksLikeVCSRemoteWork(prompt, lower) {
		addPriority("gitlab-usecase", "Required for GitLab repo remote work; distinguish linked items from child items and verify body, labels, assignee, target branch, and review-thread state.", hintPriorityRequired)
		addPriority(
			"project_docs_read/project_docs_update",
			"Read .agent-harness/VCS.md when present; after a successful exact-identity provider read, record only the portable recipe with SHA-CAS in the canonical worktree.",
			hintPriorityAction,
		)
		addPriority(
			"glab_api",
			"Discover a trusted host tool by the glab_api leaf, never by server namespace; validate exact URL identity, then use glab api fallback only when no valid MCP evidence exists.",
			hintPriorityAction,
		)
	}

	// The stable project-doc catalog is no longer injected per turn here; it is
	// established once via the SessionStart hook and re-established on PostCompact.
	// UserPromptSubmit now carries only the dynamic, per-turn signals (routing,
	// actions, profile, pending upkeep, rule).
	if len(result.Hints) == 0 && len(pendingUpkeep) == 0 && repoProfile == nil && !karpathyFirst {
		return result
	}
	result.ShouldInject = true
	context := renderHookMCPHintContext(result.Hints, pendingUpkeep, repoProfile, "")
	if strings.TrimSpace(req.Repo) != "" {
		parts := strings.Split(context, "\n")
		appendCompactWorktreeReminder(&parts, req.Repo)
		context = strings.Join(parts, "\n")
	}
	if karpathyFirst {
		context += "\n" + karpathyLine
		result.KarpathyFirst = true
		result.UserNotice = karpathyNotice
	}
	result.AdditionalContext = context
	return result
}

func promptLooksLikeVCSRemoteWork(prompt, lower string) bool {
	return containsAnySlice(lower, []string{"issue", "merge request", "pull request", " mr", " pr", "review", "branch", "child", "linked"}) ||
		containsAnySlice(prompt, []string{"이슈", "MR", "PR", "리뷰", "브랜치", "하위", "자식", "링크"})
}
