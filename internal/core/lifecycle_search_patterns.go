package core

import (
	"strings"
)

func sourceSearchNeedsCodeGraph(args []string, repo string) bool {
	if !hasStructuralSourceSearchPattern(args) {
		return false
	}
	targets := []string{}
	for _, arg := range args {
		target := searchTargetToken(arg)
		if target == "" || strings.HasPrefix(target, "-") {
			continue
		}
		if looksLikeSearchTarget(target) {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return true
	}
	for _, target := range targets {
		if isDocsOrFixtureTarget(target) {
			continue
		}
		if !isRepoLocalSearchTarget(target, repo) {
			continue
		}
		return true
	}
	return false
}

func hasStructuralSourceSearchPattern(args []string) bool {
	for _, arg := range args {
		pattern := searchPatternToken(arg)
		if pattern == "" {
			continue
		}
		if looksLikeStructuralSourcePattern(pattern) {
			return true
		}
		if !strings.HasPrefix(pattern, "-") {
			return false
		}
	}
	return false
}

func searchPatternToken(token string) string {
	cleaned := strings.Trim(strings.TrimSpace(token), `"',;`)
	if cleaned == "" || strings.HasPrefix(cleaned, "-") || looksLikeSearchTarget(cleaned) {
		return ""
	}
	return cleaned
}

func looksLikeStructuralSourcePattern(pattern string) bool {
	lower := strings.ToLower(strings.TrimSpace(pattern))
	if lower == "" {
		return false
	}
	structuralNeedles := []string{
		"func ",
		"function ",
		"type ",
		"class ",
		"interface ",
		"struct ",
		"enum ",
		"def ",
		"impl ",
		"trait ",
		"extends ",
		"implements ",
		"@controller",
		"@injectable",
	}
	for _, needle := range structuralNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
