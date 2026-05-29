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
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

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
	add := func(tool, reason string) {
		for _, h := range result.Hints {
			if h.Tool == tool && h.Reason == reason {
				return
			}
		}
		result.Hints = append(result.Hints, HookUserPromptHint{Tool: tool, Reason: reason})
	}

	add("project_docs_route", "Before acting, decide which AGENTS.md/.agent-harness documents are required for this task.")

	if containsAny(lower, "architecture", "architect", "refactor", "design", "decision", "alternative", "structure") || containsAny(prompt, "아키텍처", "리팩터", "결정", "대안", "구조", "설계") {
		add("ARCHITECTURE.md", "Read when architecture, boundaries, host-neutral design, or component responsibilities shape the work.")
		add("ADR.md", "Read or update when a structural decision, trade-off, or rejected alternative matters long-term.")
		add("project_docs_record", "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.")
	}
	if containsAny(lower, "hook", "install", "bootstrap", "update", "daemon", "mcp", "operation", "local", "run") || containsAny(prompt, "훅", "설치", "부트스트랩", "데몬", "운영", "로컬 실행") {
		add("OPERATIONS.md", "Read for install, update, hook registration, daemon, MCP, and local-run behavior.")
		add("CONVENTIONS.md", "Read for package boundaries, adapter conventions, and hook implementation rules.")
		add("TECH_STACK.md", "Read when toolchain, companion tools, or runtime choices affect the work.")
	}
	if containsAny(lower, "test", "spec", "verify", "verification", "lint", "typecheck", "ci", "golden", "race", "qa") || containsAny(prompt, "테스트", "검증", "골든") {
		add("TESTING.md", "Read for verification commands, gates, and expected test scope.")
		add("AGENT_WORKFLOW.md", "Read for agent workflow, routing, and validation expectations.")
	}
	if containsAny(lower, "bug", "fix", "regression", "failure", "false case", "caution") || containsAny(prompt, "버그", "고쳐", "회귀", "실패", "주의") {
		add("CAUTIONS.md", "Read or update when a resolved false case, recurring failure, or operational warning should be reusable.")
		add("project_docs_record", "When a resolved false case or recurring failure is reusable, consider kind=caution for CAUTIONS.md.")
	}
	if containsAny(lower, "endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler") || containsAny(prompt, "엔드포인트", "스웨거", "컨트롤러") {
		add("OPEN_API_SPEC.md", "Read for endpoint, DTO, Swagger/OpenAPI, and public API error-contract documentation rules.")
		add("api_doc_static_check", "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.")
		add("api_doc_review", "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.")
		add("project_docs_read/project_docs_update", "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.")
	}
	if containsAny(lower, "commit", "push", "pull request", "release note") || containsAny(prompt, "커밋", "푸시", "PR", "릴리즈 노트") {
		add("COMMIT_POLICY.md", "Read for Conventional Commit subject plus Lore body commit rules.")
	}
	if containsAny(lower, ".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs", "project rules") || containsAny(prompt, "문서", "컨벤션", "최신화", "프로젝트 규칙") {
		add("CONSTITUTION.md", "Read for project document precedence, safety, accuracy, and architecture principles.")
		add("CONVENTIONS.md", "Read for implementation conventions and package boundaries.")
		add("project_docs_read/project_docs_update", "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.")
	}
	if containsAny(lower, "deploy", "release", "env", "environment") || containsAny(prompt, "배포", "환경 변수") {
		add("OPERATIONS.md", "Read for operations, environment, or local-run work.")
		add("TECH_STACK.md", "Read when runtime or toolchain choices affect operations.")
	}

	if containsAny(lower, "codegraph", "symbol", "call graph", "impact", "trace", "caller", "callee") {
		add("CodeGraph", "Secondary hint: consider CodeGraph for repo-local symbol, call graph, impact, or trace questions.")
	}
	if containsAny(lower, "llm-wiki", "wiki", "knowledge base", "research", "compile") {
		add("LLM Wiki", "Secondary hint: consider upstream LLM Wiki for explicit wiki, research, knowledge-base, query, or compile workflows.")
	}
	if containsAny(lower, "claude-mem", "memory", "previous session", "last time", "already solve", "already solved") || containsAny(prompt, "전에", "지난번", "이미 해결") {
		add("claude-mem", "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.")
	}

	if len(result.Hints) == 0 {
		return result
	}
	result.ShouldInject = true
	result.AdditionalContext = renderHookMCPHintContext(result.Hints)
	return result
}

func renderHookMCPHintContext(hints []HookUserPromptHint) string {
	var docs []HookUserPromptHint
	var actions []HookUserPromptHint
	var companions []HookUserPromptHint
	for _, h := range hints {
		switch {
		case strings.HasSuffix(h.Tool, ".md"):
			docs = append(docs, h)
		case h.Tool == "CodeGraph" || h.Tool == "LLM Wiki" || h.Tool == "claude-mem":
			companions = append(companions, h)
		default:
			actions = append(actions, h)
		}
	}

	var b strings.Builder
	b.WriteString("agent_harness project-doc routing hint:\n")
	b.WriteString("- Before acting, decide whether AGENTS.md or .agent-harness documents are necessary for the current request. Use repo evidence, routing, validation, or durable records only when they materially improve correctness; do not call MCP for simple reasoning.\n")
	writeHintGroup(&b, "Project document candidates:", docs)
	writeHintGroup(&b, "MCP/action candidates:", actions)
	writeHintGroup(&b, "Secondary companion-tool hints:", companions)
	b.WriteString("- Writable tools must preserve user consensus and current file evidence; never use them for destructive actions.")
	return strings.TrimRight(b.String(), "\n")
}

func writeHintGroup(b *strings.Builder, title string, hints []HookUserPromptHint) {
	if len(hints) == 0 {
		return
	}
	b.WriteString(title)
	b.WriteString("\n")
	for _, h := range hints {
		b.WriteString("- ")
		b.WriteString(h.Tool)
		b.WriteString(": ")
		b.WriteString(h.Reason)
		b.WriteString("\n")
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
