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
	add("project_docs_route", "Before starting, route the task to the relevant AGENTS.md/.agent-harness documents.")
	if containsAny(lower, "endpoint", "controller", "dto", "openapi", "swagger", "api doc", "api-doc", "route method", "handler") || containsAny(prompt, "엔드포인트", "스웨거", "컨트롤러") {
		add("api_doc_static_check", "For API/endpoint/DTO/OpenAPI changes, consider deterministic Swagger/OpenAPI gap checks before implementation or review.")
		add("api_doc_review", "Use agent review to compare business-logic error paths such as 400/401/403/404/409 with the documented API contract.")
		add("project_docs_read/project_docs_update", "If .agent-harness/OPEN_API_SPEC.md or related docs diverge from code/user consensus, update one document at a time.")
	}
	if containsAny(lower, "test", "spec", "verify", "verification", "lint", "typecheck", "ci") || containsAny(prompt, "테스트", "검증") {
		add("project_docs_route", "For testing or verification work, route to TESTING.md and AGENT_WORKFLOW.md.")
	}
	if containsAny(lower, "bug", "fix", "regression", "failure", "false case") || containsAny(prompt, "버그", "고쳐", "회귀", "실패") {
		add("project_docs_record", "When a resolved false case or recurring failure is reusable, consider project_docs_record with kind=caution for CAUTIONS.md.")
	}
	if containsAny(lower, "architecture", "architect", "refactor", "design", "decision", "alternative") || containsAny(prompt, "아키텍처", "리팩터", "결정", "대안", "구조") {
		add("project_docs_record", "When a structural decision or rejected alternative matters long-term, consider project_docs_record with kind=adr for ADR.md.")
	}
	if containsAny(lower, "deploy", "release", "env", "local", "run", "operation") || containsAny(prompt, "배포", "환경 변수", "로컬 실행", "운영") {
		add("project_docs_route", "For operations, environment, or local-run work, route to OPERATIONS.md and TECH_STACK.md.")
	}
	if containsAny(lower, ".agent-harness", "agents.md", "claude.md", "convention", "workflow", "docs") || containsAny(prompt, "문서", "컨벤션", "최신화") {
		add("project_docs_read/project_docs_update", "If .agent-harness docs diverge from current code or user consensus, update one SHA-checked document at a time.")
	}
	if len(result.Hints) == 0 {
		return result
	}
	result.ShouldInject = true
	result.AdditionalContext = renderHookMCPHintContext(result.Hints)
	return result
}

func renderHookMCPHintContext(hints []HookUserPromptHint) string {
	var b strings.Builder
	b.WriteString("agent_harness MCP routing hint:\n")
	b.WriteString("- Before acting, decide whether any of these agent_harness MCP tools are necessary for the current request. Use them only for repo evidence, routing, validation, or durable records; do not call MCP for simple reasoning.\n")
	for _, h := range hints {
		b.WriteString("- ")
		b.WriteString(h.Tool)
		b.WriteString(": ")
		b.WriteString(h.Reason)
		b.WriteString("\n")
	}
	b.WriteString("- Writable tools must preserve user consensus and current file evidence; never use them for destructive actions.\n")
	return strings.TrimRight(b.String(), "\n")
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
