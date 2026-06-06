package searchrouting

import (
	"path/filepath"
	"strings"
)

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
