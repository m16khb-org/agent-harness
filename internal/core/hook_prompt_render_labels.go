package core

import "strings"

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
	case "vcs_remote_auth":
		return "VCS remote work: use authenticated CLI first; on missing token or auth/permission error use MCP fallback; do not print tokens"
	case "CodeGraph":
		return "CodeGraph for structural lookup; rg for exact strings"
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
