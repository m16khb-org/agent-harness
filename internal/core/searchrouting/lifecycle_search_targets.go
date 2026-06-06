package searchrouting

import (
	"path/filepath"
	"strings"
)

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
