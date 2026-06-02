package core

import (
	"strings"
	"time"
)

type HookUserPromptRequest struct {
	Prompt         string `json:"prompt"`
	Repo           string `json:"repo,omitempty"`
	EnableAgyHints bool   `json:"enable_agy_hints,omitempty"`
}

type HookUserPromptHint struct {
	Tool     string `json:"tool"`
	Reason   string `json:"reason"`
	Priority string `json:"priority,omitempty"`
}

const (
	hintPriorityRequired  = "required"
	hintPriorityConsider  = "consider"
	hintPriorityRoute     = "route"
	hintPriorityAction    = "action"
	hintPrioritySecondary = "secondary"
)

type HookRoutingRule struct {
	Tool            string
	Reason          string
	Priority        string
	LowerKeywords   []string
	PromptKeywords  []string
	RequireAgyOptIn bool
}

var hookRoutingRules = []HookRoutingRule{
	{
		Tool:           "issueops",
		Reason:         "Use the issue-driven workflow for problem intake -> domain grill -> issue -> plan -> TDD/subagents -> feedback -> PR/MR; hooks must not create issues or PRs.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"issueops", "issue-driven", "feedback loop", "pull request", "merge request"},
		PromptKeywords: []string{"문제 파악", "이슈 기반", "피드백 루프", "PR", "MR", "이슈"},
	},
	{
		Tool:           "project_docs_record",
		Reason:         "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"architecture", "architect", "refactor", "design", "decision", "alternative"},
		PromptKeywords: []string{"아키텍처", "리팩터", "결정", "대안", "설계"},
	},
	{
		Tool:           "project_docs_record",
		Reason:         "When a resolved false case or recurring failure is reusable, consider kind=caution for CAUTIONS.md.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"bug", "fix", "regression", "failure", "false case", "caution"},
		PromptKeywords: []string{"버그", "고쳐", "회귀", "실패", "주의"},
	},
	{
		Tool:           "api_doc_static_check",
		Reason:         "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "api_doc_review",
		Reason:         "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "project_docs_read/project_docs_update",
		Reason:         "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{"endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler"},
		PromptKeywords: []string{"엔드포인트", "스웨거", "컨트롤러"},
	},
	{
		Tool:           "project_docs_read/project_docs_update",
		Reason:         "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.",
		Priority:       hintPriorityAction,
		LowerKeywords:  []string{".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules"},
		PromptKeywords: []string{"문서", "컨벤션", "최신화", "프로젝트 규칙"},
	},
	{
		Tool:          "CodeGraph",
		Reason:        "Secondary hint: consider CodeGraph for repo-local symbol, call graph, impact, or trace questions.",
		Priority:      hintPrioritySecondary,
		LowerKeywords: []string{"codegraph", "symbol", "call graph", "impact", "trace", "caller", "callee"},
	},
	{
		Tool:          "LLM Wiki",
		Reason:        "Secondary hint: consider upstream LLM Wiki for explicit wiki, research, knowledge-base, query, or compile workflows.",
		Priority:      hintPrioritySecondary,
		LowerKeywords: []string{"llm-wiki", "wiki", "knowledge base", "research", "compile"},
	},
	{
		Tool:           "claude-mem",
		Reason:         "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.",
		Priority:       hintPrioritySecondary,
		LowerKeywords:  []string{"claude-mem", "agentmemory", "agent-memory", "memory", "previous session", "last time", "already solve", "already solved"},
		PromptKeywords: []string{"전에", "지난번", "이미 해결"},
	},
	{
		Tool:            "agy -p",
		Reason:          "Secondary hint: consider agy -p for foreground second-pass LLM review or background synthesis when extra model judgment is useful.",
		Priority:        hintPrioritySecondary,
		LowerKeywords:   []string{"review", "analyze", "analysis", "critique", "second opinion", "plan", "research"},
		PromptKeywords:  []string{"검토", "리뷰", "분석", "비평", "계획", "리서치", "조사"},
		RequireAgyOptIn: true,
	},
}

type HookUserPromptResult struct {
	OK                bool                     `json:"ok"`
	Kind              string                   `json:"kind"`
	GeneratedAt       string                   `json:"generated_at"`
	ShouldInject      bool                     `json:"should_inject"`
	AdditionalContext string                   `json:"additional_context,omitempty"`
	Hints             []HookUserPromptHint     `json:"hints,omitempty"`
	ProjectDocs       []ProjectDocCatalogEntry `json:"project_docs,omitempty"`
}

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
		if rule.matches(prompt, lower, req.EnableAgyHints) {
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

// ProjectDocCatalogContext is the stable project-doc menu injected at session
// boundaries (SessionStart, PostCompact) instead of every user prompt. Compact
// is the model-facing additionalContext; UserView is the pretty, user-visible
// counterpart for the systemMessage channel.
type ProjectDocCatalogContext struct {
	ShouldInject bool
	ProjectDocs  []ProjectDocCatalogEntry
	Compact      string
	UserView     string
}

// BuildProjectDocCatalogContext discovers the working repo's project docs and
// renders both the compact (model) and pretty (user) catalog views. ShouldInject
// is false when the repo has no .agent-harness docs.
func BuildProjectDocCatalogContext(repo string) ProjectDocCatalogContext {
	docs := DiscoverProjectDocs(repo)
	if len(docs) == 0 {
		return ProjectDocCatalogContext{}
	}
	return ProjectDocCatalogContext{
		ShouldInject: true,
		ProjectDocs:  docs,
		Compact:      FormatProjectDocCatalog(docs),
		UserView:     renderProjectDocCatalogUserView(docs),
	}
}

func renderProjectDocCatalogUserView(docs []ProjectDocCatalogEntry) string {
	if len(docs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("📚 agent-harness · 이 레포 project docs (관련된 것을 읽고 작업하세요)")
	for _, doc := range docs {
		name := strings.TrimPrefix(doc.RelPath, ".agent-harness/")
		desc := doc.Description
		if desc == "" {
			desc = doc.Title
		}
		b.WriteString("\n• " + name)
		if desc != "" {
			b.WriteString(" — " + desc)
		}
	}
	return b.String()
}

// RenderUserPromptUserView renders a human-facing, multi-line view of the hook
// result for the host's user-visible channel (systemMessage on Claude Code and
// Codex). It is the pretty counterpart to the compact, model-facing
// AdditionalContext: same project-doc menu, formatted for a person to read in
// the terminal. Returns "" when there is nothing worth showing the user.
//
// systemMessage multi-line rendering is host-defined and not formally
// documented; the content stays readable even if newlines are collapsed.
func RenderUserPromptUserView(result HookUserPromptResult) string {
	return renderProjectDocCatalogUserView(result.ProjectDocs)
}

// RenderUserPromptCodexContext renders only the full project-doc catalog for
// Codex. Codex has no separate hidden context channel for UserPromptSubmit
// hooks, so route/action/profile/pending-upkeep hints would also appear in the
// TUI "hook context:" row; keep those out of the Codex path and preserve the
// useful doc catalog instead.
func RenderUserPromptCodexContext(result HookUserPromptResult) string {
	return RenderUserPromptUserView(result)
}

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
	case "CodeGraph":
		return "CodeGraph for symbol/call-impact lookup"
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

func (rule HookRoutingRule) matches(prompt, lower string, enableAgyHints bool) bool {
	if rule.RequireAgyOptIn && !enableAgyHints {
		return false
	}
	return containsAnySlice(lower, rule.LowerKeywords) || containsAnySlice(prompt, rule.PromptKeywords)
}

func containsAnySlice(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
