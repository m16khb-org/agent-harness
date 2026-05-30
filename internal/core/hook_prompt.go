package core

import (
	"strings"
	"time"
)

type HookUserPromptRequest struct {
	Prompt string `json:"prompt"`
	Repo   string `json:"repo,omitempty"`
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
	addAction := func(tool, reason string) {
		addPriority(tool, reason, hintPriorityAction)
	}

	addPriority("project_docs_route", "Use when the prompt is broad, ambiguous, or needs repo-specific document selection beyond static hook hints.", hintPriorityRoute)

	// Doc selection is no longer prescribed by keyword matching: the project-doc
	// catalog injected below presents every doc and what it contains, and the
	// main agent decides which to read. Keyword rules now only route to MCP
	// tools/actions, never to a required/consider doc verdict.
	if containsAny(lower, "architecture", "architect", "refactor", "design", "decision", "alternative", "structure") || containsAny(prompt, "아키텍처", "리팩터", "결정", "대안", "구조", "설계") {
		addAction("project_docs_record", "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.")
	}
	if containsAny(lower, "bug", "fix", "regression", "failure", "false case", "caution") || containsAny(prompt, "버그", "고쳐", "회귀", "실패", "주의") {
		addAction("project_docs_record", "When a resolved false case or recurring failure is reusable, consider kind=caution for CAUTIONS.md.")
	}
	if containsAny(lower, "endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler") || containsAny(prompt, "엔드포인트", "스웨거", "컨트롤러") {
		addAction("api_doc_static_check", "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.")
		addAction("api_doc_review", "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.")
		addAction("project_docs_read/project_docs_update", "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.")
	}
	if containsAny(lower, ".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules") || containsAny(prompt, "문서", "컨벤션", "최신화", "프로젝트 규칙") {
		addAction("project_docs_read/project_docs_update", "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.")
	}

	if containsAny(lower, "codegraph", "symbol", "call graph", "impact", "trace", "caller", "callee") {
		addPriority("CodeGraph", "Secondary hint: consider CodeGraph for repo-local symbol, call graph, impact, or trace questions.", hintPrioritySecondary)
	}
	if containsAny(lower, "llm-wiki", "wiki", "knowledge base", "research", "compile") {
		addPriority("LLM Wiki", "Secondary hint: consider upstream LLM Wiki for explicit wiki, research, knowledge-base, query, or compile workflows.", hintPrioritySecondary)
	}
	if containsAny(lower, "claude-mem", "memory", "previous session", "last time", "already solve", "already solved") || containsAny(prompt, "전에", "지난번", "이미 해결") {
		addPriority("claude-mem", "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.", hintPrioritySecondary)
	}

	pendingUpkeep := []DocUpkeepEvent{}
	var repoProfile *ProjectProfile
	if strings.TrimSpace(req.Repo) != "" {
		if state, err := ResolveProjectLifecycleState(req.Repo); err == nil && state.Exists && state.NamespaceValid && state.Profile != nil {
			repoProfile = state.Profile.Metadata
		}
		if events, _, err := ReadPendingDocUpkeepEvents(req.Repo, 5); err == nil && len(events) > 0 {
			pendingUpkeep = events
			addAction("project_docs_read/project_docs_update", "Pending lifecycle state indicates shared .agent-harness docs may need an evidence-preserving refresh.")
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

	parts := []string{"[agent-harness] 프로젝트 지침 확인 중..."}
	if catalog != "" {
		parts = append(parts, catalog)
	}
	appendCompactHintGroup(&parts, "route", groups[hintPriorityRoute])
	appendCompactHintGroup(&parts, "actions", groups[hintPriorityAction])
	appendCompactProjectProfile(&parts, profile)
	appendCompactPendingUpkeep(&parts, pendingUpkeep)
	appendCompactHintGroup(&parts, "secondary", groups[hintPrioritySecondary])
	parts = append(parts, "rule: use docs/tools only when material; writes must be evidence-preserving and non-destructive")
	return strings.Join(parts, " | ")
}

func fallbackHintPriority(h HookUserPromptHint) string {
	switch {
	case strings.HasSuffix(h.Tool, ".md"):
		return hintPriorityConsider
	case h.Tool == "CodeGraph" || h.Tool == "LLM Wiki" || h.Tool == "claude-mem":
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

func compactHintLabel(h HookUserPromptHint) string {
	switch h.Tool {
	case "project_docs_route":
		return "choose project docs if ambiguous"
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
	case "CodeGraph":
		return "CodeGraph for symbol/call-impact lookup"
	case "LLM Wiki":
		return "LLM Wiki for explicit wiki/research work"
	case "claude-mem":
		return "claude-mem for previous-session memory"
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

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
