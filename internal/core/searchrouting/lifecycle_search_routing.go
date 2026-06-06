package searchrouting

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
)

func searchRoutingBlockReason(tool string, command string, repo string) string {
	switch {
	case isShellTool(tool):
		if shouldBlockRawStructuralSourceSearch(command, repo) {
			return "Use CodeGraph first for structural repo-local source search: call codegraph_context for broad areas, codegraph_search for symbols, or codegraph_trace for call paths. Keep raw grep/rg for exact strings, env keys, filenames, errors, docs, golden fixtures, or literal evidence."
		}
	case isCodeGraphTool(tool):
		if looksLikeExactSearchQuery(command) {
			return "Use rg first for exact text search such as env keys, filenames, error messages, TODO/comment/log text, or literal strings. Keep CodeGraph for symbols, call paths, module dependencies, and impact analysis."
		}
	}
	return ""
}

func shouldBlockRawStructuralSourceSearch(command string, repo string) bool {
	normalizedCommand := strings.ToLower(strings.TrimSpace(command))
	if normalizedCommand == "" {
		return false
	}
	searchArgs, ok := rawTextSearchArgs(normalizedCommand)
	if !ok {
		return false
	}
	return sourceSearchNeedsCodeGraph(searchArgs, repo)
}

func isCodeGraphTool(tool string) bool {
	normalized := strings.ToLower(strings.TrimSpace(tool))
	return normalized == "codegraph" || strings.HasPrefix(normalized, "codegraph_") || strings.Contains(normalized, "__codegraph__")
}

func isShellTool(tool string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "bash", "sh", "zsh", "shell", "exec", "run_command", "shell_command", "unified_exec", "exec_command":
		return true
	default:
		return false
	}
}

func rawTextSearchArgs(command string) ([]string, bool) {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		name := searchTokenName(token)
		if name == "git" && i+1 < len(tokens) && searchTokenName(tokens[i+1]) == "grep" {
			return tokens[i+2:], true
		}
		switch name {
		case "rg", "grep", "ag", "ack":
			return tokens[i+1:], true
		}
	}
	return nil, false
}

func searchTokenName(token string) string {
	cleaned := strings.Trim(token, `"'`)
	return filepath.Base(cleaned)
}
