package core

import (
	"path/filepath"
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

func looksLikeExactSearchQuery(query string) bool {
	cleaned := strings.Trim(strings.TrimSpace(query), `"',;`)
	if cleaned == "" {
		return false
	}
	lower := strings.ToLower(cleaned)
	if strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "log.") || strings.Contains(lower, "console.") || strings.Contains(lower, "comment") {
		return true
	}
	if strings.Contains(cleaned, ".") && !strings.Contains(cleaned, " ") {
		ext := strings.ToLower(filepath.Ext(cleaned))
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".md", ".json", ".yaml", ".yml", ".toml", ".env":
			return true
		}
	}
	if strings.Contains(cleaned, " ") && looksLikeErrorPhrase(lower) {
		return true
	}
	hasUnderscore := strings.Contains(cleaned, "_")
	hasUpper := false
	hasLower := false
	for _, r := range cleaned {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		}
		if r >= 'a' && r <= 'z' {
			hasLower = true
		}
	}
	if hasUnderscore && hasUpper && !hasLower {
		return true
	}
	return strings.HasSuffix(cleaned, "Error") || strings.HasSuffix(cleaned, "Exception") || strings.HasSuffix(cleaned, "Failure")
}

func looksLikeErrorPhrase(lower string) bool {
	errorNeedles := []string{"cannot ", "can't ", "failed", "failure", "error", "undefined", "not found", "read property", "exception"}
	for _, needle := range errorNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isRepoLocalSearchTarget(target string, repo string) bool {
	cleaned := strings.TrimSpace(target)
	if cleaned == "" {
		return true
	}
	if !filepath.IsAbs(cleaned) {
		return true
	}
	root := strings.TrimSpace(repo)
	if root == "" {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(cleaned)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func searchTargetToken(token string) string {
	return strings.Trim(strings.TrimSpace(token), `"',;`)
}

func looksLikeSearchTarget(token string) bool {
	if token == "." || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "/") {
		return true
	}
	if strings.Contains(token, "/") || strings.Contains(token, "*") {
		return true
	}
	switch token {
	case "cmd", "internal", "pkg", "src", "app", "lib", "docs", "testdata":
		return true
	default:
		return strings.HasSuffix(token, ".md")
	}
}

func isDocsOrFixtureTarget(token string) bool {
	cleaned := strings.TrimPrefix(token, "./")
	if strings.HasSuffix(cleaned, ".md") {
		return true
	}
	base := filepath.Base(cleaned)
	if base == "readme" || strings.HasPrefix(base, "readme.") {
		return true
	}
	for _, segment := range strings.Split(cleaned, "/") {
		switch segment {
		case "docs", ".agent-harness", "testdata", "golden", "goldens", "fixture", "fixtures", "snapshot", "snapshots", "__snapshots__":
			return true
		}
	}
	return false
}
