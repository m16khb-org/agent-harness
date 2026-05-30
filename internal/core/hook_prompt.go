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
	OK                bool                 `json:"ok"`
	Kind              string               `json:"kind"`
	GeneratedAt       string               `json:"generated_at"`
	ShouldInject      bool                 `json:"should_inject"`
	AdditionalContext string               `json:"additional_context,omitempty"`
	Hints             []HookUserPromptHint `json:"hints,omitempty"`
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

	if containsAny(lower, "architecture", "architect", "refactor", "design", "decision", "alternative", "structure") || containsAny(prompt, "아키텍처", "리팩터", "결정", "대안", "구조", "설계") {
		addPriority("ARCHITECTURE.md", "Required when architecture, boundaries, host-neutral design, or component responsibilities shape the work.", hintPriorityRequired)
		addPriority("ADR.md", "Consider when a structural decision, trade-off, or rejected alternative matters long-term.", hintPriorityConsider)
		addAction("project_docs_record", "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.")
	}
	if containsAny(lower, "hook", "install", "bootstrap", "update", "daemon", "mcp", "operation", "local", "run") || containsAny(prompt, "훅", "설치", "부트스트랩", "데몬", "운영", "로컬 실행") {
		addPriority("OPERATIONS.md", "Required for install, update, hook registration, daemon, MCP, and local-run behavior.", hintPriorityRequired)
		addPriority("CONVENTIONS.md", "Required for package boundaries, adapter conventions, and hook implementation rules.", hintPriorityRequired)
		addPriority("TECH_STACK.md", "Consider when toolchain, companion tools, or runtime choices affect the work.", hintPriorityConsider)
	}
	if containsAny(lower, "test", "spec", "verify", "verification", "lint", "typecheck", "ci", "golden", "race", "qa") || containsAny(prompt, "테스트", "검증", "골든") {
		addPriority("TESTING.md", "Required for verification commands, gates, and expected test scope.", hintPriorityRequired)
		addPriority("AGENT_WORKFLOW.md", "Consider for agent workflow, routing, and validation expectations.", hintPriorityConsider)
	}
	if containsAny(lower, "bug", "fix", "regression", "failure", "false case", "caution") || containsAny(prompt, "버그", "고쳐", "회귀", "실패", "주의") {
		addPriority("CAUTIONS.md", "Consider when a resolved false case, recurring failure, or operational warning should be reusable.", hintPriorityConsider)
		addAction("project_docs_record", "When a resolved false case or recurring failure is reusable, consider kind=caution for CAUTIONS.md.")
	}
	if containsAny(lower, "endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler") || containsAny(prompt, "엔드포인트", "스웨거", "컨트롤러") {
		addPriority("OPEN_API_SPEC.md", "Required for endpoint, DTO, Swagger/OpenAPI, and public API error-contract documentation rules.", hintPriorityRequired)
		addAction("api_doc_static_check", "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.")
		addAction("api_doc_review", "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.")
		addAction("project_docs_read/project_docs_update", "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.")
	}
	if containsAny(lower, "commit", "push", "pull request", "release note") || containsAny(prompt, "커밋", "푸시", "PR", "릴리즈 노트") {
		addPriority("COMMIT_POLICY.md", "Required for Conventional Commit subject plus Lore body commit rules.", hintPriorityRequired)
	}
	if containsAny(lower, ".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules") || containsAny(prompt, "문서", "컨벤션", "최신화", "프로젝트 규칙") {
		addPriority("CONSTITUTION.md", "Consider for project document precedence, safety, accuracy, and architecture principles.", hintPriorityConsider)
		addPriority("CONVENTIONS.md", "Consider for implementation conventions and package boundaries.", hintPriorityConsider)
		addAction("project_docs_read/project_docs_update", "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.")
	}
	if containsAny(lower, "deploy", "release", "env", "environment") || containsAny(prompt, "배포", "환경 변수") {
		addPriority("OPERATIONS.md", "Required for operations, environment, or local-run work.", hintPriorityRequired)
		addPriority("TECH_STACK.md", "Consider when runtime or toolchain choices affect operations.", hintPriorityConsider)
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
	if strings.TrimSpace(req.Repo) != "" {
		if events, _, err := ReadPendingDocUpkeepEvents(req.Repo, 5); err == nil && len(events) > 0 {
			pendingUpkeep = events
			addAction("project_docs_read/project_docs_update", "Pending lifecycle state indicates shared .agent-harness docs may need an evidence-preserving refresh.")
		}
	}

	if len(result.Hints) == 0 && len(pendingUpkeep) == 0 {
		return result
	}
	result.ShouldInject = true
	result.AdditionalContext = renderHookMCPHintContext(result.Hints, pendingUpkeep)
	return result
}

func renderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []DocUpkeepEvent) string {
	groups := map[string][]HookUserPromptHint{}
	for _, h := range hints {
		priority := h.Priority
		if priority == "" {
			priority = fallbackHintPriority(h)
		}
		groups[priority] = append(groups[priority], h)
	}

	parts := []string{"[agent-harness] 프로젝트 지침 확인 중..."}
	appendCompactHintGroup(&parts, "required", filterDocs(groups[hintPriorityRequired]))
	appendCompactHintGroup(&parts, "consider", filterDocs(groups[hintPriorityConsider]))
	appendCompactHintGroup(&parts, "route", groups[hintPriorityRoute])
	appendCompactHintGroup(&parts, "actions", groups[hintPriorityAction])
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

func filterDocs(hints []HookUserPromptHint) []HookUserPromptHint {
	var docs []HookUserPromptHint
	for _, h := range hints {
		if strings.HasSuffix(h.Tool, ".md") {
			docs = append(docs, h)
		}
	}
	return docs
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
	for _, event := range events {
		item := event.Summary
		if len(event.TargetDocs) > 0 {
			item = strings.Join(event.TargetDocs, ",") + " " + item
		}
		items = append(items, item)
	}
	*parts = append(*parts, "pending upkeep: "+strings.Join(items, "; "))
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
