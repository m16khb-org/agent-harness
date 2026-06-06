package searchrouting

func SearchRoutingBlockReason(tool, command, repo string) string {
	return searchRoutingBlockReason(tool, command, repo)
}

func SearchTokenName(token string) string {
	return searchTokenName(token)
}

func SearchTargetToken(token string) string {
	return searchTargetToken(token)
}

func SearchPatternToken(token string) string {
	return searchPatternToken(token)
}

func ShouldBlockRawStructuralSourceSearch(command, repo string) bool {
	return shouldBlockRawStructuralSourceSearch(command, repo)
}

func LooksLikeExactSearchQuery(query string) bool {
	return looksLikeExactSearchQuery(query)
}

func SourceSearchNeedsCodeGraph(args []string, repo string) bool {
	return sourceSearchNeedsCodeGraph(args, repo)
}

func HasStructuralSourceSearchPattern(args []string) bool {
	return hasStructuralSourceSearchPattern(args)
}

func IsShellTool(tool string) bool {
	return isShellTool(tool)
}

func IsCodeGraphTool(tool string) bool {
	return isCodeGraphTool(tool)
}
