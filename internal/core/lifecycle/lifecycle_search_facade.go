package lifecycle

import "agent-harness/internal/core/searchrouting"

func searchRoutingBlockReason(tool, command, repo string) string {
	return searchrouting.SearchRoutingBlockReason(tool, command, repo)
}

func searchTokenName(token string) string {
	return searchrouting.SearchTokenName(token)
}

func searchTargetToken(token string) string {
	return searchrouting.SearchTargetToken(token)
}

func searchPatternToken(token string) string {
	return searchrouting.SearchPatternToken(token)
}

func shouldBlockRawStructuralSourceSearch(command, repo string) bool {
	return searchrouting.ShouldBlockRawStructuralSourceSearch(command, repo)
}

func looksLikeExactSearchQuery(query string) bool {
	return searchrouting.LooksLikeExactSearchQuery(query)
}

func sourceSearchNeedsCodeGraph(args []string, repo string) bool {
	return searchrouting.SourceSearchNeedsCodeGraph(args, repo)
}

func hasStructuralSourceSearchPattern(args []string) bool {
	return searchrouting.HasStructuralSourceSearchPattern(args)
}

func isShellTool(tool string) bool {
	return searchrouting.IsShellTool(tool)
}

func isCodeGraphTool(tool string) bool {
	return searchrouting.IsCodeGraphTool(tool)
}
